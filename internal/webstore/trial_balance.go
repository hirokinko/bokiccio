package webstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"github.com/hirokinko/bokiccio/internal/ledger"
	"github.com/hirokinko/bokiccio/internal/reporting"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

func (store *Store) GetTrialBalance(ctx context.Context, period reporting.Period) (_ webapp.TrialBalanceDetail, resultErr error) {
	transaction, configuration, entries, snapshotIdentity, err := store.reportingSnapshotWithIdentity(ctx, "trial balance")
	if err != nil {
		return webapp.TrialBalanceDetail{}, err
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	balance, err := reporting.BuildTrialBalance(configuration, entries, period)
	if err != nil {
		return webapp.TrialBalanceDetail{}, err
	}
	if err := transaction.Commit(); err != nil {
		return webapp.TrialBalanceDetail{}, fmt.Errorf("commit trial balance transaction: %w", err)
	}
	return webapp.TrialBalanceDetail{
		SchemaVersion: webapp.APISchemaVersion, SnapshotIdentity: snapshotIdentity, TrialBalance: balance,
	}, nil
}

func (store *Store) reportingSnapshot(ctx context.Context, reportName string) (*sql.Tx, reporting.Configuration, []reporting.Entry, error) {
	transaction, configuration, entries, _, err := store.reportingSnapshotWithIdentity(ctx, reportName)
	return transaction, configuration, entries, err
}

func (store *Store) reportingSnapshotWithIdentity(ctx context.Context, reportName string) (*sql.Tx, reporting.Configuration, []reporting.Entry, string, error) {
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, reporting.Configuration{}, nil, "", fmt.Errorf("begin %s transaction: %w", reportName, err)
	}
	var revision int
	if err := transaction.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(revision), 0) FROM reporting_configurations`).Scan(&revision); err != nil {
		_ = transaction.Rollback()
		return nil, reporting.Configuration{}, nil, "", fmt.Errorf("read %s configuration revision: %w", reportName, err)
	}
	if revision == 0 {
		_ = transaction.Rollback()
		return nil, reporting.Configuration{}, nil, "", webapp.ErrReportingNotConfigured
	}
	detail, err := loadReportingConfiguration(ctx, transaction, revision)
	if err != nil {
		_ = transaction.Rollback()
		return nil, reporting.Configuration{}, nil, "", err
	}
	entries, err := loadCurrentApprovedEntries(ctx, transaction)
	if err != nil {
		_ = transaction.Rollback()
		return nil, reporting.Configuration{}, nil, "", err
	}
	configuration := reportingConfigurationFromDetail(detail)
	return transaction, configuration, entries, reportingSnapshotIdentity(configuration, entries), nil
}

func reportingSnapshotIdentity(configuration reporting.Configuration, entries []reporting.Entry) string {
	hash := sha256.New()
	writeSnapshotField := func(value string) {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(value))
	}
	writeSnapshotField("bokiccio.reporting-snapshot")
	writeSnapshotField("v1")
	writeSnapshotField(strconv.Itoa(configuration.Revision))
	for _, item := range entries {
		writeSnapshotField(item.ID)
		writeSnapshotField(strconv.Itoa(int(item.Entry.Date.Precision())))
		writeSnapshotField(item.Entry.Date.String())
		writeSnapshotField(item.Entry.Description)
		writeSnapshotField(strconv.Itoa(len(item.Entry.Comments)))
		for _, comment := range item.Entry.Comments {
			writeSnapshotField(comment)
		}
		writeSnapshotField(strconv.Itoa(len(item.Entry.Postings)))
		for _, posting := range item.Entry.Postings {
			writeSnapshotField(posting.Account)
			writeSnapshotField(posting.Comment)
			if posting.Amount == nil {
				writeSnapshotField("no-amount")
			} else {
				writeSnapshotField("amount")
				writeSnapshotField(posting.Amount.Value.String())
				writeSnapshotField(string(posting.Amount.Commodity))
			}
			if posting.TotalPrice == nil {
				writeSnapshotField("no-total-price")
			} else {
				writeSnapshotField("total-price")
				writeSnapshotField(posting.TotalPrice.Value.String())
				writeSnapshotField(string(posting.TotalPrice.Commodity))
			}
		}
	}
	return hex.EncodeToString(hash.Sum(nil))
}

type approvedEntryBuilder struct {
	entry reporting.Entry
}

func loadCurrentApprovedEntries(ctx context.Context, transaction *sql.Tx) ([]reporting.Entry, error) {
	statement := currentEntriesQuery + `
		SELECT c.entry_id, c.occurred_precision, c.occurred_at, c.description
        FROM current_entries c
        WHERE c.workflow_status = 'approved'
        ORDER BY substr(c.occurred_at, 1, 10), c.occurred_precision,
                 CASE WHEN c.occurred_precision = 2 THEN julianday(c.occurred_at) END,
                 c.sequence, c.record_index`
	rows, err := transaction.QueryContext(ctx, statement)
	if err != nil {
		return nil, fmt.Errorf("query trial balance entries: %w", err)
	}
	builders := map[string]*approvedEntryBuilder{}
	order := []string{}
	for rows.Next() {
		var id, occurredAt, description string
		var precision int
		if err := rows.Scan(&id, &precision, &occurredAt, &description); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan trial balance entry: %w", err)
		}
		entryTime, err := ledger.ParseEntryTime(occurredAt)
		if err != nil || int(entryTime.Precision()) != precision {
			rows.Close()
			return nil, errors.New("stored trial balance entry time is invalid")
		}
		builders[id] = &approvedEntryBuilder{
			entry: reporting.Entry{ID: id, Entry: ledger.JournalEntry{
				Date: entryTime, Description: description, Comments: []string{}, Postings: []ledger.Posting{},
			}},
		}
		order = append(order, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate trial balance entries: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close trial balance entries: %w", err)
	}
	if len(order) == 0 {
		return []reporting.Entry{}, nil
	}
	if err := loadApprovedEntryComments(ctx, transaction, builders); err != nil {
		return nil, err
	}
	if err := loadApprovedEntryPostings(ctx, transaction, builders); err != nil {
		return nil, err
	}
	entries := make([]reporting.Entry, 0, len(order))
	for _, id := range order {
		entry := builders[id].entry
		if err := ledger.Validate(entry.Entry); err != nil {
			return nil, fmt.Errorf("stored approved entry is invalid: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func loadApprovedEntryComments(ctx context.Context, transaction *sql.Tx, builders map[string]*approvedEntryBuilder) error {
	statement := currentEntriesQuery + `,
