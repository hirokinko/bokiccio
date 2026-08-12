package webui_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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

func TestUIRevisionAndApprovalForms(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	result, err := store.Import(context.Background(), []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-workflow","display":"fixtures/workflow.json","external_id":"workflow-entry"},"occurred_at":"2026-08-11","description":"Workflow candidate","comments":["original comment"],"postings":[{"account":"費用:確認","amount":"120","commodity":"JPY","comment":"old item"},{"account":"資産:現金"}]}]}`))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil || len(run.Outcomes) != 1 || run.Outcomes[0].EntryID == "" {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	entryID := run.Outcomes[0].EntryID
	handler := webui.NewHandler(store)

	entry := serve(handler, http.MethodGet, "/entries/"+url.PathEscape(entryID))
	assertHTMLResponse(t, entry, http.StatusOK)
	assertContainsAll(t, entry.Body.String(), []string{
		`action="/ui/entries/` + url.PathEscape(entryID) + `/revisions"`,
		`action="/ui/entries/` + url.PathEscape(entryID) + `/approvals"`,
		`name="entry"`, `data-tab-inserts-spaces="true"`, "2026-08-11  &#39;Workflow candidate", "    ; original comment",
		"    費用:確認 120 JPY ; old item", "    資産:現金", "revisionを保存", "承認",
	})

	revisionForm := url.Values{
		"base_revision": {"0"},
		"entry":         {"2026-08-12  'Workflow revised\n; first comment\n; second comment\n費用:確認 150 JPY ; keyboard edited\n資産:現金"},
	}
	revision := serveForm(handler, "/ui/entries/"+url.PathEscape(entryID)+"/revisions", revisionForm, nil)
	if revision.Code != http.StatusSeeOther || revision.Header().Get("Location") != "/entries/"+url.PathEscape(entryID) {
		t.Fatalf("revision status=%d Location=%q body=%s", revision.Code, revision.Header().Get("Location"), revision.Body.String())
	}
	revised := serve(handler, http.MethodGet, revision.Header().Get("Location"))
	assertHTMLResponse(t, revised, http.StatusOK)
	assertContainsAll(t, revised.Body.String(), []string{
		"Current candidate · revision 1", "Workflow revised", "first comment", "second comment",
		"keyboard edited", "150 JPY", "Revision 1",
	})

	approval := serveForm(handler, "/ui/entries/"+url.PathEscape(entryID)+"/approvals", url.Values{"revision": {"1"}}, nil)
	if approval.Code != http.StatusSeeOther || approval.Header().Get("Location") != "/entries/"+url.PathEscape(entryID) {
		t.Fatalf("approval status=%d Location=%q body=%s", approval.Code, approval.Header().Get("Location"), approval.Body.String())
	}
	approved := serve(handler, http.MethodGet, approval.Header().Get("Location"))
	assertHTMLResponse(t, approved, http.StatusOK)
	assertContainsAll(t, approved.Body.String(), []string{"Current approval: revision 1", "現在のrevisionは承認済みです。"})

	tacklerExport := serveForm(handler, "/ui/exports/tackler", url.Values{"description": {"Workflow revised"}}, nil)
	if tacklerExport.Code != http.StatusOK || tacklerExport.Header().Get("Content-Type") != "text/plain; charset=utf-8" ||
		tacklerExport.Header().Get("Content-Disposition") != `attachment; filename="bokiccio-export.txn"` {
		t.Fatalf("Tackler export status=%d headers=%v body=%s", tacklerExport.Code, tacklerExport.Header(), tacklerExport.Body.String())
	}
	assertContainsAll(t, tacklerExport.Body.String(), []string{"Workflow revised", "150 JPY", "keyboard edited"})
	assertNotContainsAny(t, tacklerExport.Body.String(), []string{"?description"})

	jsonExport := serveForm(handler, "/en/ui/exports/json", url.Values{"description": {"Workflow revised"}}, nil)
	if jsonExport.Code != http.StatusOK || jsonExport.Header().Get("Content-Type") != "application/json; charset=utf-8" ||
		jsonExport.Header().Get("Content-Disposition") != `attachment; filename="bokiccio-export.json"` {
		t.Fatalf("JSON export status=%d headers=%v body=%s", jsonExport.Code, jsonExport.Header(), jsonExport.Body.String())
	}
	assertContainsAll(t, jsonExport.Body.String(), []string{`"schema_version":1`, `"description":"Workflow revised"`, `"revision":1`})

	stale := serveForm(handler, "/ui/entries/"+url.PathEscape(entryID)+"/revisions", url.Values{
		"base_revision": {"0"}, "entry": {"2026-08-13  'private stale\n費用:確認 1 JPY\n資産:現金"},
	}, nil)
	assertHTMLResponse(t, stale, http.StatusConflict)
	assertContainsAll(t, stale.Body.String(), []string{"操作を完了できませんでした", "仕訳候補が更新されています。"})
	assertNotContainsAny(t, stale.Body.String(), []string{"private stale", `{"schema_version"`})

	english := serveForm(handler, "/en/ui/entries/"+url.PathEscape(entryID)+"/approvals", url.Values{"revision": {"0"}}, nil)
	assertHTMLResponse(t, english, http.StatusConflict)
	assertContainsAll(t, english.Body.String(), []string{`<html lang="en">`, "The operation could not be completed"})
}

func TestUIApprovalRejectsInvalidRevisionAsHTML(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	result, err := store.Import(context.Background(), []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-invalid","display":"fixtures/invalid.json","external_id":"invalid-entry"},"occurred_at":"2026-08-11","description":"Invalid workflow candidate","postings":[{"account":"費用:確認","amount":"120","commodity":"JPY"},{"account":"資産:現金"}]}]}`))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil || len(run.Outcomes) != 1 || run.Outcomes[0].EntryID == "" {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	entryID := run.Outcomes[0].EntryID
	handler := webui.NewHandler(store)

	invalid := serveForm(handler, "/ui/entries/"+url.PathEscape(entryID)+"/revisions", url.Values{
		"base_revision": {"0"}, "entry": {"2026-08-12  'Invalid revision\n費用:確認 10 JPY\n資産:現金 5 JPY"},
	}, nil)
	if invalid.Code != http.StatusSeeOther {
		t.Fatalf("invalid revision save status=%d body=%s", invalid.Code, invalid.Body.String())
	}
	page := serve(handler, http.MethodGet, invalid.Header().Get("Location"))
	assertHTMLResponse(t, page, http.StatusOK)
	assertContainsAll(t, page.Body.String(), []string{"Revision 1", "valid: false", "現在のrevisionはvalidation errorがあるため承認できません。"})

	approval := serveForm(handler, "/ui/entries/"+url.PathEscape(entryID)+"/approvals", url.Values{"revision": {"1"}}, nil)
	assertHTMLResponse(t, approval, http.StatusUnprocessableEntity)
	assertContainsAll(t, approval.Body.String(), []string{"validation errorがあるrevisionは承認できません。"})
	assertNotContainsAny(t, approval.Body.String(), []string{`{"schema_version"`})
}

func TestUIRevisionFormsRejectInvalidInputPrivately(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	result, err := store.Import(context.Background(), []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-private","display":"fixtures/private.json","external_id":"private-entry"},"occurred_at":"2026-08-11","description":"Private candidate","postings":[{"account":"費用:確認","amount":"120","commodity":"JPY"},{"account":"資産:現金"}]}]}`))
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	run, err := store.GetRun(context.Background(), result.RunIdentity)
	if err != nil || len(run.Outcomes) != 1 || run.Outcomes[0].EntryID == "" {
		t.Fatalf("GetRun() error=%v run=%+v", err, run)
	}
	entryID := run.Outcomes[0].EntryID
	handler := webui.NewHandler(store)

	for _, test := range []struct {
		name        string
		path        string
		contentType string
		body        string
	}{
		{name: "wrong media", path: "/ui/entries/" + url.PathEscape(entryID) + "/revisions", contentType: "text/plain", body: "description=private-value"},
		{name: "unknown field", path: "/ui/entries/" + url.PathEscape(entryID) + "/revisions", contentType: "application/x-www-form-urlencoded", body: "base_revision=0&entry=a&unknown=private-value"},
		{name: "query string", path: "/ui/entries/" + url.PathEscape(entryID) + "/approvals?revision=private-value", contentType: "application/x-www-form-urlencoded", body: "revision=0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := serveRawForm(handler, test.path, test.contentType, test.body, nil)
			assertHTMLResponse(t, response, http.StatusBadRequest)
			assertNotContainsAny(t, response.Body.String(), []string{"private-value", "unknown", `{"schema_version"`})
		})
	}

	method := serve(webui.NewHandler(nil), http.MethodGet, "/ui/entries/"+url.PathEscape(entryID)+"/revisions")
	assertHTMLResponse(t, method, http.StatusMethodNotAllowed)
	if method.Header().Get("Allow") != "POST" {
		t.Fatalf("Allow = %q", method.Header().Get("Allow"))
	}
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

func TestUIImportUploadRedirectsToRunPage(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	handler := webui.NewHandler(store)
	input := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-upload","display":"fixtures/upload.json","external_id":"upload-entry"},"occurred_at":"2026-08-11","description":"Uploaded candidate","warnings":[{"code":"ui.upload","message":"Needs review"}],"postings":[{"account":"費用:取込","amount":"1200","commodity":"JPY"},{"account":"資産:現金"}]}]}`)

	response := serveUpload(handler, "/ui/imports", "private-upload.json", input)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("upload status=%d body=%s", response.Code, response.Body.String())
	}
	location := response.Header().Get("Location")
	if !strings.HasPrefix(location, "/imports/") {
		t.Fatalf("Location = %q", location)
	}
	assertNotContainsAny(t, response.Body.String(), []string{"private-upload.json", string(input)})

	runPage := serve(handler, http.MethodGet, location)
	assertHTMLResponse(t, runPage, http.StatusOK)
	assertContainsAll(t, runPage.Body.String(), []string{"取込結果", "fixtures/upload.json", "完了", "/entries/"})

	englishInput := []byte(`{"schema_version":1,"records":[{"source":{"namespace":"ui-upload","display":"fixtures/upload-en.json","external_id":"upload-entry-en"},"occurred_at":"2026-08-12","description":"English uploaded candidate","postings":[{"account":"費用:取込","amount":"1500","commodity":"JPY"},{"account":"資産:現金"}]}]}`)
	english := serveUpload(handler, "/en/ui/imports", "private-english.json", englishInput)
	if english.Code != http.StatusSeeOther || !strings.HasPrefix(english.Header().Get("Location"), "/en/imports/") {
		t.Fatalf("English upload status=%d Location=%q body=%s", english.Code, english.Header().Get("Location"), english.Body.String())
	}
}

func TestUITacklerImportUploadRedirectsToRunPage(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	handler := webui.NewHandler(store)
	input := []byte("2026-08-12T07:30:00+09:00  'Txn uploaded candidate\n    ; source note\n    費用:取込 1200 JPY ; imported item\n    資産:現金\n")

	index := serve(handler, http.MethodGet, "/")
	assertHTMLResponse(t, index, http.StatusOK)
	assertContainsAll(t, index.Body.String(), []string{`action="/ui/imports/tackler"`, `accept="text/plain,.txn"`, "Tackler import"})

	response := serveUpload(handler, "/ui/imports/tackler", "private-upload.txn", input)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("txn upload status=%d body=%s", response.Code, response.Body.String())
	}
	location := response.Header().Get("Location")
	if !strings.HasPrefix(location, "/imports/") {
		t.Fatalf("Location = %q", location)
	}
	assertNotContainsAny(t, response.Body.String(), []string{"private-upload.txn", string(input)})

	runPage := serve(handler, http.MethodGet, location)
	assertHTMLResponse(t, runPage, http.StatusOK)
	assertContainsAll(t, runPage.Body.String(), []string{"取込結果", "tackler: uploaded.txn", "完了", "/entries/"})
	assertNotContainsAny(t, runPage.Body.String(), []string{"private-upload.txn"})

	duplicate := serveUpload(handler, "/en/ui/imports/tackler", "private-upload-again.txn", input)
	if duplicate.Code != http.StatusSeeOther || !strings.HasPrefix(duplicate.Header().Get("Location"), "/en/imports/") {
		t.Fatalf("duplicate status=%d Location=%q body=%s", duplicate.Code, duplicate.Header().Get("Location"), duplicate.Body.String())
	}
	duplicateRun := serve(handler, http.MethodGet, duplicate.Header().Get("Location"))
	assertHTMLResponse(t, duplicateRun, http.StatusOK)
	assertContainsAll(t, duplicateRun.Body.String(), []string{"duplicate", "tackler: uploaded.txn"})
}

func TestUIImportUploadCommitsRunWithRecordErrors(t *testing.T) {
	store := webstore.New(openUIDatabase(t))
	handler := webui.NewHandler(store)

	response := serveUpload(handler, "/ui/imports", "mixed-private.json", readWebFixture(t, "../ingest/testdata/mixed-outcomes-v1.json"))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("upload status=%d body=%s", response.Code, response.Body.String())
	}
	runPage := serve(handler, http.MethodGet, response.Header().Get("Location"))
	assertHTMLResponse(t, runPage, http.StatusOK)
	assertContainsAll(t, runPage.Body.String(), []string{"取込結果", "要確認", "error"})
}

