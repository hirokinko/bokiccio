package ingest

import "github.com/hirokinko/bokiccio/internal/ledger"

const SchemaVersion = 1

type Batch struct {
	SchemaVersion int
	Records       []Record
}

type Record struct {
	Source      Source
	Identity    RecordIdentity
	OccurredAt  ledger.EntryTime
	Description string
	Comments    []string
	Postings    []CandidatePosting
}

type Source struct {
	Namespace  string
	Display    string
	ExternalID string
}

type CandidatePosting struct {
	Account string
	Amount  *ledger.Amount
	Comment string
}
