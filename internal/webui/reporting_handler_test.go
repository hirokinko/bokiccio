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
	handler := authenticatedUIHandler(t, webui.NewHandler(store))

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

func TestCurrentOverviewHtmxInternalErrorDetailsOnlyInDevelopment(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	if err := database.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	form := url.Values{"as_of": {"2025-04-20"}, "expense_period": {"2025-04-01/2025-04-30"}}
	headers := map[string]string{"HX-Request": "true", "HX-Target": "current-balances-result"}

	production := serveForm(webui.NewHandler(store), "/ui/reports/current", form, headers)
	assertHTMLResponse(t, production, http.StatusInternalServerError)
	assertContainsAll(t, production.Body.String(), []string{`id="current-balances-result"`, "ページを表示できませんでした。"})
	assertNotContainsAny(t, production.Body.String(), []string{"database is closed", "Development error detail", "<!doctype html>"})

	development := serveForm(webui.NewHandler(store, webui.HandlerOptions{Development: true}), "/ui/reports/current", form, headers)
	assertHTMLResponse(t, development, http.StatusInternalServerError)
	assertContainsAll(t, development.Body.String(), []string{
		`id="current-balances-result"`, "Development error detail", "database is closed",
	})
}

func TestTrialBalanceUISelectionWarningsAndLocale(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	handler := authenticatedUIHandler(t, webui.NewHandler(store))

	notConfigured := serve(handler, http.MethodGet, "/reports/trial-balance")
	assertHTMLResponse(t, notConfigured, http.StatusOK)
	assertContainsAll(t, notConfigured.Body.String(), []string{"試算表を表示するには", `href="/settings/reporting"`})
	if response := serveForm(handler, "/ui/settings/reporting", reportingSettingsForm("0"), nil); response.Code != http.StatusSeeOther {
		t.Fatalf("create settings status=%d body=%s", response.Code, response.Body.String())
	}

	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-report","display":"anonymous"},"occurred_at":"2025-04-15","description":"anonymous report fixture","postings":[{"account":"Assets:Cash","amount":"125.00","commodity":"JPY"},{"account":"Revenue:Sales","amount":"-125.00","commodity":"JPY"}]}]}`)
	result, err := store.Import(context.Background(), testUIEmail, input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil || len(run.Outcomes) != 1 {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	zero := 0
	if _, err := store.ApproveRevision(context.Background(), testUIEmail, run.Outcomes[0].EntryID, webapp.ApprovalRequest{Revision: &zero}); err != nil {
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
	handler := authenticatedUIHandler(t, webui.NewHandler(store))

	notConfigured := serve(handler, http.MethodGet, "/reports/balance-sheet")
	assertHTMLResponse(t, notConfigured, http.StatusOK)
	assertContainsAll(t, notConfigured.Body.String(), []string{"先にレポート設定を保存", `href="/settings/reporting"`})
	closingNotConfigured := serve(handler, http.MethodGet, "/en/reports/closing-balance-sheet")
	assertHTMLResponse(t, closingNotConfigured, http.StatusOK)
	assertContainsAll(t, closingNotConfigured.Body.String(), []string{
		"Period-end balance sheet", "Save reporting settings", `href="/en/settings/reporting"`,
	})

	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-statements","display":"anonymous"},"occurred_at":"2025-04-01","description":"anonymous opening fixture","postings":[{"account":"Assets:Cash","amount":"100.00","commodity":"JPY"},{"account":"Equity:Opening","amount":"-100.00","commodity":"JPY"}]},{"source":{"namespace":"ui-statements","display":"anonymous"},"occurred_at":"2025-04-15","description":"anonymous expense fixture","postings":[{"account":"Expenses:Fees","amount":"20.00","commodity":"JPY"},{"account":"Assets:Cash","amount":"-20.00","commodity":"JPY"}]},{"source":{"namespace":"ui-statements","display":"anonymous"},"occurred_at":"2025-04-20","description":"anonymous opposite balance fixture","postings":[{"account":"Revenue:Refund","amount":"5.00","commodity":"JPY"},{"account":"Assets:Cash","amount":"-5.00","commodity":"JPY"}]},{"source":{"namespace":"ui-statements","display":"anonymous"},"occurred_at":"2025-04-25","description":"anonymous unclassified fixture","postings":[{"account":"Review:Suspense","amount":"2.00","commodity":"JPY"},{"account":"Assets:Cash","amount":"-2.00","commodity":"JPY"}]}]}`)
	result, err := store.Import(context.Background(), testUIEmail, input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil || len(run.Outcomes) != 4 {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	zero := 0
	for _, outcome := range run.Outcomes {
		if _, err := store.ApproveRevision(context.Background(), testUIEmail, outcome.EntryID, webapp.ApprovalRequest{Revision: &zero}); err != nil {
			t.Fatalf("ApproveRevision() error = %v", err)
		}
	}
	if _, err := store.CreateReportingConfiguration(context.Background(), testUIEmail, webapp.ReportingConfigurationRequest{
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
		`class="report-navigation"`, `href="/reports/closing-balance-sheet"`, `href="/reports/income-statement"`, `class="statement-table"`,
	})

	closingJA := serve(handler, http.MethodGet, "/reports/closing-balance-sheet?start_date=2025-04-01&end_date=2026-03-31")
	assertHTMLResponse(t, closingJA, http.StatusOK)
	assertContainsAll(t, closingJA.Body.String(), []string{
		"期末貸借対照表", "期末日: 2026-03-31", "保存済み・締め済みの値ではありません",
		"資産", "73.00", "純資産", "100.00", "当期損益（表示補正）", "-25.00", "借方", "貸借差額", "0.00",
		"未分類", "Review", "Suspense", "unclassified_account", "未分類の勘定科目があります",
		`class="current-summary-grid closing-summary-grid"`, `class="current-summary-card closing-adjustment-card"`,
		`class="current-summary-card closing-total-card"`, `class="statement-table"`,
		`action="/ui/reports/closing-balance-sheet"`,
		`<h3>貸借差額</h3><strong>0.00</strong> <span>実際の残高側: —</span>`,
	})
	closingEN := serve(handler, http.MethodGet, "/en/reports/closing-balance-sheet?start_date=2025-04-01&end_date=2026-03-31")
	assertHTMLResponse(t, closingEN, http.StatusOK)
	assertContainsAll(t, closingEN.Body.String(), []string{
		"Period-end balance sheet", "Fiscal year end: 2026-03-31", "not a saved or closed snapshot",
		"Current earnings (presentation adjustment)", "-25.00", "Debit", "Balance difference",
		`action="/en/ui/reports/closing-balance-sheet"`, `href="/en/reports/balance-sheet"`,
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
	selectedClosing := serveForm(handler, "/en/ui/reports/closing-balance-sheet", url.Values{
		"period": {"2025-04-01/2026-03-31"},
	}, nil)
	if selectedClosing.Code != http.StatusSeeOther || selectedClosing.Header().Get("Location") != "/en/reports/closing-balance-sheet?end_date=2026-03-31&start_date=2025-04-01" {
		t.Fatalf("select closing period status=%d location=%q", selectedClosing.Code, selectedClosing.Header().Get("Location"))
	}
	invalid := serve(handler, http.MethodGet, "/reports/income-statement?start_date=private&end_date=2025-04-30")
	assertHTMLResponse(t, invalid, http.StatusBadRequest)
	if strings.Contains(invalid.Body.String(), "private") {
		t.Fatalf("invalid statement period reflected private input: %s", invalid.Body.String())
	}
	invalidClosing := serve(handler, http.MethodGet, "/reports/closing-balance-sheet?start_date=private&end_date=2026-03-31")
	assertHTMLResponse(t, invalidClosing, http.StatusBadRequest)
	if strings.Contains(invalidClosing.Body.String(), "private") {
		t.Fatalf("invalid closing period reflected private input: %s", invalidClosing.Body.String())
	}
	method := serve(handler, http.MethodGet, "/ui/reports/closing-balance-sheet")
	assertHTMLResponse(t, method, http.StatusMethodNotAllowed)
}

func TestClosingBalanceSheetUIDefaultsToLastFiscalYear(t *testing.T) {
	store := webstore.New(openUIDatabase(t))
	handler := authenticatedUIHandler(t, webui.NewHandler(store))
	zero := 0
	if _, err := store.CreateReportingConfiguration(context.Background(), testUIEmail, webapp.ReportingConfigurationRequest{
		BaseRevision: &zero,
		StartMonth:   4,
		FiscalYears: []webapp.ReportingFiscalYear{
			{StartDate: "2025-04-01", EndDate: "2026-03-31", OpeningMode: reporting.OpeningAutomatic},
			{StartDate: "2026-04-01", EndDate: "2027-03-31", OpeningMode: reporting.OpeningAutomatic},
		},
	}); err != nil {
		t.Fatalf("CreateReportingConfiguration() error = %v", err)
	}

	response := serve(handler, http.MethodGet, "/reports/closing-balance-sheet")
	assertHTMLResponse(t, response, http.StatusOK)
	assertContainsAll(t, response.Body.String(), []string{
		"期末日: 2027-03-31", `option value="2026-04-01/2027-03-31" selected`,
	})
}

func TestBalanceSheetUIExplainsUnbalancedAutomaticOpening(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	handler := authenticatedUIHandler(t, webui.NewHandler(store))
	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-unbalanced-opening","display":"anonymous"},"occurred_at":"2025-05-01","description":"anonymous revenue fixture","postings":[{"account":"Assets:Cash","amount":"10","commodity":"JPY"},{"account":"Revenue:Sales","amount":"-10","commodity":"JPY"}]}]}`)
	result, err := store.Import(context.Background(), testUIEmail, input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil || len(run.Outcomes) != 1 {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	zero := 0
	if _, err := store.ApproveRevision(context.Background(), testUIEmail, run.Outcomes[0].EntryID, webapp.ApprovalRequest{Revision: &zero}); err != nil {
		t.Fatalf("ApproveRevision() error = %v", err)
	}
	if _, err := store.CreateReportingConfiguration(context.Background(), testUIEmail, webapp.ReportingConfigurationRequest{
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
	closing := serve(handler, http.MethodGet, "/reports/closing-balance-sheet?start_date=2026-04-01&end_date=2027-03-31")
	assertHTMLResponse(t, closing, http.StatusUnprocessableEntity)
	assertContainsAll(t, closing.Body.String(), []string{
		"期首残高の貸借が一致しません", `href="/settings/reporting"`,
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
	result, err := store.Import(context.Background(), testUIEmail, input)
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
		if _, err := store.ApproveRevision(context.Background(), testUIEmail, outcome.EntryID, webapp.ApprovalRequest{Revision: &zero}); err != nil {
			t.Fatalf("ApproveRevision() error = %v", err)
		}
	}
	if _, err := store.CreateReportingConfiguration(context.Background(), testUIEmail, webapp.ReportingConfigurationRequest{
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
		`hx-post="/en/ui/reports/current"`, `hx-target="#current-balances-result"`, `hx-target="#current-expenses-result"`,
		`data-swap-error-response`,
	})
	jaBody := ja.Body.String()
	balancesHeading := strings.Index(jaBody, `id="current-balances-heading"`)
	balanceDateSelector := strings.Index(jaBody, `<input type="date" name="as_of"`)
	expensesHeading := strings.Index(jaBody, `id="current-expenses-heading"`)
	expenseMonthSelector := strings.Index(jaBody, `<select name="expense_period">`)
	if balancesHeading < 0 || balanceDateSelector < balancesHeading || expensesHeading < balanceDateSelector || expenseMonthSelector < expensesHeading {
		t.Fatalf("current overview selectors are not in their corresponding sections: %s", jaBody)
	}
	if strings.Contains(jaBody, `class="overview-controls"`) {
		t.Fatalf("current overview still renders detached selector controls: %s", jaBody)
	}
	selected := serveForm(handler, "/en/ui/reports/current", url.Values{
		"as_of": {"2025-04-19"}, "expense_period": {"2025-04-01/2025-04-30"},
	}, nil)
	if selected.Code != http.StatusSeeOther || selected.Header().Get("Location") != "/en/reports/current?as_of=2025-04-19&expense_end_date=2025-04-30&expense_start_date=2025-04-01" {
		t.Fatalf("select current date status=%d location=%q", selected.Code, selected.Header().Get("Location"))
	}
	htmxBalance := serveForm(handler, "/en/ui/reports/current", url.Values{
		"as_of": {"2025-04-19"}, "expense_period": {"2025-04-01/2025-04-30"},
	}, map[string]string{"HX-Request": "true", "HX-Target": "current-balances-result"})
	assertHTMLResponse(t, htmxBalance, http.StatusOK)
	assertContainsAll(t, htmxBalance.Body.String(), []string{
		`id="current-balances-result"`, "ScheduledPayment", `id="current-overview-metadata"`, `hx-swap-oob="outerHTML"`,
		`id="current-expenses-as-of"`, `value="2025-04-19"`, "Balance date: 2025-04-19",
	})
	assertNotContainsAny(t, htmxBalance.Body.String(), []string{"<!doctype html>", `<html lang="en">`, `id="current-expenses-heading"`, "Monthly expenses"})
	if got := htmxBalance.Header().Get("HX-Push-Url"); got != "/en/reports/current?as_of=2025-04-19&expense_end_date=2025-04-30&expense_start_date=2025-04-01" {
		t.Fatalf("HX-Push-Url = %q", got)
	}
	if vary := htmxBalance.Header().Get("Vary"); !strings.Contains(vary, "HX-Request") || !strings.Contains(vary, "HX-Target") {
		t.Fatalf("Vary = %q", vary)
	}
	htmxInvalid := serveForm(handler, "/ui/reports/current", url.Values{
		"as_of": {"private"}, "expense_period": {"2025-04-01/2025-04-30"},
	}, map[string]string{"HX-Request": "true", "HX-Target": "current-balances-result"})
	assertHTMLResponse(t, htmxInvalid, http.StatusBadRequest)
	assertContainsAll(t, htmxInvalid.Body.String(), []string{
		`id="current-balances-result"`, `data-swap-error-response`, "設定された会計年度内の残高基準日と費用月を指定してください。",
	})
	assertNotContainsAny(t, htmxInvalid.Body.String(), []string{"private", "<!doctype html>", `id="current-expenses-heading"`})
	expenseSelected := serveForm(handler, "/ui/reports/current", url.Values{
		"as_of": {"2025-04-20"}, "expense_period": {"2025-05-01/2025-05-31"},
	}, nil)
	if expenseSelected.Code != http.StatusSeeOther || expenseSelected.Header().Get("Location") != "/reports/current?as_of=2025-04-20&expense_end_date=2025-05-31&expense_start_date=2025-05-01" {
		t.Fatalf("select expense month status=%d location=%q", expenseSelected.Code, expenseSelected.Header().Get("Location"))
	}
	htmxExpense := serveForm(handler, "/ui/reports/current", url.Values{
		"as_of": {"2025-04-20"}, "expense_period": {"2025-05-01/2025-05-31"},
	}, map[string]string{"HX-Request": "true", "HX-Target": "current-expenses-result"})
	assertHTMLResponse(t, htmxExpense, http.StatusOK)
	assertContainsAll(t, htmxExpense.Body.String(), []string{
		`id="current-expenses-result"`, "2025-05-01 – 2025-05-31", "この期間に計上された費用はありません",
		`id="current-balances-expense-period"`, `value="2025-05-01/2025-05-31"`, `hx-swap-oob="outerHTML"`,
	})
	assertNotContainsAny(t, htmxExpense.Body.String(), []string{"<!doctype html>", `id="current-balances-heading"`, "現在残高"})
	if got := htmxExpense.Header().Get("HX-Push-Url"); got != "/reports/current?as_of=2025-04-20&expense_end_date=2025-05-31&expense_start_date=2025-05-01" {
		t.Fatalf("HX-Push-Url = %q", got)
	}
	htmxInvalidExpense := serveForm(handler, "/en/ui/reports/current", url.Values{
		"as_of": {"2025-04-20"}, "expense_period": {"private"},
	}, map[string]string{"HX-Request": "true", "HX-Target": "current-expenses-result"})
	assertHTMLResponse(t, htmxInvalidExpense, http.StatusBadRequest)
	assertContainsAll(t, htmxInvalidExpense.Body.String(), []string{
		`id="current-expenses-result"`, `data-swap-error-response`, "Select a balance date and expense month within the configured fiscal years.",
	})
	assertNotContainsAny(t, htmxInvalidExpense.Body.String(), []string{"private", "<!doctype html>", `id="current-balances-heading"`})
	may := serve(handler, http.MethodGet, expenseSelected.Header().Get("Location"))
	assertHTMLResponse(t, may, http.StatusOK)
	assertContainsAll(t, may.Body.String(), []string{"残高基準日: 2025-04-20", "2025-05-01 – 2025-05-31", "この期間に計上された費用はありません"})

	invalid := serve(handler, http.MethodGet, "/reports/current?as_of=private&expense_start_date=2025-04-01&expense_end_date=2025-04-30")
	assertHTMLResponse(t, invalid, http.StatusBadRequest)
	assertContainsAll(t, invalid.Body.String(), []string{
		`id="current-balances-heading"`, `<input type="date" name="as_of"`,
		`id="current-expenses-heading"`, `<select name="expense_period">`,
	})
	if strings.Contains(invalid.Body.String(), "private") {
		t.Fatalf("invalid current date reflected private input: %s", invalid.Body.String())
	}
	method := serve(handler, http.MethodGet, "/ui/reports/current")
	assertHTMLResponse(t, method, http.StatusMethodNotAllowed)
}

func TestReportDrillDownUIWorksForReadOnlyViewer(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	result, err := store.Import(context.Background(), testUIEmail, []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-drill-down","display":"anonymous"},"occurred_at":"2025-04-02","description":"anonymous drill-down fixture","postings":[{"account":"Assets:Cash","amount":"25","commodity":"JPY"},{"account":"Revenue:Fees","amount":"-25","commodity":"JPY"}]}]}`))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil || len(run.Outcomes) != 1 {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	zero := 0
	if _, err := store.ApproveRevision(context.Background(), testUIEmail, run.Outcomes[0].EntryID, webapp.ApprovalRequest{Revision: &zero}); err != nil {
		t.Fatalf("ApproveRevision() error = %v", err)
	}
	configuration := webapp.ReportingConfigurationRequest{
		BaseRevision: &zero, StartMonth: 4,
		Classifications: []webapp.ReportingClassification{
			{Account: "Assets", Category: reporting.CategoryAsset},
			{Account: "Revenue", Category: reporting.CategoryRevenue},
		},
		FiscalYears: []webapp.ReportingFiscalYear{{
			StartDate: "2025-04-01", EndDate: "2026-03-31", OpeningMode: reporting.OpeningAutomatic,
		}},
	}
	if _, err := store.CreateReportingConfiguration(context.Background(), testUIEmail, configuration); err != nil {
		t.Fatalf("CreateReportingConfiguration() error = %v", err)
	}
	viewer := authenticatedUIHandlerForEmail(t, webui.NewHandler(store), "viewer@example.com")
	reportPage := serve(viewer, http.MethodGet, "/reports/trial-balance?start_date=2025-04-01&end_date=2025-04-30")
	assertHTMLResponse(t, reportPage, http.StatusOK)
	assertContainsAll(t, reportPage.Body.String(), []string{
		`action="/ui/reports/trial-balance/drill-down"`, `name="account" value="Assets"`,
		`name="scope" value="subtree"`, "根拠の仕訳",
	})
	report, err := store.GetTrialBalance(context.Background(), reporting.Period{StartDate: "2025-04-01", EndDate: "2025-04-30"})
	if err != nil {
		t.Fatalf("GetTrialBalance() error = %v", err)
	}
	form := url.Values{
		"start_date": {"2025-04-01"}, "end_date": {"2025-04-30"},
		"snapshot_identity": {report.SnapshotIdentity}, "commodity": {"JPY"}, "category": {"asset"},
		"account": {"Assets"}, "scope": {"subtree"},
	}
	drillDown := serveForm(viewer, "/ui/reports/trial-balance/drill-down", form, nil)
	assertHTMLResponse(t, drillDown, http.StatusOK)
	assertContainsAll(t, drillDown.Body.String(), []string{
		"集計値の根拠", "配下を含む小計", "anonymous drill-down fixture", "Assets:Cash", "25 JPY",
		`href="/entries/` + run.Outcomes[0].EntryID + `"`,
		`href="/reports/trial-balance?end_date=2025-04-30&amp;start_date=2025-04-01"`,
	})
	if strings.Contains(drillDown.Body.String(), "?account=") {
		t.Fatalf("private account leaked into URL: %s", drillDown.Body.String())
	}

	income, err := store.GetIncomeStatement(context.Background(), reporting.Period{StartDate: "2025-04-01", EndDate: "2025-04-30"})
	if err != nil {
		t.Fatalf("GetIncomeStatement() error = %v", err)
	}
	form.Set("snapshot_identity", income.SnapshotIdentity)
	form.Set("category", "revenue")
	form.Set("account", "Revenue")
	english := serveForm(viewer, "/en/ui/reports/income-statement/drill-down", form, nil)
	assertHTMLResponse(t, english, http.StatusOK)
	assertContainsAll(t, english.Body.String(), []string{
		"Entries behind this amount", "Subtotal including descendants", "anonymous drill-down fixture", "Revenue:Fees", "-25 JPY",
	})

	one := 1
	configuration.BaseRevision = &one
	if _, err := store.CreateReportingConfiguration(context.Background(), testUIEmail, configuration); err != nil {
		t.Fatalf("update reporting configuration error = %v", err)
	}
	stale := serveForm(viewer, "/ui/reports/trial-balance/drill-down", url.Values{
		"start_date": {"2025-04-01"}, "end_date": {"2025-04-30"},
		"snapshot_identity": {report.SnapshotIdentity}, "commodity": {"JPY"}, "category": {"asset"},
		"account": {"Assets"}, "scope": {"subtree"},
	}, nil)
	assertHTMLResponse(t, stale, http.StatusConflict)
	assertContainsAll(t, stale.Body.String(), []string{"レポートが更新されています", "レポートを再表示してください"})
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
