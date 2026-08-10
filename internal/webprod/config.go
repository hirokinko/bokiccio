// Package webprod composes production-only database and identity providers.
package webprod

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/hirokinko/bokiccio/internal/webapp"
)

const (
	DatabaseURLEnv    = "TURSO_DATABASE_URL"
	DatabaseTokenEnv  = "TURSO_AUTH_TOKEN"
	IAPAudienceEnv    = "BOKICCIO_IAP_AUDIENCE"
	OwnerEmailEnv     = "BOKICCIO_OWNER_EMAIL"
	ExternalOriginEnv = "BOKICCIO_EXTERNAL_ORIGIN"
	PortEnv           = "PORT"
)

type LookupEnv func(string) (string, bool)

type DatabaseConfig struct {
	URL       string
	authToken string
}

type ServerConfig struct {
	Database DatabaseConfig
	Security webapp.IAPSecurity
	Port     int
}

func LoadDatabaseConfig(lookup LookupEnv) (DatabaseConfig, error) {
	if lookup == nil {
		return DatabaseConfig{}, errors.New("environment lookup is required")
	}
	databaseURL, err := requiredEnvironment(lookup, DatabaseURLEnv)
	if err != nil {
		return DatabaseConfig{}, err
	}
	authToken, err := requiredEnvironment(lookup, DatabaseTokenEnv)
	if err != nil {
		return DatabaseConfig{}, err
	}
	if err := validateDatabaseURL(databaseURL); err != nil {
		return DatabaseConfig{}, fmt.Errorf("%s is invalid", DatabaseURLEnv)
	}
	return DatabaseConfig{URL: databaseURL, authToken: authToken}, nil
}

func LoadServerConfig(lookup LookupEnv) (ServerConfig, error) {
	database, err := LoadDatabaseConfig(lookup)
	if err != nil {
		return ServerConfig{}, err
	}
	audience, err := requiredEnvironment(lookup, IAPAudienceEnv)
	if err != nil {
		return ServerConfig{}, err
	}
	if !cloudRunIAPAudience.MatchString(audience) {
		return ServerConfig{}, fmt.Errorf("%s is not a Cloud Run IAP audience", IAPAudienceEnv)
	}
	ownerEmail, err := requiredEnvironment(lookup, OwnerEmailEnv)
	if err != nil {
		return ServerConfig{}, err
	}
	externalOrigin, err := requiredEnvironment(lookup, ExternalOriginEnv)
	if err != nil {
		return ServerConfig{}, err
	}
	portText, err := requiredEnvironment(lookup, PortEnv)
	if err != nil {
		return ServerConfig{}, err
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return ServerConfig{}, fmt.Errorf("%s must be an integer between 1 and 65535", PortEnv)
	}
	security := webapp.IAPSecurity{Audience: audience, OwnerEmail: ownerEmail, ExternalOrigin: externalOrigin}
	if err := security.Validate(); err != nil {
		return ServerConfig{}, fmt.Errorf("security configuration is invalid: %w", err)
	}
	return ServerConfig{Database: database, Security: security, Port: port}, nil
}

var cloudRunIAPAudience = regexp.MustCompile(`^/projects/[1-9][0-9]*/locations/[a-z][a-z0-9-]*/services/[a-z][a-z0-9-]*$`)

func requiredEnvironment(lookup LookupEnv, name string) (string, error) {
	value, ok := lookup(name)
	if !ok || value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func validateDatabaseURL(value string) error {
	if strings.TrimSpace(value) != value {
		return errors.New("surrounding whitespace")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "libsql" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("expected a credential-free libsql URL")
	}
	return nil
}
