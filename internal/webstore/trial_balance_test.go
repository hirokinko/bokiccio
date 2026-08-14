package webstore

import (
	"context"
	"errors"
	"testing"

	"github.com/hirokinko/bokiccio/internal/reporting"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

func TestTrialBalanceUsesOnlyCurrentApprovedSnapshots(t *testing.T) {
	ctx := context.Background()
	store := New(openBackupTestDatabase(t))
	result, err := store.Import(ctx, []byte(trialBalanceInput))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(ctx, result.RunIdentity)
	if err != nil || len(run.Outcomes) != 6 {
		t.Fatalf("GetRun() error = %v, outcomes = %d", err, len(run.Outcomes))
	}
	ids := make([]string, len(run.Outcomes))
	for index, outcome := range run.Outcomes {
		ids[index] = outcome.EntryID
	}
	zero := 0
	approve := func(index int, revision *int) {
		t.Helper()
		if _, err := store.ApproveRevision(ctx, ids[index], webapp.ApprovalRequest{Revision: revision}); err != nil {
			t.Fatalf("ApproveRevision(entry %d) error = %v", index, err)
		}
	}
	approve(0, &zero)

	twenty := "20"
	latest, err := store.CreateRevision(ctx, ids[1], webapp.RevisionRequest{
		BaseRevision: &zero, OccurredAt: "2025-04-10", Description: "approved current revision",
		Comments: []string{"anonymous approved revision"},
		Postings: []webapp.PostingDetail{
			{Account: "Expenses:Supplies", Amount: &twenty, Commodity: "JPY"},
			{Account: "Assets:Cash"},
		},
	})
	if err != nil || !latest.Valid {
		t.Fatalf("CreateRevision(approved latest) = %+v, error = %v", latest, err)
	}
	approve(1, &latest.Revision)

	approve(2, &zero)
	nine := "9"
	stale, err := store.CreateRevision(ctx, ids[2], webapp.RevisionRequest{
		BaseRevision: &zero, OccurredAt: "2025-04-15", Description: "unapproved current revision",
		Postings: []webapp.PostingDetail{
			{Account: "Expenses:Supplies", Amount: &nine, Commodity: "JPY"},
			{Account: "Assets:Cash"},
		},
	})
	if err != nil || !stale.Valid {
		t.Fatalf("CreateRevision(stale approval) = %+v, error = %v", stale, err)
	}

	approve(3, &zero)
	two, minusOne := "2", "-1"
	invalid, err := store.CreateRevision(ctx, ids[3], webapp.RevisionRequest{
		BaseRevision: &zero, OccurredAt: "2025-04-20", Description: "invalid current revision",
		Postings: []webapp.PostingDetail{
			{Account: "Expenses:Supplies", Amount: &two, Commodity: "JPY"},
			{Account: "Assets:Cash", Amount: &minusOne, Commodity: "JPY"},
		},
	})
	if err != nil || invalid.Valid {
		t.Fatalf("CreateRevision(invalid latest) = %+v, error = %v", invalid, err)
	}
	approve(4, &zero)
	approve(5, &zero)

	request := automaticReportingRequest(&zero, 4, "2025-04-01", "2026-03-31")
	request.Classifications = []webapp.ReportingClassification{
		{Account: "Assets", Category: reporting.CategoryAsset},
		{Account: "Equity", Category: reporting.CategoryEquity},
		{Account: "Revenue", Category: reporting.CategoryRevenue},
		{Account: "Expenses", Category: reporting.CategoryExpense},
	}
	if _, err := store.CreateReportingConfiguration(ctx, request); err != nil {
		t.Fatalf("CreateReportingConfiguration() error = %v", err)
	}

	report, err := store.GetTrialBalance(ctx, reporting.Period{StartDate: "2025-04-01", EndDate: "2025-04-30"})
	if err != nil {
		t.Fatalf("GetTrialBalance() error = %v", err)
	}
	if report.SchemaVersion != webapp.APISchemaVersion || report.ConfigurationRevision != 1 || report.Period.Month != 1 {
		t.Fatalf("trial balance metadata = %+v", report)
	}
	if len(report.Commodities) != 1 || report.Commodities[0].Commodity != "JPY" {
		t.Fatalf("trial balance commodities = %+v", report.Commodities)
	}
	section := report.Commodities[0]
	assertCategoryAmounts(t, section, reporting.CategoryAsset, "0", "0", "105", "20", "85", "0")
	assertCategoryAmounts(t, section, reporting.CategoryEquity, "0", "0", "0", "100", "0", "100")
	assertCategoryAmounts(t, section, reporting.CategoryRevenue, "0", "0", "0", "5", "0", "5")
	assertCategoryAmounts(t, section, reporting.CategoryExpense, "0", "0", "20", "0", "20", "0")

	if _, err := store.GetTrialBalance(ctx, reporting.Period{StartDate: "2025-04-02", EndDate: "2025-04-30"}); !errors.Is(err, reporting.ErrInvalidPeriod) {
		t.Fatalf("GetTrialBalance(invalid period) error = %v, want ErrInvalidPeriod", err)
	}

	balanceSheet, err := store.GetBalanceSheet(ctx, reporting.Period{StartDate: "2025-04-01", EndDate: "2026-03-31"})
	if err != nil || balanceSheet.SchemaVersion != webapp.APISchemaVersion || len(balanceSheet.Commodities) != 0 {
		t.Fatalf("GetBalanceSheet() = %+v, error = %v", balanceSheet, err)
	}
	incomeStatement, err := store.GetIncomeStatement(ctx, reporting.Period{StartDate: "2025-04-01", EndDate: "2025-04-30"})
	if err != nil || incomeStatement.SchemaVersion != webapp.APISchemaVersion || len(incomeStatement.Commodities) != 1 ||
		incomeStatement.Commodities[0].NetIncome.Debit != "15" {
		t.Fatalf("GetIncomeStatement() = %+v, error = %v", incomeStatement, err)
	}
	balanceTrend, err := store.GetBalanceTrend(ctx, reporting.Period{StartDate: "2025-04-01", EndDate: "2026-03-31"})
	if err != nil || balanceTrend.SchemaVersion != webapp.APISchemaVersion || len(balanceTrend.Points) != 12 ||
		len(balanceTrend.Points[0].Commodities) != 1 {
		t.Fatalf("GetBalanceTrend() = %+v, error = %v", balanceTrend, err)
	}
	current, err := store.GetCurrentOverview(ctx, "2025-04-10", reporting.Period{StartDate: "2025-04-01", EndDate: "2025-04-30"})
	if err != nil || current.SchemaVersion != webapp.APISchemaVersion || current.AsOf != "2025-04-10" ||
		len(current.Balances) != 1 || len(current.Expenses) != 1 {
		t.Fatalf("GetCurrentOverview() = %+v, error = %v", current, err)
	}
	currentAssets := findStatementCategory(current.Balances, "JPY", reporting.CategoryAsset)
	currentExpenses := findStatementCategory(current.Expenses, "JPY", reporting.CategoryExpense)
	if currentAssets == nil || currentAssets.Total.Debit != "80" || currentExpenses == nil || currentExpenses.Total.Debit != "20" {
		t.Fatalf("current overview categories = %+v / %+v", current.Balances, current.Expenses)
	}
}

func TestTrialBalanceRequiresReportingConfiguration(t *testing.T) {
	store := New(openBackupTestDatabase(t))
	ctx := context.Background()
	period := reporting.Period{
		StartDate: "2025-04-01", EndDate: "2025-04-30",
	}
	_, err := store.GetTrialBalance(ctx, period)
	if !errors.Is(err, webapp.ErrReportingNotConfigured) {
		t.Fatalf("GetTrialBalance() error = %v, want ErrReportingNotConfigured", err)
	}
	if _, err := store.GetBalanceSheet(ctx, period); !errors.Is(err, webapp.ErrReportingNotConfigured) {
		t.Fatalf("GetBalanceSheet() error = %v, want ErrReportingNotConfigured", err)
	}
	if _, err := store.GetIncomeStatement(ctx, period); !errors.Is(err, webapp.ErrReportingNotConfigured) {
		t.Fatalf("GetIncomeStatement() error = %v, want ErrReportingNotConfigured", err)
	}
	if _, err := store.GetBalanceTrend(ctx, period); !errors.Is(err, webapp.ErrReportingNotConfigured) {
		t.Fatalf("GetBalanceTrend() error = %v, want ErrReportingNotConfigured", err)
	}
	if _, err := store.GetCurrentOverview(ctx, "2025-04-01", period); !errors.Is(err, webapp.ErrReportingNotConfigured) {
		t.Fatalf("GetCurrentOverview() error = %v, want ErrReportingNotConfigured", err)
	}
}

func findStatementCategory(sections []reporting.StatementCommoditySection, commodity string, category reporting.Category) *reporting.StatementCategoryGroup {
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

func assertCategoryAmounts(t *testing.T, section reporting.CommoditySection, category reporting.Category,
	openingDebit, openingCredit, debitTurnover, creditTurnover, closingDebit, closingCredit string,
) {
	t.Helper()
	for _, group := range section.Groups {
		if group.Category != category {
			continue
		}
		got := group.Total
		if got.Opening.Debit != openingDebit || got.Opening.Credit != openingCredit ||
			got.DebitTurnover != debitTurnover || got.CreditTurnover != creditTurnover ||
			got.Closing.Debit != closingDebit || got.Closing.Credit != closingCredit {
			t.Fatalf("%s amounts = %+v", category, got)
		}
		return
	}
	t.Fatalf("category %s not found in %+v", category, section.Groups)
}

const trialBalanceInput = `{
  "schema_version": 1,
  "records": [
    {
      "source": {"namespace": "test", "display": "entry-0"},
      "occurred_at": "2025-04-01",
      "description": "approved original",
      "comments": ["anonymous fixture"],
      "postings": [
        {"account": "Assets:Cash", "amount": "100", "commodity": "JPY"},
        {"account": "Equity:Opening", "amount": "-100", "commodity": "JPY"}
      ]
    },
    {
      "source": {"namespace": "test", "display": "entry-1"},
      "occurred_at": "2025-04-10",
      "description": "revision candidate",
      "postings": [
        {"account": "Expenses:Supplies", "amount": "1", "commodity": "JPY"},
        {"account": "Assets:Cash", "amount": "-1", "commodity": "JPY"}
      ]
    },
    {
      "source": {"namespace": "test", "display": "entry-2"},
      "occurred_at": "2025-04-15",
      "description": "stale approval candidate",
      "postings": [
        {"account": "Expenses:Supplies", "amount": "7", "commodity": "JPY"},
        {"account": "Assets:Cash", "amount": "-7", "commodity": "JPY"}
      ]
    },
    {
      "source": {"namespace": "test", "display": "entry-3"},
      "occurred_at": "2025-04-20",
      "description": "invalid revision candidate",
      "postings": [
        {"account": "Expenses:Supplies", "amount": "11", "commodity": "JPY"},
        {"account": "Assets:Cash", "amount": "-11", "commodity": "JPY"}
      ]
    },
    {
      "source": {"namespace": "test", "display": "entry-4"},
      "occurred_at": "2025-04-30",
      "description": "period end boundary",
      "postings": [
        {"account": "Assets:Cash", "amount": "5", "commodity": "JPY"},
        {"account": "Revenue:Sales", "amount": "-5", "commodity": "JPY"}
      ]
    },
    {
      "source": {"namespace": "test", "display": "entry-5"},
      "occurred_at": "2025-05-01",
      "description": "outside selected period",
      "postings": [
        {"account": "Assets:Cash", "amount": "13", "commodity": "JPY"},
        {"account": "Revenue:Sales", "amount": "-13", "commodity": "JPY"}
      ]
    }
  ]
}`
