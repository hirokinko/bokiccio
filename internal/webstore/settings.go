package webstore

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hirokinko/bokiccio/internal/webapp"
)

func (store *Store) GetApplicationSettings(ctx context.Context) (webapp.ApplicationSettings, error) {
	return getApplicationSettings(ctx, store.database)
}

func (store *Store) SetFileUploadEnabled(ctx context.Context, enabled bool) (resultErr error) {
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin application settings transaction: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	result, err := transaction.ExecContext(ctx,
		`UPDATE application_settings SET file_upload_enabled = ? WHERE singleton = 1`, boolInteger(enabled))
	if err != nil {
		return fmt.Errorf("update application settings: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("inspect application settings update: %w", err)
	}
	if updated != 1 {
		return fmt.Errorf("update application settings: expected singleton row, updated %d", updated)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit application settings transaction: %w", err)
	}
	return nil
}

func getApplicationSettings(ctx context.Context, source interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) (webapp.ApplicationSettings, error) {
	var uploadEnabled int
	if err := source.QueryRowContext(ctx,
		`SELECT file_upload_enabled FROM application_settings WHERE singleton = 1`).Scan(&uploadEnabled); err != nil {
		return webapp.ApplicationSettings{}, fmt.Errorf("read application settings: %w", err)
	}
	if uploadEnabled != 0 && uploadEnabled != 1 {
		return webapp.ApplicationSettings{}, fmt.Errorf("read application settings: invalid file_upload_enabled value %d", uploadEnabled)
	}
	return webapp.ApplicationSettings{FileUploadEnabled: uploadEnabled == 1}, nil
}
