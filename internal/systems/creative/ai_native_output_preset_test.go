package creative

import (
	"context"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestOutputPresetRegistryExposesAllSupportedCreationSurfaces(t *testing.T) {
	registry := NewOutputPresetRegistry(NewChannelCreativeProfileRegistry())

	all := registry.List()
	if len(all) != 3 {
		t.Fatalf("registered presets = %d, want 3: %#v", len(all), all)
	}
	available := registry.ListAvailable()
	if len(available) != 3 {
		t.Fatalf("available creation presets = %#v, want all 3 supported surfaces", available)
	}
	for _, id := range []string{
		AINativeOutputPresetDouyinFeed9x16V1,
		"kuaishou_feed_9x16_v1",
		"wechat_channels_feed_9x16_v1",
	} {
		preset, err := registry.Resolve(id)
		if err != nil {
			t.Fatalf("creation preset %q must resolve: %v", id, err)
		}
		if preset.AspectRatio != "9:16" || preset.Width != 720 || preset.Height != 1280 || preset.ProfileHash == "" {
			t.Fatalf("creation preset %q has invalid output geometry or profile: %#v", id, preset)
		}
	}
}

func TestListAINativeOutputPresetsIsProjectScopedAndReadOnly(t *testing.T) {
	registry := NewOutputPresetRegistry(NewChannelCreativeProfileRegistry())
	service := Service{Projects: testProjects{}, AINativeOutputPresets: &registry}
	actor := contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
		Scopes:         []contract.Scope{ScopeRead},
	}

	items, err := service.ListAINativeOutputPresets(context.Background(), actor, "project_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].Label != "抖音信息流 · 9:16" || items[0].Width != 720 || items[0].Height != 1280 {
		t.Fatalf("output presets = %#v", items)
	}
}
