package webstore

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/hirokinko/bokiccio/internal/reporting"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

func TestReportingConfigurationHistory(t *testing.T) {
	ctx := context.Background()
	database := openBackupTestDatabase(t)
	store := New(database)
	allowTestUploads(t, ctx, store)

	if _, err := store.GetCurrentReportingConfiguration(ctx); !errors.Is(err, webapp.ErrReportingNotConfigured) {
		t.Fatalf("GetCurrentReportingConfiguration() error = %v, want ErrReportingNotConfigured", err)
	}
	zero := 0
	firstRequest := automaticReportingRequest(&zero, 4, "2025-04-01", "2026-03-31")
	first, err := store.CreateReportingConfiguration(ctx, testUploadEmail, firstRequest)
	if err != nil {
		t.Fatalf("CreateReportingConfiguration(first) error = %v", err)
	}
	if first.Revision != 1 || first.BaseRevision != 0 || first.SchemaVersion != webapp.APISchemaVersion || first.CreatedAt == "" {
		t.Fatalf("first configuration = %+v", first)
	}

	one := 1
	secondRequest := automaticReportingRequest(&one, 4, "2025-04-01", "2026-03-31")
	secondRequest.Classifications = append(secondRequest.Classifications,
		webapp.ReportingClassification{Account: "Revenue", Category: reporting.CategoryRevenue})
	second, err := store.CreateReportingConfiguration(ctx, testUploadEmail, secondRequest)
	if err != nil {
		t.Fatalf("CreateReportingConfiguration(second) error = %v", err)
	}
	current, err := store.GetCurrentReportingConfiguration(ctx)
	if err != nil {
		t.Fatalf("GetCurrentReportingConfiguration() error = %v", err)
	}
	if !reflect.DeepEqual(current, second) {
		t.Fatalf("current = %+v, want %+v", current, second)
	}
	historical, err := store.GetReportingConfiguration(ctx, first.Revision)
	if err != nil {
		t.Fatalf("GetReportingConfiguration(first) error = %v", err)
	}
	if !reflect.DeepEqual(historical, first) {
		t.Fatalf("historical = %+v, want %+v", historical, first)
	}

	if _, err := store.CreateReportingConfiguration(ctx, testUploadEmail, secondRequest); !errors.Is(err, webapp.ErrConflict) {
		t.Fatalf("CreateReportingConfiguration(stale) error = %v, want ErrConflict", err)
	}
	if _, err := store.GetReportingConfiguration(ctx, 99); !errors.Is(err, webapp.ErrNotFound) {
		t.Fatalf("GetReportingConfiguration(missing) error = %v, want ErrNotFound", err)
	}
}

func TestReportingConfigurationValidatesOpeningEntryAgainstApprovedSnapshot(t *testing.T) {
	ctx := context.Background()
	store := New(openBackupTestDatabase(t))
	allowTestUploads(t, ctx, store)
	entryID := importOpeningEntry(t, ctx, store)

	zero := 0
	request := webapp.ReportingConfigurationRequest{
		BaseRevision: &zero,
		StartMonth:   4,
		Classifications: []webapp.ReportingClassification{
			{Account: "Assets", Category: reporting.CategoryAsset},
			{Account: "Liabilities", Category: reporting.CategoryLiability},
		},
		FiscalYears: []webapp.ReportingFiscalYear{{
			StartDate: "2025-04-01", EndDate: "2026-03-31", OpeningMode: reporting.OpeningEntries,
			OpeningEntryIDs: []string{entryID},
		}},
	}
	created, err := store.CreateReportingConfiguration(ctx, testUploadEmail, request)
	if err != nil {
		t.Fatalf("CreateReportingConfiguration(opening entry) error = %v", err)
	}
	if !reflect.DeepEqual(created.FiscalYears[0].OpeningEntryIDs, []string{entryID}) {
		t.Fatalf("opening entry IDs = %v", created.FiscalYears[0].OpeningEntryIDs)
	}

	one := 1
	changedCalendar := request
	changedCalendar.BaseRevision = &one
	changedCalendar.StartMonth = 1
	changedCalendar.FiscalYears = []webapp.ReportingFiscalYear{{
		StartDate: "2026-01-01", EndDate: "2026-12-31", OpeningMode: reporting.OpeningEntries,
		OpeningEntryIDs: []string{entryID},
	}}
	if _, err := store.CreateReportingConfiguration(ctx, testUploadEmail, changedCalendar); !errors.Is(err, webapp.ErrInvalidRequest) {
		t.Fatalf("CreateReportingConfiguration(calendar change) error = %v, want ErrInvalidRequest", err)
	} else {
		var configurationErr *webapp.ReportingConfigurationError
		if !errors.As(err, &configurationErr) || configurationErr.Code != webapp.ReportingOpeningEntryDateMismatch {
			t.Fatalf("calendar change error detail = %#v", err)
		}
	}
	current, err := store.GetCurrentReportingConfiguration(ctx)
	if err != nil || current.Revision != 1 {
		t.Fatalf("current after rejected change = %+v, error = %v", current, err)
	}
}

