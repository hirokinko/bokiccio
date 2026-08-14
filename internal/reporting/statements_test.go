package reporting

import (
	"errors"
	"testing"
)

func TestBuildCurrentOverviewSeparatesBalancesAndMonthToDateExpenses(t *testing.T) {
	t.Parallel()
	configuration := testConfiguration()
	configuration.FiscalYears[0].OpeningMode = OpeningEntries
	configuration.FiscalYears[0].OpeningEntryIDs = []string{"opening"}
	entries := []Entry{
		entry(t, "opening", "2025-04-01", []postingInput{
			{"資産:普通預金", "100", "JPY", "", ""}, {"純資産:期首", "-100", "JPY", "", ""},
		}),
		entry(t, "reserve", "2025-04-02", []postingInput{
			{"資産:支払予定", "30", "JPY", "", ""}, {"資産:普通預金", "-30", "JPY", "", ""},
		}),
		entry(t, "pay", "2025-04-20", []postingInput{
			{"費用:通信費", "10", "JPY", "", ""}, {"資産:支払予定", "-10", "JPY", "", ""},
		}),
		entry(t, "future-pay", "2025-04-21", []postingInput{
			{"費用:通信費", "20", "JPY", "", ""}, {"資産:支払予定", "-20", "JPY", "", ""},
		}),
	}
	report, err := BuildCurrentOverview(configuration, entries, "2025-04-20", Period{StartDate: "2025-04-01", EndDate: "2025-04-30"})
	if err != nil {
		t.Fatalf("BuildCurrentOverview() error = %v", err)
	}
	if report.AsOf != "2025-04-20" || report.FiscalYear != (Period{StartDate: "2025-04-01", EndDate: "2026-03-31"}) ||
		report.ExpensePeriod != (Period{StartDate: "2025-04-01", EndDate: "2025-04-30"}) || !report.ClassificationComplete {
		t.Fatalf("current overview metadata = %+v", report)
	}
	cash := findStatementRow(t, report.Balances, "JPY", CategoryAsset, "資産:普通預金")
	reserved := findStatementRow(t, report.Balances, "JPY", CategoryAsset, "資産:支払予定")
	expense := findStatementRow(t, report.Expenses, "JPY", CategoryExpense, "費用:通信費")
	if cash.Subtotal.Debit != "70" || reserved.Subtotal.Debit != "20" || expense.Subtotal.Debit != "30" {
		t.Fatalf("current overview cash=%+v reserved=%+v expense=%+v", cash, reserved, expense)
	}
	if findStatementCategory(report.Balances, "JPY", CategoryExpense) != nil ||
		findStatementCategory(report.Expenses, "JPY", CategoryAsset) != nil {
		t.Fatalf("current balances and expenses were mixed: %+v / %+v", report.Balances, report.Expenses)
	}
}

func TestBuildCurrentOverviewKeepsOppositeExpenseAndUnclassifiedWarning(t *testing.T) {
	t.Parallel()
	configuration := testConfiguration()
	entries := []Entry{
		entry(t, "refund", "2025-04-02", []postingInput{
			{"費用:通信費", "-5", "JPY", "", ""}, {"資産:現金", "5", "JPY", "", ""},
		}),
		entry(t, "unknown", "2025-04-03", []postingInput{
			{"確認:仮勘定", "2", "JPY", "", ""}, {"資産:現金", "-2", "JPY", "", ""},
		}),
	}
	report, err := BuildCurrentOverview(configuration, entries, "2025-04-03", Period{StartDate: "2025-04-01", EndDate: "2025-04-30"})
	if err != nil {
		t.Fatalf("BuildCurrentOverview() error = %v", err)
	}
	expense := findStatementRow(t, report.Expenses, "JPY", CategoryExpense, "費用:通信費")
	if expense.Subtotal.Credit != "5" || len(expense.Warnings) != 1 || expense.Warnings[0].Side != "credit" {
		t.Fatalf("opposite expense = %+v", expense)
	}
	unknown := findStatementCategory(report.Balances, "JPY", CategoryUnknown)
	if report.ClassificationComplete || unknown == nil || len(report.Warnings) == 0 || report.Warnings[0].Code != "unclassified_account" {
		t.Fatalf("unclassified current overview = %+v", report)
	}
}

