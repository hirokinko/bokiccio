// Package webui provides Bokiccio's server-rendered owner interface.
package webui

import (
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/hirokinko/bokiccio/internal/ingest"
	"github.com/hirokinko/bokiccio/internal/ledger"
	"github.com/hirokinko/bokiccio/internal/tacklerfmt"
	"github.com/hirokinko/bokiccio/internal/webapp"
)

const (
	defaultPageSize      = 50
	maxSearchFormBody    = 16 << 10
	maxRevisionFormBody  = 64 << 10
	maxImportFileSize    = 10 << 20
	maxImportRequestSize = maxImportFileSize + (64 << 10)
	importFileField      = "file"
)

//go:embed assets/app.css assets/app.js assets/htmx-2.0.10.min.js
var assetFiles embed.FS

type Handler struct {
	repository webapp.Repository
}

func NewHandler(repository webapp.Repository) *Handler {
	return &Handler{repository: repository}
}

func RenderSecurityError(response http.ResponseWriter, request *http.Request, securityError webapp.SecurityError) {
	setPrivateHeaders(response)
	requestLocale, _ := localeRoute(request.URL.Path)
	msg := messagesFor(requestLocale)
	title := msg.SecurityForbiddenTitle
	message := msg.SecurityForbiddenMessage
	if securityError.Status == http.StatusUnauthorized {
		title = msg.SecurityUnauthorizedTitle
		message = msg.SecurityUnauthorizedMessage
	}
	render(response, request, securityError.Status, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: securityError.Status,
		Title: title, Message: message,
	}))
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
	if request.URL.Path == "/assets/app.js" {
		handler.asset(response, request, "assets/app.js", "text/javascript; charset=utf-8")
		return
	}

	requestLocale, localPath := localeRoute(request.URL.Path)
	switch {
	case localPath == "/":
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "GET, HEAD")
			return
		}
		handler.index(response, request, requestLocale)
	case localPath == "/ui/entries/search":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.searchEntries(response, request, requestLocale)
	case localPath == "/ui/imports":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.importRecords(response, request, requestLocale)
	case localPath == "/ui/imports/tackler":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.importTackler(response, request, requestLocale)
	case strings.HasPrefix(localPath, "/ui/exports/"):
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.exportEntries(response, request, requestLocale, strings.TrimPrefix(localPath, "/ui/exports/"))
	case strings.HasPrefix(localPath, "/ui/entries/"):
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "POST")
			return
		}
		handler.entryMutation(response, request, requestLocale, strings.TrimPrefix(localPath, "/ui/entries/"))
	case strings.HasPrefix(localPath, "/entries/"):
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "GET, HEAD")
			return
		}
		handler.entry(response, request, requestLocale, strings.TrimPrefix(localPath, "/entries/"))
	case strings.HasPrefix(localPath, "/imports/"):
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			handler.methodNotAllowed(response, request, requestLocale, localPath, "GET, HEAD")
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
	model := newIndexPageModel(requestLocale, webapp.EntryFilter{}, page, false)
	render(response, request, http.StatusOK, indexPage(model))
}

func (handler *Handler) searchEntries(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	addVary(response, "HX-Request")
	filter, cursor, ok := decodeSearchForm(response, request)
	if !ok {
		handler.invalidSearch(response, request, requestLocale)
		return
	}
	page, err := handler.repository.ListEntries(request.Context(), webapp.EntryQuery{
		Filter: filter, Limit: defaultPageSize, Cursor: cursor,
	})
	if errors.Is(err, webapp.ErrInvalidRequest) {
		handler.invalidSearch(response, request, requestLocale)
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale)
		return
	}
	model := newIndexPageModel(requestLocale, filter, page, true)
	if isHXRequest(request) {
		render(response, request, http.StatusOK, entryResults(model))
		return
	}
	render(response, request, http.StatusOK, indexPage(model))
}

