// Package ingest decodes provider-independent journal entry candidates into
// application values and processes them into validated ledger entries with
// record-level outcomes. It owns the versioned wire, identity, and processing
// contracts, but does not perform file I/O or safe publication.
package ingest
