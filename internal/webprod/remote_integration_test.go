package webprod

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hirokinko/bokiccio/internal/webstore"
)

func TestRemoteTursoMigrationAndImport(t *testing.T) {
	databaseURL := os.Getenv("BOKICCIO_TEST_TURSO_DATABASE_URL")
	authToken := os.Getenv("BOKICCIO_TEST_TURSO_AUTH_TOKEN")
	if databaseURL == "" || authToken == "" {
		t.Skip("remote Turso test credentials are not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	database, err := OpenRemote(ctx, DatabaseConfig{URL: databaseURL, authToken: authToken})
	if err != nil {
		t.Fatalf("OpenRemote() error = %v", err)
	}
	defer database.Close()
	if err := webstore.Migrate(ctx, database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := webstore.CheckSchema(ctx, database); err != nil {
		t.Fatalf("CheckSchema() error = %v", err)
	}
	nonce := time.Now().UTC().Format("20060102T150405.000000000Z")
	input := []byte(fmt.Sprintf(`{"schema_version":1,"records":[{"source":{"namespace":"remote-integration","display":"%s"},"occurred_at":"2026-08-11","description":"remote integration","postings":[{"account":"資産:確認","amount":"1","commodity":"UNIT"},{"account":"負債:確認","amount":"-1","commodity":"UNIT"}]}]}`, nonce))
	store := webstore.New(database)
	result, err := store.Import(ctx, input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if _, err := store.GetRun(ctx, result.RunIdentity); err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
}