func (handler *Handler) importRecords(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	body, uploadStatus, ok := decodeImportUpload(response, request)
	if !ok {
		handler.invalidUpload(response, request, requestLocale, uploadStatus)
		return
	}
	result, err := handler.repository.Import(request.Context(), body)
	if errors.Is(err, ingest.ErrInvalidInput) || errors.Is(err, webapp.ErrInvalidRequest) {
		handler.invalidUpload(response, request, requestLocale, http.StatusBadRequest)
		return
	}
	if err != nil {
		handler.uploadFailed(response, request, requestLocale)
		return
	}
	http.Redirect(response, request, runHref(requestLocale, result.RunIdentity), http.StatusSeeOther)
}

func (handler *Handler) importTackler(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	body, uploadStatus, ok := decodeImportUpload(response, request)
	if !ok {
		handler.invalidUpload(response, request, requestLocale, uploadStatus)
		return
	}
	entries, err := tacklerfmt.Parse(body)
	if err != nil {
		handler.invalidTacklerUpload(response, request, requestLocale, err)
		return
	}
	input, err := normalizedInputForTacklerEntries(entries)
	if err != nil {
		handler.invalidTacklerUpload(response, request, requestLocale, err)
		return
	}
	result, err := handler.repository.Import(request.Context(), input)
	if errors.Is(err, ingest.ErrInvalidInput) || errors.Is(err, webapp.ErrInvalidRequest) {
		handler.invalidUpload(response, request, requestLocale, http.StatusBadRequest)
		return
	}
	if err != nil {
		handler.uploadFailed(response, request, requestLocale)
		return
	}
	http.Redirect(response, request, runHref(requestLocale, result.RunIdentity), http.StatusSeeOther)
}

func (handler *Handler) exportEntries(response http.ResponseWriter, request *http.Request, requestLocale locale, format string) {
	if format != "tackler" && format != "json" {
		handler.notFound(response, request, requestLocale)
		return
	}
	filter, ok := decodeExportForm(response, request)
	if !ok {
		handler.invalidSearch(response, request, requestLocale)
		return
	}
	approved, err := handler.repository.ListApprovedEntries(request.Context(), filter)
	if errors.Is(err, webapp.ErrInvalidRequest) {
		handler.invalidSearch(response, request, requestLocale)
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale)
		return
	}
	if format == "tackler" {
		handler.exportTackler(response, approved)
		return
	}
	handler.exportJSON(response, approved)
}

func (handler *Handler) exportTackler(response http.ResponseWriter, approved []webapp.ApprovedEntry) {
	entries := make([]ledger.JournalEntry, 0, len(approved))
	for _, item := range approved {
		entries = append(entries, item.Entry)
	}
	output, err := tacklerfmt.Export(entries, tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted})
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		return
	}
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.Header().Set("Content-Disposition", `attachment; filename="bokiccio-export.txn"`)
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(output)
}

func (handler *Handler) exportJSON(response http.ResponseWriter, approved []webapp.ApprovedEntry) {
	exported := webapp.JSONExport{SchemaVersion: webapp.APISchemaVersion, Entries: []webapp.ExportEntry{}}
	for _, item := range approved {
		entry := webapp.ExportEntry{
			ID: item.ID, Revision: item.Revision, ApprovedAt: item.ApprovedAt, Source: item.Source,
			OccurredAt: item.Entry.Date.String(), Description: item.Entry.Description,
			Comments: append([]string(nil), item.Entry.Comments...), Postings: []webapp.PostingDetail{},
		}
		for _, posting := range item.Entry.Postings {
			detail := webapp.PostingDetail{Account: posting.Account, Comment: posting.Comment}
			if posting.Amount != nil {
				amount := posting.Amount.Value.String()
				detail.Amount = &amount
				detail.Commodity = string(posting.Amount.Commodity)
			}
			entry.Postings = append(entry.Postings, detail)
		}
		exported.Entries = append(exported.Entries, entry)
	}
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Content-Disposition", `attachment; filename="bokiccio-export.json"`)
	response.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(response).Encode(exported)
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
	render(response, request, http.StatusOK, entryPage(newEntryPageModel(requestLocale, id, detail, currentCandidate(detail), "")))
}

