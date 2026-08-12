package webui

import "github.com/hirokinko/bokiccio/internal/webapp"

type indexPageModel struct {
	Page          pageContext
	Upload        uploadFormModel
	Search        searchFormModel
	Export        exportFormModel
	Entries       []entrySummaryModel
	NextCursor    string
	SearchApplied bool
}

type uploadFormModel struct {
	Action string
}

type searchFormModel struct {
	Action    string
	ResetHref string
	Filter    webapp.EntryFilter
}

type exportFormModel struct {
	TacklerAction string
	JSONAction    string
	Filter        webapp.EntryFilter
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
	Page         pageContext
	Detail       webapp.EntryDetail
	Current      candidateModel
	RunHref      string
	RevisionForm revisionFormModel
	ApprovalForm approvalFormModel
	FormError    string
}

type candidateModel struct {
	Revision    int
	OccurredAt  string
	Description string
	Comments    []string
	Postings    []webapp.PostingDetail
	Valid       bool
	Approved    bool
}

type revisionFormModel struct {
	Action       string
	BaseRevision int
	EntryText    string
}

type approvalFormModel struct {
	Action     string
	Revision   int
	CanApprove bool
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
