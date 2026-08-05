package creative

import (
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestCommercePrerollPlannerBuildsConfirmedGuerlainWindowReveal(t *testing.T) {
	planner := CommercePrerollPlanner{}
	plan, err := planner.Plan(CommercePrerollPlanningInput{
		TaskID:             "creativetask_guerlain_01",
		IntakeVersion:      1,
		TemplateID:         CommerceWindowRevealTemplateID,
		TemplateVersion:    1,
		BrandName:          "法国娇兰",
		ProductName:        "娇兰第三代黄金复原蜜",
		ProductAsset:       contract.AssetVersionRef{AssetID: "asset_guerlain_packshot", Version: 1},
		DurationSeconds:    6,
		AspectRatio:        "9:16",
		Resolution:         "720p",
		AudioPolicy:        VideoAudioSilent,
		MandatoryElements:  []string{"保持产品外观、颜色、包装文字和蜜蜂标识真实"},
		ProhibitedElements: []string{"不改变包装", "不新增商品", "不出现人物正脸"},
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}

	if plan.Template.ID != CommerceWindowRevealTemplateID || plan.Template.Version != 1 {
		t.Fatalf("unexpected template reference: %+v", plan.Template)
	}
	if plan.FramePlan.ProductAsset != (contract.AssetVersionRef{AssetID: "asset_guerlain_packshot", Version: 1}) {
		t.Fatalf("unexpected product asset: %+v", plan.FramePlan.ProductAsset)
	}
	if len(plan.Prompt.Timeline) != 3 {
		t.Fatalf("timeline length = %d, want 3", len(plan.Prompt.Timeline))
	}
	wantStages := []TimelinePurpose{TimelineInformationGap, TimelineSingleTransformation, TimelineProductHold}
	for index, want := range wantStages {
		if plan.Prompt.Timeline[index].Purpose != want {
			t.Fatalf("timeline[%d].purpose = %q, want %q", index, plan.Prompt.Timeline[index].Purpose, want)
		}
	}
	for _, fragment := range []string{
		"娇兰第三代黄金复原蜜",
		"0.0–1.5 秒",
		"1.5–4.0 秒",
		"4.0–6.0 秒",
		"戴手套的手",
		"包装文字和蜜蜂标识",
	} {
		if !strings.Contains(plan.Prompt.CompiledPrompt, fragment) {
			t.Fatalf("compiled prompt does not contain %q:\n%s", fragment, plan.Prompt.CompiledPrompt)
		}
	}
	for _, forbidden := range []string{"立即购买", "限时优惠"} {
		if strings.Contains(plan.Prompt.CompiledPrompt, forbidden) {
			t.Fatalf("compiled prompt unexpectedly contains %q", forbidden)
		}
	}
	if !strings.HasPrefix(plan.Prompt.Hash, "sha256:") || len(plan.Prompt.Hash) != len("sha256:")+64 {
		t.Fatalf("prompt hash = %q, want canonical sha256", plan.Prompt.Hash)
	}
	if plan.Spec.DurationSeconds != 6 || plan.Spec.AspectRatio != "9:16" || plan.Spec.Resolution != "720p" ||
		plan.Spec.AudioPolicy != VideoAudioSilent || plan.Spec.CandidateCount != 1 {
		t.Fatalf("unexpected generation spec: %+v", plan.Spec)
	}
	if plan.Spec.GenerationReady {
		t.Fatal("generation spec is ready before conditioned frame assets exist")
	}
	if plan.Spec.ProductionReady {
		t.Fatal("production spec is ready before the main video exists")
	}
}

func TestCommercePrerollGenerationApprovalBindsExactFramesAndPrompt(t *testing.T) {
	plan, err := (CommercePrerollPlanner{}).Plan(CommercePrerollPlanningInput{
		TaskID:             "creativetask_guerlain_01",
		IntakeVersion:      1,
		TemplateID:         CommerceWindowRevealTemplateID,
		TemplateVersion:    1,
		BrandName:          "法国娇兰",
		ProductName:        "娇兰第三代黄金复原蜜",
		ProductAsset:       contract.AssetVersionRef{AssetID: "asset_guerlain_packshot", Version: 1},
		DurationSeconds:    6,
		AspectRatio:        "9:16",
		Resolution:         "720p",
		AudioPolicy:        VideoAudioSilent,
		MandatoryElements:  []string{"保持商品真实"},
		ProhibitedElements: []string{"不新增商品"},
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	spec, err := plan.BindFrames(ConditionedFrames{
		StartFrame: contract.AssetVersionRef{AssetID: "asset_start", Version: 1},
		TailFrame:  contract.AssetVersionRef{AssetID: "asset_tail", Version: 1},
	})
	if err != nil {
		t.Fatalf("BindFrames() error = %v", err)
	}
	if !spec.GenerationReady || spec.ProductionReady {
		t.Fatalf("readiness generation=%v production=%v, want true/false", spec.GenerationReady, spec.ProductionReady)
	}
	if len(spec.ConditioningAssets) != 2 ||
		spec.ConditioningAssets[0].Role != VideoConditioningFirstFrame ||
		spec.ConditioningAssets[1].Role != VideoConditioningLastFrame {
		t.Fatalf("conditioning assets = %+v", spec.ConditioningAssets)
	}
	if !strings.HasPrefix(spec.Hash, "sha256:") || len(spec.Hash) != len("sha256:")+64 {
		t.Fatalf("generation spec hash = %q", spec.Hash)
	}

	now := time.Date(2026, 7, 28, 9, 0, 0, 0, time.UTC)
	approval, err := ApproveVideoGeneration(spec, "principal_1", now)
	if err != nil {
		t.Fatalf("ApproveVideoGeneration() error = %v", err)
	}
	if err := approval.Authorizes(spec); err != nil {
		t.Fatalf("approval does not authorize unchanged spec: %v", err)
	}
	changed := spec
	changed.ConditioningAssets = append([]VideoConditioningAsset{}, spec.ConditioningAssets...)
	changed.ConditioningAssets[1].AssetRef.Version = 2
	changed.Hash = ""
	if err := changed.Seal(); err != nil {
		t.Fatalf("Seal() changed spec error = %v", err)
	}
	if err := approval.Authorizes(changed); err == nil {
		t.Fatal("approval authorizes a changed tail-frame version")
	}
	repeated := approval
	repeated.ConfirmedItems = []string{"product", "product", "product", "product", "product", "product"}
	if err := repeated.Authorizes(spec); err == nil {
		t.Fatal("approval accepts repeated confirmation items")
	}
	sameFrames := spec
	sameFrames.ConditioningAssets[1].AssetRef = sameFrames.ConditioningAssets[0].AssetRef
	sameFrames.Hash = ""
	if err := sameFrames.Seal(); err == nil {
		t.Fatal("generation spec accepts one asset version as both first and last frame")
	}
}

func TestCommercePrerollPlannerCompilesAllFiveTemplatesFromProductFacts(t *testing.T) {
	t.Parallel()
	templates := []string{
		CommerceProductCutTemplateID,
		CommerceWindowRevealTemplateID,
		CommerceOneClickTemplateID,
		CommerceMiniatureTemplateID,
		CommerceDeviceSummonTemplateID,
	}
	for _, templateID := range templates {
		templateID := templateID
		t.Run(templateID, func(t *testing.T) {
			t.Parallel()
			plan, err := (CommercePrerollPlanner{}).Plan(CommercePrerollPlanningInput{
				TaskID: "commerce_preview", IntakeVersion: 3,
				TemplateID: templateID, TemplateVersion: 1,
				BrandName: "Example Brand", ProductName: "Example Serum",
				ProductCategory: "skincare", SellingPoints: []string{"approved hydration"},
				VisualKeywords:  []string{"clean", "gold"},
				ProductAsset:    contract.AssetVersionRef{AssetID: "asset_product", Version: 2},
				DurationSeconds: 6, AspectRatio: "9:16", Resolution: "720p",
				AudioPolicy:        VideoAudioSilent,
				MandatoryElements:  []string{"show the front label"},
				ProhibitedElements: []string{"no medical claims"},
			})
			if err != nil {
				t.Fatalf("Plan() error = %v", err)
			}
			for _, fragment := range []string{
				"Example Brand", "Example Serum", "approved hydration",
				"show the front label", "no medical claims",
			} {
				if !strings.Contains(plan.Prompt.CompiledPrompt, fragment) {
					t.Fatalf("compiled prompt for %s does not contain %q:\n%s", templateID, fragment, plan.Prompt.CompiledPrompt)
				}
			}
			if plan.Template.ID != templateID || len(plan.Prompt.Timeline) != 3 ||
				plan.FramePlan.StartFrameKind == "" || plan.FramePlan.TailFrameKind == "" {
				t.Fatalf("incomplete plan for %s: %+v", templateID, plan)
			}
		})
	}
}

func TestCommercePrerollPlannerAllowsPromptPreviewBeforeProductImage(t *testing.T) {
	t.Parallel()
	plan, err := (CommercePrerollPlanner{}).Plan(CommercePrerollPlanningInput{
		TaskID: "commerce_preview", IntakeVersion: 1,
		TemplateID: CommerceWindowRevealTemplateID, TemplateVersion: 1,
		BrandName: "Example Brand", ProductName: "Example Product",
		DurationSeconds: 6, AspectRatio: "9:16", Resolution: "720p",
		AudioPolicy: VideoAudioSilent,
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if plan.Prompt.ProductAsset != (contract.AssetVersionRef{}) {
		t.Fatalf("preview product asset = %+v, want empty", plan.Prompt.ProductAsset)
	}
	if plan.Spec.GenerationReady {
		t.Fatal("preview without a product image is generation ready")
	}
}
