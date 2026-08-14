package webapp_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/hirokinko/bokiccio/internal/webapp"
	"github.com/hirokinko/bokiccio/internal/webstore"
	_ "turso.tech/database/tursogo"
)

func TestReadOnlyImportVerticalSlice(t *testing.T) {
	database := openDatabase(t)
	handler := webapp.NewHandler(webstore.New(database))
	input := readFixture(t, "../ingest/testdata/valid-v1.json")

	created := request(t, handler, http.MethodPost, "/api/v1/imports", input, "application/json")
	if created.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, body = %s", created.Code, created.Body.String())
	}
	var result struct {
		SchemaVersion int `json:"schema_version"`
		webapp.ImportResult
	}
	decodeJSON(t, created.Body.Bytes(), &result)
	if result.SchemaVersion != 1 || result.Counts.Success != 2 || result.HasErrors || result.DetailURL == "" {
		t.Fatalf("import result = %+v", result)
	}
	if created.Header().Get("Location") != result.DetailURL {
		t.Fatalf("Location = %q, want %q", created.Header().Get("Location"), result.DetailURL)
	}

	runResponse := request(t, handler, http.MethodGet, result.DetailURL, nil, "")
	if runResponse.Code != http.StatusOK {
		t.Fatalf("GET run status = %d, body = %s", runResponse.Code, runResponse.Body.String())
	}
	var run webapp.RunDetail
	decodeJSON(t, runResponse.Body.Bytes(), &run)
	if run.RunIdentity != result.RunIdentity || len(run.Outcomes) != 2 || run.Outcomes[0].EntryID == "" || run.Outcomes[1].EntryID == "" {
		t.Fatalf("run detail = %+v", run)
	}

	firstPageResponse := request(t, handler, http.MethodGet, "/api/v1/entries?limit=1", nil, "")
	var firstPage webapp.EntryPage
	decodeJSON(t, firstPageResponse.Body.Bytes(), &firstPage)
	if firstPageResponse.Code != http.StatusOK || len(firstPage.Entries) != 1 || firstPage.NextCursor == "" {
		t.Fatalf("first page status=%d page=%+v", firstPageResponse.Code, firstPage)
	}
	if firstPage.Entries[0].OccurredAt != "2026-08-10T14:30:00.1234+09:00" {
		t.Fatalf("first entry time = %q", firstPage.Entries[0].OccurredAt)
	}

	entryResponse := request(t, handler, http.MethodGet, "/api/v1/entries/"+url.PathEscape(firstPage.Entries[0].ID), nil, "")
	var entry webapp.EntryDetail
	decodeJSON(t, entryResponse.Body.Bytes(), &entry)
	if entryResponse.Code != http.StatusOK || len(entry.Postings) != 2 || entry.Postings[0].Amount == nil || *entry.Postings[0].Amount != "180" || entry.Postings[1].Amount != nil {
		t.Fatalf("entry detail status=%d detail=%+v", entryResponse.Code, entry)
	}

	secondPageResponse := request(t, handler, http.MethodGet, "/api/v1/entries?limit=1&cursor="+url.QueryEscape(firstPage.NextCursor), nil, "")
	var secondPage webapp.EntryPage
	decodeJSON(t, secondPageResponse.Body.Bytes(), &secondPage)
	if secondPageResponse.Code != http.StatusOK || len(secondPage.Entries) != 1 || secondPage.NextCursor != "" {
		t.Fatalf("second page status=%d page=%+v", secondPageResponse.Code, secondPage)
	}
	secondEntryResponse := request(t, handler, http.MethodGet, "/api/v1/entries/"+url.PathEscape(secondPage.Entries[0].ID), nil, "")
	decodeJSON(t, secondEntryResponse.Body.Bytes(), &entry)
	if entry.Postings[0].Amount == nil || *entry.Postings[0].Amount != "207.00" || len(entry.Comments) != 2 {
		t.Fatalf("scale/comments roundtrip = %+v", entry)
	}
}

