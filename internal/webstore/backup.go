package webstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

const (
	BackupFormat        = "bokiccio-logical-backup"
	BackupFormatVersion = 1
)

var (
	ErrInvalidBackup    = errors.New("invalid Bokiccio backup")
	ErrDatabaseNotEmpty = errors.New("restore database is not empty")
)

type backupEnvelope struct {
	Format        string         `json:"format"`
	FormatVersion int            `json:"format_version"`
	SchemaVersion int            `json:"schema_version"`
	CreatedAt     string         `json:"created_at"`
	PayloadSHA256 string         `json:"payload_sha256"`
	RowCounts     map[string]int `json:"row_counts"`
	Payload       backupPayload  `json:"payload"`
}

type backupPayload struct {
	WorkflowState       []workflowStateRow      `json:"workflow_state"`
	CommittedIdentities []committedIdentityRow  `json:"committed_identities"`
	ImportRuns          []importRunRow          `json:"import_runs"`
	Outcomes            []outcomeRow            `json:"outcomes"`
	Diagnostics         []diagnosticRow         `json:"diagnostics"`
	Entries             []entryRow              `json:"entries"`
	EntryComments       []entryCommentRow       `json:"entry_comments"`
	Postings            []postingRow            `json:"postings"`
	EntryRevisions      []entryRevisionRow      `json:"entry_revisions"`
	RevisionComments    []revisionCommentRow    `json:"revision_comments"`
	RevisionPostings    []revisionPostingRow    `json:"revision_postings"`
	RevisionDiagnostics []revisionDiagnosticRow `json:"revision_diagnostics"`
	EntryApprovals      []entryApprovalRow      `json:"entry_approvals"`
	Sequences           []sequenceRow           `json:"sequences"`
}

type workflowStateRow struct {
	Singleton  int   `json:"singleton"`
	Generation int64 `json:"generation"`
}

type committedIdentityRow struct {
	Kind             string `json:"kind"`
	AlgorithmVersion int    `json:"algorithm_version"`
	Digest           string `json:"digest"`
}

type importRunRow struct {
	Sequence           int64  `json:"sequence"`
	RunID              string `json:"run_id"`
	AlgorithmVersion   int    `json:"algorithm_version"`
	InputDigest        string `json:"input_digest"`
	PreStateGeneration int64  `json:"pre_state_generation"`
	HasErrors          int    `json:"has_errors"`
	ReportJSON         []byte `json:"report_json"`
	Journal            []byte `json:"journal,omitempty"`
}

type outcomeRow struct {
	RunID                    string `json:"run_id"`
	RecordIndex              int    `json:"record_index"`
	Status                   string `json:"status"`
	SourceNamespace          string `json:"source_namespace"`
	SourceDisplay            string `json:"source_display"`
	IdentityKind             string `json:"identity_kind"`
	IdentityAlgorithmVersion int    `json:"identity_algorithm_version"`
	IdentityDigest           string `json:"identity_digest"`
}

type diagnosticRow struct {
	RunID           string  `json:"run_id"`
	RecordIndex     int     `json:"record_index"`
	DiagnosticIndex int     `json:"diagnostic_index"`
	Code            string  `json:"code"`
	Severity        string  `json:"severity"`
	Message         string  `json:"message"`
	FieldPath       *string `json:"field_path,omitempty"`
	PostingIndex    *int64  `json:"posting_index,omitempty"`
}

type entryRow struct {
	EntryID           string `json:"entry_id"`
	RunID             string `json:"run_id"`
	RecordIndex       int    `json:"record_index"`
	OccurredPrecision int    `json:"occurred_precision"`
	OccurredAt        string `json:"occurred_at"`
	Description       string `json:"description"`
}

type entryCommentRow struct {
	EntryID      string `json:"entry_id"`
	CommentIndex int    `json:"comment_index"`
	Comment      string `json:"comment"`
}

