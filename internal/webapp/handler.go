package webapp

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hirokinko/bokiccio/internal/ingest"
	"github.com/hirokinko/bokiccio/internal/ledger"
	"github.com/hirokinko/bokiccio/internal/reporting"
	"github.com/hirokinko/bokiccio/internal/tacklerfmt"
)

const (
	maxImportBody    = 10 << 20
	maxRevisionBody  = 1 << 20
	maxReportingBody = 1 << 20
)

type Handler struct {
	repository Repository
}

func NewHandler(repository Repository) *Handler {
	return &Handler{repository: repository}
}

func (handler *Handler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	switch {
	case request.URL.Path == "/api/v1/imports":
		if request.Method != http.MethodPost {
			handler.methodNotAllowed(response)
			return
		}
		handler.importRecords(response, request)
	case strings.HasPrefix(request.URL.Path, "/api/v1/imports/"):
		if request.Method != http.MethodGet {
			handler.methodNotAllowed(response)
			return
		}
		handler.getRun(response, request, strings.TrimPrefix(request.URL.Path, "/api/v1/imports/"))
	case request.URL.Path == "/api/v1/entries":
		if request.Method != http.MethodGet {
			handler.methodNotAllowed(response)
			return
		}
		handler.listEntries(response, request)
	case request.URL.Path == "/api/v1/exports/tackler":
		if request.Method != http.MethodGet {
			handler.methodNotAllowed(response)
			return
		}
		handler.exportTackler(response, request)
	case request.URL.Path == "/api/v1/exports/json":
		if request.Method != http.MethodGet {
			handler.methodNotAllowed(response)
			return
		}
		handler.exportJSON(response, request)
	case request.URL.Path == "/api/v1/reporting/configuration":
		handler.reportingConfiguration(response, request)
	case strings.HasPrefix(request.URL.Path, "/api/v1/reporting/configurations/"):
		if request.Method != http.MethodGet {
			handler.methodNotAllowed(response)
			return
		}
		handler.getReportingConfiguration(response, request,
			strings.TrimPrefix(request.URL.Path, "/api/v1/reporting/configurations/"))
	case request.URL.Path == "/api/v1/reports/trial-balance":
		if request.Method != http.MethodGet {
			handler.methodNotAllowed(response)
			return
		}
		handler.getTrialBalance(response, request)
	case request.URL.Path == "/api/v1/reports/current-overview":
		if request.Method != http.MethodGet {
			handler.methodNotAllowed(response)
			return
		}
		handler.getCurrentOverview(response, request)
	case request.URL.Path == "/api/v1/reports/balance-sheet":
		if request.Method != http.MethodGet {
			handler.methodNotAllowed(response)
			return
		}
		handler.getBalanceSheet(response, request)
	case request.URL.Path == "/api/v1/reports/closing-balance-sheet":
		if request.Method != http.MethodGet {
			handler.methodNotAllowed(response)
			return
		}
		handler.getClosingBalanceSheet(response, request)
	case request.URL.Path == "/api/v1/reports/income-statement":
		if request.Method != http.MethodGet {
			handler.methodNotAllowed(response)
			return
		}
		handler.getIncomeStatement(response, request)
	case request.URL.Path == "/api/v1/reports/balance-trend":
		if request.Method != http.MethodGet {
			handler.methodNotAllowed(response)
			return
		}
		handler.getBalanceTrend(response, request)
	case strings.HasPrefix(request.URL.Path, "/api/v1/entries/"):
		handler.entryResource(response, request, strings.TrimPrefix(request.URL.Path, "/api/v1/entries/"))
	default:
		writeError(response, http.StatusNotFound, "not_found", "resource not found")
	}
}

func (handler *Handler) reportingConfiguration(response http.ResponseWriter, request *http.Request) {
	switch request.Method {
	case http.MethodGet:
		detail, err := handler.repository.GetCurrentReportingConfiguration(request.Context())
		if err != nil {
			handler.writeRepositoryError(response, err)
			return
		}
		writeJSON(response, http.StatusOK, detail)
	case http.MethodPost:
		actorEmail, ok := handler.requireWriteAccess(response, request)
		if !ok {
			return
		}
		var input ReportingConfigurationRequest
		if !decodeJSONRequest(response, request, maxReportingBody, &input) {
			return
		}
		detail, err := handler.repository.CreateReportingConfiguration(request.Context(), actorEmail, input)
		if err != nil {
			handler.writeRepositoryError(response, err)
			return
		}
		writeJSON(response, http.StatusCreated, detail)
	default:
		handler.methodNotAllowed(response)
	}
}

