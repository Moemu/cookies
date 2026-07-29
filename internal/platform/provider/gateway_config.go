package provider

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type TextResponseMode string

const (
	TextResponseJSONSchema TextResponseMode = "json_schema"
	TextResponseJSONObject TextResponseMode = "json_object"
	TextResponsePromptJSON TextResponseMode = "prompt_json"
)

// TextOutputTokenParameter records the upstream field used to cap output
// tokens. Most OpenAI-compatible endpoints use max_tokens, while some
// providers, including MiniMax, require max_completion_tokens.
type TextOutputTokenParameter string

const (
	TextOutputTokenParameterMaxTokens           TextOutputTokenParameter = "max_tokens"
	TextOutputTokenParameterMaxCompletionTokens TextOutputTokenParameter = "max_completion_tokens"
)

// GatewayRouteSnapshot is copied onto an invocation when it is created. Later route
// edits therefore cannot silently change the endpoint, model, or credential
// used by an already accepted image job or text skill run.
type GatewayRouteSnapshot struct {
	RouteID              string                   `json:"route_id"`
	RouteRevisionID      string                   `json:"route_revision_id"`
	ConnectionID         string                   `json:"connection_id"`
	ConnectionRevisionID string                   `json:"connection_revision_id"`
	BaseURL              string                   `json:"base_url"`
	UpstreamModel        string                   `json:"upstream_model"`
	CredentialID         string                   `json:"credential_id"`
	CredentialVersion    int64                    `json:"credential_version"`
	TimeoutSeconds       int                      `json:"timeout_seconds"`
	MaxResponseBytes     int64                    `json:"max_response_bytes"`
	TextResponseMode     TextResponseMode         `json:"text_response_mode,omitempty"`
	MaxOutputTokens      int                      `json:"max_output_tokens,omitempty"`
	OutputTokenParameter TextOutputTokenParameter `json:"output_token_parameter,omitempty"`
	Temperature          float64                  `json:"temperature,omitempty"`
	TemperatureSet       bool                     `json:"-"`
	ThinkingMode         string                   `json:"thinking_mode,omitempty"`
	ReasoningSplit       bool                     `json:"reasoning_split,omitempty"`
	VideoInputModes      []VideoInputMode         `json:"video_input_modes,omitempty"`
	VideoAudioPolicies   []VideoAudioPolicy       `json:"video_audio_policies,omitempty"`
}

func (s GatewayRouteSnapshot) Validate() error {
	return s.ValidateWithPolicy(false)
}

func (s GatewayRouteSnapshot) ValidateWithPolicy(allowInsecureHTTP bool) error {
	return s.validateWithLimits(allowInsecureHTTP, 600, 100<<20)
}

func (s GatewayRouteSnapshot) ValidateVideoWithPolicy(allowInsecureHTTP bool) error {
	if err := s.validateWithLimits(allowInsecureHTTP, 1800, 200<<20); err != nil {
		return err
	}
	if err := validateVideoInputModes(s.VideoInputModes); err != nil {
		return err
	}
	return validateVideoAudioPolicies(s.VideoAudioPolicies)
}

func (s GatewayRouteSnapshot) validateWithLimits(allowInsecureHTTP bool, maxTimeoutSeconds int, maxResponseBytes int64) error {
	if strings.TrimSpace(s.RouteID) == "" || strings.TrimSpace(s.RouteRevisionID) == "" ||
		strings.TrimSpace(s.ConnectionID) == "" || strings.TrimSpace(s.ConnectionRevisionID) == "" ||
		strings.TrimSpace(s.UpstreamModel) == "" || strings.TrimSpace(s.CredentialID) == "" ||
		s.CredentialVersion < 1 {
		return fmt.Errorf("adapter gateway route snapshot is incomplete")
	}
	parsed, err := url.Parse(s.BaseURL)
	validScheme := parsed.Scheme == "https" || (allowInsecureHTTP && parsed.Scheme == "http")
	if err != nil || !validScheme || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("adapter gateway base URL must use HTTPS (or explicitly allowed local HTTP) and contain no user info")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("adapter gateway base URL cannot contain a query or fragment")
	}
	if s.TimeoutSeconds < 1 || s.TimeoutSeconds > maxTimeoutSeconds {
		return fmt.Errorf("adapter gateway timeout must be between 1 and %d seconds", maxTimeoutSeconds)
	}
	if s.MaxResponseBytes < 1 || s.MaxResponseBytes > maxResponseBytes {
		return fmt.Errorf("adapter gateway response limit must be between 1 byte and %d bytes", maxResponseBytes)
	}
	return nil
}

