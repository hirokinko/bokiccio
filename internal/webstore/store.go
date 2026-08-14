// Package webstore persists the single-user Web workflow through database/sql.
package webstore

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hirokinko/bokiccio/internal/ingest"
	"github.com/hirokinko/bokiccio/internal/ledger"
	"github.com/hirokinko/bokiccio/internal/tacklerfmt"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

type Store struct {
	database *sql.DB
}

func New(database *sql.DB) *Store {
	return &Store{database: database}
}

func (store *Store) Import(ctx context.Context, input []byte) (_ webapp.ImportResult, resultErr error) {
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return webapp.ImportResult{}, fmt.Errorf("begin import transaction: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	settings, err := getApplicationSettings(ctx, transaction)
	if err != nil {
		return webapp.ImportResult{}, err
	}
	if !settings.FileUploadEnabled {
		return webapp.ImportResult{}, webapp.ErrUploadDisabled
	}
	state, err := loadState(ctx, transaction)
	if err != nil {
		return webapp.ImportResult{}, err
	}
	artifact, err := ingest.Build(input, state, tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted})
	if err != nil {
		return webapp.ImportResult{}, err
	}
	if err := saveArtifact(ctx, transaction, artifact); err != nil {
		return webapp.ImportResult{}, err
	}
	if artifact.NextState.Generation != state.Generation {
		for _, identity := range artifact.NextState.Identities {
			if _, err := transaction.ExecContext(ctx,
				`INSERT OR IGNORE INTO committed_identities (kind, algorithm_version, digest) VALUES (?, ?, ?)`,
				identity.Kind, identity.AlgorithmVersion, identity.HexDigest()); err != nil {
				return webapp.ImportResult{}, fmt.Errorf("save committed identity: %w", err)
			}
		}
		result, err := transaction.ExecContext(ctx,
			`UPDATE workflow_state SET generation = ? WHERE singleton = 1 AND generation = ?`,
			artifact.NextState.Generation, state.Generation)
		if err != nil {
			return webapp.ImportResult{}, fmt.Errorf("advance workflow state: %w", err)
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return webapp.ImportResult{}, fmt.Errorf("inspect workflow state update: %w", err)
		}
		if updated != 1 {
			return webapp.ImportResult{}, webapp.ErrConflict
		}
	}
	if err := transaction.Commit(); err != nil {
		return webapp.ImportResult{}, fmt.Errorf("commit import transaction: %w", err)
	}
	counts := webapp.OutcomeCounts{}
	for _, outcome := range artifact.Report.Outcomes {
		switch outcome.Status {
		case ingest.OutcomeSuccess:
			counts.Success++
		case ingest.OutcomeWarning:
			counts.Warning++
		case ingest.OutcomeError:
			counts.Error++
		case ingest.OutcomeDuplicate:
			counts.Duplicate++
		}
	}
	return webapp.ImportResult{RunIdentity: artifact.RunIdentity.HexDigest(), HasErrors: artifact.HasErrors, Counts: counts}, nil
}