func (handler *Handler) getReportingConfiguration(response http.ResponseWriter, request *http.Request, text string) {
	revision, err := strconv.Atoi(text)
	if err != nil || revision < 1 || strconv.Itoa(revision) != text {
		writeError(response, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	detail, err := handler.repository.GetReportingConfiguration(request.Context(), revision)
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, detail)
}

func (handler *Handler) getTrialBalance(response http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	detail, err := handler.repository.GetTrialBalance(request.Context(), reporting.Period{
		StartDate: query.Get("start_date"), EndDate: query.Get("end_date"),
	})
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, detail)
}

func (handler *Handler) getCurrentOverview(response http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	if len(query) != 3 || len(query["as_of"]) != 1 || len(query["expense_start_date"]) != 1 ||
		len(query["expense_end_date"]) != 1 || query.Get("as_of") == "" || query.Get("expense_start_date") == "" ||
		query.Get("expense_end_date") == "" {
		writeError(response, http.StatusBadRequest, "invalid_period", "reporting period is invalid")
		return
	}
	detail, err := handler.repository.GetCurrentOverview(request.Context(), query.Get("as_of"), reporting.Period{
		StartDate: query.Get("expense_start_date"), EndDate: query.Get("expense_end_date"),
	})
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, detail)
}

func (handler *Handler) getBalanceSheet(response http.ResponseWriter, request *http.Request) {
	period, ok := strictReportPeriod(request)
	if !ok {
		writeError(response, http.StatusBadRequest, "invalid_period", "reporting period is invalid")
		return
	}
	detail, err := handler.repository.GetBalanceSheet(request.Context(), period)
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, detail)
}

func (handler *Handler) getClosingBalanceSheet(response http.ResponseWriter, request *http.Request) {
	period, ok := strictReportPeriod(request)
	if !ok {
		writeError(response, http.StatusBadRequest, "invalid_period", "reporting period is invalid")
		return
	}
	detail, err := handler.repository.GetClosingBalanceSheet(request.Context(), period)
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, detail)
}

func (handler *Handler) getIncomeStatement(response http.ResponseWriter, request *http.Request) {
	period, ok := strictReportPeriod(request)
	if !ok {
		writeError(response, http.StatusBadRequest, "invalid_period", "reporting period is invalid")
		return
	}
	detail, err := handler.repository.GetIncomeStatement(request.Context(), period)
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, detail)
}

func (handler *Handler) getBalanceTrend(response http.ResponseWriter, request *http.Request) {
	period, ok := strictReportPeriod(request)
	if !ok {
		writeError(response, http.StatusBadRequest, "invalid_period", "reporting period is invalid")
		return
	}
	detail, err := handler.repository.GetBalanceTrend(request.Context(), period)
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, detail)
}

func strictReportPeriod(request *http.Request) (reporting.Period, bool) {
	query := request.URL.Query()
	if len(query) != 2 || len(query["start_date"]) != 1 || len(query["end_date"]) != 1 ||
		query.Get("start_date") == "" || query.Get("end_date") == "" {
		return reporting.Period{}, false
	}
	return reporting.Period{StartDate: query.Get("start_date"), EndDate: query.Get("end_date")}, true
}

