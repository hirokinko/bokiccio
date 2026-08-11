package webui_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/hirokinko/bokiccio/internal/webapp"
	"github.com/hirokinko/bokiccio/internal/webstore"
	"github.com/hirokinko/bokiccio/internal/webui"
	_ "turso.tech/database/tursogo"
)

func TestReadOnlyUIBrowseAndEscape(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-test","display":"fixtures/<source>.json","external_id":"ui-entry"},"occurred_at":"2026-08-11T09:30:00+09:00","description":"<script>alert(1)</script>","comments":["<b>comment</b>"],"warnings":[{"code":"ui.review","message":"<img src=x onerror=alert(1)>"}],"postings":[{"account":"費用:確認","amount":"120.00","commodity":"JPY","comment":"item"},{"account":"資産:確認"}]}]}`)
	result, err := store.Import(context.Background(), input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil || len(run.Outcomes) != 1 || run.Outcomes[0].EntryID == "" {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	entryID := run.Outcomes[0].EntryID
	handler := webui.NewHandler(store)

	index := serve(handler, http.MethodGet, "/")
	assertHTMLResponse(t, index, http.StatusOK)
	if body := index.Body.String(); !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") ||
		strings.Contains(body, "<script>alert(1)</script>") ||
		!strings.Contains(body, "/assets/htmx-2.0.10.min.js") ||
		strings.Contains(body, "https://") ||
		!strings.Contains(body, "/entries/"+url.PathEscape(entryID)) {
		t.Fatalf("index body does not preserve safe UI contract: %s", body)
	}
	baseRevision := 0
	revisedAmount := "125.00"
	revision, err := store.CreateRevision(context.Background(), entryID, webapp.RevisionRequest{
		BaseRevision: &baseRevision, OccurredAt: "2026-08-12", Description: "revised <entry>",
		Comments: []string{"revision <comment>"}, Postings: []webapp.PostingDetail{
			{Account: "費用:確認", Amount: &revisedAmount, Commodity: "JPY"},
			{Account: "資産:確認"},
		},
	})
	if err != nil || !revision.Valid {
		t.Fatalf("CreateRevision() revision=%+v error=%v", revision, err)
	}
	if _, err := store.ApproveRevision(context.Background(), entryID, webapp.ApprovalRequest{Revision: &revision.Revision}); err != nil {
		t.Fatalf("ApproveRevision() error=%v", err)
	}

	entry := serve(handler, http.MethodGet, "/entries/"+url.PathEscape(entryID))
	assertHTMLResponse(t, entry, http.StatusOK)
	for _, text := range []string{
		"120.00", "125.00", "JPY", "省略", "&lt;b&gt;comment&lt;/b&gt;", "ui.review",
		"&lt;img src=x onerror=alert(1)&gt;", "Revision 1", "revised &lt;entry&gt;",
		"revision &lt;comment&gt;", "Current candidate · revision 1", "Original snapshot",
		"Current approval: revision 1", "Approval history",
	} {
		if !strings.Contains(entry.Body.String(), text) {
			t.Errorf("entry body does not contain %q: %s", text, entry.Body.String())
		}
	}

	runPage := serve(handler, http.MethodGet, "/imports/"+url.PathEscape(result.RunIdentity))
	assertHTMLResponse(t, runPage, http.StatusOK)
	if !strings.Contains(runPage.Body.String(), "取込結果") || !strings.Contains(runPage.Body.String(), "/entries/"+url.PathEscape(entryID)) || !strings.Contains(runPage.Body.String(), "Input digest") {
		t.Fatalf("run page body = %s", runPage.Body.String())
	}

	head := serve(handler, http.MethodHead, "/")
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("HEAD status=%d body=%q", head.Code, head.Body.String())
	}
	method := serve(handler, http.MethodPost, "/")
	if method.Code != http.StatusMethodNotAllowed || method.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST status=%d Allow=%q", method.Code, method.Header().Get("Allow"))
	}
	englishMethod := serve(handler, http.MethodPost, "/en/")
	if englishMethod.Code != http.StatusMethodNotAllowed || !strings.Contains(englishMethod.Body.String(), `href="/"`) || !strings.Contains(englishMethod.Body.String(), `href="/en/"`) {
		t.Fatalf("POST /en/ status=%d body=%s", englishMethod.Code, englishMethod.Body.String())
	}
	missing := serve(handler, http.MethodGet, "/entries/missing")
	assertHTMLResponse(t, missing, http.StatusNotFound)
}

