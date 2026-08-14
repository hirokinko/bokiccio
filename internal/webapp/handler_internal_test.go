package webapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hirokinko/bokiccio/internal/reporting"
)

func TestWriteRepositoryErrorKeepsReportFailuresPrivate(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "closing unbalanced", err: reporting.ErrClosingUnbalanced, wantStatus: http.StatusUnprocessableEntity, wantCode: "closing_balance_unbalanced"},
		{name: "internal", err: errors.New("query closing balance sheet: private SQL detail"), wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			(&Handler{}).writeRepositoryError(response, test.err)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			body := response.Body.Bytes()
			var problem struct {
				Code string `json:"code"`
			}
			if err := json.NewDecoder(bytes.NewReader(body)).Decode(&problem); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if problem.Code != test.wantCode {
				t.Fatalf("code = %q, want %q", problem.Code, test.wantCode)
			}
			if strings.Contains(string(body), "private SQL detail") {
				t.Fatalf("response exposed internal error: %s", body)
			}
		})
	}
}
