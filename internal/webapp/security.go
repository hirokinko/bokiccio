package webapp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const iapIssuer = "https://cloud.google.com/iap"

type IAPClaims struct {
	Issuer   string
	Subject  string
	Email    string
	IssuedAt time.Time
	Expires  time.Time
}

type IAPTokenValidator interface {
	Validate(context.Context, string, string) (IAPClaims, error)
}

type IAPSecurity struct {
	Audience       string
	ExternalOrigin string
}

type SecurityError struct {
	Status  int
	Code    string
	Message string
}

type SecurityErrorWriter func(http.ResponseWriter, *http.Request, SecurityError)

func (security IAPSecurity) Validate() error {
	if security.Audience == "" || strings.TrimSpace(security.Audience) != security.Audience {
		return errors.New("IAP audience is required")
	}
	if _, err := allowedExternalOrigins(security.ExternalOrigin); err != nil {
		return err
	}
	return nil
}

func RequireIAP(next http.Handler, validator IAPTokenValidator, security IAPSecurity) (http.Handler, error) {
	return RequireIAPWithErrorWriter(next, validator, security, nil)
}

func RequireIAPWithErrorWriter(next http.Handler, validator IAPTokenValidator, security IAPSecurity, writeError SecurityErrorWriter) (http.Handler, error) {
	if next == nil || validator == nil {
		return nil, errors.New("IAP middleware dependencies are required")
	}
	if err := security.Validate(); err != nil {
		return nil, err
	}
	origins, err := allowedExternalOrigins(security.ExternalOrigin)
	if err != nil {
		return nil, err
	}
	allowedOrigins := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowedOrigins[origin] = struct{}{}
	}
	if writeError == nil {
		writeError = writeSecurityError
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		token := request.Header.Get("X-Goog-IAP-JWT-Assertion")
		claims, err := validator.Validate(request.Context(), token, security.Audience)
		if err != nil || token == "" || !validIAPClaims(claims, time.Now()) {
			writeError(response, request, SecurityError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "authentication required"})
			return
		}
		if changesState(request.Method) && !originAllowed(request.Header.Get("Origin"), allowedOrigins) {
			writeError(response, request, SecurityError{Status: http.StatusForbidden, Code: "origin_forbidden", Message: "request origin is not allowed"})
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(response, request)
	}), nil
}

func allowedExternalOrigins(value string) ([]string, error) {
	origins := strings.Split(value, ",")
	allowed := make([]string, 0, len(origins))
	seen := make(map[string]struct{}, len(origins))
	for _, originText := range origins {
		originText = strings.TrimSpace(originText)
		if originText == "" {
			return nil, errors.New("external origin must not contain empty entries")
		}
		origin, err := url.Parse(originText)
		if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") {
			return nil, errors.New("external origin must be an HTTPS origin without path, query, or fragment")
		}
		if origin.String() != originText && strings.TrimSuffix(origin.String(), "/") != originText {
			return nil, errors.New("external origin must be canonical")
		}
		normalized := strings.TrimSuffix(origin.String(), "/")
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		allowed = append(allowed, normalized)
	}
	return allowed, nil
}

func originAllowed(origin string, allowed map[string]struct{}) bool {
	_, ok := allowed[origin]
	return ok
}

func validIAPClaims(claims IAPClaims, now time.Time) bool {
	const clockSkew = 30 * time.Second
	const maximumLifetime = 10*time.Minute + 2*clockSkew
	if claims.Issuer != iapIssuer || claims.Subject == "" || claims.Email == "" {
		return false
	}
	if claims.IssuedAt.IsZero() || claims.Expires.IsZero() || claims.IssuedAt.After(now.Add(clockSkew)) || !claims.Expires.After(now.Add(-clockSkew)) {
		return false
	}
	return claims.Expires.Sub(claims.IssuedAt) <= maximumLifetime
}

func changesState(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return false
	default:
		return true
	}
}

func WriteSecurityJSON(response http.ResponseWriter, securityError SecurityError) {
	writeSecurityError(response, nil, securityError)
}

func writeSecurityError(response http.ResponseWriter, _ *http.Request, securityError SecurityError) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(securityError.Status)
	_ = json.NewEncoder(response).Encode(struct {
		SchemaVersion int    `json:"schema_version"`
		Code          string `json:"code"`
		Message       string `json:"message"`
	}{SchemaVersion: APISchemaVersion, Code: securityError.Code, Message: securityError.Message})
}
