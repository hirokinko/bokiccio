package webstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/hirokinko/bokiccio/internal/ledger"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

func (store *Store) ListApprovedEntries(ctx context.Context, filter webapp.EntryFilter) (_ []webapp.ApprovedEntry, resultErr error) {
	if filter.WorkflowStatus != "" && filter.WorkflowStatus != "approved" {
		return nil, webapp.ErrInvalidRequest
	}
	filter.WorkflowStatus = "approved"
	if err := validateEntryQuery(webapp.EntryQuery{Filter: filter, Limit: 1}); err != nil {
		return nil, webapp.ErrInvalidRequest
	}
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin export transaction: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	statement := currentEntriesQuery + `
        SELECT c.entry_id, c.current_revision, c.source_namespace, c.source_display,
               (SELECT a.approved_at FROM entry_approvals a
                WHERE a.entry_id = c.entry_id AND a.revision = c.current_revision
                ORDER BY a.approval_sequence DESC LIMIT 1)
        FROM current_entries c` + entryFilterClause + `
        ORDER BY substr(c.occurred_at, 1, 10) ASC,
                 c.occurred_precision ASC,
                 CASE WHEN c.occurred_precision = 2 THEN julianday(c.occurred_at) END ASC,
                 c.sequence ASC,
                 c.record_index ASC`
	rows, err := transaction.QueryContext(ctx, statement, entryFilterArguments(filter)...)
	if err != nil {
		return nil, fmt.Errorf("query approved entries: %w", err)
	}
	type approvedMetadata struct {
		id         string
		revision   int
		approvedAt string
		source     webapp.Source
	}
	metadata := []approvedMetadata{}
	for rows.Next() {
		var item approvedMetadata
		if err := rows.Scan(&item.id, &item.revision, &item.source.Namespace, &item.source.Display, &item.approvedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan approved entry: %w", err)
		}
		metadata = append(metadata, item)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close approved entries: %w", err)
	}
	entries := make([]webapp.ApprovedEntry, 0, len(metadata))
	for _, item := range metadata {
		entry, err := loadEntrySnapshot(ctx, transaction, item.id, item.revision)
		if err != nil {
			return nil, err
		}
		entries = append(entries, webapp.ApprovedEntry{
			ID: item.id, Revision: item.revision, ApprovedAt: item.approvedAt, Source: item.source, Entry: entry,
		})
	}
	if err := transaction.Commit(); err != nil {
		return nil, fmt.Errorf("commit export transaction: %w", err)
	}
	return entries, nil
}

