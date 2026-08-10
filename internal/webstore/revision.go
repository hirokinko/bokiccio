package webstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/hirokinko/bokiccio/internal/ledger"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

func (store *Store) CreateRevision(ctx context.Context, entryID string, request webapp.RevisionRequest) (_ webapp.RevisionDetail, resultErr error) {
	if request.BaseRevision == nil || *request.BaseRevision < 0 {
		return webapp.RevisionDetail{}, webapp.ErrInvalidRequest
	}
	entry, err := revisionEntry(request)
	if err != nil {
		return webapp.RevisionDetail{}, webapp.ErrInvalidRequest
	}
	diagnostics := revisionDiagnostics(ledger.Validate(entry))
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)

	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return webapp.RevisionDetail{}, fmt.Errorf("begin revision transaction: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	if err := requireEntry(ctx, transaction, entryID); err != nil {
		return webapp.RevisionDetail{}, err
	}
	latest, err := latestRevision(ctx, transaction, entryID)
	if err != nil {
		return webapp.RevisionDetail{}, err
	}
	if latest != *request.BaseRevision {
		return webapp.RevisionDetail{}, webapp.ErrConflict
	}
	revision := latest + 1
	result, err := transaction.ExecContext(ctx, `INSERT INTO entry_revisions
        (entry_id, revision, base_revision, created_at, occurred_precision, occurred_at, description, valid)
        SELECT ?, ?, ?, ?, ?, ?, ?, ?
        WHERE ? = (SELECT COALESCE(MAX(revision), 0) FROM entry_revisions WHERE entry_id = ?)`,
		entryID, revision, latest, createdAt, entry.Date.Precision(), entry.Date.String(), entry.Description,
		boolInteger(len(diagnostics) == 0), latest, entryID)
	if err != nil {
		return webapp.RevisionDetail{}, fmt.Errorf("save entry revision: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return webapp.RevisionDetail{}, fmt.Errorf("inspect entry revision insert: %w", err)
	}
	if inserted != 1 {
		return webapp.RevisionDetail{}, webapp.ErrConflict
	}
	if err := saveRevisionParts(ctx, transaction, entryID, revision, entry, diagnostics); err != nil {
		return webapp.RevisionDetail{}, err
	}
	if err := transaction.Commit(); err != nil {
		return webapp.RevisionDetail{}, fmt.Errorf("commit entry revision: %w", err)
	}
	return revisionDetail(revision, latest, createdAt, request, diagnostics), nil
}

func (store *Store) ApproveRevision(ctx context.Context, entryID string, request webapp.ApprovalRequest) (_ webapp.ApprovalDetail, resultErr error) {
	if request.Revision == nil || *request.Revision < 0 {
		return webapp.ApprovalDetail{}, webapp.ErrInvalidRequest
	}
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return webapp.ApprovalDetail{}, fmt.Errorf("begin approval transaction: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	if err := requireEntry(ctx, transaction, entryID); err != nil {
		return webapp.ApprovalDetail{}, err
	}
	latest, err := latestRevision(ctx, transaction, entryID)
	if err != nil {
		return webapp.ApprovalDetail{}, err
	}
	if latest != *request.Revision {
		return webapp.ApprovalDetail{}, webapp.ErrConflict
	}
	if latest > 0 {
		var valid int
		if err := transaction.QueryRowContext(ctx,
			`SELECT valid FROM entry_revisions WHERE entry_id = ? AND revision = ?`, entryID, latest).Scan(&valid); err != nil {
			return webapp.ApprovalDetail{}, notFound(err)
		}
		if valid == 0 {
			return webapp.ApprovalDetail{}, webapp.ErrInvalidRevision
		}
	}
	approvedAt := time.Now().UTC().Format(time.RFC3339Nano)
	var sequence int64
	if err := transaction.QueryRowContext(ctx, `INSERT INTO entry_approvals (entry_id, revision, approved_at)
        SELECT ?, ?, ? WHERE ? = (SELECT COALESCE(MAX(revision), 0) FROM entry_revisions WHERE entry_id = ?)
        RETURNING approval_sequence`, entryID, latest, approvedAt, latest, entryID).Scan(&sequence); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return webapp.ApprovalDetail{}, webapp.ErrConflict
		}
		return webapp.ApprovalDetail{}, fmt.Errorf("save entry approval: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return webapp.ApprovalDetail{}, fmt.Errorf("commit entry approval: %w", err)
	}
	return webapp.ApprovalDetail{Sequence: sequence, Revision: latest, ApprovedAt: approvedAt}, nil
}

func requireEntry(ctx context.Context, transaction *sql.Tx, entryID string) error {
	var found int
	if err := transaction.QueryRowContext(ctx, `SELECT 1 FROM entries WHERE entry_id = ?`, entryID).Scan(&found); err != nil {
		return notFound(err)
	}
	return nil
}

