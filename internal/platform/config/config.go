// Package config owns process configuration validation. Configuration values
// are injected through the environment; secrets are intentionally not read
// from files in this bootstrap.
package config

import (
	"fmt"
	"os"
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
	LocalIdentity *LocalIdentity
}

type ObjectStorage struct {
	Provider         string
	FilesystemRoot   string
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

// MySQL contains only connection-pool configuration. No business module owns
// a second pool; modules receive this shared dependency instead.
type MySQL struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
}

func Load() (Config, error) {
	return FromLookup(os.LookupEnv)
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
			Provider:       valueOr(lookup, "COOKIES_BLOB_PROVIDER", "filesystem"),
			FilesystemRoot: valueOr(lookup, "COOKIES_FILESYSTEM_BLOB_ROOT", ".data/blobs"),
			Endpoint:       valueOr(lookup, "COOKIES_TOS_ENDPOINT", ""), Region: valueOr(lookup, "COOKIES_TOS_REGION", ""),
			AccessKey: valueOr(lookup, "COOKIES_TOS_ACCESS_KEY", ""), SecretKey: valueOr(lookup, "COOKIES_TOS_SECRET_KEY", ""),
			SecurityToken:    valueOr(lookup, "COOKIES_TOS_SECURITY_TOKEN", ""),
			QuarantineBucket: valueOr(lookup, "COOKIES_TOS_QUARANTINE_BUCKET", "cookies-quarantine"),
			AssetsBucket:     valueOr(lookup, "COOKIES_TOS_ASSETS_BUCKET", "cookies-assets"),
		},
		Scanner: Scanner{Mode: valueOr(lookup, "COOKIES_SCANNER_MODE", "noop"), Address: valueOr(lookup, "COOKIES_CLAMAV_ADDRESS", "")},
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
	if c.ObjectStorage.Provider != "memory" && c.ObjectStorage.Provider != "filesystem" && c.ObjectStorage.Provider != "tos" {
		return fmt.Errorf("COOKIES_BLOB_PROVIDER must be memory, filesystem, or tos")
	}
	if strings.TrimSpace(c.ObjectStorage.QuarantineBucket) == "" || strings.TrimSpace(c.ObjectStorage.AssetsBucket) == "" || c.ObjectStorage.QuarantineBucket == c.ObjectStorage.AssetsBucket {
		return fmt.Errorf("object storage requires distinct quarantine and assets buckets")
	}
	if c.ObjectStorage.Provider == "tos" && (c.ObjectStorage.Endpoint == "" || c.ObjectStorage.Region == "" || c.ObjectStorage.AccessKey == "" || c.ObjectStorage.SecretKey == "") {
		return fmt.Errorf("TOS storage requires endpoint, region, access key, and secret key")
	}
	if c.ObjectStorage.Provider == "filesystem" && strings.TrimSpace(c.ObjectStorage.FilesystemRoot) == "" {
		return fmt.Errorf("filesystem storage requires COOKIES_FILESYSTEM_BLOB_ROOT")
	}
	if c.Scanner.Mode != "noop" && c.Scanner.Mode != "clamav" {
		return fmt.Errorf("COOKIES_SCANNER_MODE must be noop or clamav")
	}
	if c.Scanner.Mode == "clamav" && c.Scanner.Address == "" {
		return fmt.Errorf("ClamAV scanner requires COOKIES_CLAMAV_ADDRESS")
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
