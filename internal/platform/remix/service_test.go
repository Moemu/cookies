package remix

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestServiceCreatesAndReadsScopedPlan(t *testing.T) {
	t.Parallel()
	service := NewMemoryService(func() (string, error) { return "remixplan_1", nil })
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}

	created, err := service.Create(context.Background(), actor, "project_1", validCreatePlanRequest())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID != "remixplan_1" || created.ProjectID != "project_1" || created.OrganizationID != "org_1" {
		t.Fatalf("created plan = %#v", created)
	}

	got, err := service.Get(context.Background(), actor, "project_1", created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ClientPlanID != "client_plan_1" || len(got.Segments) != 3 {
		t.Fatalf("got plan = %#v", got)
	}

	otherActor := actor
	otherActor.OrganizationID = "org_2"
	if _, err := service.Get(context.Background(), otherActor, "project_1", created.ID); err != ErrNotFound {
		t.Fatalf("cross-org Get() error = %v, want ErrNotFound", err)
	}
}

func TestServiceNormalizesLegacyClipsToV2Shots(t *testing.T) {
	t.Parallel()
	service := NewMemoryService(func() (string, error) { return "remixplan_1", nil })
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}

	created, err := service.Create(context.Background(), actor, "project_1", validCreatePlanRequest())
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if created.SchemaVersion != SchemaVersionV2 {
		t.Fatalf("schema_version = %q, want %q", created.SchemaVersion, SchemaVersionV2)
	}
	if len(created.Segments[0].Shots) != 1 {
		t.Fatalf("shots = %#v", created.Segments[0].Shots)
	}
	shot := created.Segments[0].Shots[0]
	clip := created.Segments[0].Clips[0]
	if shot.ID != clip.ID || shot.AssetVersion != clip.AssetVersion || shot.Timeline.DurationSeconds != clip.DurationSeconds {
		t.Fatalf("shot was not derived from legacy clip: shot=%#v clip=%#v", shot, clip)
	}
	if shot.Planning.Score != clip.Score || shot.Planning.Reason != clip.Reason {
		t.Fatalf("shot planning was not derived from legacy clip: %#v", shot.Planning)
	}
}

func TestServiceCreatesAndReadsShotBasedPlan(t *testing.T) {
	t.Parallel()
	service := NewMemoryService(func() (string, error) { return "remixplan_1", nil })
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	request := validCreatePlanRequest()
	request.SchemaVersion = SchemaVersionV2
	for index := range request.Segments {
		request.Segments[index].Shots = []Shot{validShot(request.Segments[index].Segment, contract.AssetID("shot_asset_"+string(rune('1'+index))))}
		request.Segments[index].Clips = nil
	}

	created, err := service.Create(context.Background(), actor, "project_1", request)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	got, err := service.Get(context.Background(), actor, "project_1", created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.SchemaVersion != SchemaVersionV2 || len(got.Segments[0].Shots) != 1 {
		t.Fatalf("got plan = %#v", got)
	}
	if len(got.Segments[0].Clips) != 1 || got.Segments[0].Clips[0].AssetVersion != got.Segments[0].Shots[0].AssetVersion {
		t.Fatalf("compatible clips were not derived from shots: %#v", got.Segments[0])
	}
}

func TestServiceListsProjectPlansNewestFirst(t *testing.T) {
	t.Parallel()
	nextID := 0
	service := NewMemoryService(func() (string, error) {
		nextID++
		return "remixplan_" + string(rune('0'+nextID)), nil
	})
	now := time.Date(2026, 7, 25, 10, 0, 0, 0, time.UTC)
	service.nowUTC = func() time.Time {
		now = now.Add(time.Minute)
		return now
	}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}

	first, err := service.Create(context.Background(), actor, "project_1", validCreatePlanRequest())
	if err != nil {
		t.Fatal(err)
	}
	secondRequest := validCreatePlanRequest()
	secondRequest.ClientPlanID = "client_plan_2"
	second, err := service.Create(context.Background(), actor, "project_1", secondRequest)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Create(context.Background(), actor, "project_2", validCreatePlanRequest())
	if err != nil {
		t.Fatal(err)
	}

	plans, err := service.List(context.Background(), actor, "project_1", 10)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(plans) != 2 || plans[0].ID != second.ID || plans[1].ID != first.ID {
		t.Fatalf("plans = %#v", plans)
	}

	limited, err := service.List(context.Background(), actor, "project_1", 1)
	if err != nil {
		t.Fatalf("List(limit=1) error = %v", err)
	}
	if len(limited) != 1 || limited[0].ID != second.ID {
		t.Fatalf("limited = %#v", limited)
	}
}