func latestRevision(ctx context.Context, transaction *sql.Tx, entryID string) (int, error) {
	var revision int
	if err := transaction.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(revision), 0) FROM entry_revisions WHERE entry_id = ?`, entryID).Scan(&revision); err != nil {
		return 0, fmt.Errorf("read latest revision: %w", err)
	}
	return revision, nil
}

func revisionEntry(request webapp.RevisionRequest) (ledger.JournalEntry, error) {
	entryTime, err := ledger.ParseEntryTime(request.OccurredAt)
	if err != nil {
		return ledger.JournalEntry{}, err
	}
	entry := ledger.JournalEntry{Date: entryTime, Description: request.Description, Comments: append([]string(nil), request.Comments...)}
	for _, input := range request.Postings {
		posting := ledger.Posting{Account: input.Account, Comment: input.Comment}
		if input.Amount == nil {
			if input.Commodity != "" {
				return ledger.JournalEntry{}, errors.New("omitted amount has a commodity")
			}
		} else {
			value, err := ledger.ParseDecimal(*input.Amount)
			if err != nil {
				return ledger.JournalEntry{}, err
			}
			posting.Amount = &ledger.Amount{Value: value, Commodity: ledger.Commodity(input.Commodity)}
		}
		entry.Postings = append(entry.Postings, posting)
	}
	return entry, nil
}

func revisionDiagnostics(validationErr error) []webapp.DiagnosticDetail {
	if validationErr == nil {
		return []webapp.DiagnosticDetail{}
	}
	diagnostic := webapp.DiagnosticDetail{Code: "invalid_entry", Severity: "error", Message: validationErr.Error()}
	var postingErr *ledger.PostingValidationError
	if errors.As(validationErr, &postingErr) {
		diagnostic.Code = "invalid_posting"
		diagnostic.FieldPath = fmt.Sprintf("postings[%d]", postingErr.Index)
		diagnostic.PostingIndex = &postingErr.Index
	} else if errors.Is(validationErr, ledger.ErrUnbalancedEntry) {
		diagnostic.Code = "unbalanced_entry"
	}
	return []webapp.DiagnosticDetail{diagnostic}
}

func saveRevisionParts(ctx context.Context, transaction *sql.Tx, entryID string, revision int, entry ledger.JournalEntry, diagnostics []webapp.DiagnosticDetail) error {
	for index, comment := range entry.Comments {
		if _, err := transaction.ExecContext(ctx,
			`INSERT INTO revision_comments (entry_id, revision, comment_index, comment) VALUES (?, ?, ?, ?)`,
			entryID, revision, index, comment); err != nil {
			return fmt.Errorf("save revision comment %d: %w", index, err)
		}
	}
	for index, posting := range entry.Postings {
		var amountText, amountScale, commodity any
		if posting.Amount != nil {
			amountText = posting.Amount.Value.String()
			amountScale = posting.Amount.Value.Scale()
			commodity = posting.Amount.Commodity
		}
		if _, err := transaction.ExecContext(ctx, `INSERT INTO revision_postings
            (entry_id, revision, posting_index, account, amount_text, amount_scale, commodity, comment)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, entryID, revision, index, posting.Account,
			amountText, amountScale, commodity, posting.Comment); err != nil {
			return fmt.Errorf("save revision posting %d: %w", index, err)
		}
	}
	for index, diagnostic := range diagnostics {
		var postingIndex any
		if diagnostic.PostingIndex != nil {
			postingIndex = *diagnostic.PostingIndex
		}
		if _, err := transaction.ExecContext(ctx, `INSERT INTO revision_diagnostics
            (entry_id, revision, diagnostic_index, code, severity, message, field_path, posting_index)
            VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?)`, entryID, revision, index, diagnostic.Code,
			diagnostic.Severity, diagnostic.Message, diagnostic.FieldPath, postingIndex); err != nil {
			return fmt.Errorf("save revision diagnostic %d: %w", index, err)
		}
	}
	return nil
}

func revisionDetail(revision, base int, createdAt string, request webapp.RevisionRequest, diagnostics []webapp.DiagnosticDetail) webapp.RevisionDetail {
	return webapp.RevisionDetail{
		Revision: revision, BaseRevision: base, CreatedAt: createdAt, OccurredAt: request.OccurredAt,
		Description: request.Description, Comments: append([]string(nil), request.Comments...),
		Postings: append([]webapp.PostingDetail(nil), request.Postings...), Valid: len(diagnostics) == 0,
		Diagnostics: diagnostics,
	}
}