func TestImportReturnsForbiddenWhenUploadIsDisabled(t *testing.T) {
	database := openDatabase(t)
	store := webstore.New(database)
	if err := store.SetFileUploadEnabled(context.Background(), false); err != nil {
		t.Fatalf("SetFileUploadEnabled(false) error=%v", err)
	}
	handler := webapp.NewHandler(store)
	response := request(t, handler, http.MethodPost, "/api/v1/imports", []byte("not-json"), "text/plain")
	if response.Code != http.StatusForbidden {
		t.Fatalf("POST status=%d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"code":"upload_disabled"`) {
		t.Fatalf("POST body=%s", response.Body.String())
	}
	var runs int
	if err := database.QueryRow(`SELECT count(*) FROM import_runs`).Scan(&runs); err != nil || runs != 0 {
		t.Fatalf("import_runs=%d error=%v, want 0", runs, err)
	}
}

func TestDecimalBoundaryAndZeroRoundtrip(t *testing.T) {
	database := openDatabase(t)
	store := webstore.New(database)
	input := []byte(`{
  "schema_version": 1,
  "records": [
    {
      "source": {"namespace": "test", "display": "fixtures/maximum.json"},
      "occurred_at": "2026-08-11",
      "description": "最大係数",
      "postings": [
        {"account": "資産:確認", "amount": "7.9228162514264337593543950335", "commodity": "UNIT"},
        {"account": "負債:確認", "amount": "-7.9228162514264337593543950335", "commodity": "UNIT"}
      ]
    },
    {
      "source": {"namespace": "test", "display": "fixtures/zero.json"},
      "occurred_at": "2026-08-11T00:00:00Z",
      "description": "負のゼロ",
      "postings": [
        {"account": "資産:確認", "amount": "-0.000", "commodity": "UNIT"},
        {"account": "負債:確認", "amount": "0.000", "commodity": "UNIT"}
      ]
    }
  ]
}`)
	result, err := store.Import(context.Background(), input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	maximum, err := store.GetEntry(context.Background(), run.Outcomes[0].EntryID)
	if err != nil {
		t.Fatalf("GetEntry(maximum) error = %v", err)
	}
	if maximum.Postings[0].Amount == nil || *maximum.Postings[0].Amount != "7.9228162514264337593543950335" {
		t.Fatalf("maximum amount = %+v", maximum.Postings[0].Amount)
	}
	zero, err := store.GetEntry(context.Background(), run.Outcomes[1].EntryID)
	if err != nil {
		t.Fatalf("GetEntry(zero) error = %v", err)
	}
	if zero.OccurredAt != "2026-08-11T00:00:00Z" || zero.Postings[0].Amount == nil || *zero.Postings[0].Amount != "0.000" {
		t.Fatalf("zero entry = %+v", zero)
	}
}

func TestImportPreservesMixedOutcomesAndDeduplicates(t *testing.T) {
	database := openDatabase(t)
	handler := webapp.NewHandler(webstore.New(database))
	input := readFixture(t, "../ingest/testdata/mixed-outcomes-v1.json")

	first := postImport(t, handler, input)
	if !first.HasErrors || first.Counts != (webapp.OutcomeCounts{Success: 1, Warning: 1, Error: 1, Duplicate: 1}) {
		t.Fatalf("first import = %+v", first)
	}
	runResponse := request(t, handler, http.MethodGet, first.DetailURL, nil, "")
	var run webapp.RunDetail
	decodeJSON(t, runResponse.Body.Bytes(), &run)
	if len(run.Outcomes) != 4 || len(run.Outcomes[1].Diagnostics) != 2 || run.Outcomes[2].EntryID != "" {
		t.Fatalf("mixed run detail = %+v", run)
	}

	second := postImport(t, handler, input)
	if second.Counts != (webapp.OutcomeCounts{Error: 1, Duplicate: 3}) {
		t.Fatalf("second import = %+v", second)
	}
	var generation int
	if err := database.QueryRow(`SELECT generation FROM workflow_state WHERE singleton = 1`).Scan(&generation); err != nil {
		t.Fatalf("query generation: %v", err)
	}
	if generation != 1 {
		t.Fatalf("generation = %d, want 1", generation)
	}
}

func TestImportRollbackAndPrivateSafeErrors(t *testing.T) {
	database := openDatabase(t)
	if _, err := database.Exec(`CREATE TRIGGER fail_outcome BEFORE INSERT ON outcomes
        BEGIN SELECT RAISE(FAIL, 'injected private database detail'); END`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}
	handler := webapp.NewHandler(webstore.New(database))
	response := request(t, handler, http.MethodPost, "/api/v1/imports",
		readFixture(t, "../ingest/testdata/valid-v1.json"), "application/json")
	if response.Code != http.StatusInternalServerError || strings.Contains(response.Body.String(), "injected") {
		t.Fatalf("failure response status=%d body=%s", response.Code, response.Body.String())
	}
	for table, want := range map[string]int{"import_runs": 0, "outcomes": 0, "entries": 0, "committed_identities": 0} {
		var count int
		if err := database.QueryRow(`SELECT count(*) FROM ` + table).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != want {
			t.Errorf("%s count = %d, want %d", table, count, want)
		}
	}
	var generation int
	if err := database.QueryRow(`SELECT generation FROM workflow_state WHERE singleton = 1`).Scan(&generation); err != nil || generation != 0 {
		t.Fatalf("generation = %d, error=%v", generation, err)
	}
}

func TestHTTPInputFailures(t *testing.T) {
	database := openDatabase(t)
	handler := webapp.NewHandler(webstore.New(database))
	tests := []struct {
		name        string
		method      string
		path        string
		body        []byte
		contentType string
		status      int
		code        string
	}{
		{name: "media type", method: http.MethodPost, path: "/api/v1/imports", body: []byte(`{}`), contentType: "text/plain", status: 415, code: "unsupported_media_type"},
		{name: "invalid import", method: http.MethodPost, path: "/api/v1/imports", body: []byte(`{"schema_version":2,"records":[],"SECRET":"do not echo"}`), contentType: "application/json", status: 400, code: "invalid_import"},
		{name: "oversized", method: http.MethodPost, path: "/api/v1/imports", body: bytes.Repeat([]byte("x"), (10<<20)+1), contentType: "application/json", status: 413, code: "body_too_large"},
		{name: "bad cursor", method: http.MethodGet, path: "/api/v1/entries?cursor=bad", status: 400, code: "invalid_request"},
		{name: "not found", method: http.MethodGet, path: "/api/v1/imports/missing", status: 404, code: "not_found"},
		{name: "method", method: http.MethodDelete, path: "/api/v1/entries", status: 405, code: "method_not_allowed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, handler, test.method, test.path, test.body, test.contentType)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.status, response.Body.String())
			}
			var problem struct {
				Code string `json:"code"`
			}
			decodeJSON(t, response.Body.Bytes(), &problem)
			if problem.Code != test.code || strings.Contains(response.Body.String(), "SECRET") {
				t.Fatalf("problem = %+v body=%s", problem, response.Body.String())
			}
		})
	}
}

