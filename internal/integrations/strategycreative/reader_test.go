package strategycreative

import (
	"testing"

	"github.com/shikanon/cookies/internal/systems/strategy"
)

func TestCreativeRouteSnapshotOmitsVideoPurposeForImageText(t *testing.T) {
	t.Parallel()

	route := strategy.CreativeHandoffRoute{
		RouteID: "route_xiaohongshu_image_text", DeliverableType: "image_text",
		Purpose: "brand", Channels: []string{"xiaohongshu"}, Reason: "match the approved channel plan",
		Spec:           strategy.CreativeRouteSpec{AspectRatio: "3:4", Resolution: "1080x1440"},
		RouteReadiness: strategy.HandoffReadiness{Status: "ready"},
	}

	snapshot := creativeRouteSnapshotFromHandoff(route)
	if snapshot.VideoPurpose != "" {
		t.Fatalf("image-text route video purpose = %q", snapshot.VideoPurpose)
	}
	if err := snapshot.Validate(); err != nil {
		t.Fatalf("validate image-text route snapshot: %v", err)
	}
}
