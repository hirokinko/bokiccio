package webstore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hirokinko/bokiccio/internal/ingest"
	"github.com/hirokinko/bokiccio/internal/ledger"
)

func (store *Store) Restore(ctx context.Context, input []byte) (resultErr error) {
	payload, err := decodeBackup(input)
	if err != nil {
		return err
	}
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin restore transaction: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	if err := requireSchemaVersion(ctx, transaction); err != nil {
		return err
	}
	if err := requireEmptyDatabase(ctx, transaction); err != nil {
		return err
	}
	if err := insertBackupPayload(ctx, transaction, payload); err != nil {
		return err
	}
	if err := verifyRestoredCounts(ctx, transaction, payload.rowCounts()); err != nil {
		return err
	}
	if err := validateDatabaseContents(ctx, transaction); err != nil {
		return fmt.Errorf("validate restored data: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit restore transaction: %w", err)
	}
	return nil
}

func requireEmptyDatabase(ctx context.Context, transaction *sql.Tx) error {
	var generation, rows, sequences int64
	if err := transaction.QueryRowContext(ctx, `SELECT generation FROM workflow_state WHERE singleton = 1`).Scan(&generation); err != nil {
		return fmt.Errorf("read restore workflow state: %w", err)
	}
	if err := transaction.QueryRowContext(ctx, `SELECT
        (SELECT count(*) FROM committed_identities) +
        (SELECT count(*) FROM import_runs) +
        (SELECT count(*) FROM outcomes) +
        (SELECT count(*) FROM diagnostics) +
        (SELECT count(*) FROM entries) +
        (SELECT count(*) FROM entry_comments) +
        (SELECT count(*) FROM postings) +
        (SELECT count(*) FROM entry_revisions) +
        (SELECT count(*) FROM revision_comments) +
        (SELECT count(*) FROM revision_postings) +
        (SELECT count(*) FROM revision_diagnostics) +
        (SELECT count(*) FROM entry_approvals)`).Scan(&rows); err != nil {
		return fmt.Errorf("inspect restore target: %w", err)
	}
	if err := transaction.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_sequence WHERE name IN ('import_runs', 'entry_approvals')`).Scan(&sequences); err != nil {
		return fmt.Errorf("inspect restore sequences: %w", err)
	}
	if generation != 0 || rows != 0 || sequences != 0 {
		return ErrDatabaseNotEmpty
	}
	return nil
}

func insertBackupPayload(ctx context.Context, transaction *sql.Tx, payload backupPayload) error {
	if _, err := transaction.ExecContext(ctx, `UPDATE workflow_state SET generation = ? WHERE singleton = 1`,
		payload.WorkflowState[0].Generation); err != nil {
		return restoreInsertError("workflow_state", err)
	}
	for _, row := range payload.CommittedIdentities {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO committed_identities (kind, algorithm_version, digest) VALUES (?, ?, ?)`, row.Kind, row.AlgorithmVersion, row.Digest); err != nil {
			return restoreInsertError("committed_identities", err)
		}
	}
	for _, row := range payload.ImportRuns {
		var journal any
		if row.Journal != nil {
			journal = row.Journal
		}
		if _, err := transaction.ExecContext(ctx, `INSERT INTO import_runs (sequence, run_id, algorithm_version, input_digest, pre_state_generation, has_errors, report_json, journal) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, row.Sequence, row.RunID, row.AlgorithmVersion, row.InputDigest, row.PreStateGeneration, row.HasErrors, row.ReportJSON, journal); err != nil {
			return restoreInsertError("import_runs", err)
		}
	}
	for _, row := range payload.Outcomes {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO outcomes (run_id, record_index, status, source_namespace, source_display, identity_kind, identity_algorithm_version, identity_digest) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, row.RunID, row.RecordIndex, row.Status, row.SourceNamespace, row.SourceDisplay, row.IdentityKind, row.IdentityAlgorithmVersion, row.IdentityDigest); err != nil {
			return restoreInsertError("outcomes", err)
		}
	}
	for _, row := range payload.Diagnostics {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO diagnostics (run_id, record_index, diagnostic_index, code, severity, message, field_path, posting_index) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, row.RunID, row.RecordIndex, row.DiagnosticIndex, row.Code, row.Severity, row.Message, row.FieldPath, row.PostingIndex); err != nil {
			return restoreInsertError("diagnostics", err)
		}
	}
	for _, row := range payload.Entries {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO entries (entry_id, run_id, record_index, occurred_precision, occurred_at, description) VALUES (?, ?, ?, ?, ?, ?)`, row.EntryID, row.RunID, row.RecordIndex, row.OccurredPrecision, row.OccurredAt, row.Description); err != nil {
			return restoreInsertError("entries", err)
		}
	}
	for _, row := range payload.EntryComments {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO entry_comments (entry_id, comment_index, comment) VALUES (?, ?, ?)`, row.EntryID, row.CommentIndex, row.Comment); err != nil {
			return restoreInsertError("entry_comments", err)
		}
	}
	for _, row := range payload.Postings {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO postings (entry_id, posting_index, account, amount_text, amount_scale, commodity, comment) VALUES (?, ?, ?, ?, ?, ?, ?)`, row.EntryID, row.PostingIndex, row.Account, row.AmountText, row.AmountScale, row.Commodity, row.Comment); err != nil {
			return restoreInsertError("postings", err)
		}
	}
	for _, row := range payload.EntryRevisions {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO entry_revisions (entry_id, revision, base_revision, created_at, occurred_precision, occurred_at, description, valid) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, row.EntryID, row.Revision, row.BaseRevision, row.CreatedAt, row.OccurredPrecision, row.OccurredAt, row.Description, row.Valid); err != nil {
			return restoreInsertError("entry_revisions", err)
		}
	}
	for _, row := range payload.RevisionComments {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO revision_comments (entry_id, revision, comment_index, comment) VALUES (?, ?, ?, ?)`, row.EntryID, row.Revision, row.CommentIndex, row.Comment); err != nil {
			return restoreInsertError("revision_comments", err)
		}
	}
	for _, row := range payload.RevisionPostings {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO revision_postings (entry_id, revision, posting_index, account, amount_text, amount_scale, commodity, comment) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, row.EntryID, row.Revision, row.PostingIndex, row.Account, row.AmountText, row.AmountScale, row.Commodity, row.Comment); err != nil {
			return restoreInsertError("revision_postings", err)
		}
	}
	for _, row := range payload.RevisionDiagnostics {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO revision_diagnostics (entry_id, revision, diagnostic_index, code, severity, message, field_path, posting_index) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, row.EntryID, row.Revision, row.DiagnosticIndex, row.Code, row.Severity, row.Message, row.FieldPath, row.PostingIndex); err != nil {
			return restoreInsertError("revision_diagnostics", err)
		}
	}
	for _, row := range payload.EntryApprovals {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO entry_approvals (approval_sequence, entry_id, revision, approved_at) VALUES (?, ?, ?, ?)`, row.ApprovalSequence, row.EntryID, row.Revision, row.ApprovedAt); err != nil {
			return restoreInsertError("entry_approvals", err)
		}
	}
	if err := restoreSequences(ctx, transaction, payload.Sequences); err != nil {
		return err
	}
	return nil
}

