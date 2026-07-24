package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestStrategyRolloutDefaultsAreSafe(t *testing.T) {
	t.Parallel()
	value, err := FromLookup(mapLookup(nil))
	if err != nil {
		t.Fatal(err)
	}
	if !value.Strategy.Enabled || value.Strategy.RealProviderEnabled || !value.Strategy.ApproveEnabled ||
		value.Strategy.PackageToCreativeEnabled || value.Strategy.CriticEnabled ||
		value.Strategy.TextModelAlias != "cookies.text.standard" ||
		value.Strategy.PromptVersion != "strategy.generate.v2" ||
		len(value.Strategy.OrganizationAllowlist) != 0 {
		t.Fatalf("unexpected Strategy defaults: %#v", value.Strategy)
	}
	if !strings.Contains(value.MySQL.DSN, "127.0.0.1:3307") {
		t.Fatalf("default MySQL DSN does not use the isolated local port: %q", value.MySQL.DSN)
	}
}

func TestPasswordAuthenticationDefaultsToLocalOnly(t *testing.T) {
	t.Parallel()
	local, err := FromLookup(mapLookup(nil))
	if err != nil || !local.Auth.PasswordEnabled {
		t.Fatalf("local password authentication default = %#v, %v", local.Auth, err)
	}
	productionValues := secureProductionValues()
	production, err := FromLookup(mapLookup(productionValues))
	if err != nil || production.Auth.PasswordEnabled {
		t.Fatalf("production password authentication default = %#v, %v", production.Auth, err)
	}
	productionValues["COOKIES_PASSWORD_AUTH_ENABLED"] = "true"
	if _, err := FromLookup(mapLookup(productionValues)); err == nil {
		t.Fatal("production accepted the local default administrator password")
	}
}

func TestStrategyRolloutAllowsExplicitCreativeIntegration(t *testing.T) {
	t.Parallel()
	value, err := FromLookup(mapLookup(map[string]string{"COOKIES_STRATEGY_PACKAGE_TO_CREATIVE_ENABLED": "true"}))
	if err != nil || !value.Strategy.PackageToCreativeEnabled {
		t.Fatalf("expected explicit Strategy-to-Creative integration to be enabled: %#v, %v", value.Strategy, err)
	}
}

func TestStrategyRolloutRejectsInvalidBoolean(t *testing.T) {
	t.Parallel()
	_, err := FromLookup(mapLookup(map[string]string{"COOKIES_STRATEGY_APPROVE_ENABLED": "tru"}))
	if err == nil {
		t.Fatal("expected an invalid approval flag to fail closed")
	}
}

func TestStrategyCriticRequiresRealProvider(t *testing.T) {
	t.Parallel()
	_, err := FromLookup(mapLookup(map[string]string{"COOKIES_STRATEGY_CRITIC_ENABLED": "true"}))
	if err == nil {
		t.Fatal("expected Strategy critic without a real provider to be rejected")
	}
}

func TestAdapterGatewayRequiresExternalMasterKeyAndSupportsProduction(t *testing.T) {
	t.Parallel()
	_, err := FromLookup(mapLookup(map[string]string{"COOKIES_PROVIDER_IMAGE_ADAPTER": "adapter_gateway"}))
	if err == nil {
		t.Fatal("expected adapter gateway without a master key to be rejected")
	}
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	config, err := FromLookup(mapLookup(map[string]string{
		"COOKIES_ENV": "production", "COOKIES_BLOB_PROVIDER": "tos",
		"COOKIES_TOS_ENDPOINT": "tos.example.com", "COOKIES_TOS_REGION": "cn-test",
		"COOKIES_TOS_ACCESS_KEY": "key", "COOKIES_TOS_SECRET_KEY": "secret",
		"COOKIES_SCANNER_MODE": "clamav", "COOKIES_CLAMAV_ADDRESS": "127.0.0.1:3310",
		"COOKIES_PROVIDER_IMAGE_ADAPTER": "adapter_gateway",
		"COOKIES_PROVIDER_MASTER_KEY":    key, "COOKIES_PROVIDER_MASTER_KEY_VERSION": "kms-v1",
	}))
	if err != nil || config.Provider.ImageAdapter != "adapter_gateway" {
		t.Fatalf("valid production adapter gateway config rejected: config=%#v err=%v", config.Provider, err)
	}
}

