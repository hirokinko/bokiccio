# Web API v1

`webapp.Handler` is the storage-backed Web API for Bokiccio.
Tests use the local `tursogo` driver. Production composition uses remote Turso
Cloud and an IAP-protected server command.

## Routes

- `POST /api/v1/imports` accepts normalized input v1 or v2 as
  `application/json`.
  The body limit is 10 MiB. A committed run returns `201 Created`, including
  runs with record-level errors.
- `GET /api/v1/imports/{run-identity}` returns ordered outcomes, source
  references, diagnostics, and generated entry IDs.
- `GET /api/v1/entries?limit=50&cursor=...` returns current entry candidates in
  newest run/record order. Limits are from 1 through 100 and cursors are opaque.
  Optional filters are `date_from`, `date_to`, `account`, `description`,
  `status`, `workflow_status`, `source_namespace`, and `source_display`.
- `GET /api/v1/entries/{entry-id}` returns ordered comments and postings plus
  the immutable original, source, outcome diagnostics, revision history, and
  approval history.
- `POST /api/v1/entries/{entry-id}/revisions` accepts a complete entry snapshot
  and required `base_revision`. It stores both valid and invalid immutable
  revisions. A stale base returns `409 Conflict`.
- `POST /api/v1/entries/{entry-id}/approvals` approves the specified latest
  revision. Invalid revisions return `422 Unprocessable Entity`; stale
  revisions return `409 Conflict`. Revision `0` is the original import entry.
- `GET /api/v1/exports/tackler` exports approved current candidates as
  deterministic UTF-8 Tackler-compatible text while preserving an omitted
  final posting amount.
- `GET /api/v1/exports/json` exports the same approved snapshots as versioned
  JSON without pagination.
- `GET /api/v1/reporting/configuration` returns the current immutable reporting
  configuration. `POST` appends a configuration revision and requires the
  caller's `base_revision`; stale updates return `409 Conflict`.
- `GET /api/v1/reporting/configurations/{revision}` returns a historical,
  read-only reporting configuration revision.
- `GET /api/v1/reports/trial-balance?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD`
  returns a commodity-separated trial balance for an exact configured fiscal
  year or monthly period. Arbitrary date ranges are rejected. The response
  includes an opaque `snapshot_identity` for account drill-down.
- `GET /api/v1/reports/trial-balance/drill-down?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD&snapshot_identity=...&commodity=...&category=...&account=...&scope=subtree`
  returns the approved entries and posting contributions that explain all six
  opening, movement, and closing values of one trial-balance account row.
- `GET /api/v1/reports/current-overview?as_of=YYYY-MM-DD&expense_start_date=YYYY-MM-DD&expense_end_date=YYYY-MM-DD` returns asset,
  liability, and equity balances at the requested configured date, separately
  from expense totals and account details for an independently selected exact
  configured month. Entries after `as_of` are excluded from balances; the
  selected expense month is not truncated by `as_of`. This is an operational
  balance overview, not a balanced B/S.
- `GET /api/v1/reports/balance-sheet?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD`
  returns the opening balance sheet for an exact configured fiscal year. It
  uses that year's opening-entry or automatic-carry mode and excludes ordinary
  movements recorded on the fiscal-year start date.
- `GET /api/v1/reports/closing-balance-sheet?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD`
  returns the period-end balance sheet for an exact configured fiscal year.
  It adds approved movements through the fiscal-year end to the configured
  opening and presents the revenue and expense net balance as commodity-specific
  `current_earnings` without creating an account or closing entry.
- `GET /api/v1/reports/income-statement?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD`
  returns revenue, expenses, and net income for an exact configured monthly
  period. Fiscal-year and arbitrary ranges are rejected. The response includes
  an opaque `snapshot_identity` for account drill-down.
- `GET /api/v1/reports/income-statement/drill-down?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD&snapshot_identity=...&commodity=...&category=...&account=...&scope=subtree`
  returns the approved entries and posting contributions that explain one
  monthly income-statement account balance.
