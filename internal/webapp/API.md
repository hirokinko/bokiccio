# Web API v1

`webapp.Handler` is the storage-backed read-only vertical slice for Bokiccio.
Tests use the local `tursogo` driver. Production composition uses remote Turso
Cloud and an IAP-protected server command.

## Routes

- `POST /api/v1/imports` accepts normalized input v1 as `application/json`.
  The body limit is 10 MiB. A committed run returns `201 Created`, including
  runs with record-level errors.
- `GET /api/v1/imports/{run-identity}` returns ordered outcomes, source
  references, diagnostics, and generated entry IDs.
- `GET /api/v1/entries?limit=50&cursor=...` returns generated entries in newest
  run/record order. Limits are from 1 through 100 and cursors are opaque.
- `GET /api/v1/entries/{entry-id}` returns ordered comments and postings plus
  the source, outcome status, and diagnostics.

Every response carries `schema_version: 1`. Amounts are decimal strings rather
than JSON numbers. An omitted posting amount omits both `amount` and
`commodity`.

Errors contain only a stable code and safe message. Request bodies, accounting
values, SQL details, paths, and credentials are not reflected in error bodies.

## Storage

`webstore` uses `database/sql` and schema version `1`. A single import commits
the run, outcomes, diagnostics, entries, postings, accepted identities, and
workflow generation in one transaction. Decimal text and scale, date versus
timestamp precision, comment order, and posting omission are preserved.

The local test driver is the official CGO-free `tursogo` driver. Production
uses `libsql-client-go` through `database/sql`; its connector receives the
token separately from the credential-free database URL.

## Production security

`bokiccio serve` requires all of the following settings before it starts:

- `TURSO_DATABASE_URL`
- `TURSO_AUTH_TOKEN`
- `BOKICCIO_IAP_AUDIENCE`
- `BOKICCIO_OWNER_EMAIL`
- `BOKICCIO_EXTERNAL_ORIGIN`
- `PORT`

Cloud Run must have direct IAP enabled and unauthenticated invocation disabled.
Only the owner Google Account receives IAP access. Except for `/livez`, every
request must have a valid ES256 `X-Goog-IAP-JWT-Assertion` with the configured
audience, the IAP issuer, a subject, and the configured owner email. The
application does not trust unsigned identity or forwarded-host headers.

State-changing methods additionally require an `Origin` exactly matching
`BOKICCIO_EXTERNAL_ORIGIN`. The application has no login endpoint, session
store, or cookie; IAP owns the login session. `/livez` returns only `ok` and
does not inspect storage.

`bokiccio migrate` applies versioned migrations explicitly. `bokiccio serve`
never migrates implicitly and refuses to start unless the schema is current.
Both commands read the Turso token from the environment and do not print it.

The optional remote integration test writes an anonymous balanced entry to a
dedicated test database when both `BOKICCIO_TEST_TURSO_DATABASE_URL` and
`BOKICCIO_TEST_TURSO_AUTH_TOKEN` are set. Without them it is skipped.
