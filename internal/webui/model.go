package webui

import "github.com/hirokinko/bokiccio/internal/webapp"

type indexPageModel struct {
	Entries []entrySummaryModel
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
	Detail   webapp.RunDetail
	Outcomes []outcomePageModel
}

type outcomePageModel struct {
	Detail    webapp.OutcomeDetail
	EntryHref string
}

type errorPageModel struct {
	Status  int
	Title   string
	Message string
}
