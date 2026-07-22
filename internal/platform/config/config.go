// Package config owns process configuration validation. Configuration comes
// from the process environment; local development may use an ignored .env as
// a fallback, while deployed environments never read that file.
package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Environment string

const (
	EnvironmentLocal      Environment = "local"
	EnvironmentTest       Environment = "test"
	EnvironmentStaging    Environment = "staging"
	EnvironmentProduction Environment = "production"
)

type LocalIdentity struct {
	OrganizationID string
	PrincipalKind  string
	PrincipalID    string
	ProjectID      string
	Scopes         []string
}

type Config struct {
	Environment   Environment
	HTTPAddr      string
	MySQL         MySQL
	ObjectStorage ObjectStorage
	Scanner       Scanner
	Provider      Provider
	LocalIdentity *LocalIdentity
}

type ObjectStorage struct {
	Provider         string
	Endpoint         string
	Region           string
	AccessKey        string
	SecretKey        string
	SecurityToken    string
	QuarantineBucket string
	AssetsBucket     string
}

type Scanner struct {
	Mode    string
	Address string
}

// Provider contains only local composition choices. Credentials are read from
// the process environment (or ignored local .env), never from project data.
type Provider struct {
	ImageAdapter string
	ArkImage     ArkImage
	OpenAIImage  OpenAIImage
}

type ArkImage struct {
	APIKey  string
	Model   string
	BaseURL string
}

type OpenAIImage struct {
	APIKey  string
	Model   string
	BaseURL string
}

// MySQL contains only connection-pool configuration. No business module owns
// a second pool; modules receive this shared dependency instead.
type MySQL struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
}

func Load() (Config, error) {
	values, err := localDotEnvValues()
	if err != nil {
		return Config{}, err
	}
	return FromLookup(func(key string) (string, bool) {
		if value, ok := os.LookupEnv(key); ok {
			return value, true
		}
		value, ok := values[key]
		return value, ok
	})
}

// localDotEnvValues provides the local `Copy-Item .env.example .env` workflow
// without ever overriding process environment variables. A process configured
// as staging or production deliberately ignores the working-directory .env so
// deployed configuration remains environment/config-center only.
func localDotEnvValues() (map[string]string, error) {
	if environment, ok := os.LookupEnv("COOKIES_ENV"); ok && Environment(environment) != EnvironmentLocal {
		return map[string]string{}, nil
	}
	file, err := os.Open(filepath.Clean(".env"))
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open local .env: %w", err)
	}
	defer file.Close()
	values, err := parseDotEnv(file)
	if err != nil {
		return nil, fmt.Errorf("parse local .env: %w", err)
	}
	return values, nil
}

