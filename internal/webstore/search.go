package webstore

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hirokinko/bokiccio/internal/webapp"
)

type searchPosition struct {
	Sequence int64  `json:"s"`
	Record   int    `json:"r"`
	Filter   string `json:"f"`
}

func (store *Store) ListEntries(ctx context.Context, query webapp.EntryQuery) (webapp.EntryPage, error) {
	if err := validateEntryQuery(query); err != nil {
		return webapp.EntryPage{}, webapp.ErrInvalidRequest
	}
	digest := filterDigest(query.Filter)
	position, err := decodeSearchCursor(query.Cursor, digest)
	if err != nil {
		return webapp.EntryPage{}, webapp.ErrInvalidRequest
	}
	statement := currentEntriesQuery + `
        SELECT entry_id, occurred_at, description, import_status, workflow_status, current_revision,
               source_namespace, source_display, sequence, record_index
        FROM current_entries c` + entryFilterClause + `
          AND (? = 0 OR c.sequence < ? OR (c.sequence = ? AND c.record_index < ?))
        ORDER BY c.sequence DESC, c.record_index DESC LIMIT ?`
	arguments := append(entryFilterArguments(query.Filter),
		position.Sequence, position.Sequence, position.Sequence, position.Record, query.Limit+1)
	rows, err := store.database.QueryContext(ctx, statement, arguments...)
	if err != nil {
		return webapp.EntryPage{}, fmt.Errorf("query entry list: %w", err)
	}
	defer rows.Close()
	page := webapp.EntryPage{SchemaVersion: webapp.APISchemaVersion, Entries: []webapp.EntrySummary{}}
	positions := make([]searchPosition, 0, query.Limit+1)
	for rows.Next() {
		var entry webapp.EntrySummary
		var current searchPosition
		if err := rows.Scan(&entry.ID, &entry.OccurredAt, &entry.Description, &entry.Status,
			&entry.WorkflowStatus, &entry.CurrentRevision, &entry.Source.Namespace, &entry.Source.Display,
			&current.Sequence, &current.Record); err != nil {
			return webapp.EntryPage{}, fmt.Errorf("scan entry list: %w", err)
		}
		page.Entries = append(page.Entries, entry)
		positions = append(positions, current)
	}
	if err := rows.Err(); err != nil {
		return webapp.EntryPage{}, fmt.Errorf("iterate entry list: %w", err)
	}
	if len(page.Entries) > query.Limit {
		page.Entries = page.Entries[:query.Limit]
		last := positions[query.Limit-1]
		last.Filter = digest
		page.NextCursor = encodeSearchCursor(last)
	}
	return page, nil
}

const entryFilterClause = `
        WHERE (? = '' OR substr(c.occurred_at, 1, 10) >= ?)
          AND (? = '' OR substr(c.occurred_at, 1, 10) <= ?)
          AND (? = '' OR instr(c.description, ?) > 0)
          AND (? = '' OR c.import_status = ?)
          AND (? = '' OR c.workflow_status = ?)
          AND (? = '' OR c.source_namespace = ?)
          AND (? = '' OR instr(c.source_display, ?) > 0)
          AND (? = '' OR
               (c.current_revision = 0 AND EXISTS (
                   SELECT 1 FROM postings p WHERE p.entry_id = c.entry_id
                     AND (p.account = ? OR instr(p.account, ? || ':') = 1)
               )) OR
               (c.current_revision > 0 AND EXISTS (
                   SELECT 1 FROM revision_postings p WHERE p.entry_id = c.entry_id
                     AND p.revision = c.current_revision
                     AND (p.account = ? OR instr(p.account, ? || ':') = 1)
               )))`

func entryFilterArguments(filter webapp.EntryFilter) []any {
	return []any{
		filter.DateFrom, filter.DateFrom,
		filter.DateTo, filter.DateTo,
		filter.Description, filter.Description,
		filter.Status, filter.Status,
		filter.WorkflowStatus, filter.WorkflowStatus,
		filter.SourceNamespace, filter.SourceNamespace,
		filter.SourceDisplay, filter.SourceDisplay,
		filter.Account, filter.Account, filter.Account, filter.Account, filter.Account,
	}
}

const currentEntriesQuery = `WITH latest_revisions AS (
    SELECT entry_id, MAX(revision) AS revision FROM entry_revisions GROUP BY entry_id
), current_entries AS (
    SELECT e.entry_id,
           COALESCE(er.revision, 0) AS current_revision,
           COALESCE(er.occurred_precision, e.occurred_precision) AS occurred_precision,
           COALESCE(er.occurred_at, e.occurred_at) AS occurred_at,
           COALESCE(er.description, e.description) AS description,
           o.status AS import_status,
           o.source_namespace,
           o.source_display,
           r.sequence,
           e.record_index,
           CASE
             WHEN er.revision IS NOT NULL AND er.valid = 0 THEN 'invalid'
             WHEN EXISTS (
               SELECT 1 FROM entry_approvals a
               WHERE a.entry_id = e.entry_id
                 AND a.revision = COALESCE(er.revision, 0)
             ) THEN 'approved'
             ELSE 'unapproved'
           END AS workflow_status
    FROM entries e
    JOIN import_runs r ON r.run_id = e.run_id
    JOIN outcomes o ON o.run_id = e.run_id AND o.record_index = e.record_index
    LEFT JOIN latest_revisions lr ON lr.entry_id = e.entry_id
    LEFT JOIN entry_revisions er ON er.entry_id = lr.entry_id AND er.revision = lr.revision
)`

func validateEntryQuery(query webapp.EntryQuery) error {
	if query.Limit < 1 || query.Limit > 100 {
		return errors.New("invalid limit")
	}
	filter := query.Filter
	if err := validateDateRange(filter.DateFrom, filter.DateTo); err != nil {
		return err
	}
	if filter.Status != "" && filter.Status != "success" && filter.Status != "warning" {
		return errors.New("invalid import status")
	}
	if filter.WorkflowStatus != "" && filter.WorkflowStatus != "unapproved" && filter.WorkflowStatus != "invalid" && filter.WorkflowStatus != "approved" {
		return errors.New("invalid workflow status")
	}
	for _, value := range []string{filter.Account, filter.Description, filter.SourceNamespace, filter.SourceDisplay} {
		if strings.ContainsAny(value, "\r\n") {
			return errors.New("filter contains a line break")
		}
	}
	return nil
}

func validateDateRange(from, to string) error {
	for _, value := range []string{from, to} {
		if value == "" {
			continue
		}
		if parsed, err := time.Parse("2006-01-02", value); err != nil || parsed.Format("2006-01-02") != value {
			return errors.New("invalid date filter")
		}
	}
	if from != "" && to != "" && from > to {
		return errors.New("invalid date range")
	}
	return nil
}

func filterDigest(filter webapp.EntryFilter) string {
	encoded, _ := json.Marshal(filter)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func encodeSearchCursor(position searchPosition) string {
	encoded, _ := json.Marshal(position)
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func decodeSearchCursor(cursor, digest string) (searchPosition, error) {
	if cursor == "" {
		return searchPosition{}, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return searchPosition{}, err
	}
	var position searchPosition
	decoder := json.NewDecoder(strings.NewReader(string(decoded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&position); err != nil || position.Sequence < 1 || position.Record < 0 || position.Filter != digest {
		return searchPosition{}, errors.New("invalid cursor")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return searchPosition{}, errors.New("invalid cursor")
	}
	return position, nil
}
