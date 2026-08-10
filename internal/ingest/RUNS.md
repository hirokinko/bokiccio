# Run artifact and publication contract

`ingest.Build` deterministically turns normalized input bytes and committed
state into a report, an optional Tackler journal, and the next state. It does
not access the file system. `ingest.Run` loads and publishes those artifacts
under an explicit output root.

Both report and state use schema version `1`. Decoders reject unknown fields,
unknown schema versions, unsupported identity algorithms, non-lowercase
SHA-256 digests, and non-canonical state identity ordering.

## Run identity

The run identity is a versioned SHA-256 digest of:

- the exact normalized input bytes,
- workflow identity version `1`,
- the effective Tackler omitted-amount setting, and
- the pre-run state generation.

It does not contain the current time, output root, or an absolute path. Equal
input bytes, settings, and state generation therefore produce byte-identical
reports, journals, state, and run identity.

## Report v1

`report.json` records the run identity, input digest, pre-run state generation,
and every record outcome in input order. Each outcome contains its stable
record identity, display source, status, and ordered diagnostics. Diagnostics
repeat the record identity so they remain self-identifying when consumed
separately from their outcome. Generated journal entries are not duplicated in
the report.

The report contains no generated timestamp. `source.external_id` is represented
only through its stable digest and is not copied into the report.

## State v1

`state-v1.json` contains a generation and a canonically sorted unique set of
record identities. Only `success` and `warning` identities are added. The
generation advances once when a run adds at least one identity; `error` and
`duplicate` outcomes do not advance it.

## Output layout and commit protocol

```text
<output-root>/
  state-v1.json
  runs/
    <run-identity-digest>/
      report.json
      journal.txn       # only when at least one entry was accepted
```

Files are completed through a temporary file in their destination directory.
A new bundle is first assembled in a temporary directory and then renamed to
its immutable run path. The state manifest is atomically replaced only after
the bundle is complete, and is the commit point for newly accepted identities.

If publication stops before the state commit, retrying with the same input
recreates the same run identity. An already published bundle is reused only
when every expected byte matches; conflicting content returns
`ErrBundleConflict`. A duplicate-only or all-error run publishes a report but
does not create `journal.txn` or advance state.
