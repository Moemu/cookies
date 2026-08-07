package creative

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

func TestDirectionGenerationPersistsBeforeBackgroundExecutionAndRecovers(t *testing.T) {
	service := testService()
	intake := CreativeIntake{
		ContractVersion: CreativeIntakeV3ContractVersion,
		ID:              "intake_async", OrganizationID: "org_1", ProjectID: "project_1",
		Source: IntakeSourceStrategyPackage, Status: IntakeReady,
		InputIdentityHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Request: CreateIntakeRequest{SelectedRouteID: "route_brand", CoreMessage: "让关键判断回到人手中", CreativeRoutes: []CreativeRouteSnapshot{{
			RouteID: "route_brand", RouteType: CreativeRouteBrandVideo, Channels: []string{"brand_film"}, Reason: "品牌认知", ReadinessStatus: "ready",
		}}},
	}
	service.Repository.(*memoryRepository).intakes[intake.ID] = intake
	service.BrandBriefs = confirmedBrandBriefRepository(intake)
	repository := &directionRepositoryStub{}
	scheduler := &directionGenerationSchedulerStub{}
	service.Directions = repository
	service.DirectionScheduler = scheduler
	planner := &directionPlannerStub{result: DirectionPlannerResult{
		Model: "test-model", PromptVersion: "direction-test-v1", Candidates: []DirectionCandidate{
			{Concept: "判断被交还的瞬间", CreativeRationale: "以人物终于能专注判断为情绪落点", MessagePlan: []string{"先重复工作，后关键判断"}, ExecutionOutline: []string{"机械重复与人物抬头形成反差"}, GuardrailTrace: []string{"不承诺绝对效率"}, DirectionMode: "emotional", EmotionalArc: "压迫到释然", VisualGrammar: "重复蒙太奇", BrandMemoryDevice: "抬头一刻", HumanMoment: "同事交换确认眼神"},
			{Concept: "留白给重要的事", CreativeRationale: "通过空间留白建立品牌秩序感", MessagePlan: []string{"先噪声，后留白"}, ExecutionOutline: []string{"密集界面逐步退场"}, GuardrailTrace: []string{"不虚构功能指标"}, DirectionMode: "cinematic", EmotionalArc: "拥挤到从容", VisualGrammar: "宽银幕留白", BrandMemoryDevice: "空出的桌面", HumanMoment: "人物放下手中清单"},
		},
	}}
	service.DirectionPlanner = planner
	actor := testRequestContext().Actor
	batch, err := service.StartDirectionGeneration(context.Background(), actor, "project_1", intake.ID, GenerateDirectionRequest{CandidateCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	if batch.Status != DirectionBatchGenerating || len(batch.Candidates) != 0 || scheduler.calls != 1 {
		t.Fatalf("generation was not durably started: batch=%+v calls=%d", batch, scheduler.calls)
	}
	replayed, err := service.StartDirectionGeneration(context.Background(), actor, "project_1", intake.ID, GenerateDirectionRequest{CandidateCount: 2})
	if err != nil || replayed.ID != batch.ID || scheduler.calls != 1 {
		t.Fatalf("active generation was not deduplicated: batch=%+v calls=%d err=%v", replayed, scheduler.calls, err)
	}
	payload, err := json.Marshal(scheduler.operation)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.HandleDirectionGenerationJob(context.Background(), jobruntime.Claim{Job: contract.Job{
		Kind: DirectionGenerationJobKind, OrganizationID: actor.OrganizationID, ProjectID: "project_1",
	}, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := service.GetLatestDirectionBatch(context.Background(), actor, "project_1", intake.ID)
	if err != nil || restored.Status != DirectionBatchReady || len(restored.Candidates) != 2 {
		t.Fatalf("completed batch was not recoverable: batch=%+v err=%v", restored, err)
	}
	if planner.context.GenerationID != batch.ID {
		t.Fatalf("generation batch ID was not bound to the model invocation context: %+v", planner.context)
	}
}

func TestDirectionGenerationPersistsFailureForRefresh(t *testing.T) {
	service := testService()
	intake := CreativeIntake{ContractVersion: CreativeIntakeV3ContractVersion, ID: "intake_failed", OrganizationID: "org_1", ProjectID: "project_1", Source: IntakeSourceStrategyPackage, Status: IntakeReady,
		InputIdentityHash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Request:           CreateIntakeRequest{SelectedRouteID: "route_xhs", CreativeRoutes: []CreativeRouteSnapshot{{RouteID: "route_xhs", RouteType: CreativeRouteImageText, Channels: []string{"xiaohongshu"}, Reason: "内容", ReadinessStatus: "ready"}}}}
	service.Repository.(*memoryRepository).intakes[intake.ID] = intake
	repository := &directionRepositoryStub{}
	scheduler := &directionGenerationSchedulerStub{}
	service.Directions = repository
	service.DirectionScheduler = scheduler
	service.DirectionPlanner = &directionPlannerStub{err: errors.New("provider timeout")}
	actor := testRequestContext().Actor
	batch, err := service.StartDirectionGeneration(context.Background(), actor, "project_1", intake.ID, GenerateDirectionRequest{CandidateCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(scheduler.operation)
	_, err = service.HandleDirectionGenerationJob(context.Background(), jobruntime.Claim{Job: contract.Job{Kind: DirectionGenerationJobKind, OrganizationID: actor.OrganizationID, ProjectID: "project_1"}, Payload: payload})
	if err == nil {
		t.Fatal("expected background generation failure")
	}
	restored, getErr := service.GetLatestDirectionBatch(context.Background(), actor, "project_1", intake.ID)
	if getErr != nil || restored.ID != batch.ID || restored.Status != DirectionBatchFailed || restored.FailureCode != "DIRECTION_PROVIDER_FAILED" {
		t.Fatalf("failed batch was not recoverable: batch=%+v err=%v", restored, getErr)
	}
}

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

func TestManualV3ImageTextIntakeStartsDirectionPlanning(t *testing.T) {
	service := testService()
	planner := &directionPlannerStub{result: DirectionPlannerResult{
		Model: "test-model", PromptVersion: "creative-direction/xhs-v1",
		Candidates: []DirectionCandidate{
			{Concept: "通勤场景记录", CreativeRationale: "从用户输入的通勤场景展开", MessagePlan: []string{"先场景后信息"}, ExecutionOutline: []string{"通勤插画"}, GuardrailTrace: []string{"不补造产品功效"}},
			{Concept: "三类场景清单", CreativeRationale: "把用户输入整理成场景清单", MessagePlan: []string{"场景分组"}, ExecutionOutline: []string{"三栏信息图"}, GuardrailTrace: []string{"不虚构用户体验"}},
		},
	}}
	service.DirectionPlanner = planner
	service.Directions = &directionRepositoryStub{}

	intake, err := service.CreateIntake(
		context.Background(), testRequestContext(), "project_1", "manual-v3-image-text",
		CreateIntakeRequest{
			ContractVersion: CreativeIntakeCreateV3ContractVersion,
			Source:          IntakeSourceManual, Channel: ChannelXiaohongshu,
			Objective: "建立新品认知", Audience: "关注通勤饮品的年轻用户",
			CoreMessage:  "0糖青柠气泡水适合用场景化内容介绍",
			CallToAction: "搜索品牌了解更多", Tone: []string{"清爽", "克制"},
			VisualKeywords: []string{"青柠绿", "生活方式"},
			Mandatory:      []string{},
			Prohibited:     []string{"不得虚构产品功效"},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if intake.ContractVersion != CreativeIntakeV3ContractVersion || intake.InputIdentityHash == "" ||
		intake.Request.SelectedRouteID != ManualImageTextRouteID || len(intake.Request.CreativeRoutes) != 1 {
		t.Fatalf("manual intake did not freeze v3 planning lineage: %+v", intake)
	}
	view, err := intake.V3View()
	if err != nil || !view.Readiness.PlanningReady || !view.Readiness.GenerationReady {
		t.Fatalf("manual intake did not expose a ready v3 view: view=%+v err=%v", view, err)
	}

	batch, err := service.GenerateDirectionCandidates(
		context.Background(), testRequestContext().Actor, "project_1", intake.ID,
		GenerateDirectionRequest{CandidateCount: 2},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Candidates) != 2 || planner.context.SelectedRoute.RouteID != ManualImageTextRouteID ||
		planner.context.Proposition != intake.Request.CoreMessage {
		t.Fatalf("manual planning context was not preserved: batch=%+v context=%+v", batch, planner.context)
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

func TestValidateDirectionBatchQualityRejectsUtilityDisguisedAsEmotionalBrandDirection(t *testing.T) {
	t.Parallel()
	candidates := []DirectionCandidate{
		{Concept: "焦虑消解后的风险核验清单", CreativeRationale: "以人物故事承载清单", DirectionMode: "emotional", MessagePlan: []string{"先人物后清单"}, ExecutionOutline: []string{"人物打开清单"}, GuardrailTrace: []string{"不虚构"}, EmotionalArc: "从焦虑到安心", VisualGrammar: "克制近景", BrandMemoryDevice: "蓝色校准线", HumanMoment: "工程师回传标注"},
		{Concept: "毫米之间，有人回答", CreativeRationale: "以人物接力建立工程伙伴认知", DirectionMode: "cinematic", MessagePlan: []string{"问题被接住"}, ExecutionOutline: []string{"动作匹配剪辑"}, GuardrailTrace: []string{"不虚构"}, EmotionalArc: "从悬而未决到获得回应", VisualGrammar: "工业微距", BrandMemoryDevice: "银色光带", HumanMoment: "隔屏共同确认"},
		{Concept: "供应商判断工具", CreativeRationale: "提供参考框架", DirectionMode: "utility", MessagePlan: []string{"三个步骤"}, ExecutionOutline: []string{"信息卡片"}, GuardrailTrace: []string{"不虚构"}, EmotionalArc: "从迷茫到清晰", VisualGrammar: "卡片排版", BrandMemoryDevice: "蓝色印记", HumanMoment: "采购做出标注"},
	}
	err := validateDirectionBatchQuality(CreativePlanningContext{
		SelectedRoute: CreativeRouteSnapshot{RouteType: CreativeRouteBrandVideo},
	}, candidates)
	if err == nil {
		t.Fatal("utility-led emotional disguise must make the brand batch fail")
	}
}

func TestValidateDirectionBatchQualityRejectsPerformanceCTAInBrandVideo(t *testing.T) {
	t.Parallel()
	candidates := []DirectionCandidate{
		{Concept: "图纸沉默之后", CreativeRationale: "让焦虑被工程回应", DirectionMode: "emotional", MessagePlan: []string{"焦虑到笃定"}, ExecutionOutline: []string{"结尾评论区领取核验表"}, GuardrailTrace: []string{"不虚构"}, EmotionalArc: "从焦虑到笃定", VisualGrammar: "低照度长镜头", BrandMemoryDevice: "蓝色校准线", HumanMoment: "工程师回传标注"},
		{Concept: "毫米之间，有人回答", CreativeRationale: "以人物接力建立伙伴认知", DirectionMode: "cinematic", MessagePlan: []string{"问题被接住"}, ExecutionOutline: []string{"动作匹配剪辑"}, GuardrailTrace: []string{"不虚构"}, EmotionalArc: "从未知到确认", VisualGrammar: "工业微距", BrandMemoryDevice: "银色光带", HumanMoment: "隔屏共同确认"},
	}
	err := validateDirectionBatchQuality(CreativePlanningContext{SelectedRoute: CreativeRouteSnapshot{RouteType: CreativeRouteBrandVideo}}, candidates)
	if err == nil {
		t.Fatal("performance CTA must make the brand batch fail")
	}
}

func TestBrandQualityAllowsProductToolLanguageAndNarrativeClick(t *testing.T) {
	t.Parallel()
	candidate := DirectionCandidate{
		Concept: "把重复工作交给自动化", CreativeRationale: "让效率工具退到幕后，把关键判断还给人",
		DirectionMode: "emotional", MessagePlan: []string{"重复操作退场，人的判断出现"},
		ExecutionOutline: []string{"人物点击确认按钮后抬头，与同事交换眼神"}, GuardrailTrace: []string{"不承诺绝对效率"},
		EmotionalArc: "从机械疲惫到重新专注", VisualGrammar: "重复蒙太奇转为克制长镜头",
		BrandMemoryDevice: "确认声后的一秒静默", HumanMoment: "操作者点击确认后把注意力转向同事",
	}
	if isUtilityLedDirection(candidate) {
		t.Fatal("describing the product as an efficiency tool must not become utility-led content")
	}
	if cue := firstBrandPerformanceCue(candidate); cue != "" {
		t.Fatalf("a character clicking a work control is narrative action, not a performance CTA: %s", cue)
	}
	candidate.ExecutionOutline = []string{"结尾点击了解更多并立即咨询"}
	if cue := firstBrandPerformanceCue(candidate); cue != "点击了解" {
		t.Fatalf("an audience-facing click CTA must still be rejected: %s", cue)
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
	service.BrandBriefs = confirmedBrandBriefRepository(intake)
	service.DirectionPlanner = &directionPlannerStub{result: DirectionPlannerResult{
		Model: "test-model", PromptVersion: "creative-direction/brand-video-v1",
		Candidates: []DirectionCandidate{
			{Concept: "让精度被看见", CreativeRationale: "用工序证据建立信任", MessagePlan: []string{"承诺到证据"}, ExecutionOutline: []string{"工序与检测"}, GuardrailTrace: []string{"不虚构检测结论"}, DirectionMode: "emotional", EmotionalArc: "从忐忑到笃定", VisualGrammar: "微距与长镜头", BrandMemoryDevice: "蓝色校准线", HumanMoment: "工程师确认图纸"},
			{Concept: "每一微米都有来路", CreativeRationale: "把过程透明转为品牌记忆", MessagePlan: []string{"细节到全局"}, ExecutionOutline: []string{"微距到交付"}, GuardrailTrace: []string{"保护客户图纸"}, DirectionMode: "cinematic", EmotionalArc: "从未知到可见", VisualGrammar: "银色光带转场", BrandMemoryDevice: "清脆归零声", HumanMoment: "采购收到确认"},
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

type brandBriefRepositoryStub struct {
	review BrandBriefReview
}

func confirmedBrandBriefRepository(intake CreativeIntake) *brandBriefRepositoryStub {
	return &brandBriefRepositoryStub{review: BrandBriefReview{
		ContractVersion: BrandBriefReviewV1, OrganizationID: intake.OrganizationID, ProjectID: intake.ProjectID,
		IntakeID: intake.ID, InputIdentityHash: intake.InputIdentityHash, Status: BrandBriefConfirmed,
		Revision: 2, ContentHash: "sha256:confirmed-brand-brief",
		Document: BrandBriefDocument{Communication: BrandBriefCommunication{SingleMindedProposition: intake.Request.CoreMessage}},
	}}
}

func (r *brandBriefRepositoryStub) CreateBrandBrief(_ context.Context, value BrandBriefReview) (BrandBriefReview, bool, error) {
	if r.review.IntakeID != "" {
		return r.review, true, nil
	}
	r.review = value
	return value, false, nil
}

func (r *brandBriefRepositoryStub) GetBrandBrief(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, intakeID string) (BrandBriefReview, error) {
	if r.review.IntakeID != intakeID {
		return BrandBriefReview{}, ErrNotFound
	}
	return r.review, nil
}

func (r *brandBriefRepositoryStub) UpdateBrandBrief(_ context.Context, value BrandBriefReview, expectedRevision int64) (BrandBriefReview, error) {
	if r.review.Revision != expectedRevision {
		return BrandBriefReview{}, ErrVersionConflict
	}
	value.Revision++
	r.review = value
	return value, nil
}

func (r *brandBriefRepositoryStub) ConfirmBrandBrief(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, intakeID string, expectedRevision int64, confirmedBy string, confirmedAt time.Time) (BrandBriefReview, error) {
	if r.review.IntakeID != intakeID || r.review.Revision != expectedRevision {
		return BrandBriefReview{}, ErrVersionConflict
	}
	r.review.Status = BrandBriefConfirmed
	r.review.Revision++
	r.review.ConfirmedBy = confirmedBy
	r.review.ConfirmedAt = &confirmedAt
	return r.review, nil
}

func (p *directionPlannerStub) Generate(_ context.Context, _ contract.ActorContext, _ contract.ProjectContext, value CreativePlanningContext, _ int) (DirectionPlannerResult, error) {
	p.context = value
	return p.result, p.err
}

type directionRepositoryStub struct {
	batch CreativeDirectionBatch
}

type directionGenerationSchedulerStub struct {
	operation DirectionGenerationOperation
	calls     int
}

func (s *directionGenerationSchedulerStub) ScheduleDirectionGeneration(
	_ context.Context,
	_ contract.ProjectID,
	operation DirectionGenerationOperation,
) error {
	s.calls++
	s.operation = operation
	return nil
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

func (r *directionRepositoryStub) GetDirectionBatch(
	_ context.Context,
	_ contract.OrganizationID,
	_ contract.ProjectID,
	batchID string,
) (CreativeDirectionBatch, error) {
	if r.batch.ID != batchID {
		return CreativeDirectionBatch{}, ErrNotFound
	}
	return r.batch, nil
}

func (r *directionRepositoryStub) GetLatestDirectionBatch(
	_ context.Context,
	_ contract.OrganizationID,
	_ contract.ProjectID,
	intakeID string,
) (CreativeDirectionBatch, error) {
	if r.batch.IntakeID != intakeID {
		return CreativeDirectionBatch{}, ErrNotFound
	}
	return r.batch, nil
}

func (r *directionRepositoryStub) CompleteDirectionBatch(_ context.Context, value CreativeDirectionBatch) (CreativeDirectionBatch, error) {
	value.Status = DirectionBatchReady
	r.batch = value
	return value, nil
}

func (r *directionRepositoryStub) FailDirectionBatch(
	_ context.Context,
	_ contract.OrganizationID,
	_ contract.ProjectID,
	batchID string,
	failureCode string,
) error {
	if r.batch.ID != batchID {
		return ErrNotFound
	}
	r.batch.Status = DirectionBatchFailed
	r.batch.FailureCode = failureCode
	return nil
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
