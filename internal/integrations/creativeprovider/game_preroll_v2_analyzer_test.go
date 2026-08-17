package creativeprovider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/creative"
)

type gameAnalyzerRouteStub struct{ route provider.GatewayRouteSnapshot }

func (s gameAnalyzerRouteStub) ResolveTextRoute(context.Context, contract.OrganizationID, string) (provider.GatewayRouteSnapshot, error) {
	return s.route, nil
}

func TestNormalizeGamePrerollAnalysisKeepsDefaultDownloadCTA(t *testing.T) {
	result := creative.GamePrerollV2AnalysisResult{SuggestedBrief: []creative.GameBriefField{
		{ID: "cta", Key: "cta", Label: "CTA", Value: "立即预约下载", Provenance: creative.GameProvenanceAI, Required: true},
	}}
	normalizeGamePrerollAnalysis(&result)
	if got := result.SuggestedBrief[0]; got.Value != "立即下载" || got.Provenance != creative.GameProvenanceManual {
		t.Fatalf("CTA must remain the editable product default: %#v", got)
	}
}

type gameAnalyzerCredentialStub struct{}

func (gameAnalyzerCredentialStub) ResolveGatewayCredential(context.Context, string, int64) (string, error) {
	return "test-token", nil
}

func TestGamePrerollV2AnalyzerUsesGameSchemaAndFrameTimestamps(t *testing.T) {
	t.Parallel()
	var requestBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{\"game_name\":\"测试游戏\",\"gameplay_summary\":\"玩家完成操作后画面显示连续反馈结果。\",\"facts\":[{\"id\":\"fact_1\",\"label\":\"玩法\",\"value\":\"操作后反馈\",\"provenance\":\"video_evidence\",\"evidence_refs\":[\"evidence_1\"]}],\"evidence\":[{\"id\":\"evidence_1\",\"kind\":\"operation\",\"start_milliseconds\":1000,\"end_milliseconds\":1500,\"description\":\"操作\",\"verified_copy\":[]}],\"unknowns\":[\"奖励未知\"],\"suggested_brief\":[]}"}}]}`)
	}))
	defer server.Close()
	route := provider.GatewayRouteSnapshot{RouteID: "route", RouteRevisionID: "route_rev", ConnectionID: "connection", ConnectionRevisionID: "connection_rev", ConnectionType: "openai", UpstreamModel: "vision-model", BaseURL: server.URL, CredentialID: "cred", CredentialVersion: 1, TimeoutSeconds: 5, MaxResponseBytes: 1 << 20, MaxOutputTokens: 4096, OutputTokenParameter: provider.TextOutputTokenParameterMaxTokens, TextResponseMode: provider.TextResponsePromptJSON}
	helper, err := NewShortDramaV2Analyzer(ViralAnalyzerConfig{Assets: shortDramaV2AssetOpenerStub{}, Routes: gameAnalyzerRouteStub{route}, Credentials: gameAnalyzerCredentialStub{}, FFmpegPath: "ffmpeg", ModelAlias: "cookies.text.standard", PromptVersion: "game-preroll-analysis/v1", ASR: ASRConfig{Endpoint: "https://example.com/asr"}, Client: server.Client(), AllowInsecureHTTP: true})
	if err != nil {
		t.Fatal(err)
	}
	analyzer := &GamePrerollV2Analyzer{helper: helper, sampler: &CommercePrerollV2Analyzer{helper: helper}}
	result, err := analyzer.callModel(context.Background(), contract.ActorContext{OrganizationID: "org_1"}, "", []commerceSampledFrame{{TimestampMS: 1000, Content: []byte("jpeg")}, {TimestampMS: 2300, Content: []byte("jpeg")}, {TimestampMS: 4100, Content: []byte("jpeg")}})
	if err != nil {
		t.Fatal(err)
	}
	if result.GameName != "测试游戏" || result.Facts[0].Provenance != "video_evidence" || result.Unknowns[0] != "奖励未知" {
		t.Fatalf("unexpected result: %#v", result)
	}
	var payload map[string]any
	if err := json.Unmarshal(requestBody, &payload); err != nil {
		t.Fatal(err)
	}
	encoded := string(requestBody)
	for _, wanted := range []string{"效果广告的游戏原视频分析器", "frame_1 timestamp_ms=1000", "frame_3 timestamp_ms=4100", "奖励未知"} {
		if !strings.Contains(encoded, wanted) && wanted != "奖励未知" {
			t.Fatalf("request does not contain %q: %s", wanted, encoded)
		}
	}
}