func TestRevisionAndApprovalHistory(t *testing.T) {
	database := openDatabase(t)
	handler := webapp.NewHandler(webstore.New(database))
	entryID := importedEntryID(t, handler)
	entryPath := "/api/v1/entries/" + url.PathEscape(entryID)

	originalApproval := requestJSON(t, handler, http.MethodPost, entryPath+"/approvals", map[string]any{"revision": 0})
	if originalApproval.Code != http.StatusCreated {
		t.Fatalf("approve original status=%d body=%s", originalApproval.Code, originalApproval.Body.String())
	}

	invalid := requestJSON(t, handler, http.MethodPost, entryPath+"/revisions", map[string]any{
		"base_revision": 0,
		"occurred_at":   "2026-08-12T09:30:00+09:00",
		"description":   "修正中の仕訳",
		"comments":      []string{"source: fixtures/revision.json"},
		"postings": []map[string]any{
			{"account": "費用:確認", "amount": "200.00", "commodity": "JPY"},
			{"account": "資産:確認", "amount": "-100.00", "commodity": "JPY"},
		},
	})
	if invalid.Code != http.StatusCreated {
		t.Fatalf("create invalid revision status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	var invalidRevision struct {
		SchemaVersion int `json:"schema_version"`
		webapp.RevisionDetail
	}
	decodeJSON(t, invalid.Body.Bytes(), &invalidRevision)
	if invalidRevision.Revision != 1 || invalidRevision.Valid || len(invalidRevision.Diagnostics) != 1 || invalidRevision.Diagnostics[0].Code != "unbalanced_entry" {
		t.Fatalf("invalid revision = %+v", invalidRevision)
	}

	invalidApproval := requestJSON(t, handler, http.MethodPost, entryPath+"/approvals", map[string]any{"revision": 1})
	assertProblem(t, invalidApproval, http.StatusUnprocessableEntity, "invalid_revision")

	staleEdit := requestJSON(t, handler, http.MethodPost, entryPath+"/revisions", map[string]any{
		"base_revision": 0,
		"occurred_at":   "2026-08-12",
		"description":   "古い画面からの修正",
		"comments":      []string{},
		"postings": []map[string]any{
			{"account": "費用:確認", "amount": "1", "commodity": "JPY"},
			{"account": "資産:確認", "amount": "-1", "commodity": "JPY"},
		},
	})
	assertProblem(t, staleEdit, http.StatusConflict, "conflict")

	valid := requestJSON(t, handler, http.MethodPost, entryPath+"/revisions", map[string]any{
		"base_revision": 1,
		"occurred_at":   "2026-08-12T09:30:00+09:00",
		"description":   "確認済みの修正仕訳",
		"comments":      []string{"source: fixtures/revision.json", "確認済み"},
		"postings": []map[string]any{
			{"account": "費用:確認", "amount": "200.00", "commodity": "JPY", "comment": "明細"},
			{"account": "資産:確認"},
		},
	})
	if valid.Code != http.StatusCreated {
		t.Fatalf("create valid revision status=%d body=%s", valid.Code, valid.Body.String())
	}
	var validRevision struct {
		SchemaVersion int `json:"schema_version"`
		webapp.RevisionDetail
	}
	decodeJSON(t, valid.Body.Bytes(), &validRevision)
	if validRevision.Revision != 2 || !validRevision.Valid || len(validRevision.Diagnostics) != 0 {
		t.Fatalf("valid revision = %+v", validRevision)
	}

	staleApproval := requestJSON(t, handler, http.MethodPost, entryPath+"/approvals", map[string]any{"revision": 1})
	assertProblem(t, staleApproval, http.StatusConflict, "conflict")
	approved := requestJSON(t, handler, http.MethodPost, entryPath+"/approvals", map[string]any{"revision": 2})
	if approved.Code != http.StatusCreated {
		t.Fatalf("approve revision status=%d body=%s", approved.Code, approved.Body.String())
	}

	detailResponse := request(t, handler, http.MethodGet, entryPath, nil, "")
	var detail webapp.EntryDetail
	decodeJSON(t, detailResponse.Body.Bytes(), &detail)
	if detailResponse.Code != http.StatusOK || detail.CurrentRevision != 2 || detail.CurrentApproval == nil || detail.CurrentApproval.Revision != 2 {
		t.Fatalf("entry current state status=%d detail=%+v", detailResponse.Code, detail)
	}
	if len(detail.Revisions) != 2 || len(detail.Approvals) != 2 || detail.Revisions[0].Valid || !detail.Revisions[1].Valid {
		t.Fatalf("entry history = %+v", detail)
	}
	if detail.Description == detail.Revisions[1].Description || detail.Revisions[1].Postings[0].Amount == nil ||
		*detail.Revisions[1].Postings[0].Amount != "200.00" || detail.Revisions[1].Postings[1].Amount != nil {
		t.Fatalf("original was changed or revision scale was lost: %+v", detail)
	}
}

func TestRevisionRequestFailures(t *testing.T) {
	database := openDatabase(t)
	handler := webapp.NewHandler(webstore.New(database))
	entryPath := "/api/v1/entries/" + url.PathEscape(importedEntryID(t, handler))

	missingBase := requestJSON(t, handler, http.MethodPost, entryPath+"/revisions", map[string]any{
		"occurred_at": "2026-08-12", "description": "missing base", "comments": []string{}, "postings": []any{},
	})
	assertProblem(t, missingBase, http.StatusBadRequest, "invalid_request")
	missingRevision := requestJSON(t, handler, http.MethodPost, entryPath+"/approvals", map[string]any{})
	assertProblem(t, missingRevision, http.StatusBadRequest, "invalid_request")
	wrongMediaType := request(t, handler, http.MethodPost, entryPath+"/revisions", []byte(`{}`), "text/plain")
	assertProblem(t, wrongMediaType, http.StatusUnsupportedMediaType, "unsupported_media_type")
	unknownField := request(t, handler, http.MethodPost, entryPath+"/approvals", []byte(`{"revision":0,"secret":"not echoed"}`), "application/json")
	assertProblem(t, unknownField, http.StatusBadRequest, "invalid_request")
	if strings.Contains(unknownField.Body.String(), "secret") {
		t.Fatalf("private request data reflected: %s", unknownField.Body.String())
	}
}

func TestSearchAndApprovedExports(t *testing.T) {
	database := openDatabase(t)
	handler := webapp.NewHandler(webstore.New(database))
	input := []byte(`{
  "schema_version": 1,
  "records": [
    {
      "source": {"namespace": "receipt", "display": "fixtures/a.json", "external_id": "search-a"},
      "occurred_at": "2026-08-03",
      "description": "Alpha original",
      "postings": [
        {"account": "費用:食費:外食", "amount": "100.00", "commodity": "JPY"},
        {"account": "資産:現金", "amount": "-100.00", "commodity": "JPY"}
      ]
    },
    {
      "source": {"namespace": "mail", "display": "fixtures/b.json", "external_id": "search-b"},
      "occurred_at": "2026-08-05",
      "description": "Beta original",
      "warnings": [{"code": "test.review", "message": "review"}],
      "postings": [
        {"account": "費用:未設定", "amount": "300.00", "commodity": "JPY"},
        {"account": "資産:現金", "amount": "-300.00", "commodity": "JPY"}
      ]
    },
    {
      "source": {"namespace": "receipt", "display": "fixtures/c.json", "external_id": "search-c"},
      "occurred_at": "2026-08-02",
      "description": "Gamma original",
      "postings": [
        {"account": "費用:日用品", "amount": "200", "commodity": "JPY"},
        {"account": "資産:現金", "amount": "-200", "commodity": "JPY"}
      ]
    },
    {
      "source": {"namespace": "receipt", "display": "fixtures/d.json", "external_id": "search-d"},
      "occurred_at": "2026-08-04",
      "description": "Delta original",
      "postings": [
        {"account": "費用:日用品", "amount": "400", "commodity": "JPY"},
        {"account": "資産:現金", "amount": "-400", "commodity": "JPY"}
      ]
    }
  ]
}`)
	result := postImport(t, handler, input)
	runResponse := request(t, handler, http.MethodGet, result.DetailURL, nil, "")
	var run webapp.RunDetail
	decodeJSON(t, runResponse.Body.Bytes(), &run)
	if runResponse.Code != http.StatusOK || len(run.Outcomes) != 4 {
		t.Fatalf("import run status=%d detail=%+v", runResponse.Code, run)
	}
	ids := make([]string, len(run.Outcomes))
	for index, outcome := range run.Outcomes {
		ids[index] = outcome.EntryID
		if ids[index] == "" {
			t.Fatalf("outcome %d has no entry: %+v", index, outcome)
		}
	}

	approveEntry(t, handler, ids[0], 0)
	approveEntry(t, handler, ids[2], 0)
	betaRevision := createTestRevision(t, handler, ids[1], 0, map[string]any{
		"occurred_at": "2026-08-01T10:00:00+09:00", "description": "Beta revised",
		"comments": []string{"export beta"},
		"postings": []map[string]any{
			{"account": "費用:交通", "amount": "300.00", "commodity": "JPY"},
			{"account": "資産:現金"},
		},
	})
	approveEntry(t, handler, ids[1], betaRevision)
	createTestRevision(t, handler, ids[2], 0, map[string]any{
		"occurred_at": "2026-08-02", "description": "Gamma invalid revision",
		"comments": []string{},
		"postings": []map[string]any{
			{"account": "費用:日用品", "amount": "200", "commodity": "JPY"},
			{"account": "資産:現金", "amount": "-100", "commodity": "JPY"},
		},
	})

	page := getEntryPage(t, handler, "/api/v1/entries")
	if len(page.Entries) != 4 {
		t.Fatalf("entry count = %d, want 4", len(page.Entries))
	}
	wantIDs := []string{ids[3], ids[2], ids[1], ids[0]}
	wantWorkflow := []string{"unapproved", "invalid", "approved", "approved"}
	for index := range wantIDs {
		if page.Entries[index].ID != wantIDs[index] || page.Entries[index].WorkflowStatus != wantWorkflow[index] {
			t.Fatalf("entry %d = %+v, want id=%q workflow=%q", index, page.Entries[index], wantIDs[index], wantWorkflow[index])
		}
	}
	if page.Entries[2].Description != "Beta revised" || page.Entries[2].OccurredAt != "2026-08-01T10:00:00+09:00" ||
		page.Entries[2].CurrentRevision != 1 || page.Entries[2].Status != "warning" {
		t.Fatalf("revised summary = %+v", page.Entries[2])
	}

	assertEntryIDs(t, getEntryPage(t, handler, "/api/v1/entries?workflow_status=approved"), ids[1], ids[0])
	assertEntryIDs(t, getEntryPage(t, handler, "/api/v1/entries?workflow_status=invalid"), ids[2])
	assertEntryIDs(t, getEntryPage(t, handler, "/api/v1/entries?workflow_status=unapproved"), ids[3])
	assertEntryIDs(t, getEntryPage(t, handler, "/api/v1/entries?account="+url.QueryEscape("費用:食費")), ids[0])
	assertEntryIDs(t, getEntryPage(t, handler, "/api/v1/entries?description=revised"), ids[1])
	assertEntryIDs(t, getEntryPage(t, handler, "/api/v1/entries?date_from=2026-08-01&date_to=2026-08-01"), ids[1])
	assertEntryIDs(t, getEntryPage(t, handler, "/api/v1/entries?status=warning"), ids[1])
	assertEntryIDs(t, getEntryPage(t, handler, "/api/v1/entries?source_namespace=mail"), ids[1])

	first := getEntryPage(t, handler, "/api/v1/entries?workflow_status=approved&limit=1")
	if len(first.Entries) != 1 || first.NextCursor == "" {
		t.Fatalf("first filtered page = %+v", first)
	}
	second := getEntryPage(t, handler, "/api/v1/entries?workflow_status=approved&limit=1&cursor="+url.QueryEscape(first.NextCursor))
	assertEntryIDs(t, second, ids[0])
	assertProblem(t, request(t, handler, http.MethodGet,
		"/api/v1/entries?workflow_status=invalid&limit=1&cursor="+url.QueryEscape(first.NextCursor), nil, ""),
		http.StatusBadRequest, "invalid_request")
	for _, path := range []string{
		"/api/v1/entries?date_from=2026-02-30",
		"/api/v1/entries?date_from=2026-08-02&date_to=2026-08-01",
		"/api/v1/entries?status=error",
		"/api/v1/entries?workflow_status=reviewed",
	} {
		assertProblem(t, request(t, handler, http.MethodGet, path, nil, ""), http.StatusBadRequest, "invalid_request")
	}

	jsonResponse := request(t, handler, http.MethodGet, "/api/v1/exports/json", nil, "")
	if jsonResponse.Code != http.StatusOK || jsonResponse.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("JSON export status=%d content-type=%q body=%s", jsonResponse.Code, jsonResponse.Header().Get("Content-Type"), jsonResponse.Body.String())
	}
	var exported webapp.JSONExport
	decodeJSON(t, jsonResponse.Body.Bytes(), &exported)
	if exported.SchemaVersion != 1 || len(exported.Entries) != 2 || exported.Entries[0].ID != ids[1] ||
		exported.Entries[0].Revision != 1 || exported.Entries[1].ID != ids[0] {
		t.Fatalf("JSON export = %+v", exported)
	}
	if exported.Entries[0].Postings[0].Amount == nil || *exported.Entries[0].Postings[0].Amount != "300.00" ||
		exported.Entries[0].Postings[1].Amount != nil {
		t.Fatalf("revised export postings = %+v", exported.Entries[0].Postings)
	}
	filteredJSON := request(t, handler, http.MethodGet, "/api/v1/exports/json?account="+url.QueryEscape("費用:食費"), nil, "")
	var filtered webapp.JSONExport
	decodeJSON(t, filteredJSON.Body.Bytes(), &filtered)
	if filteredJSON.Code != http.StatusOK || len(filtered.Entries) != 1 || filtered.Entries[0].ID != ids[0] {
		t.Fatalf("filtered JSON export status=%d export=%+v", filteredJSON.Code, filtered)
	}
	assertProblem(t, request(t, handler, http.MethodGet, "/api/v1/exports/json?workflow_status=invalid", nil, ""),
		http.StatusBadRequest, "invalid_request")

	tacklerResponse := request(t, handler, http.MethodGet, "/api/v1/exports/tackler", nil, "")
	wantTackler := "2026-08-01T10:00:00+09:00  'Beta revised\n" +
		"    ; export beta\n" +
		"    費用:交通  300.00 JPY\n" +
		"    資産:現金\n\n" +
		"2026-08-03  'Alpha original\n" +
		"    ; source: fixtures/a.json\n" +
		"    費用:食費:外食  100.00 JPY\n" +
		"    資産:現金  -100.00 JPY\n"
	if tacklerResponse.Code != http.StatusOK || tacklerResponse.Header().Get("Content-Type") != "text/plain; charset=utf-8" ||
		tacklerResponse.Body.String() != wantTackler {
		t.Fatalf("Tackler export status=%d content-type=%q\ngot:\n%s\nwant:\n%s", tacklerResponse.Code,
			tacklerResponse.Header().Get("Content-Type"), tacklerResponse.Body.String(), wantTackler)
	}
	emptyTackler := request(t, handler, http.MethodGet, "/api/v1/exports/tackler?description=missing", nil, "")
	if emptyTackler.Code != http.StatusOK || emptyTackler.Body.Len() != 0 {
		t.Fatalf("empty Tackler export status=%d body=%q", emptyTackler.Code, emptyTackler.Body.String())
	}
	emptyJSON := request(t, handler, http.MethodGet, "/api/v1/exports/json?description=missing", nil, "")
	var noEntries webapp.JSONExport
	decodeJSON(t, emptyJSON.Body.Bytes(), &noEntries)
	if emptyJSON.Code != http.StatusOK || noEntries.SchemaVersion != 1 || noEntries.Entries == nil || len(noEntries.Entries) != 0 {
		t.Fatalf("empty JSON export status=%d export=%+v", emptyJSON.Code, noEntries)
	}
}