func (s GatewayRouteSnapshot) ValidateTextWithPolicy(allowInsecureHTTP bool) error {
	if err := s.ValidateWithPolicy(allowInsecureHTTP); err != nil {
		return err
	}
	switch s.TextResponseMode {
	case TextResponseJSONSchema, TextResponseJSONObject, TextResponsePromptJSON:
	default:
		return fmt.Errorf("adapter gateway text response mode is invalid")
	}
	if s.MaxOutputTokens < 0 || s.MaxOutputTokens > 100_000 {
		return fmt.Errorf("adapter gateway max output tokens are invalid")
	}
	if s.MaxOutputTokens > 0 {
		switch s.OutputTokenParameter {
		case "", TextOutputTokenParameterMaxTokens, TextOutputTokenParameterMaxCompletionTokens:
		default:
			return fmt.Errorf("adapter gateway output token parameter is invalid")
		}
	}
	if s.Temperature < 0 || s.Temperature > 2 {
		return fmt.Errorf("adapter gateway temperature is invalid")
	}
	switch s.ThinkingMode {
	case "", "auto", "enabled", "disabled":
	default:
		return fmt.Errorf("adapter gateway thinking mode is invalid")
	}
	return nil
}

type ImageRouteResolver interface {
	ResolveImageRoute(context.Context, contract.OrganizationID, string) (ImageRouteSnapshot, error)
}

type TextRouteResolver interface {
	ResolveTextRoute(context.Context, contract.OrganizationID, string) (GatewayRouteSnapshot, error)
}

type VideoRouteResolver interface {
	ResolveVideoRoute(context.Context, contract.OrganizationID, string) (VideoRouteSnapshot, error)
}

// ImageRouteSnapshot is retained as a source-compatible alias for the
// existing durable ProviderJob JSON contract.
type ImageRouteSnapshot = GatewayRouteSnapshot

type GatewayCredentialResolver interface {
	ResolveGatewayCredential(context.Context, string, int64) (string, error)
}

// MySQLGatewayConfigStore resolves only enabled, immutable revisions. The
// active credential version is captured in the returned job snapshot.
type MySQLGatewayConfigStore struct {
	DB                *sql.DB
	Cipher            CredentialCipher
	AllowInsecureHTTP bool
}

func (s MySQLGatewayConfigStore) ResolveImageRoute(ctx context.Context, organizationID contract.OrganizationID, modelAlias string) (ImageRouteSnapshot, error) {
	return s.resolveRoute(ctx, organizationID, "image.generate", modelAlias, "adapter_gateway")
}

func (s MySQLGatewayConfigStore) ResolveTextRoute(ctx context.Context, organizationID contract.OrganizationID, modelAlias string) (GatewayRouteSnapshot, error) {
	return s.resolveRoute(ctx, organizationID, "text.generate", modelAlias, "adapter_gateway")
}

func (s MySQLGatewayConfigStore) ResolveVideoRoute(ctx context.Context, organizationID contract.OrganizationID, modelAlias string) (VideoRouteSnapshot, error) {
	return s.resolveRoute(ctx, organizationID, "video.generate", modelAlias, "ark")
}

