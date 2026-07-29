package creative

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/media"
)

func TestManualIntakeNeedsClarificationBeforeCreatingTask(t *testing.T) {
	t.Parallel()
	service := testService()
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-1", CreateIntakeRequest{
		Source: IntakeSourceManual, Channel: ChannelXiaohongshu, Tone: []string{}, VisualKeywords: []string{}, Mandatory: []string{}, Prohibited: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if intake.Status != IntakeNeedsClarification || len(intake.MissingFields) != 3 {
		t.Fatalf("intake = %#v", intake)
	}
	if _, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, defaultTaskRequest()); !errors.Is(err, ErrIntakeNotReady) {
		t.Fatalf("error = %v, want %v", err, ErrIntakeNotReady)
	}
}

func TestManualReadyIntakeCreatesImageTextTaskAndDraft(t *testing.T) {
	t.Parallel()
	service := testService()
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-2", validManualRequest())
	if err != nil {
		t.Fatal(err)
	}
	if intake.Status != IntakeReady || intake.ConfirmedBy != "usr_1" {
		t.Fatalf("intake = %#v", intake)
	}
	task, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, defaultTaskRequest())
	if err != nil {
		t.Fatal(err)
	}
	if task.Format != FormatImageText || task.Channel != ChannelXiaohongshu {
		t.Fatalf("task = %#v", task)
	}
	detail, err := service.GetTaskDetail(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Draft.TitleCandidates) != 3 || len(detail.Draft.ImagePlan) != 4 || detail.Draft.CoverCopy == "" {
		t.Fatalf("draft = %#v", detail.Draft)
	}
}

func TestListVersionsAndPackagesRestoresDeliveredCreativeAfterRefresh(t *testing.T) {
	t.Parallel()
	service := testService()
	repository := service.Repository.(*memoryRepository)
	now := time.Date(2026, time.July, 24, 10, 0, 0, 0, time.UTC)
	version := CreativeVersion{
		ID: "creativeversion_1", OrganizationID: "org_1", ProjectID: "project_1",
		TaskID: "creativetask_1", Version: 2, DraftVersion: 2,
		Status: CreativeVersionApproved, CreatedAt: now,
	}
	pkg := CreativePackage{
		ID: "creativepackage_1", OrganizationID: "org_1", ProjectID: "project_1",
		CreativeVersionID: version.ID, CreatedAt: now,
	}
	repository.versions[version.ID] = version
	repository.packages[pkg.ID] = pkg

	versions, err := service.ListVersions(context.Background(), testRequestContext().Actor, "project_1", "creativetask_1", 20)
	if err != nil {
		t.Fatal(err)
	}
	packages, err := service.ListPackages(context.Background(), testRequestContext().Actor, "project_1", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 || versions[0].ID != version.ID || len(packages) != 1 || packages[0].ID != pkg.ID {
		t.Fatalf("versions=%#v packages=%#v", versions, packages)
	}
}

func TestApprovedStrategyPackageCreatesReadyCreativeIntake(t *testing.T) {
	t.Parallel()
	service := testService()
	service.StrategyPackages = strategyPackageReader{snapshot: StrategyPackageSnapshot{
		PackageID: "package_1", PackageVersion: 2, ContentHash: "sha256:package", CreativeReady: true,
		Objective: "建立新品认知", Audience: "关注生活方式的上班族", CoreMessage: "一杯咖啡也可以成为从容开始的仪式",
		Concept: "温暖晨光中的咖啡桌", Tone: []string{"自然"}, VisualKeywords: []string{"晨光"}, Mandatory: []string{}, Prohibited: []string{},
	}}
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-strategy", CreateIntakeRequest{
		Source: IntakeSourceStrategyPackage, StrategyPackage: &StrategyPackageReference{PackageID: "package_1", PackageVersion: 2, ExpectedContentHash: "sha256:package"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if intake.Source != IntakeSourceStrategyPackage || intake.Status != IntakeReady || intake.Request.Objective == "" {
		t.Fatalf("intake = %#v", intake)
	}
	if _, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, defaultTaskRequest()); err != nil {
		t.Fatalf("strategy intake did not create Creative task: %v", err)
	}
}

func TestCreateStrategyIntakeDeduplicatesTheSamePackageVersion(t *testing.T) {
	t.Parallel()
	service := testService()
	service.StrategyPackages = strategyPackageReader{snapshot: StrategyPackageSnapshot{PackageID: "package_1", PackageVersion: 1, ContentHash: "hash", CreativeReady: true, Objective: "目标", Audience: "受众", CoreMessage: "主张", Concept: "概念", Tone: []string{}, VisualKeywords: []string{}, Mandatory: []string{}, Prohibited: []string{}}}
	rc := testRequestContext()
	request := CreateIntakeRequest{Source: IntakeSourceStrategyPackage, StrategyPackage: &StrategyPackageReference{PackageID: "package_1", PackageVersion: 1, ExpectedContentHash: "hash"}}
	first, err := service.CreateIntake(context.Background(), rc, "project_1", "strategy-intake-first", request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateIntake(context.Background(), rc, "project_1", "strategy-intake-second", request)
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("same strategy package should return its existing Intake: first=%q second=%q", first.ID, second.ID)
	}
}

func TestStrategyPackageWithoutCreativeReadinessNeedsClarification(t *testing.T) {
	t.Parallel()
	service := testService()
	service.StrategyPackages = strategyPackageReader{snapshot: StrategyPackageSnapshot{CreativeReady: false, Objective: "目标", Audience: "受众", CoreMessage: "主张", Concept: "概念", Tone: []string{}, VisualKeywords: []string{}, Mandatory: []string{}, Prohibited: []string{}}}
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-not-ready", CreateIntakeRequest{Source: IntakeSourceStrategyPackage, StrategyPackage: &StrategyPackageReference{PackageID: "package_2", PackageVersion: 1, ExpectedContentHash: "hash"}})
	if err != nil {
		t.Fatal(err)
	}
	if intake.Status != IntakeNeedsClarification || len(intake.MissingFields) != 1 || intake.MissingFields[0] != "strategy_package.creative_ready" {
		t.Fatalf("intake = %#v", intake)
	}
}

func TestIntakeIdempotencyDoesNotCreateAnotherIntake(t *testing.T) {
	t.Parallel()
	service := testService()
	rc := testRequestContext()
	first, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-3", validManualRequest())
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-3", validManualRequest())
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("idempotency IDs = %q and %q", first.ID, second.ID)
	}
}

