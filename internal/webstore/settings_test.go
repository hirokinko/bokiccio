package webstore

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/hirokinko/bokiccio/internal/webapp"
)

func TestApplicationSettingsDefaultAndImportGate(t *testing.T) {
	ctx := context.Background()
	database := openBackupTestDatabase(t)
	store := New(database)

	settings, err := store.GetApplicationSettings(ctx)
	if err != nil || !settings.FileUploadEnabled {
		t.Fatalf("GetApplicationSettings() settings=%+v error=%v", settings, err)
	}
	access, err := store.GetUserAccess(ctx, testUploadEmail)
	if err != nil || !access.FileUploadEnabled || access.CanWrite {
		t.Fatalf("GetUserAccess(unlisted) access=%+v error=%v", access, err)
	}
	if err := store.SetFileUploadEnabled(ctx, false); err != nil {
		t.Fatalf("SetFileUploadEnabled(false) error=%v", err)
	}
	access, err = store.GetUserAccess(ctx, "not-an-email")
	if err != nil || access.FileUploadEnabled || access.CanWrite {
		t.Fatalf("GetUserAccess(disabled) access=%+v error=%v", access, err)
	}
	settings, err = store.GetApplicationSettings(ctx)
	if err != nil || settings.FileUploadEnabled {
		t.Fatalf("GetApplicationSettings(disabled) settings=%+v error=%v", settings, err)
	}
	input := []byte(`{"schema_version":1,"records":[]}`)
	if _, err := store.Import(ctx, testUploadEmail, input); !errors.Is(err, webapp.ErrUploadDisabled) {
		t.Fatalf("Import(disabled) error=%v, want ErrUploadDisabled", err)
	}
	var runs int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM import_runs`).Scan(&runs); err != nil || runs != 0 {
		t.Fatalf("import_runs=%d error=%v, want 0", runs, err)
	}
	if err := store.SetFileUploadEnabled(ctx, true); err != nil {
		t.Fatalf("SetFileUploadEnabled(true) error=%v", err)
	}
	if _, err := store.Import(ctx, testUploadEmail, input); !errors.Is(err, webapp.ErrUploadForbidden) {
		t.Fatalf("Import(unlisted) error=%v, want ErrUploadForbidden", err)
	}
	if err := store.AddDataWritePrincipal(ctx, testUploadEmail); err != nil {
		t.Fatalf("AddDataWritePrincipal() error=%v", err)
	}
	access, err = store.GetUserAccess(ctx, " OPERATOR@EXAMPLE.COM ")
	if err != nil || !access.FileUploadEnabled || !access.CanWrite {
		t.Fatalf("GetUserAccess(allowed) access=%+v error=%v", access, err)
	}
	if _, err := store.Import(ctx, testUploadEmail, input); err != nil {
		t.Fatalf("Import(enabled) error=%v", err)
	}
	if err := store.SetFileUploadEnabled(ctx, false); err != nil {
		t.Fatalf("SetFileUploadEnabled(false after allow) error=%v", err)
	}
	zero := 0
	if _, err := store.CreateReportingConfiguration(ctx, testUploadEmail,
		automaticReportingRequest(&zero, 4, "2025-04-01", "2026-03-31")); err != nil {
		t.Fatalf("CreateReportingConfiguration(upload disabled) error=%v", err)
	}
}

func TestApplicationSettingsInvalidValueIsAnError(t *testing.T) {
	database, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error=%v", err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.Exec(`CREATE TABLE application_settings (singleton INTEGER PRIMARY KEY, file_upload_enabled INTEGER NOT NULL)`); err != nil {
		t.Fatalf("create application settings: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO application_settings (singleton, file_upload_enabled) VALUES (1, 2)`); err != nil {
		t.Fatalf("insert invalid application setting: %v", err)
	}
	if _, err := New(database).GetApplicationSettings(context.Background()); err == nil {
		t.Fatal("GetApplicationSettings() error=nil")
	}
}

