// Package ingest decodes provider-independent journal entry candidates into
// application values. It owns the versioned wire contract and stable record
// identity rules, but does not perform journal processing or file I/O.
package ingest