func TestServiceCreatesRenderJobForExistingPlan(t *testing.T) {
	t.Parallel()
	nextID := 0
	service := NewMemoryService(func() (string, error) {
		nextID++
		if nextID == 1 {
			return "remixplan_1", nil
		}
		return "remixrender_1", nil
	})
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	plan, err := service.Create(context.Background(), actor, "project_1", validCreatePlanRequest())
	if err != nil {
		t.Fatal(err)
	}

	job, err := service.CreateRenderJob(context.Background(), actor, "project_1", "render-key-1", CreateRenderJobRequest{PlanID: plan.ID, TargetQuality: "draft"})
	if err != nil {
		t.Fatalf("CreateRenderJob() error = %v", err)
	}
	if job.ID != "remixrender_1" || job.Status != RenderQueued || job.Progress != 0 || job.TargetFormat != "mp4" || job.TargetQuality != "draft" {
		t.Fatalf("job = %#v", job)
	}
	if job.IdempotencyKey != "render-key-1" || job.RequestHash == "" || job.InputSnapshot.Plan.ID != plan.ID {
		t.Fatalf("job did not persist idempotency and snapshot fields: %#v", job)
	}

	got, err := service.GetRenderJob(context.Background(), actor, "project_1", job.ID)
	if err != nil {
		t.Fatalf("GetRenderJob() error = %v", err)
	}
	if got.PlanID != plan.ID {
		t.Fatalf("got job = %#v", got)
	}

	if _, err := service.CreateRenderJob(context.Background(), actor, "project_1", "render-key-missing", CreateRenderJobRequest{PlanID: "missing"}); err != ErrNotFound {
		t.Fatalf("missing plan error = %v, want ErrNotFound", err)
	}
}

func TestServiceCreateRenderJobIsIdempotentAndDetectsConflicts(t *testing.T) {
	t.Parallel()
	nextID := 0
	service := NewMemoryService(func() (string, error) {
		nextID++
		if nextID == 1 {
			return "remixplan_1", nil
		}
		return "remixrender_" + string(rune('0'+nextID-1)), nil
	})
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	plan, err := service.Create(context.Background(), actor, "project_1", validCreatePlanRequest())
	if err != nil {
		t.Fatal(err)
	}
	request := CreateRenderJobRequest{PlanID: plan.ID, TargetQuality: "draft"}

	first, err := service.CreateRenderJob(context.Background(), actor, "project_1", "render-key-1", request)
	if err != nil {
		t.Fatalf("first CreateRenderJob() error = %v", err)
	}
	second, err := service.CreateRenderJob(context.Background(), actor, "project_1", "render-key-1", request)
	if err != nil {
		t.Fatalf("second CreateRenderJob() error = %v", err)
	}
	if second.ID != first.ID || second.RequestHash != first.RequestHash {
		t.Fatalf("idempotent create returned a different job: first=%#v second=%#v", first, second)
	}

	_, err = service.CreateRenderJob(context.Background(), actor, "project_1", "render-key-1", CreateRenderJobRequest{PlanID: plan.ID, TargetQuality: "high"})
	if err != ErrIdempotencyConflict {
		t.Fatalf("conflicting CreateRenderJob() error = %v, want ErrIdempotencyConflict", err)
	}
}

func TestRenderJobStateMachinePersistsProgressAndSupportsRecovery(t *testing.T) {
	t.Parallel()
	store := NewMemoryRenderJobStore()
	nextID := 0
	newID := func() (string, error) {
		nextID++
		if nextID == 1 {
			return "remixplan_1", nil
		}
		return "remixrender_1", nil
	}
	service := NewServiceWithRenderStore(newID, store, nil)
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	plan, err := service.Create(context.Background(), actor, "project_1", validCreatePlanRequest())
	if err != nil {
		t.Fatal(err)
	}
	job, err := service.CreateRenderJob(context.Background(), actor, "project_1", "render-key-1", CreateRenderJobRequest{PlanID: plan.ID})
	if err != nil {
		t.Fatalf("CreateRenderJob() error = %v", err)
	}

	running, err := service.UpdateRenderJobStatus(context.Background(), actor, "project_1", job.ID, RenderRunning, 45, nil, false)
	if err != nil {
		t.Fatalf("UpdateRenderJobStatus(running) error = %v", err)
	}
	if running.Status != RenderRunning || running.Progress != 45 {
		t.Fatalf("running job = %#v", running)
	}

	restarted := NewServiceWithRenderStore(func() (string, error) { return "unused", nil }, store, nil)
	recovered, err := restarted.GetRenderJob(context.Background(), actor, "project_1", job.ID)
	if err != nil {
		t.Fatalf("GetRenderJob(after restart) error = %v", err)
	}
	if recovered.Status != RenderRunning || recovered.Progress != 45 || recovered.InputSnapshot.Plan.ID != plan.ID {
		t.Fatalf("recovered job = %#v", recovered)
	}

	failed, err := restarted.UpdateRenderJobStatus(context.Background(), actor, "project_1", job.ID, RenderFailed, 45, &contract.JobError{Code: "RENDER_FAILED", Message: "renderer failed", Retryable: true}, false)
	if err != nil {
		t.Fatalf("UpdateRenderJobStatus(failed) error = %v", err)
	}
	if failed.ErrorCode != "RENDER_FAILED" || failed.ErrorMessage != "renderer failed" {
		t.Fatalf("failed job = %#v", failed)
	}
	if _, err := restarted.UpdateRenderJobStatus(context.Background(), actor, "project_1", job.ID, RenderSucceeded, 100, nil, false); err == nil {
		t.Fatal("terminal failed job transitioned to succeeded")
	}
}

