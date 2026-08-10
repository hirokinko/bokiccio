package webprod

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/hirokinko/bokiccio/internal/webapp"
	"google.golang.org/api/idtoken"
)

type GoogleIAPValidator struct {
	validator *idtoken.Validator
}

func NewGoogleIAPValidator(ctx context.Context) (*GoogleIAPValidator, error) {
	validator, err := idtoken.NewValidator(ctx)
	if err != nil {
		return nil, err
	}
	return &GoogleIAPValidator{validator: validator}, nil
}

func (validator *GoogleIAPValidator) Validate(ctx context.Context, token, audience string) (webapp.IAPClaims, error) {
	if validator == nil || validator.validator == nil || token == "" {
		return webapp.IAPClaims{}, errors.New("IAP token is required")
	}
	if err := requireES256(token); err != nil {
		return webapp.IAPClaims{}, err
	}
	payload, err := validator.validator.Validate(ctx, token, audience)
	if err != nil {
		return webapp.IAPClaims{}, err
	}
	email, ok := payload.Claims["email"].(string)
	if !ok {
		return webapp.IAPClaims{}, errors.New("IAP token has no email claim")
	}
	return webapp.IAPClaims{
		Issuer:   payload.Issuer,
		Subject:  payload.Subject,
		Email:    email,
		IssuedAt: time.Unix(payload.IssuedAt, 0),
		Expires:  time.Unix(payload.Expires, 0),
	}, nil
}

func requireES256(token string) error {
	segments := strings.Split(token, ".")
	if len(segments) != 3 {
		return errors.New("IAP token is malformed")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(segments[0])
	if err != nil {
		return errors.New("IAP token header is malformed")
	}
	var header struct {
		Algorithm string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil || header.Algorithm != "ES256" {
		return errors.New("IAP token must use ES256")
	}
	return nil
}
