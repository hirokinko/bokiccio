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
)

const (
	maxImportBody   = 10 << 20
	maxRevisionBody = 1 << 20
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
	case strings.HasPrefix(request.URL.Path, "/api/v1/entries/"):
		handler.entryResource(response, request, strings.TrimPrefix(request.URL.Path, "/api/v1/entries/"))
	default:
		writeError(response, http.StatusNotFound, "not_found", "resource not found")
	}
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
	result, err := handler.repository.Import(request.Context(), body)
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
	page, err := handler.repository.ListEntries(request.Context(), limit, request.URL.Query().Get("cursor"))
	if err != nil {
		handler.writeRepositoryError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, page)
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
	var input RevisionRequest
	if !decodeJSONRequest(response, request, maxRevisionBody, &input) {
		return
	}
	revision, err := handler.repository.CreateRevision(request.Context(), id, input)
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
	var input ApprovalRequest
	if !decodeJSONRequest(response, request, maxRevisionBody, &input) {
		return
	}
	approval, err := handler.repository.ApproveRevision(request.Context(), id, input)
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
	default:
		writeError(response, http.StatusInternalServerError, "internal_error", "request could not be completed")
	}
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