func (handler *Handler) entryMutation(response http.ResponseWriter, request *http.Request, requestLocale locale, path string) {
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		handler.notFound(response, request, requestLocale)
		return
	}
	id, ok := pathID(parts[0])
	if !ok {
		handler.notFound(response, request, requestLocale)
		return
	}
	switch parts[1] {
	case "revisions":
		handler.createRevision(response, request, requestLocale, id)
	case "approvals":
		handler.approveRevision(response, request, requestLocale, id)
	default:
		handler.notFound(response, request, requestLocale)
	}
}

func (handler *Handler) createRevision(response http.ResponseWriter, request *http.Request, requestLocale locale, id string) {
	input, ok := decodeRevisionForm(response, request)
	if !ok {
		handler.renderEntryFormError(response, request, requestLocale, id, http.StatusBadRequest, messagesFor(requestLocale).InvalidRevisionFormMessage)
		return
	}
	if _, err := handler.repository.CreateRevision(request.Context(), id, input); err != nil {
		handler.renderEntryMutationError(response, request, requestLocale, id, err, mutationRevision)
		return
	}
	http.Redirect(response, request, entryHref(requestLocale, id), http.StatusSeeOther)
}

func (handler *Handler) approveRevision(response http.ResponseWriter, request *http.Request, requestLocale locale, id string) {
	input, ok := decodeApprovalForm(response, request)
	if !ok {
		handler.renderEntryFormError(response, request, requestLocale, id, http.StatusBadRequest, messagesFor(requestLocale).InvalidApprovalFormMessage)
		return
	}
	if _, err := handler.repository.ApproveRevision(request.Context(), id, input); err != nil {
		handler.renderEntryMutationError(response, request, requestLocale, id, err, mutationApproval)
		return
	}
	http.Redirect(response, request, entryHref(requestLocale, id), http.StatusSeeOther)
}

type mutationKind int

const (
	mutationRevision mutationKind = iota
	mutationApproval
)

func (handler *Handler) renderEntryMutationError(response http.ResponseWriter, request *http.Request, requestLocale locale, id string, err error, kind mutationKind) {
	msg := messagesFor(requestLocale)
	status := http.StatusInternalServerError
	message := msg.RevisionFailedMessage
	if kind == mutationApproval {
		message = msg.ApprovalFailedMessage
	}
	switch {
	case errors.Is(err, webapp.ErrNotFound):
		handler.notFound(response, request, requestLocale)
		return
	case errors.Is(err, webapp.ErrConflict):
		status = http.StatusConflict
		message = msg.RevisionConflictMessage
		if kind == mutationApproval {
			message = msg.ApprovalConflictMessage
		}
	case errors.Is(err, webapp.ErrInvalidRequest):
		status = http.StatusBadRequest
		message = msg.InvalidRevisionFormMessage
		if kind == mutationApproval {
			message = msg.InvalidApprovalFormMessage
		}
	case errors.Is(err, webapp.ErrInvalidRevision):
		status = http.StatusUnprocessableEntity
		message = msg.ApprovalInvalidRevisionMessage
	}
	handler.renderEntryFormError(response, request, requestLocale, id, status, message)
}

func (handler *Handler) renderEntryFormError(response http.ResponseWriter, request *http.Request, requestLocale locale, id string, status int, message string) {
	detail, err := handler.repository.GetEntry(request.Context(), id)
	if errors.Is(err, webapp.ErrNotFound) {
		handler.notFound(response, request, requestLocale)
		return
	}
	if err != nil {
		handler.internalError(response, request, requestLocale)
		return
	}
	current := currentCandidate(detail)
	render(response, request, status, entryPage(newEntryPageModel(requestLocale, id, detail, current, message)))
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
		handler.methodNotAllowed(response, request, localeJA, request.URL.Path, "GET, HEAD")
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

func (handler *Handler) methodNotAllowed(response http.ResponseWriter, request *http.Request, requestLocale locale, localPath, allow string) {
	msg := messagesFor(requestLocale)
	response.Header().Set("Allow", allow)
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

func (handler *Handler) invalidSearch(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	msg := messagesFor(requestLocale)
	render(response, request, http.StatusBadRequest, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: http.StatusBadRequest,
		Title: msg.InvalidSearchTitle, Message: msg.InvalidSearchMessage,
	}))
}

