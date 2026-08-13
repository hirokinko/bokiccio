package webapp_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/hirokinko/bokiccio/internal/webapp"
	"github.com/hirokinko/bokiccio/internal/webstore"
)

func TestReportingConfigurationAPIHistoryAndValidation(t *testing.T) {
	handler := webapp.NewHandler(webstore.New(openDatabase(t)))

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
	handler := webapp.NewHandler(webstore.New(openDatabase(t)))
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

	assertProblem(t, request(t, handler, http.MethodGet,
		"/api/v1/reports/trial-balance?start_date=2025-04-02&end_date=2025-04-30", nil, ""),
		http.StatusBadRequest, "invalid_period")
	assertProblem(t, request(t, handler, http.MethodGet,
		"/api/v1/reports/trial-balance?start_date="+url.QueryEscape("private-value"), nil, ""),
		http.StatusBadRequest, "invalid_period")
	assertProblem(t, request(t, handler, http.MethodPost, periodPath, []byte(`{}`), "application/json"),
		http.StatusMethodNotAllowed, "method_not_allowed")
}

func TestTrialBalanceAPINotConfigured(t *testing.T) {
	handler := webapp.NewHandler(webstore.New(openDatabase(t)))
	response := request(t, handler, http.MethodGet,
		"/api/v1/reports/trial-balance?start_date=2025-04-01&end_date=2025-04-30", nil, "")
	assertProblem(t, response, http.StatusConflict, "reporting_not_configured")
}
