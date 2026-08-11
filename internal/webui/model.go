package webui

import "github.com/hirokinko/bokiccio/internal/webapp"

type indexPageModel struct {
	Page          pageContext
	Search        searchFormModel
	Entries       []entrySummaryModel
	NextCursor    string
	SearchApplied bool
}

type searchFormModel struct {
	Action    string
	ResetHref string
	Filter    webapp.EntryFilter
}

type entrySummaryModel struct {
	Href            string
	OccurredAt      string
	Description     string
	Status          string
	WorkflowStatus  string
	CurrentRevision int
	Source          webapp.Source
}

type entryPageModel struct {
	Page    pageContext
	Detail  webapp.EntryDetail
	Current candidateModel
	RunHref string
}

type candidateModel struct {
	Revision    int
	OccurredAt  string
	Description string
	Comments    []string
	Postings    []webapp.PostingDetail
}

type runPageModel struct {
	Page     pageContext
	Detail   webapp.RunDetail
	Outcomes []outcomePageModel
}

type outcomePageModel struct {
	Detail    webapp.OutcomeDetail
	EntryHref string
}

type errorPageModel struct {
	Page    pageContext
	Status  int
	Title   string
	Message string
}
