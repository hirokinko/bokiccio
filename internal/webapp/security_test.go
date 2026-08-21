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
	primaryOrigin := "https://bokiccio.example.com"
	secondaryOrigin := "https://bokiccio-123.asia-northeast1.run.app"
	security := IAPSecurity{
		Audience:       "/projects/123/locations/asia-northeast1/services/bokiccio",
		ExternalOrigin: primaryOrigin + "," + secondaryOrigin,
	}
	validClaims := IAPClaims{
		Issuer:   iapIssuer,
		Subject:  "iap-user-subject",
		Email:    "iap-user@example.com",
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
		{name: "authenticated mutation primary origin", method: http.MethodPost, token: "signed", origin: primaryOrigin, claims: validClaims, status: http.StatusNoContent},
		{name: "authenticated mutation secondary origin", method: http.MethodPost, token: "signed", origin: secondaryOrigin, claims: validClaims, status: http.StatusNoContent},
		{name: "missing token", method: http.MethodGet, claims: validClaims, status: http.StatusUnauthorized},
		{name: "invalid signature", method: http.MethodGet, token: "invalid", claims: validClaims, err: errors.New("invalid"), status: http.StatusUnauthorized},
		{name: "wrong issuer", method: http.MethodGet, token: "signed", claims: replaceClaims(validClaims, func(value *IAPClaims) { value.Issuer = "https://accounts.google.com" }), status: http.StatusUnauthorized},
		{name: "different IAP user", method: http.MethodGet, token: "signed", claims: replaceClaims(validClaims, func(value *IAPClaims) { value.Email = "other@example.com" }), status: http.StatusNoContent},
		{name: "missing subject", method: http.MethodGet, token: "signed", claims: replaceClaims(validClaims, func(value *IAPClaims) { value.Subject = "" }), status: http.StatusUnauthorized},
		{name: "missing email", method: http.MethodGet, token: "signed", claims: replaceClaims(validClaims, func(value *IAPClaims) { value.Email = "" }), status: http.StatusUnauthorized},
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
	valid := IAPSecurity{Audience: "audience", ExternalOrigin: "https://example.com"}
	validMultiple := IAPSecurity{Audience: "audience", ExternalOrigin: "https://example.com, https://service.run.app/"}
	tests := []IAPSecurity{
		{},
		{Audience: " audience", ExternalOrigin: valid.ExternalOrigin},
		{Audience: valid.Audience, ExternalOrigin: "http://example.com"},
		{Audience: valid.Audience, ExternalOrigin: "https://example.com/path"},
		{Audience: valid.Audience, ExternalOrigin: "https://example.com,"},
	}
	for _, security := range tests {
		if err := security.Validate(); err == nil {
			t.Fatalf("Validate(%+v) error = nil", security)
		}
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid security error = %v", err)
	}
	if err := validMultiple.Validate(); err != nil {
		t.Fatalf("valid multiple security error = %v", err)
	}
}

func TestRequireIAPStoresValidatedClaimsInRequestContext(t *testing.T) {
	now := time.Now()
	want := IAPClaims{
		Issuer:   iapIssuer,
		Subject:  "iap-user-subject",
		Email:    "iap-user@example.com",
		IssuedAt: now.Add(-time.Minute),
		Expires:  now.Add(5 * time.Minute),
	}
	validator := &stubIAPValidator{claims: want}
	handler, err := RequireIAP(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		claims, ok := IAPClaimsFromContext(request.Context())
		if !ok {
			t.Fatal("IAPClaimsFromContext() ok = false")
		}
		if claims != want {
			t.Fatalf("IAPClaimsFromContext() claims = %+v, want %+v", claims, want)
		}
		response.WriteHeader(http.StatusNoContent)
	}), validator, IAPSecurity{Audience: "audience", ExternalOrigin: "https://example.com"})
	if err != nil {
		t.Fatalf("RequireIAP() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)
	request.Header.Set("X-Goog-IAP-JWT-Assertion", "signed")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestIAPClaimsFromContextRejectsUntrustedValues(t *testing.T) {
	if claims, ok := IAPClaimsFromContext(context.Background()); ok || claims != (IAPClaims{}) {
		t.Fatalf("IAPClaimsFromContext(empty) claims = %+v, ok = %t", claims, ok)
	}
	ctx := context.WithValue(context.Background(), "iap-claims", IAPClaims{Email: "attacker@example.com"})
	if claims, ok := IAPClaimsFromContext(ctx); ok || claims != (IAPClaims{}) {
		t.Fatalf("IAPClaimsFromContext(untrusted) claims = %+v, ok = %t", claims, ok)
	}
}

func TestNormalizeEmail(t *testing.T) {
	got, err := NormalizeEmail("  Operator+Demo@Example.COM \t")
	if err != nil || got != "operator+demo@example.com" {
		t.Fatalf("NormalizeEmail() = %q, %v", got, err)
	}
	for _, invalid := range []string{"", "   ", "not-an-email", "Operator <operator@example.com>"} {
		if got, err := NormalizeEmail(invalid); !errors.Is(err, ErrInvalidEmail) || got != "" {
			t.Fatalf("NormalizeEmail(%q) = %q, %v", invalid, got, err)
		}
	}
}

func replaceClaims(claims IAPClaims, replace func(*IAPClaims)) IAPClaims {
	replace(&claims)
	return claims
}