func TestBuildCurrentOverviewAllowsUnbalancedAutomaticOpening(t *testing.T) {
	t.Parallel()
	configuration := twoYearConfiguration()
	entries := []Entry{
		entry(t, "opening", "2024-04-01", []postingInput{
			{"資産:現金", "100", "JPY", "", ""}, {"純資産:期首", "-100", "JPY", "", ""},
		}),
		entry(t, "expense", "2024-05-01", []postingInput{
			{"費用:食費", "20", "JPY", "", ""}, {"資産:現金", "-20", "JPY", "", ""},
		}),
	}
	report, err := BuildCurrentOverview(configuration, entries, "2025-04-01", Period{StartDate: "2024-05-01", EndDate: "2024-05-31"})
	if err != nil {
		t.Fatalf("BuildCurrentOverview() error = %v", err)
	}
	cash := findStatementRow(t, report.Balances, "JPY", CategoryAsset, "資産:現金")
	equity := findStatementCategory(report.Balances, "JPY", CategoryEquity)
	if cash.Subtotal.Debit != "80" || equity == nil || equity.Total.Credit != "100" {
		t.Fatalf("unbalanced automatic current overview = %+v", report.Balances)
	}
	if len(report.Expenses) != 1 {
		t.Fatalf("independent prior-year expense period = %+v", report.Expenses)
	}
	if _, err := BuildCurrentOverview(configuration, entries, "2026-04-01", Period{StartDate: "2024-05-01", EndDate: "2024-05-31"}); !errors.Is(err, ErrInvalidPeriod) {
		t.Fatalf("BuildCurrentOverview(outside configuration) error = %v, want ErrInvalidPeriod", err)
	}
	if _, err := BuildCurrentOverview(configuration, entries, "2025-4-1", Period{StartDate: "2024-05-01", EndDate: "2024-05-31"}); !errors.Is(err, ErrInvalidPeriod) {
		t.Fatalf("BuildCurrentOverview(invalid date) error = %v, want ErrInvalidPeriod", err)
	}
	if _, err := BuildCurrentOverview(configuration, entries, "2025-04-01", Period{StartDate: "2024-05-01", EndDate: "2024-05-30"}); !errors.Is(err, ErrInvalidPeriod) {
		t.Fatalf("BuildCurrentOverview(partial expense month) error = %v, want ErrInvalidPeriod", err)
	}
}

func TestBuildBalanceSheetUsesConfiguredFiscalOpening(t *testing.T) {
	t.Parallel()
	configuration := testConfiguration()
	configuration.FiscalYears[0].OpeningMode = OpeningEntries
	configuration.FiscalYears[0].OpeningEntryIDs = []string{"opening"}
	entries := []Entry{
		entry(t, "opening", "2025-04-01", []postingInput{
			{"資産:現金", "100", "JPY", "", ""}, {"純資産:期首", "-100", "JPY", "", ""},
		}),
		entry(t, "same-day-sale", "2025-04-01", []postingInput{
			{"資産:現金", "20", "JPY", "", ""}, {"収益:売上", "-20", "JPY", "", ""},
		}),
	}
	report, err := BuildBalanceSheet(configuration, entries, Period{StartDate: "2025-04-01", EndDate: "2026-03-31"})
	if err != nil {
		t.Fatalf("BuildBalanceSheet() error = %v", err)
	}
	if report.AsOf != "2025-04-01" || report.ConfigurationRevision != 1 || !report.ClassificationComplete {
		t.Fatalf("balance sheet metadata = %+v", report)
	}
	cash := findStatementRow(t, report.Commodities, "JPY", CategoryAsset, "資産:現金")
	if cash.Direct.Debit != "100" || cash.Direct.Credit != "0" {
		t.Fatalf("cash = %+v", cash)
	}
	if findStatementCategory(report.Commodities, "JPY", CategoryRevenue) != nil {
		t.Fatal("same-day movement was included in opening balance sheet")
	}
	if _, err := BuildBalanceSheet(configuration, entries, Period{StartDate: "2025-04-01", EndDate: "2025-04-30"}); !errors.Is(err, ErrInvalidPeriod) {
		t.Fatalf("BuildBalanceSheet(month) error = %v, want ErrInvalidPeriod", err)
	}
}

func TestBuildBalanceSheetAutomaticCarryRequiresBalancedOpening(t *testing.T) {
	t.Parallel()
	configuration := twoYearConfiguration()
	entries := []Entry{
		entry(t, "opening", "2024-04-01", []postingInput{
			{"資産:現金", "100", "JPY", "", ""}, {"純資産:期首", "-100", "JPY", "", ""},
		}),
		entry(t, "expense", "2024-05-01", []postingInput{
			{"費用:食費", "20", "JPY", "", ""}, {"資産:現金", "-20", "JPY", "", ""},
		}),
	}
	selected := Period{StartDate: "2025-04-01", EndDate: "2026-03-31"}
	if _, err := BuildBalanceSheet(configuration, entries, selected); !errors.Is(err, ErrOpeningUnbalanced) {
		t.Fatalf("BuildBalanceSheet(unclosed automatic) error = %v, want ErrOpeningUnbalanced", err)
	}
	entries = append(entries, entry(t, "close", "2025-03-31", []postingInput{
		{"純資産:繰越", "20", "JPY", "", ""}, {"費用:食費", "-20", "JPY", "", ""},
	}))
	report, err := BuildBalanceSheet(configuration, entries, selected)
	if err != nil {
		t.Fatalf("BuildBalanceSheet(closed automatic) error = %v", err)
	}
	cash := findStatementRow(t, report.Commodities, "JPY", CategoryAsset, "資産:現金")
	equity := findStatementCategory(report.Commodities, "JPY", CategoryEquity)
	if cash.Direct.Debit != "80" || equity == nil || equity.Total.Credit != "80" {
		t.Fatalf("automatic balance sheet = %+v", report.Commodities)
	}
}