- `GET /api/v1/reports/balance-trend?start_date=YYYY-MM-DD&end_date=YYYY-MM-DD`
  returns twelve month-end points for an exact configured fiscal year. Each
  point contains cumulative balances for all five categories and unclassified
  accounts; it is not a monthly balance sheet.

Every JSON response carries `schema_version: 1`. Amounts are decimal strings
rather than JSON numbers. An omitted posting amount omits both `amount` and
`commodity`. A posting with a total price includes a nested `total_price`
object with decimal-string `amount` and `commodity`.

Both drill-down routes require the exact `snapshot_identity` returned by their
source report. `scope=subtree` explains the normal account subtotal, including
descendants; `scope=direct` explains only postings to the exact account. The
response preserves the report target amount, reports `total_entries`, and
groups contribution postings by approved entry in accounting order. Trial
balance entries are labelled `opening` or `movement`. Effective contribution
amounts use total price when present and otherwise use the explicit or inferred
posting amount, without converting commodities. Optional `limit` defaults to
50 and ranges from 1 through 100; `next_cursor` is opaque and bound to the full
selector and snapshot. Category, commodity, report totals, net income, and
reports other than the trial balance and monthly income statement are not
drill-down targets in this version.

Entry filters are combined with AND. Date bounds are inclusive and compare the
recorded local date. Account matching includes the exact account and its
colon-separated descendants. Description and source display use case-sensitive
substring matching; source namespace is exact. `status` remains the import
outcome (`success` or `warning`), while `workflow_status` is `unapproved`,
`invalid`, or `approved` for the current candidate. A cursor is bound to its
normalized filters and cannot be reused with different filters.

Exports accept the same filters, but `workflow_status` may only be omitted or
set to `approved`. They include only a currently approved candidate: an entry
whose older revision was approved but whose latest revision is unapproved or
invalid is excluded. Export order is recorded local date ascending, date-only
before timestamps on the same date, timestamp instant ascending, then import
and record order ascending. Empty Tackler and JSON exports return empty bytes
and `entries: []`, respectively.

Errors contain only a stable code and safe message. Request bodies, accounting
values, SQL details, paths, and credentials are not reflected in error bodies.
An IAP-authenticated user whose normalized verified email is absent from
`data_write_principals` may use every GET route, including entry and import
details, reporting configurations, reports, and exports. The user cannot call
imports, entry revision creation, approvals, or reporting configuration
creation. These non-import mutations return `403 Forbidden` with code
`write_forbidden` before parsing the request body and create no rows.
When the operator-managed file upload setting is disabled, `POST
/api/v1/imports` returns `403 Forbidden` with code `upload_disabled` before
parsing the upload and does not create an import run. Existing read, review,
approval, export, and reporting routes remain available.
When the setting is enabled but the normalized email from verified IAP claims
is absent from `data_write_principals`, the same route returns `403 Forbidden`
with code `upload_forbidden` before parsing and creates no import run. Missing
verified claims and raw identity headers without trusted request context are
treated as forbidden.
`reporting_not_configured` indicates that only reporting setup is missing;
existing import, review, approval, and export routes remain available.
`report_snapshot_changed` uses `409 Conflict` when the reporting configuration
or current approved entry input no longer matches the source report; clients
must reload that report before requesting drill-down again. Invalid selectors,
cursors, limits, or unknown and duplicate parameters return `400 Bad Request`
with `invalid_drill_down`.
`opening_balance_unbalanced` indicates that automatic carry-forward did not
produce a balanced asset, liability, and equity opening. Balance-sheet and
balance-trend responses use `422 Unprocessable Entity` in that state; the
period-end balance sheet uses the same error for any unbalanced configured
opening. `closing_balance_unbalanced` indicates that the period-end balance
sheet remains unbalanced after applying `current_earnings`; both errors use
`422 Unprocessable Entity`. Monthly income statements remain available.

## Storage

`webstore` uses `database/sql` and schema version `7`. A single import commits
the run, outcomes, diagnostics, entries, postings, accepted identities, and
workflow generation in one transaction. Decimal text and scale, date versus
timestamp precision, comment order, and posting omission are preserved.

