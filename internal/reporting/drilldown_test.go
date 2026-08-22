package reporting

import "testing"

func TestBuildTrialBalanceDrillDownExplainsOpeningAndMovement(t *testing.T) {
	t.Parallel()
	configuration := testConfiguration()
	configuration.FiscalYears[0].OpeningMode = OpeningEntries
	configuration.FiscalYears[0].OpeningEntryIDs = []string{"opening"}
	entries := []Entry{
		entry(t, "opening", "2025-04-01", []postingInput{
			{"資産:現金", "100", "JPY", "", ""}, {"純資産:期首", "-100", "JPY", "", ""},
		}),
		entry(t, "sale", "2025-04-02", []postingInput{
			{"資産:現金", "30", "JPY", "", ""}, {"収益:売上", "-30", "JPY", "", ""},
		}),
		entry(t, "expense", "2025-04-03", []postingInput{
			{"費用:備品", "20", "JPY", "", ""}, {"資産:現金", "", "", "", ""},
		}),
	}
	result, err := BuildTrialBalanceDrillDown(configuration, entries, DrillDownQuery{
		Period:    Period{StartDate: "2025-04-01", EndDate: "2025-04-30"},
		Commodity: "JPY", Category: CategoryAsset, Account: "資産:現金", Scope: DrillDownDirect,
	})
	if err != nil {
		t.Fatalf("BuildTrialBalanceDrillDown() error = %v", err)
	}
	want := Amounts{
		Opening: Balance{Debit: "100", Credit: "0"}, DebitTurnover: "30", CreditTurnover: "20",
		Closing: Balance{Debit: "110", Credit: "0"},
	}
	if result.Amounts != want || len(result.Entries) != 3 {
		t.Fatalf("drill-down = %+v, want amounts=%+v entries=3", result, want)
	}
	if result.Entries[0].Role != "opening" || result.Entries[0].ID != "opening" ||
		result.Entries[1].Role != "movement" || result.Entries[2].Contributions[0].Amount != "-20" ||
		result.Entries[2].Contributions[0].PostingIndex != 1 {
		t.Fatalf("contribution entries = %+v", result.Entries)
	}
}

func TestBuildTrialBalanceDrillDownCarriesPermanentPostingProvenance(t *testing.T) {
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
	result, err := BuildTrialBalanceDrillDown(configuration, entries, DrillDownQuery{
		Period:    Period{StartDate: "2025-04-01", EndDate: "2025-04-30"},
		Commodity: "JPY", Category: CategoryAsset, Account: "資産:現金", Scope: DrillDownDirect,
	})
	if err != nil {
		t.Fatalf("BuildTrialBalanceDrillDown() error = %v", err)
	}
	if result.Amounts.Opening != (Balance{Debit: "80", Credit: "0"}) || len(result.Entries) != 2 {
		t.Fatalf("automatic-carry drill-down = %+v", result)
	}
	if result.Entries[0].Role != "opening" || result.Entries[1].Role != "opening" ||
		result.Entries[1].Contributions[0].Amount != "-20" {
		t.Fatalf("automatic-carry entries = %+v", result.Entries)
	}
}

func TestBuildIncomeStatementDrillDownUsesSubtreeAndTotalPrice(t *testing.T) {
	t.Parallel()
	configuration := testConfiguration()
	entries := []Entry{
		entry(t, "priced", "2025-04-02", []postingInput{
			{"費用:備品:消耗品", "2", "ITEM", "500.00", "JPY"}, {"資産:現金", "-500.00", "JPY", "", ""},
		}),
		entry(t, "direct", "2025-04-03", []postingInput{
			{"費用", "10", "JPY", "", ""}, {"資産:現金", "-10", "JPY", "", ""},
		}),
	}
	subtree, err := BuildIncomeStatementDrillDown(configuration, entries, DrillDownQuery{
		Period:    Period{StartDate: "2025-04-01", EndDate: "2025-04-30"},
		Commodity: "JPY", Category: CategoryExpense, Account: "費用", Scope: DrillDownSubtree,
	})
	if err != nil {
		t.Fatalf("BuildIncomeStatementDrillDown(subtree) error = %v", err)
	}
	if subtree.Balance != (Balance{Debit: "510.00", Credit: "0"}) || len(subtree.Entries) != 2 ||
		subtree.Entries[0].Contributions[0].Amount != "500.00" || subtree.Entries[0].Contributions[0].Commodity != "JPY" {
		t.Fatalf("subtree drill-down = %+v", subtree)
	}
	direct, err := BuildIncomeStatementDrillDown(configuration, entries, DrillDownQuery{
		Period:    Period{StartDate: "2025-04-01", EndDate: "2025-04-30"},
		Commodity: "JPY", Category: CategoryExpense, Account: "費用", Scope: DrillDownDirect,
	})
	if err != nil {
		t.Fatalf("BuildIncomeStatementDrillDown(direct) error = %v", err)
	}
	if direct.Balance != (Balance{Debit: "10", Credit: "0"}) || len(direct.Entries) != 1 || direct.Entries[0].ID != "direct" {
		t.Fatalf("direct drill-down = %+v", direct)
	}
}

func TestBuildIncomeStatementDrillDownExplainsFullYear(t *testing.T) {
	t.Parallel()
	configuration := testConfiguration()
	entries := []Entry{
		entry(t, "april", "2025-04-02", []postingInput{
			{"費用:備品", "10", "JPY", "", ""}, {"資産:現金", "-10", "JPY", "", ""},
		}),
		entry(t, "may", "2025-05-02", []postingInput{
			{"費用:備品:消耗品", "20", "JPY", "", ""}, {"資産:現金", "-20", "JPY", "", ""},
		}),
		entry(t, "june", "2025-06-02", []postingInput{
			{"費用:備品", "7", "JPY", "", ""}, {"資産:現金", "-7", "JPY", "", ""},
		}),
	}
	result, err := BuildIncomeStatementDrillDown(configuration, entries, DrillDownQuery{
		Period:    Period{StartDate: "2025-04-01", EndDate: "2026-03-31"},
		Commodity: "JPY", Category: CategoryExpense, Account: "費用:備品", Scope: DrillDownSubtree,
	})
	if err != nil {
		t.Fatalf("BuildIncomeStatementDrillDown(full year) error = %v", err)
	}
	if result.Period.Month != 0 || result.Balance != (Balance{Debit: "37", Credit: "0"}) ||
		len(result.Entries) != 3 || result.Entries[0].ID != "april" || result.Entries[1].ID != "may" || result.Entries[2].ID != "june" {
		t.Fatalf("full-year drill-down = %+v", result)
	}
	yearToDate, err := BuildIncomeStatementDrillDown(configuration, entries, DrillDownQuery{
		Period:    Period{StartDate: "2025-04-01", EndDate: "2025-05-31"},
		Commodity: "JPY", Category: CategoryExpense, Account: "費用:備品", Scope: DrillDownSubtree,
	})
	if err != nil {
		t.Fatalf("BuildIncomeStatementDrillDown(year to date) error = %v", err)
	}
	if yearToDate.Period.Month != 2 || yearToDate.Balance != (Balance{Debit: "30", Credit: "0"}) ||
		len(yearToDate.Entries) != 2 || yearToDate.Entries[0].ID != "april" || yearToDate.Entries[1].ID != "may" {
		t.Fatalf("year-to-date drill-down = %+v", yearToDate)
	}
}
