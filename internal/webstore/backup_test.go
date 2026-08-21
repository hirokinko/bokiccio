package webstore

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/hirokinko/bokiccio/internal/reporting"
	"github.com/hirokinko/bokiccio/internal/webapp"
	_ "turso.tech/database/tursogo"
)

const testUploadEmail = "operator@example.com"

func TestBackupRestoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	sourceDB := openBackupTestDatabase(t)
	source := New(sourceDB)
	allowTestUploads(t, ctx, source)
	input, err := os.ReadFile("../ingest/testdata/mixed-outcomes-v1.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	result, err := source.Import(ctx, testUploadEmail, input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := source.GetRun(ctx, result.RunIdentity)
	if err != nil || len(run.Outcomes) != 4 {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	entryID := run.Outcomes[0].EntryID
	zero := 0
	if _, err := source.ApproveRevision(ctx, testUploadEmail, entryID, webapp.ApprovalRequest{Revision: &zero}); err != nil {
		t.Fatalf("ApproveRevision(original) error = %v", err)
	}
	invalidAmount, invalidOffset := "2.00", "-1.00"
	invalid, err := source.CreateRevision(ctx, testUploadEmail, entryID, webapp.RevisionRequest{
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
	valid, err := source.CreateRevision(ctx, testUploadEmail, entryID, webapp.RevisionRequest{
		BaseRevision: &invalid.Revision, OccurredAt: "2026-08-12T10:00:00+09:00", Description: "valid backup revision",
		Comments: []string{"valid revision"}, Postings: []webapp.PostingDetail{
			{Account: "費用:確認", Amount: &validAmount, Commodity: "UNIT"},
			{Account: "資産:確認"},
		},
	})
	if err != nil || !valid.Valid {
		t.Fatalf("valid revision=%+v error=%v", valid, err)
	}
	if _, err := source.ApproveRevision(ctx, testUploadEmail, entryID, webapp.ApprovalRequest{Revision: &valid.Revision}); err != nil {
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
	duplicate, err := target.Import(ctx, testUploadEmail, input)
	if err != nil || duplicate.Counts.Duplicate != 3 || duplicate.Counts.Error != 1 {
		t.Fatalf("restored deduplication error=%v result=%+v", err, duplicate)
	}
}

func TestBackupRestoreTotalPrice(t *testing.T) {
	ctx := context.Background()
	sourceDB := openBackupTestDatabase(t)
	source := New(sourceDB)
	allowTestUploads(t, ctx, source)
	input := []byte(`{"schema_version":2,"records":[{"source":{"namespace":"tackler","display":"uploaded.txn","external_id":"total-price-record"},"occurred_at":"2026-08-11","description":"匿名投資取引","postings":[{"account":"資産:投資信託","amount":"350","commodity":"口","total_price":{"amount":"675","commodity":"JPY"}},{"account":"資産:購入予定"}]}]}`)
	result, err := source.Import(ctx, testUploadEmail, input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := source.GetRun(ctx, result.RunIdentity)
	if err != nil || len(run.Outcomes) != 1 {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	entryID := run.Outcomes[0].EntryID
	zero := 0
	if _, err := source.ApproveRevision(ctx, testUploadEmail, entryID, webapp.ApprovalRequest{Revision: &zero}); err != nil {
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

func TestBackupRestoreReportingConfiguration(t *testing.T) {
	ctx := context.Background()
	source := New(openBackupTestDatabase(t))
	entryID := importOpeningEntry(t, ctx, source)
	zero := 0
	want, err := source.CreateReportingConfiguration(ctx, testUploadEmail, webapp.ReportingConfigurationRequest{
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
	})
	if err != nil {
		t.Fatalf("CreateReportingConfiguration() error = %v", err)
	}
	backup, err := source.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	payload, err := decodeBackup(backup)
	if err != nil || len(payload.ReportingConfigurations) != 1 || len(payload.ReportingClassifications) != 2 ||
		len(payload.ReportingFiscalYears) != 1 || len(payload.ReportingOpeningEntries) != 1 {
		t.Fatalf("reporting backup error=%v counts=%+v", err, payload.rowCounts())
	}
	target := New(openBackupTestDatabase(t))
	if err := target.Restore(ctx, backup); err != nil {
		t.Fatalf("Restore() error = %v", err)
	}
	got, err := target.GetCurrentReportingConfiguration(ctx)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("restored reporting configuration error=%v got=%+v want=%+v", err, got, want)
	}
}

func TestBackupRestoreApplicationSettings(t *testing.T) {
	ctx := context.Background()
	source := New(openBackupTestDatabase(t))
	if err := source.SetFileUploadEnabled(ctx, false); err != nil {
		t.Fatalf("SetFileUploadEnabled(false) error=%v", err)
	}
	for _, email := range []string{"operator@example.com", "backup@example.com"} {
		if err := source.AddDataWritePrincipal(ctx, email); err != nil {
			t.Fatalf("AddDataWritePrincipal(%q) error=%v", email, err)
		}
	}
	backup, err := source.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error=%v", err)
	}
	target := New(openBackupTestDatabase(t))
	if err := target.Restore(ctx, backup); err != nil {
		t.Fatalf("Restore() error=%v", err)
	}
	settings, err := target.GetApplicationSettings(ctx)
	if err != nil || settings.FileUploadEnabled {
		t.Fatalf("restored settings=%+v error=%v", settings, err)
	}
	principals, err := target.ListDataWritePrincipals(ctx)
	wantPrincipals := []string{"backup@example.com", "operator@example.com"}
	if err != nil || !reflect.DeepEqual(principals, wantPrincipals) {
		t.Fatalf("restored principals=%v error=%v want=%v", principals, err, wantPrincipals)
	}
}

func TestRestoreAcceptsLegacyBackups(t *testing.T) {
	ctx := context.Background()
	source := New(openBackupTestDatabase(t))
	allowTestUploads(t, ctx, source)
	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"backup-v2","display":"record.json","external_id":"record"},"occurred_at":"2026-08-11","description":"backup record","postings":[{"account":"資産:確認","amount":"1","commodity":"UNIT"},{"account":"負債:確認","amount":"-1","commodity":"UNIT"}]}]}`)
	if _, err := source.Import(ctx, testUploadEmail, input); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	backup, err := source.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	for _, schemaVersion := range []int{2, 3, 4, 5, 6} {
		t.Run(fmt.Sprintf("schema-v%d", schemaVersion), func(t *testing.T) {
			target := New(openBackupTestDatabase(t))
			if err := target.Restore(ctx, legacyBackup(t, backup, schemaVersion)); err != nil {
				t.Fatalf("Restore(schema v%d) error = %v", schemaVersion, err)
			}
			if _, err := target.GetCurrentReportingConfiguration(ctx); !errors.Is(err, webapp.ErrReportingNotConfigured) {
				t.Fatalf("GetCurrentReportingConfiguration() error = %v", err)
			}
			settings, err := target.GetApplicationSettings(ctx)
			if err != nil || !settings.FileUploadEnabled {
				t.Fatalf("GetApplicationSettings() settings=%+v error=%v", settings, err)
			}
			principals, err := target.ListDataWritePrincipals(ctx)
			if err != nil || len(principals) != 0 {
				t.Fatalf("ListDataWritePrincipals() principals=%v error=%v", principals, err)
			}
		})
	}
}

func TestRestoreRejectsInvalidDataWritePrincipals(t *testing.T) {
	ctx := context.Background()
	source := New(openBackupTestDatabase(t))
	backup, err := source.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error=%v", err)
	}
	payload, err := decodeBackup(backup)
	if err != nil {
		t.Fatalf("decodeBackup() error=%v", err)
	}
	for name, principals := range map[string][]dataWritePrincipalRow{
		"invalid":      {{Email: "not-an-email"}},
		"unnormalized": {{Email: "Operator@Example.com"}},
		"duplicate":    {{Email: "operator@example.com"}, {Email: "operator@example.com"}},
		"unsorted":     {{Email: "second@example.com"}, {Email: "first@example.com"}},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := payload
			candidate.DataWritePrincipals = principals
			broken, err := encodeBackup(candidate, time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatalf("encodeBackup() error=%v", err)
			}
			targetDB := openBackupTestDatabase(t)
			if err := New(targetDB).Restore(ctx, broken); !errors.Is(err, ErrInvalidBackup) {
				t.Fatalf("Restore() error=%v, want ErrInvalidBackup", err)
			}
			assertBackupTestEmpty(t, targetDB)
		})
	}
}

func TestRestoreRejectsIncompleteReportingPayload(t *testing.T) {
	ctx := context.Background()
	source := New(openBackupTestDatabase(t))
	backup, err := source.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	var envelope backupEnvelope
	if err := json.Unmarshal(backup, &envelope); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	envelope.Payload.ReportingFiscalYears = nil
	broken, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	targetDB := openBackupTestDatabase(t)
	if err := New(targetDB).Restore(ctx, broken); !errors.Is(err, ErrInvalidBackup) {
		t.Fatalf("Restore(incomplete reporting payload) error = %v", err)
	}
	assertBackupTestEmpty(t, targetDB)
}

func TestRestoreRollsBackInvalidReportingHistory(t *testing.T) {
	ctx := context.Background()
	source := New(openBackupTestDatabase(t))
	allowTestUploads(t, ctx, source)
	zero := 0
	if _, err := source.CreateReportingConfiguration(ctx, testUploadEmail,
		automaticReportingRequest(&zero, 4, "2025-04-01", "2026-03-31")); err != nil {
		t.Fatalf("CreateReportingConfiguration() error = %v", err)
	}
	backup, err := source.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}
	payload, err := decodeBackup(backup)
	if err != nil {
		t.Fatalf("decodeBackup() error = %v", err)
	}
	payload.ReportingConfigurations[0].Revision = 2
	payload.ReportingConfigurations[0].BaseRevision = 1
	for index := range payload.ReportingClassifications {
		payload.ReportingClassifications[index].Revision = 2
	}
	for index := range payload.ReportingFiscalYears {
		payload.ReportingFiscalYears[index].Revision = 2
	}
	broken, err := encodeBackup(payload, time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("encodeBackup() error = %v", err)
	}
	targetDB := openBackupTestDatabase(t)
	if err := New(targetDB).Restore(ctx, broken); err == nil {
		t.Fatal("Restore(invalid reporting history) error = nil")
	}
	assertBackupTestEmpty(t, targetDB)
}

func TestRestoreRejectsInvalidAndNonEmptyWithoutChanges(t *testing.T) {
	ctx := context.Background()
	sourceDB := openBackupTestDatabase(t)
	source := New(sourceDB)
	allowTestUploads(t, ctx, source)
	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"backup-test","display":"record.json","external_id":"backup-record"},"occurred_at":"2026-08-11","description":"backup record","postings":[{"account":"資産:確認","amount":"1","commodity":"UNIT"},{"account":"負債:確認","amount":"-1","commodity":"UNIT"}]}]}`)
	if _, err := source.Import(ctx, testUploadEmail, input); err != nil {
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
	allowTestUploads(t, ctx, source)
	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"rollback-test","display":"record.json","external_id":"rollback-record"},"occurred_at":"2026-08-11","description":"rollback record","postings":[{"account":"資産:確認","amount":"1","commodity":"UNIT"},{"account":"負債:確認","amount":"-1","commodity":"UNIT"}]}]}`)
	if _, err := source.Import(ctx, testUploadEmail, input); err != nil {
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

func allowTestUploads(t *testing.T, ctx context.Context, store *Store) {
	t.Helper()
	if err := store.AddDataWritePrincipal(ctx, testUploadEmail); err != nil {
		t.Fatalf("AddDataWritePrincipal() error=%v", err)
	}
}

func assertBackupTestEmpty(t *testing.T, database *sql.DB) {
	t.Helper()
	var generation, runs, reportingConfigurations, dataWritePrincipals int
	if err := database.QueryRow(`SELECT generation FROM workflow_state WHERE singleton = 1`).Scan(&generation); err != nil {
		t.Fatalf("read generation: %v", err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM import_runs`).Scan(&runs); err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM reporting_configurations`).Scan(&reportingConfigurations); err != nil {
		t.Fatalf("count reporting configurations: %v", err)
	}
	if err := database.QueryRow(`SELECT count(*) FROM data_write_principals`).Scan(&dataWritePrincipals); err != nil {
		t.Fatalf("count data write principals: %v", err)
	}
	if generation != 0 || runs != 0 || reportingConfigurations != 0 || dataWritePrincipals != 0 {
		t.Fatalf("target changed: generation=%d runs=%d reporting_configurations=%d data_write_principals=%d",
			generation, runs, reportingConfigurations, dataWritePrincipals)
	}
}

func legacyBackup(t *testing.T, current []byte, schemaVersion int) []byte {
	t.Helper()
	var envelope backupEnvelope
	if err := json.Unmarshal(current, &envelope); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	envelope.SchemaVersion = schemaVersion
	if schemaVersion < 4 {
		envelope.Payload.ReportingConfigurations = nil
		envelope.Payload.ReportingClassifications = nil
		envelope.Payload.ReportingFiscalYears = nil
		envelope.Payload.ReportingOpeningEntries = nil
	}
	if schemaVersion < 5 {
		envelope.Payload.ApplicationSettings = nil
	}
	if schemaVersion < 7 {
		envelope.Payload.DataWritePrincipals = nil
	}
	if schemaVersion < 4 {
		delete(envelope.RowCounts, "reporting_configurations")
		delete(envelope.RowCounts, "reporting_classifications")
		delete(envelope.RowCounts, "reporting_fiscal_years")
		delete(envelope.RowCounts, "reporting_opening_entries")
	}
	if schemaVersion < 5 {
		delete(envelope.RowCounts, "application_settings")
	}
	if schemaVersion < 7 {
		delete(envelope.RowCounts, "data_write_principals")
	}
	payloadBytes, err := marshalBackupPayload(envelope.Payload, schemaVersion)
	if err != nil {
		t.Fatalf("Marshal(payload) error = %v", err)
	}
	digest := sha256.Sum256(payloadBytes)
	legacyEnvelope := struct {
		Format        string          `json:"format"`
		FormatVersion int             `json:"format_version"`
		SchemaVersion int             `json:"schema_version"`
		CreatedAt     string          `json:"created_at"`
		PayloadSHA256 string          `json:"payload_sha256"`
		RowCounts     map[string]int  `json:"row_counts"`
		Payload       json.RawMessage `json:"payload"`
	}{
		Format: envelope.Format, FormatVersion: envelope.FormatVersion, SchemaVersion: schemaVersion,
		CreatedAt: envelope.CreatedAt, PayloadSHA256: fmt.Sprintf("%x", digest),
		RowCounts: envelope.RowCounts, Payload: payloadBytes,
	}
	encoded, err := json.Marshal(legacyEnvelope)
	if err != nil {
		t.Fatalf("Marshal(envelope) error = %v", err)
	}
	return encoded
}
