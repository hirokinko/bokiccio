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
	if request.URL.Path == "/en" {
		http.Redirect(response, request, "/en/", http.StatusPermanentRedirect)
		return
	}
	if request.URL.Path == "/assets/app.css" {
		handler.asset(response, request, "assets/app.css", "text/css; charset=utf-8")
		return
	}
	if request.URL.Path == "/assets/htmx-2.0.10.min.js" {
		handler.asset(response, request, "assets/htmx-2.0.10.min.js", "text/javascript; charset=utf-8")
		return
	}

	requestLocale, localPath := localeRoute(request.URL.Path)
	switch {
	case localPath == "/":
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath)
			return
		}
		handler.index(response, request, requestLocale)
	case strings.HasPrefix(localPath, "/entries/"):
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath)
			return
		}
		handler.entry(response, request, requestLocale, strings.TrimPrefix(localPath, "/entries/"))
	case strings.HasPrefix(localPath, "/imports/"):
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath)
			return
		}
		handler.run(response, request, requestLocale, strings.TrimPrefix(localPath, "/imports/"))
	default:
		handler.notFound(response, request, requestLocale)
	}
}

func (handler *Handler) index(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	page, err := handler.repository.ListEntries(request.Context(), webapp.EntryQuery{Limit: defaultPageSize})
	if err != nil {
		handler.internalError(response, request, requestLocale)
		return
	}
	model := indexPageModel{
		Page:    newPageContext(requestLocale, "/"),
		Entries: make([]entrySummaryModel, 0, len(page.Entries)),
	}
	for _, entry := range page.Entries {
		model.Entries = append(model.Entries, entrySummaryModel{
			Href: entryHref(requestLocale, entry.ID), OccurredAt: entry.OccurredAt,
			Description: entry.Description, Status: entry.Status, WorkflowStatus: entry.WorkflowStatus,
			CurrentRevision: entry.CurrentRevision, Source: entry.Source,
		})
	}
	render(response, request, http.StatusOK, indexPage(model))
}

func (handler *Handler) entry(response http.ResponseWriter, request *http.Request, requestLocale locale, escapedID string) {
	id, ok := pathID(escapedID)
	if !ok {
		handler.notFound(response, request, requestLocale)
		return
	}
	detail, err := handler.repository.GetEntry(request.Context(), id)
	if errors.Is(err, webapp.ErrNotFound) {
		handler.notFound(response, request, requestLocale)
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale)
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
		Page: newPageContext(requestLocale, "/entries/"+url.PathEscape(id)), Detail: detail, Current: current,
		RunHref: runHref(requestLocale, detail.RunIdentity),
	}))
}

func (handler *Handler) run(response http.ResponseWriter, request *http.Request, requestLocale locale, escapedID string) {
	id, ok := pathID(escapedID)
	if !ok {
		handler.notFound(response, request, requestLocale)
		return
	}
	detail, err := handler.repository.GetRun(request.Context(), id)
	if errors.Is(err, webapp.ErrNotFound) {
		handler.notFound(response, request, requestLocale)
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale)
		return
	}
	model := runPageModel{
		Page:     newPageContext(requestLocale, "/imports/"+url.PathEscape(id)),
		Detail:   detail,
		Outcomes: make([]outcomePageModel, 0, len(detail.Outcomes)),
	}
	for _, outcome := range detail.Outcomes {
		outcomeEntryHref := ""
		if outcome.EntryID != "" {
			outcomeEntryHref = entryHref(requestLocale, outcome.EntryID)
		}
		model.Outcomes = append(model.Outcomes, outcomePageModel{Detail: outcome, EntryHref: outcomeEntryHref})
	}
	render(response, request, http.StatusOK, runPage(model))
}

func (handler *Handler) asset(response http.ResponseWriter, request *http.Request, name, contentType string) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		handler.methodNotAllowed(response, request, localeJA, request.URL.Path)
		return
	}
	data, err := assetFiles.ReadFile(name)
	if err != nil {
		handler.notFound(response, request, localeJA)
		return
	}
	response.Header().Set("Content-Type", contentType)
	response.Header().Set("Content-Length", strconv.Itoa(len(data)))
	response.WriteHeader(http.StatusOK)
	if request.Method != http.MethodHead {
		_, _ = response.Write(data)
	}
}

func (handler *Handler) methodNotAllowed(response http.ResponseWriter, request *http.Request, requestLocale locale, localPath string) {
	msg := messagesFor(requestLocale)
	response.Header().Set("Allow", "GET, HEAD")
	render(response, request, http.StatusMethodNotAllowed, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, localPath), Status: http.StatusMethodNotAllowed,
		Title: msg.MethodNotAllowedTitle, Message: msg.MethodNotAllowedMessage,
	}))
}

func (handler *Handler) notFound(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	msg := messagesFor(requestLocale)
	render(response, request, http.StatusNotFound, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: http.StatusNotFound,
		Title: msg.NotFoundTitle, Message: msg.NotFoundMessage,
	}))
}

func (handler *Handler) internalError(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	msg := messagesFor(requestLocale)
	render(response, request, http.StatusInternalServerError, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: http.StatusInternalServerError,
		Title: msg.InternalErrorTitle, Message: msg.InternalErrorMessage,
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

func localeRoute(path string) (locale, string) {
	if strings.HasPrefix(path, "/en/") {
		localPath := strings.TrimPrefix(path, "/en")
		if localPath == "" {
			localPath = "/"
		}
		return localeEN, localPath
	}
	return localeJA, path
}

func setPrivateHeaders(response http.ResponseWriter) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Security-Policy", "default-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
	response.Header().Set("Referrer-Policy", "no-referrer")
	response.Header().Set("X-Content-Type-Options", "nosniff")
}