func (s MySQLGatewayConfigStore) resolveRoute(ctx context.Context, organizationID contract.OrganizationID, capability, modelAlias, connectionType string) (ImageRouteSnapshot, error) {
	if s.DB == nil {
		return ImageRouteSnapshot{}, fmt.Errorf("MySQL database is required")
	}
	var snapshot ImageRouteSnapshot
	var constraintsJSON []byte
	err := s.DB.QueryRowContext(ctx, `SELECT
			r.id, rr.id, c.id, cr.id, cr.base_url, rr.upstream_model,
			pc.id, pc.credential_version, cr.timeout_seconds, cr.max_response_bytes,
			COALESCE(rr.constraints_json, JSON_OBJECT())
		FROM provider_model_routes r
		JOIN provider_model_route_revisions rr ON rr.id = r.current_revision_id AND rr.route_id = r.id
		JOIN provider_connections c ON c.id = rr.connection_id AND c.status = 'enabled' AND c.connection_type = ?
		JOIN provider_connection_revisions cr ON cr.id = rr.connection_revision_id AND cr.connection_id = c.id
		JOIN provider_credentials pc ON pc.connection_id = c.id AND pc.status = 'active'
		WHERE r.capability = ? AND r.model_alias = ? AND r.status = 'enabled'
			AND (r.organization_id = ? OR r.organization_id IS NULL)
			AND pc.active_from <= UTC_TIMESTAMP(6)
			AND (pc.active_until IS NULL OR pc.active_until > UTC_TIMESTAMP(6))
		ORDER BY (r.organization_id IS NOT NULL) DESC, pc.credential_version DESC
		LIMIT 1`,
		connectionType, capability, modelAlias, organizationID,
	).Scan(
		&snapshot.RouteID, &snapshot.RouteRevisionID, &snapshot.ConnectionID, &snapshot.ConnectionRevisionID,
		&snapshot.BaseURL, &snapshot.UpstreamModel, &snapshot.CredentialID, &snapshot.CredentialVersion,
		&snapshot.TimeoutSeconds, &snapshot.MaxResponseBytes, &constraintsJSON,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ImageRouteSnapshot{}, fmt.Errorf("no enabled %s %s route for model alias %q", connectionType, capability, modelAlias)
	}
	if err != nil {
		return ImageRouteSnapshot{}, err
	}
	if capability == "text.generate" {
		if err := applyTextRouteConstraints(&snapshot, constraintsJSON); err != nil {
			return ImageRouteSnapshot{}, fmt.Errorf("invalid adapter gateway route %q constraints: %w", modelAlias, err)
		}
	} else if capability == "video.generate" {
		if err := applyVideoRouteConstraints(&snapshot, constraintsJSON); err != nil {
			return ImageRouteSnapshot{}, fmt.Errorf("invalid adapter gateway route %q constraints: %w", modelAlias, err)
		}
	}
	validate := snapshot.ValidateWithPolicy
	if capability == "text.generate" {
		validate = snapshot.ValidateTextWithPolicy
	} else if capability == "video.generate" {
		validate = snapshot.ValidateVideoWithPolicy
	}
	if err := validate(s.AllowInsecureHTTP); err != nil {
		return ImageRouteSnapshot{}, fmt.Errorf("invalid adapter gateway route %q: %w", modelAlias, err)
	}
	return snapshot, nil
}

func applyVideoRouteConstraints(snapshot *GatewayRouteSnapshot, raw json.RawMessage) error {
	if snapshot == nil {
		return fmt.Errorf("route snapshot is required")
	}
	var constraints struct {
		InputModes    []VideoInputMode   `json:"video_input_modes"`
		AudioPolicies []VideoAudioPolicy `json:"video_audio_policies"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &constraints); err != nil {
			return err
		}
	}
	if err := validateVideoInputModes(constraints.InputModes); err != nil {
		return err
	}
	if err := validateVideoAudioPolicies(constraints.AudioPolicies); err != nil {
		return err
	}
	snapshot.VideoInputModes = append([]VideoInputMode(nil), constraints.InputModes...)
	snapshot.VideoAudioPolicies = append([]VideoAudioPolicy(nil), constraints.AudioPolicies...)
	return nil
}

func validateVideoInputModes(values []VideoInputMode) error {
	seen := make(map[VideoInputMode]struct{}, len(values))
	for _, value := range values {
		switch value {
		case VideoInputTextOnly, VideoInputReferenceImage, VideoInputFirstLastFrame:
		default:
			return fmt.Errorf("video input mode %q is invalid", value)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("video input mode %q is duplicated", value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateVideoAudioPolicies(values []VideoAudioPolicy) error {
	seen := make(map[VideoAudioPolicy]struct{}, len(values))
	for _, value := range values {
		switch value {
		case VideoAudioSilent, VideoAudioGenerated:
		default:
			return fmt.Errorf("video audio policy %q is invalid", value)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("video audio policy %q is duplicated", value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func applyTextRouteConstraints(snapshot *GatewayRouteSnapshot, raw json.RawMessage) error {
	if snapshot == nil {
		return fmt.Errorf("route snapshot is required")
	}
	var constraints struct {
		ResponseMode         TextResponseMode         `json:"text_response_mode"`
		MaxOutputTokens      int                      `json:"max_output_tokens"`
		OutputTokenParameter TextOutputTokenParameter `json:"output_token_parameter"`
		Temperature          *float64                 `json:"temperature"`
		ThinkingMode         string                   `json:"thinking_mode"`
		ReasoningSplit       bool                     `json:"reasoning_split"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &constraints); err != nil {
			return err
		}
	}
	// prompt_json is the safest backward-compatible mode for existing
	// OpenAI-compatible routes whose strict schema capability was never
	// recorded. A route must opt into stronger response modes explicitly.
	if constraints.ResponseMode == "" {
		constraints.ResponseMode = TextResponsePromptJSON
	}
	snapshot.TextResponseMode = constraints.ResponseMode
	snapshot.MaxOutputTokens = constraints.MaxOutputTokens
	snapshot.OutputTokenParameter = constraints.OutputTokenParameter
	if constraints.Temperature != nil {
		snapshot.Temperature = *constraints.Temperature
		snapshot.TemperatureSet = true
	}
	snapshot.ThinkingMode = constraints.ThinkingMode
	snapshot.ReasoningSplit = constraints.ReasoningSplit
	return nil
}

