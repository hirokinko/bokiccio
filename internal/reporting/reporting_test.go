package reporting

import (
	"errors"
	"testing"

	"github.com/hirokinko/bokiccio/internal/ledger"
)

func TestFiscalPeriods(t *testing.T) {
	t.Parallel()
	year := FiscalYear{StartDate: "2023-04-01", EndDate: "2024-03-31", OpeningMode: OpeningAutomatic}
	periods, err := FiscalPeriods(year, 4)
	if err != nil {
		t.Fatalf("FiscalPeriods() error = %v", err)
	}
	if len(periods) != 13 || periods[0].StartDate != year.StartDate || periods[0].EndDate != year.EndDate ||
		periods[11].StartDate != "2024-02-01" || periods[11].EndDate != "2024-02-29" || periods[12].Month != 12 {
		t.Fatalf("FiscalPeriods() = %+v", periods)
	}
}

func TestValidateConfigurationRejectsClassificationOverlap(t *testing.T) {
	t.Parallel()
	configuration := testConfiguration()
	configuration.Classifications = append(configuration.Classifications,
		Classification{Account: "資産:現金", Category: CategoryAsset})
	if err := ValidateConfiguration(configuration); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("ValidateConfiguration() error = %v", err)
	}
}

func TestBuildTrialBalanceSeparatesCommodityAndUsesTotalPrice(t *testing.T) {
	t.Parallel()
	configuration := testConfiguration()
	configuration.FiscalYears[0].OpeningMode = OpeningEntries
	configuration.FiscalYears[0].OpeningEntryIDs = []string{"opening-jpy"}
	entries := []Entry{
		entry(t, "opening-jpy", "2025-04-01", []postingInput{{"資産:現金", "1000", "JPY", "", ""}, {"純資産:期首", "-1000", "JPY", "", ""}}),
		entry(t, "sale", "2025-04-02", []postingInput{{"資産:現金", "300", "JPY", "", ""}, {"収益:売上", "-300", "JPY", "", ""}}),
		entry(t, "fund", "2025-04-03", []postingInput{{"資産:投資", "2", "UNIT", "200", "JPY"}, {"資産:現金", "-200", "JPY", "", ""}}),
		entry(t, "usd", "2025-04-04", []postingInput{{"資産:外貨", "5", "USD", "", ""}, {"純資産:調整", "-5", "USD", "", ""}}),
	}
	report, err := BuildTrialBalance(configuration, entries, Period{StartDate: "2025-04-01", EndDate: "2025-04-30"})
	if err != nil {
		t.Fatalf("BuildTrialBalance() error = %v", err)
	}
	if len(report.Commodities) != 2 || report.Commodities[0].Commodity != "JPY" || report.Commodities[1].Commodity != "USD" {
		t.Fatalf("commodities = %+v", report.Commodities)
	}
	asset := findRow(t, report, "JPY", CategoryAsset, "資産:投資")
	if asset.Direct.DebitTurnover != "200" || asset.Direct.Closing.Debit != "200" {
		t.Fatalf("investment row = %+v", asset)
	}
	cash := findRow(t, report, "JPY", CategoryAsset, "資産:現金")
	if cash.Direct.Opening.Debit != "1000" || cash.Direct.DebitTurnover != "300" ||
		cash.Direct.CreditTurnover != "200" || cash.Direct.Closing.Debit != "1100" {
		t.Fatalf("cash row = %+v", cash)
	}
}

func TestBuildTrialBalanceInfersOmittedAndBuildsHierarchy(t *testing.T) {
	t.Parallel()
	configuration := testConfiguration()
	entries := []Entry{entry(t, "expense", "2025-04-10", []postingInput{
		{"費用:食費:外食", "120.00", "JPY", "", ""}, {"資産:現金", "", "", "", ""},
	})}
	report, err := BuildTrialBalance(configuration, entries, Period{StartDate: "2025-04-01", EndDate: "2025-04-30"})
	if err != nil {
		t.Fatalf("BuildTrialBalance() error = %v", err)
	}
	root := findRow(t, report, "JPY", CategoryExpense, "費用")
	leaf := findRow(t, report, "JPY", CategoryExpense, "費用:食費:外食")
	if root.Direct.DebitTurnover != "0" || root.Subtotal.DebitTurnover != "120.00" || leaf.Depth != 2 {
		t.Fatalf("root=%+v leaf=%+v", root, leaf)
	}
	cash := findRow(t, report, "JPY", CategoryAsset, "資産:現金")
	if cash.Direct.CreditTurnover != "120.00" {
		t.Fatalf("cash row = %+v", cash)
	}
}