func TestUIImportUploadRejectsInvalidInputPrivately(t *testing.T) {
	handler := webui.NewHandler(webstore.New(openUIDatabase(t)))
	for _, test := range []struct {
		name   string
		path   string
		parts  []uploadPart
		status int
	}{
		{name: "invalid json", path: "/ui/imports", parts: []uploadPart{{field: "file", filename: "secret-invalid.json", body: []byte("private-content")}}, status: http.StatusBadRequest},
		{name: "missing file", path: "/ui/imports", status: http.StatusBadRequest},
		{name: "extra field", path: "/ui/imports", parts: []uploadPart{{field: "note", body: []byte("private-note")}, {field: "file", filename: "valid.json", body: []byte(`{"schema_version":1,"records":[]}`)}}, status: http.StatusBadRequest},
		{name: "multiple files", path: "/ui/imports", parts: []uploadPart{{field: "file", filename: "first-secret.json", body: []byte(`{"schema_version":1,"records":[]}`)}, {field: "file", filename: "second-secret.json", body: []byte(`{"schema_version":1,"records":[]}`)}}, status: http.StatusBadRequest},
		{name: "query string", path: "/ui/imports?source=private", parts: []uploadPart{{field: "file", filename: "valid.json", body: []byte(`{"schema_version":1,"records":[]}`)}}, status: http.StatusBadRequest},
		{name: "too large", path: "/ui/imports", parts: []uploadPart{{field: "file", filename: "too-large-private.json", body: bytes.Repeat([]byte("x"), (10<<20)+1)}}, status: http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := serveMultipart(handler, test.path, test.parts)
			assertHTMLResponse(t, response, test.status)
			assertNotContainsAny(t, response.Body.String(), []string{
				"secret-invalid.json", "private-content", "private-note", "first-secret.json",
				"second-secret.json", "source=private", "too-large-private.json",
			})
		})
	}

	wrongMedia := serveRawForm(handler, "/en/ui/imports", "text/plain", "private-content", nil)
	assertHTMLResponse(t, wrongMedia, http.StatusUnsupportedMediaType)
	assertContainsAll(t, wrongMedia.Body.String(), []string{`<html lang="en">`, "Unsupported upload"})
	assertNotContainsAny(t, wrongMedia.Body.String(), []string{"private-content"})

	method := serve(webui.NewHandler(nil), http.MethodGet, "/ui/imports")
	assertHTMLResponse(t, method, http.StatusMethodNotAllowed)
	if method.Header().Get("Allow") != "POST" {
		t.Fatalf("Allow = %q", method.Header().Get("Allow"))
	}

	invalidTxn := serveUpload(handler, "/ui/imports/tackler", "secret-invalid.txn", []byte("private-content"))
	assertHTMLResponse(t, invalidTxn, http.StatusBadRequest)
	assertContainsAll(t, invalidTxn.Body.String(), []string{"Invalid Tackler upload", "line 1", "entry header is required"})
	assertNotContainsAny(t, invalidTxn.Body.String(), []string{"secret-invalid.txn", "private-content", `{"schema_version"`})

	invalidAmount := serveUpload(handler, "/ui/imports/tackler", "secret-amount.txn", []byte("2026-08-10  'private shop\n    費用:確認 private-amount JPY\n    資産:現金\n"))
	assertHTMLResponse(t, invalidAmount, http.StatusBadRequest)
	assertContainsAll(t, invalidAmount.Body.String(), []string{"Invalid Tackler upload", "line 2", "private-amount"})
	assertNotContainsAny(t, invalidAmount.Body.String(), []string{"secret-amount.txn", `{"schema_version"`})
}