approved_entries AS (
    SELECT entry_id, current_revision FROM current_entries WHERE workflow_status = 'approved'
),
approved_comments AS (
    SELECT c.entry_id AS entry_id, oc.comment_index AS comment_index, oc.comment AS comment
    FROM approved_entries c
    JOIN entry_comments oc ON oc.entry_id = c.entry_id
    WHERE c.current_revision = 0
    UNION ALL
    SELECT c.entry_id AS entry_id, rc.comment_index AS comment_index, rc.comment AS comment
    FROM approved_entries c
    JOIN revision_comments rc ON rc.entry_id = c.entry_id AND rc.revision = c.current_revision
    WHERE c.current_revision > 0
)
SELECT entry_id, comment_index, comment
FROM approved_comments
ORDER BY entry_id, comment_index`
	rows, err := transaction.QueryContext(ctx, statement)
	if err != nil {
		return fmt.Errorf("query trial balance comments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, comment string
		var index int
		if err := rows.Scan(&id, &index, &comment); err != nil {
			return fmt.Errorf("scan trial balance comment: %w", err)
		}
		builder, found := builders[id]
		if !found || index != len(builder.entry.Entry.Comments) {
			return errors.New("stored trial balance comments are invalid")
		}
		builder.entry.Entry.Comments = append(builder.entry.Entry.Comments, comment)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate trial balance comments: %w", err)
	}
	return nil
}

func loadApprovedEntryPostings(ctx context.Context, transaction *sql.Tx, builders map[string]*approvedEntryBuilder) error {
	statement := currentEntriesQuery + `,
