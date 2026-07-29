package prompts

import (
	"strings"
	"testing"
)

func TestBuildProposalStrategyMessagesIncludesComplianceAndRequiredJSON(t *testing.T) {
	t.Parallel()
	messages := BuildProposalStrategyMessages(ProposalInput{
		Brand: "极地鲜生", Product: "深海鳕鱼柳",
		Compliance: []string{"禁用绝对化用语"}, Directions: []string{"冷链鲜度"},
	})
	if len(messages) != 2 || !strings.Contains(messages[0].Content, "资深品牌电商广告策划") {
		t.Fatalf("unexpected strategy messages: %#v", messages)
	}
	if !strings.Contains(messages[0].Content, "JSON") || !strings.Contains(messages[1].Content, "禁用绝对化用语") {
		t.Fatalf("strategy prompt lost output or compliance constraints: %#v", messages)
	}
}

func TestMediaPromptsUseStableVariantDirection(t *testing.T) {
	t.Parallel()
	input := ProposalInput{Brand: "极地鲜生", Product: "深海鳕鱼柳", Compliance: []string{}, Directions: []string{"冷链鲜度"}}
	if !strings.Contains(BuildImagePrompt(input, 1), "冷链鲜度") || !strings.Contains(BuildVideoPrompt(input, 2), "冷链鲜度") {
		t.Fatal("media prompts must retain selected creative direction")
	}
}

func TestProposalStrategyMessagesIncludeVolcadPackageSource(t *testing.T) {
	t.Parallel()
	input := ProposalInput{
		Brand: "极地鲜生", Product: "深海鳕鱼柳",
		Compliance: []string{"禁用绝对化用语"}, Directions: []string{"家庭共食氛围"},
		Source: ProposalSource{Type: "tos", ObjectURI: "tos://cookies-assets/demo/volcad/mock_proposal_package.zip"},
		ProposalPackage: &VolcadProposalPackage{
			CampaignName:   "极地鲜生 618 新品推广",
			Brief:          VolcadBrandBrief{SellingPoints: []string{"野生捕捞", "冷链直达"}},
			Options:        VolcadGenerationOptions{MaterialTypes: []string{"素材二创", "卡点直播"}},
			VideoDirection: VolcadVideoDirection{OpeningHooks: []string{"晚饭不知道吃什么的"}},
		},
	}
	messages := BuildProposalStrategyMessages(input)
	user := messages[1].Content
	for _, want := range []string{"tos://cookies-assets/demo/volcad/mock_proposal_package.zip", "极地鲜生 618 新品推广", "野生捕捞", "卡点直播", "晚饭不知道吃什么的"} {
		if !strings.Contains(user, want) {
			t.Fatalf("proposal context missing %q: %s", want, user)
		}
	}
}
