package webui

import (
	"strings"

	"github.com/hirokinko/bokiccio/internal/ledger"
	"github.com/hirokinko/bokiccio/internal/reporting"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

type indexPageModel struct {
	Page          pageContext
	UploadEnabled bool
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
	CanWrite     bool
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
	CanWrite             bool
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

type currentOverviewPageModel struct {
	Page       pageContext
	Configured bool
	SetupHref  string
	FormAction string
	AsOf       string
	Periods    []trialBalancePeriodOption
	Selected   reporting.Period
	Report     *webapp.CurrentOverviewDetail
	FormError  string
}

type balanceSheetPageModel struct {
	Page       pageContext
	Configured bool
	SetupHref  string
	FormAction string
	Periods    []trialBalancePeriodOption
	Selected   reporting.Period
	Report     *webapp.BalanceSheetDetail
	FormError  string
}

type closingBalanceSheetPageModel struct {
	Page       pageContext
	Configured bool
	SetupHref  string
	FormAction string
	Periods    []trialBalancePeriodOption
	Selected   reporting.Period
	Report     *webapp.ClosingBalanceSheetDetail
	FormError  string
}

type incomeStatementPageModel struct {
	Page       pageContext
	Configured bool
	SetupHref  string
	FormAction string
	Periods    []trialBalancePeriodOption
	Selected   reporting.Period
	Report     *webapp.IncomeStatementDetail
	FormError  string
}

type balanceTrendPageModel struct {
	Page       pageContext
	Configured bool
	SetupHref  string
	FormAction string
	Periods    []trialBalancePeriodOption
	Selected   reporting.Period
	Report     *webapp.BalanceTrendDetail
	FormError  string
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

func hasDistinctDirectAmounts(row reporting.AccountRow) bool {
	return row.Direct != row.Subtotal
}

func statementAccountLabel(row reporting.StatementAccountRow) string {
	return strings.Repeat("— ", row.Depth) + row.Label
}

func hasDistinctStatementDirect(row reporting.StatementAccountRow) bool {
	return row.Direct != row.Subtotal
}

func statementAmount(category reporting.Category, balance reporting.Balance) string {
	if decimalTextNonZero(balance.Credit) {
		if category == reporting.CategoryLiability || category == reporting.CategoryEquity || category == reporting.CategoryRevenue {
			return balance.Credit
		}
		return "-" + balance.Credit
	}
	if !decimalTextNonZero(balance.Debit) {
		if balance.Debit != "0" {
			return balance.Debit
		}
		if balance.Credit != "0" {
			return balance.Credit
		}
		return "0"
	}
	if category == reporting.CategoryLiability || category == reporting.CategoryEquity || category == reporting.CategoryRevenue {
		return "-" + balance.Debit
	}
	return balance.Debit
}

func statementActualSide(msg messages, balance reporting.Balance) string {
	if decimalTextNonZero(balance.Credit) {
		return msg.StatementCreditSide
	}
	if decimalTextNonZero(balance.Debit) {
		return msg.StatementDebitSide
	}
	return "—"
}

func decimalTextNonZero(value string) bool {
	decimal, err := ledger.ParseDecimal(value)
	return err != nil || decimal.Sign() != 0
}

func warningSideLabel(msg messages, side string) string {
	if side == "credit" {
		return msg.StatementCreditSide
	}
	if side == "debit" {
		return msg.StatementDebitSide
	}
	return ""
}

func currentBalanceSummaryGroups(section reporting.StatementCommoditySection) []reporting.StatementCategoryGroup {
	categories := []reporting.Category{
		reporting.CategoryAsset, reporting.CategoryLiability, reporting.CategoryEquity,
	}
	result := make([]reporting.StatementCategoryGroup, 0, len(categories)+1)
	for _, category := range categories {
		group := reporting.StatementCategoryGroup{
			Category: category, Total: reporting.Balance{Debit: "0", Credit: "0"}, Accounts: []reporting.StatementAccountRow{},
		}
		for _, candidate := range section.Groups {
			if candidate.Category == category {
				group = candidate
				break
			}
		}
		result = append(result, group)
	}
	for _, group := range section.Groups {
		if group.Category == reporting.CategoryUnknown {
			result = append(result, group)
		}
	}
	return result
}

func closingBalanceSheetStatementCommodity(commodity reporting.ClosingBalanceSheetCommodity) reporting.StatementCommoditySection {
	return reporting.StatementCommoditySection{
		Commodity: commodity.Commodity,
		Total:     commodity.Total,
		Groups:    commodity.Groups,
	}
}
