package webprod

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/tursodatabase/libsql-client-go/libsql"
)

func OpenRemote(ctx context.Context, config DatabaseConfig) (*sql.DB, error) {
	if err := validateDatabaseURL(config.URL); err != nil || config.authToken == "" {
		return nil, fmt.Errorf("invalid remote database configuration")
	}
	connector, err := libsql.NewConnector(config.URL, libsql.WithAuthToken(config.authToken))
	if err != nil {
		return nil, fmt.Errorf("configure remote database connection: %w", err)
	}
	database := sql.OpenDB(connector)
	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("connect to remote database: %w", err)
	}
	return database, nil
}