func TestTaskCreationAllowsSeveralDistinctDirectionsForTheSameIntake(t *testing.T) {
	t.Parallel()
	service := testService()
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-4", validManualRequest())
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, defaultTaskRequest())
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, CreateTaskRequest{ContentType: ContentTypeIngredientExplanation, Focus: "成分解释"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || first.Direction.ContentType == second.Direction.ContentType {
		t.Fatalf("task directions were not created separately: first=%#v second=%#v", first, second)
	}
}

func TestArchiveTaskHidesItFromActiveQueueButRetainsItsLineage(t *testing.T) {
	t.Parallel()
	service := testService()
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-intake-archive", validManualRequest())
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, defaultTaskRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RegisterCoverImageJob(context.Background(), rc.Actor, "project_1", task.ID, "provider_job_1"); err != nil {
		t.Fatal(err)
	}
	if err := service.ArchiveTask(context.Background(), rc.Actor, "project_1", task.ID); err != nil {
		t.Fatal(err)
	}
	active, err := service.ListTasks(context.Background(), rc.Actor, "project_1", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("active tasks = %#v, want archived task omitted", active)
	}
	detail, err := service.GetTaskDetail(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Task.Status != TaskArchived || len(detail.ProductionJobs) != 1 || detail.Draft.TaskID != task.ID {
		t.Fatalf("archived detail should retain lineage: %#v", detail)
	}
}

func TestImagePlanRetriesRetainEachProviderAttempt(t *testing.T) {
	t.Parallel()
	service := testService()
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "creative-image-retry-intake", validManualRequest())
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.CreateTask(context.Background(), rc.Actor, "project_1", intake.ID, defaultTaskRequest())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RegisterImagePlanJob(context.Background(), rc.Actor, "project_1", task.ID, 2, "provider_job_first"); err != nil {
		t.Fatal(err)
	}
	if err := service.RegisterImagePlanJob(context.Background(), rc.Actor, "project_1", task.ID, 2, "provider_job_retry"); err != nil {
		t.Fatal(err)
	}
	detail, err := service.GetTaskDetail(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.ProductionJobs) != 2 || detail.ProductionJobs[0].Kind == detail.ProductionJobs[1].Kind {
		t.Fatalf("production retries = %#v", detail.ProductionJobs)
	}
}

