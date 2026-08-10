package webapp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type stubIAPValidator struct {
	claims IAPClaims
	err    error
	token  string
	aud    string
}

func (validator *stubIAPValidator) Validate(_ context.Context, token, audience string) (IAPClaims, error) {
	validator.token = token
	validator.aud = audience
	return validator.claims, validator.err
}

func TestRequireIAP(t *testing.T) {
	security := IAPSecurity{
		Audience:       "/projects/123/locations/asia-northeast1/services/bokiccio",
		OwnerEmail:     "owner@example.com",
		ExternalOrigin: "https://bokiccio.example.com",
	}
	validClaims := IAPClaims{
		Issuer:   iapIssuer,
		Subject:  "owner-subject",
		Email:    "OWNER@example.com",
		IssuedAt: time.Now().Add(-time.Minute),
		Expires:  time.Now().Add(5 * time.Minute),
	}
	tests := []struct {
		name   string
		method string
		token  string
		origin string
		claims IAPClaims
		err    error
		status int
	}{
		{name: "authenticated read", method: http.MethodGet, token: "signed", claims: validClaims, status: http.StatusNoContent},
		{name: "authenticated mutation", method: http.MethodPost, token: "signed", origin: security.ExternalOrigin, claims: validClaims, status: http.StatusNoContent},
		{name: "missing token", method: http.MethodGet, claims: validClaims, status: http.StatusUnauthorized},
		{name: "invalid signature", method: http.MethodGet, token: "invalid", claims: validClaims, err: errors.New("invalid"), status: http.StatusUnauthorized},
		{name: "wrong issuer", method: http.MethodGet, token: "signed", claims: replaceClaims(validClaims, func(value *IAPClaims) { value.Issuer = "https://accounts.google.com" }), status: http.StatusUnauthorized},
		{name: "wrong owner", method: http.MethodGet, token: "signed", claims: replaceClaims(validClaims, func(value *IAPClaims) { value.Email = "other@example.com" }), status: http.StatusUnauthorized},
		{name: "future issued at", method: http.MethodGet, token: "signed", claims: replaceClaims(validClaims, func(value *IAPClaims) { value.IssuedAt = time.Now().Add(time.Minute) }), status: http.StatusUnauthorized},
		{name: "excessive lifetime", method: http.MethodGet, token: "signed", claims: replaceClaims(validClaims, func(value *IAPClaims) { value.Expires = value.IssuedAt.Add(12 * time.Minute) }), status: http.StatusUnauthorized},
		{name: "missing mutation origin", method: http.MethodPost, token: "signed", claims: validClaims, status: http.StatusForbidden},
		{name: "cross origin mutation", method: http.MethodPost, token: "signed", origin: "https://attacker.example", claims: validClaims, status: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validator := &stubIAPValidator{claims: test.claims, err: test.err}
			handler, err := RequireIAP(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
				response.WriteHeader(http.StatusNoContent)
			}), validator, security)
			if err != nil {
				t.Fatalf("RequireIAP() error = %v", err)
			}
			request := httptest.NewRequest(test.method, "https://service.run.app/api/v1/entries", nil)
			if test.token != "" {
				request.Header.Set("X-Goog-IAP-JWT-Assertion", test.token)
			}
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.status, response.Body.String())
			}
			if validator.aud != security.Audience {
				t.Fatalf("validator audience = %q", validator.aud)
			}
		})
	}
}

func TestIAPSecurityValidation(t *testing.T) {
	valid := IAPSecurity{Audience: "audience", OwnerEmail: "owner@example.com", ExternalOrigin: "https://example.com"}
	tests := []IAPSecurity{
		{},
		{Audience: " audience", OwnerEmail: valid.OwnerEmail, ExternalOrigin: valid.ExternalOrigin},
		{Audience: valid.Audience, OwnerEmail: "owner", ExternalOrigin: valid.ExternalOrigin},
		{Audience: valid.Audience, OwnerEmail: valid.OwnerEmail, ExternalOrigin: "http://example.com"},
		{Audience: valid.Audience, OwnerEmail: valid.OwnerEmail, ExternalOrigin: "https://example.com/path"},
	}
	for _, security := range tests {
		if err := security.Validate(); err == nil {
			t.Fatalf("Validate(%+v) error = nil", security)
		}
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid security error = %v", err)
	}
}

func replaceClaims(claims IAPClaims, replace func(*IAPClaims)) IAPClaims {
	replace(&claims)
	return claims
}
