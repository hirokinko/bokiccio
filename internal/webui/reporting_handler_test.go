package webui_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/hirokinko/bokiccio/internal/reporting"
	"github.com/hirokinko/bokiccio/internal/webapp"
	"github.com/hirokinko/bokiccio/internal/webstore"
	"github.com/hirokinko/bokiccio/internal/webui"
)

func TestReportingSettingsUIJapaneseAndEnglish(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	handler := webui.NewHandler(store)

	unsetJA := serve(handler, http.MethodGet, "/settings/reporting")
	unsetEN := serve(handler, http.MethodGet, "/en/settings/reporting")
	assertHTMLResponse(t, unsetJA, http.StatusOK)
	assertHTMLResponse(t, unsetEN, http.StatusOK)
	assertContainsAll(t, unsetJA.Body.String(), []string{"レポート設定", "レポート設定はまだありません", `action="/ui/settings/reporting"`})
	assertContainsAll(t, unsetEN.Body.String(), []string{"Reporting settings", "Reporting is not configured", `action="/en/ui/settings/reporting"`})
	invalidYearForm := reportingSettingsForm("0")
	invalidYearForm.Set("fiscal_start_date", "2025-01-01")
	invalidYearForm.Set("fiscal_end_date", "2025-12-31")
	invalidYear := serveForm(handler, "/ui/settings/reporting", invalidYearForm, nil)
	assertHTMLResponse(t, invalidYear, http.StatusBadRequest)
	if !strings.Contains(invalidYear.Body.String(), "会計年度の日付が開始月と一致しません") {
		t.Fatalf("invalid fiscal year cause is missing: %s", invalidYear.Body.String())
	}

	form := reportingSettingsForm("0")
	created := serveForm(handler, "/ui/settings/reporting", form, nil)
	if created.Code != http.StatusSeeOther || created.Header().Get("Location") != "/settings/reporting" {
		t.Fatalf("create settings status=%d location=%q body=%s", created.Code, created.Header().Get("Location"), created.Body.String())
	}
	page := serve(handler, http.MethodGet, created.Header().Get("Location"))
	assertHTMLResponse(t, page, http.StatusOK)
	body := page.Body.String()
	assertContainsAll(t, body, []string{
		"Current revision: 1", `value="Assets"`, `value="2025-04-01"`,
		"勘定科目の分類を追加", "会計年度を追加", `data-remove-row`,
	})
	classificationHTML := strings.Split(body, `<template id="classification-row-template">`)[0]
	fiscalYearHTML := strings.Split(body, `<template id="fiscal-year-row-template">`)[0]
	if strings.Count(classificationHTML, `value="Assets"`) != 1 || strings.Count(fiscalYearHTML, `value="2025-04-01"`) != 1 {
		t.Fatalf("saved reporting rows were duplicated: %s", body)
	}
	reopened := serve(handler, http.MethodGet, "/settings/reporting")
	assertHTMLResponse(t, reopened, http.StatusOK)
	if strings.Count(strings.Split(reopened.Body.String(), `<template id="classification-row-template">`)[0], `value="Assets"`) != 1 ||
		strings.Count(strings.Split(reopened.Body.String(), `<template id="fiscal-year-row-template">`)[0], `value="2025-04-01"`) != 1 {
		t.Fatalf("reopened reporting rows were duplicated: %s", reopened.Body.String())
	}
	removedAllYears := serveForm(handler, "/ui/settings/reporting", url.Values{
		"base_revision":           {"1"},
		"start_month":             {"4"},
		"classification_account":  {""},
		"classification_category": {"asset"},
		"fiscal_start_date":       {""},
		"fiscal_end_date":         {""},
		"opening_mode":            {"automatic"},
		"opening_entry_ids":       {""},
	}, nil)
	assertHTMLResponse(t, removedAllYears, http.StatusBadRequest)
	if !strings.Contains(removedAllYears.Body.String(), "会計年度を1件以上指定してください") {
		t.Fatalf("removing every fiscal year did not reach domain validation: %s", removedAllYears.Body.String())
	}

	stale := serveForm(handler, "/ui/settings/reporting", form, nil)
	assertHTMLResponse(t, stale, http.StatusConflict)
	if !strings.Contains(stale.Body.String(), "レポート設定が更新されています") {
		t.Fatalf("stale settings body=%s", stale.Body.String())
	}

	malicious := reportingSettingsForm("1")
	malicious.Set("opening_mode", "opening_entries")
	malicious.Set("opening_entry_ids", `</textarea><script>alert("private")</script>`)
	invalid := serveForm(handler, "/ui/settings/reporting", malicious, nil)
	assertHTMLResponse(t, invalid, http.StatusBadRequest)
	if strings.Contains(invalid.Body.String(), `</textarea><script>`) || !strings.Contains(invalid.Body.String(), "&lt;/textarea&gt;") ||
		!strings.Contains(invalid.Body.String(), "現在のrevisionが承認済みではありません") {
		t.Fatalf("reporting form did not escape private input: %s", invalid.Body.String())
	}

	method := serve(handler, http.MethodGet, "/ui/settings/reporting")
	assertHTMLResponse(t, method, http.StatusMethodNotAllowed)
}

