# Tackler compatibility contract

This package emits a subset of the Tackler journal format. It does not claim
full format compatibility.

The compatibility reference is:

- Tackler CLI: `26.1.2`
- Tackler grammar commit: `0641bb09b7cc52bd037c6f6ce4cc377fb72facec`
- Grammar files: `docs/devel/antlr/TxnLexer.g4` and `TxnParser.g4`

## Supported syntax

- `YYYY-MM-DD` transaction dates
- descriptions introduced by two spaces and a single quote
- single-line transaction and posting comments introduced by `; `
- Tackler-compatible colon-separated account identifiers
- signed integer or fixed-point amounts with an explicit commodity
- two or more postings in input order
- an amount omitted only from the final posting
- exactly one blank line between exported transactions and a final newline

Timestamps, transaction codes, metadata, tags, prices, costs, commodity
conversion, and parsing Tackler journals are outside this subset.

## Fixtures

`testdata/compatibility/manifest.json` records each syntax feature, whether
Tackler must accept or reject it, and its single-transaction fixture. Fixtures
use invented names and values and are intended to be reusable as a future
Tree-sitter corpus. The integration check also loads
`testdata/explicit_entries.golden`, which is byte-compared with exporter output
by the normal test suite.

The normal test suite validates the manifest, fixture coverage, UTF-8 line
shape, and single-transaction organization without requiring Tackler:

```sh
go test ./...
```

The pinned CLI compatibility check is opt-in:

```sh
go test -tags=tackler_integration ./internal/tacklerfmt
```
