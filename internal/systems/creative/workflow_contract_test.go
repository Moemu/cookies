package creative

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

type frozenWorkflowContract struct {
	ContractVersion string            `json:"contract_version"`
	Contracts       map[string]string `json:"contracts"`
	RouteProfiles   map[string]struct {
		DeliverableType string `json:"deliverable_type"`
		Purpose         string `json:"purpose"`
		PerformanceMode string `json:"performance_mode"`
	} `json:"route_profiles"`
	States map[string]struct {
		Transitions map[string][]string `json:"transitions"`
	} `json:"states"`
}

func TestFrozenCreativeWorkflowContractMatchesGoDomain(t *testing.T) {
	t.Parallel()
	payload, err := os.ReadFile("../../../api/fixtures/creative-shared-workflow-v1-frozen.json")
	if err != nil {
		t.Fatal(err)
	}
	var frozen frozenWorkflowContract
	if err := json.Unmarshal(payload, &frozen); err != nil {
		t.Fatal(err)
	}
	if frozen.ContractVersion != CreativeSharedWorkflowV1 {
		t.Fatalf("workflow contract = %q, want %q", frozen.ContractVersion, CreativeSharedWorkflowV1)
	}
	wantContracts := map[string]string{
		"intake_create":             CreativeIntakeCreateV3ContractVersion,
		"intake":                    CreativeIntakeV3ContractVersion,
		"planning_context":          CreativePlanningContextV1,
		"direction_candidate_batch": CreativeDirectionBatchV1,
		"direction":                 CreativeDirectionVersionV1,
	}
	for name, want := range wantContracts {
		if got := frozen.Contracts[name]; got != want {
			t.Fatalf("%s contract = %q, want %q", name, got, want)
		}
	}
	if profile := frozen.RouteProfiles[CreativeRouteImageText]; profile.DeliverableType != "image_text" ||
		profile.Purpose != "brand" || profile.PerformanceMode != "" {
		t.Fatalf("image-text route profile is not frozen: %+v", profile)
	}
	if profile := frozen.RouteProfiles[CreativeRouteBrandVideo]; profile.DeliverableType != "video" ||
		profile.Purpose != "brand" || profile.PerformanceMode != CreativeRouteBrandVideo {
		t.Fatalf("brand-video route profile is not frozen: %+v", profile)
	}

	assertFrozenTransitions(t, frozen.States["intake"].Transitions,
		[]string{"needs_clarification", "ready", "superseded"},
		func(from, to string) bool {
			return CanTransitionCreativeIntakeV3Status(IntakeStatus(from), IntakeStatus(to))
		},
	)
	assertFrozenTransitions(t, frozen.States["direction_batch"].Transitions,
		[]string{"generating", "ready", "failed"},
		func(from, to string) bool {
			return CanTransitionDirectionBatchStatus(
				CreativeDirectionBatchStatus(from),
				CreativeDirectionBatchStatus(to),
			)
		},
	)
	assertFrozenTransitions(t, frozen.States["direction"].Transitions,
		[]string{"candidate", "confirmed", "superseded"},
		func(from, to string) bool {
			return CanTransitionDirectionStatus(CreativeDirectionStatus(from), CreativeDirectionStatus(to))
		},
	)
	assertFrozenTransitions(t, frozen.States["task"].Transitions,
		[]string{"draft", "in_progress", "generating", "generated", "rendering", "ready_for_review", "approved", "delivered", "archived"},
		func(from, to string) bool {
			return CanTransitionCreativeTaskStatus(TaskStatus(from), TaskStatus(to))
		},
	)
	assertFrozenTransitions(t, frozen.States["creative_version"].Transitions,
		[]string{"created", "checked", "approved", "superseded"},
		func(from, to string) bool {
			return CanTransitionCreativeVersionStatus(CreativeVersionStatus(from), CreativeVersionStatus(to))
		},
	)
}

func TestFrozenRouteProfilesSupportImageTextAndBrandVideo(t *testing.T) {
	t.Parallel()
	imageText := CreativeRouteSnapshot{
		RouteID: "route_xhs", RouteType: CreativeRouteImageText,
		Channels: []string{"xiaohongshu"}, Reason: "建立品牌认知", AspectRatio: "3:4",
	}
	if err := imageText.Validate(); err != nil {
		t.Fatalf("image-text route: %v", err)
	}
	brandVideo := CreativeRouteSnapshot{
		RouteID: "route_brand_video", RouteType: CreativeRouteBrandVideo,
		VideoPurpose: "brand", Channels: []string{"douyin", "wechat_official_account"},
		Reason: "建立长期品牌记忆", TargetDurationSeconds: 30, AspectRatio: "16:9",
		RequiresHumanConfirmation: true,
	}
	if err := brandVideo.Validate(); err != nil {
		t.Fatalf("brand-video route: %v", err)
	}
	brandVideo.VideoPurpose = "performance"
	if err := brandVideo.Validate(); err == nil {
		t.Fatal("brand-video route accepted a performance purpose")
	}
}

func assertFrozenTransitions(
	t *testing.T,
	transitions map[string][]string,
	states []string,
	canTransition func(string, string) bool,
) {
	t.Helper()
	for _, from := range states {
		allowed, ok := transitions[from]
		if !ok {
			t.Fatalf("frozen state machine is missing %q", from)
		}
		for _, to := range states {
			want := from == to || slices.Contains(allowed, to)
			if got := canTransition(from, to); got != want {
				t.Fatalf("transition %s -> %s = %t, want %t", from, to, got, want)
			}
		}
	}
}