func TestCompleteRenderJobOutputCreatesIntakeAndReturnsStableAsset(t *testing.T) {
	t.Parallel()
	store := NewMemoryRenderJobStore()
	intakes := &fakeRenderOutputIntake{
		result: assets.GeneratedIntake{
			ID: "intake_1",
			ProjectAssetRef: &contract.ProjectAssetRef{
				ProjectID:    "project_1",
				AssetVersion: contract.AssetVersionRef{AssetID: "asset_output_1", Version: 1},
			},
		},
	}
	nextID := 0
	service := NewServiceWithRenderOutputIntake(func() (string, error) {
		nextID++
		if nextID == 1 {
			return "remixplan_1", nil
		}
		return "remixrender_1", nil
	}, store, nil, intakes)
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	requestContext := contract.RequestContext{RequestID: "req_1", TraceID: "trace_1", Actor: actor}
	plan, err := service.Create(context.Background(), actor, "project_1", validCreatePlanRequest())
	if err != nil {
		t.Fatal(err)
	}
	job, err := service.CreateRenderJob(context.Background(), actor, "project_1", "render-key-1", CreateRenderJobRequest{PlanID: plan.ID})
	if err != nil {
		t.Fatal(err)
	}

	completed, err := service.CompleteRenderJobOutput(context.Background(), requestContext, "project_1", job.ID, CompleteRenderJobOutputRequest{
		Output: contract.ProviderOutputRef{
			ProviderCode:       "fake-renderer",
			ProviderJobID:      "provider_job_1",
			OutputID:           "output_1",
			RetrievalExpiresAt: time.Now().Add(time.Hour),
			DeclaredMIMEType:   "video/mp4",
			DeclaredSizeBytes:  1024,
		},
		ModelAlias:            "remix-renderer",
		ModelVersion:          "v1",
		ProjectContextVersion: 7,
	})
	if err != nil {
		t.Fatalf("CompleteRenderJobOutput() error = %v", err)
	}

	if completed.Status != RenderSucceeded || completed.Progress != 100 || completed.OutputAsset == nil {
		t.Fatalf("completed job = %#v", completed)
	}
	if completed.OutputPreview == nil || completed.OutputPreview.URL != "/platform/v1/projects/project_1/assets/asset_output_1/versions/1/preview" {
		t.Fatalf("missing output preview: %#v", completed.OutputPreview)
	}
	if completed.Provenance == nil || completed.Provenance.PlanID != plan.ID || completed.Provenance.RenderJobID != job.ID || len(completed.Provenance.InputAssets) != 3 {
		t.Fatalf("missing provenance summary: %#v", completed.Provenance)
	}
	if intakes.request.ProviderJobID != "provider_job_1" || intakes.request.Output.OutputID != "output_1" {
		t.Fatalf("unexpected intake request: %#v", intakes.request)
	}
	if len(intakes.request.Provenance.SourceAssetRefs) != 3 || len(intakes.request.Provenance.SourceResourceRefs) != 2 {
		t.Fatalf("unexpected intake provenance: %#v", intakes.request.Provenance)
	}
	if intakes.request.Provenance.SourceResourceRefs[0].Type != "remix_plan" || intakes.request.Provenance.SourceResourceRefs[1].Type != "remix_render_job" {
		t.Fatalf("missing remix resource refs: %#v", intakes.request.Provenance.SourceResourceRefs)
	}
	if string(intakes.key) != "remix-render-remixrender_1-output-output_1" {
		t.Fatalf("unexpected idempotency key: %q", intakes.key)
	}
}

func TestQualityReportCriticalVerdictFailsRenderJob(t *testing.T) {
	t.Parallel()
	ids := []string{"remixplan_1", "remixrender_1", "qualityreport_1"}
	service := NewMemoryService(func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	})
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	request := validCreatePlanRequest()
	request.Warnings = []string{"quality:critical"}
	plan, err := service.Create(context.Background(), actor, "project_1", request)
	if err != nil {
		t.Fatal(err)
	}
	job, err := service.CreateRenderJob(context.Background(), actor, "project_1", "render-key-1", CreateRenderJobRequest{PlanID: plan.ID})
	if err != nil {
		t.Fatalf("CreateRenderJob() error = %v", err)
	}

	report, err := service.CreateQualityReport(context.Background(), actor, "project_1", CreateQualityReportRequest{RenderJobID: job.ID})
	if err != nil {
		t.Fatalf("CreateQualityReport() error = %v", err)
	}
	if report.ID != "qualityreport_1" || report.Verdict != QualityVerdictCritical || len(report.Issues) != 1 {
		t.Fatalf("report = %#v", report)
	}
	updated, err := service.GetRenderJob(context.Background(), actor, "project_1", job.ID)
	if err != nil {
		t.Fatalf("GetRenderJob() error = %v", err)
	}
	if updated.Status != RenderFailed || updated.QualityReportID != report.ID || updated.ErrorCode != "QUALITY_CRITICAL" {
		t.Fatalf("quality verdict was not mapped onto render job: %#v", updated)
	}
}