func TestBuildIncomeStatementUsesOnlySelectedMonthAndKeepsUnknown(t *testing.T) {
	t.Parallel()
	configuration := testConfiguration()
	entries := []Entry{
		entry(t, "previous", "2025-03-31", []postingInput{
			{"資産:現金", "50", "JPY", "", ""}, {"収益:売上", "-50", "JPY", "", ""},
		}),
		entry(t, "sale", "2025-04-02", []postingInput{
			{"資産:現金", "300", "JPY", "", ""}, {"収益:売上", "-300", "JPY", "", ""},
		}),
		entry(t, "expense", "2025-04-03", []postingInput{
			{"費用:食費", "120", "JPY", "", ""}, {"資産:現金", "-120", "JPY", "", ""},
		}),
		entry(t, "unknown", "2025-04-04", []postingInput{
			{"確認:仮勘定", "5", "JPY", "", ""}, {"資産:現金", "-5", "JPY", "", ""},
		}),
		entry(t, "next", "2025-05-01", []postingInput{
			{"費用:食費", "40", "JPY", "", ""}, {"資産:現金", "-40", "JPY", "", ""},
		}),
	}
	report, err := BuildIncomeStatement(configuration, entries, Period{StartDate: "2025-04-01", EndDate: "2025-04-30"})
	if err != nil {
		t.Fatalf("BuildIncomeStatement() error = %v", err)
	}
	if report.ClassificationComplete || len(report.Warnings) != 1 || report.Warnings[0].Code != "unclassified_account" {
		t.Fatalf("income statement warnings=%+v complete=%v", report.Warnings, report.ClassificationComplete)
	}
	if len(report.Commodities) != 1 || report.Commodities[0].NetIncome.Credit != "180" {
		t.Fatalf("income statement commodities = %+v", report.Commodities)
	}
	revenue := findStatementCategory([]StatementCommoditySection{report.Commodities[0].StatementCommoditySection}, "JPY", CategoryRevenue)
	expense := findStatementCategory([]StatementCommoditySection{report.Commodities[0].StatementCommoditySection}, "JPY", CategoryExpense)
	unknown := findStatementCategory([]StatementCommoditySection{report.Commodities[0].StatementCommoditySection}, "JPY", CategoryUnknown)
	if revenue == nil || revenue.Total.Credit != "300" || expense == nil || expense.Total.Debit != "120" ||
		unknown == nil || unknown.Total.Debit != "5" {
		t.Fatalf("income statement groups = %+v", report.Commodities[0].Groups)
	}
	if _, err := BuildIncomeStatement(configuration, entries, Period{StartDate: "2025-04-01", EndDate: "2026-03-31"}); !errors.Is(err, ErrInvalidPeriod) {
		t.Fatalf("BuildIncomeStatement(year) error = %v, want ErrInvalidPeriod", err)
	}
}

func TestBuildIncomeStatementWarnsOnOppositeBalance(t *testing.T) {
	t.Parallel()
	configuration := testConfiguration()
	entries := []Entry{entry(t, "refund", "2025-04-02", []postingInput{
		{"収益:売上", "10", "JPY", "", ""}, {"資産:現金", "-10", "JPY", "", ""},
	})}
	report, err := BuildIncomeStatement(configuration, entries, Period{StartDate: "2025-04-01", EndDate: "2025-04-30"})
	if err != nil {
		t.Fatalf("BuildIncomeStatement() error = %v", err)
	}
	row := findStatementRow(t, []StatementCommoditySection{report.Commodities[0].StatementCommoditySection}, "JPY", CategoryRevenue, "収益:売上")
	if row.Direct.Debit != "10" || len(row.Warnings) != 1 || row.Warnings[0].Side != "debit" {
		t.Fatalf("opposite revenue row = %+v", row)
	}
}

