package webprod

import (
	"context"
	"strings"
	"testing"
)

func TestLoadServerConfig(t *testing.T) {
	values := map[string]string{
		DatabaseURLEnv:    "libsql://bokiccio-example.turso.io",
		DatabaseTokenEnv:  "private-token",
		IAPAudienceEnv:    "/projects/123/locations/asia-northeast1/services/bokiccio",
		ExternalOriginEnv: "https://bokiccio.example.com,https://bokiccio-123.asia-northeast1.run.app",
		PortEnv:           "8080",
	}
	config, err := LoadServerConfig(mapLookup(values))
	if err != nil {
		t.Fatalf("LoadServerConfig() error = %v", err)
	}
	if config.Database.URL != values[DatabaseURLEnv] || config.Database.authToken != values[DatabaseTokenEnv] || config.Port != 8080 {
		t.Fatalf("config = %+v", config)
	}
	if config.Security.Audience != values[IAPAudienceEnv] {
		t.Fatalf("security = %+v", config.Security)
	}
	if config.Security.ExternalOrigin != values[ExternalOriginEnv] {
		t.Fatalf("external origin = %q, want %q", config.Security.ExternalOrigin, values[ExternalOriginEnv])
	}
}

func TestLoadServerConfigDevelopmentErrorsAreExplicit(t *testing.T) {
	values := map[string]string{
		DatabaseURLEnv: "libsql://bokiccio-example.turso.io", DatabaseTokenEnv: "private-token",
		IAPAudienceEnv:    "/projects/123/locations/asia-northeast1/services/bokiccio",
		ExternalOriginEnv: "https://bokiccio.example.com", PortEnv: "8080",
	}
	production, err := LoadServerConfig(mapLookup(values))
	if err != nil || production.Development {
		t.Fatalf("default config=%+v error=%v", production, err)
	}
	values[EnvironmentEnv] = "development"
	development, err := LoadServerConfig(mapLookup(values))
	if err != nil || !development.Development {
		t.Fatalf("development config=%+v error=%v", development, err)
	}
	values[EnvironmentEnv] = "private-invalid-value"
	if _, err := LoadServerConfig(mapLookup(values)); err == nil || !strings.Contains(err.Error(), EnvironmentEnv) || strings.Contains(err.Error(), "private-invalid-value") {
		t.Fatalf("invalid environment error=%v", err)
	}
}

func TestConfigurationFailuresDoNotExposeToken(t *testing.T) {
	secret := "do-not-print-this-token"
	tests := []map[string]string{
		{DatabaseTokenEnv: secret},
		{DatabaseURLEnv: "https://wrong.example", DatabaseTokenEnv: secret},
		{DatabaseURLEnv: "libsql://db.example?authToken=" + secret, DatabaseTokenEnv: secret},
	}
	for _, values := range tests {
		_, err := LoadServerConfig(mapLookup(values))
		if err == nil {
			t.Fatalf("LoadServerConfig(%v) error = nil", values)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error exposes token: %v", err)
		}
	}
	if _, err := OpenRemote(context.Background(), DatabaseConfig{URL: "invalid", authToken: secret}); err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("OpenRemote() error = %v", err)
	}
}

func TestServerConfigRequiresEverySecuritySetting(t *testing.T) {
	base := map[string]string{
		DatabaseURLEnv:    "libsql://bokiccio-example.turso.io",
		DatabaseTokenEnv:  "token",
		IAPAudienceEnv:    "/projects/123/locations/asia-northeast1/services/bokiccio",
		ExternalOriginEnv: "https://bokiccio.example.com",
		PortEnv:           "8080",
	}
	for _, name := range []string{DatabaseURLEnv, DatabaseTokenEnv, IAPAudienceEnv, ExternalOriginEnv, PortEnv} {
		values := make(map[string]string, len(base))
		for key, value := range base {
			values[key] = value
		}
		delete(values, name)
		_, err := LoadServerConfig(mapLookup(values))
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Fatalf("missing %s error = %v", name, err)
		}
	}
}

func mapLookup(values map[string]string) LookupEnv {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}