func TestApplicationSettingsMissingRowIsAnError(t *testing.T) {
	ctx := context.Background()
	database := openBackupTestDatabase(t)
	store := New(database)
	if _, err := database.ExecContext(ctx, `DELETE FROM application_settings`); err != nil {
		t.Fatalf("delete application settings: %v", err)
	}
	if _, err := store.GetApplicationSettings(ctx); err == nil {
		t.Fatal("GetApplicationSettings() error=nil")
	}
	if _, err := store.GetUserAccess(ctx, testUploadEmail); err == nil {
		t.Fatal("GetUserAccess() error=nil")
	}
	if err := store.SetFileUploadEnabled(ctx, false); err == nil {
		t.Fatal("SetFileUploadEnabled() error=nil")
	}
	if _, err := store.Import(ctx, testUploadEmail, []byte(`{"schema_version":1,"records":[]}`)); err == nil {
		t.Fatal("Import() error=nil")
	}
}

func TestDataWritePrincipals(t *testing.T) {
	ctx := context.Background()
	store := New(openBackupTestDatabase(t))

	principals, err := store.ListDataWritePrincipals(ctx)
	if err != nil || len(principals) != 0 {
		t.Fatalf("ListDataWritePrincipals(empty) = %v, %v", principals, err)
	}
	for _, email := range []string{" Beta@Example.COM ", "alpha@example.com", "beta@example.com"} {
		if err := store.AddDataWritePrincipal(ctx, email); err != nil {
			t.Fatalf("AddDataWritePrincipal(%q) error=%v", email, err)
		}
	}
	principals, err = store.ListDataWritePrincipals(ctx)
	want := []string{"alpha@example.com", "beta@example.com"}
	if err != nil || !reflect.DeepEqual(principals, want) {
		t.Fatalf("ListDataWritePrincipals() = %v, %v, want %v", principals, err, want)
	}
	allowed, err := store.IsDataWritePrincipal(ctx, " ALPHA@EXAMPLE.COM ")
	if err != nil || !allowed {
		t.Fatalf("IsDataWritePrincipal(allowed) = %t, %v", allowed, err)
	}
	allowed, err = store.IsDataWritePrincipal(ctx, "viewer@example.com")
	if err != nil || allowed {
		t.Fatalf("IsDataWritePrincipal(viewer) = %t, %v", allowed, err)
	}
	for range 2 {
		if err := store.RemoveDataWritePrincipal(ctx, "ALPHA@example.com"); err != nil {
			t.Fatalf("RemoveDataWritePrincipal() error=%v", err)
		}
	}
	principals, err = store.ListDataWritePrincipals(ctx)
	if err != nil || !reflect.DeepEqual(principals, []string{"beta@example.com"}) {
		t.Fatalf("ListDataWritePrincipals(after remove) = %v, %v", principals, err)
	}
}

func TestDataWritePrincipalsRejectInvalidEmail(t *testing.T) {
	ctx := context.Background()
	store := New(openBackupTestDatabase(t))
	for _, email := range []string{"", "not-an-email", "Operator <operator@example.com>"} {
		if err := store.AddDataWritePrincipal(ctx, email); !errors.Is(err, webapp.ErrInvalidEmail) {
			t.Fatalf("AddDataWritePrincipal(%q) error=%v", email, err)
		}
		if err := store.RemoveDataWritePrincipal(ctx, email); !errors.Is(err, webapp.ErrInvalidEmail) {
			t.Fatalf("RemoveDataWritePrincipal(%q) error=%v", email, err)
		}
		if allowed, err := store.IsDataWritePrincipal(ctx, email); !errors.Is(err, webapp.ErrInvalidEmail) || allowed {
			t.Fatalf("IsDataWritePrincipal(%q) = %t, %v", email, allowed, err)
		}
	}
}