type postingRow struct {
	EntryID      string  `json:"entry_id"`
	PostingIndex int     `json:"posting_index"`
	Account      string  `json:"account"`
	AmountText   *string `json:"amount_text,omitempty"`
	AmountScale  *int64  `json:"amount_scale,omitempty"`
	Commodity    *string `json:"commodity,omitempty"`
	Comment      string  `json:"comment"`
}

type entryRevisionRow struct {
	EntryID           string `json:"entry_id"`
	Revision          int    `json:"revision"`
	BaseRevision      int    `json:"base_revision"`
	CreatedAt         string `json:"created_at"`
	OccurredPrecision int    `json:"occurred_precision"`
	OccurredAt        string `json:"occurred_at"`
	Description       string `json:"description"`
	Valid             int    `json:"valid"`
}

type revisionCommentRow struct {
	EntryID      string `json:"entry_id"`
	Revision     int    `json:"revision"`
	CommentIndex int    `json:"comment_index"`
	Comment      string `json:"comment"`
}

type revisionPostingRow struct {
	EntryID      string  `json:"entry_id"`
	Revision     int     `json:"revision"`
	PostingIndex int     `json:"posting_index"`
	Account      string  `json:"account"`
	AmountText   *string `json:"amount_text,omitempty"`
	AmountScale  *int64  `json:"amount_scale,omitempty"`
	Commodity    *string `json:"commodity,omitempty"`
	Comment      string  `json:"comment"`
}

type revisionDiagnosticRow struct {
	EntryID         string  `json:"entry_id"`
	Revision        int     `json:"revision"`
	DiagnosticIndex int     `json:"diagnostic_index"`
	Code            string  `json:"code"`
	Severity        string  `json:"severity"`
	Message         string  `json:"message"`
	FieldPath       *string `json:"field_path,omitempty"`
	PostingIndex    *int64  `json:"posting_index,omitempty"`
}

type entryApprovalRow struct {
	ApprovalSequence int64  `json:"approval_sequence"`
	EntryID          string `json:"entry_id"`
	Revision         int    `json:"revision"`
	ApprovedAt       string `json:"approved_at"`
}

type sequenceRow struct {
	Name string `json:"name"`
	Seq  int64  `json:"seq"`
}

func (store *Store) Backup(ctx context.Context) (_ []byte, resultErr error) {
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin backup transaction: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	if err := requireSchemaVersion(ctx, transaction); err != nil {
		return nil, err
	}
	payload, err := readBackupPayload(ctx, transaction)
	if err != nil {
		return nil, err
	}
	if err := validateBackupPayloadShape(payload); err != nil {
		return nil, fmt.Errorf("validate backup payload: %w", err)
	}
	if err := validateDatabaseContents(ctx, transaction); err != nil {
		return nil, fmt.Errorf("validate backup source: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return nil, fmt.Errorf("commit backup transaction: %w", err)
	}
	return encodeBackup(payload, time.Now().UTC())
}

func encodeBackup(payload backupPayload, createdAt time.Time) ([]byte, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode backup payload: %w", err)
	}
	digest := sha256.Sum256(payloadBytes)
	envelope := backupEnvelope{
		Format: BackupFormat, FormatVersion: BackupFormatVersion, SchemaVersion: SchemaVersion,
		CreatedAt: createdAt.Format(time.RFC3339Nano), PayloadSHA256: hex.EncodeToString(digest[:]),
		RowCounts: payload.rowCounts(), Payload: payload,
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("encode backup envelope: %w", err)
	}
	return append(encoded, '\n'), nil
}

