package strategy

import (
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type CreativeRoute struct {
	RouteType                 string                     `json:"route_type"`
	VideoPurpose              string                     `json:"video_purpose"`
	Channels                  []string                   `json:"channels"`
	Reason                    string                     `json:"reason"`
	TargetDurationSeconds     int                        `json:"target_duration_seconds"`
	AspectRatio               string                     `json:"aspect_ratio"`
	SourceAssetRefs           []contract.AssetVersionRef `json:"source_asset_refs"`
	EvidenceRefs              []string                   `json:"evidence_refs"`
	RequiresHumanConfirmation bool                       `json:"requires_human_confirmation"`
}

func (r CreativeRoute) Validate() error {
	if r.RouteType != "pre_roll" || r.VideoPurpose != "performance" {
		return fmt.Errorf("unsupported creative route")
	}
	if len(r.Channels) == 0 || r.TargetDurationSeconds != 5 || r.AspectRatio != "9:16" || strings.TrimSpace(r.Reason) == "" || !r.RequiresHumanConfirmation {
		return fmt.Errorf("pre-roll route is incomplete")
	}
	for _, channel := range r.Channels {
		if channel != "douyin" && channel != "kuaishou" {
			return fmt.Errorf("pre-roll route channel %q is unsupported", channel)
		}
	}
	for _, ref := range r.SourceAssetRefs {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("source asset: %w", err)
		}
	}
	return nil
}

func creativeRoutesForPackage(brief BriefVersion, document StrategyDocument) []CreativeRoute {
	seen := map[string]bool{}
	channels := make([]string, 0, 2)
	for _, channel := range brief.Snapshot.Channels {
		normalized := strings.ToLower(strings.TrimSpace(channel))
		if (normalized == "douyin" || normalized == "kuaishou") && !seen[normalized] {
			seen[normalized] = true
			channels = append(channels, normalized)
		}
	}
	if len(channels) == 0 {
		return nil
	}
	reason := "在正片前快速建立产品与目标人群的关联"
	directions := document.CreativeDirections()
	if len(directions) > 0 && strings.TrimSpace(directions[0]) != "" {
		reason = strings.TrimSpace(directions[0])
	}
	return []CreativeRoute{{
		RouteType: "pre_roll", VideoPurpose: "performance", Channels: channels,
		Reason: reason, TargetDurationSeconds: 5, AspectRatio: "9:16",
		SourceAssetRefs: []contract.AssetVersionRef{}, EvidenceRefs: []string{},
		RequiresHumanConfirmation: true,
	}}
}