func loadState(ctx context.Context, transaction *sql.Tx) (ingest.State, error) {
	state := ingest.EmptyState()
	if err := transaction.QueryRowContext(ctx, `SELECT generation FROM workflow_state WHERE singleton = 1`).Scan(&state.Generation); err != nil {
		return ingest.State{}, fmt.Errorf("read workflow state: %w", err)
	}
	rows, err := transaction.QueryContext(ctx,
		`SELECT kind, algorithm_version, digest FROM committed_identities ORDER BY kind, algorithm_version, digest`)
	if err != nil {
		return ingest.State{}, fmt.Errorf("read committed identities: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind string
		var algorithm int
		var digest string
		if err := rows.Scan(&kind, &algorithm, &digest); err != nil {
			return ingest.State{}, fmt.Errorf("scan committed identity: %w", err)
		}
		identity, err := decodeIdentity(kind, algorithm, digest)
		if err != nil {
			return ingest.State{}, err
		}
		state.Identities = append(state.Identities, identity)
	}
	if err := rows.Err(); err != nil {
		return ingest.State{}, fmt.Errorf("iterate committed identities: %w", err)
	}
	return state, nil
}

func saveArtifact(ctx context.Context, transaction *sql.Tx, artifact ingest.Artifact) error {
	var journal any
	if len(artifact.Journal) > 0 {
		journal = artifact.Journal
	}
	if _, err := transaction.ExecContext(ctx, `INSERT INTO import_runs
        (run_id, algorithm_version, input_digest, pre_state_generation, has_errors, report_json, journal)
        VALUES (?, ?, ?, ?, ?, ?, ?)`,
		artifact.RunIdentity.HexDigest(), artifact.RunIdentity.AlgorithmVersion,
		fmt.Sprintf("%x", artifact.Report.InputDigest), artifact.Report.PreStateGeneration,
		boolInteger(artifact.HasErrors), artifact.ReportBytes, journal); err != nil {
		return fmt.Errorf("save import run: %w", err)
	}
	for _, outcome := range artifact.Report.Outcomes {
		if _, err := transaction.ExecContext(ctx, `INSERT INTO outcomes
            (run_id, record_index, status, source_namespace, source_display,
             identity_kind, identity_algorithm_version, identity_digest)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			artifact.RunIdentity.HexDigest(), outcome.RecordIndex, outcome.Status,
			outcome.Source.Namespace, outcome.Source.Display, outcome.Identity.Kind,
			outcome.Identity.AlgorithmVersion, outcome.Identity.HexDigest()); err != nil {
			return fmt.Errorf("save outcome %d: %w", outcome.RecordIndex, err)
		}
		for index, diagnostic := range outcome.Diagnostics {
			var postingIndex any
			if diagnostic.PostingIndex != nil {
				postingIndex = *diagnostic.PostingIndex
			}
			if _, err := transaction.ExecContext(ctx, `INSERT INTO diagnostics
                (run_id, record_index, diagnostic_index, code, severity, message, field_path, posting_index)
                VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?)`,
				artifact.RunIdentity.HexDigest(), outcome.RecordIndex, index, diagnostic.Code,
				diagnostic.Severity, diagnostic.Message, diagnostic.FieldPath, postingIndex); err != nil {
				return fmt.Errorf("save diagnostic %d/%d: %w", outcome.RecordIndex, index, err)
			}
		}
		if outcome.Entry != nil {
			if err := saveEntry(ctx, transaction, artifact.RunIdentity.HexDigest(), outcome.RecordIndex, *outcome.Entry); err != nil {
				return err
			}
		}
	}
	return nil
}

func saveEntry(ctx context.Context, transaction *sql.Tx, runID string, recordIndex int, entry ledger.JournalEntry) error {
	entryID := entryIdentifier(runID, recordIndex)
	if _, err := transaction.ExecContext(ctx, `INSERT INTO entries
        (entry_id, run_id, record_index, occurred_precision, occurred_at, description)
        VALUES (?, ?, ?, ?, ?, ?)`, entryID, runID, recordIndex, entry.Date.Precision(), entry.Date.String(), entry.Description); err != nil {
		return fmt.Errorf("save entry %d: %w", recordIndex, err)
	}
	for index, comment := range entry.Comments {
		if _, err := transaction.ExecContext(ctx,
			`INSERT INTO entry_comments (entry_id, comment_index, comment) VALUES (?, ?, ?)`,
			entryID, index, comment); err != nil {
			return fmt.Errorf("save entry comment %d/%d: %w", recordIndex, index, err)
		}
	}
	for index, posting := range entry.Postings {
		var amountText, amountScale, commodity any
		if posting.Amount != nil {
			amountText = posting.Amount.Value.String()
			amountScale = posting.Amount.Value.Scale()
			commodity = posting.Amount.Commodity
		}
		var totalPriceText, totalPriceScale, totalPriceCommodity any
		if posting.TotalPrice != nil {
			totalPriceText = posting.TotalPrice.Value.String()
			totalPriceScale = posting.TotalPrice.Value.Scale()
			totalPriceCommodity = posting.TotalPrice.Commodity
		}
		if _, err := transaction.ExecContext(ctx, `INSERT INTO postings
            (entry_id, posting_index, account, amount_text, amount_scale, commodity,
             total_price_amount_text, total_price_amount_scale, total_price_commodity, comment)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			entryID, index, posting.Account, amountText, amountScale, commodity,
			totalPriceText, totalPriceScale, totalPriceCommodity, posting.Comment); err != nil {
			return fmt.Errorf("save posting %d/%d: %w", recordIndex, index, err)
		}
	}
	return nil
}