func TestReportingConfigurationRejectsUnapprovedAndTemporaryOpeningEntries(t *testing.T) {
	ctx := context.Background()
	store := New(openBackupTestDatabase(t))
	allowTestUploads(t, ctx, store)
	input, err := os.ReadFile("../ingest/testdata/valid-v1.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	result, err := store.Import(ctx, testUploadEmail, input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(ctx, result.RunIdentity)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	unapprovedID := run.Outcomes[0].EntryID

	zero := 0
	request := webapp.ReportingConfigurationRequest{
		BaseRevision: &zero,
		StartMonth:   8,
		Classifications: []webapp.ReportingClassification{
			{Account: "費用", Category: reporting.CategoryExpense},
			{Account: "資産", Category: reporting.CategoryAsset},
		},
		FiscalYears: []webapp.ReportingFiscalYear{{
			StartDate: "2026-08-01", EndDate: "2027-07-31", OpeningMode: reporting.OpeningEntries,
			OpeningEntryIDs: []string{unapprovedID},
		}},
	}
	if _, err := store.CreateReportingConfiguration(ctx, testUploadEmail, request); !errors.Is(err, webapp.ErrInvalidRequest) {
		t.Fatalf("CreateReportingConfiguration(unapproved) error = %v, want ErrInvalidRequest", err)
	} else {
		var configurationErr *webapp.ReportingConfigurationError
		if !errors.As(err, &configurationErr) || configurationErr.Code != webapp.ReportingOpeningEntryNotApproved {
			t.Fatalf("unapproved error detail = %#v", err)
		}
	}

	amount := "207.00"
	revision, err := store.CreateRevision(ctx, testUploadEmail, unapprovedID, webapp.RevisionRequest{
		BaseRevision: &zero, OccurredAt: "2026-08-01", Description: "anonymous temporary opening",
		Postings: []webapp.PostingDetail{
			{Account: "費用:食費", Amount: &amount, Commodity: "JPY"},
			{Account: "資産:現金"},
		},
	})
	if err != nil || !revision.Valid {
		t.Fatalf("CreateRevision() revision=%+v error=%v", revision, err)
	}
	if _, err := store.ApproveRevision(ctx, testUploadEmail, unapprovedID, webapp.ApprovalRequest{Revision: &revision.Revision}); err != nil {
		t.Fatalf("ApproveRevision() error = %v", err)
	}
	if _, err := store.CreateReportingConfiguration(ctx, testUploadEmail, request); !errors.Is(err, webapp.ErrInvalidRequest) {
		t.Fatalf("CreateReportingConfiguration(temporary account) error = %v, want ErrInvalidRequest", err)
	} else {
		var configurationErr *webapp.ReportingConfigurationError
		if !errors.As(err, &configurationErr) || configurationErr.Code != webapp.ReportingOpeningEntryTemporaryAccount {
			t.Fatalf("temporary account error detail = %#v", err)
		}
	}
}

func automaticReportingRequest(baseRevision *int, startMonth int, startDate, endDate string) webapp.ReportingConfigurationRequest {
	return webapp.ReportingConfigurationRequest{
		BaseRevision: baseRevision,
		StartMonth:   startMonth,
		Classifications: []webapp.ReportingClassification{
			{Account: "Assets", Category: reporting.CategoryAsset},
			{Account: "Liabilities", Category: reporting.CategoryLiability},
		},
		FiscalYears: []webapp.ReportingFiscalYear{{
			StartDate: startDate, EndDate: endDate, OpeningMode: reporting.OpeningAutomatic,
			OpeningEntryIDs: []string{},
		}},
	}
}

func importOpeningEntry(t *testing.T, ctx context.Context, store *Store) string {
	t.Helper()
	allowTestUploads(t, ctx, store)
	input, err := os.ReadFile("../ingest/testdata/valid-v1.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	result, err := store.Import(ctx, testUploadEmail, input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(ctx, result.RunIdentity)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	entryID := run.Outcomes[0].EntryID
	zero := 0
	amount := "100"
	revision, err := store.CreateRevision(ctx, testUploadEmail, entryID, webapp.RevisionRequest{
		BaseRevision: &zero, OccurredAt: "2025-04-01", Description: "anonymous opening balance",
		Postings: []webapp.PostingDetail{
			{Account: "Assets:Cash", Amount: &amount, Commodity: "JPY"},
			{Account: "Liabilities:Opening"},
		},
	})
	if err != nil || !revision.Valid {
		t.Fatalf("CreateRevision(opening) = %+v, error = %v", revision, err)
	}
	if _, err := store.ApproveRevision(ctx, testUploadEmail, entryID, webapp.ApprovalRequest{Revision: &revision.Revision}); err != nil {
		t.Fatalf("ApproveRevision(opening) error = %v", err)
	}
	return entryID
}
