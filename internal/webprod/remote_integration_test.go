package webprod

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hirokinko/bokiccio/internal/webapp"
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
	input := []byte(fmt.Sprintf(`{"schema_version":1,"records":[{"source":{"namespace":"remote-integration","display":"%[1]s"},"occurred_at":"2026-08-11","description":"remote integration %[1]s","postings":[{"account":"資産:確認","amount":"1","commodity":"UNIT"},{"account":"負債:確認","amount":"-1","commodity":"UNIT"}]}]}`, nonce))
	store := webstore.New(database)
	result, err := store.Import(ctx, input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(ctx, result.RunIdentity)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if len(run.Outcomes) != 1 || run.Outcomes[0].EntryID == "" {
		t.Fatalf("imported run has no entry: %+v", run)
	}
	baseRevision := 0
	amount := "2.00"
	revision, err := store.CreateRevision(ctx, run.Outcomes[0].EntryID, webapp.RevisionRequest{
		BaseRevision: &baseRevision,
		OccurredAt:   "2026-08-11T12:00:00+09:00",
		Description:  "remote integration revision",
		Comments:     []string{"remote integration"},
		Postings: []webapp.PostingDetail{
			{Account: "資産:確認", Amount: &amount, Commodity: "UNIT"},
			{Account: "負債:確認"},
		},
	})
	if err != nil || !revision.Valid {
		t.Fatalf("CreateRevision() revision=%+v error=%v", revision, err)
	}
	if _, err := store.ApproveRevision(ctx, run.Outcomes[0].EntryID, webapp.ApprovalRequest{Revision: &revision.Revision}); err != nil {
		t.Fatalf("ApproveRevision() error = %v", err)
	}
}
