package provider

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultVolcengineSpeechEndpoint = "https://openspeech.bytedance.com/api/v3/tts/unidirectional"

type VolcengineSpeechConfig struct {
	Endpoint     string
	APIKey       string
	ResourceID   string
	DefaultVoice string
	VoiceAliases map[string]string
}

type VolcengineSpeechAdapter struct {
	config       VolcengineSpeechConfig
	client       *http.Client
	newRequestID func() (string, error)
}

func NewVolcengineSpeechAdapter(config VolcengineSpeechConfig) (*VolcengineSpeechAdapter, error) {
	if strings.TrimSpace(config.Endpoint) == "" {
		config.Endpoint = defaultVolcengineSpeechEndpoint
	}
	if !strings.HasPrefix(config.Endpoint, "https://") && !strings.HasPrefix(config.Endpoint, "http://127.0.0.1") && !strings.HasPrefix(config.Endpoint, "http://localhost") {
		return nil, fmt.Errorf("Volcengine speech endpoint must use HTTPS")
	}
	if strings.TrimSpace(config.APIKey) == "" || strings.TrimSpace(config.ResourceID) == "" || strings.TrimSpace(config.DefaultVoice) == "" {
		return nil, fmt.Errorf("Volcengine speech API key, resource ID and default voice are required")
	}
	return &VolcengineSpeechAdapter{config: config, client: &http.Client{Timeout: 2 * time.Minute}, newRequestID: randomSpeechRequestID}, nil
}

func randomSpeechRequestID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(value[:]), nil
}

type volcengineSpeechRequest struct {
	Parameters struct {
		Text    string `json:"text"`
		Speaker string `json:"speaker"`
		Audio   struct {
			Format           string `json:"format"`
			SampleRate       int    `json:"sample_rate"`
			SpeechRate       int    `json:"speech_rate"`
			EnableSubtitle   bool   `json:"enable_subtitle"`
			ExplicitLanguage string `json:"explicit_language,omitempty"`
		} `json:"audio_params"`
	} `json:"req_params"`
}

type volcengineSpeechChunk struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Data     string `json:"data"`
	Sentence struct {
		Text  string `json:"text"`
		Words []struct {
			Word      string  `json:"word"`
			StartTime float64 `json:"startTime"`
			EndTime   float64 `json:"endTime"`
		} `json:"words"`
	} `json:"sentence"`
}

func (a *VolcengineSpeechAdapter) Synthesize(ctx context.Context, input SpeechSynthesisInput) (SpeechSynthesisResult, error) {
	if err := input.Validate(); err != nil {
		return SpeechSynthesisResult{}, err
	}
	requestID := strings.TrimSpace(input.RequestID)
	if requestID == "" {
		var err error
		requestID, err = a.newRequestID()
		if err != nil {
			return SpeechSynthesisResult{}, err
		}
	}
	voice := a.config.DefaultVoice
	if mapped := a.config.VoiceAliases[input.VoiceAlias]; strings.TrimSpace(mapped) != "" {
		voice = mapped
	} else if !strings.HasPrefix(input.VoiceAlias, "cookies.") {
		voice = input.VoiceAlias
	}
	var payload volcengineSpeechRequest
	payload.Parameters.Text, payload.Parameters.Speaker = strings.TrimSpace(input.Text), voice
	payload.Parameters.Audio.Format, payload.Parameters.Audio.SampleRate = input.Format, input.SampleRate
	payload.Parameters.Audio.SpeechRate = int((input.SpeakingRate - 1) * 100)
	payload.Parameters.Audio.EnableSubtitle = input.NeedTimestamps
	payload.Parameters.Audio.ExplicitLanguage = normalizeVolcengineSpeechLanguage(input.Language)
	body, err := json.Marshal(payload)
	if err != nil {
		return SpeechSynthesisResult{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.config.Endpoint, bytes.NewReader(body))
	if err != nil {
		return SpeechSynthesisResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Api-Key", a.config.APIKey)
	request.Header.Set("X-Api-Resource-Id", a.config.ResourceID)
	request.Header.Set("X-Api-Request-Id", requestID)
	response, err := a.client.Do(request)
	if err != nil {
		return SpeechSynthesisResult{}, SpeechProviderError{Code: "transport_error", Message: err.Error(), Retryable: true}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return SpeechSynthesisResult{}, classifyVolcengineSpeechError(response.StatusCode, string(message))
	}
	result := SpeechSynthesisResult{Codec: input.Format, SampleRate: input.SampleRate, OriginalText: input.Text, WordTimings: []SpeechWordTiming{}, ProviderRequestID: response.Header.Get("X-Tt-Logid"), ModelAndVoiceSnapshot: a.config.ResourceID + "/" + voice}
	decoder := json.NewDecoder(response.Body)
	for {
		var chunk volcengineSpeechChunk
		if err := decoder.Decode(&chunk); err == io.EOF {
			break
		} else if err != nil {
			return SpeechSynthesisResult{}, SpeechProviderError{Code: "invalid_response", Message: err.Error(), Retryable: true}
		}
		if chunk.Code != 0 {
			return SpeechSynthesisResult{}, classifyVolcengineSpeechError(chunk.Code, chunk.Message)
		}
		if chunk.Data != "" {
			audio, decodeErr := base64.StdEncoding.DecodeString(chunk.Data)
			if decodeErr != nil {
				return SpeechSynthesisResult{}, SpeechProviderError{Code: "invalid_audio", Message: decodeErr.Error(), Retryable: true}
			}
			result.Audio = append(result.Audio, audio...)
		}
		if chunk.Sentence.Text != "" {
			result.NormalizedText = chunk.Sentence.Text
		}
		for _, word := range chunk.Sentence.Words {
			timing := SpeechWordTiming{Text: word.Word, BeginMS: int(word.StartTime * 1000), EndMS: int(word.EndTime * 1000)}
			result.WordTimings = append(result.WordTimings, timing)
			if timing.EndMS > result.DurationMS {
				result.DurationMS = timing.EndMS
			}
		}
	}
	if len(result.Audio) == 0 {
		return SpeechSynthesisResult{}, SpeechProviderError{Code: "empty_audio", Message: "speech provider returned no audio", Retryable: true}
	}
	if result.NormalizedText == "" {
		result.NormalizedText = input.Text
	}
	return result, nil
}

func normalizeVolcengineSpeechLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "zh-cn", "zh":
		return "zh-cn"
	case "en", "en-us", "en-gb":
		return "en"
	default:
		return ""
	}
}

func classifyVolcengineSpeechError(code int, message string) SpeechProviderError {
	lower := strings.ToLower(message)
	switch {
	case code == http.StatusTooManyRequests || strings.Contains(lower, "quota") || strings.Contains(lower, "concurrency"):
		return SpeechProviderError{Code: "quota_exceeded", Message: message, Retryable: true}
	case code >= 500 || strings.Contains(lower, "timeout") || strings.Contains(lower, "internal"):
		return SpeechProviderError{Code: "provider_unavailable", Message: message, Retryable: true}
	case code == http.StatusUnauthorized || code == http.StatusForbidden || strings.Contains(lower, "access denied"):
		return SpeechProviderError{Code: "capability_unavailable", Message: message, Retryable: false}
	default:
		return SpeechProviderError{Code: "input_rejected", Message: message, Retryable: false}
	}
}
