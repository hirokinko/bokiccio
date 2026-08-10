package ingest

import "github.com/hirokinko/bokiccio/internal/ledger"

type OutcomeStatus string

const (
	OutcomeSuccess   OutcomeStatus = "success"
	OutcomeWarning   OutcomeStatus = "warning"
	OutcomeError     OutcomeStatus = "error"
	OutcomeDuplicate OutcomeStatus = "duplicate"
)

type DiagnosticSeverity string

const (
	SeverityInfo    DiagnosticSeverity = "info"
	SeverityWarning DiagnosticSeverity = "warning"
	SeverityError   DiagnosticSeverity = "error"
)

type Diagnostic struct {
	Code         string
	Severity     DiagnosticSeverity
	Message      string
	Identity     RecordIdentity
	FieldPath    string
	PostingIndex *int
}

type Outcome struct {
	RecordIndex int
	Identity    RecordIdentity
	Source      Source
	Status      OutcomeStatus
	Diagnostics []Diagnostic
	Entry       *ledger.JournalEntry
}

type ProcessResult struct {
	Outcomes []Outcome
	Entries  []ledger.JournalEntry
}