// parseDotEnv supports the constrained dotenv syntax used by .env.example:
// comments, optional `export`, KEY=VALUE, and single- or double-quoted
// values. It intentionally does not expand variables, preventing unexpected
// interpolation of credentials during local startup.
func parseDotEnv(reader io.Reader) (map[string]string, error) {
	values := make(map[string]string)
	scanner := bufio.NewScanner(reader)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		key, value, found := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !found || key == "" {
			return nil, fmt.Errorf("line %d must use KEY=VALUE", lineNumber)
		}
		value = strings.TrimSpace(value)
		if len(value) >= 2 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')) {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func FromLookup(lookup func(string) (string, bool)) (Config, error) {
	environment := Environment(valueOr(lookup, "COOKIES_ENV", string(EnvironmentLocal)))
	config := Config{
		Environment: environment,
		HTTPAddr:    valueOr(lookup, "COOKIES_HTTP_ADDR", ":8080"),
		MySQL: MySQL{
			DSN:          valueOr(lookup, "COOKIES_MYSQL_DSN", "cookies:cookies_local_development_only@tcp(127.0.0.1:3306)/cookies?parseTime=true&multiStatements=true"),
			MaxOpenConns: intValueOr(lookup, "COOKIES_MYSQL_MAX_OPEN_CONNS", 10),
			MaxIdleConns: intValueOr(lookup, "COOKIES_MYSQL_MAX_IDLE_CONNS", 5),
		},
		ObjectStorage: ObjectStorage{
			Provider: valueOr(lookup, "COOKIES_BLOB_PROVIDER", "memory"),
			Endpoint: valueOr(lookup, "COOKIES_TOS_ENDPOINT", ""), Region: valueOr(lookup, "COOKIES_TOS_REGION", ""),
			AccessKey: valueOr(lookup, "COOKIES_TOS_ACCESS_KEY", ""), SecretKey: valueOr(lookup, "COOKIES_TOS_SECRET_KEY", ""),
			SecurityToken:    valueOr(lookup, "COOKIES_TOS_SECURITY_TOKEN", ""),
			QuarantineBucket: valueOr(lookup, "COOKIES_TOS_QUARANTINE_BUCKET", "cookies-quarantine"),
			AssetsBucket:     valueOr(lookup, "COOKIES_TOS_ASSETS_BUCKET", "cookies-assets"),
		},
		Scanner: Scanner{Mode: valueOr(lookup, "COOKIES_SCANNER_MODE", "noop"), Address: valueOr(lookup, "COOKIES_CLAMAV_ADDRESS", "")},
		Provider: Provider{
			ImageAdapter: valueOr(lookup, "COOKIES_PROVIDER_IMAGE_ADAPTER", "fake"),
			ArkImage: ArkImage{
				APIKey:  valueOr(lookup, "COOKIES_ARK_IMAGE_API_KEY", ""),
				Model:   valueOr(lookup, "COOKIES_ARK_IMAGE_MODEL", ""),
				BaseURL: valueOr(lookup, "COOKIES_ARK_IMAGE_BASE_URL", ""),
			},
			OpenAIImage: OpenAIImage{
				APIKey:  valueOr(lookup, "COOKIES_OPENAI_IMAGE_API_KEY", ""),
				Model:   valueOr(lookup, "COOKIES_OPENAI_IMAGE_MODEL", ""),
				BaseURL: valueOr(lookup, "COOKIES_OPENAI_IMAGE_BASE_URL", ""),
			},
		},
	}

	identityValues := map[string]string{
		"organization_id": valueOr(lookup, "COOKIES_LOCAL_ORGANIZATION_ID", ""),
		"principal_kind":  valueOr(lookup, "COOKIES_LOCAL_PRINCIPAL_KIND", ""),
		"principal_id":    valueOr(lookup, "COOKIES_LOCAL_PRINCIPAL_ID", ""),
		"project_id":      valueOr(lookup, "COOKIES_LOCAL_PROJECT_ID", ""),
		"scopes":          valueOr(lookup, "COOKIES_LOCAL_SCOPES", ""),
	}
	if anyValue(identityValues) {
		config.LocalIdentity = &LocalIdentity{
			OrganizationID: identityValues["organization_id"],
			PrincipalKind:  identityValues["principal_kind"],
			PrincipalID:    identityValues["principal_id"],
			ProjectID:      identityValues["project_id"],
			Scopes:         splitCSV(identityValues["scopes"]),
		}
	}

	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c Config) Validate() error {
	switch c.Environment {
	case EnvironmentLocal, EnvironmentTest, EnvironmentStaging, EnvironmentProduction:
	default:
		return fmt.Errorf("COOKIES_ENV must be one of local, test, staging, production")
	}
	if strings.TrimSpace(c.HTTPAddr) == "" {
		return fmt.Errorf("COOKIES_HTTP_ADDR must not be empty")
	}
	if strings.TrimSpace(c.MySQL.DSN) == "" {
		return fmt.Errorf("COOKIES_MYSQL_DSN must not be empty")
	}
	if c.MySQL.MaxOpenConns < 1 || c.MySQL.MaxIdleConns < 0 || c.MySQL.MaxIdleConns > c.MySQL.MaxOpenConns {
		return fmt.Errorf("MySQL connection pool limits are invalid")
	}
	if c.ObjectStorage.Provider != "memory" && c.ObjectStorage.Provider != "tos" {
		return fmt.Errorf("COOKIES_BLOB_PROVIDER must be memory or tos")
	}
	if strings.TrimSpace(c.ObjectStorage.QuarantineBucket) == "" || strings.TrimSpace(c.ObjectStorage.AssetsBucket) == "" || c.ObjectStorage.QuarantineBucket == c.ObjectStorage.AssetsBucket {
		return fmt.Errorf("object storage requires distinct quarantine and assets buckets")
	}
	if c.ObjectStorage.Provider == "tos" && (c.ObjectStorage.Endpoint == "" || c.ObjectStorage.Region == "" || c.ObjectStorage.AccessKey == "" || c.ObjectStorage.SecretKey == "") {
		return fmt.Errorf("TOS storage requires endpoint, region, access key, and secret key")
	}
	if c.Scanner.Mode != "noop" && c.Scanner.Mode != "clamav" {
		return fmt.Errorf("COOKIES_SCANNER_MODE must be noop or clamav")
	}
	if c.Scanner.Mode == "clamav" && c.Scanner.Address == "" {
		return fmt.Errorf("ClamAV scanner requires COOKIES_CLAMAV_ADDRESS")
	}
	if c.Provider.ImageAdapter != "fake" && c.Provider.ImageAdapter != "ark_image" && c.Provider.ImageAdapter != "openai_image" {
		return fmt.Errorf("COOKIES_PROVIDER_IMAGE_ADAPTER must be fake, ark_image, or openai_image")
	}
	if c.Provider.ImageAdapter == "ark_image" && (c.Environment != EnvironmentLocal || strings.TrimSpace(c.Provider.ArkImage.APIKey) == "" || strings.TrimSpace(c.Provider.ArkImage.Model) == "") {
		return fmt.Errorf("ark_image is local-only and requires COOKIES_ARK_IMAGE_API_KEY and COOKIES_ARK_IMAGE_MODEL")
	}
	if c.Provider.ImageAdapter == "openai_image" && (c.Environment != EnvironmentLocal || strings.TrimSpace(c.Provider.OpenAIImage.APIKey) == "" || strings.TrimSpace(c.Provider.OpenAIImage.Model) == "" || strings.TrimSpace(c.Provider.OpenAIImage.BaseURL) == "") {
		return fmt.Errorf("openai_image is local-only and requires COOKIES_OPENAI_IMAGE_API_KEY, COOKIES_OPENAI_IMAGE_MODEL, and COOKIES_OPENAI_IMAGE_BASE_URL")
	}
	if c.Environment == EnvironmentProduction {
		if c.ObjectStorage.Provider != "tos" {
			return fmt.Errorf("production requires TOS object storage")
		}
		if c.Scanner.Mode != "clamav" {
			return fmt.Errorf("production requires ClamAV content scanning")
		}
	}
	if c.LocalIdentity == nil {
		return nil
	}
	if c.Environment != EnvironmentLocal {
		return fmt.Errorf("local identity is only permitted when COOKIES_ENV=local")
	}
	if strings.TrimSpace(c.LocalIdentity.OrganizationID) == "" ||
		strings.TrimSpace(c.LocalIdentity.PrincipalKind) == "" ||
		strings.TrimSpace(c.LocalIdentity.PrincipalID) == "" || strings.TrimSpace(c.LocalIdentity.ProjectID) == "" {
		return fmt.Errorf("local identity requires organization, principal kind, principal ID, and project ID")
	}
	return nil
}

func valueOr(lookup func(string) (string, bool), key, fallback string) string {
	if value, ok := lookup(key); ok {
		return strings.TrimSpace(value)
	}
	return fallback
}

func anyValue(values map[string]string) bool {
	for _, value := range values {
		if value != "" {
			return true
		}
	}
	return false
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func intValueOr(lookup func(string) (string, bool), key string, fallback int) int {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	var result int
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &result); err != nil {
		return -1
	}
	return result
}
