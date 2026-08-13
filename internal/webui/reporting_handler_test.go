package webui_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

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
