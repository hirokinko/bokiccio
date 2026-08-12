// Package tacklerfmt imports and exports ledger journal entries as a
// deliberately limited subset of Tackler journal syntax.
//
// The supported subset consists of YYYY-MM-DD dates, timezone-qualified RFC
// 3339 timestamps, quoted descriptions, single-line transaction and posting
// comments, colon-separated accounts, signed fixed-point amounts with an
// explicit commodity, and two or more postings. Timestamps without an offset,
// transaction codes, metadata, prices, costs, tags, and multiple-commodity
// valuation are not emitted or parsed. Compatibility is checked against
// Tackler 26.1.2 or later and the upstream grammar revision documented in
// COMPATIBILITY.md.
package tacklerfmt
