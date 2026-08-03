package creative

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestCompileImagePromptPackageFreezesLineageAndSourceAssets(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"},
	}
	task := CreativeTask{
		ID: "task_1",
		Direction: CreativeDirection{
			InputIdentityHash: "sha256:input",
		},
	}
	direction := CreativeDirectionVersion{
		ID: "direction_1", ContentHash: "sha256:direction", Concept: "precision made visible",
		ExecutionOutline: []string{"show evidence", "close with CTA"},
	}
	draft := ImageTextDraft{
		ContractVersion: ImageTextDraftV2Contract, TaskID: task.ID, Version: 2,
		DirectionRef: &ImageTextDirectionRef{
			DirectionID: direction.ID, ContentHash: direction.ContentHash,
		},
		InputIdentityHash: task.Direction.InputIdentityHash,
	}
	slot := ImagePlanItem{
		Order: 1, Role: string(ImageTextRoleCover), Purpose: "cover",
		VisualBrief: "machined detail", Caption: "cover", OverlayCopy: "看得见的精度",
		LayoutPreset: "cover_center_v1",
	}
	source := contract.AssetVersionRef{AssetID: "asset_product", Version: 3}
	prompt, err := CompileImagePromptPackage(
		"prompt_1", actor, "project_1", task, draft, slot, direction,
		[]contract.AssetVersionRef{source}, time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("CompileImagePromptPackage() error = %v", err)
	}
	if len(prompt.SourceAssetRefs) != 1 || prompt.SourceAssetRefs[0] != source {
		t.Fatalf("source assets = %+v, want %+v", prompt.SourceAssetRefs, source)
	}
	if prompt.CompilerVersion != ImagePromptCompilerV2 ||
		!strings.Contains(prompt.CompiledPrompt, "one single full-bleed photorealistic commercial photograph") ||
		!strings.Contains(prompt.CompiledPrompt, "Do not create a collage, triptych, split screen") ||
		!strings.Contains(prompt.CompiledPrompt, "Cover role") {
		t.Fatalf("compiled prompt does not enforce the clean photographic base layer: %q", prompt.CompiledPrompt)
	}
	if strings.Contains(prompt.CompiledPrompt, "show evidence") || strings.Contains(prompt.CompiledPrompt, "close with CTA") {
		t.Fatalf("compiled prompt leaked cross-slot execution instructions: %q", prompt.CompiledPrompt)
	}
	withoutSource, err := CompileImagePromptPackage(
		"prompt_2", actor, "project_1", task, draft, slot, direction,
		nil, prompt.CreatedAt,
	)
	if err != nil {
		t.Fatalf("CompileImagePromptPackage() without source error = %v", err)
	}
	if withoutSource.ContentHash == prompt.ContentHash {
		t.Fatal("prompt content hash did not bind source asset lineage")
	}
}

func TestImageTextDraftPlanRequiresFrozenThreeSlotRoles(t *testing.T) {
	t.Parallel()
	plan := ImageTextDraftPlan{
		TitleCandidates: []string{"A", "B", "C"}, SelectedTitle: "A", Body: "body",
		ImagePlan: []ImagePlanItem{
			{Order: 1, Role: "cover", Purpose: "cover", VisualBrief: "visual", Caption: "caption", OverlayCopy: "copy", LayoutPreset: "cover_center_v1"},
			{Order: 2, Role: "proof", Purpose: "proof", VisualBrief: "visual", Caption: "caption", OverlayCopy: "copy", LayoutPreset: "proof_lower_left_v1"},
			{Order: 3, Role: "cta", Purpose: "cta", VisualBrief: "visual", Caption: "caption", OverlayCopy: "copy", LayoutPreset: "cta_bottom_v1"},
		},
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("valid plan rejected: %v", err)
	}
	plan.ImagePlan[1].Role = "cover"
	if err := plan.Validate(); err == nil {
		t.Fatal("plan with duplicated role was accepted")
	}
}

func TestImagePlanWithoutAssetsPreservesAuthoringFields(t *testing.T) {
	asset := contract.AssetVersionRef{AssetID: "asset_old", Version: 2}
	input := []ImagePlanItem{{
		Order: 1, Role: "cover", Purpose: "cover", VisualBrief: "single hero photograph",
		Caption: "caption", OverlayCopy: "headline", LayoutPreset: "cover_center_v1", AssetRef: &asset,
	}}
	result := imagePlanWithoutAssets(input)
	if result[0].AssetRef != nil || result[0].VisualBrief != input[0].VisualBrief || result[0].OverlayCopy != input[0].OverlayCopy {
		t.Fatalf("rework plan = %+v, want authoring fields without the old asset", result[0])
	}
	if input[0].AssetRef == nil {
		t.Fatal("imagePlanWithoutAssets mutated the materialized source draft")
	}
}

func TestImageTextDraftPlanRejectsHighRiskOutboundClaims(t *testing.T) {
	plan := ImageTextDraftPlan{
		TitleCandidates: []string{"夏日场景清单", "三种饮用场景", "通勤聚会饮品参考"},
		SelectedTitle:   "夏日场景清单",
		Body:            "这款饮品完全没负担，控糖人群可以放心入。",
		Topics:          []string{"夏季饮品"},
		ImagePlan: []ImagePlanItem{
			{Order: 1, Role: "cover", Purpose: "cover", VisualBrief: "visual", Caption: "caption", OverlayCopy: "场景清单", LayoutPreset: "cover_center_v1"},
			{Order: 2, Role: "proof", Purpose: "proof", VisualBrief: "visual", Caption: "caption", OverlayCopy: "产品信息", LayoutPreset: "proof_lower_left_v1"},
			{Order: 3, Role: "cta", Purpose: "cta", VisualBrief: "visual", Caption: "caption", OverlayCopy: "了解更多", LayoutPreset: "cta_bottom_v1"},
		},
	}
	if err := plan.Validate(); err == nil || !strings.Contains(err.Error(), "完全没负担") {
		t.Fatalf("Validate() error = %v, want high-risk claim rejection", err)
	}
}