Schema v2 adds immutable entry snapshots and append-only approval events while
leaving the original v1 entry rows unchanged. Revision creation reruns ledger
domain validation and records its diagnostics. Entry detail reports the latest
revision and reports a current approval only when that latest revision has been
approved. Schema v3 adds optional total-price columns to original and revision
postings while preserving posting quantities separately.
Schema v4 adds append-only reporting configuration revisions, explicit account
classifications, fiscal-year date ranges, opening-balance modes, and retained
opening-entry references. Trial balances read the latest configuration and all
currently approved entry snapshots in one database transaction. Decimal values
remain canonical strings and commodities are never implicitly converted.
Schema v5 adds the typed singleton `application_settings` row. Its checked
`file_upload_enabled` value defaults to enabled during migration and is read
inside every import transaction; a missing, invalid, or unreadable setting is
an error rather than an enabled fallback.
Schema v6 adds the checked `data_write_principals` email allowlist. Migration
creates an empty allowlist, normalized duplicate additions are idempotent, and
logical backup/restore preserves its rows. Restoring a schema v2-v5 backup
keeps this allowlist empty so every Web user remains read-only until an operator
explicitly registers an email.
Opening and period-end balance sheets, monthly income statements, and balance
trends use the same transactional reporting snapshot and do not add stored
report tables.

The local test driver is the official CGO-free `tursogo` driver. Production
uses `libsql-client-go` through `database/sql`; its connector receives the
token separately from the credential-free database URL.

## Production security

`bokiccio serve` requires all of the following settings before it starts:

- `TURSO_DATABASE_URL`
- `TURSO_AUTH_TOKEN`
- `BOKICCIO_IAP_AUDIENCE`
- `BOKICCIO_EXTERNAL_ORIGIN`
- `PORT`

`BOKICCIO_ENVIRONMENT` is optional and defaults to `production`. The exact
value `development` enables escaped internal error details on server-rendered
HTML error pages for controlled development only; JSON API errors remain
private-safe. Enable it only in a controlled environment restricted to trusted
developers.

Cloud Run must have direct IAP enabled and unauthenticated invocation disabled.
Only Google Accounts explicitly granted access by the IAP IAM policy may reach
the application. Except for `/livez`, every
request must have a valid ES256 `X-Goog-IAP-JWT-Assertion` with the configured
audience, the IAP issuer, a subject, a non-empty email, and a valid lifetime.
IAP IAM controls application-wide access. Data-changing routes additionally
normalize the verified email and require it in the database
`data_write_principals` allowlist. Import also requires the global upload
switch. The application does not trust unsigned identity or forwarded-host
headers. No HTTP deletion route is provided.

State-changing methods additionally require an `Origin` exactly matching one of
the comma-separated HTTPS origins in `BOKICCIO_EXTERNAL_ORIGIN`. The application
has no login endpoint, session store, or cookie; IAP owns the login session.
`/livez` returns only `ok` and does not inspect storage.

`bokiccio migrate` applies versioned migrations explicitly. `bokiccio serve`
never migrates implicitly and refuses to start unless the schema is current.
Both commands read the Turso token from the environment and do not print it.

`bokiccio backup` and `bokiccio restore` are operator CLI commands rather than
HTTP routes. They use a checksummed logical JSON envelope. Restore requires an
already-migrated empty database and never merges or replaces existing data.
See [logical backup format v1](../webstore/BACKUP.md).

`bokiccio settings set --file-upload-enabled=<true|false>` manages the global
upload switch. It uses the existing Turso environment variables, updates the
singleton in a transaction, and prints only the new boolean value. The data
write allowlist is operator-managed directly in the database for now; no
allowlist CLI, Web UI, or JSON API write route is provided.

The optional remote integration test writes an anonymous balanced entry to a
dedicated test database when both `BOKICCIO_TEST_TURSO_DATABASE_URL` and
`BOKICCIO_TEST_TURSO_AUTH_TOKEN` are set. Without them it is skipped.