func TestQualityReportMajorVerdictRequiresReviewAndIsScoped(t *testing.T) {
	t.Parallel()
	ids := []string{"remixplan_1", "remixrender_1", "qualityreport_1"}
	service := NewMemoryService(func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	})
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	request := validCreatePlanRequest()
	request.Warnings = []string{"quality:major"}
	plan, err := service.Create(context.Background(), actor, "project_1", request)
	if err != nil {
		t.Fatal(err)
	}
	job, err := service.CreateRenderJob(context.Background(), actor, "project_1", "render-key-1", CreateRenderJobRequest{PlanID: plan.ID})
	if err != nil {
		t.Fatalf("CreateRenderJob() error = %v", err)
	}

	report, err := service.CreateQualityReport(context.Background(), actor, "project_1", CreateQualityReportRequest{RenderJobID: job.ID})
	if err != nil {
		t.Fatalf("CreateQualityReport() error = %v", err)
	}
	if report.Verdict != QualityVerdictMajor || report.Score <= 0 {
		t.Fatalf("report = %#v", report)
	}
	updated, err := service.GetRenderJob(context.Background(), actor, "project_1", job.ID)
	if err != nil {
		t.Fatalf("GetRenderJob() error = %v", err)
	}
	if updated.Status != RenderRequiresReview || !updated.RequiresReview || updated.ErrorCode != "QUALITY_REVIEW_REQUIRED" {
		t.Fatalf("major verdict was not mapped to review: %#v", updated)
	}
	got, err := service.GetQualityReportForRenderJob(context.Background(), actor, "project_1", job.ID)
	if err != nil {
		t.Fatalf("GetQualityReportForRenderJob() error = %v", err)
	}
	if got.ID != report.ID {
		t.Fatalf("got report = %#v", got)
	}
	otherActor := actor
	otherActor.OrganizationID = "org_2"
	if _, err := service.GetQualityReport(context.Background(), otherActor, "project_1", report.ID); err != ErrNotFound {
		t.Fatalf("cross-org GetQualityReport() error = %v, want ErrNotFound", err)
	}
}

func TestCreatePlanRequestRejectsNonVideoClips(t *testing.T) {
	t.Parallel()
	request := validCreatePlanRequest()
	request.Segments[0].Clips[0].MimeType = "image/png"

	if err := request.Validate(); err == nil {
		t.Fatal("Validate() succeeded for image clip")
	}
}

func TestCreatePlanRequestRejectsInvalidShotTimeline(t *testing.T) {
	t.Parallel()
	request := validCreatePlanRequest()
	request.SchemaVersion = SchemaVersionV2
	request.Segments[0].Shots = []Shot{validShot(SegmentOpening, "asset_opening")}
	request.Segments[0].Shots[0].Timeline.OutPointSeconds = -1
	request.Segments[0].Clips = nil

	if err := request.Validate(); err == nil {
		t.Fatal("Validate() succeeded for invalid shot timeline")
	}
}

func TestHitAnalysisUsesContinuousNonOverlappingSegments(t *testing.T) {
	t.Parallel()
	service := NewMemoryService(func() (string, error) { return "hitanalysis_1", nil })
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}

	analysis, err := service.CreateHitAnalysis(context.Background(), actor, "project_1", CreateHitAnalysisRequest{
		SourceAsset:     contract.AssetVersionRef{AssetID: "source_video", Version: 1},
		Title:           "爆款拆解样本",
		DurationSeconds: 25,
	})
	if err != nil {
		t.Fatalf("CreateHitAnalysis() error = %v", err)
	}

	if analysis.ID != "hitanalysis_1" || analysis.VideoMeta.DurationSeconds != 25 || len(analysis.Segments) != 3 {
		t.Fatalf("analysis = %#v", analysis)
	}
	cursor := 0.0
	for _, segment := range analysis.Segments {
		if segment.StartSeconds != cursor || segment.EndSeconds <= segment.StartSeconds {
			t.Fatalf("segments are not continuous: %#v", analysis.Segments)
		}
		cursor = segment.EndSeconds
	}
	if cursor != analysis.VideoMeta.DurationSeconds {
		t.Fatalf("segments do not cover duration: cursor=%v analysis=%#v", cursor, analysis)
	}
	if len(analysis.Scripts) != len(analysis.Segments) || len(analysis.ConversionNodes) != len(analysis.Segments) {
		t.Fatalf("analysis missing structured outputs: %#v", analysis)
	}
}

