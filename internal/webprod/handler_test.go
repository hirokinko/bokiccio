package webprod

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hirokinko/bokiccio/internal/webapp"
)

func TestApplicationHandlerKeepsAPIAndUIRepresentationsSeparate(t *testing.T) {
	api := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"kind":"api"}`))
	})
	ui := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/html")
		_, _ = response.Write([]byte(`<p>ui</p>`))
	})
	handler, err := NewApplicationHandler(api, ui)
	if err != nil {
		t.Fatalf("NewApplicationHandler() error = %v", err)
	}
	for _, test := range []struct {
		path, contentType, body string
	}{
		{path: "/", contentType: "text/html", body: `<p>ui</p>`},
		{path: "/entries/id", contentType: "text/html", body: `<p>ui</p>`},
		{path: "/api", contentType: "application/json", body: `{"kind":"api"}`},
		{path: "/api/v1/entries", contentType: "application/json", body: `{"kind":"api"}`},
		{path: "/api/unknown", contentType: "application/json", body: `{"kind":"api"}`},
	} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, bytes.NewReader(nil)))
		if response.Header().Get("Content-Type") != test.contentType || response.Body.String() != test.body {
			t.Errorf("%s content-type=%q body=%q", test.path, response.Header().Get("Content-Type"), response.Body.String())
		}
	}
	if _, err := NewApplicationHandler(nil, ui); err == nil {
		t.Fatal("NewApplicationHandler(nil, ui) error = nil")
	}
}

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

	for _, path := range []string{"/", "/entries/id", "/assets/app.css", "/api/v1/entries"} {
		private := httptest.NewRecorder()
		handler.ServeHTTP(private, httptest.NewRequest(http.MethodGet, path, nil))
		if private.Code != http.StatusUnauthorized {
			t.Errorf("unauthenticated %s status=%d body=%q", path, private.Code, private.Body.String())
		}
	}

	for _, path := range []string{"/", "/api/v1/entries"} {
		authenticated := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("X-Goog-IAP-JWT-Assertion", "signed")
		handler.ServeHTTP(authenticated, request)
		if authenticated.Code != http.StatusNoContent {
			t.Errorf("authenticated %s status=%d body=%q", path, authenticated.Code, authenticated.Body.String())
		}
	}
}