type rowQueryer interface {
	queryer
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func loadEntrySnapshot(ctx context.Context, source rowQueryer, entryID string, revision int) (ledger.JournalEntry, error) {
	var occurredAt, description string
	var precision int
	if revision == 0 {
		if err := source.QueryRowContext(ctx, `SELECT occurred_precision, occurred_at, description
            FROM entries WHERE entry_id = ?`, entryID).Scan(&precision, &occurredAt, &description); err != nil {
			return ledger.JournalEntry{}, notFound(err)
		}
	} else {
		var valid int
		if err := source.QueryRowContext(ctx, `SELECT occurred_precision, occurred_at, description, valid
            FROM entry_revisions WHERE entry_id = ? AND revision = ?`, entryID, revision).
			Scan(&precision, &occurredAt, &description, &valid); err != nil {
			return ledger.JournalEntry{}, notFound(err)
		}
		if valid == 0 {
			return ledger.JournalEntry{}, webapp.ErrInvalidRevision
		}
	}
	entryTime, err := ledger.ParseEntryTime(occurredAt)
	if err != nil || int(entryTime.Precision()) != precision {
		return ledger.JournalEntry{}, errors.New("stored export entry time is invalid")
	}
	entry := ledger.JournalEntry{Date: entryTime, Description: description}
	if revision == 0 {
		entry.Comments, err = loadOriginalComments(ctx, source, entryID)
		if err == nil {
			entry.Postings, err = loadOriginalPostings(ctx, source, entryID)
		}
	} else {
		entry.Comments, err = loadRevisionComments(ctx, source, entryID, revision)
		if err == nil {
			var postings []webapp.PostingDetail
			postings, err = loadRevisionPostings(ctx, source, entryID, revision)
			if err == nil {
				entry.Postings, err = postingDetailsToLedger(postings)
			}
		}
	}
	if err != nil {
		return ledger.JournalEntry{}, err
	}
	if err := ledger.Validate(entry); err != nil {
		return ledger.JournalEntry{}, fmt.Errorf("stored approved entry is invalid: %w", err)
	}
	return entry, nil
}

func loadOriginalComments(ctx context.Context, source queryer, entryID string) ([]string, error) {
	rows, err := source.QueryContext(ctx,
		`SELECT comment FROM entry_comments WHERE entry_id = ? ORDER BY comment_index`, entryID)
	if err != nil {
		return nil, fmt.Errorf("query original comments: %w", err)
	}
	defer rows.Close()
	comments := []string{}
	for rows.Next() {
		var comment string
		if err := rows.Scan(&comment); err != nil {
			return nil, fmt.Errorf("scan original comment: %w", err)
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

func loadOriginalPostings(ctx context.Context, source queryer, entryID string) ([]ledger.Posting, error) {
	rows, err := source.QueryContext(ctx, `SELECT account, amount_text, amount_scale, commodity,
        total_price_amount_text, total_price_amount_scale, total_price_commodity, comment
        FROM postings WHERE entry_id = ? ORDER BY posting_index`, entryID)
	if err != nil {
		return nil, fmt.Errorf("query original postings: %w", err)
	}
	defer rows.Close()
	return scanLedgerPostings(rows, "original")
}

func postingDetailsToLedger(details []webapp.PostingDetail) ([]ledger.Posting, error) {
	postings := make([]ledger.Posting, 0, len(details))
	for _, detail := range details {
		posting := ledger.Posting{Account: detail.Account, Comment: detail.Comment}
		if detail.Amount != nil {
			value, err := ledger.ParseDecimal(*detail.Amount)
			if err != nil {
				return nil, errors.New("stored revision posting amount is invalid")
			}
			posting.Amount = &ledger.Amount{Value: value, Commodity: ledger.Commodity(detail.Commodity)}
		}
		if detail.TotalPrice != nil {
			value, err := ledger.ParseDecimal(detail.TotalPrice.Amount)
			if err != nil {
				return nil, errors.New("stored revision posting total price is invalid")
			}
			posting.TotalPrice = &ledger.Amount{Value: value, Commodity: ledger.Commodity(detail.TotalPrice.Commodity)}
		}
		postings = append(postings, posting)
	}
	return postings, nil
}

func scanLedgerPostings(rows *sql.Rows, label string) ([]ledger.Posting, error) {
	postings := []ledger.Posting{}
	for rows.Next() {
		var posting ledger.Posting
		var amountText, commodity, totalPriceText, totalPriceCommodity sql.NullString
		var amountScale, totalPriceScale sql.NullInt64
		if err := rows.Scan(&posting.Account, &amountText, &amountScale, &commodity,
			&totalPriceText, &totalPriceScale, &totalPriceCommodity, &posting.Comment); err != nil {
			return nil, fmt.Errorf("scan %s posting: %w", label, err)
		}
		if amountText.Valid != amountScale.Valid || amountText.Valid != commodity.Valid {
			return nil, fmt.Errorf("stored %s posting amount is invalid", label)
		}
		if totalPriceText.Valid != totalPriceScale.Valid || totalPriceText.Valid != totalPriceCommodity.Valid {
			return nil, fmt.Errorf("stored %s posting total price is invalid", label)
		}
		if amountText.Valid {
			value, err := ledger.ParseDecimal(amountText.String)
			if err != nil || int64(value.Scale()) != amountScale.Int64 || !commodity.Valid {
				return nil, fmt.Errorf("stored %s posting amount is invalid", label)
			}
			posting.Amount = &ledger.Amount{Value: value, Commodity: ledger.Commodity(commodity.String)}
		}
		if totalPriceText.Valid {
			value, err := ledger.ParseDecimal(totalPriceText.String)
			if err != nil || int64(value.Scale()) != totalPriceScale.Int64 || !totalPriceCommodity.Valid {
				return nil, fmt.Errorf("stored %s posting total price is invalid", label)
			}
			posting.TotalPrice = &ledger.Amount{Value: value, Commodity: ledger.Commodity(totalPriceCommodity.String)}
		}
		postings = append(postings, posting)
	}
	return postings, rows.Err()
}