func TestProductMappingRequiresAuthorizedReplacementAssets(t *testing.T) {
	t.Parallel()
	nextID := 0
	service := NewMemoryService(func() (string, error) {
		nextID++
		if nextID == 1 {
			return "hitanalysis_1", nil
		}
		return "productmapping_1", nil
	})
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	analysis, err := service.CreateHitAnalysis(context.Background(), actor, "project_1", CreateHitAnalysisRequest{
		SourceAsset:     contract.AssetVersionRef{AssetID: "source_video", Version: 1},
		Title:           "爆款拆解样本",
		DurationSeconds: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := validProductMappingRequest(analysis.ID)
	request.RequiredAssets = request.RequiredAssets[:2]

	if _, err := service.CreateProductMapping(context.Background(), actor, "project_1", request); err == nil {
		t.Fatal("CreateProductMapping() succeeded without all mapped assets listed as required")
	}

	request = validProductMappingRequest(analysis.ID)
	request.ReplacementRules[0].TargetAsset = analysis.SourceAsset
	request.RequiredAssets = append(request.RequiredAssets, analysis.SourceAsset)
	if _, err := service.CreateProductMapping(context.Background(), actor, "project_1", request); err == nil {
		t.Fatal("CreateProductMapping() succeeded while reusing the source video asset")
	}
}

func TestProductMappingGeneratesShotBasedPlanWithoutSourceVideo(t *testing.T) {
	t.Parallel()
	ids := []string{"hitanalysis_1", "productmapping_1", "remixplan_1"}
	service := NewMemoryService(func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	})
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	analysis, err := service.CreateHitAnalysis(context.Background(), actor, "project_1", CreateHitAnalysisRequest{
		SourceAsset:     contract.AssetVersionRef{AssetID: "source_video", Version: 1},
		Title:           "爆款拆解样本",
		DurationSeconds: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	mapping, err := service.CreateProductMapping(context.Background(), actor, "project_1", validProductMappingRequest(analysis.ID))
	if err != nil {
		t.Fatalf("CreateProductMapping() error = %v", err)
	}

	plan, err := service.GeneratePlanFromProductMapping(context.Background(), actor, "project_1", mapping.ID)
	if err != nil {
		t.Fatalf("GeneratePlanFromProductMapping() error = %v", err)
	}

	if plan.SchemaVersion != SchemaVersionV2 || plan.ClientPlanID != "mapping_productmapping_1" || len(plan.Segments) != 3 {
		t.Fatalf("plan = %#v", plan)
	}
	used := map[contract.AssetVersionRef]bool{}
	for _, segment := range plan.Segments {
		if len(segment.Shots) == 0 || len(segment.Clips) == 0 {
			t.Fatalf("segment missing shot/clip compatibility: %#v", segment)
		}
		for _, shot := range segment.Shots {
			used[shot.AssetVersion] = true
			if shot.AssetVersion == analysis.SourceAsset {
				t.Fatalf("plan reused source video asset: %#v", shot)
			}
			if shot.Source != ShotSourceExistingAsset || shot.Planning.Score <= 0 {
				t.Fatalf("shot is not a valid mapped existing-asset shot: %#v", shot)
			}
		}
	}
	if len(used) != 3 || plan.Summary.UsedAssets != 3 {
		t.Fatalf("used assets = %#v summary=%#v", used, plan.Summary)
	}
}

func TestFeedbackEventsAreAppendOnlyAndAggregateDeterministically(t *testing.T) {
	t.Parallel()
	ids := []string{"feedback_1", "feedback_2", "feedback_3", "weights_1"}
	service := NewMemoryService(func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	})
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	service.nowUTC = func() time.Time {
		now = now.Add(time.Minute)
		return now
	}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	asset := contract.AssetVersionRef{AssetID: "asset_output_1", Version: 1}

	first, err := service.CreateFeedbackEvent(context.Background(), actor, "project_1", CreateFeedbackEventRequest{
		EventType: FeedbackEventRating, TargetType: FeedbackTargetAsset, TargetID: "asset_output_1", AssetVersion: &asset, Rating: 5, Comment: "成片节奏优秀",
	})
	if err != nil {
		t.Fatalf("CreateFeedbackEvent(rating) error = %v", err)
	}
	second, err := service.CreateFeedbackEvent(context.Background(), actor, "project_1", CreateFeedbackEventRequest{
		EventType: FeedbackEventAssetSelected, TargetType: FeedbackTargetRemixPlan, TargetID: "remixplan_1", AssetVersion: &asset,
	})
	if err != nil {
		t.Fatalf("CreateFeedbackEvent(selected) error = %v", err)
	}
	_, err = service.CreateFeedbackEvent(context.Background(), actor, "project_1", CreateFeedbackEventRequest{
		EventType: FeedbackEventRenderSucceeded, TargetType: FeedbackTargetRenderJob, TargetID: "remixrender_1", AssetVersion: &asset,
	})
	if err != nil {
		t.Fatalf("CreateFeedbackEvent(render_succeeded) error = %v", err)
	}
	first.Comment = "mutated locally"

	events, err := service.ListFeedbackEvents(context.Background(), actor, "project_1", FeedbackEventFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListFeedbackEvents() error = %v", err)
	}
	if len(events) != 3 || events[0].ID != "feedback_3" || events[2].Comment != "成片节奏优秀" {
		t.Fatalf("events are not append-only newest-first clones: %#v", events)
	}
	filtered, err := service.ListFeedbackEvents(context.Background(), actor, "project_1", FeedbackEventFilter{TargetType: FeedbackTargetRemixPlan, TargetID: "remixplan_1", Limit: 10})
	if err != nil {
		t.Fatalf("filtered ListFeedbackEvents() error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != second.ID {
		t.Fatalf("filtered events = %#v", filtered)
	}

	performance, err := service.GetAssetPerformanceSnapshot(context.Background(), actor, "project_1")
	if err != nil {
		t.Fatalf("GetAssetPerformanceSnapshot() error = %v", err)
	}
	if len(performance) != 1 || performance[0].SelectedCount != 1 || performance[0].RenderSucceededCount != 1 || performance[0].AverageRating != 5 {
		t.Fatalf("performance = %#v", performance)
	}
	again, err := service.GetAssetPerformanceSnapshot(context.Background(), actor, "project_1")
	if err != nil {
		t.Fatalf("GetAssetPerformanceSnapshot(second) error = %v", err)
	}
	if len(again) != 1 || again[0] != performance[0] {
		t.Fatalf("performance is not deterministic: first=%#v second=%#v", performance, again)
	}

	snapshot, err := service.CreatePlannerWeightSnapshot(context.Background(), actor, "project_1")
	if err != nil {
		t.Fatalf("CreatePlannerWeightSnapshot() error = %v", err)
	}
	if snapshot.ID != "weights_1" || len(snapshot.AssetWeights) != 1 || snapshot.AssetWeights[0].Weight <= 0 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestCreatePrerollValidatesHookTypeAndSupportsPromptOnly(t *testing.T) {
	t.Parallel()
	ids := []string{"remixplan_1", "preroll_1"}
	service := NewMemoryService(func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	})
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	plan, err := service.Create(context.Background(), actor, "project_1", validCreatePlanRequest())
	if err != nil {
		t.Fatal(err)
	}
	invalid := validPrerollRequest(plan.ID)
	invalid.HookType = "unknown"
	if _, err := service.CreatePreroll(context.Background(), actor, "project_1", invalid); err == nil {
		t.Fatal("CreatePreroll() succeeded for invalid hook type")
	}

	preroll, err := service.CreatePreroll(context.Background(), actor, "project_1", validPrerollRequest(plan.ID))
	if err != nil {
		t.Fatalf("CreatePreroll(prompt_only) error = %v", err)
	}
	if preroll.ID != "preroll_1" || preroll.Status != PrerollDraft || preroll.OutputAsset != nil || preroll.QualityVerdict != QualityVerdictPass {
		t.Fatalf("prompt-only preroll = %#v", preroll)
	}
	if !strings.Contains(preroll.PromptDraft, string(HookTypeConflict)) || !strings.Contains(preroll.PromptDraft, "asset_opening@v1") {
		t.Fatalf("prompt draft does not include hook and reference asset: %q", preroll.PromptDraft)
	}
	if _, err := service.ApplyPreroll(context.Background(), actor, "project_1", preroll.ID); err != ErrPrerollNotReady {
		t.Fatalf("ApplyPreroll(prompt_only) error = %v, want ErrPrerollNotReady", err)
	}
}

func TestPrerollVideoAppliesToOpeningAndRecalculatesTimeline(t *testing.T) {
	t.Parallel()
	ids := []string{"remixplan_1", "preroll_1"}
	service := NewMemoryService(func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	})
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	plan, err := service.Create(context.Background(), actor, "project_1", validCreatePlanRequest())
	if err != nil {
		t.Fatal(err)
	}
	request := validPrerollRequest(plan.ID)
	request.Mode = PrerollModeGenerateVideo
	request.DurationSeconds = 4

	preroll, err := service.CreatePreroll(context.Background(), actor, "project_1", request)
	if err != nil {
		t.Fatalf("CreatePreroll(video) error = %v", err)
	}
	if preroll.Status != PrerollReady || preroll.OutputAsset == nil {
		t.Fatalf("video preroll was not ready: %#v", preroll)
	}
	applied, err := service.ApplyPreroll(context.Background(), actor, "project_1", preroll.ID)
	if err != nil {
		t.Fatalf("ApplyPreroll() error = %v", err)
	}

	opening := applied.Segments[0]
	if opening.Segment != SegmentOpening || len(opening.Shots) != 2 || len(opening.Clips) != 2 {
		t.Fatalf("opening segment was not prepended with compatible shot/clip: %#v", opening)
	}
	if opening.Shots[0].AssetVersion.AssetID != "preroll_1_asset" || opening.Shots[0].Timeline.StartSeconds != 0 || opening.Shots[0].Timeline.DurationSeconds != 4 {
		t.Fatalf("unexpected preroll shot: %#v", opening.Shots[0])
	}
	if opening.Shots[1].Timeline.StartSeconds != 4 {
		t.Fatalf("existing opening shot was not shifted: %#v", opening.Shots[1])
	}
	if applied.ActualSeconds != 18.4 || opening.ActualSeconds != 8.8 || applied.Summary.UsedAssets != 4 {
		t.Fatalf("timeline summary was not recalculated: plan=%#v opening=%#v", applied, opening)
	}
	updatedPreroll, err := service.GetPreroll(context.Background(), actor, "project_1", preroll.ID)
	if err != nil {
		t.Fatalf("GetPreroll() error = %v", err)
	}
	if updatedPreroll.Status != PrerollApplied || updatedPreroll.AppliedPlanID != plan.ID {
		t.Fatalf("preroll apply status = %#v", updatedPreroll)
	}
}

func TestPrerollQualityFailureBlocksApply(t *testing.T) {
	t.Parallel()
	ids := []string{"remixplan_1", "preroll_1"}
	service := NewMemoryService(func() (string, error) {
		id := ids[0]
		ids = ids[1:]
		return id, nil
	})
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	plan, err := service.Create(context.Background(), actor, "project_1", validCreatePlanRequest())
	if err != nil {
		t.Fatal(err)
	}
	request := validPrerollRequest(plan.ID)
	request.Mode = PrerollModeGenerateVideo
	request.StyleConstraints = append(request.StyleConstraints, "quality:critical")

	preroll, err := service.CreatePreroll(context.Background(), actor, "project_1", request)
	if err != nil {
		t.Fatalf("CreatePreroll(critical) error = %v", err)
	}
	if preroll.Status != PrerollFailed || preroll.QualityVerdict != QualityVerdictCritical || preroll.OutputAsset != nil {
		t.Fatalf("critical preroll should be failed without output: %#v", preroll)
	}
	if _, err := service.ApplyPreroll(context.Background(), actor, "project_1", preroll.ID); err != ErrPrerollNotReady {
		t.Fatalf("ApplyPreroll(critical) error = %v, want ErrPrerollNotReady", err)
	}
}

func TestEvalRunIsDeterministicAndReportsFailedCases(t *testing.T) {
	t.Parallel()
	service := NewMemoryService(func() (string, error) { return "unused", nil })
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	request := CreateEvalRunRequest{
		PlannerVersion: "planner.v1",
		PromptVersion:  "prompt.v1",
		Submissions: []EvalSubmission{
			{CaseID: "remix_mmlu_hook_mcq_v1", ChoiceID: "a"},
			{CaseID: "remix_mmlu_rubric_v1", AnswerText: "authorized assets and timeline are covered"},
		},
	}

	first, err := service.CreateEvalRun(context.Background(), actor, "project_1", request)
	if err != nil {
		t.Fatalf("CreateEvalRun() error = %v", err)
	}
	second, err := service.CreateEvalRun(context.Background(), actor, "project_1", request)
	if err != nil {
		t.Fatalf("CreateEvalRun() second error = %v", err)
	}

	if first.Score != second.Score || first.PassedCases != second.PassedCases {
		t.Fatalf("runs are not deterministic: first=%#v second=%#v", first, second)
	}
	if first.TotalCases != 2 || first.PassedCases != 1 || len(first.FailedCases) != 1 || first.FailedCases[0] != "remix_mmlu_hook_mcq_v1" {
		t.Fatalf("unexpected run summary: %#v", first)
	}
	if first.Results[0].Passed || first.Results[0].Actual != "a" {
		t.Fatalf("mcq result = %#v", first.Results[0])
	}
	if !first.Results[1].Passed {
		t.Fatalf("rubric result = %#v", first.Results[1])
	}
}

func TestEvalCasesAreSeededAndScoped(t *testing.T) {
	t.Parallel()
	service := NewMemoryService(func() (string, error) { return "unused", nil })
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}

	cases, err := service.ListEvalCases(context.Background(), actor, "project_1")
	if err != nil {
		t.Fatalf("ListEvalCases() error = %v", err)
	}
	if len(cases) != 2 || cases[0].ID != "remix_mmlu_hook_mcq_v1" || cases[1].ID != "remix_mmlu_rubric_v1" {
		t.Fatalf("seed cases = %#v", cases)
	}

	otherActor := actor
	otherActor.OrganizationID = "org_2"
	otherCases, err := service.ListEvalCases(context.Background(), otherActor, "project_1")
	if err != nil {
		t.Fatalf("ListEvalCases(other org) error = %v", err)
	}
	if len(otherCases) != 2 || otherCases[0].OrganizationID != "org_2" {
		t.Fatalf("seed cases were not scoped to the requesting org: %#v", otherCases)
	}
}

func TestCreateEvalCaseValidatesMCQExpectedChoice(t *testing.T) {
	t.Parallel()
	request := CreateEvalCaseRequest{
		Type:           EvalCaseMCQ,
		Title:          "bad mcq",
		Prompt:         "choose one",
		PlannerVersion: "planner.v1",
		PromptVersion:  "prompt.v1",
		Choices:        []EvalChoice{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}},
		ExpectedChoice: "c",
		PassingScore:   1,
	}

	if err := request.Validate(); err == nil {
		t.Fatal("Validate() succeeded for an unknown expected choice")
	}
}

