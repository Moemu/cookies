package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type minimaxSpeechRouteStub struct {
	route GatewayRouteSnapshot
	err   error
}

func (s minimaxSpeechRouteStub) ResolveSpeechRoute(context.Context, contract.OrganizationID, string) (GatewayRouteSnapshot, error) {
	return s.route, s.err
}

type minimaxCredentialStub struct {
	value string
	err   error
}

func (s minimaxCredentialStub) ResolveGatewayCredential(context.Context, string, int64) (string, error) {
	return s.value, s.err
}

func TestMiniMaxSpeechAdapterSynthesizesHexAudioThroughResolvedEncryptedRoute(t *testing.T) {
	var authorization string
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		fmt.Fprint(w, `{"data":{"audio":"52494646","status":2},"extra_info":{"audio_length":860,"audio_sample_rate":32000,"audio_format":"wav"},"trace_id":"trace-1","base_resp":{"status_code":0,"status_msg":"success"}}`)
	}))
	defer server.Close()
	route := GatewayRouteSnapshot{BaseURL: server.URL, UpstreamModel: "speech-2.8-turbo", CredentialID: "credential_1", CredentialVersion: 2}
	adapter := MiniMaxSpeechAdapter{Routes: minimaxSpeechRouteStub{route: route}, Credentials: minimaxCredentialStub{value: "secret-key"}, ModelAlias: "cookies.speech.brand", VoiceAliases: map[string]string{"cookies.voice.brand.warm_female": "Chinese (Mandarin)_Warm_Bestie"}, Client: server.Client()}
	result, err := adapter.Synthesize(context.Background(), SpeechSynthesisInput{OrganizationID: "org_1", Text: "娇兰二十五倍蜂皇水。", VoiceAlias: "cookies.voice.brand.warm_female", Language: "zh-CN", Format: "wav", SampleRate: 32000, SpeakingRate: 1, NeedTimestamps: true})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Audio) != "RIFF" || result.DurationMS != 860 || result.ProviderRequestID != "trace-1" || result.ModelAndVoiceSnapshot != "minimax/speech-2.8-turbo/Chinese (Mandarin)_Warm_Bestie" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if authorization != "Bearer secret-key" || body["model"] != "speech-2.8-turbo" || body["output_format"] != "hex" || body["subtitle_enable"] != true {
		t.Fatalf("unexpected request auth=%q body=%#v", authorization, body)
	}
}

func TestMiniMaxSpeechAdapterClassifiesQuotaAndCredentialFailures(t *testing.T) {
	for _, tc := range []struct {
		code  int
		want  string
		retry bool
	}{{1008, "quota_exceeded", false}, {2049, "capability_unavailable", false}, {1002, "rate_limited", true}, {1024, "provider_unavailable", true}} {
		t.Run(fmt.Sprint(tc.code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				fmt.Fprintf(w, `{"base_resp":{"status_code":%d,"status_msg":"failed"}}`, tc.code)
			}))
			defer server.Close()
			adapter := MiniMaxSpeechAdapter{Routes: minimaxSpeechRouteStub{route: GatewayRouteSnapshot{BaseURL: server.URL, UpstreamModel: "speech-2.8-turbo", CredentialID: "credential_1", CredentialVersion: 1}}, Credentials: minimaxCredentialStub{value: "key"}, ModelAlias: "cookies.speech.brand", VoiceAliases: map[string]string{"voice": "upstream"}, Client: server.Client()}
			_, err := adapter.Synthesize(context.Background(), SpeechSynthesisInput{OrganizationID: "org", Text: "测试", VoiceAlias: "voice", Language: "zh-CN", Format: "wav", SampleRate: 32000, SpeakingRate: 1})
			providerErr, ok := err.(SpeechProviderError)
			if !ok || providerErr.Code != tc.want || providerErr.Retryable != tc.retry {
				t.Fatalf("unexpected error %#v", err)
			}
		})
	}
}