func TestUIImportUploadRepositoryFailureIsPrivateSafe(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	if err := database.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	response := serveUpload(webui.NewHandler(store), "/ui/imports", "private-storage.json", []byte(`{"schema_version":1,"records":[]}`))
	assertHTMLResponse(t, response, http.StatusInternalServerError)
	assertNotContainsAny(t, response.Body.String(), []string{"database is closed", "SQL", "private-storage.json", "schema_version"})
}

func TestUISearchFullPageAndHtmxFragment(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	input := []byte(`{"schema_version":1,"records":[
{"source":{"namespace":"receipt","display":"fixtures/alpha.json","external_id":"ui-search-alpha"},"occurred_at":"2026-08-01","description":"Alpha candidate","postings":[{"account":"費用:食費","amount":"100","commodity":"JPY"},{"account":"資産:現金"}]},
{"source":{"namespace":"mail","display":"fixtures/beta.json","external_id":"ui-search-beta"},"occurred_at":"2026-08-02","description":"Beta <needle>","warnings":[{"code":"ui.search","message":"Preserve warning"}],"postings":[{"account":"費用:交通","amount":"200","commodity":"JPY"},{"account":"資産:現金"}]},
{"source":{"namespace":"receipt","display":"fixtures/gamma.json","external_id":"ui-search-gamma"},"occurred_at":"2026-08-03","description":"Gamma candidate","postings":[{"account":"費用:日用品","amount":"300","commodity":"JPY"},{"account":"資産:現金"}]}
]}`)
	if _, err := store.Import(context.Background(), input); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	handler := webui.NewHandler(store)

	full := serveForm(handler, "/ui/entries/search", url.Values{"description": {"Beta <needle>"}}, nil)
	assertHTMLResponse(t, full, http.StatusOK)
	assertContainsAll(t, full.Body.String(), []string{
		`<html lang="ja">`, `action="/ui/entries/search"`, `value="Beta &lt;needle&gt;"`,
		"Beta &lt;needle&gt;", "mail: fixtures/beta.json", "1件を表示",
	})
	assertNotContainsAny(t, full.Body.String(), []string{"Alpha candidate", "Gamma candidate", "?description", "<needle>"})
	if vary := full.Header().Get("Vary"); !strings.Contains(vary, "HX-Request") {
		t.Fatalf("Vary = %q", vary)
	}

	fragment := serveForm(handler, "/en/ui/entries/search", url.Values{"source_namespace": {"mail"}}, map[string]string{"HX-Request": "true"})
	assertHTMLResponse(t, fragment, http.StatusOK)
	assertContainsAll(t, fragment.Body.String(), []string{
		`id="entry-results"`, "Search results", "Beta &lt;needle&gt;", `/en/entries/`,
	})
	assertNotContainsAny(t, fragment.Body.String(), []string{"<!doctype html>", `<html lang="en">`, "Alpha candidate", "Gamma candidate", "?source_namespace"})
}