func (store *Store) loadRevisionHistory(ctx context.Context, detail *webapp.EntryDetail) error {
	detail.Revisions = []webapp.RevisionDetail{}
	detail.Approvals = []webapp.ApprovalDetail{}
	rows, err := store.database.QueryContext(ctx, `SELECT revision, base_revision, created_at, occurred_precision,
        occurred_at, description, valid FROM entry_revisions WHERE entry_id = ? ORDER BY revision`, detail.ID)
	if err != nil {
		return fmt.Errorf("query entry revisions: %w", err)
	}
	for rows.Next() {
		var revision webapp.RevisionDetail
		var precision, valid int
		if err := rows.Scan(&revision.Revision, &revision.BaseRevision, &revision.CreatedAt, &precision,
			&revision.OccurredAt, &revision.Description, &valid); err != nil {
			rows.Close()
			return fmt.Errorf("scan entry revision: %w", err)
		}
		entryTime, err := ledger.ParseEntryTime(revision.OccurredAt)
		if err != nil || int(entryTime.Precision()) != precision {
			rows.Close()
			return errors.New("stored revision entry time is invalid")
		}
		revision.Valid = valid != 0
		detail.Revisions = append(detail.Revisions, revision)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close entry revisions: %w", err)
	}
	for index := range detail.Revisions {
		revision := &detail.Revisions[index]
		revision.Comments, err = loadRevisionComments(ctx, store.database, detail.ID, revision.Revision)
		if err != nil {
			return err
		}
		revision.Postings, err = loadRevisionPostings(ctx, store.database, detail.ID, revision.Revision)
		if err != nil {
			return err
		}
		revision.Diagnostics, err = loadRevisionDiagnostics(ctx, store.database, detail.ID, revision.Revision)
		if err != nil {
			return err
		}
		detail.CurrentRevision = revision.Revision
	}
	approvalRows, err := store.database.QueryContext(ctx, `SELECT approval_sequence, revision, approved_at
        FROM entry_approvals WHERE entry_id = ? ORDER BY approval_sequence`, detail.ID)
	if err != nil {
		return fmt.Errorf("query entry approvals: %w", err)
	}
	for approvalRows.Next() {
		var approval webapp.ApprovalDetail
		if err := approvalRows.Scan(&approval.Sequence, &approval.Revision, &approval.ApprovedAt); err != nil {
			approvalRows.Close()
			return fmt.Errorf("scan entry approval: %w", err)
		}
		detail.Approvals = append(detail.Approvals, approval)
	}
	if err := approvalRows.Close(); err != nil {
		return fmt.Errorf("close entry approvals: %w", err)
	}
	if len(detail.Approvals) > 0 {
		latest := detail.Approvals[len(detail.Approvals)-1]
		if latest.Revision == detail.CurrentRevision {
			detail.CurrentApproval = &latest
		}
	}
	return nil
}

func loadRevisionComments(ctx context.Context, source queryer, entryID string, revision int) ([]string, error) {
	rows, err := source.QueryContext(ctx, `SELECT comment FROM revision_comments
        WHERE entry_id = ? AND revision = ? ORDER BY comment_index`, entryID, revision)
	if err != nil {
		return nil, fmt.Errorf("query revision comments: %w", err)
	}
	defer rows.Close()
	comments := []string{}
	for rows.Next() {
		var comment string
		if err := rows.Scan(&comment); err != nil {
			return nil, fmt.Errorf("scan revision comment: %w", err)
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func loadRevisionPostings(ctx context.Context, source queryer, entryID string, revision int) ([]webapp.PostingDetail, error) {
	rows, err := source.QueryContext(ctx, `SELECT account, amount_text, amount_scale, commodity, comment
        FROM revision_postings WHERE entry_id = ? AND revision = ? ORDER BY posting_index`, entryID, revision)
	if err != nil {
		return nil, fmt.Errorf("query revision postings: %w", err)
	}
	defer rows.Close()
	postings := []webapp.PostingDetail{}
	for rows.Next() {
		var posting webapp.PostingDetail
		var amountText, commodity sql.NullString
		var amountScale sql.NullInt64
		if err := rows.Scan(&posting.Account, &amountText, &amountScale, &commodity, &posting.Comment); err != nil {
			return nil, fmt.Errorf("scan revision posting: %w", err)
		}
		if amountText.Valid {
			decimal, err := ledger.ParseDecimal(amountText.String)
			if err != nil || int64(decimal.Scale()) != amountScale.Int64 || !commodity.Valid {
				return nil, errors.New("stored revision posting amount is invalid")
			}
			posting.Amount = &amountText.String
			posting.Commodity = commodity.String
		}
		postings = append(postings, posting)
	}
	return postings, rows.Err()
}

func loadRevisionDiagnostics(ctx context.Context, source queryer, entryID string, revision int) ([]webapp.DiagnosticDetail, error) {
	rows, err := source.QueryContext(ctx, `SELECT code, severity, message, field_path, posting_index
        FROM revision_diagnostics WHERE entry_id = ? AND revision = ? ORDER BY diagnostic_index`, entryID, revision)
	if err != nil {
		return nil, fmt.Errorf("query revision diagnostics: %w", err)
	}
	defer rows.Close()
	diagnostics := []webapp.DiagnosticDetail{}
	for rows.Next() {
		var diagnostic webapp.DiagnosticDetail
		var fieldPath sql.NullString
		var postingIndex sql.NullInt64
		if err := rows.Scan(&diagnostic.Code, &diagnostic.Severity, &diagnostic.Message, &fieldPath, &postingIndex); err != nil {
			return nil, fmt.Errorf("scan revision diagnostic: %w", err)
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