func (s MySQLGatewayConfigStore) ResolveGatewayCredential(ctx context.Context, credentialID string, version int64) (string, error) {
	if s.DB == nil || s.Cipher == nil {
		return "", fmt.Errorf("MySQL database and credential cipher are required")
	}
	var ciphertext, nonce []byte
	var keyVersion string
	err := s.DB.QueryRowContext(ctx, `SELECT ciphertext, nonce, key_version
		FROM provider_credentials
		WHERE id = ? AND credential_version = ? AND status IN ('active', 'retired')`,
		credentialID, version,
	).Scan(&ciphertext, &nonce, &keyVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("adapter gateway credential version is unavailable")
	}
	if err != nil {
		return "", err
	}
	plaintext, err := s.Cipher.Decrypt(ciphertext, nonce, keyVersion)
	if err != nil {
		return "", fmt.Errorf("decrypt adapter gateway credential: %w", err)
	}
	token := strings.TrimSpace(string(plaintext))
	if token == "" {
		return "", fmt.Errorf("adapter gateway credential is empty")
	}
	return token, nil
}

type CredentialCipher interface {
	Encrypt([]byte) (ciphertext, nonce []byte, keyVersion string, err error)
	Decrypt(ciphertext, nonce []byte, keyVersion string) ([]byte, error)
}

// AESGCMCredentialCipher keeps the master key outside MySQL. The database
// contains only authenticated ciphertext, a random nonce, and a key version.
type AESGCMCredentialCipher struct {
	key        []byte
	keyVersion string
}

func NewAESGCMCredentialCipher(base64Key, keyVersion string) (*AESGCMCredentialCipher, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(base64Key))
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("provider master key must be base64-encoded 32 bytes")
	}
	if strings.TrimSpace(keyVersion) == "" {
		return nil, fmt.Errorf("provider master key version is required")
	}
	return &AESGCMCredentialCipher{key: key, keyVersion: strings.TrimSpace(keyVersion)}, nil
}

func (c *AESGCMCredentialCipher) Encrypt(plaintext []byte) ([]byte, []byte, string, error) {
	if c == nil || len(c.key) != 32 || len(plaintext) == 0 {
		return nil, nil, "", fmt.Errorf("credential cipher and plaintext are required")
	}
	aead, err := newGCM(c.key)
	if err != nil {
		return nil, nil, "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, "", err
	}
	return aead.Seal(nil, nonce, plaintext, []byte(c.keyVersion)), nonce, c.keyVersion, nil
}

func (c *AESGCMCredentialCipher) Decrypt(ciphertext, nonce []byte, keyVersion string) ([]byte, error) {
	if c == nil || keyVersion != c.keyVersion {
		return nil, fmt.Errorf("provider master key version %q is unavailable", keyVersion)
	}
	aead, err := newGCM(c.key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("credential nonce has an invalid length")
	}
	return aead.Open(nil, nonce, ciphertext, []byte(keyVersion))
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func routeDeadline(snapshot GatewayRouteSnapshot, now time.Time) time.Time {
	return now.Add(time.Duration(snapshot.TimeoutSeconds) * time.Second)
}