func TestUISearchPaginationUsesFormBodyState(t *testing.T) {
	database := openUIDatabase(t)
	store := webstore.New(database)
	if _, err := store.Import(context.Background(), manyEntriesInput(51)); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	handler := webui.NewHandler(store)

	first := serveForm(handler, "/ui/entries/search", url.Values{}, nil)
	assertHTMLResponse(t, first, http.StatusOK)
	assertContainsAll(t, first.Body.String(), []string{"Page 50", "Page 01", "次のページ", `name="cursor"`})
	assertNotContainsAny(t, first.Body.String(), []string{"Page 00", "?cursor"})
	cursor := hiddenInputValue(t, first.Body.String(), "cursor")

	second := serveForm(handler, "/ui/entries/search", url.Values{"cursor": {cursor}}, map[string]string{"HX-Request": "true"})
	assertHTMLResponse(t, second, http.StatusOK)
	assertContainsAll(t, second.Body.String(), []string{`id="entry-results"`, "Page 00"})
	assertNotContainsAny(t, second.Body.String(), []string{"Page 50", "次のページ", "?cursor"})

	staleCursor := serveForm(handler, "/ui/entries/search", url.Values{"description": {"Page"}, "cursor": {cursor}}, nil)
	assertHTMLResponse(t, staleCursor, http.StatusBadRequest)
	assertContainsAll(t, staleCursor.Body.String(), []string{"Invalid search", "検索条件を処理できませんでした。"})
	assertNotContainsAny(t, staleCursor.Body.String(), []string{cursor, "Page 50"})
}