func TestImageTextDraftPlanRejectsUnsupportedGroupPreference(t *testing.T) {
	plan := ImageTextDraftPlan{
		TitleCandidates: []string{"夏日场景清单", "三种饮用场景", "通勤聚会饮品参考"},
		SelectedTitle:   "夏日场景清单",
		Body:            "聚会饮品适合多数人的喜好，饮用无额外负担。",
		Topics:          []string{"夏季饮品"},
		ImagePlan: []ImagePlanItem{
			{Order: 1, Role: "cover", Purpose: "cover", VisualBrief: "visual", Caption: "caption", OverlayCopy: "场景清单", LayoutPreset: "cover_center_v1"},
			{Order: 2, Role: "proof", Purpose: "proof", VisualBrief: "visual", Caption: "caption", OverlayCopy: "产品信息", LayoutPreset: "proof_lower_left_v1"},
			{Order: 3, Role: "cta", Purpose: "cta", VisualBrief: "visual", Caption: "caption", OverlayCopy: "了解更多", LayoutPreset: "cta_bottom_v1"},
		},
	}
	if err := plan.Validate(); err == nil || !strings.Contains(err.Error(), "无额外负担") {
		t.Fatalf("Validate() error = %v, want unsupported group preference rejection", err)
	}
}

func TestImageTextDraftPlanRejectsGroupPreferenceVariants(t *testing.T) {
	plan := ImageTextDraftPlan{
		TitleCandidates: []string{"夏日场景清单", "三种饮用场景", "通勤聚会饮品参考"},
		SelectedTitle:   "夏日场景清单",
		Body:            "这款饮品适配多数人饮用偏好。",
		Topics:          []string{"夏季饮品"},
		ImagePlan: []ImagePlanItem{
			{Order: 1, Role: "cover", Purpose: "cover", VisualBrief: "visual", Caption: "caption", OverlayCopy: "场景清单", LayoutPreset: "cover_center_v1"},
			{Order: 2, Role: "proof", Purpose: "proof", VisualBrief: "visual", Caption: "caption", OverlayCopy: "产品信息", LayoutPreset: "proof_lower_left_v1"},
			{Order: 3, Role: "cta", Purpose: "cta", VisualBrief: "visual", Caption: "caption", OverlayCopy: "了解更多", LayoutPreset: "cta_bottom_v1"},
		},
	}
	if err := plan.Validate(); err == nil || !strings.Contains(err.Error(), "群体普适性表述") {
		t.Fatalf("Validate() error = %v, want group preference rejection", err)
	}
}

func TestImageTextReadinessBlocksUnlicensedGenerationAsset(t *testing.T) {
	t.Parallel()
	handoff, err := json.Marshal(map[string]any{
		"creative_view": map[string]any{
			"assets": []any{map[string]any{
				"asset_ref": map[string]any{"asset_id": "asset_product", "version": 1},
				"role":      "product_image",
				"rights": map[string]any{
					"status": "verified", "generative_ai_allowed": false,
					"derivative_work_allowed": true,
					"allowed_channels":        []string{"xiaohongshu"},
				},
			}},
		},
		"routes": []any{map[string]any{
			"route_id": "route_xhs",
			"asset_requirements": []any{map[string]any{
				"role": "product_image", "required_stage": "generation",
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	detail := TaskDetail{
		Task: CreativeTask{
			Format: FormatImageText, Channel: ChannelXiaohongshu,
			Direction: CreativeDirection{
				DirectionVersionID: "direction_1", InputIdentityHash: "sha256:input",
			},
		},
		Intake: CreativeIntake{
			ContractVersion: CreativeIntakeV3ContractVersion, InputIdentityHash: "sha256:input",
			Request: CreateIntakeRequest{
				SelectedRouteID: "route_xhs", StrategyHandoffInput: handoff,
				CreativeRoutes: []CreativeRouteSnapshot{{
					RouteID: "route_xhs", RouteType: CreativeRouteImageText,
					Channels: []string{"xiaohongshu"}, AspectRatio: "3:4",
					Reason: "selected", ReadinessStatus: "ready",
				}},
			},
		},
		Draft: ImageTextDraft{
			ContractVersion: ImageTextDraftV2Contract, InputIdentityHash: "sha256:input",
			ImagePlan: []ImagePlanItem{
				{Order: 1, VisualBrief: "cover", OverlayCopy: "cover"},
				{Order: 2, VisualBrief: "proof", OverlayCopy: "proof"},
				{Order: 3, VisualBrief: "cta", OverlayCopy: "cta"},
			},
		},
	}
	blockers := imageTextReadinessBlockers(detail, time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC))
	found := false
	for _, blocker := range blockers {
		if blocker == "source_asset_rights_blocked" {
			found = true
		}
	}
	if !found {
		t.Fatalf("blockers = %v, want source_asset_rights_blocked", blockers)
	}
}