approved_entries AS (
    SELECT entry_id, current_revision FROM current_entries WHERE workflow_status = 'approved'
),
approved_postings AS (
    SELECT c.entry_id AS entry_id, op.posting_index AS posting_index, op.account AS account,
           op.amount_text AS amount_text, op.amount_scale AS amount_scale, op.commodity AS commodity,
           op.total_price_amount_text AS total_price_amount_text,
           op.total_price_amount_scale AS total_price_amount_scale,
           op.total_price_commodity AS total_price_commodity, op.comment AS comment
    FROM approved_entries c
    JOIN postings op ON op.entry_id = c.entry_id
    WHERE c.current_revision = 0
    UNION ALL
    SELECT c.entry_id AS entry_id, rp.posting_index AS posting_index, rp.account AS account,
           rp.amount_text AS amount_text, rp.amount_scale AS amount_scale, rp.commodity AS commodity,
           rp.total_price_amount_text AS total_price_amount_text,
           rp.total_price_amount_scale AS total_price_amount_scale,
           rp.total_price_commodity AS total_price_commodity, rp.comment AS comment
    FROM approved_entries c
    JOIN revision_postings rp ON rp.entry_id = c.entry_id AND rp.revision = c.current_revision
    WHERE c.current_revision > 0
)
SELECT entry_id, posting_index, account, amount_text, amount_scale, commodity,
       total_price_amount_text, total_price_amount_scale, total_price_commodity, comment
FROM approved_postings
ORDER BY entry_id, posting_index`
	rows, err := transaction.QueryContext(ctx, statement)
	if err != nil {
		return fmt.Errorf("query trial balance postings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var index int
		var posting ledger.Posting
		var amountText, commodity, totalPriceText, totalPriceCommodity sql.NullString
		var amountScale, totalPriceScale sql.NullInt64
		if err := rows.Scan(&id, &index, &posting.Account, &amountText, &amountScale, &commodity,
			&totalPriceText, &totalPriceScale, &totalPriceCommodity, &posting.Comment); err != nil {
			return fmt.Errorf("scan trial balance posting: %w", err)
		}
		builder, found := builders[id]
		if !found || index != len(builder.entry.Entry.Postings) {
			return errors.New("stored trial balance postings are invalid")
		}
		if amountText.Valid != amountScale.Valid || amountText.Valid != commodity.Valid {
			return errors.New("stored trial balance posting amount is invalid")
		}
		if totalPriceText.Valid != totalPriceScale.Valid || totalPriceText.Valid != totalPriceCommodity.Valid {
			return errors.New("stored trial balance posting total price is invalid")
		}
		if amountText.Valid {
			value, err := ledger.ParseDecimal(amountText.String)
			if err != nil || int64(value.Scale()) != amountScale.Int64 {
				return errors.New("stored trial balance posting amount is invalid")
			}
			posting.Amount = &ledger.Amount{Value: value, Commodity: ledger.Commodity(commodity.String)}
		}
		if totalPriceText.Valid {
			value, err := ledger.ParseDecimal(totalPriceText.String)
			if err != nil || int64(value.Scale()) != totalPriceScale.Int64 {
				return errors.New("stored trial balance posting total price is invalid")
			}
			posting.TotalPrice = &ledger.Amount{Value: value, Commodity: ledger.Commodity(totalPriceCommodity.String)}
		}
		builder.entry.Entry.Postings = append(builder.entry.Entry.Postings, posting)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate trial balance postings: %w", err)
	}
	return nil
}

func reportingConfigurationFromDetail(detail webapp.ReportingConfigurationDetail) reporting.Configuration {
	baseRevision := detail.BaseRevision
	request := webapp.ReportingConfigurationRequest{
		BaseRevision: &baseRevision, StartMonth: detail.StartMonth,
		Classifications: detail.Classifications, FiscalYears: detail.FiscalYears,
	}
	return reportingConfiguration(detail.Revision, request)
}