func restoreSequences(ctx context.Context, transaction *sql.Tx, sequences []sequenceRow) error {
	seen := map[string]bool{}
	for _, row := range sequences {
		if (row.Name != "import_runs" && row.Name != "entry_approvals") || row.Seq < 0 || seen[row.Name] {
			return ErrInvalidBackup
		}
		seen[row.Name] = true
		var maximum int64
		column := "sequence"
		if row.Name == "entry_approvals" {
			column = "approval_sequence"
		}
		if err := transaction.QueryRowContext(ctx, `SELECT COALESCE(MAX(`+column+`), 0) FROM `+row.Name).Scan(&maximum); err != nil {
			return restoreInsertError("sequences", err)
		}
		if row.Seq < maximum {
			return ErrInvalidBackup
		}
	}
	if _, err := transaction.ExecContext(ctx, `DELETE FROM sqlite_sequence WHERE name IN ('import_runs', 'entry_approvals')`); err != nil {
		return restoreInsertError("sequences", err)
	}
	for _, row := range sequences {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO sqlite_sequence (name, seq) VALUES (?, ?)`, row.Name, row.Seq); err != nil {
			return restoreInsertError("sequences", err)
		}
	}
	return nil
}

func verifyRestoredCounts(ctx context.Context, transaction *sql.Tx, want map[string]int) error {
	for _, table := range []string{"workflow_state", "committed_identities", "import_runs", "outcomes", "diagnostics", "entries", "entry_comments", "postings", "entry_revisions", "revision_comments", "revision_postings", "revision_diagnostics", "entry_approvals"} {
		var count int
		if err := transaction.QueryRowContext(ctx, `SELECT count(*) FROM `+table).Scan(&count); err != nil {
			return fmt.Errorf("count restored table %s: %w", table, err)
		}
		if count != want[table] {
			return ErrInvalidBackup
		}
	}
	var sequenceCount int
	if err := transaction.QueryRowContext(ctx, `SELECT count(*) FROM sqlite_sequence WHERE name IN ('import_runs', 'entry_approvals')`).Scan(&sequenceCount); err != nil {
		return fmt.Errorf("count restored sequences: %w", err)
	}
	if sequenceCount != want["sequences"] {
		return ErrInvalidBackup
	}
	return nil
}

func validateDatabaseContents(ctx context.Context, source rowQueryer) error {
	if transaction, ok := source.(*sql.Tx); ok {
		if _, err := loadState(ctx, transaction); err != nil {
			return err
		}
	}
	foreignKeys, err := source.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("check foreign keys: %w", err)
	}
	if foreignKeys.Next() {
		foreignKeys.Close()
		return errors.New("foreign key validation failed")
	}
	if err := foreignKeys.Err(); err != nil {
		foreignKeys.Close()
		return fmt.Errorf("iterate foreign key check: %w", err)
	}
	if err := foreignKeys.Close(); err != nil {
		return fmt.Errorf("close foreign key check: %w", err)
	}

	reports, err := source.QueryContext(ctx, `SELECT run_id, algorithm_version, input_digest,
        pre_state_generation, report_json FROM import_runs ORDER BY sequence`)
	if err != nil {
		return fmt.Errorf("query reports: %w", err)
	}
	for reports.Next() {
		var runID, inputDigest string
		var algorithmVersion int
		var preStateGeneration uint64
		var reportBytes []byte
		if err := reports.Scan(&runID, &algorithmVersion, &inputDigest, &preStateGeneration, &reportBytes); err != nil || !json.Valid(reportBytes) {
			reports.Close()
			return errors.New("stored report is invalid")
		}
		report, err := ingest.DecodeReport(bytes.NewReader(reportBytes))
		if err != nil || report.RunIdentity.HexDigest() != runID ||
			report.RunIdentity.AlgorithmVersion != algorithmVersion ||
			fmt.Sprintf("%x", report.InputDigest) != inputDigest || report.PreStateGeneration != preStateGeneration {
			reports.Close()
			return errors.New("stored report metadata is inconsistent")
		}
	}
	if err := reports.Err(); err != nil {
		reports.Close()
		return fmt.Errorf("iterate reports: %w", err)
	}
	if err := reports.Close(); err != nil {
		return fmt.Errorf("close reports: %w", err)
	}

	entries, err := source.QueryContext(ctx, `SELECT entry_id FROM entries ORDER BY entry_id`)
	if err != nil {
		return fmt.Errorf("query restored entries: %w", err)
	}
	entryIDs := []string{}
	for entries.Next() {
		var id string
		if err := entries.Scan(&id); err != nil {
			entries.Close()
			return err
		}
		entryIDs = append(entryIDs, id)
	}
	if err := entries.Err(); err != nil {
		entries.Close()
		return err
	}
	if err := entries.Close(); err != nil {
		return err
	}
	for _, id := range entryIDs {
		if _, err := loadEntrySnapshot(ctx, source, id, 0); err != nil {
			return err
		}
	}

	revisions, err := source.QueryContext(ctx, `SELECT entry_id, revision, valid, occurred_precision, occurred_at, description FROM entry_revisions ORDER BY entry_id, revision`)
	if err != nil {
		return fmt.Errorf("query restored revisions: %w", err)
	}
	type revisionState struct {
		id, occurredAt, description string
		revision, valid, precision  int
	}
	states := []revisionState{}
	for revisions.Next() {
		var state revisionState
		if err := revisions.Scan(&state.id, &state.revision, &state.valid, &state.precision, &state.occurredAt, &state.description); err != nil {
			revisions.Close()
			return err
		}
		states = append(states, state)
	}
	if err := revisions.Err(); err != nil {
		revisions.Close()
		return err
	}
	if err := revisions.Close(); err != nil {
		return err
	}
	for _, state := range states {
		if state.valid == 1 {
			if _, err := loadEntrySnapshot(ctx, source, state.id, state.revision); err != nil {
				return err
			}
			continue
		}
		entryTime, err := ledger.ParseEntryTime(state.occurredAt)
		if err != nil || int(entryTime.Precision()) != state.precision {
			return errors.New("stored invalid revision time is invalid")
		}
		postings, err := loadRevisionPostings(ctx, source, state.id, state.revision)
		if err != nil {
			return err
		}
		ledgerPostings, err := postingDetailsToLedger(postings)
		if err != nil {
			return err
		}
		if err := ledger.Validate(ledger.JournalEntry{Date: entryTime, Description: state.description, Postings: ledgerPostings}); err == nil {
			return errors.New("stored invalid revision validates successfully")
		}
	}

	for _, statement := range []string{
		`SELECT 1 FROM entries WHERE entry_id != run_id || ':' || record_index LIMIT 1`,
		`SELECT 1 FROM entry_revisions WHERE base_revision != revision - 1 LIMIT 1`,
		`SELECT 1 FROM entry_approvals a LEFT JOIN entry_revisions r ON r.entry_id = a.entry_id AND r.revision = a.revision WHERE a.revision > 0 AND (r.revision IS NULL OR r.valid != 1) LIMIT 1`,
	} {
		var violation int
		err := source.QueryRowContext(ctx, statement).Scan(&violation)
		if err == nil {
			return errors.New("stored relationship is invalid")
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	timestamps, err := source.QueryContext(ctx, `SELECT created_at FROM entry_revisions
        UNION ALL SELECT approved_at FROM entry_approvals`)
	if err != nil {
		return fmt.Errorf("query stored timestamps: %w", err)
	}
	for timestamps.Next() {
		var text string
		if err := timestamps.Scan(&text); err != nil {
			timestamps.Close()
			return err
		}
		if parsed, err := time.Parse(time.RFC3339Nano, text); err != nil || parsed.Format(time.RFC3339Nano) != text {
			timestamps.Close()
			return errors.New("stored timestamp is invalid")
		}
	}
	if err := timestamps.Err(); err != nil {
		timestamps.Close()
		return err
	}
	if err := timestamps.Close(); err != nil {
		return err
	}
	return nil
}

func restoreInsertError(table string, err error) error {
	return fmt.Errorf("restore table %s: %w", table, err)
}
