package provider

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const DefaultMiniMaxSpeechModelAlias = "cookies.speech.brand"

type MiniMaxSpeechAdapter struct {
	Routes            SpeechRouteResolver
	Credentials       GatewayCredentialResolver
	ModelAlias        string
	DefaultVoiceAlias string
	VoiceAliases      map[string]string
	Client            *http.Client
}

func (a MiniMaxSpeechAdapter) Synthesize(ctx context.Context, input SpeechSynthesisInput) (SpeechSynthesisResult, error) {
	if err := input.Validate(); err != nil {
		return SpeechSynthesisResult{}, err
	}
	if a.Routes == nil || a.Credentials == nil || input.OrganizationID == "" {
		return SpeechSynthesisResult{}, fmt.Errorf("MiniMax speech route, credentials, and organization are required")
	}
	modelAlias := strings.TrimSpace(input.ModelAlias)
	if modelAlias == "" {
		modelAlias = strings.TrimSpace(a.ModelAlias)
	}
	if modelAlias == "" {
		modelAlias = DefaultMiniMaxSpeechModelAlias
	}
	route, err := a.Routes.ResolveSpeechRoute(ctx, input.OrganizationID, modelAlias)
	if err != nil {
		return SpeechSynthesisResult{}, SpeechProviderError{Code: "capability_unavailable", Message: err.Error(), Retryable: false}
	}
	credential, err := a.Credentials.ResolveGatewayCredential(ctx, route.CredentialID, route.CredentialVersion)
	if err != nil {
		return SpeechSynthesisResult{}, SpeechProviderError{Code: "capability_unavailable", Message: err.Error(), Retryable: false}
	}
	voiceID := strings.TrimSpace(route.SpeechVoiceAliases[input.VoiceAlias])
	if voiceID == "" {
		voiceID = strings.TrimSpace(a.VoiceAliases[input.VoiceAlias])
	}
	if voiceID == "" {
		return SpeechSynthesisResult{}, SpeechProviderError{Code: "voice_unavailable", Message: "logical voice alias is not configured for MiniMax", Retryable: false}
	}
	if input.Format != "wav" && input.Format != "mp3" {
		return SpeechSynthesisResult{}, SpeechProviderError{Code: "input_rejected", Message: "MiniMax brand speech supports WAV or MP3", Retryable: false}
	}
	requestBody := struct {
		Model  string `json:"model"`
		Text   string `json:"text"`
		Stream bool   `json:"stream"`
		Voice  struct {
			VoiceID string  `json:"voice_id"`
			Speed   float64 `json:"speed"`
			Volume  float64 `json:"vol"`
			Pitch   int     `json:"pitch"`
			Emotion string  `json:"emotion"`
		} `json:"voice_setting"`
		Audio struct {
			SampleRate int    `json:"sample_rate"`
			Bitrate    int    `json:"bitrate"`
			Format     string `json:"format"`
			Channel    int    `json:"channel"`
		} `json:"audio_setting"`
		Language     string `json:"language_boost,omitempty"`
		Subtitle     bool   `json:"subtitle_enable"`
		SubtitleType string `json:"subtitle_type,omitempty"`
		OutputFormat string `json:"output_format"`
	}{Model: route.UpstreamModel, Text: input.Text, Stream: false, Language: minimaxLanguageBoost(input.Language), Subtitle: input.NeedTimestamps, OutputFormat: "hex"}
	requestBody.Voice.VoiceID, requestBody.Voice.Speed, requestBody.Voice.Volume, requestBody.Voice.Emotion = voiceID, input.SpeakingRate, 1, "calm"
	requestBody.Audio.SampleRate, requestBody.Audio.Bitrate, requestBody.Audio.Format, requestBody.Audio.Channel = input.SampleRate, 128000, input.Format, 1
	if input.NeedTimestamps {
		requestBody.SubtitleType = "word"
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return SpeechSynthesisResult{}, err
	}
	endpoint, err := minimaxSpeechEndpoint(route.BaseURL)
	if err != nil {
		return SpeechSynthesisResult{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return SpeechSynthesisResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+credential)
	req.Header.Set("Content-Type", "application/json")
	client := a.Client
	if client == nil {
		timeout := time.Duration(route.TimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 60 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	response, err := client.Do(req)
	if err != nil {
		return SpeechSynthesisResult{}, SpeechProviderError{Code: "transport_error", Message: err.Error(), Retryable: true}
	}
	defer response.Body.Close()
	limit := route.MaxResponseBytes
	if limit <= 0 {
		limit = 16 << 20
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return SpeechSynthesisResult{}, SpeechProviderError{Code: "invalid_response", Message: err.Error(), Retryable: true}
	}
	if int64(len(contents)) > limit {
		return SpeechSynthesisResult{}, SpeechProviderError{Code: "invalid_response", Message: "MiniMax response exceeded route limit", Retryable: true}
	}
	var value struct {
		Data *struct {
			Audio  string `json:"audio"`
			Status int    `json:"status"`
		} `json:"data"`
		Extra struct {
			AudioLength int    `json:"audio_length"`
			SampleRate  int    `json:"audio_sample_rate"`
			Format      string `json:"audio_format"`
		} `json:"extra_info"`
		TraceID string `json:"trace_id"`
		Base    struct {
			StatusCode    int    `json:"status_code"`
			StatusMessage string `json:"status_msg"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(contents, &value); err != nil {
		return SpeechSynthesisResult{}, SpeechProviderError{Code: "invalid_response", Message: err.Error(), Retryable: true}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || value.Base.StatusCode != 0 {
		return SpeechSynthesisResult{}, classifyMiniMaxSpeechError(response.StatusCode, value.Base.StatusCode, value.Base.StatusMessage)
	}
	if value.Data == nil || strings.TrimSpace(value.Data.Audio) == "" {
		return SpeechSynthesisResult{}, SpeechProviderError{Code: "empty_audio", Message: "MiniMax returned no audio", Retryable: true}
	}
	audio, err := hex.DecodeString(value.Data.Audio)
	if err != nil {
		return SpeechSynthesisResult{}, SpeechProviderError{Code: "invalid_audio", Message: err.Error(), Retryable: true}
	}
	sampleRate := value.Extra.SampleRate
	if sampleRate <= 0 {
		sampleRate = input.SampleRate
	}
	return SpeechSynthesisResult{Audio: audio, Codec: input.Format, SampleRate: sampleRate, DurationMS: value.Extra.AudioLength, OriginalText: input.Text, NormalizedText: input.Text, WordTimings: []SpeechWordTiming{}, ProviderRequestID: value.TraceID, ModelAndVoiceSnapshot: "minimax/" + route.UpstreamModel + "/" + voiceID}, nil
}

func (a MiniMaxSpeechAdapter) ProbeSpeechCapability(ctx context.Context, organizationID contract.OrganizationID) SpeechCapability {
	modelAlias := a.ModelAlias
	if modelAlias == "" {
		modelAlias = DefaultMiniMaxSpeechModelAlias
	}
	if a.Routes == nil {
		return SpeechCapability{Provider: "minimax", Available: false, ErrorCode: "capability_unavailable", ErrorMessage: "MiniMax speech route is not configured", VoiceAliases: []string{}}
	}
	route, routeErr := a.Routes.ResolveSpeechRoute(ctx, organizationID, modelAlias)
	if routeErr != nil {
		return SpeechCapability{Provider: "minimax", Available: false, ErrorCode: "capability_unavailable", ErrorMessage: routeErr.Error(), VoiceAliases: []string{}}
	}
	voices := route.SpeechVoiceAliases
	if len(voices) == 0 {
		voices = a.VoiceAliases
	}
	aliases := make([]string, 0, len(voices))
	for alias := range voices {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	voiceAlias := strings.TrimSpace(a.DefaultVoiceAlias)
	if voiceAlias == "" && len(aliases) > 0 {
		voiceAlias = aliases[0]
	}
	result, err := a.Synthesize(ctx, SpeechSynthesisInput{OrganizationID: organizationID, ModelAlias: modelAlias, Text: "语音能力测试。", VoiceAlias: voiceAlias, Language: "zh-CN", Format: "wav", SampleRate: 32000, SpeakingRate: 1})
	if err != nil {
		capability := SpeechCapability{Provider: "minimax", Available: false, VoiceAliases: aliases}
		if providerErr, ok := err.(SpeechProviderError); ok {
			capability.ErrorCode, capability.ErrorMessage = providerErr.Code, providerErr.Message
		} else {
			capability.ErrorCode, capability.ErrorMessage = "probe_failed", err.Error()
		}
		return capability
	}
	parts := strings.Split(result.ModelAndVoiceSnapshot, "/")
	capability := SpeechCapability{Provider: "minimax", Available: true, VoiceAliases: aliases}
	if len(parts) >= 3 {
		capability.Model, capability.VoiceID = parts[1], strings.Join(parts[2:], "/")
	}
	return capability
}

func minimaxSpeechEndpoint(base string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("MiniMax speech base URL is invalid")
	}
	path := strings.TrimSuffix(parsed.Path, "/")
	if strings.HasSuffix(path, "/v1") {
		parsed.Path = path + "/t2a_v2"
	} else {
		parsed.Path = path + "/v1/t2a_v2"
	}
	return parsed.String(), nil
}

func minimaxLanguageBoost(language string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "zh") {
		return "Chinese"
	}
	return "auto"
}

func classifyMiniMaxSpeechError(httpStatus, code int, message string) SpeechProviderError {
	if message == "" {
		message = fmt.Sprintf("MiniMax speech request failed with status %d code %d", httpStatus, code)
	}
	switch code {
	case 1002:
		return SpeechProviderError{Code: "rate_limited", Message: message, Retryable: true}
	case 1004, 2049:
		return SpeechProviderError{Code: "capability_unavailable", Message: message, Retryable: false}
	case 1008, 2056:
		return SpeechProviderError{Code: "quota_exceeded", Message: message, Retryable: false}
	case 1000, 1001, 1024, 1033:
		return SpeechProviderError{Code: "provider_unavailable", Message: message, Retryable: true}
	case 2013, 20132, 2042:
		return SpeechProviderError{Code: "input_rejected", Message: message, Retryable: false}
	}
	if httpStatus == http.StatusUnauthorized || httpStatus == http.StatusForbidden {
		return SpeechProviderError{Code: "capability_unavailable", Message: message, Retryable: false}
	}
	if httpStatus == http.StatusTooManyRequests || httpStatus >= 500 {
		return SpeechProviderError{Code: "provider_unavailable", Message: message, Retryable: true}
	}
	return SpeechProviderError{Code: "input_rejected", Message: message, Retryable: false}
}
