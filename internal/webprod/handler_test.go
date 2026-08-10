package webprod

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hirokinko/bokiccio/internal/webapp"
)

type acceptingValidator struct{}

func (acceptingValidator) Validate(_ context.Context, _, _ string) (webapp.IAPClaims, error) {
	return webapp.IAPClaims{
		Issuer:   "https://cloud.google.com/iap",
		Subject:  "owner",
		Email:    "owner@example.com",
		IssuedAt: time.Now().Add(-time.Minute),
		Expires:  time.Now().Add(5 * time.Minute),
	}, nil
}

func TestProductionHandlerOnlyExemptsHealth(t *testing.T) {
	application := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})
	security := webapp.IAPSecurity{Audience: "audience", OwnerEmail: "owner@example.com", ExternalOrigin: "https://example.com"}
	handler, err := NewProductionHandler(application, acceptingValidator{}, security)
	if err != nil {
		t.Fatalf("NewProductionHandler() error = %v", err)
	}

	liveness := httptest.NewRecorder()
	handler.ServeHTTP(liveness, httptest.NewRequest(http.MethodGet, "/livez", nil))
	if liveness.Code != http.StatusOK || liveness.Body.String() != "ok\n" {
		t.Fatalf("liveness status=%d body=%q", liveness.Code, liveness.Body.String())
	}
	if got := liveness.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("liveness Cache-Control = %q, want no-store", got)
	}

	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/livez", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Fatalf("liveness HEAD status=%d body=%q", head.Code, head.Body.String())
	}

	methodNotAllowed := httptest.NewRecorder()
	handler.ServeHTTP(methodNotAllowed, httptest.NewRequest(http.MethodPost, "/livez", nil))
	if methodNotAllowed.Code != http.StatusMethodNotAllowed {
		t.Fatalf("liveness POST status=%d body=%q", methodNotAllowed.Code, methodNotAllowed.Body.String())
	}
	if got := methodNotAllowed.Header().Get("Allow"); got != "GET, HEAD" {
		t.Fatalf("liveness Allow = %q, want GET, HEAD", got)
	}

	legacyHealth := httptest.NewRecorder()
	handler.ServeHTTP(legacyHealth, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if legacyHealth.Code != http.StatusUnauthorized {
		t.Fatalf("legacy health status=%d body=%q", legacyHealth.Code, legacyHealth.Body.String())
	}

	private := httptest.NewRecorder()
	handler.ServeHTTP(private, httptest.NewRequest(http.MethodGet, "/api/v1/entries", nil))
	if private.Code != http.StatusUnauthorized {
		t.Fatalf("private status=%d body=%q", private.Code, private.Body.String())
	}

	authenticated := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/entries", nil)
	request.Header.Set("X-Goog-IAP-JWT-Assertion", "signed")
	handler.ServeHTTP(authenticated, request)
	if authenticated.Code != http.StatusNoContent {
		t.Fatalf("authenticated status=%d body=%q", authenticated.Code, authenticated.Body.String())
	}
}