func TestApprovedExportOrdersDateBeforeTimestampAndTimestampsByInstant(t *testing.T) {
	database := openDatabase(t)
	handler := webapp.NewHandler(webstore.New(database))
	input := []byte(`{
  "schema_version": 1,
  "records": [
    {"source":{"namespace":"ordering","display":"date","external_id":"order-date"},"occurred_at":"2026-08-01","description":"date","postings":[{"account":"資産:確認","amount":"1","commodity":"UNIT"},{"account":"負債:確認","amount":"-1","commodity":"UNIT"}]},
    {"source":{"namespace":"ordering","display":"later","external_id":"order-later"},"occurred_at":"2026-08-01T10:00:00+09:00","description":"later instant","postings":[{"account":"資産:確認","amount":"1","commodity":"UNIT"},{"account":"負債:確認","amount":"-1","commodity":"UNIT"}]},
    {"source":{"namespace":"ordering","display":"earlier","external_id":"order-earlier"},"occurred_at":"2026-08-01T00:30:00Z","description":"earlier instant","postings":[{"account":"資産:確認","amount":"1","commodity":"UNIT"},{"account":"負債:確認","amount":"-1","commodity":"UNIT"}]}
  ]
}`)
	result := postImport(t, handler, input)
	runResponse := request(t, handler, http.MethodGet, result.DetailURL, nil, "")
	var run webapp.RunDetail
	decodeJSON(t, runResponse.Body.Bytes(), &run)
	if runResponse.Code != http.StatusOK || len(run.Outcomes) != 3 {
		t.Fatalf("ordering import run status=%d detail=%+v", runResponse.Code, run)
	}
	for _, outcome := range run.Outcomes {
		approveEntry(t, handler, outcome.EntryID, 0)
	}
	response := request(t, handler, http.MethodGet, "/api/v1/exports/json?source_namespace=ordering", nil, "")
	var exported webapp.JSONExport
	decodeJSON(t, response.Body.Bytes(), &exported)
	want := []string{"date", "earlier instant", "later instant"}
	if response.Code != http.StatusOK || len(exported.Entries) != len(want) {
		t.Fatalf("ordering export status=%d export=%+v", response.Code, exported)
	}
	for index, description := range want {
		if exported.Entries[index].Description != description {
			t.Fatalf("exported entry %d description=%q, want=%q", index, exported.Entries[index].Description, description)
		}
	}
}

