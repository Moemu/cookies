package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVolcengineSpeechAdapterSynthesizesChunkedAudioAndWordTimings(t *testing.T) {
	var gotHeader, gotResource, gotRequestID, gotSpeaker string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader, gotResource, gotRequestID = r.Header.Get("X-Api-Key"), r.Header.Get("X-Api-Resource-Id"), r.Header.Get("X-Api-Request-Id")
		var request volcengineSpeechRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode speech request: %v", err)
		}
		gotSpeaker = request.Parameters.Speaker
		w.Header().Set("X-Tt-Logid", "log-1")
		fmt.Fprintf(w, `{"code":0,"message":"OK","data":%q}`+"\n", base64.StdEncoding.EncodeToString([]byte("audio-one")))
		fmt.Fprintf(w, `{"code":0,"message":"OK","data":%q,"sentence":{"text":"你好","words":[{"word":"你","startTime":0.0,"endTime":0.2},{"word":"好","startTime":0.2,"endTime":0.5}]}}`+"\n", base64.StdEncoding.EncodeToString([]byte("-two")))
		fmt.Fprintln(w, `{"code":20000000,"message":"OK"}`)
	}))
	defer server.Close()

	adapter, err := NewVolcengineSpeechAdapter(VolcengineSpeechConfig{Endpoint: server.URL, APIKey: "secret", ResourceID: "seed-tts-2.0", DefaultVoice: "zh_female_test"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Synthesize(context.Background(), SpeechSynthesisInput{RequestID: "attempt-stable-1", Text: "你好", VoiceAlias: "cookies.voice.douyin.default", Language: "zh-CN", Format: "mp3", SampleRate: 24000, SpeakingRate: 1, NeedTimestamps: true})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Audio) != "audio-one-two" || result.NormalizedText != "你好" || len(result.WordTimings) != 2 || result.DurationMS != 500 {
		t.Fatalf("unexpected speech result: %#v", result)
	}
	if gotHeader != "secret" || gotResource != "seed-tts-2.0" || gotRequestID != "attempt-stable-1" || gotSpeaker != "zh_female_test" || result.ProviderRequestID != "log-1" {
		t.Fatalf("request mapping was not preserved: key=%q resource=%q request=%q speaker=%q result=%#v", gotHeader, gotResource, gotRequestID, gotSpeaker, result)
	}
}

func TestVolcengineSpeechAdapterMapsLegacyAINativeVoiceAliasToConfiguredSpeaker(t *testing.T) {
	var gotSpeaker string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request volcengineSpeechRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode speech request: %v", err)
		}
		gotSpeaker = request.Parameters.Speaker
		fmt.Fprintf(w, `{"code":0,"message":"OK","data":%q}`+"\n", base64.StdEncoding.EncodeToString([]byte("audio")))
	}))
	defer server.Close()

	adapter, err := NewVolcengineSpeechAdapter(VolcengineSpeechConfig{Endpoint: server.URL, APIKey: "secret", ResourceID: "seed-tts-2.0", DefaultVoice: "zh_female_vv_uranus_bigtts"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Synthesize(context.Background(), SpeechSynthesisInput{RequestID: "legacy-attempt", Text: "你好", VoiceAlias: "douyin-female-01", Language: "zh-CN", Format: "mp3", SampleRate: 24000, SpeakingRate: 1})
	if err != nil {
		t.Fatal(err)
	}
	if gotSpeaker != "zh_female_vv_uranus_bigtts" {
		t.Fatalf("legacy AI native alias leaked to provider as %q", gotSpeaker)
	}
}

func TestVolcengineSpeechAdapterClassifiesProviderFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, `{"code":45000000,"message":"quota exceeded"}`)
	}))
	defer server.Close()
	adapter, err := NewVolcengineSpeechAdapter(VolcengineSpeechConfig{Endpoint: server.URL, APIKey: "secret", ResourceID: "seed-tts-2.0", DefaultVoice: "voice"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.Synthesize(context.Background(), SpeechSynthesisInput{Text: "你好", VoiceAlias: "voice", Language: "zh-CN", Format: "mp3", SampleRate: 24000, SpeakingRate: 1})
	providerErr, ok := err.(SpeechProviderError)
	if !ok || providerErr.Code != "quota_exceeded" || !providerErr.Retryable {
		t.Fatalf("unexpected provider error: %#v", err)
	}
}