func (handler *Handler) entryResource(response http.ResponseWriter, request *http.Request, path string) {
	parts := strings.Split(path, "/")
	if len(parts) < 1 || len(parts) > 2 {
		writeError(response, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	id, err := pathIdentifier(parts[0])
	if err != nil {
		writeError(response, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	if len(parts) == 1 {
		if request.Method != http.MethodGet {
			handler.methodNotAllowed(response)
			return
		}
		handler.getEntry(response, request, id)
		return
	}
	if request.Method != http.MethodPost {
		handler.methodNotAllowed(response)
		return
	}
	switch parts[1] {
	case "revisions":
		handler.createRevision(response, request, id)
	case "approvals":
		handler.approveRevision(response, request, id)
	default:
		writeError(response, http.StatusNotFound, "not_found", "resource not found")
	}
}

func (handler *Handler) importRecords(response http.ResponseWriter, request *http.Request) {
	actorEmail, access, err := handler.userAccess(request)
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	if !access.FileUploadEnabled {
		handler.writeRepositoryError(response, ErrUploadDisabled)
		return
	}
	if !access.CanWrite {
		handler.writeRepositoryError(response, ErrUploadForbidden)
		return
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(response, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, maxImportBody+1))
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid_body", "request body could not be read")
		return
	}
	if len(body) > maxImportBody {
		writeError(response, http.StatusRequestEntityTooLarge, "body_too_large", "request body exceeds 10 MiB")
		return
	}
	result, err := handler.repository.Import(request.Context(), actorEmail, body)
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	result.DetailURL = "/api/v1/imports/" + url.PathEscape(result.RunIdentity)
	response.Header().Set("Location", result.DetailURL)
	writeJSON(response, http.StatusCreated, struct {
		SchemaVersion int `json:"schema_version"`
		ImportResult
	}{SchemaVersion: APISchemaVersion, ImportResult: result})
}

func (handler *Handler) getRun(response http.ResponseWriter, request *http.Request, escapedID string) {
	id, err := pathIdentifier(escapedID)
	if err != nil {
		writeError(response, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	detail, err := handler.repository.GetRun(request.Context(), id)
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, detail)
}

func (handler *Handler) listEntries(response http.ResponseWriter, request *http.Request) {
	limit := 50
	if text := request.URL.Query().Get("limit"); text != "" {
		value, err := strconv.Atoi(text)
		if err != nil || value < 1 || value > 100 {
			writeError(response, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
			return
		}
		limit = value
	}
	page, err := handler.repository.ListEntries(request.Context(), EntryQuery{
		Filter: entryFilter(request), Limit: limit, Cursor: request.URL.Query().Get("cursor"),
	})
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, page)
}

func entryFilter(request *http.Request) EntryFilter {
	query := request.URL.Query()
	return EntryFilter{
		DateFrom: query.Get("date_from"), DateTo: query.Get("date_to"), Account: query.Get("account"),
		Description: query.Get("description"), Status: query.Get("status"),
		WorkflowStatus: query.Get("workflow_status"), SourceNamespace: query.Get("source_namespace"),
		SourceDisplay: query.Get("source_display"),
	}
}

func (handler *Handler) exportTackler(response http.ResponseWriter, request *http.Request) {
	approved, err := handler.repository.ListApprovedEntries(request.Context(), entryFilter(request))
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	entries := make([]ledger.JournalEntry, 0, len(approved))
	for _, item := range approved {
		entries = append(entries, item.Entry)
	}
	output, err := tacklerfmt.Export(entries, tacklerfmt.Options{OmittedAmounts: tacklerfmt.PreserveOmitted})
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	response.Header().Set("Content-Disposition", `attachment; filename="bokiccio-export.txn"`)
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write(output)
}

func (handler *Handler) exportJSON(response http.ResponseWriter, request *http.Request) {
	approved, err := handler.repository.ListApprovedEntries(request.Context(), entryFilter(request))
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	exported := JSONExport{SchemaVersion: APISchemaVersion, Entries: []ExportEntry{}}
	for _, item := range approved {
		entry := ExportEntry{
			ID: item.ID, Revision: item.Revision, ApprovedAt: item.ApprovedAt, Source: item.Source,
			OccurredAt: item.Entry.Date.String(), Description: item.Entry.Description,
			Comments: append([]string(nil), item.Entry.Comments...), Postings: []PostingDetail{},
		}
		for _, posting := range item.Entry.Postings {
			detail := PostingDetail{Account: posting.Account, Comment: posting.Comment}
			if posting.Amount != nil {
				amount := posting.Amount.Value.String()
				detail.Amount = &amount
				detail.Commodity = string(posting.Amount.Commodity)
			}
			if posting.TotalPrice != nil {
				detail.TotalPrice = &AmountDetail{
					Amount: posting.TotalPrice.Value.String(), Commodity: string(posting.TotalPrice.Commodity),
				}
			}
			entry.Postings = append(entry.Postings, detail)
		}
		exported.Entries = append(exported.Entries, entry)
	}
	writeJSON(response, http.StatusOK, exported)
}

func (handler *Handler) getEntry(response http.ResponseWriter, request *http.Request, id string) {
	detail, err := handler.repository.GetEntry(request.Context(), id)
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, detail)
}

func (handler *Handler) createRevision(response http.ResponseWriter, request *http.Request, id string) {
	actorEmail, ok := handler.requireWriteAccess(response, request)
	if !ok {
		return
	}
	var input RevisionRequest
	if !decodeJSONRequest(response, request, maxRevisionBody, &input) {
		return
	}
	revision, err := handler.repository.CreateRevision(request.Context(), actorEmail, id, input)
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, struct {
		SchemaVersion int `json:"schema_version"`
		RevisionDetail
	}{SchemaVersion: APISchemaVersion, RevisionDetail: revision})
}

func (handler *Handler) approveRevision(response http.ResponseWriter, request *http.Request, id string) {
	actorEmail, ok := handler.requireWriteAccess(response, request)
	if !ok {
		return
	}
	var input ApprovalRequest
	if !decodeJSONRequest(response, request, maxRevisionBody, &input) {
		return
	}
	approval, err := handler.repository.ApproveRevision(request.Context(), actorEmail, id, input)
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, struct {
		SchemaVersion int `json:"schema_version"`
		ApprovalDetail
	}{SchemaVersion: APISchemaVersion, ApprovalDetail: approval})
}

func decodeJSONRequest(response http.ResponseWriter, request *http.Request, limit int64, destination any) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(response, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return false
	}
	body, err := io.ReadAll(io.LimitReader(request.Body, limit+1))
	if err != nil {
		writeError(response, http.StatusBadRequest, "invalid_body", "request body could not be read")
		return false
	}
	if int64(len(body)) > limit {
		writeError(response, http.StatusRequestEntityTooLarge, "body_too_large", "request body is too large")
		return false
	}
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(response, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		writeError(response, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return false
	}
	return true
}

func (handler *Handler) writeRepositoryError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUploadDisabled):
		writeError(response, http.StatusForbidden, "upload_disabled", "file upload is disabled")
	case errors.Is(err, ErrUploadForbidden):
		writeError(response, http.StatusForbidden, "upload_forbidden", "file upload is not permitted for this user")
	case errors.Is(err, ErrWriteForbidden):
		writeError(response, http.StatusForbidden, "write_forbidden", "data changes are not permitted for this user")
	case errors.Is(err, ingest.ErrInvalidInput):
		writeError(response, http.StatusBadRequest, "invalid_import", "normalized import is invalid")
	case errors.Is(err, ErrNotFound):
		writeError(response, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, ErrInvalidRequest):
		writeError(response, http.StatusBadRequest, "invalid_request", "request parameters are invalid")
	case errors.Is(err, ErrInvalidRevision):
		writeError(response, http.StatusUnprocessableEntity, "invalid_revision", "revision failed validation and cannot be approved")
	case errors.Is(err, ErrConflict):
		writeError(response, http.StatusConflict, "conflict", "resource state changed; retry the request")
	case errors.Is(err, ErrReportingNotConfigured):
		writeError(response, http.StatusConflict, "reporting_not_configured", "financial reporting is not configured")
	case errors.Is(err, reporting.ErrInvalidPeriod):
		writeError(response, http.StatusBadRequest, "invalid_period", "reporting period is invalid")
	case errors.Is(err, reporting.ErrAmountOverflow):
		writeError(response, http.StatusUnprocessableEntity, "report_amount_overflow", "report amount exceeds the supported range")
	case errors.Is(err, reporting.ErrOpeningUnbalanced):
		writeError(response, http.StatusUnprocessableEntity, "opening_balance_unbalanced", "reporting opening balance is unbalanced")
	case errors.Is(err, reporting.ErrClosingUnbalanced):
		writeError(response, http.StatusUnprocessableEntity, "closing_balance_unbalanced", "reporting closing balance is unbalanced")
	default:
		writeError(response, http.StatusInternalServerError, "internal_error", "request could not be completed")
	}
}