func createTestRevision(t *testing.T, handler http.Handler, entryID string, baseRevision int, fields map[string]any) int {
	t.Helper()
	fields["base_revision"] = baseRevision
	response := requestJSON(t, handler, http.MethodPost,
		"/api/v1/entries/"+url.PathEscape(entryID)+"/revisions", fields)
	if response.Code != http.StatusCreated {
		t.Fatalf("create revision status=%d body=%s", response.Code, response.Body.String())
	}
	var revision struct {
		SchemaVersion int `json:"schema_version"`
		webapp.RevisionDetail
	}
	decodeJSON(t, response.Body.Bytes(), &revision)
	return revision.Revision
}

func approveEntry(t *testing.T, handler http.Handler, entryID string, revision int) {
	t.Helper()
	response := requestJSON(t, handler, http.MethodPost,
		"/api/v1/entries/"+url.PathEscape(entryID)+"/approvals", map[string]any{"revision": revision})
	if response.Code != http.StatusCreated {
		t.Fatalf("approve revision %d status=%d body=%s", revision, response.Code, response.Body.String())
	}
}

func getEntryPage(t *testing.T, handler http.Handler, path string) webapp.EntryPage {
	t.Helper()
	response := request(t, handler, http.MethodGet, path, nil, "")
	if response.Code != http.StatusOK {
		t.Fatalf("GET %s status=%d body=%s", path, response.Code, response.Body.String())
	}
	var page webapp.EntryPage
	decodeJSON(t, response.Body.Bytes(), &page)
	return page
}

