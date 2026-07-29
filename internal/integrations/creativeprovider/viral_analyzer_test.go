package creativeprovider

import (
	"testing"

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