func TestDataMutationsRequireWritePrincipal(t *testing.T) {
	ctx := context.Background()
	database := openBackupTestDatabase(t)
	store := New(database)
	allowTestUploads(t, ctx, store)
	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"authorization","display":"anonymous.json"},"occurred_at":"2026-08-21","description":"Authorization fixture","postings":[{"account":"Assets:Cash","amount":"1","commodity":"JPY"},{"account":"Equity:Opening","amount":"-1","commodity":"JPY"}]}]}`)
	result, err := store.Import(ctx, testUploadEmail, input)
	if err != nil {
		t.Fatalf("Import() error=%v", err)
	}
	run, err := store.GetRun(ctx, result.RunIdentity)
	if err != nil || len(run.Outcomes) != 1 {
		t.Fatalf("GetRun() run=%+v error=%v", run, err)
	}
	if err := store.RemoveDataWritePrincipal(ctx, testUploadEmail); err != nil {
		t.Fatalf("RemoveDataWritePrincipal() error=%v", err)
	}
	zero := 0
	amount := "2"
	if _, err := store.CreateRevision(ctx, testUploadEmail, run.Outcomes[0].EntryID, webapp.RevisionRequest{
		BaseRevision: &zero, OccurredAt: "2026-08-21", Description: "Forbidden revision",
		Postings: []webapp.PostingDetail{
			{Account: "Assets:Cash", Amount: &amount, Commodity: "JPY"},
			{Account: "Equity:Opening"},
		},
	}); !errors.Is(err, webapp.ErrWriteForbidden) {
		t.Fatalf("CreateRevision() error=%v, want ErrWriteForbidden", err)
	}
	if _, err := store.ApproveRevision(ctx, testUploadEmail, run.Outcomes[0].EntryID,
		webapp.ApprovalRequest{Revision: &zero}); !errors.Is(err, webapp.ErrWriteForbidden) {
		t.Fatalf("ApproveRevision() error=%v, want ErrWriteForbidden", err)
	}
	if _, err := store.CreateReportingConfiguration(ctx, testUploadEmail,
		automaticReportingRequest(&zero, 4, "2025-04-01", "2026-03-31")); !errors.Is(err, webapp.ErrWriteForbidden) {
		t.Fatalf("CreateReportingConfiguration() error=%v, want ErrWriteForbidden", err)
	}
	var revisions, approvals, configurations int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM entry_revisions`).Scan(&revisions); err != nil {
		t.Fatalf("count entry revisions: %v", err)
	}
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM entry_approvals`).Scan(&approvals); err != nil {
		t.Fatalf("count entry approvals: %v", err)
	}
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM reporting_configurations`).Scan(&configurations); err != nil {
		t.Fatalf("count reporting configurations: %v", err)
	}
	if revisions != 0 || approvals != 0 || configurations != 0 {
		t.Fatalf("forbidden mutations changed rows: revisions=%d approvals=%d configurations=%d", revisions, approvals, configurations)
	}
}

func TestMigrationV7ReplacesFileUploadAllowlist(t *testing.T) {
	ctx := context.Background()
	database, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error=%v", err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	if _, err := database.ExecContext(ctx, `CREATE TABLE schema_metadata (
        singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
        version INTEGER NOT NULL CHECK (version >= 0)
    )`); err != nil {
		t.Fatalf("create schema metadata: %v", err)
	}
	if _, err := database.ExecContext(ctx, `INSERT INTO schema_metadata (singleton, version) VALUES (1, 6)`); err != nil {
		t.Fatalf("insert schema metadata: %v", err)
	}
	if _, err := database.ExecContext(ctx, `CREATE TABLE file_upload_principals (
        email TEXT NOT NULL PRIMARY KEY CHECK (email <> '' AND email = trim(email) AND email = lower(email))
    )`); err != nil {
		t.Fatalf("create v6 file upload principals: %v", err)
	}
	if err := Migrate(ctx, database); err != nil {
		t.Fatalf("Migrate(v6) error=%v", err)
	}
	var version, principals int
	if err := database.QueryRowContext(ctx, `SELECT version FROM schema_metadata WHERE singleton = 1`).Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM data_write_principals`).Scan(&principals); err != nil {
		t.Fatalf("count data write principals: %v", err)
	}
	if version != 7 || principals != 0 {
		t.Fatalf("migration result version=%d principals=%d", version, principals)
	}
	var legacyTables int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_schema WHERE type = 'table' AND name = 'file_upload_principals'`).Scan(&legacyTables); err != nil {
		t.Fatalf("count legacy file upload tables: %v", err)
	}
	if legacyTables != 0 {
		t.Fatalf("legacy file upload tables=%d, want 0", legacyTables)
	}
}
