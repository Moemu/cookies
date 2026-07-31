package creativeprovider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/shikanon/cookies/internal/platform/provider"
	"github.com/shikanon/cookies/internal/systems/creative"
)

func TestDecodeViralAnalysisAcceptsExactlyStructuredSeed2Output(t *testing.T) {
	t.Parallel()
	result, err := decodeViralAnalysis(`{
	  "dimensions": [
	    {"id":"task_goal_type","prompt":"15 秒转化广告","evidence_refs":["frame:1"],"confidence":0.9},
	    {"id":"quality_style_lighting","prompt":"清晰商业光","evidence_refs":["frame:2"],"confidence":0.8},
	    {"id":"environment_atmosphere","prompt":"冬日户外","evidence_refs":["frame:3"],"confidence":0.8},
	    {"id":"camera_content","prompt":"钩子、证明、CTA","evidence_refs":["frame:4"],"confidence":0.9},
	    {"id":"music_sound","prompt":"节奏递进","evidence_refs":["asr:transcript"],"confidence":0.7}
	  ],
	  "preserve_rules":["保留节奏功能"],
	  "replace_rules":["替换人物和品牌"],
	  "confidence":0.82
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Dimensions) != 5 || result.Dimensions[0].ID != creative.ViralTaskGoalType ||
		result.Dimensions[4].Source != "ai_extracted" || len(result.ReplaceRules) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCallVisionModelClassifiesGatewayAndResponseFailures(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		status    int
		body      string
		wantError error
	}{
		{
			name: "gateway unavailable", status: http.StatusInternalServerError,
			body: `{"error":{"message":"upstream failed"}}`, wantError: creative.ErrViralAnalysisProviderUnavailable,
		},
		{
			name: "gateway rejected image input", status: http.StatusUnprocessableEntity,
			body: `{"error":{"message":"unsupported input"}}`, wantError: creative.ErrViralAnalysisProviderRejected,
		},
		{
			name: "invalid structured response", status: http.StatusOK,
			body: `{"choices":[{"message":{"content":"not json"}}]}`, wantError: creative.ErrViralAnalysisResponseInvalid,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/v1/chat/completions" {
					t.Fatalf("path = %q", request.URL.Path)
				}
				writer.Header().Set("Content-Type", "application/json")
				writer.WriteHeader(tt.status)
				_, _ = writer.Write([]byte(tt.body))
			}))
			defer server.Close()

			analyzer := ViralAnalyzer{config: ViralAnalyzerConfig{Client: server.Client()}}
			_, err := analyzer.callVisionModel(context.Background(), provider.GatewayRouteSnapshot{
				BaseURL: server.URL, UpstreamModel: "seed-2-pro", TimeoutSeconds: 5, MaxResponseBytes: 1024,
			}, "test-token", creative.ViralAnalysisRequest{}, "", [][]byte{[]byte("frame")})
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("error = %v, want category %v", err, tt.wantError)
			}
		})
	}
}
