package webstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/hirokinko/bokiccio/internal/reporting"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

func (store *Store) GetCurrentReportingConfiguration(ctx context.Context) (webapp.ReportingConfigurationDetail, error) {
	return store.getReportingConfiguration(ctx, 0)
}

func (store *Store) GetReportingConfiguration(ctx context.Context, revision int) (webapp.ReportingConfigurationDetail, error) {
	if revision < 1 {
		return webapp.ReportingConfigurationDetail{}, webapp.ErrInvalidRequest
	}
	return store.getReportingConfiguration(ctx, revision)
}

func (store *Store) getReportingConfiguration(ctx context.Context, revision int) (_ webapp.ReportingConfigurationDetail, resultErr error) {
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return webapp.ReportingConfigurationDetail{}, fmt.Errorf("begin reporting configuration query: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	if revision == 0 {
		if err := transaction.QueryRowContext(ctx,
			`SELECT COALESCE(MAX(revision), 0) FROM reporting_configurations`).Scan(&revision); err != nil {
			return webapp.ReportingConfigurationDetail{}, fmt.Errorf("read current reporting configuration revision: %w", err)
		}
		if revision == 0 {
			return webapp.ReportingConfigurationDetail{}, webapp.ErrReportingNotConfigured
		}
	}
	detail, err := loadReportingConfiguration(ctx, transaction, revision)
	if err != nil {
		return webapp.ReportingConfigurationDetail{}, err
	}
	if err := transaction.Commit(); err != nil {
		return webapp.ReportingConfigurationDetail{}, fmt.Errorf("commit reporting configuration query: %w", err)
	}
	return detail, nil
}