func TestLocalizedReadOnlyRoutesPreserveAccountingValues(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-locale","display":"fixtures/source.json","external_id":"ui-locale-entry"},"occurred_at":"2026-08-11","description":"Locale candidate","warnings":[{"code":"ui.locale","message":"Preserve diagnostic"}],"postings":[{"account":"費用:品質","amount":"120.00","commodity":"JPY"},{"account":"資産:品質"}]}]}`)
	result, err := store.Import(context.Background(), input)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil || len(run.Outcomes) != 1 || run.Outcomes[0].EntryID == "" {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	entryID := run.Outcomes[0].EntryID
	handler := webui.NewHandler(store)

	japaneseIndex := serve(handler, http.MethodGet, "/")
	englishIndex := serve(handler, http.MethodGet, "/en/")
	assertHTMLResponse(t, japaneseIndex, http.StatusOK)
	assertHTMLResponse(t, englishIndex, http.StatusOK)
	assertContainsAll(t, japaneseIndex.Body.String(), []string{`<html lang="ja">`, "1件を表示", `/entries/` + url.PathEscape(entryID), `href="/en/"`, "English"})
	assertContainsAll(t, englishIndex.Body.String(), []string{`<html lang="en">`, "Showing 1 entry", `/en/entries/` + url.PathEscape(entryID), "日本語"})
	assertAccountingTextPreserved(t, japaneseIndex.Body.String(), englishIndex.Body.String(), []string{
		"Locale candidate", "ui-locale", "fixtures/source.json",
	})

	japaneseEntry := serve(handler, http.MethodGet, "/entries/"+url.PathEscape(entryID))
	englishEntry := serve(handler, http.MethodGet, "/en/entries/"+url.PathEscape(entryID))
	assertHTMLResponse(t, japaneseEntry, http.StatusOK)
	assertHTMLResponse(t, englishEntry, http.StatusOK)
	assertContainsAll(t, japaneseEntry.Body.String(), []string{`<html lang="ja">`, `/imports/` + url.PathEscape(result.RunIdentity), `/en/entries/` + url.PathEscape(entryID)})
	assertContainsAll(t, englishEntry.Body.String(), []string{`<html lang="en">`, "Back to journal candidates", `/en/imports/` + url.PathEscape(result.RunIdentity), `/entries/` + url.PathEscape(entryID)})
	assertAccountingTextPreserved(t, japaneseEntry.Body.String(), englishEntry.Body.String(), []string{
		"Locale candidate", "費用:品質", "資産:品質", "120.00", "JPY", "ui.locale", "Preserve diagnostic", result.RunIdentity,
	})

	japaneseRun := serve(handler, http.MethodGet, "/imports/"+url.PathEscape(result.RunIdentity))
	englishRun := serve(handler, http.MethodGet, "/en/imports/"+url.PathEscape(result.RunIdentity))
	assertHTMLResponse(t, japaneseRun, http.StatusOK)
	assertHTMLResponse(t, englishRun, http.StatusOK)
	assertContainsAll(t, japaneseRun.Body.String(), []string{`<html lang="ja">`, "取込結果", `/entries/` + url.PathEscape(entryID), `/en/imports/` + url.PathEscape(result.RunIdentity)})
	assertContainsAll(t, englishRun.Body.String(), []string{`<html lang="en">`, "Import result", `/en/entries/` + url.PathEscape(entryID), `/imports/` + url.PathEscape(result.RunIdentity)})
	assertAccountingTextPreserved(t, japaneseRun.Body.String(), englishRun.Body.String(), []string{
		result.RunIdentity, "ui-locale", "fixtures/source.json", entryID,
	})
}

func TestLocalizedEmptyAndErrorRoutes(t *testing.T) {
	handler := webui.NewHandler(webstore.New(openUIDatabase(t)))

	for _, test := range []struct {
		name string
		path string
		want []string
	}{
		{name: "ja empty", path: "/", want: []string{`<html lang="ja">`, "仕訳候補はまだありません"}},
		{name: "en empty", path: "/en/", want: []string{`<html lang="en">`, "No journal candidates yet"}},
		{name: "en malformed detail", path: "/en/entries/a/b", want: []string{`<html lang="en">`, "The requested page was not found."}},
		{name: "unsupported locale prefix", path: "/fr/", want: []string{`<html lang="ja">`, "指定されたページは見つかりませんでした。"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := serve(handler, http.MethodGet, test.path)
			status := http.StatusOK
			if !strings.Contains(test.name, "empty") {
				status = http.StatusNotFound
			}
			assertHTMLResponse(t, response, status)
			assertContainsAll(t, response.Body.String(), test.want)
			if response.Header().Get("Location") != "" {
				t.Fatalf("Location = %q", response.Header().Get("Location"))
			}
		})
	}

	redirect := serve(handler, http.MethodGet, "/en")
	if redirect.Code != http.StatusPermanentRedirect || redirect.Header().Get("Location") != "/en/" {
		t.Fatalf("/en redirect status=%d Location=%q", redirect.Code, redirect.Header().Get("Location"))
	}
}

func TestEmbeddedAssetsArePinnedAndPrivate(t *testing.T) {
	handler := webui.NewHandler(nil)
	javascript := serve(handler, http.MethodGet, "/assets/htmx-2.0.10.min.js")
	if javascript.Code != http.StatusOK || javascript.Header().Get("Content-Type") != "text/javascript; charset=utf-8" {
		t.Fatalf("javascript status=%d content-type=%q", javascript.Code, javascript.Header().Get("Content-Type"))
	}
	digest := sha256.Sum256(javascript.Body.Bytes())
	if got := fmt.Sprintf("%x", digest); got != "71ea67185bfa8c98c39d31717c6fce5d852370fcdfd129db4543774d3145c0de" {
		t.Fatalf("htmx SHA-256 = %s", got)
	}
	if javascript.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("htmx Cache-Control = %q", javascript.Header().Get("Cache-Control"))
	}
	css := serve(handler, http.MethodGet, "/assets/app.css")
	if css.Code != http.StatusOK || css.Body.Len() == 0 || css.Header().Get("Content-Type") != "text/css; charset=utf-8" {
		t.Fatalf("CSS status=%d bytes=%d content-type=%q", css.Code, css.Body.Len(), css.Header().Get("Content-Type"))
	}
}