func (handler *Handler) invalidUpload(response http.ResponseWriter, request *http.Request, requestLocale locale, status int) {
	msg := messagesFor(requestLocale)
	title := msg.InvalidUploadTitle
	message := msg.InvalidUploadMessage
	switch status {
	case http.StatusRequestEntityTooLarge:
		title = msg.UploadTooLargeTitle
		message = msg.UploadTooLargeMessage
	case http.StatusUnsupportedMediaType:
		title = msg.UnsupportedUploadTitle
		message = msg.UnsupportedUploadMessage
	}
	render(response, request, status, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: status,
		Title: title, Message: message,
	}))
}

func (handler *Handler) invalidTacklerUpload(response http.ResponseWriter, request *http.Request, requestLocale locale, err error) {
	msg := messagesFor(requestLocale)
	diagnostic := tacklerfmt.DetailedDiagnostic(err)
	log.Printf("webui: tackler import rejected: %s", diagnostic)
	render(response, request, http.StatusBadRequest, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: http.StatusBadRequest,
		Title: msg.InvalidTacklerUploadTitle, Message: msg.InvalidTacklerUploadMessage + " " + diagnostic,
	}))
}

func (handler *Handler) uploadFailed(response http.ResponseWriter, request *http.Request, requestLocale locale) {
	msg := messagesFor(requestLocale)
	render(response, request, http.StatusInternalServerError, errorPage(errorPageModel{
		Page: newPageContext(requestLocale, "/"), Status: http.StatusInternalServerError,
		Title: msg.UploadFailedTitle, Message: msg.UploadFailedMessage,
	}))
}

func newIndexPageModel(requestLocale locale, filter webapp.EntryFilter, page webapp.EntryPage, searchApplied bool) indexPageModel {
	model := indexPageModel{
		Page:          newPageContext(requestLocale, "/"),
		Upload:        uploadFormModel{Action: importHref(requestLocale)},
		TacklerUpload: uploadFormModel{Action: tacklerImportHref(requestLocale)},
		Search:        searchFormModel{Action: searchHref(requestLocale), ResetHref: localizedPath(requestLocale, "/"), Filter: filter},
		Export:        exportFormModel{TacklerAction: exportHref(requestLocale, "tackler"), JSONAction: exportHref(requestLocale, "json"), Filter: filter},
		Entries:       make([]entrySummaryModel, 0, len(page.Entries)),
		NextCursor:    page.NextCursor,
		SearchApplied: searchApplied,
	}
	for _, entry := range page.Entries {
		model.Entries = append(model.Entries, entrySummaryModel{
			Href: entryHref(requestLocale, entry.ID), OccurredAt: entry.OccurredAt,
			Description: entry.Description, Status: entry.Status, WorkflowStatus: entry.WorkflowStatus,
			CurrentRevision: entry.CurrentRevision, Source: entry.Source,
		})
	}
	return model
}

func newEntryPageModel(requestLocale locale, id string, detail webapp.EntryDetail, current candidateModel, formError string) entryPageModel {
	current.Approved = detail.CurrentApproval != nil && detail.CurrentApproval.Revision == current.Revision
	return entryPageModel{
		Page:    newPageContext(requestLocale, "/entries/"+url.PathEscape(id)),
		Detail:  detail,
		Current: current,
		RunHref: runHref(requestLocale, detail.RunIdentity),
		RevisionForm: revisionFormModel{
			Action:       entryRevisionHref(requestLocale, id),
			BaseRevision: current.Revision,
			EntryText:    entryText(current),
		},
		ApprovalForm: approvalFormModel{
			Action:     entryApprovalHref(requestLocale, id),
			Revision:   current.Revision,
			CanApprove: current.Valid && !current.Approved,
		},
		FormError: formError,
	}
}

