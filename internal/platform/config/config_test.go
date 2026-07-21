package config

import "testing"

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

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
