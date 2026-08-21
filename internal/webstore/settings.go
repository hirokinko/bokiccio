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

func (store *Store) GetUserAccess(ctx context.Context, email string) (webapp.UserAccess, error) {
	return getUserAccess(ctx, store.database, email)
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

func (store *Store) AddDataWritePrincipal(ctx context.Context, email string) (resultErr error) {
	normalized, err := webapp.NormalizeEmail(email)
	if err != nil {
		return err
	}
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin data write principal transaction: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	if _, err := transaction.ExecContext(ctx,
		`INSERT OR IGNORE INTO data_write_principals (email) VALUES (?)`, normalized); err != nil {
		return fmt.Errorf("add data write principal: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit data write principal transaction: %w", err)
	}
	return nil
}

func (store *Store) RemoveDataWritePrincipal(ctx context.Context, email string) (resultErr error) {
	normalized, err := webapp.NormalizeEmail(email)
	if err != nil {
		return err
	}
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin data write principal transaction: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	if _, err := transaction.ExecContext(ctx,
		`DELETE FROM data_write_principals WHERE email = ?`, normalized); err != nil {
		return fmt.Errorf("remove data write principal: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit data write principal transaction: %w", err)
	}
	return nil
}

func (store *Store) ListDataWritePrincipals(ctx context.Context) ([]string, error) {
	return listDataWritePrincipals(ctx, store.database)
}

func (store *Store) IsDataWritePrincipal(ctx context.Context, email string) (bool, error) {
	normalized, err := webapp.NormalizeEmail(email)
	if err != nil {
		return false, err
	}
	return isDataWritePrincipal(ctx, store.database, normalized)
}

func listDataWritePrincipals(ctx context.Context, source interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) ([]string, error) {
	rows, err := source.QueryContext(ctx, `SELECT email FROM data_write_principals ORDER BY email`)
	if err != nil {
		return nil, fmt.Errorf("list data write principals: %w", err)
	}
	defer rows.Close()
	principals := []string{}
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, fmt.Errorf("read data write principal: %w", err)
		}
		normalized, err := webapp.NormalizeEmail(email)
		if err != nil || normalized != email {
			return nil, fmt.Errorf("read data write principal: invalid email")
		}
		principals = append(principals, email)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate data write principals: %w", err)
	}
	return principals, nil
}

func isDataWritePrincipal(ctx context.Context, source interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, normalizedEmail string) (bool, error) {
	var allowed int
	if err := source.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM data_write_principals WHERE email = ?)`, normalizedEmail).Scan(&allowed); err != nil {
		return false, fmt.Errorf("read data write principal membership: %w", err)
	}
	if allowed != 0 && allowed != 1 {
		return false, fmt.Errorf("read data write principal membership: invalid value %d", allowed)
	}
	return allowed == 1, nil
}

func getUserAccess(ctx context.Context, source interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, email string) (webapp.UserAccess, error) {
	settings, err := getApplicationSettings(ctx, source)
	if err != nil {
		return webapp.UserAccess{}, err
	}
	access := webapp.UserAccess{FileUploadEnabled: settings.FileUploadEnabled}
	normalized, err := webapp.NormalizeEmail(email)
	if err != nil {
		return access, nil
	}
	access.CanWrite, err = isDataWritePrincipal(ctx, source, normalized)
	if err != nil {
		return webapp.UserAccess{}, err
	}
	return access, nil
}

func requireWriteAccess(ctx context.Context, source interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, email string) error {
	normalized, err := webapp.NormalizeEmail(email)
	if err != nil {
		return webapp.ErrWriteForbidden
	}
	allowed, err := isDataWritePrincipal(ctx, source, normalized)
	if err != nil {
		return err
	}
	if !allowed {
		return webapp.ErrWriteForbidden
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