func validCreatePlanRequest() CreatePlanRequest {
	return CreatePlanRequest{
		ClientPlanID:  "client_plan_1",
		TargetSeconds: 30,
		ActualSeconds: 14.4,
		Pace:          PaceBalanced,
		Segments: []SegmentPlan{
			validSegment(SegmentOpening, "前段", "asset_opening"),
			validSegment(SegmentMiddle, "中段", "asset_middle"),
			validSegment(SegmentEnding, "后段", "asset_ending"),
		},
		Warnings: []string{"duration is estimated"},
		Summary:  PlanSummary{SelectedAssets: 3, UsedAssets: 3, CoveragePercent: 48, Strategy: "balanced"},
	}
}

func validProductMappingRequest(analysisID string) CreateProductMappingRequest {
	return CreateProductMappingRequest{
		HitAnalysisID: analysisID,
		TargetProduct: ProductProfile{
			Name:          "白域精工新品",
			SellingPoints: []string{"±0.01mm 精度", "98% 准时交付"},
			CTA:           "预约获取打样方案",
		},
		RequiredAssets: []contract.AssetVersionRef{
			{AssetID: "target_hook", Version: 1},
			{AssetID: "target_proof", Version: 1},
			{AssetID: "target_cta", Version: 1},
		},
		ReplacementRules: []ReplacementRule{
			{Role: HitRoleHook, TargetAsset: contract.AssetVersionRef{AssetID: "target_hook", Version: 1}, Message: "先展示交期风险反差"},
			{Role: HitRoleProof, TargetAsset: contract.AssetVersionRef{AssetID: "target_proof", Version: 1}, Message: "用精度和产线证据替换原证明段"},
			{Role: HitRoleCTA, TargetAsset: contract.AssetVersionRef{AssetID: "target_cta", Version: 1}, Message: "引导预约打样方案"},
		},
		Constraints:   []string{"不得复用原视频二进制"},
		TargetSeconds: 30,
		Pace:          PaceBalanced,
	}
}

