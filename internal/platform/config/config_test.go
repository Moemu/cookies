package config

import (
	"strings"
	"testing"
)

func TestParseDotEnvAcceptsLocalDevelopmentValues(t *testing.T) {
	t.Parallel()
	values, err := parseDotEnv(strings.NewReader("# local only\nCOOKIES_MYSQL_DSN='cookies:pass@tcp(127.0.0.1:3307)/cookies?parseTime=true'\nexport COOKIES_HTTP_ADDR=:8080\n"))
	if err != nil {
		t.Fatalf("parseDotEnv() error = %v", err)
	}
	if values["COOKIES_MYSQL_DSN"] != "cookies:pass@tcp(127.0.0.1:3307)/cookies?parseTime=true" || values["COOKIES_HTTP_ADDR"] != ":8080" {
		t.Fatalf("unexpected dotenv values: %#v", values)
	}
}

func TestParseDotEnvRejectsMalformedLine(t *testing.T) {
	t.Parallel()
	if _, err := parseDotEnv(strings.NewReader("COOKIES_ENV\n")); err == nil {
		t.Fatal("parseDotEnv() error = nil, want malformed line rejection")
	}
}

func TestFromLookupRejectsLocalIdentityOutsideLocal(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"COOKIES_ENV":                   "production",
		"COOKIES_LOCAL_ORGANIZATION_ID": "org_1",
		"COOKIES_LOCAL_PRINCIPAL_KIND":  "user",
		"COOKIES_LOCAL_PRINCIPAL_ID":    "usr_1",
	}
	_, err := FromLookup(mapLookup(values))
	if err == nil {
		t.Fatal("expected production local identity to be rejected")
	}
}

func TestFromLookupBuildsExplicitLocalIdentity(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"COOKIES_ENV":                   "local",
		"COOKIES_HTTP_ADDR":             "127.0.0.1:8081",
		"COOKIES_LOCAL_ORGANIZATION_ID": "org_1",
		"COOKIES_LOCAL_PRINCIPAL_KIND":  "user",
		"COOKIES_LOCAL_PRINCIPAL_ID":    "usr_1",
		"COOKIES_LOCAL_PROJECT_ID":      "project_1",
		"COOKIES_LOCAL_SCOPES":          "project.read, project.write",
	}
	config, err := FromLookup(mapLookup(values))
	if err != nil {
		t.Fatalf("FromLookup() error = %v", err)
	}
	if config.HTTPAddr != "127.0.0.1:8081" || config.LocalIdentity == nil {
		t.Fatalf("unexpected config: %#v", config)
	}
	if got, want := len(config.LocalIdentity.Scopes), 2; got != want {
		t.Fatalf("scope count = %d, want %d", got, want)
	}
}

func TestFromLookupRejectsInvalidMySQLPool(t *testing.T) {
	t.Parallel()

	_, err := FromLookup(mapLookup(map[string]string{
		"COOKIES_MYSQL_MAX_OPEN_CONNS": "0",
	}))
	if err == nil {
		t.Fatal("expected invalid MySQL connection pool to be rejected")
	}
}

func TestProductionRequiresTOSAndMalwareScanning(t *testing.T) {
	t.Parallel()
	_, err := FromLookup(mapLookup(map[string]string{"COOKIES_ENV": "production"}))
	if err == nil {
		t.Fatal("expected insecure production storage defaults to be rejected")
	}
	config, err := FromLookup(mapLookup(map[string]string{"COOKIES_ENV": "production", "COOKIES_BLOB_PROVIDER": "tos", "COOKIES_TOS_ENDPOINT": "tos.example.com", "COOKIES_TOS_REGION": "cn-test", "COOKIES_TOS_ACCESS_KEY": "key", "COOKIES_TOS_SECRET_KEY": "secret", "COOKIES_SCANNER_MODE": "clamav", "COOKIES_CLAMAV_ADDRESS": "127.0.0.1:3310"}))
	if err != nil {
		t.Fatalf("secure production config rejected: %v", err)
	}
	if config.ObjectStorage.Provider != "tos" || config.Scanner.Mode != "clamav" {
		t.Fatalf("unexpected config: %#v", config)
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
