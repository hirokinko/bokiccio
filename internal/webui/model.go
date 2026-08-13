package webui

import (
	"strings"

	"github.com/hirokinko/bokiccio/internal/reporting"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

type indexPageModel struct {
	Page          pageContext
	Upload        uploadFormModel
	TacklerUpload uploadFormModel
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
	Detail  string
}

type reportingSettingsPageModel struct {
	Page                 pageContext
	Configured           bool
	Form                 reportingConfigurationFormModel
	UnclassifiedAccounts []string
	FormError            string
}

type reportingConfigurationFormModel struct {
	Action          string
	BaseRevision    int
	StartMonth      int
	Classifications []webapp.ReportingClassification
	FiscalYears     []reportingFiscalYearFormModel
}

type reportingFiscalYearFormModel struct {
	StartDate       string
	EndDate         string
	OpeningMode     reporting.OpeningMode
	OpeningEntryIDs string
}

type trialBalancePageModel struct {
	Page       pageContext
	Configured bool
	SetupHref  string
	FormAction string
	Periods    []trialBalancePeriodOption
	Selected   reporting.Period
	Report     *webapp.TrialBalanceDetail
	FormError  string
}

type trialBalancePeriodOption struct {
	Period reporting.Period
	Label  string
}

func reportingCategoryLabel(msg messages, category reporting.Category) string {
	switch category {
	case reporting.CategoryAsset:
		return msg.CategoryAsset
	case reporting.CategoryLiability:
		return msg.CategoryLiability
	case reporting.CategoryEquity:
		return msg.CategoryEquity
	case reporting.CategoryRevenue:
		return msg.CategoryRevenue
	case reporting.CategoryExpense:
		return msg.CategoryExpense
	default:
		return msg.CategoryUnclassified
	}
}

type reportingCategoryOption struct {
	Value reporting.Category
	Label string
}

func reportingCategoryOptions(msg messages) []reportingCategoryOption {
	return []reportingCategoryOption{
		{Value: reporting.CategoryAsset, Label: msg.CategoryAsset},
		{Value: reporting.CategoryLiability, Label: msg.CategoryLiability},
		{Value: reporting.CategoryEquity, Label: msg.CategoryEquity},
		{Value: reporting.CategoryRevenue, Label: msg.CategoryRevenue},
		{Value: reporting.CategoryExpense, Label: msg.CategoryExpense},
	}
}

func accountLabel(row reporting.AccountRow) string {
	return strings.Repeat("— ", row.Depth) + row.Label
}