func TestUIRepositoryFailureIsPrivateSafe(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	if err := database.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	response := serve(webui.NewHandler(store), http.MethodGet, "/")
	assertHTMLResponse(t, response, http.StatusInternalServerError)
	if strings.Contains(response.Body.String(), "database is closed") || strings.Contains(response.Body.String(), "SQL") {
		t.Fatalf("private detail leaked: %s", response.Body.String())
	}
}

func openUIDatabase(t *testing.T) *sql.DB {
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
	return database
}

func serve(handler http.Handler, method, path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, path, nil))
	return response
}

func assertHTMLResponse(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status || response.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("status=%d content-type=%q body=%s", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
	for header, want := range map[string]string{
		"Cache-Control": "no-store", "Referrer-Policy": "no-referrer", "X-Content-Type-Options": "nosniff",
	} {
		if got := response.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	if csp := response.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Errorf("Content-Security-Policy = %q", csp)
	}
}

func assertContainsAll(t *testing.T, body string, values []string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(body, value) {
			t.Fatalf("body does not contain %q: %s", value, body)
		}
	}
}

func assertAccountingTextPreserved(t *testing.T, japanese, english string, values []string) {
	t.Helper()
	for _, value := range values {
		if !strings.Contains(japanese, value) {
			t.Fatalf("Japanese body does not contain %q: %s", value, japanese)
		}
		if !strings.Contains(english, value) {
			t.Fatalf("English body does not contain %q: %s", value, english)
		}
	}
}