func TestBuildTrialBalanceWarnsWithoutExcluding(t *testing.T) {
	t.Parallel()
	configuration := testConfiguration()
	configuration.FiscalYears[0].OpeningMode = OpeningEntries
	configuration.FiscalYears[0].OpeningEntryIDs = []string{"opening-jpy"}
	entries := []Entry{
		entry(t, "opening-jpy", "2025-04-01", []postingInput{{"資産:現金", "-10", "JPY", "", ""}, {"純資産:期首", "10", "JPY", "", ""}}),
		entry(t, "unknown", "2025-04-02", []postingInput{{"確認:仮勘定", "5", "JPY", "", ""}, {"資産:現金", "-5", "JPY", "", ""}}),
	}
	report, err := BuildTrialBalance(configuration, entries, Period{StartDate: "2025-04-01", EndDate: "2025-04-30"})
	if err != nil {
		t.Fatalf("BuildTrialBalance() error = %v", err)
	}
	if report.ClassificationComplete || len(report.Warnings) != 5 {
		t.Fatalf("report warnings=%+v complete=%v", report.Warnings, report.ClassificationComplete)
	}
	unknown := findRow(t, report, "JPY", CategoryUnknown, "確認:仮勘定")
	if unknown.Direct.DebitTurnover != "5" || len(unknown.Warnings) != 1 || unknown.Warnings[0].Code != "unclassified_account" {
		t.Fatalf("unknown row = %+v", unknown)
	}
}

func TestBuildTrialBalanceCanSwitchOpeningModeByYear(t *testing.T) {
	t.Parallel()
	configuration := testConfiguration()
	configuration.FiscalYears = append([]FiscalYear{{
		StartDate: "2024-04-01", EndDate: "2025-03-31", OpeningMode: OpeningEntries,
		OpeningEntryIDs: []string{"opening-2024"},
	}}, configuration.FiscalYears...)
	entries := []Entry{
		entry(t, "opening-2024", "2024-04-01", []postingInput{{"資産:現金", "100", "JPY", "", ""}, {"純資産:期首", "-100", "JPY", "", ""}}),
		entry(t, "expense-2024", "2024-05-01", []postingInput{{"費用:食費", "20", "JPY", "", ""}, {"資産:現金", "-20", "JPY", "", ""}}),
	}
	report, err := BuildTrialBalance(configuration, entries, Period{StartDate: "2025-04-01", EndDate: "2025-04-30"})
	if err != nil {
		t.Fatalf("BuildTrialBalance() error = %v", err)
	}
	cash := findRow(t, report, "JPY", CategoryAsset, "資産:現金")
	if cash.Direct.Opening.Debit != "80" || cash.Direct.DebitTurnover != "0" {
		t.Fatalf("cash row = %+v", cash)
	}
	if findCategory(report, "JPY", CategoryExpense) != nil {
		t.Fatal("expense was carried into next fiscal year")
	}
}

func testConfiguration() Configuration {
	return Configuration{
		Revision: 1, StartMonth: 4,
		Classifications: []Classification{
			{Account: "資産", Category: CategoryAsset},
			{Account: "負債", Category: CategoryLiability},
			{Account: "純資産", Category: CategoryEquity},
			{Account: "収益", Category: CategoryRevenue},
			{Account: "費用", Category: CategoryExpense},
		},
		FiscalYears: []FiscalYear{{
			StartDate: "2025-04-01", EndDate: "2026-03-31", OpeningMode: OpeningAutomatic,
		}},
	}
}

type postingInput struct {
	account, amount, commodity, total, totalCommodity string
}

func entry(t *testing.T, id, date string, postings []postingInput) Entry {
	t.Helper()
	entryTime, err := ledger.ParseEntryTime(date)
	if err != nil {
		t.Fatal(err)
	}
	result := Entry{ID: id, Entry: ledger.JournalEntry{Date: entryTime, Description: id}}
	for _, input := range postings {
		posting := ledger.Posting{Account: input.account}
		if input.amount != "" {
			value, err := ledger.ParseDecimal(input.amount)
			if err != nil {
				t.Fatal(err)
			}
			posting.Amount = &ledger.Amount{Value: value, Commodity: ledger.Commodity(input.commodity)}
		}
		if input.total != "" {
			value, err := ledger.ParseDecimal(input.total)
			if err != nil {
				t.Fatal(err)
			}
			posting.TotalPrice = &ledger.Amount{Value: value, Commodity: ledger.Commodity(input.totalCommodity)}
		}
		result.Entry.Postings = append(result.Entry.Postings, posting)
	}
	return result
}

func findRow(t *testing.T, report TrialBalance, commodity string, category Category, account string) AccountRow {
	t.Helper()
	group := findCategory(report, commodity, category)
	if group == nil {
		t.Fatalf("category %s/%s not found", commodity, category)
	}
	for _, row := range group.Accounts {
		if row.Account == account {
			return row
		}
	}
	t.Fatalf("account %s not found", account)
	return AccountRow{}
}

func findCategory(report TrialBalance, commodity string, category Category) *CategoryGroup {
	for _, section := range report.Commodities {
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