func TestAdapterGatewayAllowsInsecureHTTPOnlyForLocalIntegration(t *testing.T) {
	t.Parallel()
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	local, err := FromLookup(mapLookup(map[string]string{
		"COOKIES_ENV": "local", "COOKIES_PROVIDER_IMAGE_ADAPTER": "adapter_gateway",
		"COOKIES_PROVIDER_MASTER_KEY":          key,
		"COOKIES_PROVIDER_ALLOW_INSECURE_HTTP": "true",
	}))
	if err != nil || !local.Provider.AllowInsecureHTTP {
		t.Fatalf("local insecure HTTP config rejected: config=%#v err=%v", local.Provider, err)
	}
	_, err = FromLookup(mapLookup(map[string]string{
		"COOKIES_ENV": "staging", "COOKIES_PROVIDER_IMAGE_ADAPTER": "adapter_gateway",
		"COOKIES_PROVIDER_MASTER_KEY":          key,
		"COOKIES_PROVIDER_ALLOW_INSECURE_HTTP": "true",
	}))
	if err == nil {
		t.Fatal("staging accepted local-only insecure HTTP setting")
	}
}

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
	if config.ObjectStorage.Provider != "filesystem" || config.ObjectStorage.FilesystemRoot != ".data/blobs" {
		t.Fatalf("unexpected local object storage: %#v", config.ObjectStorage)
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

func TestArkImageAdapterIsExplicitAndLocalOnly(t *testing.T) {
	t.Parallel()
	_, err := FromLookup(mapLookup(map[string]string{"COOKIES_PROVIDER_IMAGE_ADAPTER": "ark_image"}))
	if err == nil {
		t.Fatal("expected Ark configuration without credentials to be rejected")
	}
	config, err := FromLookup(mapLookup(map[string]string{
		"COOKIES_ENV": "local", "COOKIES_PROVIDER_IMAGE_ADAPTER": "ark_image",
		"COOKIES_ARK_IMAGE_API_KEY": "test-key", "COOKIES_ARK_IMAGE_MODEL": "seedream-test",
	}))
	if err != nil || config.Provider.ImageAdapter != "ark_image" {
		t.Fatalf("valid local Ark configuration rejected: config=%#v err=%v", config.Provider, err)
	}
	_, err = FromLookup(mapLookup(map[string]string{
		"COOKIES_ENV": "staging", "COOKIES_BLOB_PROVIDER": "memory", "COOKIES_PROVIDER_IMAGE_ADAPTER": "ark_image",
		"COOKIES_ARK_IMAGE_API_KEY": "test-key", "COOKIES_ARK_IMAGE_MODEL": "seedream-test",
	}))
	if err == nil {
		t.Fatal("expected Ark adapter outside local to be rejected")
	}
}

func TestFromLookupUsesObjectStorageCompatibilityNamesForTOS(t *testing.T) {
	t.Parallel()
	config, err := FromLookup(mapLookup(map[string]string{
		"COOKIES_BLOB_PROVIDER":            "tos",
		"OBJECT_STORAGE_ENDPOINT":          "tos.compat.example",
		"OBJECT_STORAGE_REGION":            "cn-test",
		"OBJECT_STORAGE_ACCESS_KEY":        "test-access-key",
		"OBJECT_STORAGE_SECRET_KEY":        "test-secret-key",
		"OBJECT_STORAGE_SECURITY_TOKEN":    "test-security-token",
		"OBJECT_STORAGE_QUARANTINE_BUCKET": "compat-quarantine",
		"OBJECT_STORAGE_ASSETS_BUCKET":     "compat-assets",
	}))
	if err != nil {
		t.Fatalf("FromLookup() error = %v", err)
	}
	if got, want := config.ObjectStorage.Endpoint, "tos.compat.example"; got != want {
		t.Fatalf("Endpoint = %q, want %q", got, want)
	}
	if got, want := config.ObjectStorage.Region, "cn-test"; got != want {
		t.Fatalf("Region = %q, want %q", got, want)
	}
	if got, want := config.ObjectStorage.AccessKey, "test-access-key"; got != want {
		t.Fatalf("AccessKey = %q, want %q", got, want)
	}
	if got, want := config.ObjectStorage.SecretKey, "test-secret-key"; got != want {
		t.Fatalf("SecretKey = %q, want %q", got, want)
	}
	if got, want := config.ObjectStorage.SecurityToken, "test-security-token"; got != want {
		t.Fatalf("SecurityToken = %q, want %q", got, want)
	}
	if got, want := config.ObjectStorage.QuarantineBucket, "compat-quarantine"; got != want {
		t.Fatalf("QuarantineBucket = %q, want %q", got, want)
	}
	if got, want := config.ObjectStorage.AssetsBucket, "compat-assets"; got != want {
		t.Fatalf("AssetsBucket = %q, want %q", got, want)
	}
}

func secureProductionValues() map[string]string {
	return map[string]string{
		"COOKIES_ENV":            "production",
		"COOKIES_BLOB_PROVIDER":  "tos",
		"COOKIES_TOS_ENDPOINT":   "tos.example.com",
		"COOKIES_TOS_REGION":     "cn-test",
		"COOKIES_TOS_ACCESS_KEY": "key",
		"COOKIES_TOS_SECRET_KEY": "secret",
		"COOKIES_SCANNER_MODE":   "clamav",
		"COOKIES_CLAMAV_ADDRESS": "127.0.0.1:3310",
	}
}

func TestFromLookupUsesLegacyObjectStorageNamesForTOS(t *testing.T) {
	t.Parallel()
	config, err := FromLookup(mapLookup(map[string]string{
		"OBJECT_STORAGE_PROVIDER":          "tos",
		"OBJECT_STORAGE_ENDPOINT":          "tos.compat.example",
		"OBJECT_STORAGE_REGION":            "cn-test",
		"OBJECT_STORAGE_ACCESS_KEY_ID":     "test-access-key",
		"OBJECT_STORAGE_ACCESS_KEY_SECRET": "test-secret-key",
		"OBJECT_STORAGE_BUCKET_NAME":       "compat-assets",
	}))
	if err != nil {
		t.Fatalf("FromLookup() error = %v", err)
	}
	if got, want := config.ObjectStorage.Provider, "tos"; got != want {
		t.Fatalf("Provider = %q, want %q", got, want)
	}
	if got, want := config.ObjectStorage.AccessKey, "test-access-key"; got != want {
		t.Fatalf("AccessKey = %q, want %q", got, want)
	}
	if got, want := config.ObjectStorage.SecretKey, "test-secret-key"; got != want {
		t.Fatalf("SecretKey = %q, want %q", got, want)
	}
	if got, want := config.ObjectStorage.AssetsBucket, "compat-assets"; got != want {
		t.Fatalf("AssetsBucket = %q, want %q", got, want)
	}
}

func TestFromLookupPrefersCookiesTOSConfiguration(t *testing.T) {
	t.Parallel()
	config, err := FromLookup(mapLookup(map[string]string{
		"COOKIES_BLOB_PROVIDER":            "tos",
		"COOKIES_TOS_ENDPOINT":             "tos.cookies.example",
		"COOKIES_TOS_REGION":               "cn-cookies",
		"COOKIES_TOS_ACCESS_KEY":           "cookies-access-key",
		"COOKIES_TOS_SECRET_KEY":           "cookies-secret-key",
		"COOKIES_TOS_QUARANTINE_BUCKET":    "cookies-quarantine",
		"COOKIES_TOS_ASSETS_BUCKET":        "cookies-assets",
		"OBJECT_STORAGE_ENDPOINT":          "tos.compat.example",
		"OBJECT_STORAGE_REGION":            "cn-compat",
		"OBJECT_STORAGE_ACCESS_KEY":        "compat-access-key",
		"OBJECT_STORAGE_SECRET_KEY":        "compat-secret-key",
		"OBJECT_STORAGE_QUARANTINE_BUCKET": "compat-quarantine",
		"OBJECT_STORAGE_ASSETS_BUCKET":     "compat-assets",
	}))
	if err != nil {
		t.Fatalf("FromLookup() error = %v", err)
	}
	if got, want := config.ObjectStorage.Endpoint, "tos.cookies.example"; got != want {
		t.Fatalf("Endpoint = %q, want %q", got, want)
	}
	if got, want := config.ObjectStorage.AccessKey, "cookies-access-key"; got != want {
		t.Fatalf("AccessKey = %q, want %q", got, want)
	}
	if got, want := config.ObjectStorage.AssetsBucket, "cookies-assets"; got != want {
		t.Fatalf("AssetsBucket = %q, want %q", got, want)
	}
}

func TestArkTextAdapterIsExplicitAndLocalOnly(t *testing.T) {
	t.Parallel()
	_, err := FromLookup(mapLookup(map[string]string{"COOKIES_PROVIDER_TEXT_ADAPTER": "ark_text"}))
	if err == nil {
		t.Fatal("expected Ark text configuration without credentials to be rejected")
	}
	config, err := FromLookup(mapLookup(map[string]string{
		"COOKIES_ENV":                   "local",
		"COOKIES_PROVIDER_TEXT_ADAPTER": "ark_text",
		"COOKIES_ARK_TEXT_API_KEY":      "test-key",
		"COOKIES_ARK_TEXT_MODEL":        "doubao-test",
	}))
	if err != nil || config.Provider.TextAdapter != "ark_text" || config.Provider.ArkText.Model != "doubao-test" {
		t.Fatalf("valid local Ark text configuration rejected: config=%#v err=%v", config.Provider, err)
	}
	_, err = FromLookup(mapLookup(map[string]string{
		"COOKIES_ENV":                   "staging",
		"COOKIES_BLOB_PROVIDER":         "memory",
		"COOKIES_PROVIDER_TEXT_ADAPTER": "ark_text",
		"COOKIES_ARK_TEXT_API_KEY":      "test-key",
		"COOKIES_ARK_TEXT_MODEL":        "doubao-test",
	}))
	if err == nil {
		t.Fatal("expected Ark text adapter outside local to be rejected")
	}
}

func TestOpenAIImageAdapterRequiresCompleteLocalGatewayConfiguration(t *testing.T) {
	t.Parallel()
	_, err := FromLookup(mapLookup(map[string]string{"COOKIES_PROVIDER_IMAGE_ADAPTER": "openai_image"}))
	if err == nil {
		t.Fatal("expected incomplete OpenAI-compatible image configuration to be rejected")
	}
	config, err := FromLookup(mapLookup(map[string]string{
		"COOKIES_PROVIDER_IMAGE_ADAPTER": "openai_image", "COOKIES_OPENAI_IMAGE_API_KEY": "test-key",
		"COOKIES_OPENAI_IMAGE_MODEL": "gpt-image-2", "COOKIES_OPENAI_IMAGE_BASE_URL": "http://gateway.example",
	}))
	if err != nil || config.Provider.ImageAdapter != "openai_image" {
		t.Fatalf("valid local OpenAI-compatible configuration rejected: config=%#v err=%v", config.Provider, err)
	}
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}