func currentCandidate(detail webapp.EntryDetail) candidateModel {
	current := candidateModel{
		Revision: 0, OccurredAt: detail.OccurredAt, Description: detail.Description,
		Comments: detail.Comments, Postings: detail.Postings, Valid: true,
	}
	if len(detail.Revisions) > 0 {
		latest := detail.Revisions[len(detail.Revisions)-1]
		current = candidateModel{
			Revision: latest.Revision, OccurredAt: latest.OccurredAt, Description: latest.Description,
			Comments: latest.Comments, Postings: latest.Postings, Valid: latest.Valid,
		}
	}
	return current
}

func decodeImportUpload(response http.ResponseWriter, request *http.Request) ([]byte, int, bool) {
	if request.URL.RawQuery != "" {
		return nil, http.StatusBadRequest, false
	}
	mediaType, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		return nil, http.StatusUnsupportedMediaType, false
	}
	boundary := params["boundary"]
	if boundary == "" {
		return nil, http.StatusBadRequest, false
	}

	reader := multipart.NewReader(http.MaxBytesReader(response, request.Body, maxImportRequestSize), boundary)
	var body []byte
	fileSeen := false
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if requestTooLarge(err) {
				return nil, http.StatusRequestEntityTooLarge, false
			}
			return nil, http.StatusBadRequest, false
		}
		data, status, ok := readImportPart(part, fileSeen)
		_ = part.Close()
		if !ok {
			return nil, status, false
		}
		body = data
		fileSeen = true
	}
	if !fileSeen {
		return nil, http.StatusBadRequest, false
	}
	return body, http.StatusOK, true
}

func readImportPart(part *multipart.Part, fileSeen bool) ([]byte, int, bool) {
	if fileSeen || part.FormName() != importFileField || part.FileName() == "" {
		return nil, http.StatusBadRequest, false
	}
	body, err := io.ReadAll(io.LimitReader(part, maxImportFileSize+1))
	if err != nil {
		if requestTooLarge(err) {
			return nil, http.StatusRequestEntityTooLarge, false
		}
		return nil, http.StatusBadRequest, false
	}
	if len(body) > maxImportFileSize {
		return nil, http.StatusRequestEntityTooLarge, false
	}
	return body, http.StatusOK, true
}

func requestTooLarge(err error) bool {
	var maxBytesError *http.MaxBytesError
	return errors.As(err, &maxBytesError)
}

func render(response http.ResponseWriter, request *http.Request, status int, component templ.Component) {
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	response.WriteHeader(status)
	if request.Method == http.MethodHead {
		return
	}
	_ = component.Render(request.Context(), response)
}

func decodeSearchForm(response http.ResponseWriter, request *http.Request) (webapp.EntryFilter, string, bool) {
	if request.URL.RawQuery != "" {
		return webapp.EntryFilter{}, "", false
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		return webapp.EntryFilter{}, "", false
	}
	body, err := io.ReadAll(http.MaxBytesReader(response, request.Body, maxSearchFormBody))
	if err != nil {
		return webapp.EntryFilter{}, "", false
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return webapp.EntryFilter{}, "", false
	}
	for key, values := range form {
		if !searchFormFieldAllowed(key) || len(values) > 1 {
			return webapp.EntryFilter{}, "", false
		}
	}
	return webapp.EntryFilter{
		DateFrom:        form.Get(searchFieldDateFrom),
		DateTo:          form.Get(searchFieldDateTo),
		Account:         form.Get(searchFieldAccount),
		Description:     form.Get(searchFieldDescription),
		Status:          form.Get(searchFieldStatus),
		WorkflowStatus:  form.Get(searchFieldWorkflowStatus),
		SourceNamespace: form.Get(searchFieldSourceNamespace),
		SourceDisplay:   form.Get(searchFieldSourceDisplay),
	}, form.Get(searchFieldCursor), true
}