func decodeBackup(input []byte) (backupPayload, error) {
	var envelope backupEnvelope
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return backupPayload{}, ErrInvalidBackup
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return backupPayload{}, ErrInvalidBackup
	}
	if envelope.Format != BackupFormat || envelope.FormatVersion != BackupFormatVersion ||
		envelope.SchemaVersion != SchemaVersion || !envelope.Payload.complete() {
		return backupPayload{}, ErrInvalidBackup
	}
	if err := validateBackupPayloadShape(envelope.Payload); err != nil {
		return backupPayload{}, ErrInvalidBackup
	}
	createdAt, err := time.Parse(time.RFC3339Nano, envelope.CreatedAt)
	if err != nil || createdAt.Format(time.RFC3339Nano) != envelope.CreatedAt {
		return backupPayload{}, ErrInvalidBackup
	}
	payloadBytes, err := json.Marshal(envelope.Payload)
	if err != nil {
		return backupPayload{}, ErrInvalidBackup
	}
	digest := sha256.Sum256(payloadBytes)
	if envelope.PayloadSHA256 != hex.EncodeToString(digest[:]) ||
		!equalRowCounts(envelope.RowCounts, envelope.Payload.rowCounts()) {
		return backupPayload{}, ErrInvalidBackup
	}
	return envelope.Payload, nil
}

func (payload backupPayload) rowCounts() map[string]int {
	return map[string]int{
		"workflow_state": len(payload.WorkflowState), "committed_identities": len(payload.CommittedIdentities),
		"import_runs": len(payload.ImportRuns), "outcomes": len(payload.Outcomes),
		"diagnostics": len(payload.Diagnostics), "entries": len(payload.Entries),
		"entry_comments": len(payload.EntryComments), "postings": len(payload.Postings),
		"entry_revisions": len(payload.EntryRevisions), "revision_comments": len(payload.RevisionComments),
		"revision_postings": len(payload.RevisionPostings), "revision_diagnostics": len(payload.RevisionDiagnostics),
		"entry_approvals": len(payload.EntryApprovals), "sequences": len(payload.Sequences),
	}
}

func equalRowCounts(left, right map[string]int) bool {
	if len(left) != len(right) {
		return false
	}
	for name, count := range right {
		actual, found := left[name]
		if !found || actual != count {
			return false
		}
	}
	return true
}

func (payload backupPayload) complete() bool {
	return payload.WorkflowState != nil && payload.CommittedIdentities != nil && payload.ImportRuns != nil &&
		payload.Outcomes != nil && payload.Diagnostics != nil && payload.Entries != nil &&
		payload.EntryComments != nil && payload.Postings != nil && payload.EntryRevisions != nil &&
		payload.RevisionComments != nil && payload.RevisionPostings != nil &&
		payload.RevisionDiagnostics != nil && payload.EntryApprovals != nil && payload.Sequences != nil
}

func validateBackupPayloadShape(payload backupPayload) error {
	if len(payload.WorkflowState) != 1 || payload.WorkflowState[0].Singleton != 1 ||
		payload.WorkflowState[0].Generation < 0 {
		return ErrInvalidBackup
	}
	maximum := map[string]int64{"import_runs": 0, "entry_approvals": 0}
	for _, row := range payload.ImportRuns {
		if row.Sequence > maximum["import_runs"] {
			maximum["import_runs"] = row.Sequence
		}
	}
	for _, row := range payload.EntryApprovals {
		if row.ApprovalSequence > maximum["entry_approvals"] {
			maximum["entry_approvals"] = row.ApprovalSequence
		}
	}
	seen := map[string]bool{}
	for _, row := range payload.Sequences {
		if _, allowed := maximum[row.Name]; !allowed || seen[row.Name] || row.Seq < maximum[row.Name] {
			return ErrInvalidBackup
		}
		seen[row.Name] = true
	}
	for name, value := range maximum {
		if value > 0 && !seen[name] {
			return ErrInvalidBackup
		}
	}
	return nil
}

