# Local read-only Web API v1

`webapp.Handler` is the first storage-backed vertical slice for Bokiccio. It is
an internal development API intended for `httptest` or an explicitly loopback
listener. The repository does not provide a public server command yet because
single-owner authentication is not implemented.

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

The local test driver is the official CGO-free `tursogo` driver. Remote Turso
Cloud composition and credentials are intentionally deferred until the
authenticated production-server slice.