func (store *Store) GetRun(ctx context.Context, runID string) (webapp.RunDetail, error) {
	detail := webapp.RunDetail{SchemaVersion: webapp.APISchemaVersion, RunIdentity: runID, Outcomes: []webapp.OutcomeDetail{}}
	var hasErrors int
	if err := store.database.QueryRowContext(ctx, `SELECT input_digest, pre_state_generation, has_errors
        FROM import_runs WHERE run_id = ?`, runID).Scan(&detail.InputDigest, &detail.PreStateGeneration, &hasErrors); err != nil {
		return webapp.RunDetail{}, notFound(err)
	}
	detail.HasErrors = hasErrors != 0
	rows, err := store.database.QueryContext(ctx, `SELECT o.record_index, o.status, o.source_namespace,
        o.source_display, o.identity_kind, o.identity_algorithm_version, o.identity_digest, e.entry_id
        FROM outcomes o LEFT JOIN entries e ON e.run_id = o.run_id AND e.record_index = o.record_index
        WHERE o.run_id = ? ORDER BY o.record_index`, runID)
	if err != nil {
		return webapp.RunDetail{}, fmt.Errorf("query outcomes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var outcome webapp.OutcomeDetail
		var entryID sql.NullString
		if err := rows.Scan(&outcome.RecordIndex, &outcome.Status, &outcome.Source.Namespace,
			&outcome.Source.Display, &outcome.Identity.Kind, &outcome.Identity.AlgorithmVersion,
			&outcome.Identity.Digest, &entryID); err != nil {
			return webapp.RunDetail{}, fmt.Errorf("scan outcome: %w", err)
		}
		outcome.EntryID = entryID.String
		detail.Outcomes = append(detail.Outcomes, outcome)
	}
	if err := rows.Err(); err != nil {
		return webapp.RunDetail{}, fmt.Errorf("iterate outcomes: %w", err)
	}
	if err := rows.Close(); err != nil {
		return webapp.RunDetail{}, fmt.Errorf("close outcomes: %w", err)
	}
	for index := range detail.Outcomes {
		detail.Outcomes[index].Diagnostics, err = loadDiagnostics(ctx, store.database, runID, detail.Outcomes[index].RecordIndex)
		if err != nil {
			return webapp.RunDetail{}, err
		}
	}
	return detail, nil
}

func (store *Store) GetEntry(ctx context.Context, entryID string) (webapp.EntryDetail, error) {
	detail := webapp.EntryDetail{SchemaVersion: webapp.APISchemaVersion, ID: entryID, Comments: []string{}, Postings: []webapp.PostingDetail{}, Diagnostics: []webapp.DiagnosticDetail{}}
	var precision int
	if err := store.database.QueryRowContext(ctx, `SELECT e.run_id, e.record_index, e.occurred_precision,
        e.occurred_at, e.description, o.status, o.source_namespace, o.source_display
        FROM entries e JOIN outcomes o ON o.run_id = e.run_id AND o.record_index = e.record_index
        WHERE e.entry_id = ?`, entryID).Scan(&detail.RunIdentity, &detail.RecordIndex, &precision,
		&detail.OccurredAt, &detail.Description, &detail.Status, &detail.Source.Namespace, &detail.Source.Display); err != nil {
		return webapp.EntryDetail{}, notFound(err)
	}
	entryTime, err := ledger.ParseEntryTime(detail.OccurredAt)
	if err != nil || int(entryTime.Precision()) != precision {
		return webapp.EntryDetail{}, errors.New("stored entry time is invalid")
	}
	comments, err := store.database.QueryContext(ctx,
		`SELECT comment FROM entry_comments WHERE entry_id = ? ORDER BY comment_index`, entryID)
	if err != nil {
		return webapp.EntryDetail{}, fmt.Errorf("query entry comments: %w", err)
	}
	for comments.Next() {
		var comment string
		if err := comments.Scan(&comment); err != nil {
			comments.Close()
			return webapp.EntryDetail{}, fmt.Errorf("scan entry comment: %w", err)
		}
		detail.Comments = append(detail.Comments, comment)
	}
	if err := comments.Close(); err != nil {
		return webapp.EntryDetail{}, fmt.Errorf("close entry comments: %w", err)
	}
	postings, err := store.database.QueryContext(ctx, `SELECT account, amount_text, amount_scale, commodity,
        total_price_amount_text, total_price_amount_scale, total_price_commodity, comment
        FROM postings WHERE entry_id = ? ORDER BY posting_index`, entryID)
	if err != nil {
		return webapp.EntryDetail{}, fmt.Errorf("query postings: %w", err)
	}
	for postings.Next() {
		var posting webapp.PostingDetail
		var amountText, commodity, totalPriceText, totalPriceCommodity sql.NullString
		var amountScale, totalPriceScale sql.NullInt64
		if err := postings.Scan(&posting.Account, &amountText, &amountScale, &commodity,
			&totalPriceText, &totalPriceScale, &totalPriceCommodity, &posting.Comment); err != nil {
			postings.Close()
			return webapp.EntryDetail{}, fmt.Errorf("scan posting: %w", err)
		}
		if amountText.Valid != amountScale.Valid || amountText.Valid != commodity.Valid {
			postings.Close()
			return webapp.EntryDetail{}, errors.New("stored posting amount is invalid")
		}
		if totalPriceText.Valid != totalPriceScale.Valid || totalPriceText.Valid != totalPriceCommodity.Valid {
			postings.Close()
			return webapp.EntryDetail{}, errors.New("stored posting total price is invalid")
		}
		if amountText.Valid {
			decimal, err := ledger.ParseDecimal(amountText.String)
			if err != nil || int64(decimal.Scale()) != amountScale.Int64 || !commodity.Valid {
				postings.Close()
				return webapp.EntryDetail{}, errors.New("stored posting amount is invalid")
			}
			posting.Amount = &amountText.String
			posting.Commodity = commodity.String
		}
		if totalPriceText.Valid {
			decimal, err := ledger.ParseDecimal(totalPriceText.String)
			if err != nil || int64(decimal.Scale()) != totalPriceScale.Int64 || !totalPriceCommodity.Valid {
				postings.Close()
				return webapp.EntryDetail{}, errors.New("stored posting total price is invalid")
			}
			posting.TotalPrice = &webapp.AmountDetail{Amount: totalPriceText.String, Commodity: totalPriceCommodity.String}
		}
		detail.Postings = append(detail.Postings, posting)
	}
	if err := postings.Close(); err != nil {
		return webapp.EntryDetail{}, fmt.Errorf("close postings: %w", err)
	}
	detail.Diagnostics, err = loadDiagnostics(ctx, store.database, detail.RunIdentity, detail.RecordIndex)
	if err != nil {
		return webapp.EntryDetail{}, err
	}
	if err := store.loadRevisionHistory(ctx, &detail); err != nil {
		return webapp.EntryDetail{}, err
	}
	return detail, nil
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadDiagnostics(ctx context.Context, source queryer, runID string, recordIndex int) ([]webapp.DiagnosticDetail, error) {
	rows, err := source.QueryContext(ctx, `SELECT code, severity, message, field_path, posting_index
        FROM diagnostics WHERE run_id = ? AND record_index = ? ORDER BY diagnostic_index`, runID, recordIndex)
	if err != nil {
		return nil, fmt.Errorf("query diagnostics: %w", err)
	}
	defer rows.Close()
	diagnostics := []webapp.DiagnosticDetail{}
	for rows.Next() {
		var diagnostic webapp.DiagnosticDetail
		var fieldPath sql.NullString
		var postingIndex sql.NullInt64
		if err := rows.Scan(&diagnostic.Code, &diagnostic.Severity, &diagnostic.Message, &fieldPath, &postingIndex); err != nil {
			return nil, fmt.Errorf("scan diagnostic: %w", err)
		}
		diagnostic.FieldPath = fieldPath.String
		if postingIndex.Valid {
			value := int(postingIndex.Int64)
			diagnostic.PostingIndex = &value
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	return diagnostics, rows.Err()
}

func decodeIdentity(kind string, algorithm int, text string) (ingest.RecordIdentity, error) {
	if text != strings.ToLower(text) || len(text) != 64 {
		return ingest.RecordIdentity{}, errors.New("stored identity digest is invalid")
	}
	decoded, err := hex.DecodeString(text)
	if err != nil || len(decoded) != 32 {
		return ingest.RecordIdentity{}, errors.New("stored identity digest is invalid")
	}
	identity := ingest.RecordIdentity{Kind: ingest.IdentityKind(kind), AlgorithmVersion: algorithm}
	copy(identity.Digest[:], decoded)
	return identity, nil
}

func entryIdentifier(runID string, recordIndex int) string {
	return runID + ":" + strconv.Itoa(recordIndex)
}

func boolInteger(value bool) int {
	if value {
		return 1
	}
	return 0
}

func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return webapp.ErrNotFound
	}
	return err
}
