package creative

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	AINativeOutputPresetAvailable   = "available"
	AINativeOutputPresetUnavailable = "unavailable"
)

// AINativeOutputPreset is a creation-output catalog entry. Availability means
// the current generation and renderer path can produce the frozen geometry; it
// does not imply that an advertising-account delivery integration exists.
type AINativeOutputPreset struct {
	AINativeOutputPresetSnapshot
	Status            string `json:"status"`
	UnavailableReason string `json:"unavailable_reason,omitempty"`
}

type OutputPresetRegistry struct {
	items []AINativeOutputPreset
	byID  map[string]AINativeOutputPreset
}

func NewOutputPresetRegistry(profiles ChannelCreativeProfileRegistry) OutputPresetRegistry {
	items := []AINativeOutputPreset{
		newAvailableOutputPreset(profiles, AINativeOutputPresetSnapshot{
			ID: AINativeOutputPresetDouyinFeed9x16V1, Label: "抖音信息流 · 9:16", Channel: "douyin", Placement: "feed",
			AspectRatio: "9:16", Width: 720, Height: 1280, Resolution: "720p", ProfileID: "douyin.performance.v1",
			SafeZone: AINativeOutputSafeZone{Top: 96, Right: 48, Bottom: 240, Left: 48},
		}),
		newAvailableOutputPreset(profiles, AINativeOutputPresetSnapshot{
			ID: "kuaishou_feed_9x16_v1", Label: "快手信息流 · 9:16", Channel: "kuaishou", Placement: "feed",
			AspectRatio: "9:16", Width: 720, Height: 1280, Resolution: "720p", ProfileID: "kuaishou.performance.v1",
			SafeZone: AINativeOutputSafeZone{Top: 96, Right: 48, Bottom: 240, Left: 48},
		}),
		newAvailableOutputPreset(profiles, AINativeOutputPresetSnapshot{
			ID: "wechat_channels_feed_9x16_v1", Label: "视频号信息流 · 9:16", Channel: "wechat_channels", Placement: "feed",
			AspectRatio: "9:16", Width: 720, Height: 1280, Resolution: "720p", ProfileID: "wechat_channels.performance.v1",
			SafeZone: AINativeOutputSafeZone{Top: 96, Right: 48, Bottom: 240, Left: 48},
		}),
		newAvailableOutputPreset(profiles, AINativeOutputPresetSnapshot{
			ID: "xiaohongshu_feed_9x16_v1", Label: "小红书视频信息流 · 9:16", Channel: "xiaohongshu", Placement: "feed",
			AspectRatio: "9:16", Width: 720, Height: 1280, Resolution: "720p", ProfileID: "xiaohongshu.performance.v1",
			SafeZone: AINativeOutputSafeZone{Top: 96, Right: 48, Bottom: 240, Left: 48},
		}),
	}
	byID := make(map[string]AINativeOutputPreset, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	return OutputPresetRegistry{items: items, byID: byID}
}

func newAvailableOutputPreset(profiles ChannelCreativeProfileRegistry, snapshot AINativeOutputPresetSnapshot) AINativeOutputPreset {
	profile, err := profiles.ResolveID(snapshot.ProfileID)
	if err != nil {
		return AINativeOutputPreset{AINativeOutputPresetSnapshot: snapshot, Status: AINativeOutputPresetUnavailable, UnavailableReason: "channel profile is unavailable"}
	}
	snapshot.ProfileVersion = profile.Version
	snapshot.ProfileHash = profile.ContentHash
	if err := snapshot.Validate(); err != nil {
		return AINativeOutputPreset{AINativeOutputPresetSnapshot: snapshot, Status: AINativeOutputPresetUnavailable, UnavailableReason: err.Error()}
	}
	return AINativeOutputPreset{AINativeOutputPresetSnapshot: snapshot, Status: AINativeOutputPresetAvailable}
}

func (r OutputPresetRegistry) List() []AINativeOutputPreset {
	return append([]AINativeOutputPreset(nil), r.items...)
}

func (r OutputPresetRegistry) ListAvailable() []AINativeOutputPreset {
	result := make([]AINativeOutputPreset, 0, len(r.items))
	for _, item := range r.items {
		if item.Status == AINativeOutputPresetAvailable {
			result = append(result, item)
		}
	}
	return result
}

func (r OutputPresetRegistry) Resolve(id string) (AINativeOutputPresetSnapshot, error) {
	item, ok := r.byID[strings.TrimSpace(id)]
	if !ok || item.Status != AINativeOutputPresetAvailable {
		return AINativeOutputPresetSnapshot{}, fmt.Errorf("AI native output preset %q is unavailable", strings.TrimSpace(id))
	}
	return item.AINativeOutputPresetSnapshot, nil
}

func (s Service) outputPresetRegistry() OutputPresetRegistry {
	if s.AINativeOutputPresets != nil {
		return *s.AINativeOutputPresets
	}
	return NewOutputPresetRegistry(NewChannelCreativeProfileRegistry())
}

func (s Service) ListAINativeOutputPresets(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]AINativeOutputPreset, error) {
	if s.Projects == nil {
		return nil, fmt.Errorf("AI native output preset dependencies are incomplete")
	}
	if !actor.HasScope(ScopeRead) {
		return nil, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.outputPresetRegistry().ListAvailable(), nil
}
