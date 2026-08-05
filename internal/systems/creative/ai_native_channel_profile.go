package creative

import (
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type ChannelCreativeProfile struct {
	ID              string   `json:"id"`
	Channel         string   `json:"channel"`
	Purpose         string   `json:"purpose"`
	Version         string   `json:"version"`
	PromptVersion   string   `json:"prompt_version"`
	Rules           []string `json:"rules"`
	ForbiddenClaims []string `json:"forbidden_claims"`
	ContentHash     string   `json:"content_hash"`
}

type ChannelCreativeProfileRegistry struct {
	profiles map[string]ChannelCreativeProfile
}

func NewChannelCreativeProfileRegistry() ChannelCreativeProfileRegistry {
	profile := ChannelCreativeProfile{
		ID: "douyin.performance.v1", Channel: "douyin", Purpose: "performance", Version: "v1",
		PromptVersion: "ai-ad-script/douyin/v1",
		Rules: []string{
			"0至3秒使用痛点、对比、结果或具体场景形成开场钩子",
			"商品或用户问题尽早进入画面，只表达一个主张且最多使用三个证明点",
			"每2至4秒发生可理解的画面变化，卖点必须由动作或场景证明",
			"字幕短句并位于安全区，旁白与字幕语义一致",
			"结尾明确商品、目标人群和下一步行动",
		},
		ForbiddenClaims: []string{
			"不得编造材质、性能、价格、优惠、销量、认证或医疗功效",
			"不得承诺必然爆款、平台推荐、收益或确定性效果",
		},
	}
	hashInput := profile
	hashInput.ContentHash = ""
	profile.ContentHash, _ = contract.CanonicalJSONHash(hashInput)
	return ChannelCreativeProfileRegistry{profiles: map[string]ChannelCreativeProfile{profile.ID: profile}}
}

func (r ChannelCreativeProfileRegistry) Resolve(channel, purpose, version string) (ChannelCreativeProfile, error) {
	id := strings.Join([]string{strings.TrimSpace(channel), strings.TrimSpace(purpose), strings.TrimSpace(version)}, ".")
	profile, ok := r.profiles[id]
	if !ok {
		return ChannelCreativeProfile{}, fmt.Errorf("channel creative profile %q is unavailable", id)
	}
	return profile, nil
}
