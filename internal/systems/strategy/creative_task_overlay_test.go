package strategy

import (
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestMaterializeCreativeTaskOverlayKeepsStrategyDimensionsWithoutCreativeAuthorship(t *testing.T) {
	version := CreativeTaskStrategyVersion{
		PlanID: "plan_1", Version: 2, OrganizationID: "org_1", ProjectID: "project_1",
		ContentHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		CreatedAt:   time.Date(2026, time.July, 31, 8, 0, 0, 0, time.UTC),
		Document: CreativeTaskStrategyDocument{
			ContractVersion: "creative-task-strategy/v2",
			PackageRef: &CreativeTaskStrategyPackageRef{
				PackageID: "package_1", PackageVersion: 3,
				PackageContentHash:     "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				HandoffContractVersion: CreativeHandoffContractVersion,
				HandoffContentHash:     "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			},
			HandoffRef: &CreativeTaskStrategyHandoffRef{
				ContractVersion: CreativeHandoffContractVersion,
				ContentHash:     "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			},
			SelectedRouteID: "route_xhs", MessageHierarchy: []string{"先场景后证据"},
			BusinessStrategy: map[string]any{"content_angle": "通勤场景"},
			Hypotheses:       []CreativeTaskHypothesis{{Statement: "场景先行提升代入感"}},
			Guardrails:       []string{"不得夸大"}, OpenQuestions: []string{},
		},
	}
	overlay, err := materializeCreativeTaskOverlay("overlay_1", version)
	if err != nil {
		t.Fatal(err)
	}
	if overlay.StrategyDimensions["content_angle"] != "通勤场景" ||
		overlay.SelectedRouteID != "route_xhs" || overlay.ContentHash == "" {
		t.Fatalf("unexpected task overlay: %+v", overlay)
	}
	hashInput := overlay
	hashInput.ContentHash = ""
	hash, err := contract.NewContentHash(hashInput)
	if err != nil || string(hash) != overlay.ContentHash {
		t.Fatalf("overlay hash mismatch: calculated=%s stored=%s err=%v", hash, overlay.ContentHash, err)
	}
	if containsReservedOutput(overlay.StrategyDimensions) {
		t.Fatal("overlay leaked a Creative-owned output field")
	}
}
