package webstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/hirokinko/bokiccio/internal/webapp"
	_ "turso.tech/database/tursogo"
)

func TestBackupRestoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	sourceDB := openBackupTestDatabase(t)
	source := New(sourceDB)
	input, err := os.ReadFile("../ingest/testdata/mixed-outcomes-v1.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	result, err := source.Import(ctx, input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := source.GetRun(ctx, result.RunIdentity)
	if err != nil || len(run.Outcomes) != 4 {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	entryID := run.Outcomes[0].EntryID
	zero := 0
	if _, err := source.ApproveRevision(ctx, entryID, webapp.ApprovalRequest{Revision: &zero}); err != nil {
		t.Fatalf("ApproveRevision(original) error = %v", err)
	}
	invalidAmount, invalidOffset := "2.00", "-1.00"
	invalid, err := source.CreateRevision(ctx, entryID, webapp.RevisionRequest{
		BaseRevision: &zero, OccurredAt: "2026-08-12", Description: "invalid backup revision",
		Comments: []string{"invalid revision"}, Postings: []webapp.PostingDetail{
			{Account: "費用:確認", Amount: &invalidAmount, Commodity: "UNIT"},
			{Account: "資産:確認", Amount: &invalidOffset, Commodity: "UNIT"},
		},
	})
	if err != nil || invalid.Valid {
		t.Fatalf("invalid revision=%+v error=%v", invalid, err)
	}
	validAmount := "2.00"
	valid, err := source.CreateRevision(ctx, entryID, webapp.RevisionRequest{
		BaseRevision: &invalid.Revision, OccurredAt: "2026-08-12T10:00:00+09:00", Description: "valid backup revision",
		Comments: []string{"valid revision"}, Postings: []webapp.PostingDetail{
			{Account: "費用:確認", Amount: &validAmount, Commodity: "UNIT"},
			{Account: "資産:確認"},
		},
	})
	if err != nil || !valid.Valid {
		t.Fatalf("valid revision=%+v error=%v", valid, err)
	}
	if _, err := source.ApproveRevision(ctx, entryID, webapp.ApprovalRequest{Revision: &valid.Revision}); err != nil {
		t.Fatalf("ApproveRevision(valid) error = %v", err)
	}

	backup, err := source.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	sourcePayload, err := decodeBackup(backup)
	if err != nil {
		t.Fatalf("decodeBackup(source) error = %v", err)
	}
	if len(sourcePayload.ImportRuns) != 1 || len(sourcePayload.EntryRevisions) != 2 ||
		len(sourcePayload.EntryApprovals) != 2 || len(sourcePayload.RevisionDiagnostics) == 0 {
		t.Fatalf("source backup coverage = %+v", sourcePayload.rowCounts())
	}

	targetDB := openBackupTestDatabase(t)
	target := New(targetDB)
	if err := target.Restore(ctx, backup); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	restoredBackup, err := target.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup(restored) error = %v", err)
	}
	restoredPayload, err := decodeBackup(restoredBackup)
	if err != nil {
		t.Fatalf("decodeBackup(restored) error = %v", err)
	}
	if !reflect.DeepEqual(restoredPayload, sourcePayload) {
		t.Fatalf("restored payload differs\nsource=%+v\nrestored=%+v", sourcePayload.rowCounts(), restoredPayload.rowCounts())
	}
	detail, err := target.GetEntry(ctx, entryID)
	if err != nil || detail.CurrentRevision != 2 || detail.CurrentApproval == nil || len(detail.Revisions) != 2 || len(detail.Approvals) != 2 {
		t.Fatalf("restored entry error=%v detail=%+v", err, detail)
	}
	duplicate, err := target.Import(ctx, input)
	if err != nil || duplicate.Counts.Duplicate != 3 || duplicate.Counts.Error != 1 {
		t.Fatalf("restored deduplication error=%v result=%+v", err, duplicate)
	}
}