func TestBuildBalanceTrendReturnsTwelveAlignedFullBalancePoints(t *testing.T) {
	t.Parallel()
	configuration := testConfiguration()
	configuration.FiscalYears[0].OpeningMode = OpeningEntries
	configuration.FiscalYears[0].OpeningEntryIDs = []string{"opening"}
	entries := []Entry{
		entry(t, "opening", "2025-04-01", []postingInput{
			{"資産:現金", "100", "JPY", "", ""}, {"純資産:期首", "-100", "JPY", "", ""},
		}),
		entry(t, "sale", "2025-04-30", []postingInput{
			{"資産:売掛金", "30", "JPY", "", ""}, {"収益:売上", "-30", "JPY", "", ""},
		}),
		entry(t, "collect", "2025-05-01", []postingInput{
			{"資産:現金", "30", "JPY", "", ""}, {"資産:売掛金", "-30", "JPY", "", ""},
		}),
		entry(t, "expense", "2025-05-31", []postingInput{
			{"費用:食費", "20", "JPY", "", ""}, {"資産:現金", "-20", "JPY", "", ""},
		}),
		entry(t, "unknown", "2025-06-30", []postingInput{
			{"確認:仮勘定", "5", "JPY", "", ""}, {"資産:現金", "-5", "JPY", "", ""},
		}),
	}
	report, err := BuildBalanceTrend(configuration, entries, Period{StartDate: "2025-04-01", EndDate: "2026-03-31"})
	if err != nil {
		t.Fatalf("BuildBalanceTrend() error = %v", err)
	}
	if len(report.Points) != 12 || report.Points[10].Period.EndDate != "2026-02-28" || report.ClassificationComplete {
		t.Fatalf("balance trend metadata = %+v", report)
	}
	aprilReceivable := findStatementRow(t, report.Points[0].Commodities, "JPY", CategoryAsset, "資産:売掛金")
	mayReceivable := findStatementRow(t, report.Points[1].Commodities, "JPY", CategoryAsset, "資産:売掛金")
	if aprilReceivable.Direct.Debit != "30" || mayReceivable.Direct.Debit != "0" || mayReceivable.Direct.Credit != "0" {
		t.Fatalf("aligned receivable april=%+v may=%+v", aprilReceivable, mayReceivable)
	}
	mayRevenue := findStatementCategory(report.Points[1].Commodities, "JPY", CategoryRevenue)
	mayExpense := findStatementCategory(report.Points[1].Commodities, "JPY", CategoryExpense)
	if mayRevenue == nil || mayRevenue.Total.Credit != "30" || mayExpense == nil || mayExpense.Total.Debit != "20" {
		t.Fatalf("may cumulative balances = %+v", report.Points[1].Commodities)
	}
	if len(report.Warnings) == 0 || report.Warnings[0].PeriodEnd == "" {
		t.Fatalf("trend warnings = %+v", report.Warnings)
	}
	for _, point := range report.Points {
		for _, section := range point.Commodities {
			if section.Total.Debit != "0" || section.Total.Credit != "0" {
				t.Fatalf("unbalanced point %s/%s total=%+v", point.Period.EndDate, section.Commodity, section.Total)
			}
		}
	}
}

func TestBuildBalanceTrendRejectsUnbalancedAutomaticOpening(t *testing.T) {
	t.Parallel()
	configuration := twoYearConfiguration()
	entries := []Entry{
		entry(t, "opening", "2024-04-01", []postingInput{
			{"資産:現金", "100", "JPY", "", ""}, {"純資産:期首", "-100", "JPY", "", ""},
		}),
		entry(t, "expense", "2024-05-01", []postingInput{
			{"費用:食費", "20", "JPY", "", ""}, {"資産:現金", "-20", "JPY", "", ""},
		}),
	}
	_, err := BuildBalanceTrend(configuration, entries, Period{StartDate: "2025-04-01", EndDate: "2026-03-31"})
	if !errors.Is(err, ErrOpeningUnbalanced) {
		t.Fatalf("BuildBalanceTrend() error = %v, want ErrOpeningUnbalanced", err)
	}
}

func twoYearConfiguration() Configuration {
	configuration := testConfiguration()
	configuration.FiscalYears = []FiscalYear{
		{StartDate: "2024-04-01", EndDate: "2025-03-31", OpeningMode: OpeningEntries, OpeningEntryIDs: []string{"opening"}},
		{StartDate: "2025-04-01", EndDate: "2026-03-31", OpeningMode: OpeningAutomatic},
	}
	return configuration
}

func findStatementRow(t *testing.T, sections []StatementCommoditySection, commodity string, category Category, account string) StatementAccountRow {
	t.Helper()
	group := findStatementCategory(sections, commodity, category)
	if group == nil {
		t.Fatalf("statement category %s/%s not found", commodity, category)
	}
	for _, row := range group.Accounts {
		if row.Account == account {
			return row
		}
	}
	t.Fatalf("statement account %s not found", account)
	return StatementAccountRow{}
}

func findStatementCategory(sections []StatementCommoditySection, commodity string, category Category) *StatementCategoryGroup {
	for _, section := range sections {
		if section.Commodity != commodity {
			continue
		}
		for _, group := range section.Groups {
			if group.Category == category {
				return &group
			}
		}
	}
	return nil
}
