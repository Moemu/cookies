// Package config owns process configuration validation. Configuration comes
// from the process environment; local development may use an ignored .env as
// a fallback, while deployed environments never read that file.
package config

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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
	Auth          Auth
	ObjectStorage ObjectStorage
	Scanner       Scanner
	Media         Media
	Provider      Provider
	Creative      Creative
	Strategy      Strategy
	Research      Research
	LocalIdentity *LocalIdentity
}

type Auth struct {
	PasswordEnabled bool
	Username        string
	Password        string
	SessionHours    int
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

// Media configures optional local media executables. Empty executable paths
// keep non-video capabilities available while video probing/rendering reports
// an explicit capability error at the operation boundary.
type Media struct {
	FFmpegPath    string
	FFprobePath   string
	VideoWorkRoot string
}

// Strategy controls gradual rollout independently from the Creative system.
// Package-to-Creative permits only the explicit package-to-Intake handoff;
// approval never creates a Creative task implicitly.
type Strategy struct {
	Enabled                     bool
	V2Enabled                   bool
	RealProviderEnabled         bool
	ApproveEnabled              bool
	PackageToCreativeEnabled    bool
	CreativeTaskPlanningEnabled bool
	TextModelAlias              string
	DeepReviewModelAlias        string
	PromptVersion               string
	ConversationPromptVersion   string
	RevisePromptVersion         string
	ReviewPromptVersion         string
	RepairPromptVersion         string
	CreativeTaskPromptVersion   string
	CriticEnabled               bool
	ContextSelectionEnabled     bool
	OrganizationAllowlist       []string
}

type Creative struct {
	DirectionPlanningEnabled       bool
	DirectionPlannerModelAlias     string
	ShortDramaModelPlannerEnabled  bool
	ShortDramaPlannerModelAlias    string
	GamePrerollModelPlannerEnabled bool
	GamePrerollPlannerModelAlias   string
	BrandFilmModelPlannerEnabled   bool
	BrandFilmPlannerModelAlias     string
	AINativeMaxActiveUnits         int
}

// Research configures backend-owned web research. MCP fields are retained only
// for decoding older local configuration and are not used by the API process.
type Research struct {
	SeedEnabled        bool
	SeedModelAlias     string
	MaxConcurrent      int
	TikaEnabled        bool
	TikaBaseURL        string
	TikaVersion        string
	TikaTimeoutSeconds int
	TikaMaxOutputBytes int
	MCPStdioCommand    string
	MCPStdioArgs       []string
	MCPToolName        string
	MCPProtocolVersion string
	MCPEnvAllowlist    []string
	TimeoutSeconds     int
	MaxOutputBytes     int
}

// Provider contains only local composition choices. Credentials are read from
// the process environment (or ignored local .env), never from project data.
type Provider struct {
	ImageAdapter      string
	VideoAdapter      string
	TextAdapter       string
	AudioAdapter      string
	SpeechAdapter     string
	MasterKey         string
	MasterKeyVersion  string
	OutputBucket      string
	AllowInsecureHTTP bool
	ArkImage          ArkImage
	ArkVideo          ArkVideo
	ArkText           ArkText
	OpenAIImage       OpenAIImage
	VolcengineASR     VolcengineASR
	VolcengineSpeech  VolcengineSpeech
}

type ArkImage struct {
	APIKey  string
	Model   string
	BaseURL string
}

type ArkText struct {
	APIKey  string
	Model   string
	BaseURL string
}

type ArkVideo struct {
	APIKey  string
	Model   string
	BaseURL string
}

type OpenAIImage struct {
	APIKey  string
	Model   string
	BaseURL string
}

// VolcengineASR is the local-only preconfiguration for the recording-file
// recognition capability. The actual audio.transcribe execution adapter is
// introduced with the Creative Phase 2 runtime, so keeping this separate from
// Ark text/video prevents one credential from being used for the wrong API.
type VolcengineASR struct {
	Endpoint    string
	AuthMode    string
	AppID       string
	AccessToken string
	APIKey      string
	ResourceID  string
	Model       string
}

type VolcengineSpeech struct {
	Endpoint     string
	APIKey       string
	ResourceID   string
	DefaultVoice string
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
		if lineNumber == 1 {
			line = strings.TrimPrefix(line, "\uFEFF")
		}
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
	strategyEnabled, err := strictBoolValueOr(lookup, "COOKIES_STRATEGY_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	strategyRealProviderEnabled, err := strictBoolValueOr(lookup, "COOKIES_STRATEGY_REAL_PROVIDER_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	strategyApproveEnabled, err := strictBoolValueOr(lookup, "COOKIES_STRATEGY_APPROVE_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	strategyPackageToCreativeEnabled, err := strictBoolValueOr(lookup, "COOKIES_STRATEGY_PACKAGE_TO_CREATIVE_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	strategyCreativeTaskPlanningEnabled, err := strictBoolValueOr(
		lookup,
		"COOKIES_STRATEGY_CREATIVE_TASK_PLANNING_ENABLED",
		environment == EnvironmentLocal || environment == EnvironmentTest,
	)
	if err != nil {
		return Config{}, err
	}
	strategyCriticEnabled, err := strictBoolValueOr(lookup, "COOKIES_STRATEGY_CRITIC_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	strategyContextSelectionEnabled, err := strictBoolValueOr(
		lookup,
		"COOKIES_STRATEGY_CONTEXT_SELECTION_ENABLED",
		environment == EnvironmentLocal || environment == EnvironmentTest,
	)
	if err != nil {
		return Config{}, err
	}
	passwordAuthEnabled, err := strictBoolValueOr(
		lookup,
		"COOKIES_PASSWORD_AUTH_ENABLED",
		environment == EnvironmentLocal || environment == EnvironmentTest,
	)
	if err != nil {
		return Config{}, err
	}
	strategyV2Enabled, err := strictBoolValueOr(lookup, "COOKIES_STRATEGY_V2_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	gamePrerollModelPlannerEnabled, err := strictBoolValueOr(
		lookup,
		"COOKIES_CREATIVE_GAME_PREROLL_MODEL_PLANNER_ENABLED",
		true,
	)
	if err != nil {
		return Config{}, err
	}
	shortDramaModelPlannerEnabled, err := strictBoolValueOr(
		lookup,
		"COOKIES_CREATIVE_SHORT_DRAMA_MODEL_PLANNER_ENABLED",
		true,
	)
	if err != nil {
		return Config{}, err
	}
	brandFilmModelPlannerEnabled, err := strictBoolValueOr(
		lookup,
		"COOKIES_CREATIVE_BRAND_FILM_MODEL_PLANNER_ENABLED",
		true,
	)
	if err != nil {
		return Config{}, err
	}
	directionPlanningEnabled, err := strictBoolValueOr(
		lookup,
		"COOKIES_CREATIVE_DIRECTION_PLANNING_ENABLED",
		false,
	)
	if err != nil {
		return Config{}, err
	}
	researchSeedEnabled, err := strictBoolValueOr(lookup, "COOKIES_RESEARCH_SEED_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	researchTikaEnabled, err := strictBoolValueOr(lookup, "COOKIES_RESEARCH_TIKA_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	generatePromptDefault := "strategy.generate.v2"
	conversationPromptDefault := "strategy.conversation.v3"
	revisePromptDefault := "strategy.revise.v2"
	reviewPromptDefault := "strategy.review.deep.v1"
	repairPromptDefault := "strategy.repair.v1"
	if environment == EnvironmentLocal || environment == EnvironmentTest {
		generatePromptDefault = "strategy.generate.v3"
		conversationPromptDefault = "strategy.conversation.v5"
		revisePromptDefault = "strategy.revise.v3"
		reviewPromptDefault = "strategy.review.deep.v2"
		repairPromptDefault = "strategy.repair.v2"
	}
	config := Config{
		Environment: environment,
		HTTPAddr:    valueOr(lookup, "COOKIES_HTTP_ADDR", ":8080"),
		MySQL: MySQL{
			DSN:          valueOr(lookup, "COOKIES_MYSQL_DSN", "cookies:cookies_local_development_only@tcp(127.0.0.1:3307)/cookies?parseTime=true&multiStatements=true"),
			MaxOpenConns: intValueOr(lookup, "COOKIES_MYSQL_MAX_OPEN_CONNS", 10),
			MaxIdleConns: intValueOr(lookup, "COOKIES_MYSQL_MAX_IDLE_CONNS", 5),
		},
		Auth: Auth{
			PasswordEnabled: passwordAuthEnabled,
			Username:        valueOr(lookup, "COOKIES_ADMIN_USERNAME", "Admin"),
			Password:        valueOr(lookup, "COOKIES_ADMIN_PASSWORD", "123456"),
			SessionHours:    intValueOr(lookup, "COOKIES_SESSION_HOURS", 8),
		},
		ObjectStorage: ObjectStorage{
			Provider:         valueOrCompatibility(lookup, "COOKIES_BLOB_PROVIDER", "OBJECT_STORAGE_PROVIDER", "filesystem"),
			FilesystemRoot:   valueOr(lookup, "COOKIES_FILESYSTEM_BLOB_ROOT", ".data/blobs"),
			Endpoint:         valueOrCompatibility(lookup, "COOKIES_TOS_ENDPOINT", "OBJECT_STORAGE_ENDPOINT", ""),
			Region:           valueOrCompatibility(lookup, "COOKIES_TOS_REGION", "OBJECT_STORAGE_REGION", ""),
			AccessKey:        valueOrCompatibility(lookup, "COOKIES_TOS_ACCESS_KEY", "OBJECT_STORAGE_ACCESS_KEY", "OBJECT_STORAGE_ACCESS_KEY_ID", ""),
			SecretKey:        valueOrCompatibility(lookup, "COOKIES_TOS_SECRET_KEY", "OBJECT_STORAGE_SECRET_KEY", "OBJECT_STORAGE_ACCESS_KEY_SECRET", ""),
			SecurityToken:    valueOrCompatibility(lookup, "COOKIES_TOS_SECURITY_TOKEN", "OBJECT_STORAGE_SECURITY_TOKEN", ""),
			QuarantineBucket: valueOrCompatibility(lookup, "COOKIES_TOS_QUARANTINE_BUCKET", "OBJECT_STORAGE_QUARANTINE_BUCKET", "cookies-quarantine"),
			AssetsBucket:     valueOrCompatibility(lookup, "COOKIES_TOS_ASSETS_BUCKET", "OBJECT_STORAGE_ASSETS_BUCKET", "OBJECT_STORAGE_BUCKET_NAME", "cookies-assets"),
		},
		Scanner: Scanner{Mode: valueOr(lookup, "COOKIES_SCANNER_MODE", "noop"), Address: valueOr(lookup, "COOKIES_CLAMAV_ADDRESS", "")},
		Media: Media{
			FFmpegPath:    valueOr(lookup, "COOKIES_FFMPEG_PATH", ""),
			FFprobePath:   valueOr(lookup, "COOKIES_FFPROBE_PATH", ""),
			VideoWorkRoot: valueOr(lookup, "COOKIES_VIDEO_WORK_ROOT", ".data/video-work"),
		},
		Creative: Creative{
			DirectionPlanningEnabled:       directionPlanningEnabled,
			DirectionPlannerModelAlias:     valueOr(lookup, "COOKIES_CREATIVE_DIRECTION_PLANNER_MODEL_ALIAS", "cookies.text.standard"),
			ShortDramaModelPlannerEnabled:  shortDramaModelPlannerEnabled,
			ShortDramaPlannerModelAlias:    valueOr(lookup, "COOKIES_CREATIVE_SHORT_DRAMA_PLANNER_MODEL_ALIAS", "cookies.text.standard"),
			GamePrerollModelPlannerEnabled: gamePrerollModelPlannerEnabled,
			GamePrerollPlannerModelAlias:   valueOr(lookup, "COOKIES_CREATIVE_GAME_PREROLL_PLANNER_MODEL_ALIAS", "cookies.text.standard"),
			BrandFilmModelPlannerEnabled:   brandFilmModelPlannerEnabled,
			BrandFilmPlannerModelAlias:     valueOr(lookup, "COOKIES_CREATIVE_BRAND_FILM_PLANNER_MODEL_ALIAS", "cookies.text.standard"),
			AINativeMaxActiveUnits:         intValueOr(lookup, "COOKIES_AI_AD_MAX_ACTIVE_UNITS", 2),
		},
		Strategy: Strategy{
			Enabled:                     strategyEnabled,
			V2Enabled:                   strategyV2Enabled,
			RealProviderEnabled:         strategyRealProviderEnabled,
			ApproveEnabled:              strategyApproveEnabled,
			PackageToCreativeEnabled:    strategyPackageToCreativeEnabled,
			CreativeTaskPlanningEnabled: strategyCreativeTaskPlanningEnabled,
			TextModelAlias:              valueOr(lookup, "COOKIES_STRATEGY_TEXT_MODEL_ALIAS", "cookies.text.standard"),
			DeepReviewModelAlias:        valueOr(lookup, "COOKIES_STRATEGY_DEEP_REVIEW_MODEL_ALIAS", "cookies.text.deep_review"),
			PromptVersion:               valueOr(lookup, "COOKIES_STRATEGY_PROMPT_VERSION", generatePromptDefault),
			ConversationPromptVersion:   valueOr(lookup, "COOKIES_STRATEGY_CONVERSATION_PROMPT_VERSION", conversationPromptDefault),
			RevisePromptVersion:         valueOr(lookup, "COOKIES_STRATEGY_REVISE_PROMPT_VERSION", revisePromptDefault),
			ReviewPromptVersion:         valueOr(lookup, "COOKIES_STRATEGY_REVIEW_PROMPT_VERSION", reviewPromptDefault),
			RepairPromptVersion:         valueOr(lookup, "COOKIES_STRATEGY_REPAIR_PROMPT_VERSION", repairPromptDefault),
			CreativeTaskPromptVersion:   valueOr(lookup, "COOKIES_STRATEGY_CREATIVE_TASK_PROMPT_VERSION", "strategy.creative_task.generate.v2"),
			CriticEnabled:               strategyCriticEnabled,
			ContextSelectionEnabled:     strategyContextSelectionEnabled,
			OrganizationAllowlist:       splitCSV(valueOr(lookup, "COOKIES_STRATEGY_ORGANIZATION_ALLOWLIST", "")),
		},
		Research: Research{
			SeedEnabled:        researchSeedEnabled,
			SeedModelAlias:     valueOr(lookup, "COOKIES_RESEARCH_SEED_MODEL_ALIAS", "cookies.research.web.standard"),
			MaxConcurrent:      intValueOr(lookup, "COOKIES_RESEARCH_MAX_CONCURRENT", 3),
			TikaEnabled:        researchTikaEnabled,
			TikaBaseURL:        valueOr(lookup, "COOKIES_RESEARCH_TIKA_BASE_URL", "http://127.0.0.1:9998"),
			TikaVersion:        valueOr(lookup, "COOKIES_RESEARCH_TIKA_VERSION", "3.2.3.0"),
			TikaTimeoutSeconds: intValueOr(lookup, "COOKIES_RESEARCH_TIKA_TIMEOUT_SECONDS", 120),
			TikaMaxOutputBytes: intValueOr(lookup, "COOKIES_RESEARCH_TIKA_MAX_OUTPUT_BYTES", 20*1024*1024),
			MCPStdioCommand:    strings.TrimSpace(valueOr(lookup, "COOKIES_RESEARCH_MCP_STDIO_COMMAND", "")),
			MCPToolName:        valueOr(lookup, "COOKIES_RESEARCH_MCP_TOOL_NAME", "research"),
			MCPProtocolVersion: valueOr(lookup, "COOKIES_RESEARCH_MCP_PROTOCOL_VERSION", "2025-11-25"),
			MCPEnvAllowlist:    splitCSV(valueOr(lookup, "COOKIES_RESEARCH_MCP_ENV_ALLOWLIST", "PATH,PATHEXT,SystemRoot,TEMP,TMP,ComSpec")),
			TimeoutSeconds:     intValueOr(lookup, "COOKIES_RESEARCH_TIMEOUT_SECONDS", 120),
			MaxOutputBytes:     intValueOr(lookup, "COOKIES_RESEARCH_MAX_OUTPUT_BYTES", 4*1024*1024),
		},
		Provider: Provider{
			ImageAdapter:      valueOr(lookup, "COOKIES_PROVIDER_IMAGE_ADAPTER", "fake"),
			VideoAdapter:      valueOr(lookup, "COOKIES_PROVIDER_VIDEO_ADAPTER", "fake"),
			TextAdapter:       valueOr(lookup, "COOKIES_PROVIDER_TEXT_ADAPTER", "fake"),
			AudioAdapter:      valueOr(lookup, "COOKIES_PROVIDER_AUDIO_ADAPTER", "fake"),
			SpeechAdapter:     valueOr(lookup, "COOKIES_PROVIDER_SPEECH_ADAPTER", "fake"),
			MasterKey:         valueOr(lookup, "COOKIES_PROVIDER_MASTER_KEY", ""),
			MasterKeyVersion:  valueOr(lookup, "COOKIES_PROVIDER_MASTER_KEY_VERSION", "v1"),
			OutputBucket:      valueOr(lookup, "COOKIES_PROVIDER_OUTPUT_BUCKET", "cookies-provider-output"),
			AllowInsecureHTTP: boolValueOr(lookup, "COOKIES_PROVIDER_ALLOW_INSECURE_HTTP", false),
			ArkImage: ArkImage{
				APIKey:  valueOr(lookup, "COOKIES_ARK_IMAGE_API_KEY", ""),
				Model:   valueOr(lookup, "COOKIES_ARK_IMAGE_MODEL", ""),
				BaseURL: valueOr(lookup, "COOKIES_ARK_IMAGE_BASE_URL", ""),
			},
			ArkVideo: ArkVideo{
				APIKey:  valueOr(lookup, "COOKIES_ARK_VIDEO_API_KEY", ""),
				Model:   valueOr(lookup, "COOKIES_ARK_VIDEO_MODEL", ""),
				BaseURL: valueOr(lookup, "COOKIES_ARK_VIDEO_BASE_URL", ""),
			},
			ArkText: ArkText{
				APIKey:  valueOr(lookup, "COOKIES_ARK_TEXT_API_KEY", ""),
				Model:   valueOr(lookup, "COOKIES_ARK_TEXT_MODEL", ""),
				BaseURL: valueOr(lookup, "COOKIES_ARK_TEXT_BASE_URL", ""),
			},
			OpenAIImage: OpenAIImage{
				APIKey:  valueOr(lookup, "COOKIES_OPENAI_IMAGE_API_KEY", ""),
				Model:   valueOr(lookup, "COOKIES_OPENAI_IMAGE_MODEL", ""),
				BaseURL: valueOr(lookup, "COOKIES_OPENAI_IMAGE_BASE_URL", ""),
			},
			VolcengineASR: VolcengineASR{
				Endpoint:    valueOr(lookup, "COOKIES_VOLCENGINE_ASR_ENDPOINT", "https://openspeech.bytedance.com/api/v3/auc/bigmodel/recognize/flash"),
				AuthMode:    valueOr(lookup, "COOKIES_VOLCENGINE_ASR_AUTH_MODE", "legacy"),
				AppID:       valueOr(lookup, "COOKIES_VOLCENGINE_ASR_APP_ID", ""),
				AccessToken: valueOr(lookup, "COOKIES_VOLCENGINE_ASR_ACCESS_TOKEN", ""),
				APIKey:      valueOr(lookup, "COOKIES_VOLCENGINE_ASR_API_KEY", ""),
				ResourceID:  valueOr(lookup, "COOKIES_VOLCENGINE_ASR_RESOURCE_ID", "volc.bigasr.auc_turbo"),
				Model:       valueOr(lookup, "COOKIES_VOLCENGINE_ASR_MODEL", "bigmodel"),
			},
			VolcengineSpeech: VolcengineSpeech{
				Endpoint:     valueOr(lookup, "COOKIES_VOLCENGINE_SPEECH_ENDPOINT", "https://openspeech.bytedance.com/api/v3/tts/unidirectional"),
				APIKey:       valueOr(lookup, "COOKIES_VOLCENGINE_SPEECH_API_KEY", ""),
				ResourceID:   valueOr(lookup, "COOKIES_VOLCENGINE_SPEECH_RESOURCE_ID", "seed-tts-2.0"),
				DefaultVoice: valueOr(lookup, "COOKIES_VOLCENGINE_SPEECH_DEFAULT_VOICE", ""),
			},
		},
	}
	if raw := strings.TrimSpace(valueOr(lookup, "COOKIES_RESEARCH_MCP_STDIO_ARGS_JSON", "")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &config.Research.MCPStdioArgs); err != nil {
			return Config{}, fmt.Errorf("COOKIES_RESEARCH_MCP_STDIO_ARGS_JSON must be a JSON string array: %w", err)
		}
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
	if c.Auth.PasswordEnabled && (strings.TrimSpace(c.Auth.Username) == "" ||
		strings.TrimSpace(c.Auth.Password) == "" || c.Auth.SessionHours < 1 || c.Auth.SessionHours > 168) {
		return fmt.Errorf("password authentication requires username, password, and session hours between 1 and 168")
	}
	if c.Auth.PasswordEnabled && c.Environment != EnvironmentLocal && c.Environment != EnvironmentTest &&
		strings.EqualFold(strings.TrimSpace(c.Auth.Username), "Admin") && c.Auth.Password == "123456" {
		return fmt.Errorf("default local administrator credentials are forbidden outside local and test")
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
	if strings.TrimSpace(c.Media.VideoWorkRoot) == "" {
		return fmt.Errorf("COOKIES_VIDEO_WORK_ROOT must not be empty")
	}
	if strings.TrimSpace(c.Creative.ShortDramaPlannerModelAlias) == "" {
		return fmt.Errorf("COOKIES_CREATIVE_SHORT_DRAMA_PLANNER_MODEL_ALIAS must not be empty")
	}
	if strings.TrimSpace(c.Creative.GamePrerollPlannerModelAlias) == "" {
		return fmt.Errorf("COOKIES_CREATIVE_GAME_PREROLL_PLANNER_MODEL_ALIAS must not be empty")
	}
	if strings.TrimSpace(c.Creative.BrandFilmPlannerModelAlias) == "" {
		return fmt.Errorf("Creative brand film planner model alias is required")
	}
	if c.Creative.AINativeMaxActiveUnits < 1 || c.Creative.AINativeMaxActiveUnits > 10 {
		return fmt.Errorf("COOKIES_AI_AD_MAX_ACTIVE_UNITS must be between 1 and 10")
	}
	if c.Research.TimeoutSeconds < 1 || c.Research.TimeoutSeconds > 600 {
		return fmt.Errorf("COOKIES_RESEARCH_TIMEOUT_SECONDS must be between 1 and 600")
	}
	if c.Research.MaxOutputBytes < 1024 || c.Research.MaxOutputBytes > 16*1024*1024 {
		return fmt.Errorf("COOKIES_RESEARCH_MAX_OUTPUT_BYTES must be between 1024 and 16777216")
	}
	if strings.TrimSpace(c.Research.SeedModelAlias) == "" {
		return fmt.Errorf("COOKIES_RESEARCH_SEED_MODEL_ALIAS must not be empty")
	}
	if c.Research.MaxConcurrent < 1 || c.Research.MaxConcurrent > 4 {
		return fmt.Errorf("COOKIES_RESEARCH_MAX_CONCURRENT must be between 1 and 4")
	}
	if c.Research.TikaTimeoutSeconds < 1 || c.Research.TikaTimeoutSeconds > 600 {
		return fmt.Errorf("COOKIES_RESEARCH_TIKA_TIMEOUT_SECONDS must be between 1 and 600")
	}
	if c.Research.TikaMaxOutputBytes < 1024 || c.Research.TikaMaxOutputBytes > 32*1024*1024 {
		return fmt.Errorf("COOKIES_RESEARCH_TIKA_MAX_OUTPUT_BYTES must be between 1024 and 33554432")
	}
	if c.Research.TikaEnabled {
		tikaURL, err := url.Parse(strings.TrimSpace(c.Research.TikaBaseURL))
		if err != nil || (tikaURL.Scheme != "http" && tikaURL.Scheme != "https") || tikaURL.Host == "" {
			return fmt.Errorf("COOKIES_RESEARCH_TIKA_BASE_URL must be an absolute HTTP(S) URL")
		}
		if strings.TrimSpace(c.Research.TikaVersion) == "" {
			return fmt.Errorf("COOKIES_RESEARCH_TIKA_VERSION must not be empty")
		}
	}
	if c.Research.MCPStdioCommand != "" &&
		(strings.TrimSpace(c.Research.MCPToolName) == "" || strings.TrimSpace(c.Research.MCPProtocolVersion) == "") {
		return fmt.Errorf("MCP stdio research requires a tool name and protocol version")
	}
	if c.Provider.ImageAdapter != "fake" && c.Provider.ImageAdapter != "ark_image" && c.Provider.ImageAdapter != "openai_image" && c.Provider.ImageAdapter != "adapter_gateway" {
		return fmt.Errorf("COOKIES_PROVIDER_IMAGE_ADAPTER must be fake, ark_image, openai_image, or adapter_gateway")
	}
	if c.Provider.TextAdapter != "fake" && c.Provider.TextAdapter != "adapter_gateway" && c.Provider.TextAdapter != "ark_text" {
		return fmt.Errorf("COOKIES_PROVIDER_TEXT_ADAPTER must be fake, adapter_gateway, or ark_text")
	}
	if c.Provider.VideoAdapter != "fake" && c.Provider.VideoAdapter != "ark_video" {
		return fmt.Errorf("COOKIES_PROVIDER_VIDEO_ADAPTER must be fake or ark_video")
	}
	if c.Provider.AudioAdapter != "fake" && c.Provider.AudioAdapter != "volcengine_asr" {
		return fmt.Errorf("COOKIES_PROVIDER_AUDIO_ADAPTER must be fake or volcengine_asr")
	}
	if c.Provider.SpeechAdapter != "fake" && c.Provider.SpeechAdapter != "volcengine_speech" && c.Provider.SpeechAdapter != "minimax_speech" {
		return fmt.Errorf("COOKIES_PROVIDER_SPEECH_ADAPTER must be fake, volcengine_speech, or minimax_speech")
	}
	if c.Strategy.RealProviderEnabled && c.Provider.TextAdapter != "adapter_gateway" && c.Provider.TextAdapter != "ark_text" {
		return fmt.Errorf("COOKIES_STRATEGY_REAL_PROVIDER_ENABLED requires a real text adapter")
	}
	if strings.TrimSpace(c.Strategy.TextModelAlias) == "" {
		return fmt.Errorf("COOKIES_STRATEGY_TEXT_MODEL_ALIAS must not be empty")
	}
	if strings.TrimSpace(c.Strategy.DeepReviewModelAlias) == "" {
		return fmt.Errorf("COOKIES_STRATEGY_DEEP_REVIEW_MODEL_ALIAS must not be empty")
	}
	if strings.TrimSpace(c.Strategy.PromptVersion) == "" {
		return fmt.Errorf("COOKIES_STRATEGY_PROMPT_VERSION must not be empty")
	}
	if !oneOf(c.Strategy.PromptVersion, "strategy.generate.v2", "strategy.generate.v3") {
		return fmt.Errorf("COOKIES_STRATEGY_PROMPT_VERSION is unsupported")
	}
	if strings.TrimSpace(c.Strategy.ConversationPromptVersion) == "" {
		return fmt.Errorf("COOKIES_STRATEGY_CONVERSATION_PROMPT_VERSION must not be empty")
	}
	if !oneOf(c.Strategy.ConversationPromptVersion, "strategy.conversation.v3", "strategy.conversation.v4", "strategy.conversation.v5") {
		return fmt.Errorf("COOKIES_STRATEGY_CONVERSATION_PROMPT_VERSION is unsupported")
	}
	if strings.TrimSpace(c.Strategy.RevisePromptVersion) == "" {
		return fmt.Errorf("COOKIES_STRATEGY_REVISE_PROMPT_VERSION must not be empty")
	}
	if !oneOf(c.Strategy.RevisePromptVersion, "strategy.revise.v2", "strategy.revise.v3") {
		return fmt.Errorf("COOKIES_STRATEGY_REVISE_PROMPT_VERSION is unsupported")
	}
	if strings.TrimSpace(c.Strategy.ReviewPromptVersion) == "" {
		return fmt.Errorf("COOKIES_STRATEGY_REVIEW_PROMPT_VERSION must not be empty")
	}
	if !oneOf(c.Strategy.ReviewPromptVersion, "strategy.review.deep.v1", "strategy.review.deep.v2") {
		return fmt.Errorf("COOKIES_STRATEGY_REVIEW_PROMPT_VERSION is unsupported")
	}
	if strings.TrimSpace(c.Strategy.RepairPromptVersion) == "" {
		return fmt.Errorf("COOKIES_STRATEGY_REPAIR_PROMPT_VERSION must not be empty")
	}
	if !oneOf(c.Strategy.RepairPromptVersion, "strategy.repair.v1", "strategy.repair.v2") {
		return fmt.Errorf("COOKIES_STRATEGY_REPAIR_PROMPT_VERSION is unsupported")
	}
	if c.Strategy.CreativeTaskPromptVersion != "strategy.creative_task.generate.v2" {
		return fmt.Errorf("COOKIES_STRATEGY_CREATIVE_TASK_PROMPT_VERSION is unsupported")
	}
	if c.Strategy.CriticEnabled && !c.Strategy.RealProviderEnabled {
		return fmt.Errorf("COOKIES_STRATEGY_CRITIC_ENABLED requires COOKIES_STRATEGY_REAL_PROVIDER_ENABLED=true")
	}
	if c.Provider.ImageAdapter == "ark_image" && (c.Environment != EnvironmentLocal || strings.TrimSpace(c.Provider.ArkImage.APIKey) == "" || strings.TrimSpace(c.Provider.ArkImage.Model) == "") {
		return fmt.Errorf("ark_image is local-only and requires COOKIES_ARK_IMAGE_API_KEY and COOKIES_ARK_IMAGE_MODEL")
	}
	if c.Provider.VideoAdapter == "ark_video" && c.Environment != EnvironmentLocal {
		return fmt.Errorf("ark_video is local-only in Phase 1")
	}
	if c.Provider.ImageAdapter == "openai_image" && (c.Environment != EnvironmentLocal || strings.TrimSpace(c.Provider.OpenAIImage.APIKey) == "" || strings.TrimSpace(c.Provider.OpenAIImage.Model) == "" || strings.TrimSpace(c.Provider.OpenAIImage.BaseURL) == "") {
		return fmt.Errorf("openai_image is local-only and requires COOKIES_OPENAI_IMAGE_API_KEY, COOKIES_OPENAI_IMAGE_MODEL, and COOKIES_OPENAI_IMAGE_BASE_URL")
	}
	if c.Provider.TextAdapter == "ark_text" && (c.Environment != EnvironmentLocal || strings.TrimSpace(c.Provider.ArkText.APIKey) == "" || strings.TrimSpace(c.Provider.ArkText.Model) == "") {
		return fmt.Errorf("ark_text is local-only and requires COOKIES_ARK_TEXT_API_KEY and COOKIES_ARK_TEXT_MODEL")
	}
	if c.Provider.AudioAdapter == "volcengine_asr" {
		if c.Environment != EnvironmentLocal {
			return fmt.Errorf("volcengine_asr is local-only until the audio.transcribe runtime is introduced")
		}
		endpoint, err := url.Parse(c.Provider.VolcengineASR.Endpoint)
		if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
			return fmt.Errorf("COOKIES_VOLCENGINE_ASR_ENDPOINT must be an absolute HTTPS URL")
		}
		if strings.TrimSpace(c.Provider.VolcengineASR.ResourceID) == "" || strings.TrimSpace(c.Provider.VolcengineASR.Model) == "" {
			return fmt.Errorf("volcengine_asr requires resource ID and model")
		}
		switch c.Provider.VolcengineASR.AuthMode {
		case "legacy":
			if strings.TrimSpace(c.Provider.VolcengineASR.AppID) == "" || strings.TrimSpace(c.Provider.VolcengineASR.AccessToken) == "" {
				return fmt.Errorf("legacy volcengine_asr requires COOKIES_VOLCENGINE_ASR_APP_ID and COOKIES_VOLCENGINE_ASR_ACCESS_TOKEN")
			}
		case "api_key":
			if strings.TrimSpace(c.Provider.VolcengineASR.APIKey) == "" {
				return fmt.Errorf("api_key volcengine_asr requires COOKIES_VOLCENGINE_ASR_API_KEY")
			}
		default:
			return fmt.Errorf("COOKIES_VOLCENGINE_ASR_AUTH_MODE must be legacy or api_key")
		}
	}
	if c.Provider.SpeechAdapter == "volcengine_speech" {
		endpoint, err := url.Parse(c.Provider.VolcengineSpeech.Endpoint)
		if err != nil || endpoint.Scheme != "https" || endpoint.Host == "" {
			return fmt.Errorf("COOKIES_VOLCENGINE_SPEECH_ENDPOINT must be an absolute HTTPS URL")
		}
		if strings.TrimSpace(c.Provider.VolcengineSpeech.APIKey) == "" || strings.TrimSpace(c.Provider.VolcengineSpeech.ResourceID) == "" || strings.TrimSpace(c.Provider.VolcengineSpeech.DefaultVoice) == "" {
			return fmt.Errorf("volcengine_speech requires API key, resource ID and default voice")
		}
	}
	usesGenerationBroker := c.Provider.ImageAdapter == "adapter_gateway" ||
		c.Provider.TextAdapter == "adapter_gateway" ||
		c.Provider.VideoAdapter == "ark_video" ||
		c.Provider.SpeechAdapter == "minimax_speech"
	usesCredentialBroker := usesGenerationBroker || c.Research.SeedEnabled
	if usesCredentialBroker && (strings.TrimSpace(c.Provider.MasterKey) == "" || strings.TrimSpace(c.Provider.MasterKeyVersion) == "") {
		return fmt.Errorf("configured Provider adapter requires COOKIES_PROVIDER_MASTER_KEY and COOKIES_PROVIDER_MASTER_KEY_VERSION")
	}
	if usesCredentialBroker {
		key, err := base64.StdEncoding.DecodeString(c.Provider.MasterKey)
		if err != nil || len(key) != 32 {
			return fmt.Errorf("COOKIES_PROVIDER_MASTER_KEY must be base64-encoded 32 bytes")
		}
		if usesGenerationBroker &&
			(strings.TrimSpace(c.Provider.OutputBucket) == "" ||
				c.Provider.OutputBucket == c.ObjectStorage.AssetsBucket ||
				c.Provider.OutputBucket == c.ObjectStorage.QuarantineBucket) {
			return fmt.Errorf("adapter_gateway requires a distinct COOKIES_PROVIDER_OUTPUT_BUCKET")
		}
		if c.Provider.AllowInsecureHTTP && c.Environment != EnvironmentLocal {
			return fmt.Errorf("COOKIES_PROVIDER_ALLOW_INSECURE_HTTP is permitted only when COOKIES_ENV=local")
		}
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

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if strings.TrimSpace(value) == candidate {
			return true
		}
	}
	return false
}

func valueOr(lookup func(string) (string, bool), key, fallback string) string {
	if value, ok := lookup(key); ok {
		return strings.TrimSpace(value)
	}
	return fallback
}

// valueOrCompatibility lets deployments migrate from generic object-storage
// names without changing the existing COOKIES_TOS_* configuration contract.
func valueOrCompatibility(lookup func(string) (string, bool), key string, compatibilityKeys ...string) string {
	if value, ok := lookup(key); ok {
		return strings.TrimSpace(value)
	}
	for _, compatibilityKey := range compatibilityKeys[:len(compatibilityKeys)-1] {
		if value, ok := lookup(compatibilityKey); ok {
			return strings.TrimSpace(value)
		}
	}
	return compatibilityKeys[len(compatibilityKeys)-1]
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

func boolValueOr(lookup func(string) (string, bool), key string, fallback bool) bool {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func strictBoolValueOr(lookup func(string) (string, bool), key string, fallback bool) (bool, error) {
	value, ok := lookup(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return parsed, nil
}