func decodeExportForm(response http.ResponseWriter, request *http.Request) (webapp.EntryFilter, bool) {
	filter, cursor, ok := decodeSearchForm(response, request)
	return filter, ok && cursor == ""
}

func decodeRevisionForm(response http.ResponseWriter, request *http.Request) (webapp.RevisionRequest, bool) {
	form, ok := decodeURLEncodedForm(response, request, maxRevisionFormBody)
	if !ok {
		return webapp.RevisionRequest{}, false
	}
	if !revisionFormFieldsAllowed(form) {
		return webapp.RevisionRequest{}, false
	}
	baseRevision, err := strconv.Atoi(form.Get(revisionFieldBaseRevision))
	if err != nil || baseRevision < 0 {
		return webapp.RevisionRequest{}, false
	}
	input, ok := parseEntryText(form.Get(revisionFieldEntry))
	if !ok {
		return webapp.RevisionRequest{}, false
	}
	input.BaseRevision = &baseRevision
	return input, true
}

func decodeApprovalForm(response http.ResponseWriter, request *http.Request) (webapp.ApprovalRequest, bool) {
	form, ok := decodeURLEncodedForm(response, request, maxRevisionFormBody)
	if !ok {
		return webapp.ApprovalRequest{}, false
	}
	for key, values := range form {
		if key != approvalFieldRevision || len(values) > 1 {
			return webapp.ApprovalRequest{}, false
		}
	}
	revision, err := strconv.Atoi(form.Get(approvalFieldRevision))
	if err != nil || revision < 0 {
		return webapp.ApprovalRequest{}, false
	}
	return webapp.ApprovalRequest{Revision: &revision}, true
}

func decodeURLEncodedForm(response http.ResponseWriter, request *http.Request, limit int64) (url.Values, bool) {
	if request.URL.RawQuery != "" {
		return nil, false
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		return nil, false
	}
	body, err := io.ReadAll(http.MaxBytesReader(response, request.Body, limit))
	if err != nil {
		return nil, false
	}
	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, false
	}
	for _, values := range form {
		if len(values) > 1 {
			return nil, false
		}
	}
	return form, true
}

func revisionFormFieldsAllowed(form url.Values) bool {
	for key := range form {
		switch key {
		case revisionFieldBaseRevision, revisionFieldEntry:
			continue
		default:
			return false
		}
	}
	return true
}

func entryText(entry candidateModel) string {
	lines := []string{entry.OccurredAt + "  '" + entry.Description}
	for _, comment := range entry.Comments {
		lines = append(lines, "    ; "+comment)
	}
	for _, posting := range entry.Postings {
		var builder strings.Builder
		builder.WriteString(posting.Account)
		if posting.Amount != nil {
			builder.WriteByte(' ')
			builder.WriteString(*posting.Amount)
			builder.WriteByte(' ')
			builder.WriteString(posting.Commodity)
		}
		if posting.Comment != "" {
			builder.WriteString(" ; ")
			builder.WriteString(posting.Comment)
		}
		lines = append(lines, "    "+builder.String())
	}
	return strings.Join(lines, "\n")
}

func parseEntryText(text string) (webapp.RevisionRequest, bool) {
	entries, err := tacklerfmt.ParseUnvalidated([]byte(text))
	if err != nil || len(entries) != 1 {
		return webapp.RevisionRequest{}, false
	}
	entry := entries[0]
	input := webapp.RevisionRequest{
		OccurredAt:  entry.Date.String(),
		Description: entry.Description,
		Comments:    append([]string(nil), entry.Comments...),
		Postings:    make([]webapp.PostingDetail, 0, len(entry.Postings)),
	}
	for _, posting := range entry.Postings {
		detail := webapp.PostingDetail{Account: posting.Account, Comment: posting.Comment}
		if posting.Amount != nil {
			amount := posting.Amount.Value.String()
			detail.Amount = &amount
			detail.Commodity = string(posting.Amount.Commodity)
		}
		input.Postings = append(input.Postings, detail)
	}
	return input, true
}