func (handler *Handler) userAccess(request *http.Request) (string, UserAccess, error) {
	actorEmail := ""
	if claims, ok := IAPClaimsFromContext(request.Context()); ok {
		actorEmail = claims.Email
	}
	access, err := handler.repository.GetUserAccess(request.Context(), actorEmail)
	return actorEmail, access, err
}

func (handler *Handler) requireWriteAccess(response http.ResponseWriter, request *http.Request) (string, bool) {
	actorEmail, access, err := handler.userAccess(request)
	if err != nil {
		handler.writeRepositoryError(response, err)
		return "", false
	}
	if !access.CanWrite {
		handler.writeRepositoryError(response, ErrWriteForbidden)
		return "", false
	}
	return actorEmail, true
}

func (handler *Handler) methodNotAllowed(response http.ResponseWriter) {
	writeError(response, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}

func pathIdentifier(escaped string) (string, error) {
	if escaped == "" || strings.Contains(escaped, "/") {
		return "", errors.New("invalid path identifier")
	}
	value, err := url.PathUnescape(escaped)
	if err != nil || value == "" || strings.Contains(value, "/") {
		return "", errors.New("invalid path identifier")
	}
	return value, nil
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func writeError(response http.ResponseWriter, status int, code, message string) {
	writeJSON(response, status, struct {
		SchemaVersion int    `json:"schema_version"`
		Code          string `json:"code"`
		Message       string `json:"message"`
	}{SchemaVersion: APISchemaVersion, Code: code, Message: message})
}
