// Package webui provides Bokiccio's server-rendered owner interface.
package webui

import (
	"embed"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

const defaultPageSize = 50

//go:embed assets/app.css assets/htmx-2.0.10.min.js
var assetFiles embed.FS

type Handler struct {
	repository webapp.Repository
}

func NewHandler(repository webapp.Repository) *Handler {
	return &Handler{repository: repository}
}

func (handler *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	setPrivateHeaders(response)
	switch {
	case request.URL.Path == "/":
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request)
			return
		}
		handler.index(response, request)
	case strings.HasPrefix(request.URL.Path, "/entries/"):
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request)
			return
		}
		handler.entry(response, request, strings.TrimPrefix(request.URL.Path, "/entries/"))
	case strings.HasPrefix(request.URL.Path, "/imports/"):
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request)
			return
		}
		handler.run(response, request, strings.TrimPrefix(request.URL.Path, "/imports/"))
	case request.URL.Path == "/assets/app.css":
		handler.asset(response, request, "assets/app.css", "text/css; charset=utf-8")
	case request.URL.Path == "/assets/htmx-2.0.10.min.js":
		handler.asset(response, request, "assets/htmx-2.0.10.min.js", "text/javascript; charset=utf-8")
	default:
		handler.notFound(response, request)
	}
}

func (handler *Handler) index(response http.ResponseWriter, request *http.Request) {
	page, err := handler.repository.ListEntries(request.Context(), webapp.EntryQuery{Limit: defaultPageSize})
	if err != nil {
		handler.internalError(response, request)
		return
	}
	model := indexPageModel{Entries: make([]entrySummaryModel, 0, len(page.Entries))}
	for _, entry := range page.Entries {
		model.Entries = append(model.Entries, entrySummaryModel{
			Href: "/entries/" + url.PathEscape(entry.ID), OccurredAt: entry.OccurredAt,
			Description: entry.Description, Status: entry.Status, WorkflowStatus: entry.WorkflowStatus,
			CurrentRevision: entry.CurrentRevision, Source: entry.Source,
		})
	}
	render(response, request, http.StatusOK, indexPage(model))
}

func (handler *Handler) entry(response http.ResponseWriter, request *http.Request, escapedID string) {
	id, ok := pathID(escapedID)
	if !ok {
		handler.notFound(response, request)
		return
	}
	detail, err := handler.repository.GetEntry(request.Context(), id)
	if errors.Is(err, webapp.ErrNotFound) {
		handler.notFound(response, request)
		return
	}
	if err != nil {
		handler.internalError(response, request)
		return
	}
	current := candidateModel{
		Revision: detail.CurrentRevision, OccurredAt: detail.OccurredAt, Description: detail.Description,
		Comments: detail.Comments, Postings: detail.Postings,
	}
	if len(detail.Revisions) > 0 {
		latest := detail.Revisions[len(detail.Revisions)-1]
		current = candidateModel{
			Revision: latest.Revision, OccurredAt: latest.OccurredAt, Description: latest.Description,
			Comments: latest.Comments, Postings: latest.Postings,
		}
	}
	render(response, request, http.StatusOK, entryPage(entryPageModel{
		Detail: detail, Current: current, RunHref: "/imports/" + url.PathEscape(detail.RunIdentity),
	}))
}

func (handler *Handler) run(response http.ResponseWriter, request *http.Request, escapedID string) {
	id, ok := pathID(escapedID)
	if !ok {
		handler.notFound(response, request)
		return
	}
	detail, err := handler.repository.GetRun(request.Context(), id)
	if errors.Is(err, webapp.ErrNotFound) {
		handler.notFound(response, request)
		return
	}
	if err != nil {
		handler.internalError(response, request)
		return
	}
	model := runPageModel{Detail: detail, Outcomes: make([]outcomePageModel, 0, len(detail.Outcomes))}
	for _, outcome := range detail.Outcomes {
		entryHref := ""
		if outcome.EntryID != "" {
			entryHref = "/entries/" + url.PathEscape(outcome.EntryID)
		}
		model.Outcomes = append(model.Outcomes, outcomePageModel{Detail: outcome, EntryHref: entryHref})
	}
	render(response, request, http.StatusOK, runPage(model))
}

func (handler *Handler) asset(response http.ResponseWriter, request *http.Request, name, contentType string) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		handler.methodNotAllowed(response, request)
		return
	}
	data, err := assetFiles.ReadFile(name)
	if err != nil {
		handler.notFound(response, request)
		return
	}
	response.Header().Set("Content-Type", contentType)
	response.Header().Set("Content-Length", strconv.Itoa(len(data)))
	response.WriteHeader(http.StatusOK)
	if request.Method != http.MethodHead {
		_, _ = response.Write(data)
	}
}

func (handler *Handler) methodNotAllowed(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Allow", "GET, HEAD")
	render(response, request, http.StatusMethodNotAllowed, errorPage(errorPageModel{
		Status: http.StatusMethodNotAllowed, Title: "Method not allowed", Message: "この操作には対応していません。",
	}))
}

func (handler *Handler) notFound(response http.ResponseWriter, request *http.Request) {
	render(response, request, http.StatusNotFound, errorPage(errorPageModel{
		Status: http.StatusNotFound, Title: "Not found", Message: "指定されたページは見つかりませんでした。",
	}))
}

func (handler *Handler) internalError(response http.ResponseWriter, request *http.Request) {
	render(response, request, http.StatusInternalServerError, errorPage(errorPageModel{
		Status: http.StatusInternalServerError, Title: "Internal error", Message: "ページを表示できませんでした。",
	}))
}

func render(response http.ResponseWriter, request *http.Request, status int, component templ.Component) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(status)
	if request.Method == http.MethodHead {
		return
	}
	_ = component.Render(request.Context(), response)
}

func pathID(escaped string) (string, bool) {
	if escaped == "" || strings.Contains(escaped, "/") {
		return "", false
	}
	id, err := url.PathUnescape(escaped)
	return id, err == nil && id != "" && !strings.Contains(id, "/")
}

func setPrivateHeaders(response http.ResponseWriter) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Security-Policy", "default-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	response.Header().Set("Referrer-Policy", "no-referrer")
	response.Header().Set("X-Content-Type-Options", "nosniff")
}