func assertEntryIDs(t *testing.T, page webapp.EntryPage, want ...string) {
	t.Helper()
	if len(page.Entries) != len(want) {
		t.Fatalf("entry count=%d, want=%d page=%+v", len(page.Entries), len(want), page)
	}
	for index, id := range want {
		if page.Entries[index].ID != id {
			t.Fatalf("entry %d id=%q, want=%q page=%+v", index, page.Entries[index].ID, id, page)
		}
	}
}

func TestMigrationRejectsFutureVersion(t *testing.T) {
	database := openDatabase(t)
	if _, err := database.Exec(`UPDATE schema_metadata SET version = ? WHERE singleton = 1`, webstore.SchemaVersion+1); err != nil {
		t.Fatalf("set future version: %v", err)
	}
	if err := webstore.Migrate(context.Background(), database); !errors.Is(err, webstore.ErrUnsupportedSchema) {
		t.Fatalf("Migrate() error = %v, want ErrUnsupportedSchema", err)
	}
}

func TestMigrationPreservesV1Entries(t *testing.T) {
	database := openDatabase(t)
	store := webstore.New(database)
	result, err := store.Import(context.Background(), readFixture(t, "../ingest/testdata/valid-v1.json"))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	for _, statement := range []string{
		`DROP TABLE application_settings`,
		`DROP TABLE reporting_opening_entries`,
		`DROP TABLE reporting_fiscal_years`,
		`DROP TABLE reporting_classifications`,
		`DROP TABLE reporting_configurations`,
		`ALTER TABLE postings DROP COLUMN total_price_commodity`,
		`ALTER TABLE postings DROP COLUMN total_price_amount_scale`,
		`ALTER TABLE postings DROP COLUMN total_price_amount_text`,
		`DROP TABLE entry_approvals`,
		`DROP TABLE revision_diagnostics`,
		`DROP TABLE revision_postings`,
		`DROP TABLE revision_comments`,
		`DROP TABLE entry_revisions`,
		`UPDATE schema_metadata SET version = 1 WHERE singleton = 1`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare v1 schema: %v", err)
		}
	}
	if err := webstore.Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate(v1 to current) error = %v", err)
	}
	settings, err := store.GetApplicationSettings(context.Background())
	if err != nil || !settings.FileUploadEnabled {
		t.Fatalf("GetApplicationSettings() settings=%+v error=%v", settings, err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil || len(run.Outcomes) == 0 {
		t.Fatalf("GetRun() after migration error=%v run=%+v", err, run)
	}
	entry, err := store.GetEntry(context.Background(), run.Outcomes[0].EntryID)
	if err != nil || entry.CurrentRevision != 0 || len(entry.Revisions) != 0 {
		t.Fatalf("GetEntry() after migration error=%v entry=%+v", err, entry)
	}
}

func TestMigrationFailurePreservesV1Data(t *testing.T) {
	database := openDatabase(t)
	store := webstore.New(database)
	result, err := store.Import(context.Background(), readFixture(t, "../ingest/testdata/valid-v1.json"))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	for _, statement := range []string{
		`ALTER TABLE postings DROP COLUMN total_price_commodity`,
		`ALTER TABLE postings DROP COLUMN total_price_amount_scale`,
		`ALTER TABLE postings DROP COLUMN total_price_amount_text`,
		`DROP TABLE entry_approvals`,
		`DROP TABLE revision_diagnostics`,
		`DROP TABLE revision_postings`,
		`DROP TABLE revision_comments`,
		`DROP TABLE entry_revisions`,
		`UPDATE schema_metadata SET version = 1 WHERE singleton = 1`,
		`CREATE TABLE entry_revisions (conflict TEXT)`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare v2 migration conflict: %v", err)
		}
	}
	if err := webstore.Migrate(context.Background(), database); err == nil {
		t.Fatal("Migrate(v1 to v2) error = nil, want schema failure")
	}
	var version int
	if err := database.QueryRow(`SELECT version FROM schema_metadata WHERE singleton = 1`).Scan(&version); err != nil || version != 1 {
		t.Fatalf("schema version=%d error=%v", version, err)
	}
	if _, err := store.GetRun(context.Background(), result.RunIdentity); err != nil {
		t.Fatalf("GetRun() after failed migration error=%v", err)
	}
	if _, err := database.Exec(`SELECT count(*) FROM revision_comments`); err == nil {
		t.Fatal("failed migration left a v2 table created after the failure point")
	}
}