func TestReportingInternalErrorDetailsOnlyInDevelopment(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	if err := database.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	production := serve(webui.NewHandler(store), http.MethodGet, "/settings/reporting")
	assertHTMLResponse(t, production, http.StatusInternalServerError)
	if strings.Contains(production.Body.String(), "database is closed") || strings.Contains(production.Body.String(), "Development error detail") {
		t.Fatalf("production response leaked internal detail: %s", production.Body.String())
	}

	development := serve(webui.NewHandler(store, webui.HandlerOptions{Development: true}), http.MethodGet, "/settings/reporting")
	assertHTMLResponse(t, development, http.StatusInternalServerError)
	assertContainsAll(t, development.Body.String(), []string{"Development error detail", "database is closed"})
}

func TestTrialBalanceUISelectionWarningsAndLocale(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	handler := webui.NewHandler(store)

	notConfigured := serve(handler, http.MethodGet, "/reports/trial-balance")
	assertHTMLResponse(t, notConfigured, http.StatusOK)
	assertContainsAll(t, notConfigured.Body.String(), []string{"試算表を表示するには", `href="/settings/reporting"`})
	if response := serveForm(handler, "/ui/settings/reporting", reportingSettingsForm("0"), nil); response.Code != http.StatusSeeOther {
		t.Fatalf("create settings status=%d body=%s", response.Code, response.Body.String())
	}

	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-report","display":"anonymous"},"occurred_at":"2025-04-15","description":"anonymous report fixture","postings":[{"account":"Assets:Cash","amount":"125.00","commodity":"JPY"},{"account":"Revenue:Sales","amount":"-125.00","commodity":"JPY"}]}]}`)
	result, err := store.Import(context.Background(), input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil || len(run.Outcomes) != 1 {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	zero := 0
	if _, err := store.ApproveRevision(context.Background(), run.Outcomes[0].EntryID, webapp.ApprovalRequest{Revision: &zero}); err != nil {
		t.Fatalf("ApproveRevision() error = %v", err)
	}

	reportJA := serve(handler, http.MethodGet, "/reports/trial-balance?start_date=2025-04-01&end_date=2025-04-30")
	reportEN := serve(handler, http.MethodGet, "/en/reports/trial-balance?start_date=2025-04-01&end_date=2025-04-30")
	assertHTMLResponse(t, reportJA, http.StatusOK)
	assertHTMLResponse(t, reportEN, http.StatusOK)
	assertContainsAll(t, reportJA.Body.String(), []string{
		"試算表", "JPY", "125.00", "未分類", "unclassified_account", `href="/settings/reporting"`,
		"各科目の金額は配下を含む小計です", `class="direct-detail-row"`,
		`class="trial-balance-mobile"`, `class="mobile-amount-grid"`, `class="direct-amount-details"`,
	})
	assertContainsAll(t, reportEN.Body.String(), []string{"Trial balance", "JPY", "125.00", "Unclassified", "unclassified_account", `href="/en/settings/reporting"`})
	if strings.Contains(reportJA.Body.String(), `colspan="6"`) {
		t.Fatalf("trial balance kept the wide direct/subtotal column groups: %s", reportJA.Body.String())
	}

	settings := serve(handler, http.MethodGet, "/settings/reporting")
	assertHTMLResponse(t, settings, http.StatusOK)
	assertContainsAll(t, settings.Body.String(), []string{"未分類の勘定科目", "Revenue:Sales"})

	selected := serveForm(handler, "/en/ui/reports/trial-balance", url.Values{
		"period": {"2025-04-01/2025-04-30"},
	}, nil)
	if selected.Code != http.StatusSeeOther || selected.Header().Get("Location") != "/en/reports/trial-balance?end_date=2025-04-30&start_date=2025-04-01" {
		t.Fatalf("select period status=%d location=%q body=%s", selected.Code, selected.Header().Get("Location"), selected.Body.String())
	}
	invalid := serve(handler, http.MethodGet, "/reports/trial-balance?start_date=private&end_date=2025-04-30")
	assertHTMLResponse(t, invalid, http.StatusBadRequest)
	if strings.Contains(invalid.Body.String(), "private") {
		t.Fatalf("invalid period reflected private input: %s", invalid.Body.String())
	}
	method := serve(handler, http.MethodGet, "/ui/reports/trial-balance")
	assertHTMLResponse(t, method, http.StatusMethodNotAllowed)
}

func TestFinancialStatementUIShowsSignedBalancesAndResponsiveTrend(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	handler := webui.NewHandler(store)

	notConfigured := serve(handler, http.MethodGet, "/reports/balance-sheet")
	assertHTMLResponse(t, notConfigured, http.StatusOK)
	assertContainsAll(t, notConfigured.Body.String(), []string{"先にレポート設定を保存", `href="/settings/reporting"`})

	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-statements","display":"anonymous"},"occurred_at":"2025-04-01","description":"anonymous opening fixture","postings":[{"account":"Assets:Cash","amount":"100.00","commodity":"JPY"},{"account":"Equity:Opening","amount":"-100.00","commodity":"JPY"}]},{"source":{"namespace":"ui-statements","display":"anonymous"},"occurred_at":"2025-04-15","description":"anonymous expense fixture","postings":[{"account":"Expenses:Fees","amount":"20.00","commodity":"JPY"},{"account":"Assets:Cash","amount":"-20.00","commodity":"JPY"}]},{"source":{"namespace":"ui-statements","display":"anonymous"},"occurred_at":"2025-04-20","description":"anonymous opposite balance fixture","postings":[{"account":"Revenue:Refund","amount":"5.00","commodity":"JPY"},{"account":"Assets:Cash","amount":"-5.00","commodity":"JPY"}]}]}`)
	result, err := store.Import(context.Background(), input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil || len(run.Outcomes) != 3 {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	zero := 0
	for _, outcome := range run.Outcomes {
		if _, err := store.ApproveRevision(context.Background(), outcome.EntryID, webapp.ApprovalRequest{Revision: &zero}); err != nil {
			t.Fatalf("ApproveRevision() error = %v", err)
		}
	}
	if _, err := store.CreateReportingConfiguration(context.Background(), webapp.ReportingConfigurationRequest{
		BaseRevision: &zero,
		StartMonth:   4,
		Classifications: []webapp.ReportingClassification{
			{Account: "Assets", Category: reporting.CategoryAsset},
			{Account: "Equity", Category: reporting.CategoryEquity},
			{Account: "Expenses", Category: reporting.CategoryExpense},
			{Account: "Revenue", Category: reporting.CategoryRevenue},
		},
		FiscalYears: []webapp.ReportingFiscalYear{{
			StartDate: "2025-04-01", EndDate: "2026-03-31", OpeningMode: reporting.OpeningEntries,
			OpeningEntryIDs: []string{run.Outcomes[0].EntryID},
		}},
	}); err != nil {
		t.Fatalf("CreateReportingConfiguration() error = %v", err)
	}

	bs := serve(handler, http.MethodGet, "/reports/balance-sheet?start_date=2025-04-01&end_date=2026-03-31")
	assertHTMLResponse(t, bs, http.StatusOK)
	assertContainsAll(t, bs.Body.String(), []string{
		"期首貸借対照表", "期首日: 2025-04-01", "Assets", "Cash", "100.00", "借方", "Equity", "Opening", "貸方",
		`class="report-navigation"`, `href="/reports/income-statement"`, `class="statement-table"`,
	})

	pl := serve(handler, http.MethodGet, "/en/reports/income-statement?start_date=2025-04-01&end_date=2025-04-30")
	assertHTMLResponse(t, pl, http.StatusOK)
	assertContainsAll(t, pl.Body.String(), []string{
		"Monthly income statement", "Expenses", "Fees", "20.00", "Revenue", "Refund", "-5.00",
		"opposite_normal_balance", "Debit", "Net income for the month", "-25.00",
		`action="/en/ui/reports/income-statement"`,
	})

	trend := serve(handler, http.MethodGet, "/reports/balance-trend?start_date=2025-04-01&end_date=2026-03-31")
	assertHTMLResponse(t, trend, http.StatusOK)
	assertContainsAll(t, trend.Body.String(), []string{
		"勘定残高推移", "会計年度の期首から累計", `class="balance-trend-grid"`,
		"月次 1: 2025-04-01 – 2025-04-30", "月次 12: 2026-03-01 – 2026-03-31", "-5.00", "opposite_normal_balance", "借方",
	})
	if count := strings.Count(trend.Body.String(), `class="balance-trend-point"`); count != 12 {
		t.Fatalf("balance trend point count = %d, want 12", count)
	}

	selected := serveForm(handler, "/ui/reports/balance-sheet", url.Values{
		"period": {"2025-04-01/2026-03-31"},
	}, nil)
	if selected.Code != http.StatusSeeOther || selected.Header().Get("Location") != "/reports/balance-sheet?end_date=2026-03-31&start_date=2025-04-01" {
		t.Fatalf("select statement period status=%d location=%q", selected.Code, selected.Header().Get("Location"))
	}
	invalid := serve(handler, http.MethodGet, "/reports/income-statement?start_date=private&end_date=2025-04-30")
	assertHTMLResponse(t, invalid, http.StatusBadRequest)
	if strings.Contains(invalid.Body.String(), "private") {
		t.Fatalf("invalid statement period reflected private input: %s", invalid.Body.String())
	}
}

func TestBalanceSheetUIExplainsUnbalancedAutomaticOpening(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	handler := webui.NewHandler(store)
	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-unbalanced-opening","display":"anonymous"},"occurred_at":"2025-05-01","description":"anonymous revenue fixture","postings":[{"account":"Assets:Cash","amount":"10","commodity":"JPY"},{"account":"Revenue:Sales","amount":"-10","commodity":"JPY"}]}]}`)
	result, err := store.Import(context.Background(), input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil || len(run.Outcomes) != 1 {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	zero := 0
	if _, err := store.ApproveRevision(context.Background(), run.Outcomes[0].EntryID, webapp.ApprovalRequest{Revision: &zero}); err != nil {
		t.Fatalf("ApproveRevision() error = %v", err)
	}
	if _, err := store.CreateReportingConfiguration(context.Background(), webapp.ReportingConfigurationRequest{
		BaseRevision: &zero,
		StartMonth:   4,
		Classifications: []webapp.ReportingClassification{
			{Account: "Assets", Category: reporting.CategoryAsset},
			{Account: "Revenue", Category: reporting.CategoryRevenue},
		},
		FiscalYears: []webapp.ReportingFiscalYear{
			{StartDate: "2025-04-01", EndDate: "2026-03-31", OpeningMode: reporting.OpeningAutomatic},
			{StartDate: "2026-04-01", EndDate: "2027-03-31", OpeningMode: reporting.OpeningAutomatic},
		},
	}); err != nil {
		t.Fatalf("CreateReportingConfiguration() error = %v", err)
	}

	response := serve(handler, http.MethodGet, "/reports/balance-sheet?start_date=2026-04-01&end_date=2027-03-31")
	assertHTMLResponse(t, response, http.StatusUnprocessableEntity)
	assertContainsAll(t, response.Body.String(), []string{
		"自動繰越で作成した期首残高の貸借が一致しません", `href="/settings/reporting"`,
	})
}

func TestCurrentOverviewUISelectsBalanceDateAndExpenseMonthIndependently(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	now := func() time.Time { return time.Date(2025, 4, 20, 12, 0, 0, 0, time.UTC) }
	handler := webui.NewHandler(store, webui.HandlerOptions{Now: now})

	notConfigured := serve(handler, http.MethodGet, "/reports/current")
	assertHTMLResponse(t, notConfigured, http.StatusOK)
	assertContainsAll(t, notConfigured.Body.String(), []string{
		"現在残高・月間費用", "先にレポート設定を保存", `href="/settings/reporting"`,
	})

	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-current","display":"anonymous"},"occurred_at":"2025-04-01","description":"anonymous opening fixture","postings":[{"account":"Assets:Cash","amount":"100","commodity":"JPY"},{"account":"Equity:Opening","amount":"-100","commodity":"JPY"}]},{"source":{"namespace":"ui-current","display":"anonymous"},"occurred_at":"2025-04-02","description":"anonymous reservation fixture","postings":[{"account":"Assets:ScheduledPayment","amount":"30","commodity":"JPY"},{"account":"Assets:Cash","amount":"-30","commodity":"JPY"}]},{"source":{"namespace":"ui-current","display":"anonymous"},"occurred_at":"2025-04-20","description":"anonymous expense fixture","postings":[{"account":"Expenses:Communication","amount":"10","commodity":"JPY"},{"account":"Assets:ScheduledPayment","amount":"-10","commodity":"JPY"}]},{"source":{"namespace":"ui-current","display":"anonymous"},"occurred_at":"2025-04-21","description":"anonymous future fixture","postings":[{"account":"Expenses:Communication","amount":"20","commodity":"JPY"},{"account":"Assets:ScheduledPayment","amount":"-20","commodity":"JPY"}]}]}`)
	result, err := store.Import(context.Background(), input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil || len(run.Outcomes) != 4 {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	zero := 0
	for _, outcome := range run.Outcomes {
		if outcome.EntryID == "" {
			t.Fatalf("current overview fixture outcome has no entry: %+v", outcome)
		}
		if _, err := store.ApproveRevision(context.Background(), outcome.EntryID, webapp.ApprovalRequest{Revision: &zero}); err != nil {
			t.Fatalf("ApproveRevision() error = %v", err)
		}
	}
	if _, err := store.CreateReportingConfiguration(context.Background(), webapp.ReportingConfigurationRequest{
		BaseRevision: &zero,
		StartMonth:   4,
		Classifications: []webapp.ReportingClassification{
			{Account: "Assets", Category: reporting.CategoryAsset},
			{Account: "Equity", Category: reporting.CategoryEquity},
			{Account: "Expenses", Category: reporting.CategoryExpense},
		},
		FiscalYears: []webapp.ReportingFiscalYear{{
			StartDate: "2025-04-01", EndDate: "2026-03-31", OpeningMode: reporting.OpeningEntries,
			OpeningEntryIDs: []string{run.Outcomes[0].EntryID},
		}},
	}); err != nil {
		t.Fatalf("CreateReportingConfiguration() error = %v", err)
	}

	ja := serve(handler, http.MethodGet, "/reports/current")
	en := serve(handler, http.MethodGet, "/en/reports/current?as_of=2025-04-20&expense_start_date=2025-04-01&expense_end_date=2025-04-30")
	assertHTMLResponse(t, ja, http.StatusOK)
	assertHTMLResponse(t, en, http.StatusOK)
	assertContainsAll(t, ja.Body.String(), []string{
		"現在残高・月間費用", "残高基準日: 2025-04-20", "現在残高", "月間費用", "費用月", "資産", "負債", "純資産", "Assets", "ScheduledPayment",
		"費用合計", "30", "2025-04-01 – 2025-04-30", `class="current-summary-grid"`,
		`class="current-summary-card expense-summary-card"`, `href="/reports/current"`, "検証用試算表",
	})
	assertContainsAll(t, en.Body.String(), []string{
		"Current balance and expenses", "Balance date: 2025-04-20", "Current balances", "Monthly expenses", "Expense month",
		"ScheduledPayment", "Expense total", `action="/en/ui/reports/current"`, "Verification trial balance",
	})
	selected := serveForm(handler, "/en/ui/reports/current", url.Values{
		"as_of": {"2025-04-19"}, "expense_period": {"2025-04-01/2025-04-30"},
	}, nil)
	if selected.Code != http.StatusSeeOther || selected.Header().Get("Location") != "/en/reports/current?as_of=2025-04-19&expense_end_date=2025-04-30&expense_start_date=2025-04-01" {
		t.Fatalf("select current date status=%d location=%q", selected.Code, selected.Header().Get("Location"))
	}
	expenseSelected := serveForm(handler, "/ui/reports/current", url.Values{
		"as_of": {"2025-04-20"}, "expense_period": {"2025-05-01/2025-05-31"},
	}, nil)
	if expenseSelected.Code != http.StatusSeeOther || expenseSelected.Header().Get("Location") != "/reports/current?as_of=2025-04-20&expense_end_date=2025-05-31&expense_start_date=2025-05-01" {
		t.Fatalf("select expense month status=%d location=%q", expenseSelected.Code, expenseSelected.Header().Get("Location"))
	}
	may := serve(handler, http.MethodGet, expenseSelected.Header().Get("Location"))
	assertHTMLResponse(t, may, http.StatusOK)
	assertContainsAll(t, may.Body.String(), []string{"残高基準日: 2025-04-20", "2025-05-01 – 2025-05-31", "この期間に計上された費用はありません"})

	invalid := serve(handler, http.MethodGet, "/reports/current?as_of=private&expense_start_date=2025-04-01&expense_end_date=2025-04-30")
	assertHTMLResponse(t, invalid, http.StatusBadRequest)
	if strings.Contains(invalid.Body.String(), "private") {
		t.Fatalf("invalid current date reflected private input: %s", invalid.Body.String())
	}
	method := serve(handler, http.MethodGet, "/ui/reports/current")
	assertHTMLResponse(t, method, http.StatusMethodNotAllowed)
}

func reportingSettingsForm(baseRevision string) url.Values {
	return url.Values{
		"base_revision":           {baseRevision},
		"start_month":             {"4"},
		"classification_account":  {"Assets"},
		"classification_category": {"asset"},
		"fiscal_start_date":       {"2025-04-01"},
		"fiscal_end_date":         {"2026-03-31"},
		"opening_mode":            {"automatic"},
		"opening_entry_ids":       {""},
	}
}
