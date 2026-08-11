package creative

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestBrandBriefReviewProjectsFrozenFactsAndConfirmsCreativeInputs(t *testing.T) {
	service := testService()
	intake := brandBriefTestIntake(t)
	service.Repository.(*memoryRepository).intakes[intake.ID] = intake
	repository := &brandBriefRepositoryStub{}
	service.BrandBriefs = repository
	service.StrategyPackages = strategyPackageReader{snapshot: brandBriefStrategyPackageSnapshot()}
	actor := testRequestContext().Actor

	prepared, err := service.PrepareBrandBriefReview(context.Background(), actor, intake.ProjectID, intake.ID)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Status != BrandBriefDraft || prepared.Revision != 1 || prepared.ContentHash == "" {
		t.Fatalf("unexpected prepared review: %+v", prepared)
	}
	if len(prepared.Blockers) != 0 || prepared.Document.Product.BrandName != "Kanon" ||
		prepared.Document.Product.ProductName != "研发协作平台" || len(prepared.Document.Product.UsageScenarios) != 1 {
		t.Fatalf("Strategy package facts were not reused by Brand Brief: %+v", prepared)
	}
	replayed, err := service.PrepareBrandBriefReview(context.Background(), actor, intake.ProjectID, intake.ID)
	if err != nil || replayed.ContentHash != prepared.ContentHash || replayed.Revision != prepared.Revision {
		t.Fatalf("prepare is not refresh-safe: review=%+v err=%v", replayed, err)
	}

	document := prepared.Document
	// These fields belong to the frozen handoff and must not be editable here.
	document.Route.RouteID = "tampered-route"
	document.Claims = nil
	document.Assets = nil
	updated, err := service.UpdateBrandBriefReview(context.Background(), actor, intake.ProjectID, intake.ID, UpdateBrandBriefReviewRequest{
		ExpectedRevision: prepared.Revision,
		Document:         document,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Document.Route.RouteID != intake.Request.SelectedRouteID || len(updated.Document.Claims) != 1 || len(updated.Document.Assets) != 1 {
		t.Fatalf("frozen lineage was changed by Creative edit: %+v", updated.Document)
	}
	if len(updated.Blockers) != 0 {
		t.Fatalf("complete Brief still has blockers: %v", updated.Blockers)
	}

	confirmed, err := service.ConfirmBrandBriefReview(context.Background(), actor, intake.ProjectID, intake.ID, ConfirmBrandBriefReviewRequest{ExpectedRevision: updated.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Status != BrandBriefConfirmed || confirmed.Revision != updated.Revision+1 || confirmed.ConfirmedAt == nil {
		t.Fatalf("Brief was not confirmed with an immutable revision: %+v", confirmed)
	}
	if _, err = service.UpdateBrandBriefReview(context.Background(), actor, intake.ProjectID, intake.ID, UpdateBrandBriefReviewRequest{ExpectedRevision: confirmed.Revision, Document: confirmed.Document}); err != ErrVersionConflict {
		t.Fatalf("confirmed Brief remained mutable: %v", err)
	}
}

func TestUpgradeProjectedBrandBriefFactsUsesCanonicalPackageWithoutOverwritingUserEdits(t *testing.T) {
	legacy := BrandBriefDocument{Product: BrandBriefProduct{SellingPoints: []string{"护肤"}}}
	canonical := BrandBriefDocument{Product: BrandBriefProduct{
		BrandName: "法国娇兰", ProductName: "娇兰第三代黄金复原蜜", SellingPoints: []string{"护肤"},
	}}
	untouched := legacy
	untouched.Product.BrandName = "娇兰"
	untouched.Product.ProductName = "黄金蜜"
	upgradeProjectedBrandBriefFacts(&untouched, legacy, canonical)
	if untouched.Product.BrandName != "法国娇兰" || untouched.Product.ProductName != "娇兰第三代黄金复原蜜" {
		t.Fatalf("untouched legacy facts were not upgraded: %#v", untouched.Product)
	}

	edited := legacy
	edited.Product.BrandName = "GUERLAIN（用户确认）"
	edited.Product.ProductName = "黄金蜜"
	upgradeProjectedBrandBriefFacts(&edited, legacy, canonical)
	if edited.Product.BrandName != "GUERLAIN（用户确认）" {
		t.Fatalf("user edit was overwritten: %#v", edited.Product)
	}
	if edited.Product.ProductName != "娇兰第三代黄金复原蜜" {
		t.Fatalf("untouched product name was not upgraded: %#v", edited.Product)
	}
}

func TestBrandDirectionRequiresConfirmedCurrentBrandBrief(t *testing.T) {
	service := testService()
	intake := brandBriefTestIntake(t)
	service.Repository.(*memoryRepository).intakes[intake.ID] = intake
	repository := &brandBriefRepositoryStub{}
	service.BrandBriefs = repository
	service.StrategyPackages = strategyPackageReader{snapshot: brandBriefStrategyPackageSnapshot()}
	actor := testRequestContext().Actor

	if _, err := service.prepareDirectionPlanning(context.Background(), actor, intake.ProjectID, intake.ID, GenerateDirectionRequest{CandidateCount: 3}); err == nil || !strings.Contains(err.Error(), "confirm the brand Brief") {
		t.Fatalf("direction planning did not block missing Brand Brief confirmation: %v", err)
	}
	prepared, err := service.PrepareBrandBriefReview(context.Background(), actor, intake.ProjectID, intake.ID)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.UpdateBrandBriefReview(context.Background(), actor, intake.ProjectID, intake.ID, UpdateBrandBriefReviewRequest{ExpectedRevision: prepared.Revision, Document: prepared.Document})
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := service.ConfirmBrandBriefReview(context.Background(), actor, intake.ProjectID, intake.ID, ConfirmBrandBriefReviewRequest{ExpectedRevision: updated.Revision})
	if err != nil {
		t.Fatal(err)
	}
	planning, err := service.prepareDirectionPlanning(context.Background(), actor, intake.ProjectID, intake.ID, GenerateDirectionRequest{CandidateCount: 3})
	if err != nil {
		t.Fatal(err)
	}
	if planning.BrandBriefRef == nil || planning.BrandBriefRef.Revision != confirmed.Revision || planning.PlanningContext.BrandBrief == nil {
		t.Fatalf("confirmed Brand Brief lineage did not reach planning: %+v", planning)
	}
}

func brandBriefTestIntake(t *testing.T) CreativeIntake {
	t.Helper()
	handoff, err := json.Marshal(map[string]any{
		"creative_view": map[string]any{
			"market": "CN", "language": "zh-CN",
			"objective":         map[string]any{"objective_type": "brand_awareness", "statement": "建立可信的品牌认知", "success_signals": []string{"用户能复述核心主张"}},
			"audience_segments": []any{map[string]any{"segment_id": "audience_1", "label": "研发负责人", "priority": 1, "insight": "重复协调挤压关键判断", "tension": "既要效率，也怕失去控制", "evidence_ref_ids": []string{"evidence_1"}}},
			"product_and_offer": map[string]any{"product_ref_ids": []string{"product_1"}},
			"communication":     map[string]any{"single_minded_proposition": "把重复工作交给自动化，人只处理关键判断", "message_hierarchy": []any{map[string]any{"priority": 1, "message": "关键判断仍由人负责", "evidence_ref_ids": []string{"evidence_1"}}}, "cta_intent": "brand_recall", "approved_ctas": []string{}, "tone_constraints": []string{"克制", "可信"}},
			"guardrails":        []any{map[string]any{"guardrail_id": "guard_1", "kind": "prohibited", "severity": "blocking", "scope": "all", "text": "不得承诺绝对效率", "source_ref_ids": []string{"source_1"}}},
			"claims":            []any{map[string]any{"claim_id": "claim_1", "approved_text": "关键操作可追溯", "paraphrase_policy": "exact", "evidence_ref_ids": []string{"evidence_1"}, "required_disclaimer": "", "validity": map[string]any{"markets": []string{"CN"}, "channels": []string{"douyin"}}}},
			"assets":            []any{map[string]any{"asset_ref": map[string]any{"asset_id": "logo_1", "version": 1}, "role": "brand_logo", "rights": map[string]any{"status": "verified", "generative_ai_allowed": true, "derivative_work_allowed": true, "allowed_channels": []string{"douyin"}, "territories": []string{"CN"}, "valid_until": nil}}},
			"open_questions":    []any{}, "source_refs": []any{map[string]any{"ref_id": "source_1", "ref_type": "brief", "producer": "strategy", "resource_uri": "strategy://package/1", "version": "1", "content_hash": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "observed_at": "2026-08-06T00:00:00Z"}},
		},
		"routes": []any{map[string]any{"route_id": "route_brand", "deliverable_type": "video", "purpose": "brand", "performance_mode": CreativeRouteBrandVideo, "channels": []string{"douyin"}, "reason": "建立品牌记忆", "spec": map[string]any{"target_duration_seconds": 30, "aspect_ratio": "16:9", "resolution": "1080p", "composition_required": true}, "cta_policy": map[string]any{"required_for_generation": false, "required_for_delivery": false, "cta_intent": "brand_recall"}, "claim_refs": []string{"claim_1"}, "asset_requirements": []any{map[string]any{"role": "brand_logo", "required_stage": "generation"}}, "asset_refs": []string{"logo_1"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return CreativeIntake{
		ContractVersion: CreativeIntakeV3ContractVersion, ID: "intake_brand_brief", OrganizationID: "org_1", ProjectID: "project_1",
		Source: IntakeSourceStrategyPackage, Status: IntakeReady,
		InputIdentityHash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Request: CreateIntakeRequest{Source: IntakeSourceStrategyPackage,
			StrategyPackage: &StrategyPackageReference{PackageID: "package_1", PackageVersion: 1, ExpectedContentHash: "sha256:package", HandoffContractVersion: "strategy-creative-handoff/v1", ExpectedHandoffHash: "sha256:handoff"},
			SelectedRouteID: "route_brand", StrategyHandoffInput: handoff, CoreMessage: "把重复工作交给自动化，人只处理关键判断", CreativeRoutes: []CreativeRouteSnapshot{{RouteID: "route_brand", RouteType: CreativeRouteBrandVideo, Channels: []string{"douyin"}, VideoPurpose: "brand", Reason: "建立品牌记忆", TargetDurationSeconds: 30, AspectRatio: "16:9", ReadinessStatus: "ready"}}},
	}
}

func brandBriefStrategyPackageSnapshot() StrategyPackageSnapshot {
	return StrategyPackageSnapshot{
		PackageID: "package_1", PackageVersion: 1, ContentHash: "sha256:package",
		HandoffContractVersion: "strategy-creative-handoff/v1", HandoffContentHash: "sha256:handoff",
		CreativeReady: true, Objective: "建立可信的品牌认知", Audience: "研发负责人",
		CoreMessage: "把重复工作交给自动化，人只处理关键判断",
		BrandName:   "Kanon", ProductName: "研发协作平台",
		SellingPoints: []string{"关键操作可追溯"}, ProofPoints: []string{"evidence_1"},
		UsageScenarios: []string{"复杂研发决策复核"},
	}
}

func brandBriefBool(value bool) *bool { return &value }

func containsBrandBriefBlocker(values []string, text string) bool {
	for _, value := range values {
		if strings.Contains(value, text) {
			return true
		}
	}
	return false
}