func TestUISearchRejectsInvalidFormPrivately(t *testing.T) {
	handler := webui.NewHandler(webstore.New(openUIDatabase(t)))
	for _, test := range []struct {
		name        string
		path        string
		contentType string
		body        string
	}{
		{name: "wrong media type", path: "/ui/entries/search", contentType: "text/plain", body: "description=private-value"},
		{name: "unknown field", path: "/ui/entries/search", contentType: "application/x-www-form-urlencoded", body: "unknown=private-value"},
		{name: "duplicate field", path: "/ui/entries/search", contentType: "application/x-www-form-urlencoded", body: "description=a&description=b"},
		{name: "query string", path: "/ui/entries/search?description=private-value", contentType: "application/x-www-form-urlencoded", body: ""},
		{name: "invalid date", path: "/en/ui/entries/search", contentType: "application/x-www-form-urlencoded", body: "date_from=2026-02-30"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := serveRawForm(handler, test.path, test.contentType, test.body, nil)
			assertHTMLResponse(t, response, http.StatusBadRequest)
			assertNotContainsAny(t, response.Body.String(), []string{"private-value", "2026-02-30", "unknown"})
		})
	}

	method := serve(webui.NewHandler(nil), http.MethodGet, "/ui/entries/search")
	assertHTMLResponse(t, method, http.StatusMethodNotAllowed)
	if method.Header().Get("Allow") != "POST" {
		t.Fatalf("Allow = %q", method.Header().Get("Allow"))
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
	app := serve(handler, http.MethodGet, "/assets/app.js")
	if app.Code != http.StatusOK || !strings.Contains(app.Body.String(), "tabInsertsSpaces") || app.Header().Get("Content-Type") != "text/javascript; charset=utf-8" {
		t.Fatalf("app.js status=%d content-type=%q body=%s", app.Code, app.Header().Get("Content-Type"), app.Body.String())
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

func serveForm(handler http.Handler, path string, form url.Values, headers map[string]string) *httptest.ResponseRecorder {
	return serveRawForm(handler, path, "application/x-www-form-urlencoded", form.Encode(), headers)
}

func serveRawForm(handler http.Handler, path, contentType, body string, headers map[string]string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	handler.ServeHTTP(response, request)
	return response
}

type uploadPart struct {
	field    string
	filename string
	body     []byte
}

func serveUpload(handler http.Handler, path, filename string, body []byte) *httptest.ResponseRecorder {
	return serveMultipart(handler, path, []uploadPart{{field: "file", filename: filename, body: body}})
}

func serveMultipart(handler http.Handler, path string, parts []uploadPart) *httptest.ResponseRecorder {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, part := range parts {
		var file io.Writer
		var err error
		if part.filename == "" {
			file, err = writer.CreateFormField(part.field)
		} else {
			file, err = writer.CreateFormFile(part.field, part.filename)
		}
		if err != nil {
			panic(err)
		}
		if _, err := file.Write(part.body); err != nil {
			panic(err)
		}
	}
	if err := writer.Close(); err != nil {
		panic(err)
	}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	handler.ServeHTTP(response, request)
	return response
}

func readWebFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	return data
}

func assertHTMLResponse(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status || response.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("status=%d content-type=%q body=%s", response.Code, response.Header().Get("Content-Type"), response.Body.String())
	}
	for header, want := range map[string]string{
		"Cache-Control": "no-store", "Referrer-Policy": "same-origin", "X-Content-Type-Options": "nosniff",
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

func assertNotContainsAny(t *testing.T, body string, values []string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(body, value) {
			t.Fatalf("body contains %q: %s", value, body)
		}
	}
}

func hiddenInputValue(t *testing.T, body, name string) string {
	t.Helper()
	prefix := `name="` + name + `" value="`
	index := strings.Index(body, prefix)
	if index < 0 {
		t.Fatalf("hidden input %q not found: %s", name, body)
	}
	start := index + len(prefix)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		t.Fatalf("hidden input %q value is not closed: %s", name, body)
	}
	value := body[start : start+end]
	if value == "" {
		t.Fatalf("hidden input %q value is empty", name)
	}
	return value
}

func manyEntriesInput(count int) []byte {
	var builder strings.Builder
	builder.WriteString(`{"schema_version":1,"records":[`)
	for index := range count {
		if index > 0 {
			builder.WriteByte(',')
		}
		_, _ = fmt.Fprintf(&builder, `{"source":{"namespace":"page","display":"fixtures/page-%02d.json","external_id":"ui-page-%02d"},"occurred_at":"2026-08-11","description":"Page %02d","postings":[{"account":"費用:確認","amount":"1","commodity":"UNIT"},{"account":"資産:確認"}]}`, index, index, index)
	}
	builder.WriteString(`]}`)
	return []byte(builder.String())
}