func TestMigrationFailureRollsBackPartialSchema(t *testing.T) {
	database, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	database.SetMaxOpenConns(1)
	defer database.Close()
	for _, statement := range []string{
		`CREATE TABLE schema_metadata (singleton INTEGER PRIMARY KEY, version INTEGER NOT NULL)`,
		`INSERT INTO schema_metadata (singleton, version) VALUES (1, 0)`,
		`CREATE TABLE workflow_state (conflict TEXT)`,
	} {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare migration conflict: %v", err)
		}
	}
	if err := webstore.Migrate(context.Background(), database); err == nil {
		t.Fatal("Migrate() error = nil, want schema failure")
	}
	var version int
	if err := database.QueryRow(`SELECT version FROM schema_metadata WHERE singleton = 1`).Scan(&version); err != nil || version != 0 {
		t.Fatalf("schema version = %d, error=%v", version, err)
	}
	if _, err := database.Exec(`SELECT count(*) FROM committed_identities`); err == nil {
		t.Fatal("migration left a table created after the failure point")
	}
}

func openDatabase(t *testing.T) *sql.DB {
	t.Helper()
	database, err := sql.Open("turso", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	database.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = database.Close() })
	if err := webstore.Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := webstore.Migrate(context.Background(), database); err != nil {
		t.Fatalf("Migrate() idempotent error = %v", err)
	}
	if err := webstore.CheckSchema(context.Background(), database); err != nil {
		t.Fatalf("CheckSchema() error = %v", err)
	}
	return database
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", path, err)
	}
	return data
}

