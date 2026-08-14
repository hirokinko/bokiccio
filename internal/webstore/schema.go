package webstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const SchemaVersion = 5

var ErrUnsupportedSchema = errors.New("unsupported web storage schema")

func CheckSchema(ctx context.Context, database *sql.DB) error {
	var version int
	if err := database.QueryRowContext(ctx, `SELECT version FROM schema_metadata WHERE singleton = 1`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version != SchemaVersion {
		return fmt.Errorf("%w: database version %d, required version %d", ErrUnsupportedSchema, version, SchemaVersion)
	}
	return nil
}

var migrationV1 = []string{
	`CREATE TABLE workflow_state (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    generation INTEGER NOT NULL CHECK (generation >= 0)
)`,
	`INSERT INTO workflow_state (singleton, generation) VALUES (1, 0)`,
	`CREATE TABLE committed_identities (
    kind TEXT NOT NULL,
    algorithm_version INTEGER NOT NULL,
    digest TEXT NOT NULL CHECK (length(digest) = 64),
    PRIMARY KEY (kind, algorithm_version, digest)
)`,
	`CREATE TABLE import_runs (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL UNIQUE CHECK (length(run_id) = 64),
    algorithm_version INTEGER NOT NULL,
    input_digest TEXT NOT NULL CHECK (length(input_digest) = 64),
    pre_state_generation INTEGER NOT NULL CHECK (pre_state_generation >= 0),
    has_errors INTEGER NOT NULL CHECK (has_errors IN (0, 1)),
    report_json BLOB NOT NULL,
    journal BLOB
)`,
	`CREATE TABLE outcomes (
    run_id TEXT NOT NULL,
    record_index INTEGER NOT NULL CHECK (record_index >= 0),
    status TEXT NOT NULL CHECK (status IN ('success', 'warning', 'error', 'duplicate')),
    source_namespace TEXT NOT NULL,
    source_display TEXT NOT NULL,
    identity_kind TEXT NOT NULL,
    identity_algorithm_version INTEGER NOT NULL,
    identity_digest TEXT NOT NULL CHECK (length(identity_digest) = 64),
    PRIMARY KEY (run_id, record_index),
    FOREIGN KEY (run_id) REFERENCES import_runs(run_id)
)`,
	`CREATE TABLE diagnostics (
    run_id TEXT NOT NULL,
    record_index INTEGER NOT NULL,
    diagnostic_index INTEGER NOT NULL CHECK (diagnostic_index >= 0),
    code TEXT NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('info', 'warning', 'error')),
    message TEXT NOT NULL,
    field_path TEXT,
    posting_index INTEGER,
    PRIMARY KEY (run_id, record_index, diagnostic_index),
    FOREIGN KEY (run_id, record_index) REFERENCES outcomes(run_id, record_index)
)`,
	`CREATE TABLE entries (
    entry_id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL,
    record_index INTEGER NOT NULL,
    occurred_precision INTEGER NOT NULL CHECK (occurred_precision IN (1, 2)),
    occurred_at TEXT NOT NULL,
    description TEXT NOT NULL,
    UNIQUE (run_id, record_index),
    FOREIGN KEY (run_id, record_index) REFERENCES outcomes(run_id, record_index)
)`,
	`CREATE TABLE entry_comments (
    entry_id TEXT NOT NULL,
    comment_index INTEGER NOT NULL CHECK (comment_index >= 0),
    comment TEXT NOT NULL,
    PRIMARY KEY (entry_id, comment_index),
    FOREIGN KEY (entry_id) REFERENCES entries(entry_id)
)`,
	`CREATE TABLE postings (
    entry_id TEXT NOT NULL,
    posting_index INTEGER NOT NULL CHECK (posting_index >= 0),
    account TEXT NOT NULL,
    amount_text TEXT,
    amount_scale INTEGER,
    commodity TEXT,
    comment TEXT NOT NULL,
    PRIMARY KEY (entry_id, posting_index),
    CHECK ((amount_text IS NULL AND amount_scale IS NULL AND commodity IS NULL) OR
           (amount_text IS NOT NULL AND amount_scale IS NOT NULL AND commodity IS NOT NULL)),
    FOREIGN KEY (entry_id) REFERENCES entries(entry_id)
)`,
	`CREATE INDEX entries_run_order ON entries(run_id, record_index)`,
}

var migrationV2 = []string{
	`CREATE TABLE entry_revisions (
    entry_id TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK (revision >= 1),
    base_revision INTEGER NOT NULL CHECK (base_revision >= 0),
    created_at TEXT NOT NULL,
    occurred_precision INTEGER NOT NULL CHECK (occurred_precision IN (1, 2)),
    occurred_at TEXT NOT NULL,
    description TEXT NOT NULL,
    valid INTEGER NOT NULL CHECK (valid IN (0, 1)),
    PRIMARY KEY (entry_id, revision),
    FOREIGN KEY (entry_id) REFERENCES entries(entry_id)
)`,
	`CREATE TABLE revision_comments (
    entry_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    comment_index INTEGER NOT NULL CHECK (comment_index >= 0),
    comment TEXT NOT NULL,
    PRIMARY KEY (entry_id, revision, comment_index),
    FOREIGN KEY (entry_id, revision) REFERENCES entry_revisions(entry_id, revision)
)`,
	`CREATE TABLE revision_postings (
    entry_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    posting_index INTEGER NOT NULL CHECK (posting_index >= 0),
    account TEXT NOT NULL,
    amount_text TEXT,
    amount_scale INTEGER,
    commodity TEXT,
    comment TEXT NOT NULL,
    PRIMARY KEY (entry_id, revision, posting_index),
    CHECK ((amount_text IS NULL AND amount_scale IS NULL AND commodity IS NULL) OR
           (amount_text IS NOT NULL AND amount_scale IS NOT NULL AND commodity IS NOT NULL)),
    FOREIGN KEY (entry_id, revision) REFERENCES entry_revisions(entry_id, revision)
)`,
	`CREATE TABLE revision_diagnostics (
    entry_id TEXT NOT NULL,
    revision INTEGER NOT NULL,
    diagnostic_index INTEGER NOT NULL CHECK (diagnostic_index >= 0),
    code TEXT NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('info', 'warning', 'error')),
    message TEXT NOT NULL,
    field_path TEXT,
    posting_index INTEGER,
    PRIMARY KEY (entry_id, revision, diagnostic_index),
    FOREIGN KEY (entry_id, revision) REFERENCES entry_revisions(entry_id, revision)
)`,
	`CREATE TABLE entry_approvals (
    approval_sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    entry_id TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK (revision >= 0),
    approved_at TEXT NOT NULL,
    FOREIGN KEY (entry_id) REFERENCES entries(entry_id)
)`,
	`CREATE INDEX entry_approvals_history ON entry_approvals(entry_id, approval_sequence)`,
}

var migrationV3 = []string{
	`ALTER TABLE postings ADD COLUMN total_price_amount_text TEXT`,
	`ALTER TABLE postings ADD COLUMN total_price_amount_scale INTEGER`,
	`ALTER TABLE postings ADD COLUMN total_price_commodity TEXT`,
	`ALTER TABLE revision_postings ADD COLUMN total_price_amount_text TEXT`,
	`ALTER TABLE revision_postings ADD COLUMN total_price_amount_scale INTEGER`,
	`ALTER TABLE revision_postings ADD COLUMN total_price_commodity TEXT`,
}

var migrationV4 = []string{
	`CREATE TABLE reporting_configurations (
    revision INTEGER PRIMARY KEY CHECK (revision >= 1),
    base_revision INTEGER NOT NULL CHECK (base_revision >= 0),
    created_at TEXT NOT NULL,
    start_month INTEGER NOT NULL CHECK (start_month BETWEEN 1 AND 12),
    CHECK (revision = base_revision + 1)
)`,
	`CREATE TABLE reporting_classifications (
    revision INTEGER NOT NULL,
    account TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('asset', 'liability', 'equity', 'revenue', 'expense')),
    PRIMARY KEY (revision, account),
    FOREIGN KEY (revision) REFERENCES reporting_configurations(revision)
)`,
	`CREATE TABLE reporting_fiscal_years (
    revision INTEGER NOT NULL,
    start_date TEXT NOT NULL,
    end_date TEXT NOT NULL,
    opening_mode TEXT NOT NULL CHECK (opening_mode IN ('automatic', 'opening_entries')),
    PRIMARY KEY (revision, start_date, end_date),
    FOREIGN KEY (revision) REFERENCES reporting_configurations(revision)
)`,
	`CREATE TABLE reporting_opening_entries (
    revision INTEGER NOT NULL,
    fiscal_year_start TEXT NOT NULL,
    fiscal_year_end TEXT NOT NULL,
    entry_index INTEGER NOT NULL CHECK (entry_index >= 0),
    entry_id TEXT NOT NULL,
    PRIMARY KEY (revision, fiscal_year_start, fiscal_year_end, entry_index),
    UNIQUE (revision, fiscal_year_start, fiscal_year_end, entry_id),
    FOREIGN KEY (revision, fiscal_year_start, fiscal_year_end)
        REFERENCES reporting_fiscal_years(revision, start_date, end_date),
    FOREIGN KEY (entry_id) REFERENCES entries(entry_id)
)`,
}

var migrationV5 = []string{
	`CREATE TABLE application_settings (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    file_upload_enabled INTEGER NOT NULL CHECK (file_upload_enabled IN (0, 1))
)`,
	`INSERT INTO application_settings (singleton, file_upload_enabled) VALUES (1, 1)`,
}

func Migrate(ctx context.Context, database *sql.DB) (resultErr error) {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin schema migration: %w", err)
	}
	defer func() {
		if resultErr != nil {
			_ = transaction.Rollback()
		}
	}()
	if _, err := transaction.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_metadata (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    version INTEGER NOT NULL CHECK (version >= 0)
)`); err != nil {
		return fmt.Errorf("create schema metadata: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, `INSERT OR IGNORE INTO schema_metadata (singleton, version) VALUES (1, 0)`); err != nil {
		return fmt.Errorf("initialize schema metadata: %w", err)
	}
	var version int
	if err := transaction.QueryRowContext(ctx, `SELECT version FROM schema_metadata WHERE singleton = 1`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > SchemaVersion {
		return fmt.Errorf("%w: database version %d is newer than supported version %d", ErrUnsupportedSchema, version, SchemaVersion)
	}
	if version == 0 {
		for index, statement := range migrationV1 {
			if _, err := transaction.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply schema v1 statement %d: %w", index+1, err)
			}
		}
		if _, err := transaction.ExecContext(ctx, `UPDATE schema_metadata SET version = 1 WHERE singleton = 1 AND version = 0`); err != nil {
			return fmt.Errorf("commit schema version 1: %w", err)
		}
		version = 1
	}
	if version == 1 {
		for index, statement := range migrationV2 {
			if _, err := transaction.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply schema v2 statement %d: %w", index+1, err)
			}
		}
		if _, err := transaction.ExecContext(ctx, `UPDATE schema_metadata SET version = 2 WHERE singleton = 1 AND version = 1`); err != nil {
			return fmt.Errorf("commit schema version 2: %w", err)
		}
		version = 2
	}
	if version == 2 {
		for index, statement := range migrationV3 {
			if _, err := transaction.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply schema v3 statement %d: %w", index+1, err)
			}
		}
		if _, err := transaction.ExecContext(ctx, `UPDATE schema_metadata SET version = 3 WHERE singleton = 1 AND version = 2`); err != nil {
			return fmt.Errorf("commit schema version 3: %w", err)
		}
		version = 3
	}
	if version == 3 {
		for index, statement := range migrationV4 {
			if _, err := transaction.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply schema v4 statement %d: %w", index+1, err)
			}
		}
		if _, err := transaction.ExecContext(ctx, `UPDATE schema_metadata SET version = 4 WHERE singleton = 1 AND version = 3`); err != nil {
			return fmt.Errorf("commit schema version 4: %w", err)
		}
		version = 4
	}
	if version == 4 {
		for index, statement := range migrationV5 {
			if _, err := transaction.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("apply schema v5 statement %d: %w", index+1, err)
			}
		}
		if _, err := transaction.ExecContext(ctx, `UPDATE schema_metadata SET version = 5 WHERE singleton = 1 AND version = 4`); err != nil {
			return fmt.Errorf("commit schema version 5: %w", err)
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit schema migration: %w", err)
	}
	return nil
}
