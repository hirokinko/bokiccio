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
	OwnerEmail     string
	ExternalOrigin string
}

func (security IAPSecurity) Validate() error {
	if security.Audience == "" || strings.TrimSpace(security.Audience) != security.Audience {
		return errors.New("IAP audience is required")
	}
	if security.OwnerEmail == "" || strings.TrimSpace(security.OwnerEmail) != security.OwnerEmail || !strings.Contains(security.OwnerEmail, "@") {
		return errors.New("owner email is invalid")
	}
	origin, err := url.Parse(security.ExternalOrigin)
	if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") {
		return errors.New("external origin must be an HTTPS origin without path, query, or fragment")
	}
	if origin.String() != security.ExternalOrigin && strings.TrimSuffix(origin.String(), "/") != security.ExternalOrigin {
		return errors.New("external origin must be canonical")
	}
	return nil
}

func RequireIAP(next http.Handler, validator IAPTokenValidator, security IAPSecurity) (http.Handler, error) {
	if next == nil || validator == nil {
		return nil, errors.New("IAP middleware dependencies are required")
	}
	if err := security.Validate(); err != nil {
		return nil, err
	}
	externalOrigin := strings.TrimSuffix(security.ExternalOrigin, "/")
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		token := request.Header.Get("X-Goog-IAP-JWT-Assertion")
		claims, err := validator.Validate(request.Context(), token, security.Audience)
		if err != nil || token == "" || !validIAPClaims(claims, security.OwnerEmail, time.Now()) {
			writeSecurityError(response, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}
		if changesState(request.Method) && request.Header.Get("Origin") != externalOrigin {
			writeSecurityError(response, http.StatusForbidden, "origin_forbidden", "request origin is not allowed")
			return
		}
		response.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(response, request)
	}), nil
}

func validIAPClaims(claims IAPClaims, ownerEmail string, now time.Time) bool {
	const clockSkew = 30 * time.Second
	const maximumLifetime = 10*time.Minute + 2*clockSkew
	if claims.Issuer != iapIssuer || claims.Subject == "" || claims.Email == "" || !strings.EqualFold(claims.Email, ownerEmail) {
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

func writeSecurityError(response http.ResponseWriter, status int, code, message string) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(struct {
		SchemaVersion int    `json:"schema_version"`
		Code          string `json:"code"`
		Message       string `json:"message"`
	}{SchemaVersion: APISchemaVersion, Code: code, Message: message})
}