func (store *Store) CreateReportingConfiguration(ctx context.Context, request webapp.ReportingConfigurationRequest) (_ webapp.ReportingConfigurationDetail, resultErr error) {
	if request.BaseRevision == nil || *request.BaseRevision < 0 {
		return webapp.ReportingConfigurationDetail{}, webapp.ErrInvalidRequest
	}
	revision := *request.BaseRevision + 1
	configuration := reportingConfiguration(revision, request)
	if err := reporting.ValidateConfiguration(configuration); err != nil {
		return webapp.ReportingConfigurationDetail{}, fmt.Errorf("%w: %w", webapp.ErrInvalidRequest, err)
	}

	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return webapp.ReportingConfigurationDetail{}, fmt.Errorf("begin reporting configuration transaction: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	var latest int
	if err := transaction.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(revision), 0) FROM reporting_configurations`).Scan(&latest); err != nil {
		return webapp.ReportingConfigurationDetail{}, fmt.Errorf("read latest reporting configuration revision: %w", err)
	}
	if latest != *request.BaseRevision {
		return webapp.ReportingConfigurationDetail{}, webapp.ErrConflict
	}
	if err := validateOpeningEntries(ctx, transaction, configuration); err != nil {
		return webapp.ReportingConfigurationDetail{}, err
	}

	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := transaction.ExecContext(ctx, `INSERT INTO reporting_configurations
        (revision, base_revision, created_at, start_month)
        SELECT ?, ?, ?, ?
        WHERE ? = (SELECT COALESCE(MAX(revision), 0) FROM reporting_configurations)`,
		revision, latest, createdAt, request.StartMonth, latest)
	if err != nil {
		return webapp.ReportingConfigurationDetail{}, fmt.Errorf("save reporting configuration: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return webapp.ReportingConfigurationDetail{}, fmt.Errorf("inspect reporting configuration insert: %w", err)
	}
	if inserted != 1 {
		return webapp.ReportingConfigurationDetail{}, webapp.ErrConflict
	}
	if err := saveReportingConfigurationParts(ctx, transaction, revision, request); err != nil {
		return webapp.ReportingConfigurationDetail{}, err
	}
	if err := transaction.Commit(); err != nil {
		return webapp.ReportingConfigurationDetail{}, fmt.Errorf("commit reporting configuration: %w", err)
	}
	return reportingConfigurationDetail(revision, latest, createdAt, request), nil
}

func saveReportingConfigurationParts(ctx context.Context, transaction *sql.Tx, revision int, request webapp.ReportingConfigurationRequest) error {
	for _, classification := range request.Classifications {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO reporting_classifications
            (revision, account, category) VALUES (?, ?, ?)`, revision, classification.Account, classification.Category); err != nil {
			return fmt.Errorf("save reporting classification: %w", err)
		}
	}
	for _, year := range request.FiscalYears {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO reporting_fiscal_years
            (revision, start_date, end_date, opening_mode) VALUES (?, ?, ?, ?)`,
			revision, year.StartDate, year.EndDate, year.OpeningMode); err != nil {
			return fmt.Errorf("save reporting fiscal year: %w", err)
		}
		for index, entryID := range year.OpeningEntryIDs {
			if _, err := transaction.ExecContext(ctx, `INSERT INTO reporting_opening_entries
                (revision, fiscal_year_start, fiscal_year_end, entry_index, entry_id)
                VALUES (?, ?, ?, ?, ?)`, revision, year.StartDate, year.EndDate, index, entryID); err != nil {
				return fmt.Errorf("save reporting opening entry: %w", err)
			}
		}
	}
	return nil
}

func validateOpeningEntries(ctx context.Context, transaction *sql.Tx, configuration reporting.Configuration) error {
	for _, year := range configuration.FiscalYears {
		for _, entryID := range year.OpeningEntryIDs {
			entry, err := loadCurrentApprovedEntry(ctx, transaction, entryID)
			if err != nil {
				if errors.Is(err, webapp.ErrNotFound) || errors.Is(err, webapp.ErrInvalidRevision) {
					return &webapp.ReportingConfigurationError{Code: webapp.ReportingOpeningEntryNotApproved}
				}
				return err
			}
			if entry.Entry.Date.String()[:10] != year.StartDate {
				return &webapp.ReportingConfigurationError{Code: webapp.ReportingOpeningEntryDateMismatch}
			}
			for _, posting := range entry.Entry.Postings {
				category, err := reporting.Classify(configuration, posting.Account)
				if err != nil || (category != reporting.CategoryAsset && category != reporting.CategoryLiability && category != reporting.CategoryEquity) {
					return &webapp.ReportingConfigurationError{Code: webapp.ReportingOpeningEntryTemporaryAccount}
				}
			}
		}
	}
	return nil
}

func loadCurrentApprovedEntry(ctx context.Context, transaction *sql.Tx, entryID string) (reporting.Entry, error) {
	statement := currentEntriesQuery + `
        SELECT c.current_revision
        FROM current_entries c
        WHERE c.entry_id = ? AND c.workflow_status = 'approved'`
	var revision int
	if err := transaction.QueryRowContext(ctx, statement, entryID).Scan(&revision); err != nil {
		return reporting.Entry{}, notFound(err)
	}
	snapshot, err := loadEntrySnapshot(ctx, transaction, entryID, revision)
	if err != nil {
		return reporting.Entry{}, err
	}
	return reporting.Entry{ID: entryID, Entry: snapshot}, nil
}

func loadReportingConfiguration(ctx context.Context, transaction *sql.Tx, revision int) (webapp.ReportingConfigurationDetail, error) {
	detail := webapp.ReportingConfigurationDetail{
		SchemaVersion:   webapp.APISchemaVersion,
		Revision:        revision,
		Classifications: []webapp.ReportingClassification{},
		FiscalYears:     []webapp.ReportingFiscalYear{},
	}
	if err := transaction.QueryRowContext(ctx, `SELECT base_revision, created_at, start_month
        FROM reporting_configurations WHERE revision = ?`, revision).
		Scan(&detail.BaseRevision, &detail.CreatedAt, &detail.StartMonth); err != nil {
		return webapp.ReportingConfigurationDetail{}, notFound(err)
	}
	rows, err := transaction.QueryContext(ctx, `SELECT account, category FROM reporting_classifications
        WHERE revision = ? ORDER BY account`, revision)
	if err != nil {
		return webapp.ReportingConfigurationDetail{}, fmt.Errorf("query reporting classifications: %w", err)
	}
	for rows.Next() {
		var classification webapp.ReportingClassification
		if err := rows.Scan(&classification.Account, &classification.Category); err != nil {
			rows.Close()
			return webapp.ReportingConfigurationDetail{}, fmt.Errorf("scan reporting classification: %w", err)
		}
		detail.Classifications = append(detail.Classifications, classification)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return webapp.ReportingConfigurationDetail{}, fmt.Errorf("iterate reporting classifications: %w", err)
	}
	if err := rows.Close(); err != nil {
		return webapp.ReportingConfigurationDetail{}, fmt.Errorf("close reporting classifications: %w", err)
	}
	years, err := transaction.QueryContext(ctx, `SELECT start_date, end_date, opening_mode
        FROM reporting_fiscal_years WHERE revision = ? ORDER BY start_date`, revision)
	if err != nil {
		return webapp.ReportingConfigurationDetail{}, fmt.Errorf("query reporting fiscal years: %w", err)
	}
	for years.Next() {
		year := webapp.ReportingFiscalYear{OpeningEntryIDs: []string{}}
		if err := years.Scan(&year.StartDate, &year.EndDate, &year.OpeningMode); err != nil {
			years.Close()
			return webapp.ReportingConfigurationDetail{}, fmt.Errorf("scan reporting fiscal year: %w", err)
		}
		detail.FiscalYears = append(detail.FiscalYears, year)
	}
	if err := years.Err(); err != nil {
		years.Close()
		return webapp.ReportingConfigurationDetail{}, fmt.Errorf("iterate reporting fiscal years: %w", err)
	}
	if err := years.Close(); err != nil {
		return webapp.ReportingConfigurationDetail{}, fmt.Errorf("close reporting fiscal years: %w", err)
	}
	for index := range detail.FiscalYears {
		year := &detail.FiscalYears[index]
		opening, err := transaction.QueryContext(ctx, `SELECT entry_id FROM reporting_opening_entries
            WHERE revision = ? AND fiscal_year_start = ? AND fiscal_year_end = ? ORDER BY entry_index`,
			revision, year.StartDate, year.EndDate)
		if err != nil {
			return webapp.ReportingConfigurationDetail{}, fmt.Errorf("query reporting opening entries: %w", err)
		}
		for opening.Next() {
			var entryID string
			if err := opening.Scan(&entryID); err != nil {
				opening.Close()
				return webapp.ReportingConfigurationDetail{}, fmt.Errorf("scan reporting opening entry: %w", err)
			}
			year.OpeningEntryIDs = append(year.OpeningEntryIDs, entryID)
		}
		if err := opening.Err(); err != nil {
			opening.Close()
			return webapp.ReportingConfigurationDetail{}, fmt.Errorf("iterate reporting opening entries: %w", err)
		}
		if err := opening.Close(); err != nil {
			return webapp.ReportingConfigurationDetail{}, fmt.Errorf("close reporting opening entries: %w", err)
		}
	}
	return detail, nil
}

func reportingConfiguration(revision int, request webapp.ReportingConfigurationRequest) reporting.Configuration {
	configuration := reporting.Configuration{Revision: revision, StartMonth: request.StartMonth}
	for _, classification := range request.Classifications {
		configuration.Classifications = append(configuration.Classifications, reporting.Classification{
			Account: classification.Account, Category: classification.Category,
		})
	}
	for _, year := range request.FiscalYears {
		configuration.FiscalYears = append(configuration.FiscalYears, reporting.FiscalYear{
			StartDate: year.StartDate, EndDate: year.EndDate, OpeningMode: year.OpeningMode,
			OpeningEntryIDs: append([]string(nil), year.OpeningEntryIDs...),
		})
	}
	return configuration
}

func reportingConfigurationDetail(revision, baseRevision int, createdAt string, request webapp.ReportingConfigurationRequest) webapp.ReportingConfigurationDetail {
	detail := webapp.ReportingConfigurationDetail{
		SchemaVersion: webapp.APISchemaVersion, Revision: revision, BaseRevision: baseRevision,
		CreatedAt: createdAt, StartMonth: request.StartMonth,
		Classifications: append([]webapp.ReportingClassification(nil), request.Classifications...),
		FiscalYears:     make([]webapp.ReportingFiscalYear, len(request.FiscalYears)),
	}
	for index, year := range request.FiscalYears {
		detail.FiscalYears[index] = year
		detail.FiscalYears[index].OpeningEntryIDs = append([]string{}, year.OpeningEntryIDs...)
	}
	return detail
}
