package webapp_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/hirokinko/bokiccio/internal/webapp"
	"github.com/hirokinko/bokiccio/internal/webstore"
)

func TestReportingConfigurationAPIHistoryAndValidation(t *testing.T) {
	handler := authenticatedAPIHandler(t, webstore.New(openDatabase(t)))

	assertProblem(t, request(t, handler, http.MethodGet, "/api/v1/reporting/configuration", nil, ""),
		http.StatusConflict, "reporting_not_configured")
	first := requestJSON(t, handler, http.MethodPost, "/api/v1/reporting/configuration", map[string]any{
		"base_revision": 0,
		"start_month":   4,
		"classifications": []map[string]any{
			{"account": "Assets", "category": "asset"},
			{"account": "Liabilities", "category": "liability"},
			{"account": "Revenue", "category": "revenue"},
			{"account": "Expenses", "category": "expense"},
		},
		"fiscal_years": []map[string]any{{
			"start_date": "2025-04-01", "end_date": "2026-03-31",
			"opening_mode": "automatic", "opening_entry_ids": []string{},
		}},
	})
	if first.Code != http.StatusCreated {
		t.Fatalf("create reporting configuration status=%d body=%s", first.Code, first.Body.String())
	}
	var created webapp.ReportingConfigurationDetail
	decodeJSON(t, first.Body.Bytes(), &created)
	if created.SchemaVersion != webapp.APISchemaVersion || created.Revision != 1 || created.BaseRevision != 0 || created.CreatedAt == "" {
		t.Fatalf("created reporting configuration = %+v", created)
	}

	current := request(t, handler, http.MethodGet, "/api/v1/reporting/configuration", nil, "")
	var currentDetail webapp.ReportingConfigurationDetail
	decodeJSON(t, current.Body.Bytes(), &currentDetail)
	if current.Code != http.StatusOK || currentDetail.Revision != 1 || currentDetail.StartMonth != 4 {
		t.Fatalf("current configuration status=%d detail=%+v", current.Code, currentDetail)
	}
	historical := request(t, handler, http.MethodGet, "/api/v1/reporting/configurations/1", nil, "")
	var historicalDetail webapp.ReportingConfigurationDetail
	decodeJSON(t, historical.Body.Bytes(), &historicalDetail)
	if historical.Code != http.StatusOK || historicalDetail.Revision != 1 {
		t.Fatalf("historical configuration status=%d detail=%+v", historical.Code, historicalDetail)
	}

	stale := requestJSON(t, handler, http.MethodPost, "/api/v1/reporting/configuration", map[string]any{
		"base_revision": 0,
		"start_month":   4,
		"classifications": []map[string]any{
			{"account": "Assets", "category": "asset"},
		},
		"fiscal_years": []map[string]any{{
			"start_date": "2025-04-01", "end_date": "2026-03-31",
			"opening_mode": "automatic", "opening_entry_ids": []string{},
		}},
	})
	assertProblem(t, stale, http.StatusConflict, "conflict")
	assertProblem(t, requestJSON(t, handler, http.MethodPost, "/api/v1/reporting/configuration", map[string]any{
		"base_revision": 1, "start_month": 13, "classifications": []any{}, "fiscal_years": []any{},
	}), http.StatusBadRequest, "invalid_request")
	assertProblem(t, request(t, handler, http.MethodGet, "/api/v1/reporting/configurations/01", nil, ""),
		http.StatusNotFound, "not_found")
	assertProblem(t, request(t, handler, http.MethodGet, "/api/v1/reporting/configurations/2", nil, ""),
		http.StatusNotFound, "not_found")
	assertProblem(t, request(t, handler, http.MethodPost, "/api/v1/reporting/configurations/1", []byte(`{}`), "application/json"),
		http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestTrialBalanceAPIEmptyAndMultipleCommodities(t *testing.T) {
	handler := authenticatedAPIHandler(t, webstore.New(openDatabase(t)))
	createResponse := requestJSON(t, handler, http.MethodPost, "/api/v1/reporting/configuration", map[string]any{
		"base_revision": 0,
		"start_month":   4,
		"classifications": []map[string]any{
			{"account": "Assets", "category": "asset"},
			{"account": "Revenue", "category": "revenue"},
		},
		"fiscal_years": []map[string]any{{
			"start_date": "2025-04-01", "end_date": "2026-03-31",
			"opening_mode": "automatic", "opening_entry_ids": []string{},
		}},
	})
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create reporting configuration status=%d body=%s", createResponse.Code, createResponse.Body.String())
	}

	periodPath := "/api/v1/reports/trial-balance?start_date=2025-04-01&end_date=2025-04-30"
	emptyResponse := request(t, handler, http.MethodGet, periodPath, nil, "")
	var empty webapp.TrialBalanceDetail
	decodeJSON(t, emptyResponse.Body.Bytes(), &empty)
	if emptyResponse.Code != http.StatusOK || empty.SchemaVersion != webapp.APISchemaVersion ||
		empty.ConfigurationRevision != 1 || empty.Period.Month != 1 || empty.Commodities == nil || len(empty.Commodities) != 0 {
		t.Fatalf("empty trial balance status=%d detail=%+v", emptyResponse.Code, empty)
	}

	input := []byte(`{
  "schema_version": 1,
  "records": [
    {"source":{"namespace":"report-api","display":"jpy"},"occurred_at":"2025-04-15","description":"JPY fixture","postings":[{"account":"Assets:Cash","amount":"100","commodity":"JPY"},{"account":"Revenue:Sales","amount":"-100","commodity":"JPY"}]},
    {"source":{"namespace":"report-api","display":"usd"},"occurred_at":"2025-04-16","description":"USD fixture","postings":[{"account":"Assets:Cash","amount":"2.50","commodity":"USD"},{"account":"Revenue:Sales","amount":"-2.50","commodity":"USD"}]}
  ]
}`)
	result := postImport(t, handler, input)
	runResponse := request(t, handler, http.MethodGet, result.DetailURL, nil, "")
	var run webapp.RunDetail
	decodeJSON(t, runResponse.Body.Bytes(), &run)
	if runResponse.Code != http.StatusOK || len(run.Outcomes) != 2 {
		t.Fatalf("report fixture import status=%d run=%+v", runResponse.Code, run)
	}
	for _, outcome := range run.Outcomes {
		approveEntry(t, handler, outcome.EntryID, 0)
	}

	reportResponse := request(t, handler, http.MethodGet, periodPath, nil, "")
	var report webapp.TrialBalanceDetail
	decodeJSON(t, reportResponse.Body.Bytes(), &report)
	if reportResponse.Code != http.StatusOK || len(report.Commodities) != 2 ||
		report.Commodities[0].Commodity != "JPY" || report.Commodities[1].Commodity != "USD" {
		t.Fatalf("multi-commodity trial balance status=%d detail=%+v", reportResponse.Code, report)
	}
	if report.Commodities[0].Total.DebitTurnover != "100" || report.Commodities[0].Total.CreditTurnover != "100" ||
		report.Commodities[1].Total.DebitTurnover != "2.50" || report.Commodities[1].Total.CreditTurnover != "2.50" {
		t.Fatalf("multi-commodity totals = %+v", report.Commodities)
	}

	balanceSheetResponse := request(t, handler, http.MethodGet,
		"/api/v1/reports/balance-sheet?start_date=2025-04-01&end_date=2026-03-31", nil, "")
	var balanceSheet webapp.BalanceSheetDetail
	decodeJSON(t, balanceSheetResponse.Body.Bytes(), &balanceSheet)
	if balanceSheetResponse.Code != http.StatusOK || balanceSheet.SchemaVersion != webapp.APISchemaVersion ||
		balanceSheet.AsOf != "2025-04-01" || balanceSheet.Commodities == nil || len(balanceSheet.Commodities) != 0 {
		t.Fatalf("balance sheet status=%d detail=%+v", balanceSheetResponse.Code, balanceSheet)
	}

	closingResponse := request(t, handler, http.MethodGet,
		"/api/v1/reports/closing-balance-sheet?start_date=2025-04-01&end_date=2026-03-31", nil, "")
	var closing webapp.ClosingBalanceSheetDetail
	decodeJSON(t, closingResponse.Body.Bytes(), &closing)
	if closingResponse.Code != http.StatusOK || closing.SchemaVersion != webapp.APISchemaVersion ||
		closing.ConfigurationRevision != 1 || closing.AsOf != "2026-03-31" || len(closing.Commodities) != 2 ||
		closing.Commodities[0].Commodity != "JPY" || closing.Commodities[0].CurrentEarnings.Credit != "100" ||
		closing.Commodities[1].Commodity != "USD" || closing.Commodities[1].CurrentEarnings.Credit != "2.50" ||
		closing.Commodities[0].Total.Debit != "0" || closing.Commodities[0].Total.Credit != "0" ||
		closing.Commodities[1].Total.Debit != "0.00" || closing.Commodities[1].Total.Credit != "0" {
		t.Fatalf("closing balance sheet status=%d detail=%+v", closingResponse.Code, closing)
	}

	incomeResponse := request(t, handler, http.MethodGet, "/api/v1/reports/income-statement?start_date=2025-04-01&end_date=2025-04-30", nil, "")
	var income webapp.IncomeStatementDetail
	decodeJSON(t, incomeResponse.Body.Bytes(), &income)
	if incomeResponse.Code != http.StatusOK || len(income.Commodities) != 2 ||
		income.Commodities[0].NetIncome.Credit != "100" || income.Commodities[1].NetIncome.Credit != "2.50" {
		t.Fatalf("income statement status=%d detail=%+v", incomeResponse.Code, income)
	}

	trendResponse := request(t, handler, http.MethodGet,
		"/api/v1/reports/balance-trend?start_date=2025-04-01&end_date=2026-03-31", nil, "")
	var trend webapp.BalanceTrendDetail
	decodeJSON(t, trendResponse.Body.Bytes(), &trend)
	if trendResponse.Code != http.StatusOK || len(trend.Points) != 12 || len(trend.Points[0].Commodities) != 2 {
		t.Fatalf("balance trend status=%d detail=%+v", trendResponse.Code, trend)
	}

	currentResponse := request(t, handler, http.MethodGet,
		"/api/v1/reports/current-overview?as_of=2025-04-15&expense_start_date=2025-04-01&expense_end_date=2025-04-30", nil, "")
	var current webapp.CurrentOverviewDetail
	decodeJSON(t, currentResponse.Body.Bytes(), &current)
	if currentResponse.Code != http.StatusOK || current.SchemaVersion != webapp.APISchemaVersion || current.AsOf != "2025-04-15" ||
		len(current.Balances) != 1 || len(current.Expenses) != 0 {
		t.Fatalf("current overview status=%d detail=%+v", currentResponse.Code, current)
	}

	assertProblem(t, request(t, handler, http.MethodGet,
		"/api/v1/reports/trial-balance?start_date=2025-04-02&end_date=2025-04-30", nil, ""),
		http.StatusBadRequest, "invalid_period")
	assertProblem(t, request(t, handler, http.MethodGet,
		"/api/v1/reports/trial-balance?start_date="+url.QueryEscape("private-value"), nil, ""),
		http.StatusBadRequest, "invalid_period")
	assertProblem(t, request(t, handler, http.MethodPost, periodPath, []byte(`{}`), "application/json"),
		http.StatusMethodNotAllowed, "method_not_allowed")
	assertProblem(t, request(t, handler, http.MethodGet,
		"/api/v1/reports/income-statement?start_date=2025-04-01&end_date=2025-04-30&extra=private", nil, ""),
		http.StatusBadRequest, "invalid_period")
	assertProblem(t, request(t, handler, http.MethodPost,
		"/api/v1/reports/balance-trend?start_date=2025-04-01&end_date=2026-03-31", []byte(`{}`), "application/json"),
		http.StatusMethodNotAllowed, "method_not_allowed")
	assertProblem(t, request(t, handler, http.MethodGet,
		"/api/v1/reports/closing-balance-sheet?start_date=2025-04-01&end_date=2025-04-30", nil, ""),
		http.StatusBadRequest, "invalid_period")
	assertProblem(t, request(t, handler, http.MethodGet,
		"/api/v1/reports/closing-balance-sheet?start_date=2025-04-01&end_date=2026-03-31&extra=private", nil, ""),
		http.StatusBadRequest, "invalid_period")
	assertProblem(t, request(t, handler, http.MethodPost,
		"/api/v1/reports/closing-balance-sheet?start_date=2025-04-01&end_date=2026-03-31", []byte(`{}`), "application/json"),
		http.StatusMethodNotAllowed, "method_not_allowed")
	assertProblem(t, request(t, handler, http.MethodGet,
		"/api/v1/reports/current-overview?as_of=2025-04-15&expense_start_date=2025-04-01&expense_end_date=2025-04-30&extra=private", nil, ""),
		http.StatusBadRequest, "invalid_period")
	assertProblem(t, request(t, handler, http.MethodGet,
		"/api/v1/reports/current-overview?as_of=2026-04-01&expense_start_date=2025-04-01&expense_end_date=2025-04-30", nil, ""),
		http.StatusBadRequest, "invalid_period")
	assertProblem(t, request(t, handler, http.MethodGet,
		"/api/v1/reports/current-overview?as_of=2025-04-15&expense_start_date=2025-04-01&expense_end_date=2025-04-29", nil, ""),
		http.StatusBadRequest, "invalid_period")
	assertProblem(t, request(t, handler, http.MethodPost,
		"/api/v1/reports/current-overview?as_of=2025-04-15&expense_start_date=2025-04-01&expense_end_date=2025-04-30", []byte(`{}`), "application/json"),
		http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestFinancialReportAPIOpeningBalanceUnbalanced(t *testing.T) {
	handler := authenticatedAPIHandler(t, webstore.New(openDatabase(t)))
	input := []byte(`{
  "schema_version": 1,
  "records": [
    {"source":{"namespace":"report-api","display":"opening"},"occurred_at":"2024-04-01","description":"opening fixture","postings":[{"account":"Assets:Cash","amount":"100","commodity":"JPY"},{"account":"Equity:Opening","amount":"-100","commodity":"JPY"}]},
    {"source":{"namespace":"report-api","display":"expense"},"occurred_at":"2024-05-01","description":"expense fixture","postings":[{"account":"Expenses:Supplies","amount":"20","commodity":"JPY"},{"account":"Assets:Cash","amount":"-20","commodity":"JPY"}]}
  ]
}`)
	result := postImport(t, handler, input)
	runResponse := request(t, handler, http.MethodGet, result.DetailURL, nil, "")
	var run webapp.RunDetail
	decodeJSON(t, runResponse.Body.Bytes(), &run)
	if len(run.Outcomes) != 2 {
		t.Fatalf("financial report fixture run = %+v", run)
	}
	for _, outcome := range run.Outcomes {
		approveEntry(t, handler, outcome.EntryID, 0)
	}
	created := requestJSON(t, handler, http.MethodPost, "/api/v1/reporting/configuration", map[string]any{
		"base_revision": 0,
		"start_month":   4,
		"classifications": []map[string]any{
			{"account": "Assets", "category": "asset"},
			{"account": "Equity", "category": "equity"},
			{"account": "Expenses", "category": "expense"},
		},
		"fiscal_years": []map[string]any{
			{"start_date": "2024-04-01", "end_date": "2025-03-31", "opening_mode": "opening_entries", "opening_entry_ids": []string{run.Outcomes[0].EntryID}},
			{"start_date": "2025-04-01", "end_date": "2026-03-31", "opening_mode": "automatic", "opening_entry_ids": []string{}},
		},
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("create unbalanced reporting configuration status=%d body=%s", created.Code, created.Body.String())
	}
	assertProblem(t, request(t, handler, http.MethodGet,
		"/api/v1/reports/balance-sheet?start_date=2025-04-01&end_date=2026-03-31", nil, ""),
		http.StatusUnprocessableEntity, "opening_balance_unbalanced")
	assertProblem(t, request(t, handler, http.MethodGet,
		"/api/v1/reports/closing-balance-sheet?start_date=2025-04-01&end_date=2026-03-31", nil, ""),
		http.StatusUnprocessableEntity, "opening_balance_unbalanced")
	assertProblem(t, request(t, handler, http.MethodGet,
		"/api/v1/reports/balance-trend?start_date=2025-04-01&end_date=2026-03-31", nil, ""),
		http.StatusUnprocessableEntity, "opening_balance_unbalanced")
	income := request(t, handler, http.MethodGet,
		"/api/v1/reports/income-statement?start_date=2025-04-01&end_date=2025-04-30", nil, "")
	if income.Code != http.StatusOK {
		t.Fatalf("income statement after unbalanced opening status=%d body=%s", income.Code, income.Body.String())
	}
	current := request(t, handler, http.MethodGet,
		"/api/v1/reports/current-overview?as_of=2025-04-01&expense_start_date=2024-05-01&expense_end_date=2024-05-31", nil, "")
	if current.Code != http.StatusOK {
		t.Fatalf("current overview after unbalanced opening status=%d body=%s", current.Code, current.Body.String())
	}
}

func TestTrialBalanceAPINotConfigured(t *testing.T) {
	handler := authenticatedAPIHandler(t, webstore.New(openDatabase(t)))
	response := request(t, handler, http.MethodGet,
		"/api/v1/reports/trial-balance?start_date=2025-04-01&end_date=2025-04-30", nil, "")
	assertProblem(t, response, http.StatusConflict, "reporting_not_configured")
	closing := request(t, handler, http.MethodGet,
		"/api/v1/reports/closing-balance-sheet?start_date=2025-04-01&end_date=2026-03-31", nil, "")
	assertProblem(t, closing, http.StatusConflict, "reporting_not_configured")
	current := request(t, handler, http.MethodGet,
		"/api/v1/reports/current-overview?as_of=2025-04-01&expense_start_date=2025-04-01&expense_end_date=2025-04-30", nil, "")
	assertProblem(t, current, http.StatusConflict, "reporting_not_configured")
}

func TestReportDrillDownAPIExplainsEntriesAndRejectsStaleSnapshot(t *testing.T) {
	database := openDatabase(t)
	store := webstore.New(database)
	owner := authenticatedAPIHandler(t, store)
	viewer := authenticatedAPIHandlerForEmail(t, store, "viewer@example.com")
	configuration := map[string]any{
		"base_revision": 0, "start_month": 4,
		"classifications": []map[string]any{
			{"account": "Assets", "category": "asset"}, {"account": "Revenue", "category": "revenue"},
		},
		"fiscal_years": []map[string]any{{
			"start_date": "2025-04-01", "end_date": "2026-03-31", "opening_mode": "automatic", "opening_entry_ids": []string{},
		}},
	}
	if response := requestJSON(t, owner, http.MethodPost, "/api/v1/reporting/configuration", configuration); response.Code != http.StatusCreated {
		t.Fatalf("create reporting configuration status=%d body=%s", response.Code, response.Body.String())
	}
	result := postImport(t, owner, []byte(`{"schema_version":1,"records":[{"source":{"namespace":"drill-down","display":"sale"},"occurred_at":"2025-04-02","description":"anonymous sale","postings":[{"account":"Assets:Cash","amount":"20","commodity":"JPY"},{"account":"Revenue:Sales","amount":"-20","commodity":"JPY"}]}]}`))
	runResponse := request(t, owner, http.MethodGet, result.DetailURL, nil, "")
	var run webapp.RunDetail
	decodeJSON(t, runResponse.Body.Bytes(), &run)
	approveEntry(t, owner, run.Outcomes[0].EntryID, 0)

	periodPath := "/api/v1/reports/trial-balance?start_date=2025-04-01&end_date=2025-04-30"
	reportResponse := request(t, viewer, http.MethodGet, periodPath, nil, "")
	var report webapp.TrialBalanceDetail
	decodeJSON(t, reportResponse.Body.Bytes(), &report)
	if reportResponse.Code != http.StatusOK || len(report.SnapshotIdentity) != 64 {
		t.Fatalf("trial balance status=%d detail=%+v", reportResponse.Code, report)
	}
	query := url.Values{
		"start_date": {"2025-04-01"}, "end_date": {"2025-04-30"},
		"snapshot_identity": {report.SnapshotIdentity}, "commodity": {"JPY"}, "category": {"asset"},
		"account": {"Assets"}, "scope": {"subtree"},
	}
	drillPath := "/api/v1/reports/trial-balance/drill-down?" + query.Encode()
	drillResponse := request(t, viewer, http.MethodGet, drillPath, nil, "")
	var drill webapp.TrialBalanceDrillDownDetail
	decodeJSON(t, drillResponse.Body.Bytes(), &drill)
	if drillResponse.Code != http.StatusOK || drill.TotalEntries != 1 || len(drill.Entries) != 1 ||
		drill.Amounts.DebitTurnover != "20" || drill.Entries[0].ID != run.Outcomes[0].EntryID ||
		drill.Entries[0].Contributions[0].PostingIndex != 0 {
		t.Fatalf("trial balance drill-down status=%d detail=%+v", drillResponse.Code, drill)
	}

	configuration["base_revision"] = 1
	if response := requestJSON(t, owner, http.MethodPost, "/api/v1/reporting/configuration", configuration); response.Code != http.StatusCreated {
		t.Fatalf("update reporting configuration status=%d body=%s", response.Code, response.Body.String())
	}
	assertProblem(t, request(t, viewer, http.MethodGet, drillPath, nil, ""), http.StatusConflict, "report_snapshot_changed")
	query.Set("extra", "private")
	assertProblem(t, request(t, viewer, http.MethodGet,
		"/api/v1/reports/trial-balance/drill-down?"+query.Encode(), nil, ""), http.StatusBadRequest, "invalid_drill_down")
	assertProblem(t, request(t, viewer, http.MethodPost, drillPath, []byte(`{}`), "application/json"),
		http.StatusMethodNotAllowed, "method_not_allowed")
}
