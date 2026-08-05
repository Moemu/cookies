package creative

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestAnalyzeViralRemakePersistsFiveDimensionsAndSurvivesReload(t *testing.T) {
	t.Parallel()
	service, taskID := viralWorkflowTestService()
	service.ViralAnalyzer = stubViralAnalyzer{result: ViralAnalysisResult{
		Dimensions: []ViralAnalysisDimension{
			{ID: ViralTaskGoalType, Prompt: "15 秒转化广告", EvidenceRefs: []string{"timeline:0-15"}, Confidence: .9, Source: "ai_extracted"},
			{ID: ViralQualityStyleLighting, Prompt: "清晰商业光", EvidenceRefs: []string{"frame:1"}, Confidence: .8, Source: "ai_extracted"},
			{ID: ViralEnvironmentAtmosphere, Prompt: "冬日户外", EvidenceRefs: []string{"frame:2"}, Confidence: .8, Source: "ai_extracted"},
			{ID: ViralCameraContent, Prompt: "钩子、证明、CTA", EvidenceRefs: []string{"frame:3"}, Confidence: .9, Source: "ai_extracted"},
			{ID: ViralMusicSound, Prompt: "节奏递进", EvidenceRefs: []string{"asr:transcript"}, Confidence: .7, Source: "ai_extracted"},
		},
		PreserveRules: []string{"保留节奏功能"}, ReplaceRules: []string{"替换人物和品牌"},
		Transcript: "测试对白", Confidence: .82, EvidenceRefs: []string{"frame:1", "asr:transcript"},
		RouteRevisionID: "route_seed2_r1", PromptVersion: "viral.analyze.v1",
	}}
	actor := testRequestContext().Actor
	workspace, err := service.AnalyzeViralRemake(context.Background(), actor, "project_1", taskID)
	if err != nil {
		t.Fatal(err)
	}
	got := workspace.VideoDraft.ViralRemake
	if got.Status != ViralAnalysisReady || got.Revision != 2 || got.Analysis == nil ||
		len(got.Analysis.Dimensions) != 5 || got.PromptDraft == nil ||
		len(got.PromptDraft.Dimensions) != 5 || got.Readiness.GenerationReady {
		t.Fatalf("unexpected analyzed workspace: %+v", got)
	}
	reloaded, err := service.GetTaskDetail(context.Background(), actor, "project_1", taskID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.VideoDraft.ViralRemake.Analysis.ContentHash != got.Analysis.ContentHash {
		t.Fatal("analysis snapshot did not survive repository reload")
	}
}

func TestConfirmedViralPromptCreatesTraceableCandidateAndReview(t *testing.T) {
	t.Parallel()
	service, taskID := viralWorkflowTestService()
	service.ViralAnalyzer = stubViralAnalyzer{result: completeViralAnalysisResult()}
	actor := testRequestContext().Actor
	analyzed, err := service.AnalyzeViralRemake(context.Background(), actor, "project_1", taskID)
	if err != nil {
		t.Fatal(err)
	}
	dimensions := cloneViralDimensions(analyzed.VideoDraft.ViralRemake.PromptDraft.Dimensions)
	dimensions[ViralEnvironmentAtmosphere] = "清晨城市通勤场景"
	revised, err := service.UpdateViralPrompt(context.Background(), actor, "project_1", taskID, UpdateViralPromptRequest{
		ExpectedRevision: analyzed.VideoDraft.Revision, Dimensions: dimensions,
	})
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := service.ConfirmViralGeneration(context.Background(), actor, "project_1", taskID, ConfirmViralGenerationRequest{
		ExpectedRevision: revised.VideoDraft.Revision, ConfirmReferenceVideoRights: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed.VideoDraft.ViralRemake.Readiness.GenerationReady ||
		confirmed.VideoDraft.ViralRemake.PromptPackage == nil {
		t.Fatalf("prompt was not confirmed: %+v", confirmed.VideoDraft.ViralRemake)
	}
	input, promptHash, err := service.ViralProviderInput(context.Background(), actor, "project_1", taskID)
	if err != nil || input.InputMode != "text_only" || input.DurationSeconds != 15 || promptHash == "" {
		t.Fatalf("provider input = %+v, hash=%q, err=%v", input, promptHash, err)
	}
	registered, err := service.RegisterViralCandidateJob(context.Background(), actor, "project_1", taskID, "providerjob_viral_1")
	if err != nil {
		t.Fatal(err)
	}
	candidate := registered.VideoDraft.ViralRemake.Candidates[0]
	now := time.Date(2026, time.July, 28, 2, 0, 0, 0, time.UTC)
	reconciled, err := service.ReconcileViralCandidate(context.Background(), actor, "project_1", taskID, contract.ProviderJob{
		ID: "providerjob_viral_1", Kind: "provider.video.generate", OrganizationID: "org_1", ProjectID: "project_1",
		ExecutionStatus: contract.JobSucceeded, ProviderStatus: contract.ProviderJobSucceeded, Progress: 100,
		ProjectAssetRefs: []contract.ProjectAssetRef{{
			ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "asset_generated", Version: 1},
		}},
		AttemptCount: 1, MaxAttempts: 360, Version: 3, CreatedAt: now.Add(-time.Minute), UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reconciled.VideoDraft.ViralRemake.Readiness.ProductionReady ||
		reconciled.VideoDraft.ViralRemake.Candidates[0].OutputAssetRef == nil {
		t.Fatalf("candidate did not become production-ready: %+v", reconciled.VideoDraft.ViralRemake)
	}
	reviewed, err := service.SubmitViralCandidateReview(context.Background(), actor, "project_1", taskID, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.VideoDraft.ViralRemake.Status != ViralReadyForReview ||
		reviewed.VideoDraft.ViralRemake.Candidates[0].Status != ViralCandidateReviewed {
		t.Fatalf("candidate was not submitted for review: %+v", reviewed.VideoDraft.ViralRemake)
	}
}

func viralWorkflowTestService() (Service, string) {
	service := testService()
	repository := service.Repository.(*memoryRepository)
	service.ViralRemakes = repository
	now := service.now()
	taskID := "creativetask_viral"
	input := ViralRemakeInputSnapshot{
		Source: IntakeSourceManual, SelectedRouteID: ManualViralRemakeRouteID,
		ReferenceVideo: contract.AssetVersionRef{AssetID: "asset_video", Version: 1},
		ProductName:    "测试产品", SellingPoints: []string{"卖点"}, CallToAction: "立即了解",
		UserInstruction: "原创改写", MandatoryElements: []string{}, ProhibitedClaims: []string{},
		ReferenceVideoRights: RightsPending,
	}
	repository.tasks[taskID] = TaskDetail{
		Task: CreativeTask{
			ID: taskID, OrganizationID: "org_1", ProjectID: "project_1", IntakeID: "intake_viral",
			Format: FormatVideo, Channel: ChannelDouyin, PerformanceMode: PerformanceModeViralRemake,
			Status: TaskDraft, Version: 1, CreatedAt: now, UpdatedAt: now,
		},
		Intake: CreativeIntake{ID: "intake_viral", OrganizationID: "org_1", ProjectID: "project_1"},
		VideoDraft: &VideoDraft{
			ContractVersion: "creative-video-draft/v1", TaskID: taskID, Revision: 1,
			Concept: "爆款原创改写", Prompt: "等待分析", DurationSeconds: 15,
			AspectRatio: "9:16", Resolution: "720p",
			SourceVideo: input.ReferenceVideo, Mandatory: []string{}, Prohibited: []string{},
			CallToAction: input.CallToAction, CreatedAt: now,
			ViralRemake: &ViralRemakeDraft{
				ContractVersion: "creative-viral-remake-draft/v1", TaskID: taskID, Revision: 1,
				Status: ViralWaitingForAnalysis, SelectedRouteID: ManualViralRemakeRouteID,
				InputSnapshot: input, InputHash: "input-hash",
				Readiness:  CreativeReadiness{PlanningReady: true, MissingFields: []string{}, Blockers: []string{"analysis_snapshot", "confirmed_prompt_package", "reference_video_rights"}},
				Candidates: []ViralCandidate{}, CreatedAt: now, UpdatedAt: now,
			},
		},
		ProductionJobs: []ProductionJob{},
	}
	return service, taskID
}

type stubViralAnalyzer struct {
	result ViralAnalysisResult
	err    error
}

func (s stubViralAnalyzer) Analyze(context.Context, contract.ActorContext, contract.ProjectID, ViralAnalysisRequest) (ViralAnalysisResult, error) {
	return s.result, s.err
}

func completeViralAnalysisResult() ViralAnalysisResult {
	return ViralAnalysisResult{
		Dimensions: []ViralAnalysisDimension{
			{ID: ViralTaskGoalType, Prompt: "15 秒转化广告", EvidenceRefs: []string{"timeline:0-15"}, Confidence: .9, Source: "ai_extracted"},
			{ID: ViralQualityStyleLighting, Prompt: "清晰商业光", EvidenceRefs: []string{"frame:1"}, Confidence: .8, Source: "ai_extracted"},
			{ID: ViralEnvironmentAtmosphere, Prompt: "冬日户外", EvidenceRefs: []string{"frame:2"}, Confidence: .8, Source: "ai_extracted"},
			{ID: ViralCameraContent, Prompt: "钩子、证明、CTA", EvidenceRefs: []string{"frame:3"}, Confidence: .9, Source: "ai_extracted"},
			{ID: ViralMusicSound, Prompt: "节奏递进", EvidenceRefs: []string{"asr:transcript"}, Confidence: .7, Source: "ai_extracted"},
		},
		PreserveRules: []string{"保留节奏功能"}, ReplaceRules: []string{"替换人物和品牌"},
		Transcript: "测试对白", Confidence: .82, EvidenceRefs: []string{"frame:1", "asr:transcript"},
		RouteRevisionID: "route_seed2_r1", PromptVersion: "viral.analyze.v1",
	}
}
