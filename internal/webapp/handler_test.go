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

func TestMigrationRejectsFutureVersion(t *testing.T) {
	database := openDatabase(t)
	if _, err := database.Exec(`UPDATE schema_metadata SET version = 2 WHERE singleton = 1`); err != nil {
		t.Fatalf("set future version: %v", err)
	}
	if err := webstore.Migrate(context.Background(), database); !errors.Is(err, webstore.ErrUnsupportedSchema) {
		t.Fatalf("Migrate() error = %v, want ErrUnsupportedSchema", err)
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
