package webstore

import (
	"context"
	"database/sql"
	"errors"
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
	if err := store.SetFileUploadEnabled(ctx, false); err != nil {
		t.Fatalf("SetFileUploadEnabled(false) error=%v", err)
	}
	settings, err = store.GetApplicationSettings(ctx)
	if err != nil || settings.FileUploadEnabled {
		t.Fatalf("GetApplicationSettings(disabled) settings=%+v error=%v", settings, err)
	}
	input := []byte(`{"schema_version":1,"records":[]}`)
	if _, err := store.Import(ctx, input); !errors.Is(err, webapp.ErrUploadDisabled) {
		t.Fatalf("Import(disabled) error=%v, want ErrUploadDisabled", err)
	}
	var runs int
	if err := database.QueryRowContext(ctx, `SELECT count(*) FROM import_runs`).Scan(&runs); err != nil || runs != 0 {
		t.Fatalf("import_runs=%d error=%v, want 0", runs, err)
	}
	if err := store.SetFileUploadEnabled(ctx, true); err != nil {
		t.Fatalf("SetFileUploadEnabled(true) error=%v", err)
	}
	if _, err := store.Import(ctx, input); err != nil {
		t.Fatalf("Import(enabled) error=%v", err)
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
	if err := store.SetFileUploadEnabled(ctx, false); err == nil {
		t.Fatal("SetFileUploadEnabled() error=nil")
	}
	if _, err := store.Import(ctx, []byte(`{"schema_version":1,"records":[]}`)); err == nil {
		t.Fatal("Import() error=nil")
	}
}