func postImport(t *testing.T, handler http.Handler, input []byte) webapp.ImportResult {
	t.Helper()
	response := request(t, handler, http.MethodPost, "/api/v1/imports", input, "application/json")
	if response.Code != http.StatusCreated {
		t.Fatalf("POST import status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct {
		SchemaVersion int `json:"schema_version"`
		webapp.ImportResult
	}
	decodeJSON(t, response.Body.Bytes(), &result)
	return result.ImportResult
}

func importedEntryID(t *testing.T, handler http.Handler) string {
	t.Helper()
	result := postImport(t, handler, readFixture(t, "../ingest/testdata/valid-v1.json"))
	runResponse := request(t, handler, http.MethodGet, result.DetailURL, nil, "")
	var run webapp.RunDetail
	decodeJSON(t, runResponse.Body.Bytes(), &run)
	if runResponse.Code != http.StatusOK || len(run.Outcomes) == 0 || run.Outcomes[0].EntryID == "" {
		t.Fatalf("imported run status=%d detail=%+v", runResponse.Code, run)
	}
	return run.Outcomes[0].EntryID
}

func requestJSON(t *testing.T, handler http.Handler, method, path string, value any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("Marshal JSON error = %v", err)
	}
	return request(t, handler, method, path, body, "application/json")
}

func assertProblem(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d, want=%d body=%s", response.Code, status, response.Body.String())
	}
	var problem struct {
		Code string `json:"code"`
	}
	decodeJSON(t, response.Body.Bytes(), &problem)
	if problem.Code != code {
		t.Fatalf("problem code=%q, want=%q body=%s", problem.Code, code, response.Body.String())
	}
}

func request(t *testing.T, handler http.Handler, method, path string, body []byte, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func decodeJSON(t *testing.T, data []byte, destination any) {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(destination); err != nil {
		t.Fatalf("Decode JSON error = %v; body=%s", err, data)
	}
	if _, err := io.ReadAll(decoder.Buffered()); err != nil {
		t.Fatalf("read buffered JSON: %v", err)
	}
}
