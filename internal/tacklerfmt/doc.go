// Package tacklerfmt exports ledger journal entries as a deliberately limited
// subset of Tackler journal syntax.
//
// The supported subset consists of YYYY-MM-DD dates, quoted descriptions,
// single-line transaction and posting comments, colon-separated accounts,
// signed fixed-point amounts with an explicit commodity, and two or more
// postings. Timestamps, transaction codes, metadata, prices, costs, tags, and
// multiple-commodity valuation are not emitted.
package tacklerfmt
