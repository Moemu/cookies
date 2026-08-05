package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type SpeechSynthesisInput struct {
	OrganizationID contract.OrganizationID `json:"-"`
	ModelAlias     string                  `json:"-"`
	RequestID      string                  `json:"-"`
	Text           string                  `json:"text"`
	VoiceAlias     string                  `json:"voice_alias"`
	Language       string                  `json:"language"`
	Format         string                  `json:"format"`
	SampleRate     int                     `json:"sample_rate"`
	SpeakingRate   float64                 `json:"speaking_rate"`
	NeedTimestamps bool                    `json:"need_timestamps"`
}

func (i SpeechSynthesisInput) Validate() error {
	if strings.TrimSpace(i.Text) == "" || len([]rune(i.Text)) > 1024 {
		return fmt.Errorf("speech text must contain 1 to 1024 characters")
	}
	if strings.TrimSpace(i.VoiceAlias) == "" || strings.TrimSpace(i.Language) == "" {
		return fmt.Errorf("speech voice and language are required")
	}
	switch i.Format {
	case "mp3", "pcm", "ogg_opus", "wav":
	default:
		return fmt.Errorf("speech audio format is not supported")
	}
	switch i.SampleRate {
	case 8000, 16000, 22050, 24000, 32000, 44100, 48000:
	default:
		return fmt.Errorf("speech sample rate is not supported")
	}
	if i.SpeakingRate < 0.5 || i.SpeakingRate > 2 {
		return fmt.Errorf("speech rate must be between 0.5 and 2")
	}
	return nil
}

type SpeechWordTiming struct {
	Text    string `json:"text"`
	BeginMS int    `json:"begin_ms"`
	EndMS   int    `json:"end_ms"`
}

type SpeechSynthesisResult struct {
	Audio                 []byte             `json:"-"`
	Codec                 string             `json:"codec"`
	SampleRate            int                `json:"sample_rate"`
	DurationMS            int                `json:"duration_ms"`
	OriginalText          string             `json:"original_text"`
	NormalizedText        string             `json:"normalized_text"`
	WordTimings           []SpeechWordTiming `json:"word_timings"`
	ProviderRequestID     string             `json:"provider_request_id"`
	ModelAndVoiceSnapshot string             `json:"model_and_voice_snapshot"`
}

type SpeechSynthesizer interface {
	Synthesize(context.Context, SpeechSynthesisInput) (SpeechSynthesisResult, error)
}

type SpeechCapability struct {
	Provider     string   `json:"provider"`
	Model        string   `json:"model"`
	VoiceID      string   `json:"voice_id"`
	Available    bool     `json:"available"`
	ErrorCode    string   `json:"error_code,omitempty"`
	ErrorMessage string   `json:"error_message,omitempty"`
	VoiceAliases []string `json:"voice_aliases"`
}

type SpeechCapabilityProber interface {
	ProbeSpeechCapability(context.Context, contract.OrganizationID) SpeechCapability
}

type SpeechProviderError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e SpeechProviderError) Error() string { return e.Code + ": " + e.Message }
