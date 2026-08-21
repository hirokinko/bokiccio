package webstore

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hirokinko/bokiccio/internal/reporting"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

type drillDownCursor struct {
	Offset   int    `json:"o"`
	Selector string `json:"s"`
}

func (store *Store) GetTrialBalanceDrillDown(ctx context.Context, query webapp.ReportDrillDownQuery) (_ webapp.TrialBalanceDrillDownDetail, resultErr error) {
	if err := validateReportDrillDownQuery(query); err != nil {
		return webapp.TrialBalanceDrillDownDetail{}, reporting.ErrInvalidDrillDown
	}
	transaction, configuration, entries, identity, err := store.reportingSnapshotWithIdentity(ctx, "trial balance drill-down")
	if err != nil {
		return webapp.TrialBalanceDrillDownDetail{}, err
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	if !sameSnapshotIdentity(query.SnapshotIdentity, identity) {
		return webapp.TrialBalanceDrillDownDetail{}, webapp.ErrReportSnapshotChanged
	}
	result, err := reporting.BuildTrialBalanceDrillDown(configuration, entries, query.DrillDown)
	if err != nil {
		return webapp.TrialBalanceDrillDownDetail{}, err
	}
	total := len(result.Entries)
	next, err := paginateTrialBalanceDrillDown(&result, query)
	if err != nil {
		return webapp.TrialBalanceDrillDownDetail{}, reporting.ErrInvalidDrillDown
	}
	if err := transaction.Commit(); err != nil {
		return webapp.TrialBalanceDrillDownDetail{}, fmt.Errorf("commit trial balance drill-down transaction: %w", err)
	}
	return webapp.TrialBalanceDrillDownDetail{
		SchemaVersion: webapp.APISchemaVersion, SnapshotIdentity: identity, TotalEntries: total,
		NextCursor: next, TrialBalanceDrillDown: result,
	}, nil
}

func (store *Store) GetIncomeStatementDrillDown(ctx context.Context, query webapp.ReportDrillDownQuery) (_ webapp.IncomeStatementDrillDownDetail, resultErr error) {
	if err := validateReportDrillDownQuery(query); err != nil {
		return webapp.IncomeStatementDrillDownDetail{}, reporting.ErrInvalidDrillDown
	}
	transaction, configuration, entries, identity, err := store.reportingSnapshotWithIdentity(ctx, "income statement drill-down")
	if err != nil {
		return webapp.IncomeStatementDrillDownDetail{}, err
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	if !sameSnapshotIdentity(query.SnapshotIdentity, identity) {
		return webapp.IncomeStatementDrillDownDetail{}, webapp.ErrReportSnapshotChanged
	}
	result, err := reporting.BuildIncomeStatementDrillDown(configuration, entries, query.DrillDown)
	if err != nil {
		return webapp.IncomeStatementDrillDownDetail{}, err
	}
	total := len(result.Entries)
	next, err := paginateIncomeStatementDrillDown(&result, query)
	if err != nil {
		return webapp.IncomeStatementDrillDownDetail{}, reporting.ErrInvalidDrillDown
	}
	if err := transaction.Commit(); err != nil {
		return webapp.IncomeStatementDrillDownDetail{}, fmt.Errorf("commit income statement drill-down transaction: %w", err)
	}
	return webapp.IncomeStatementDrillDownDetail{
		SchemaVersion: webapp.APISchemaVersion, SnapshotIdentity: identity, TotalEntries: total,
		NextCursor: next, IncomeStatementDrillDown: result,
	}, nil
}

func validateReportDrillDownQuery(query webapp.ReportDrillDownQuery) error {
	if query.Limit < 1 || query.Limit > 100 || len(query.SnapshotIdentity) != 64 {
		return errors.New("invalid report drill-down query")
	}
	for _, character := range query.SnapshotIdentity {
		if character < '0' || (character > '9' && character < 'a') || character > 'f' {
			return errors.New("invalid report snapshot identity")
		}
	}
	return nil
}

func sameSnapshotIdentity(left, right string) bool {
	return len(left) == len(right) && subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func paginateTrialBalanceDrillDown(result *reporting.TrialBalanceDrillDown, query webapp.ReportDrillDownQuery) (string, error) {
	selector := drillDownSelector(query, "trial-balance")
	offset, err := drillDownOffset(query, selector)
	if err != nil || offset > len(result.Entries) {
		return "", errors.New("invalid drill-down cursor")
	}
	total := len(result.Entries)
	end := min(offset+query.Limit, total)
	result.Entries = result.Entries[offset:end]
	if end < total {
		return encodeDrillDownCursor(end, selector), nil
	}
	return "", nil
}

func paginateIncomeStatementDrillDown(result *reporting.IncomeStatementDrillDown, query webapp.ReportDrillDownQuery) (string, error) {
	selector := drillDownSelector(query, "income-statement")
	offset, err := drillDownOffset(query, selector)
	if err != nil || offset > len(result.Entries) {
		return "", errors.New("invalid drill-down cursor")
	}
	total := len(result.Entries)
	end := min(offset+query.Limit, total)
	result.Entries = result.Entries[offset:end]
	if end < total {
		return encodeDrillDownCursor(end, selector), nil
	}
	return "", nil
}

func drillDownOffset(query webapp.ReportDrillDownQuery, selector string) (int, error) {
	if query.Cursor == "" {
		return 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(query.Cursor)
	if err != nil {
		return 0, err
	}
	var cursor drillDownCursor
	decoder := json.NewDecoder(strings.NewReader(string(decoded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cursor); err != nil || cursor.Offset < 1 || cursor.Selector != selector {
		return 0, errors.New("invalid drill-down cursor")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return 0, errors.New("invalid drill-down cursor")
	}
	return cursor.Offset, nil
}

func drillDownSelector(query webapp.ReportDrillDownQuery, reportKind string) string {
	projection := struct {
		ReportKind       string                   `json:"report_kind"`
		DrillDown        reporting.DrillDownQuery `json:"drill_down"`
		SnapshotIdentity string                   `json:"snapshot_identity"`
	}{ReportKind: reportKind, DrillDown: query.DrillDown, SnapshotIdentity: query.SnapshotIdentity}
	encoded, _ := json.Marshal(projection)
	return filterDigest(webapp.EntryFilter{Description: string(encoded)})
}

func encodeDrillDownCursor(offset int, selector string) string {
	encoded, _ := json.Marshal(drillDownCursor{Offset: offset, Selector: selector})
	return base64.RawURLEncoding.EncodeToString(encoded)
}
