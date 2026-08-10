package webapp

import (
	"context"
	"errors"
)

const APISchemaVersion = 1

var (
	ErrNotFound       = errors.New("web resource not found")
	ErrConflict       = errors.New("web resource conflict")
	ErrInvalidRequest = errors.New("invalid web request")
)

type Repository interface {
	Import(context.Context, []byte) (ImportResult, error)
	GetRun(context.Context, string) (RunDetail, error)
	ListEntries(context.Context, int, string) (EntryPage, error)
	GetEntry(context.Context, string) (EntryDetail, error)
}

type ImportResult struct {
	RunIdentity string        `json:"run_identity"`
	HasErrors   bool          `json:"has_errors"`
	Counts      OutcomeCounts `json:"counts"`
	DetailURL   string        `json:"detail_url"`
}

type OutcomeCounts struct {
	Success   int `json:"success"`
	Warning   int `json:"warning"`
	Error     int `json:"error"`
	Duplicate int `json:"duplicate"`
}

type RunDetail struct {
	SchemaVersion      int             `json:"schema_version"`
	RunIdentity        string          `json:"run_identity"`
	InputDigest        string          `json:"input_digest"`
	PreStateGeneration uint64          `json:"pre_state_generation"`
	HasErrors          bool            `json:"has_errors"`
	Outcomes           []OutcomeDetail `json:"outcomes"`
}

type OutcomeDetail struct {
	RecordIndex int                `json:"record_index"`
	Status      string             `json:"status"`
	Identity    Identity           `json:"identity"`
	Source      Source             `json:"source"`
	Diagnostics []DiagnosticDetail `json:"diagnostics"`
	EntryID     string             `json:"entry_id,omitempty"`
}

type Identity struct {
	Kind             string `json:"kind"`
	AlgorithmVersion int    `json:"algorithm_version"`
	Digest           string `json:"digest"`
}

type Source struct {
	Namespace string `json:"namespace"`
	Display   string `json:"display"`
}

type DiagnosticDetail struct {
	Code         string `json:"code"`
	Severity     string `json:"severity"`
	Message      string `json:"message"`
	FieldPath    string `json:"field_path,omitempty"`
	PostingIndex *int   `json:"posting_index,omitempty"`
}

type EntryPage struct {
	SchemaVersion int            `json:"schema_version"`
	Entries       []EntrySummary `json:"entries"`
	NextCursor    string         `json:"next_cursor,omitempty"`
}

type EntrySummary struct {
	ID          string `json:"id"`
	OccurredAt  string `json:"occurred_at"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Source      Source `json:"source"`
}

type EntryDetail struct {
	SchemaVersion int                `json:"schema_version"`
	ID            string             `json:"id"`
	RunIdentity   string             `json:"run_identity"`
	RecordIndex   int                `json:"record_index"`
	OccurredAt    string             `json:"occurred_at"`
	Description   string             `json:"description"`
	Comments      []string           `json:"comments"`
	Postings      []PostingDetail    `json:"postings"`
	Status        string             `json:"status"`
	Source        Source             `json:"source"`
	Diagnostics   []DiagnosticDetail `json:"diagnostics"`
}

type PostingDetail struct {
	Account   string  `json:"account"`
	Amount    *string `json:"amount,omitempty"`
	Commodity string  `json:"commodity,omitempty"`
	Comment   string  `json:"comment,omitempty"`
}
