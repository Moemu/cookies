package creative

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestGenerateDirectionCandidatesStartsCreativeAuthorship(t *testing.T) {
	service := testService()
	intake := CreativeIntake{
		ContractVersion: "creative-intake/v3",
		ID:              "intake_v3", OrganizationID: "org_1", ProjectID: "project_1",
		Source: IntakeSourceStrategyPackage, Status: IntakeReady,
		InputIdentityHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Request: CreateIntakeRequest{
			Source: IntakeSourceStrategyPackage, SelectedRouteID: "route_xhs",
			Objective: "建立新品认知", Audience: "通勤女性", CoreMessage: "轻巧但可靠",
			Tone: []string{"可信"}, Mandatory: []string{"展示产品"}, Prohibited: []string{"绝对化承诺"},
			CreativeRoutes: []CreativeRouteSnapshot{{
				RouteID: "route_xhs", RouteType: "image_text", Channels: []string{"xiaohongshu"},
				Reason: "验证场景表达", AspectRatio: "3:4", ReadinessStatus: "ready",
			}},
		},
	}
	service.Repository.(*memoryRepository).intakes[intake.ID] = intake
	planner := &directionPlannerStub{result: DirectionPlannerResult{
		Model: "test-model", PromptVersion: "creative-direction/xhs-v1",
		Candidates: []DirectionCandidate{
			{Concept: "口袋里的可靠感", CreativeRationale: "把便携转成场景价值", MessagePlan: []string{"先场景后证据"}, ExecutionOutline: []string{"通勤包开场"}, GuardrailTrace: []string{"展示产品"}},
			{Concept: "忙碌时也不掉线", CreativeRationale: "回应通勤焦虑", MessagePlan: []string{"痛点到解决"}, ExecutionOutline: []string{"地铁切换"}, GuardrailTrace: []string{"避免绝对化承诺"}},
		},
	}}
	directions := &directionRepositoryStub{}
	service.DirectionPlanner = planner
	service.Directions = directions

	batch, err := service.GenerateDirectionCandidates(
		context.Background(), testRequestContext().Actor, "project_1", intake.ID,
		GenerateDirectionRequest{CandidateCount: 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Status != "ready" || len(batch.Candidates) != 2 ||
		batch.Candidates[0].Concept != "口袋里的可靠感" {
		t.Fatalf("unexpected candidate batch: %+v", batch)
	}
	if planner.context.Proposition != "轻巧但可靠" || planner.context.SelectedRoute.RouteID != "route_xhs" {
		t.Fatalf("planner did not receive the frozen planning context: %+v", planner.context)
	}
	confirmed, err := service.ConfirmDirection(
		context.Background(), testRequestContext().Actor, "project_1", batch.Candidates[0].ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.CreateTask(
		context.Background(), testRequestContext().Actor, "project_1", intake.ID,
		CreateTaskRequest{DirectionID: confirmed.ID, ContentType: ContentTypeCustom},
	)
	if err != nil {
		t.Fatal(err)
	}
	if task.Direction.Concept != confirmed.Concept ||
		task.Direction.DirectionVersionID != confirmed.ID ||
		task.Direction.InputIdentityHash != intake.InputIdentityHash {
		t.Fatalf("Creative task did not consume the confirmed direction: %+v", task.Direction)
	}
}

func TestGenerateDirectionCandidatesHasNoProviderFallback(t *testing.T) {
	service := testService()
	intake := CreativeIntake{
		ID: "intake_v3", OrganizationID: "org_1", ProjectID: "project_1",
		Source: IntakeSourceStrategyPackage, Status: IntakeReady,
		InputIdentityHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Request: CreateIntakeRequest{
			SelectedRouteID: "route_xhs",
			CreativeRoutes: []CreativeRouteSnapshot{{
				RouteID: "route_xhs", RouteType: "image_text", Channels: []string{"xiaohongshu"},
				Reason: "验证场景表达", AspectRatio: "3:4", ReadinessStatus: "ready",
			}},
		},
	}
	service.Repository.(*memoryRepository).intakes[intake.ID] = intake
	service.DirectionPlanner = &directionPlannerStub{err: errors.New("provider unavailable")}
	service.Directions = &directionRepositoryStub{}

	_, err := service.GenerateDirectionCandidates(
		context.Background(), testRequestContext().Actor, "project_1", intake.ID,
		GenerateDirectionRequest{CandidateCount: 2},
	)
	if err == nil {
		t.Fatal("expected provider failure instead of a fabricated fallback direction")
	}
}

func TestValidateDirectionCandidateClaimsRejectsUnsafeCopyButAllowsPerspective(t *testing.T) {
	t.Parallel()
	unsafe := DirectionCandidate{
		Concept: "夏季适配度第一", CreativeRationale: "大家都爱的神仙饮品",
		MessagePlan: []string{"先场景后产品"}, ExecutionOutline: []string{"通勤实拍"},
		GuardrailTrace: []string{"避免绝对化表达"},
	}
	if err := validateDirectionCandidateClaims(unsafe); err == nil {
		t.Fatal("unsafe customer-facing claims must be rejected")
	}
	safe := DirectionCandidate{
		Concept: "通勤清爽时刻", CreativeRationale: "用第一视角记录真实场景",
		MessagePlan: []string{"先场景后证据"}, ExecutionOutline: []string{"通勤实拍"},
		GuardrailTrace: []string{"避免绝对化表达"},
	}
	if err := validateDirectionCandidateClaims(safe); err != nil {
		t.Fatalf("first-person perspective is not a ranking claim: %v", err)
	}
}

func TestGenerateDirectionCandidatesSupportsFrozenBrandVideoRoute(t *testing.T) {
	service := testService()
	intake := CreativeIntake{
		ContractVersion: CreativeIntakeV3ContractVersion,
		ID:              "intake_brand_video", OrganizationID: "org_1", ProjectID: "project_1",
		Source: IntakeSourceStrategyPackage, Status: IntakeReady,
		InputIdentityHash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Request: CreateIntakeRequest{
			Source: IntakeSourceStrategyPackage, SelectedRouteID: "route_brand_video",
			Objective: "建立品牌认知", Audience: "研发负责人", CoreMessage: "精度可被验证",
			CreativeRoutes: []CreativeRouteSnapshot{{
				RouteID: "route_brand_video", RouteType: CreativeRouteBrandVideo,
				VideoPurpose: "brand", Channels: []string{"douyin"},
				Reason: "建立长期品牌记忆", TargetDurationSeconds: 30, AspectRatio: "16:9",
				RequiresHumanConfirmation: true, ReadinessStatus: "ready",
			}},
		},
	}
	service.Repository.(*memoryRepository).intakes[intake.ID] = intake
	service.DirectionPlanner = &directionPlannerStub{result: DirectionPlannerResult{
		Model: "test-model", PromptVersion: "creative-direction/brand-video-v1",
		Candidates: []DirectionCandidate{
			{Concept: "让精度被看见", CreativeRationale: "用工序证据建立信任", MessagePlan: []string{"承诺到证据"}, ExecutionOutline: []string{"工序与检测"}, GuardrailTrace: []string{"不虚构检测结论"}},
			{Concept: "每一微米都有来路", CreativeRationale: "把过程透明转为品牌记忆", MessagePlan: []string{"细节到全局"}, ExecutionOutline: []string{"微距到交付"}, GuardrailTrace: []string{"保护客户图纸"}},
		},
	}}
	service.Directions = &directionRepositoryStub{}

	batch, err := service.GenerateDirectionCandidates(
		context.Background(), testRequestContext().Actor, "project_1", intake.ID,
		GenerateDirectionRequest{CandidateCount: 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	if batch.Status != DirectionBatchReady || batch.Candidates[0].RouteID != "route_brand_video" {
		t.Fatalf("unexpected brand-video direction batch: %+v", batch)
	}
}

func TestV3StrategyIntakeSupportsBaseAndMatchingTaskOverlay(t *testing.T) {
	service := testService()
	handoffJSON := []byte(`{
		"creative_view":{
			"objective":{"statement":"建立认知"},
			"audience_segments":[{"segment_id":"audience_1","label":"通勤女性"}],
			"communication":{"single_minded_proposition":"可靠且便携","message_hierarchy":[{"priority":1,"message":"先场景"}]},
			"guardrails":[{"kind":"prohibited","text":"不得夸大"}],
			"claims":[{"claim_id":"claim_1","approved_text":"已批准事实"}],
			"assets":[{"asset_ref":{"asset_id":"asset_1","version":1}}],
			"creative_hypotheses":[{"hypothesis_id":"hypothesis_1","statement":"场景先行"}]
		},
		"routes":[{"route_id":"route_xhs","route_readiness":{"status":"ready"}}],
		"upstream_readiness":{"blockers":[],"warnings":[]}
	}`)
	service.StrategyPackages = strategyPackageReader{snapshot: StrategyPackageSnapshot{
		PackageID: "package_1", PackageVersion: 2, ContentHash: "sha256:package",
		HandoffContractVersion: "strategy-creative-handoff/v1",
		HandoffContentHash:     "sha256:handoff", CreativeReady: true,
		Objective: "建立认知", Audience: "通勤女性", CoreMessage: "可靠且便携",
		Tone: []string{}, Mandatory: []string{}, Prohibited: []string{},
		CreativeRoutes: []CreativeRouteSnapshot{{
			RouteID: "route_xhs", RouteType: "image_text", Channels: []string{"xiaohongshu"},
			Reason: "验证场景表达", AspectRatio: "3:4", ReadinessStatus: "ready",
		}},
		HandoffSnapshot: handoffJSON,
	}}
	service.TaskOverlays = taskOverlayReaderStub{snapshot: TaskOverlaySnapshot{
		ContractVersion: TaskOverlayContractVersion, OverlayID: "overlay_1",
		ContentHash: "sha256:overlay",
		PackageRef: StrategyPackageReference{
			PackageID: "package_1", PackageVersion: 2, ExpectedContentHash: "sha256:package",
			HandoffContractVersion: "strategy-creative-handoff/v1",
			ExpectedHandoffHash:    "sha256:handoff",
		},
		SelectedRouteID: "route_xhs", MessagePriorities: []string{"先场景后证据"},
		StrategyDimensions: map[string]any{"content_angle": "通勤场景"},
		RawSnapshot:        []byte(`{"contract_version":"strategy-creative-task-overlay/v1"}`),
	}}
	intake, err := service.CreateIntake(
		context.Background(), testRequestContext(), "project_1", "v3-intake",
		CreateIntakeRequest{
			ContractVersion: CreativeIntakeCreateV3ContractVersion,
			Source:          IntakeSourceStrategyPackage,
			StrategyPackageRef: &StrategyPackageContractReference{
				PackageID: "package_1", PackageVersion: 2, PackageContentHash: "sha256:package",
				HandoffContractVersion: "strategy-creative-handoff/v1",
				HandoffContentHash:     "sha256:handoff",
			},
			SelectedRouteID: "route_xhs",
			TaskOverlayRef: &TaskOverlayContractReference{
				OverlayID: "overlay_1", ContentHash: "sha256:overlay",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if intake.ContractVersion != "creative-intake/v3" || intake.InputIdentityHash == "" ||
		intake.Request.Concept != "" || len(intake.Request.VisualKeywords) != 0 ||
		intake.Request.TaskOverlayInput == nil {
		t.Fatalf("unexpected v3 intake: %+v", intake)
	}
	if _, err := intake.V3View(); err != nil {
		t.Fatalf("project v3 snapshot: %v", err)
	}
	contextValue, err := planningContextFromIntake(intake, intake.Request.CreativeRoutes[0])
	if err != nil || len(contextValue.Claims) != 1 || len(contextValue.Assets) != 1 ||
		len(contextValue.Hypotheses) != 1 {
		t.Fatalf("planning context lost formal Handoff facts: context=%+v err=%v", contextValue, err)
	}
}

func TestLegacyTaskStrategyIntakeCreationCanBeDisabled(t *testing.T) {
	service := testService()
	service.AllowLegacyTaskStrategyIntakeWrites = false
	_, err := service.CreateIntake(
		context.Background(), testRequestContext(), "project_1", "legacy-task-strategy",
		CreateIntakeRequest{
			Source: IntakeSourceTaskStrategy,
			TaskStrategy: &TaskStrategyReference{
				PlanID: "plan_1", StrategyVersion: 1, ExpectedContentHash: "sha256:strategy",
			},
		},
	)
	if err == nil {
		t.Fatal("expected legacy task_strategy writes to be rejected")
	}
}

type directionPlannerStub struct {
	context CreativePlanningContext
	result  DirectionPlannerResult
	err     error
}

func (p *directionPlannerStub) Generate(_ context.Context, _ contract.ActorContext, _ contract.ProjectContext, value CreativePlanningContext, _ int) (DirectionPlannerResult, error) {
	p.context = value
	return p.result, p.err
}

type directionRepositoryStub struct {
	batch CreativeDirectionBatch
}

type taskOverlayReaderStub struct {
	snapshot TaskOverlaySnapshot
	err      error
}

func (r taskOverlayReaderStub) ReadTaskOverlayForCreative(
	_ context.Context,
	_ contract.ActorContext,
	_ contract.ProjectID,
	reference TaskOverlayReference,
) (TaskOverlaySnapshot, error) {
	if r.err != nil {
		return TaskOverlaySnapshot{}, r.err
	}
	if r.snapshot.OverlayID != reference.OverlayID ||
		r.snapshot.ContentHash != reference.ExpectedContentHash {
		return TaskOverlaySnapshot{}, errors.New("task overlay reference mismatch")
	}
	return r.snapshot, nil
}

func (r *directionRepositoryStub) CreateDirectionBatch(_ context.Context, value CreativeDirectionBatch) (CreativeDirectionBatch, error) {
	r.batch = value
	return value, nil
}

func (r *directionRepositoryStub) GetDirection(
	_ context.Context,
	_ contract.OrganizationID,
	_ contract.ProjectID,
	directionID string,
) (CreativeDirectionVersion, error) {
	for _, candidate := range r.batch.Candidates {
		if candidate.ID == directionID {
			return candidate, nil
		}
	}
	return CreativeDirectionVersion{}, ErrNotFound
}

func (r *directionRepositoryStub) ConfirmDirection(
	_ context.Context,
	_ contract.OrganizationID,
	_ contract.ProjectID,
	directionID string,
	confirmedBy string,
	confirmedAt time.Time,
) (CreativeDirectionVersion, error) {
	for index, candidate := range r.batch.Candidates {
		if candidate.ID == directionID {
			candidate.Status = "confirmed"
			candidate.ConfirmedBy = confirmedBy
			candidate.ConfirmedAt = &confirmedAt
			r.batch.Candidates[index] = candidate
			return candidate, nil
		}
	}
	return CreativeDirectionVersion{}, ErrNotFound
}