func TestCreateVideoTaskConsumesApprovedRouteAndReadyProjectVideo(t *testing.T) {
	t.Parallel()
	service := testService()
	service.StrategyPackages = strategyPackageReader{snapshot: StrategyPackageSnapshot{
		PackageID: "package_1", PackageVersion: 1, ContentHash: "hash_1", CreativeReady: true,
		Objective: "转化", Audience: "短剧观众", CoreMessage: "五秒建立产品利益点", Concept: "先看利益点再进正片",
		Tone: []string{"直接"}, VisualKeywords: []string{"竖屏"}, Mandatory: []string{}, Prohibited: []string{},
		CreativeRoutes: []CreativeRouteSnapshot{{
			RouteType: "pre_roll", VideoPurpose: "performance", Channels: []string{"douyin"},
			Reason: "正片前快速建立关联", TargetDurationSeconds: 5, AspectRatio: "9:16",
			SourceAssetRefs: []contract.AssetVersionRef{}, EvidenceRefs: []string{}, RequiresHumanConfirmation: true,
		}},
	}}
	service.Assets = testAssetReader{snapshot: CreativeAssetSnapshot{
		Ref: contract.AssetVersionRef{AssetID: "asset_main", Version: 1}, Kind: contract.AssetVideo, MIMEType: "video/mp4", Ready: true,
	}}
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "video-intake-1", CreateIntakeRequest{
		Source:          IntakeSourceStrategyPackage,
		StrategyPackage: &StrategyPackageReference{PackageID: "package_1", PackageVersion: 1, ExpectedContentHash: "hash_1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.CreateVideoTask(context.Background(), rc.Actor, "project_1", intake.ID, CreateVideoTaskRequest{
		RouteIndex: 0, Channel: ChannelDouyin,
		SourceVideo: contract.AssetVersionRef{AssetID: "asset_main", Version: 1},
		Concept:     "利益点前贴", Prompt: "竖屏五秒产品利益点广告", CallToAction: "继续观看",
		Mandatory: []string{}, Prohibited: []string{}, ConfirmRoute: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := service.GetTaskDetail(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if task.Format != FormatVideo || task.PerformanceMode != "pre_roll" || detail.VideoDraft == nil ||
		detail.VideoDraft.SourceVideo.AssetID != "asset_main" || detail.VideoDraft.DurationSeconds != 5 {
		t.Fatalf("unexpected video task detail: %+v", detail)
	}
}

func TestCreateManualViralRemakeTaskUsesStableRouteAndRestorableSnapshot(t *testing.T) {
	t.Parallel()
	service := testService()
	service.Assets = testAssetReader{snapshots: map[contract.AssetID]CreativeAssetSnapshot{
		"asset_reference_video": {
			Ref:  contract.AssetVersionRef{AssetID: "asset_reference_video", Version: 2},
			Kind: contract.AssetVideo, MIMEType: "video/mp4", Ready: true,
		},
		"asset_product_image": {
			Ref:  contract.AssetVersionRef{AssetID: "asset_product_image", Version: 1},
			Kind: contract.AssetImage, MIMEType: "image/png", Ready: true,
		},
	}}
	rc := testRequestContext()
	referenceImage := contract.AssetVersionRef{AssetID: "asset_product_image", Version: 1}
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "manual-viral-intake-1", CreateIntakeRequest{
		Source: IntakeSourceManual, Format: FormatVideo, PerformanceMode: PerformanceModeViralRemake,
		Channel: ChannelDouyin, Objective: "复用高停留结构，生成原创转化广告", Audience: "效率工具用户",
		CoreMessage: "减少重复操作", CallToAction: "立即体验", Concept: "保留功能节奏并替换受保护表达",
		Tone: []string{"清晰"}, VisualKeywords: []string{"高反差开场"}, Mandatory: []string{}, Prohibited: []string{},
		CreativeRoutes: []CreativeRouteSnapshot{{
			RouteID: ManualViralRemakeRouteID, RouteType: PerformanceModeViralRemake,
			VideoPurpose: "performance", Channels: []string{"douyin"}, Reason: "用户选择爆款复刻",
			TargetDurationSeconds: 15, AspectRatio: "9:16", RequiresHumanConfirmation: true,
		}},
		ManualViralRemake: &ManualViralRemakeInput{
			ProductName: "FlowKit", SellingPoints: []string{"自动整理任务", "减少重复操作"},
			UserInstruction: "保留钩子功能和节奏，替换人物、品牌、字幕和音乐",
			ReferenceVideo:  contract.AssetVersionRef{AssetID: "asset_reference_video", Version: 2},
			ReferenceImage:  &referenceImage, ReferenceVideoRights: RightsConfirmed,
			ReferenceImageRights: RightsConfirmed,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	task, err := service.CreateVideoTask(context.Background(), rc.Actor, "project_1", intake.ID, CreateVideoTaskRequest{
		SelectedRouteID: ManualViralRemakeRouteID, Channel: ChannelDouyin,
		SourceVideo: contract.AssetVersionRef{AssetID: "asset_reference_video", Version: 2},
		Concept:     "原创效率工具广告", Prompt: "等待 Phase 2 真实拆解后生成", CallToAction: "立即体验",
		Mandatory: []string{}, Prohibited: []string{}, ConfirmRoute: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := service.GetTaskDetail(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if task.PerformanceMode != PerformanceModeViralRemake || detail.VideoDraft == nil ||
		detail.VideoDraft.ViralRemake == nil {
		t.Fatalf("viral workspace was not persisted: %+v", detail)
	}
	viral := detail.VideoDraft.ViralRemake
	if viral.SelectedRouteID != ManualViralRemakeRouteID || viral.Revision != 1 ||
		viral.InputSnapshot.ReferenceVideo != (contract.AssetVersionRef{AssetID: "asset_reference_video", Version: 2}) {
		t.Fatalf("viral input snapshot = %+v", viral)
	}
	if !viral.Readiness.PlanningReady || viral.Readiness.GenerationReady || viral.Readiness.ProductionReady {
		t.Fatalf("viral readiness = %+v", viral.Readiness)
	}
}

func TestRenderJobPersistsFinalAssetLineage(t *testing.T) {
	t.Parallel()
	service := testService()
	repository := service.Repository.(*memoryRepository)
	now := service.now()
	intake := CreativeIntake{
		ID: "intake_video", OrganizationID: "org_1", ProjectID: "project_1", Status: IntakeReady,
		Request: CreateIntakeRequest{
			StrategyPackage: &StrategyPackageReference{PackageID: "strategy_package_1", PackageVersion: 1, ExpectedContentHash: "sha256:strategy"},
			CreativeRoutes: []CreativeRouteSnapshot{{
				RouteType: "pre_roll", VideoPurpose: "performance", Channels: []string{"douyin"}, Reason: "approved",
				TargetDurationSeconds: 5, AspectRatio: "9:16", RequiresHumanConfirmation: true,
			}},
		},
	}
	repository.intakes[intake.ID] = intake
	task := CreativeTask{
		ID: "task_video", OrganizationID: "org_1", ProjectID: "project_1", IntakeID: intake.ID,
		Format: FormatVideo, Channel: ChannelDouyin, VideoPurpose: "performance", PerformanceMode: "pre_roll",
		Status: TaskDraft, Version: 1, CreatedAt: now, UpdatedAt: now,
	}
	draft := VideoDraft{
		ContractVersion: "creative-video-draft/v1", TaskID: task.ID, Revision: 1, Concept: "concept", Prompt: "prompt",
		DurationSeconds: 5, AspectRatio: "9:16", Resolution: "720p",
		SourceVideo: contract.AssetVersionRef{AssetID: "asset_main", Version: 1},
		Mandatory:   []string{}, Prohibited: []string{}, CreatedAt: now,
	}
	if _, err := repository.CreateVideoTask(context.Background(), task, draft); err != nil {
		t.Fatal(err)
	}
	taskDetail := repository.tasks[task.ID]
	taskDetail.ProductionJobs = []ProductionJob{{TaskID: task.ID, Kind: "video_generate", ProviderJobID: "providerjob_video_1", CreatedAt: now}}
	repository.tasks[task.ID] = taskDetail
	service.Assets = testAssetReader{snapshot: CreativeAssetSnapshot{
		Ref: contract.AssetVersionRef{AssetID: "asset_preroll", Version: 1}, Kind: contract.AssetVideo, MIMEType: "video/mp4", Ready: true,
	}}
	scheduler := &testRenderScheduler{}
	writer := &testRenderedAssetWriter{ref: contract.ProjectAssetRef{
		ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "asset_final", Version: 1},
	}}
	service.RenderScheduler = scheduler
	service.Composer = testVideoComposer{}
	service.RenderedAssets = writer
	rc := testRequestContext()
	render, _, err := service.CreateRenderJob(context.Background(), rc, "project_1", task.ID, CreateRenderJobRequest{
		PreRollVideo: contract.AssetVersionRef{AssetID: "asset_preroll", Version: 1},
	}, "render-once")
	if err != nil {
		t.Fatal(err)
	}
	if scheduler.render.ID != render.ID {
		t.Fatalf("render was not durably scheduled: %+v", scheduler.render)
	}
	if err := service.ExecuteRenderJob(context.Background(), "org_1", "project_1", render.ID); err != nil {
		t.Fatal(err)
	}
	stored, err := repository.GetRenderJob(context.Background(), "org_1", "project_1", render.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != RenderSucceeded || stored.OutputAsset == nil || stored.OutputAsset.AssetVersion.AssetID != "asset_final" || writer.renderJobID != render.ID {
		t.Fatalf("render lineage is incomplete: render=%+v writer=%+v", stored, writer)
	}
	version, _, err := service.FreezeVersion(context.Background(), rc, "project_1", task.ID, FreezeVersionRequest{
		DraftVersion: 1, RenderJobID: render.ID,
	}, "freeze-video-once")
	if err != nil {
		t.Fatal(err)
	}
	if version.VideoSnapshot == nil || version.VideoSnapshot.FinalVideo.AssetID != "asset_final" ||
		version.VideoSnapshot.ProviderJobID != "providerjob_video_1" {
		t.Fatalf("video version lineage is incomplete: %+v", version)
	}
	checked, err := service.CheckVersion(context.Background(), rc.Actor, "project_1", version.ID)
	if err != nil || checked.Check == nil || !checked.Check.Passed {
		t.Fatalf("video check failed: version=%+v err=%v", checked, err)
	}
	if _, err := service.ApproveVersion(context.Background(), rc.Actor, "project_1", version.ID); err != nil {
		t.Fatal(err)
	}
	pkg, err := service.DeliverVersion(context.Background(), rc.Actor, "project_1", version.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Format != FormatVideo || pkg.VideoSnapshot == nil || pkg.VideoSnapshot.RenderJobID != render.ID {
		t.Fatalf("delivered package lost video lineage: %+v", pkg)
	}
}

func validManualRequest() CreateIntakeRequest {
	return CreateIntakeRequest{
		Source: IntakeSourceManual, Channel: ChannelXiaohongshu, Objective: "建立新品认知", Audience: "关注生活方式的年轻上班族", CoreMessage: "一杯咖啡，也可以成为从容开始的仪式", CallToAction: "收藏这份晨间灵感",
		Concept: "柔和自然光下的蓝白咖啡桌", Tone: []string{"自然", "克制"}, VisualKeywords: []string{"蓝白", "晨光"}, Mandatory: []string{"产品主体"}, Prohibited: []string{},
	}
}

func defaultTaskRequest() CreateTaskRequest {
	return CreateTaskRequest{ContentType: ContentTypeLifestyle, Focus: "生活方式种草"}
}

func testRequestContext() contract.RequestContext {
	return contract.RequestContext{RequestID: "req_1", TraceID: "trace_1", Actor: contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}, Scopes: []contract.Scope{ScopeRead, ScopeWrite}}}
}

func testService() Service {
	sequence := 0
	return Service{
		Repository: &memoryRepository{
			intakes: map[string]CreativeIntake{}, tasks: map[string]TaskDetail{}, renders: map[string]RenderJob{},
			versions: map[string]CreativeVersion{}, packages: map[string]CreativePackage{},
		},
		Projects: testProjects{},
		Now:      func() time.Time { return time.Date(2026, time.July, 23, 1, 0, 0, 0, time.UTC) },
		NewID: func(prefix string) (string, error) {
			sequence++
			return fmt.Sprintf("%s_%d", prefix, sequence), nil
		},
	}
}

type testProjects struct{}

type strategyPackageReader struct {
	snapshot StrategyPackageSnapshot
}

type testAssetReader struct {
	snapshot  CreativeAssetSnapshot
	snapshots map[contract.AssetID]CreativeAssetSnapshot
	err       error
}

type testRenderScheduler struct{ render RenderJob }

func (s *testRenderScheduler) ScheduleRender(_ context.Context, render RenderJob) error {
	s.render = render
	return nil
}

type testVideoComposer struct{}

func (testVideoComposer) ComposePreRoll(context.Context, media.PreRollCompositionRequest) (media.CompositionOutput, error) {
	return media.CompositionOutput{
		Content: io.NopCloser(bytes.NewReader([]byte("rendered-video"))), SizeBytes: 14,
		Metadata: assets.VideoMetadata{DurationMS: 1000, WidthPixels: 720, HeightPixels: 1280, FrameRate: "25/1", VideoCodec: "h264"},
	}, nil
}

type testRenderedAssetWriter struct {
	ref         contract.ProjectAssetRef
	renderJobID string
}

func (w *testRenderedAssetWriter) IngestRenderedVideo(_ context.Context, _ contract.RequestContext, _ contract.ProjectID, renderJobID string, _ io.Reader, _ int64) (contract.ProjectAssetRef, error) {
	w.renderJobID = renderJobID
	return w.ref, nil
}

func (r testAssetReader) ReadForCreative(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, ref contract.AssetVersionRef) (CreativeAssetSnapshot, error) {
	if r.snapshots != nil {
		value, ok := r.snapshots[ref.AssetID]
		if !ok {
			return CreativeAssetSnapshot{}, ErrNotFound
		}
		return value, r.err
	}
	return r.snapshot, r.err
}

func (r strategyPackageReader) ReadForCreative(_ context.Context, _ contract.ActorContext, _ contract.ProjectID, reference StrategyPackageReference) (StrategyPackageSnapshot, error) {
	value := r.snapshot
	if value.PackageID == "" {
		value.PackageID, value.PackageVersion, value.ContentHash = reference.PackageID, reference.PackageVersion, reference.ExpectedContentHash
	}
	if value.ContentHash != reference.ExpectedContentHash {
		return StrategyPackageSnapshot{}, fmt.Errorf("hash mismatch")
	}
	return value, nil
}

func (testProjects) RequireActiveContext(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectContext, error) {
	brand := contract.BrandID("brand_1")
	return contract.ProjectContext{OrganizationID: actor.OrganizationID, ProjectID: projectID, BrandID: &brand, ProductIDs: []contract.ProductID{}, ProjectContextVersion: 1}, nil
}

type memoryRepository struct {
	intakes  map[string]CreativeIntake
	tasks    map[string]TaskDetail
	renders  map[string]RenderJob
	versions map[string]CreativeVersion
	packages map[string]CreativePackage
}

func (r *memoryRepository) CreateIntake(_ context.Context, intake CreativeIntake) (CreativeIntake, bool, error) {
	for _, existing := range r.intakes {
		if existing.IdempotencyKey == intake.IdempotencyKey && existing.Principal == intake.Principal && existing.ProjectID == intake.ProjectID {
			if existing.RequestHash != intake.RequestHash {
				return CreativeIntake{}, false, ErrIdempotencyConflict
			}
			return existing, true, nil
		}
		if intake.Source == IntakeSourceStrategyPackage && existing.Source == IntakeSourceStrategyPackage && sameStrategyPackage(existing.Request.StrategyPackage, intake.Request.StrategyPackage) {
			return existing, true, nil
		}
	}
	r.intakes[intake.ID] = intake
	return intake, false, nil
}

func sameStrategyPackage(left, right *StrategyPackageReference) bool {
	return left != nil && right != nil && left.PackageID == right.PackageID && left.PackageVersion == right.PackageVersion && left.ExpectedContentHash == right.ExpectedContentHash
}
func (r *memoryRepository) ListIntakes(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ int) ([]CreativeIntake, error) {
	values := make([]CreativeIntake, 0, len(r.intakes))
	for _, value := range r.intakes {
		values = append(values, value)
	}
	return values, nil
}
func (r *memoryRepository) GetIntake(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string) (CreativeIntake, error) {
	value, ok := r.intakes[id]
	if !ok {
		return CreativeIntake{}, ErrNotFound
	}
	return value, nil
}

func (r *memoryRepository) CreateTask(_ context.Context, task CreativeTask, draft ImageTextDraft) (CreativeTask, error) {
	r.tasks[task.ID] = TaskDetail{Task: task, Intake: r.intakes[task.IntakeID], Draft: draft, ProductionJobs: []ProductionJob{}}
	return task, nil
}
func (r *memoryRepository) CreateVideoTask(_ context.Context, task CreativeTask, draft VideoDraft) (CreativeTask, error) {
	value := draft
	r.tasks[task.ID] = TaskDetail{Task: task, Intake: r.intakes[task.IntakeID], VideoDraft: &value, ProductionJobs: []ProductionJob{}}
	return task, nil
}
func (r *memoryRepository) CreateRenderJob(_ context.Context, value RenderJob) (RenderJob, bool, error) {
	for _, existing := range r.renders {
		if existing.IdempotencyKey == value.IdempotencyKey && existing.CreatedBy == value.CreatedBy && existing.ProjectID == value.ProjectID {
			if existing.RequestHash != value.RequestHash {
				return RenderJob{}, false, ErrIdempotencyConflict
			}
			return existing, true, nil
		}
	}
	r.renders[value.ID] = value
	return value, false, nil
}
func (r *memoryRepository) GetRenderJob(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string) (RenderJob, error) {
	value, ok := r.renders[id]
	if !ok {
		return RenderJob{}, ErrNotFound
	}
	return value, nil
}
func (r *memoryRepository) MarkRenderRunning(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string, now time.Time) (RenderJob, error) {
	value := r.renders[id]
	value.Status, value.UpdatedAt = RenderRunning, now
	r.renders[id] = value
	return value, nil
}
func (r *memoryRepository) CompleteRenderJob(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string, ref contract.ProjectAssetRef, now time.Time) error {
	value := r.renders[id]
	value.Status, value.OutputAsset, value.UpdatedAt = RenderSucceeded, &ref, now
	r.renders[id] = value
	return nil
}
func (r *memoryRepository) FailRenderJob(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id, code, message string, now time.Time) error {
	value := r.renders[id]
	value.Status, value.ErrorCode, value.ErrorMessage, value.UpdatedAt = RenderFailed, code, message, now
	r.renders[id] = value
	return nil
}
func (r *memoryRepository) ListTasks(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ int) ([]CreativeTask, error) {
	values := make([]CreativeTask, 0, len(r.tasks))
	for _, value := range r.tasks {
		if value.Task.Status == TaskArchived {
			continue
		}
		values = append(values, value.Task)
	}
	return values, nil
}
func (r *memoryRepository) GetTaskDetail(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string) (TaskDetail, error) {
	value, ok := r.tasks[id]
	if !ok {
		return TaskDetail{}, ErrNotFound
	}
	return value, nil
}
func (r *memoryRepository) ArchiveTask(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, taskID string, now time.Time) error {
	value, ok := r.tasks[taskID]
	if !ok {
		return ErrNotFound
	}
	if value.Task.Status == TaskArchived {
		return ErrInvalidState
	}
	value.Task.Status = TaskArchived
	value.Task.UpdatedAt = now
	r.tasks[taskID] = value
	return nil
}
func (r *memoryRepository) ReviseDraft(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, taskID string, expectedVersion int64, draft ImageTextDraft) (ImageTextDraft, error) {
	value, ok := r.tasks[taskID]
	if !ok {
		return ImageTextDraft{}, ErrNotFound
	}
	if value.Draft.Version != expectedVersion || value.Task.Version != expectedVersion {
		return ImageTextDraft{}, ErrVersionConflict
	}
	value.Draft = draft
	value.Task.Version = draft.Version
	value.Task.UpdatedAt = draft.CreatedAt
	r.tasks[taskID] = value
	return draft, nil
}
func (r *memoryRepository) ReviseVideoDraft(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, taskID string, expectedRevision int64, draft VideoDraft, status TaskStatus) (VideoDraft, error) {
	value, ok := r.tasks[taskID]
	if !ok {
		return VideoDraft{}, ErrNotFound
	}
	if value.VideoDraft == nil || value.VideoDraft.Revision != expectedRevision || draft.Revision != expectedRevision+1 {
		return VideoDraft{}, ErrVersionConflict
	}
	value.VideoDraft = &draft
	value.Task.Status = status
	value.Task.Version++
	value.Task.UpdatedAt = draft.CreatedAt
	r.tasks[taskID] = value
	return draft, nil
}
func (r *memoryRepository) RegisterProductionJob(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, taskID string, job ProductionJob) error {
	value, ok := r.tasks[taskID]
	if !ok {
		return ErrNotFound
	}
	for _, existing := range value.ProductionJobs {
		if existing.Kind == job.Kind {
			if existing.ProviderJobID == job.ProviderJobID {
				return nil
			}
			return ErrProviderJobConflict
		}
	}
	value.ProductionJobs = append(value.ProductionJobs, job)
	r.tasks[taskID] = value
	return nil
}

func (r *memoryRepository) CreateVersion(_ context.Context, value CreativeVersion) (CreativeVersion, bool, error) {
	for _, existing := range r.versions {
		if existing.ProjectID == value.ProjectID && existing.CreatedBy == value.CreatedBy && existing.IdempotencyKey == value.IdempotencyKey {
			if existing.RequestHash != value.RequestHash {
				return CreativeVersion{}, false, ErrIdempotencyConflict
			}
			return existing, true, nil
		}
		if existing.TaskID == value.TaskID && existing.Version == value.Version {
			if !existing.ContentHash.Equal(value.ContentHash) {
				return CreativeVersion{}, false, ErrVersionConflict
			}
			return existing, false, nil
		}
	}
	r.versions[value.ID] = value
	return value, false, nil
}

func (r *memoryRepository) GetVersion(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string) (CreativeVersion, error) {
	value, ok := r.versions[id]
	if !ok {
		return CreativeVersion{}, ErrNotFound
	}
	return value, nil
}

func (r *memoryRepository) ListVersions(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, taskID string, limit int) ([]CreativeVersion, error) {
	values := make([]CreativeVersion, 0, len(r.versions))
	for _, value := range r.versions {
		if value.OrganizationID == organizationID && value.ProjectID == projectID && (taskID == "" || value.TaskID == taskID) {
			values = append(values, value)
			if len(values) == limit {
				break
			}
		}
	}
	return values, nil
}

func (r *memoryRepository) RecordVersionCheck(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string, check CreativeCheck) (CreativeVersion, error) {
	value, ok := r.versions[id]
	if !ok {
		return CreativeVersion{}, ErrNotFound
	}
	value.Status = CreativeVersionChecked
	value.Check = &check
	r.versions[id] = value
	return value, nil
}

func (r *memoryRepository) ApproveVersion(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, id string, approval CreativeApproval) (CreativeVersion, error) {
	value, ok := r.versions[id]
	if !ok {
		return CreativeVersion{}, ErrNotFound
	}
	if value.Status != CreativeVersionChecked || value.Check == nil || !value.Check.Passed {
		return CreativeVersion{}, ErrInvalidState
	}
	value.Status = CreativeVersionApproved
	value.Approval = &approval
	r.versions[id] = value
	return value, nil
}

func (r *memoryRepository) CreatePackage(_ context.Context, value CreativePackage) (CreativePackage, error) {
	for _, existing := range r.packages {
		if existing.CreativeVersionID == value.CreativeVersionID {
			return existing, nil
		}
	}
	r.packages[value.ID] = value
	return value, nil
}

func (r *memoryRepository) ListPackages(_ context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, limit int) ([]CreativePackage, error) {
	values := make([]CreativePackage, 0, len(r.packages))
	for _, value := range r.packages {
		if value.OrganizationID == organizationID && value.ProjectID == projectID {
			values = append(values, value)
			if len(values) == limit {
				break
			}
		}
	}
	return values, nil
}