func TestBackupRestoreTotalPrice(t *testing.T) {
	ctx := context.Background()
	sourceDB := openBackupTestDatabase(t)
	source := New(sourceDB)
	input := []byte(`{"schema_version":2,"records":[{"source":{"namespace":"tackler","display":"uploaded.txn","external_id":"total-price-record"},"occurred_at":"2026-08-11","description":"匿名投資取引","postings":[{"account":"資産:投資信託","amount":"350","commodity":"口","total_price":{"amount":"675","commodity":"JPY"}},{"account":"資産:購入予定"}]}]}`)
	result, err := source.Import(ctx, input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := source.GetRun(ctx, result.RunIdentity)
	if err != nil || len(run.Outcomes) != 1 {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	entryID := run.Outcomes[0].EntryID
	zero := 0
	if _, err := source.ApproveRevision(ctx, entryID, webapp.ApprovalRequest{Revision: &zero}); err != nil {
		t.Fatalf("ApproveRevision() error = %v", err)
	}

	backup, err := source.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	payload, err := decodeBackup(backup)
	if err != nil || len(payload.Postings) != 2 || payload.Postings[0].TotalPriceAmountText == nil || *payload.Postings[0].TotalPriceAmountText != "675" {
		t.Fatalf("backup total price error=%v payload=%+v", err, payload.Postings)
	}

	target := New(openBackupTestDatabase(t))
	if err := target.Restore(ctx, backup); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	detail, err := target.GetEntry(ctx, entryID)
	if err != nil || len(detail.Postings) != 2 || detail.Postings[0].TotalPrice == nil || detail.Postings[0].TotalPrice.Amount != "675" {
		t.Fatalf("restored detail error=%v detail=%+v", err, detail)
	}
	approved, err := target.ListApprovedEntries(ctx, webapp.EntryFilter{})
	if err != nil || len(approved) != 1 || approved[0].Entry.Postings[0].TotalPrice == nil {
		t.Fatalf("restored approved entries error=%v entries=%+v", err, approved)
	}
}

func TestRestoreAcceptsSchemaV2Backup(t *testing.T) {
	ctx := context.Background()
	source := New(openBackupTestDatabase(t))
	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"backup-v2","display":"record.json","external_id":"record"},"occurred_at":"2026-08-11","description":"backup record","postings":[{"account":"資産:確認","amount":"1","commodity":"UNIT"},{"account":"負債:確認","amount":"-1","commodity":"UNIT"}]}]}`)
	if _, err := source.Import(ctx, input); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	backup, err := source.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	var envelope backupEnvelope
	if err := json.Unmarshal(backup, &envelope); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	envelope.SchemaVersion = 2
	schemaV2Backup, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	target := New(openBackupTestDatabase(t))
	if err := target.Restore(ctx, schemaV2Backup); err != nil {
		t.Fatalf("Restore(schema v2) error = %v", err)
	}
}

func TestRestoreRejectsInvalidAndNonEmptyWithoutChanges(t *testing.T) {
	ctx := context.Background()
	sourceDB := openBackupTestDatabase(t)
	source := New(sourceDB)
	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"backup-test","display":"record.json","external_id":"backup-record"},"occurred_at":"2026-08-11","description":"backup record","postings":[{"account":"資産:確認","amount":"1","commodity":"UNIT"},{"account":"負債:確認","amount":"-1","commodity":"UNIT"}]}]}`)
	if _, err := source.Import(ctx, input); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	backup, err := source.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	var envelope backupEnvelope
	if err := json.Unmarshal(backup, &envelope); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	envelope.Payload.Entries[0].Description = "tampered"
	tampered, _ := json.Marshal(envelope)
	emptyDB := openBackupTestDatabase(t)
	if err := New(emptyDB).Restore(ctx, tampered); !errors.Is(err, ErrInvalidBackup) {
		t.Fatalf("Restore(tampered) error = %v, want ErrInvalidBackup", err)
	}
	assertBackupTestEmpty(t, emptyDB)

	var futureEnvelope backupEnvelope
	if err := json.Unmarshal(backup, &futureEnvelope); err != nil {
		t.Fatalf("Unmarshal(future) error = %v", err)
	}
	futureEnvelope.SchemaVersion = SchemaVersion + 1
	future, _ := json.Marshal(futureEnvelope)
	if err := New(emptyDB).Restore(ctx, future); !errors.Is(err, ErrInvalidBackup) {
		t.Fatalf("Restore(future) error = %v, want ErrInvalidBackup", err)
	}
	assertBackupTestEmpty(t, emptyDB)

	if err := source.Restore(ctx, backup); !errors.Is(err, ErrDatabaseNotEmpty) {
		t.Fatalf("Restore(non-empty) error = %v, want ErrDatabaseNotEmpty", err)
	}
	var runCount int
	if err := sourceDB.QueryRow(`SELECT count(*) FROM import_runs`).Scan(&runCount); err != nil || runCount != 1 {
		t.Fatalf("source run count=%d error=%v", runCount, err)
	}
}

func TestRestoreRollsBackInsertFailure(t *testing.T) {
	ctx := context.Background()
	sourceDB := openBackupTestDatabase(t)
	source := New(sourceDB)
	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"rollback-test","display":"record.json","external_id":"rollback-record"},"occurred_at":"2026-08-11","description":"rollback record","postings":[{"account":"資産:確認","amount":"1","commodity":"UNIT"},{"account":"負債:確認","amount":"-1","commodity":"UNIT"}]}]}`)
	if _, err := source.Import(ctx, input); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	backup, err := source.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	payload, err := decodeBackup(backup)
	if err != nil {
		t.Fatalf("decodeBackup() error = %v", err)
	}
	payload.Postings = append(payload.Postings, payload.Postings[0])
	broken, err := encodeBackup(payload, time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("encodeBackup() error = %v", err)
	}
	targetDB := openBackupTestDatabase(t)
	if err := New(targetDB).Restore(ctx, broken); err == nil {
		t.Fatal("Restore(broken) error = nil")
	}
	assertBackupTestEmpty(t, targetDB)
}

func openBackupTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	if err := Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return database
}

func assertBackupTestEmpty(t *testing.T, database *sql.DB) {
	t.Helper()
	var generation, runs int
	if err := database.QueryRow(`SELECT generation FROM workflow_state WHERE singleton = 1`).Scan(&generation); err != nil {
		t.Fatalf("read generation: %v", err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM import_runs`).Scan(&runs); err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if generation != 0 || runs != 0 {
		t.Fatalf("target changed: generation=%d runs=%d", generation, runs)
	}
}