func readBackupPayload(ctx context.Context, source queryer) (backupPayload, error) {
	var payload backupPayload
	var err error
	payload.WorkflowState, err = readRows(ctx, source, `SELECT singleton, generation FROM workflow_state ORDER BY singleton`,
		func(rows *sql.Rows, row *workflowStateRow) error { return rows.Scan(&row.Singleton, &row.Generation) })
	if err != nil {
		return backupPayload{}, backupReadError("workflow_state", err)
	}
	payload.CommittedIdentities, err = readRows(ctx, source, `SELECT kind, algorithm_version, digest FROM committed_identities ORDER BY kind, algorithm_version, digest`,
		func(rows *sql.Rows, row *committedIdentityRow) error {
			return rows.Scan(&row.Kind, &row.AlgorithmVersion, &row.Digest)
		})
	if err != nil {
		return backupPayload{}, backupReadError("committed_identities", err)
	}
	payload.ImportRuns, err = readRows(ctx, source, `SELECT sequence, run_id, algorithm_version, input_digest, pre_state_generation, has_errors, report_json, journal FROM import_runs ORDER BY sequence`,
		func(rows *sql.Rows, row *importRunRow) error {
			return rows.Scan(&row.Sequence, &row.RunID, &row.AlgorithmVersion, &row.InputDigest, &row.PreStateGeneration, &row.HasErrors, &row.ReportJSON, &row.Journal)
		})
	if err != nil {
		return backupPayload{}, backupReadError("import_runs", err)
	}
	payload.Outcomes, err = readRows(ctx, source, `SELECT run_id, record_index, status, source_namespace, source_display, identity_kind, identity_algorithm_version, identity_digest FROM outcomes ORDER BY run_id, record_index`,
		func(rows *sql.Rows, row *outcomeRow) error {
			return rows.Scan(&row.RunID, &row.RecordIndex, &row.Status, &row.SourceNamespace, &row.SourceDisplay, &row.IdentityKind, &row.IdentityAlgorithmVersion, &row.IdentityDigest)
		})
	if err != nil {
		return backupPayload{}, backupReadError("outcomes", err)
	}
	payload.Diagnostics, err = readRows(ctx, source, `SELECT run_id, record_index, diagnostic_index, code, severity, message, field_path, posting_index FROM diagnostics ORDER BY run_id, record_index, diagnostic_index`,
		func(rows *sql.Rows, row *diagnosticRow) error {
			return rows.Scan(&row.RunID, &row.RecordIndex, &row.DiagnosticIndex, &row.Code, &row.Severity, &row.Message, &row.FieldPath, &row.PostingIndex)
		})
	if err != nil {
		return backupPayload{}, backupReadError("diagnostics", err)
	}
	payload.Entries, err = readRows(ctx, source, `SELECT entry_id, run_id, record_index, occurred_precision, occurred_at, description FROM entries ORDER BY entry_id`,
		func(rows *sql.Rows, row *entryRow) error {
			return rows.Scan(&row.EntryID, &row.RunID, &row.RecordIndex, &row.OccurredPrecision, &row.OccurredAt, &row.Description)
		})
	if err != nil {
		return backupPayload{}, backupReadError("entries", err)
	}
	payload.EntryComments, err = readRows(ctx, source, `SELECT entry_id, comment_index, comment FROM entry_comments ORDER BY entry_id, comment_index`,
		func(rows *sql.Rows, row *entryCommentRow) error {
			return rows.Scan(&row.EntryID, &row.CommentIndex, &row.Comment)
		})
	if err != nil {
		return backupPayload{}, backupReadError("entry_comments", err)
	}
	payload.Postings, err = readRows(ctx, source, `SELECT entry_id, posting_index, account, amount_text, amount_scale, commodity, comment FROM postings ORDER BY entry_id, posting_index`,
		func(rows *sql.Rows, row *postingRow) error {
			return rows.Scan(&row.EntryID, &row.PostingIndex, &row.Account, &row.AmountText, &row.AmountScale, &row.Commodity, &row.Comment)
		})
	if err != nil {
		return backupPayload{}, backupReadError("postings", err)
	}
	payload.EntryRevisions, err = readRows(ctx, source, `SELECT entry_id, revision, base_revision, created_at, occurred_precision, occurred_at, description, valid FROM entry_revisions ORDER BY entry_id, revision`,
		func(rows *sql.Rows, row *entryRevisionRow) error {
			return rows.Scan(&row.EntryID, &row.Revision, &row.BaseRevision, &row.CreatedAt, &row.OccurredPrecision, &row.OccurredAt, &row.Description, &row.Valid)
		})
	if err != nil {
		return backupPayload{}, backupReadError("entry_revisions", err)
	}
	payload.RevisionComments, err = readRows(ctx, source, `SELECT entry_id, revision, comment_index, comment FROM revision_comments ORDER BY entry_id, revision, comment_index`,
		func(rows *sql.Rows, row *revisionCommentRow) error {
			return rows.Scan(&row.EntryID, &row.Revision, &row.CommentIndex, &row.Comment)
		})
	if err != nil {
		return backupPayload{}, backupReadError("revision_comments", err)
	}
	payload.RevisionPostings, err = readRows(ctx, source, `SELECT entry_id, revision, posting_index, account, amount_text, amount_scale, commodity, comment FROM revision_postings ORDER BY entry_id, revision, posting_index`,
		func(rows *sql.Rows, row *revisionPostingRow) error {
			return rows.Scan(&row.EntryID, &row.Revision, &row.PostingIndex, &row.Account, &row.AmountText, &row.AmountScale, &row.Commodity, &row.Comment)
		})
	if err != nil {
		return backupPayload{}, backupReadError("revision_postings", err)
	}
	payload.RevisionDiagnostics, err = readRows(ctx, source, `SELECT entry_id, revision, diagnostic_index, code, severity, message, field_path, posting_index FROM revision_diagnostics ORDER BY entry_id, revision, diagnostic_index`,
		func(rows *sql.Rows, row *revisionDiagnosticRow) error {
			return rows.Scan(&row.EntryID, &row.Revision, &row.DiagnosticIndex, &row.Code, &row.Severity, &row.Message, &row.FieldPath, &row.PostingIndex)
		})
	if err != nil {
		return backupPayload{}, backupReadError("revision_diagnostics", err)
	}
	payload.EntryApprovals, err = readRows(ctx, source, `SELECT approval_sequence, entry_id, revision, approved_at FROM entry_approvals ORDER BY approval_sequence`,
		func(rows *sql.Rows, row *entryApprovalRow) error {
			return rows.Scan(&row.ApprovalSequence, &row.EntryID, &row.Revision, &row.ApprovedAt)
		})
	if err != nil {
		return backupPayload{}, backupReadError("entry_approvals", err)
	}
	payload.Sequences, err = readRows(ctx, source, `SELECT name, seq FROM sqlite_sequence WHERE name IN ('import_runs', 'entry_approvals') ORDER BY name`,
		func(rows *sql.Rows, row *sequenceRow) error { return rows.Scan(&row.Name, &row.Seq) })
	if err != nil {
		return backupPayload{}, backupReadError("sequences", err)
	}
	return payload, nil
}

func readRows[T any](ctx context.Context, source queryer, statement string, scan func(*sql.Rows, *T) error) ([]T, error) {
	rows, err := source.QueryContext(ctx, statement)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []T{}
	for rows.Next() {
		var row T
		if err := scan(rows, &row); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func backupReadError(table string, err error) error {
	return fmt.Errorf("read backup table %s: %w", table, err)
}

func requireSchemaVersion(ctx context.Context, source interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}) error {
	var version int
	if err := source.QueryRowContext(ctx, `SELECT version FROM schema_metadata WHERE singleton = 1`).Scan(&version); err != nil {
		return fmt.Errorf("read backup schema version: %w", err)
	}
	if version != SchemaVersion {
		return fmt.Errorf("%w: database version %d, required version %d", ErrUnsupportedSchema, version, SchemaVersion)
	}
	return nil
}