func normalizedInputForTacklerEntries(entries []ledger.JournalEntry) ([]byte, error) {
	batch := normalizedBatch{SchemaVersion: 1, Records: make([]normalizedRecord, 0, len(entries))}
	for index, entry := range entries {
		record := normalizedRecord{
			Source: normalizedSource{
				Namespace:  "tackler",
				Display:    "uploaded.txn",
				ExternalID: tacklerExternalID(entry, index),
			},
			OccurredAt:  entry.Date.String(),
			Description: entry.Description,
			Comments:    append([]string(nil), entry.Comments...),
			Postings:    make([]normalizedPosting, 0, len(entry.Postings)),
		}
		for _, posting := range entry.Postings {
			detail := normalizedPosting{Account: posting.Account, Comment: posting.Comment}
			if posting.Amount != nil {
				detail.Amount = posting.Amount.Value.String()
				detail.Commodity = string(posting.Amount.Commodity)
			}
			record.Postings = append(record.Postings, detail)
		}
		batch.Records = append(batch.Records, record)
	}
	return json.Marshal(batch)
}

func tacklerExternalID(entry ledger.JournalEntry, index int) string {
	output, err := tacklerfmt.Export([]ledger.JournalEntry{entry}, tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted})
	if err != nil {
		output = []byte(entry.Date.String() + "\n" + entry.Description)
	}
	sum := sha256.Sum256(append([]byte("bokiccio.tackler-import.v1\x00"), output...))
	return fmt.Sprintf("entry-%06d-%x", index+1, sum[:])
}

const (
	revisionFieldBaseRevision = "base_revision"
	revisionFieldEntry        = "entry"
	approvalFieldRevision     = "revision"
)

func formInt(value int) string {
	return strconv.Itoa(value)
}

const (
	searchFieldDateFrom        = "date_from"
	searchFieldDateTo          = "date_to"
	searchFieldAccount         = "account"
	searchFieldDescription     = "description"
	searchFieldStatus          = "status"
	searchFieldWorkflowStatus  = "workflow_status"
	searchFieldSourceNamespace = "source_namespace"
	searchFieldSourceDisplay   = "source_display"
	searchFieldCursor          = "cursor"
)

var searchFormFields = map[string]struct{}{
	searchFieldDateFrom:        {},
	searchFieldDateTo:          {},
	searchFieldAccount:         {},
	searchFieldDescription:     {},
	searchFieldStatus:          {},
	searchFieldWorkflowStatus:  {},
	searchFieldSourceNamespace: {},
	searchFieldSourceDisplay:   {},
	searchFieldCursor:          {},
}

type normalizedBatch struct {
	SchemaVersion int                `json:"schema_version"`
	Records       []normalizedRecord `json:"records"`
}

type normalizedRecord struct {
	Source      normalizedSource    `json:"source"`
	OccurredAt  string              `json:"occurred_at"`
	Description string              `json:"description"`
	Comments    []string            `json:"comments,omitempty"`
	Postings    []normalizedPosting `json:"postings"`
}

type normalizedSource struct {
	Namespace  string `json:"namespace"`
	Display    string `json:"display"`
	ExternalID string `json:"external_id"`
}

type normalizedPosting struct {
	Account   string `json:"account"`
	Amount    string `json:"amount,omitempty"`
	Commodity string `json:"commodity,omitempty"`
	Comment   string `json:"comment,omitempty"`
}

func searchFormFieldAllowed(key string) bool {
	_, ok := searchFormFields[key]
	return ok
}

func isHXRequest(request *http.Request) bool {
	return request.Header.Get("HX-Request") == "true"
}

func addVary(response http.ResponseWriter, value string) {
	current := response.Header().Get("Vary")
	if current == "" {
		response.Header().Set("Vary", value)
		return
	}
	for _, item := range strings.Split(current, ",") {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return
		}
	}
	response.Header().Set("Vary", current+", "+value)
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
	response.Header().Set("Referrer-Policy", "same-origin")
	response.Header().Set("X-Content-Type-Options", "nosniff")
}