func validPrerollRequest(planID string) CreatePrerollRequest {
	return CreatePrerollRequest{
		PlanID:          planID,
		HookType:        HookTypeConflict,
		ReferenceAsset:  contract.AssetVersionRef{AssetID: "asset_opening", Version: 1},
		DurationSeconds: 6,
		Mode:            PrerollModePromptOnly,
		StyleConstraints: []string{
			"9:16 竖版",
			"静音可理解",
		},
	}
}

func validShot(segment Segment, assetID contract.AssetID) Shot {
	return Shot{
		ID:           string(segment) + "_shot_1",
		Segment:      segment,
		Source:       ShotSourceExistingAsset,
		AssetVersion: contract.AssetVersionRef{AssetID: assetID, Version: 1},
		Timeline: ShotTimeline{
			StartSeconds:    0,
			DurationSeconds: 4.8,
			InPointSeconds:  0,
			OutPointSeconds: 4.8,
		},
		Creative: ShotCreative{
			ShotType:   "close_up",
			Transition: "cut",
		},
		Planning: ShotPlanning{
			Score:       0.8,
			ReasonCodes: []string{"test"},
			Reason:      "test",
			Evidence:    []string{"fixture"},
		},
		Risks: []string{},
	}
}

func validSegment(segment Segment, label string, assetID contract.AssetID) SegmentPlan {
	return SegmentPlan{
		Segment:       segment,
		Label:         label,
		TargetSeconds: 10,
		ActualSeconds: 4.8,
		Clips: []Clip{{
			ID:              string(segment) + "_clip_1",
			Segment:         segment,
			AssetVersion:    contract.AssetVersionRef{AssetID: assetID, Version: 1},
			Label:           string(assetID) + " · v1",
			SourceType:      "upload",
			MimeType:        "video/mp4",
			Aspect:          "vertical",
			StartSeconds:    0,
			DurationSeconds: 4.8,
			InPointSeconds:  0,
			OutPointSeconds: 4.8,
			Score:           0.8,
			Reason:          "test",
		}},
	}
}

type fakeRenderOutputIntake struct {
	result  assets.GeneratedIntake
	key     contract.IdempotencyKey
	request assets.GeneratedAssetIntakeRequest
}

func (f *fakeRenderOutputIntake) Create(_ context.Context, _ contract.RequestContext, _ contract.ProjectID, key contract.IdempotencyKey, request assets.GeneratedAssetIntakeRequest) (assets.GeneratedIntake, error) {
	f.key = key
	f.request = request
	return f.result, nil
}
