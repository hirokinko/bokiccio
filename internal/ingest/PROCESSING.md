# Record processing contract

`ingest.Process` evaluates decoded records in input order. It accepts a batch
and the identities committed by earlier runs, and returns one outcome per
record plus the validated journal entries accepted for export.

## Outcomes

- `success`: domain validation succeeded and the record has no warning.
- `warning`: domain validation succeeded and structured warnings are present.
- `error`: domain validation failed. No journal entry is returned.
- `duplicate`: the identity was committed earlier or already occurred in the
  current batch. No journal entry is returned.

Committed duplicates take precedence over same-batch duplicate detection.
Within a batch, the first occurrence is evaluated and later occurrences are
duplicates. Accepted entries preserve input order.

Every diagnostic contains a stable code, severity, record identity, and human
message. A field path and posting index are included when the location is
known. Callers must branch on code or severity rather than message text.

Processor-generated codes currently include:

- `record.duplicate_in_batch`
- `record.already_committed`
- `ledger.invalid_entry`
- `ledger.invalid_posting`
- `ledger.invalid_account`
- `ledger.invalid_commodity`
- `ledger.invalid_omission`
- `ledger.decimal_overflow`
- `ledger.commodity_mismatch`
- `ledger.unbalanced_entry`

## Journal projection

The display source is inserted first as the transaction comment
`source: <display source>`. Existing transaction and posting comments retain
their order.

A structured warning is projected as `WARN [<code>]: <message>`. Warnings with
a valid posting index are appended to that posting comment with ` / `;
warnings without a posting index become transaction comments. The structured
diagnostic remains authoritative.

Processing does not mutate the decoded batch. Outcome entries and the accepted
entry list do not share mutable comment or posting slices.

The deterministic report, deduplication state, and safe publication protocol
are documented in [RUNS.md](RUNS.md).
