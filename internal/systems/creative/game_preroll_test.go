package creative

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/media"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type gamePrerollV2AnalyzerStub struct {
	result GamePrerollV2AnalysisResult
	err    error
}

type inlineGameAnalysisLauncher struct{}

func (inlineGameAnalysisLauncher) Launch(work func()) { work() }

func (s gamePrerollV2AnalyzerStub) Analyze(context.Context, contract.ActorContext, contract.ProjectContext, contract.ProjectAssetRef) (GamePrerollV2AnalysisResult, error) {
	return s.result, s.err
}

func testGamePrerollV2Service(t *testing.T) (Service, contract.RequestContext, TaskDetail) {
	t.Helper()
	service := testService()
	service.ViralRemakes = service.Repository.(*memoryRepository)
	service.GamePrerollV2AnalysisLauncher = inlineGameAnalysisLauncher{}
	service.Assets = testAssetReader{snapshot: CreativeAssetSnapshot{Ref: contract.AssetVersionRef{AssetID: "asset_uploaded_gameplay", Version: 1}, Kind: contract.AssetVideo, MIMEType: "video/mp4", Ready: true, WidthPixels: 1080, HeightPixels: 1920, DurationMS: 45000, FrameRate: "30/1", VideoCodec: "h264", AudioCodec: "aac"}}
	rc := testRequestContext()
	detail, err := service.CreateGamePrerollV2Workspace(context.Background(), rc, "project_1", "game-v2-test", CreateGamePrerollV2WorkspaceRequest{SourceVideo: contract.AssetVersionRef{AssetID: "asset_uploaded_gameplay", Version: 1}, RightsConfirmed: true})
	if err != nil {
		t.Fatalf("create game workspace: %v", err)
	}
	return service, rc, detail
}

func TestPlanGamePrerollCandidateBatchBuildsThreeEvidenceGroundedCandidates(t *testing.T) {
	t.Parallel()

	snapshot := GamePrerollInputSnapshot{
		Source:          IntakeSourceManual,
		SelectedRouteID: ManualGamePrerollRouteID,
		BriefID:         "fixture_defend_sunflower_v1",
		BriefVersion:    1,
		BriefName:       "《保卫向日葵》技能选择挑战",
		GameName:        "保卫向日葵",
		GameplaySummary: "竖屏塔防战斗中，玩家在波次间选择技能来调整后续战斗能力。",
		CallToAction:    "立即下载",
		EvidenceMoments: []GameEvidenceMoment{
			{
				ID: "skill_choice_1", Kind: GameEvidenceSkillChoice,
				StartMilliseconds: 20292, EndMilliseconds: 22250,
				Description:  "第 1 次技能三选一，画面展示幻系易伤、怪物易伤、获得格子。",
				VerifiedCopy: []string{"幻系易伤", "怪物易伤", "获得格子"},
			},
			{
				ID: "skill_choice_2", Kind: GameEvidenceSkillChoice,
				StartMilliseconds: 29792, EndMilliseconds: 31375,
				Description:  "第 2 次技能三选一，画面展示激光弹射、格子概率、全体加攻。",
				VerifiedCopy: []string{"激光弹射", "格子概率", "全体加攻"},
			},
			{
				ID: "wave_2", Kind: GameEvidenceWaveProgress,
				StartMilliseconds: 34000, EndMilliseconds: 35500,
				Description:  "选择完成后进入第 2/10 波。",
				VerifiedCopy: []string{"第2/10波"},
			},
		},
		AllowedMechanisms: []GameHookMechanism{
			GameHookChoiceChallenge,
			GameHookTacticalTradeoff,
			GameHookWaveEscalation,
		},
		ProhibitedMechanisms: []GameHookMechanism{
			GameHookFailureReversal,
			GameHookMergeUpgrade,
			GameHookRewardReveal,
		},
	}

	batch, err := planGamePrerollCandidateBatch(
		snapshot,
		"sha256:fixture",
		"game_batch_1",
		1,
		GamePrerollGenerationConfig{
			SubtitleStyle: "high_contrast_dynamic",
			HookStrength:  4,
			PaceProfile:   "punchy",
		},
		time.Unix(1, 0).UTC(),
	)
	if err != nil {
		t.Fatalf("plan game preroll candidates: %v", err)
	}
	if len(batch.Candidates) != 3 {
		t.Fatalf("candidate count = %d, want 3", len(batch.Candidates))
	}

	seenMechanisms := map[GameHookMechanism]bool{}
	for _, candidate := range batch.Candidates {
		if candidate.PromptPackage.ContentHash == "" || candidate.PromptPackage.CompiledPrompt == "" {
			t.Fatalf("candidate %q is missing a compiled PromptPackage", candidate.ID)
		}
		if candidate.PromptPackage.InputSnapshotHash != "sha256:fixture" {
			t.Fatalf("candidate %q input snapshot hash = %q", candidate.ID, candidate.PromptPackage.InputSnapshotHash)
		}
		if len(candidate.EvidenceMomentIDs) == 0 {
			t.Fatalf("candidate %q is not grounded in evidence", candidate.ID)
		}
		if !containsGameHookMechanism(snapshot.AllowedMechanisms, candidate.HookMechanism) {
			t.Fatalf("candidate %q used unapproved mechanism %q", candidate.ID, candidate.HookMechanism)
		}
		if containsGameHookMechanism(snapshot.ProhibitedMechanisms, candidate.HookMechanism) {
			t.Fatalf("candidate %q used prohibited mechanism %q", candidate.ID, candidate.HookMechanism)
		}
		if len(candidate.Storyboard) != 3 ||
			candidate.Storyboard[0].StartMilliseconds != 0 ||
			candidate.Storyboard[2].EndMilliseconds != 6000 {
			t.Fatalf("candidate %q storyboard does not cover one 6-second video: %#v", candidate.ID, candidate.Storyboard)
		}
		seenMechanisms[candidate.HookMechanism] = true
	}
	if len(seenMechanisms) != 3 {
		t.Fatalf("candidate mechanisms are not diverse: %#v", seenMechanisms)
	}
}

func TestCreateGamePrerollV2WorkspaceFreezesSourceAndStartsBeforeAnalysis(t *testing.T) {
	t.Parallel()

	service := testService()
	service.ViralRemakes = service.Repository.(*memoryRepository)
	source := contract.AssetVersionRef{AssetID: "asset_uploaded_gameplay", Version: 1}
	service.Assets = testAssetReader{snapshot: CreativeAssetSnapshot{
		Ref: source, Kind: contract.AssetVideo, MIMEType: "video/mp4", Ready: true,
		WidthPixels: 1080, HeightPixels: 1920, DurationMS: 45000,
		FrameRate: "30/1", VideoCodec: "h264", AudioCodec: "aac",
	}}
	rc := testRequestContext()

	detail, err := service.CreateGamePrerollV2Workspace(context.Background(), rc, "project_1", "game-v2-create", CreateGamePrerollV2WorkspaceRequest{
		SourceVideo: source, RightsConfirmed: true,
	})
	if err != nil {
		t.Fatalf("create game preroll V2 workspace: %v", err)
	}
	workspace := detail.VideoDraft.GamePreroll
	if workspace == nil || workspace.ContractVersion != GamePrerollV2ContractVersion || workspace.Stage != GamePrerollStageSourceReady {
		t.Fatalf("unexpected workspace: %#v", workspace)
	}
	if workspace.InputSnapshot.SourceVideo != source || workspace.SourceMetadata.DurationMS != 45000 || workspace.GenerationConfig.DurationSeconds != 8 {
		t.Fatalf("source or defaults were not frozen: %#v", workspace)
	}
	if workspace.Analysis.Status != GamePrerollResourceIdle || workspace.Readiness.PlanningReady || workspace.Readiness.GenerationReady || len(workspace.Candidates) != 0 {
		t.Fatalf("workspace skipped analysis and human confirmation gates: %#v", workspace)
	}
}

func TestGamePrerollV2AnalysisBriefAndCandidatePlanningPreserveProvenance(t *testing.T) {
	t.Parallel()

	service, rc, detail := testGamePrerollV2Service(t)
	service.GamePrerollV2Analyzer = gamePrerollV2AnalyzerStub{result: GamePrerollV2AnalysisResult{
		InputHash: "sha256:analysis", PromptVersion: "game-preroll-analysis/v1", GameName: "测试游戏", GameplaySummary: "玩家选择技能并立即进入战斗，画面展示清晰操作反馈。",
		Facts:          []GameAnalysisFact{{ID: "gameplay", Label: "核心玩法", Value: "技能选择后战斗", Provenance: GameProvenanceVideo, EvidenceRefs: []string{"operation"}}},
		Evidence:       []GameEvidenceMoment{{ID: "operation", Kind: GameEvidenceSkillChoice, StartMilliseconds: 1000, EndMilliseconds: 2500, Description: "玩家选择一个技能", VerifiedCopy: []string{"选择技能"}}, {ID: "result", Kind: GameEvidenceBattle, StartMilliseconds: 3000, EndMilliseconds: 5000, Description: "战斗反馈增强", VerifiedCopy: []string{"战斗反馈"}}, {ID: "ending", Kind: GameEvidenceWaveProgress, StartMilliseconds: 5500, EndMilliseconds: 7000, Description: "进入下一阶段", VerifiedCopy: []string{"下一阶段"}}},
		SuggestedBrief: []GameBriefField{{ID: "objective", Key: "objective", Label: "广告目标", Value: "促进下载", Provenance: GameProvenanceAI, Required: true}, {ID: "audience", Key: "audience", Label: "目标受众", Value: "喜欢即时反馈的玩家", Provenance: GameProvenanceAI, Required: true, EvidenceRefs: []string{"result"}}, {ID: "selling", Key: "selling_point", Label: "主推卖点", Value: "选择立即影响战斗", Provenance: GameProvenanceVideo, Required: true, EvidenceRefs: []string{"operation", "result"}}, {ID: "cta", Key: "cta", Label: "CTA", Value: "立即下载", Provenance: GameProvenanceManual, Required: true}},
	}}
	analyzed, err := service.AnalyzeGamePrerollV2Source(context.Background(), rc.Actor, "project_1", detail.Task.ID, AnalyzeGamePrerollV2SourceRequest{ExpectedRevision: detail.VideoDraft.Revision})
	if err != nil {
		t.Fatalf("analyze game source: %v", err)
	}
	analyzed, err = service.GetGamePrerollV2Workspace(context.Background(), rc.Actor, "project_1", detail.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := service.ConfirmGamePrerollV2Brief(context.Background(), rc.Actor, "project_1", detail.Task.ID, ConfirmGamePrerollV2BriefRequest{ExpectedRevision: analyzed.VideoDraft.Revision, Fields: analyzed.VideoDraft.GamePreroll.AnalysisSuggestedBrief()})
	if err != nil {
		t.Fatalf("confirm game brief: %v", err)
	}
	planned, err := service.PlanGamePrerollV2Candidates(context.Background(), rc.Actor, "project_1", detail.Task.ID, PlanGamePrerollV2CandidatesRequest{ExpectedRevision: confirmed.VideoDraft.Revision})
	if err != nil {
		t.Fatalf("plan candidates: %v", err)
	}
	w := planned.VideoDraft.GamePreroll
	if w.ConfirmedBrief == nil || len(w.Candidates) != 3 || w.SelectedCandidateID != "" || w.Stage != GamePrerollStageCandidatesReady || w.ConfirmedBrief.Fields[2].Provenance != GameProvenanceVideo {
		t.Fatalf("unexpected planned workspace: %#v", w)
	}
}

func TestGamePrerollV2AnalysisFailureIsRetryableWithoutLosingSource(t *testing.T) {
	t.Parallel()
	service, rc, detail := testGamePrerollV2Service(t)
	service.GamePrerollV2Analyzer = gamePrerollV2AnalyzerStub{err: errors.New("model unavailable")}
	failed, err := service.AnalyzeGamePrerollV2Source(context.Background(), rc.Actor, "project_1", detail.Task.ID, AnalyzeGamePrerollV2SourceRequest{ExpectedRevision: detail.VideoDraft.Revision})
	if err != nil {
		t.Fatal(err)
	}
	failed, err = service.GetGamePrerollV2Workspace(context.Background(), rc.Actor, "project_1", detail.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	w := failed.VideoDraft.GamePreroll
	if w.Analysis.Status != GamePrerollResourceFailed || w.Analysis.ErrorCode != "GAME_SOURCE_ANALYSIS_FAILED" || w.InputSnapshot.SourceVideo.AssetID != "asset_uploaded_gameplay" || w.Stage != GamePrerollStageSourceReady {
		t.Fatalf("failed analysis did not retain retryable source: %#v", w)
	}
}

func TestGamePrerollV2ModelFailureFallsBackToGenericEvidenceCandidates(t *testing.T) {
	t.Parallel()
	service, rc, detail := testGamePrerollV2Service(t)
	service.GamePrerollV2Analyzer = gamePrerollV2AnalyzerStub{result: GamePrerollV2AnalysisResult{
		InputHash: "sha256:analysis", PromptVersion: "game-preroll-analysis/v1", GameName: "任意游戏", GameplaySummary: "玩家完成一次操作后，画面呈现连续且可以核验的反馈。",
		Evidence:       []GameEvidenceMoment{{ID: "op", Kind: GameEvidenceOperation, StartMilliseconds: 1000, EndMilliseconds: 2000, Description: "操作", VerifiedCopy: []string{}}, {ID: "feedback", Kind: GameEvidenceResult, StartMilliseconds: 2200, EndMilliseconds: 3400, Description: "反馈", VerifiedCopy: []string{}}, {ID: "ending", Kind: GameEvidenceGameplay, StartMilliseconds: 3600, EndMilliseconds: 5200, Description: "后续", VerifiedCopy: []string{}}},
		SuggestedBrief: []GameBriefField{{ID: "objective", Key: "objective", Label: "广告目标", Value: "下载", Provenance: GameProvenanceAI, Required: true, EvidenceRefs: []string{}}, {ID: "audience", Key: "audience", Label: "目标受众", Value: "玩家", Provenance: GameProvenanceAI, Required: true, EvidenceRefs: []string{}}, {ID: "selling", Key: "selling_point", Label: "卖点", Value: "操作反馈", Provenance: GameProvenanceVideo, Required: true, EvidenceRefs: []string{"op", "feedback"}}, {ID: "cta", Key: "cta", Label: "CTA", Value: "立即下载", Provenance: GameProvenanceManual, Required: true, EvidenceRefs: []string{}}},
	}}
	analyzed, err := service.AnalyzeGamePrerollV2Source(context.Background(), rc.Actor, "project_1", detail.Task.ID, AnalyzeGamePrerollV2SourceRequest{ExpectedRevision: detail.VideoDraft.Revision})
	if err != nil {
		t.Fatal(err)
	}
	analyzed, err = service.GetGamePrerollV2Workspace(context.Background(), rc.Actor, "project_1", detail.Task.ID)
	if err != nil {
		t.Fatal(err)
	}
	confirmed, err := service.ConfirmGamePrerollV2Brief(context.Background(), rc.Actor, "project_1", detail.Task.ID, ConfirmGamePrerollV2BriefRequest{ExpectedRevision: analyzed.VideoDraft.Revision, Fields: analyzed.VideoDraft.GamePreroll.AnalysisSuggestedBrief()})
	if err != nil {
		t.Fatal(err)
	}
	service.GamePrerollPlanner = FallbackGamePrerollPlanner{Primary: gamePlannerFailureStub{err: errors.New("model unavailable")}, Fallback: GenericGamePrerollPlanner{}}
	planned, err := service.PlanGamePrerollV2Candidates(context.Background(), rc.Actor, "project_1", detail.Task.ID, PlanGamePrerollV2CandidatesRequest{ExpectedRevision: confirmed.VideoDraft.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.VideoDraft.GamePreroll.Candidates) != 3 || !strings.Contains(planned.VideoDraft.GamePreroll.ActiveCandidateBatch.PlannerVersion, "game-preroll-evidence-fallback/v2") {
		t.Fatalf("generic fallback not used: %#v", planned.VideoDraft.GamePreroll.ActiveCandidateBatch)
	}
	for _, candidate := range planned.VideoDraft.GamePreroll.Candidates {
		if !gameEvidenceIDsExist(analyzed.VideoDraft.GamePreroll.Analysis.Evidence, candidate.EvidenceMomentIDs) {
			t.Fatalf("candidate used fixture evidence: %#v", candidate)
		}
	}
}

func TestGamePrerollV2SelectedCandidateCompilesEightSecondSpecAndReconcilesAsset(t *testing.T) {
	t.Parallel()
	service, rc, detail := testGamePrerollV2Service(t)
	workspace := detail.VideoDraft.GamePreroll
	workspace.Analysis = GamePrerollAnalysis{Status: GamePrerollResourceReady, Revision: 1, GameName: "测试游戏", GameplaySummary: "真实游戏操作产生即时可见结果", Evidence: []GameEvidenceMoment{{ID: "e1", Kind: GameEvidenceBattle, StartMilliseconds: 1000, EndMilliseconds: 2000, Description: "操作", VerifiedCopy: []string{"操作"}}, {ID: "e2", Kind: GameEvidenceBattle, StartMilliseconds: 2500, EndMilliseconds: 4000, Description: "反馈", VerifiedCopy: []string{"反馈"}}, {ID: "e3", Kind: GameEvidenceWaveProgress, StartMilliseconds: 4500, EndMilliseconds: 6000, Description: "结果", VerifiedCopy: []string{"结果"}}}}
	workspace.InputSnapshot.GameName = "测试游戏"
	workspace.InputSnapshot.GameplaySummary = "真实游戏操作产生即时可见结果"
	workspace.InputSnapshot.EvidenceMoments = workspace.Analysis.Evidence
	workspace.InputSnapshot.AllowedMechanisms = []GameHookMechanism{GameHookChoiceChallenge, GameHookTacticalTradeoff, GameHookWaveEscalation}
	workspace.InputSnapshot.ProhibitedMechanisms = []GameHookMechanism{GameHookMergeUpgrade, GameHookRewardReveal}
	workspace.InputSnapshot.CallToAction = "立即下载"
	workspace.ConfirmedBrief = &GameBriefVersion{ID: "brief", Version: 1, AnalysisRevision: 1, Fields: []GameBriefField{{ID: "objective", Key: "objective", Value: "下载", Required: true}, {ID: "audience", Key: "audience", Value: "玩家", Required: true}, {ID: "selling", Key: "selling_point", Value: "即时反馈", Required: true}, {ID: "cta", Key: "cta", Value: "立即下载", Required: true}}, ConfirmedBy: rc.Actor.Principal.ID, ConfirmedAt: time.Now(), ContentHash: "sha256:brief"}
	batch, err := planGenericGameCandidateBatch(workspace.InputSnapshot, workspace.InputHash, detail.Task.ID+"_batch", 2, workspace.GenerationConfig, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	workspace.ActiveCandidateBatch = &batch
	workspace.Candidates = batch.Candidates
	workspace.Stage = GamePrerollStageCandidatesReady
	workspace.Readiness.PlanningReady = true
	workspace.EvidenceAssets = &GameEvidenceAssetSet{SourceVideo: workspace.InputSnapshot.SourceVideo, Status: "ready", Frames: []GameEvidenceFrameAsset{{EvidenceMomentID: "e1", FrameAsset: contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "frame1", Version: 1}}}, {EvidenceMomentID: "e2", FrameAsset: contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "frame2", Version: 1}}}, {EvidenceMomentID: "e3", FrameAsset: contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "frame3", Version: 1}}}}, ContentHash: "sha256:frames"}
	repo := service.Repository.(*memoryRepository)
	stored := repo.tasks[detail.Task.ID]
	next := *stored.VideoDraft
	next.GamePreroll = workspace
	repo.tasks[detail.Task.ID] = TaskDetail{Task: stored.Task, Intake: stored.Intake, VideoDraft: &next}
	selected, err := service.SelectGamePrerollCandidate(context.Background(), rc.Actor, "project_1", detail.Task.ID, SelectGamePrerollCandidateRequest{ExpectedRevision: next.Revision, CandidateID: batch.Candidates[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if selected.VideoDraft.GamePreroll.GenerationSpec.DurationSeconds != 8 {
		t.Fatalf("duration=%d", selected.VideoDraft.GamePreroll.GenerationSpec.DurationSeconds)
	}
	registered, err := service.RegisterGamePrerollV2VideoJob(context.Background(), rc.Actor, "project_1", detail.Task.ID, selected.VideoDraft.Revision, "provider_job_game_1")
	if err != nil {
		t.Fatal(err)
	}
	output := contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "asset_output", Version: 1}}
	completed, err := service.ReconcileGamePrerollV2Video(context.Background(), rc.Actor, "project_1", detail.Task.ID, ReconcileGamePrerollV2VideoRequest{ExpectedRevision: registered.VideoDraft.Revision, Job: contract.ProviderJob{ID: "provider_job_game_1", ProjectID: "project_1", ProviderStatus: contract.ProviderJobSucceeded, ProjectAssetRefs: []contract.ProjectAssetRef{output}}})
	if err != nil {
		t.Fatal(err)
	}
	if completed.VideoDraft.GamePreroll.OutputAsset == nil || completed.VideoDraft.GamePreroll.OutputAsset.AssetVersion != output.AssetVersion || completed.VideoDraft.GamePreroll.Stage != GamePrerollStageVideoReady {
		t.Fatalf("unexpected completion: %#v", completed.VideoDraft.GamePreroll)
	}
	if len(completed.GamePrerollGenerationAttempts) != 1 || completed.GamePrerollGenerationAttempts[0].ProviderJobID != "provider_job_game_1" || completed.GamePrerollGenerationAttempts[0].OutputAssetVersion == nil || *completed.GamePrerollGenerationAttempts[0].OutputAssetVersion != output.AssetVersion {
		t.Fatalf("attempt lineage was not completed: %#v", completed.GamePrerollGenerationAttempts)
	}
}

func TestPrepareGamePrerollV2EvidenceRetainsWorkspaceContractAndSelectedStage(t *testing.T) {
	t.Parallel()
	service, rc, detail := testGamePrerollV2Service(t)
	workspace := detail.VideoDraft.GamePreroll
	workspace.InputSnapshot.GameName = "测试游戏"
	workspace.InputSnapshot.GameplaySummary = "玩家操作后立即得到真实可见的战斗反馈"
	workspace.InputSnapshot.EvidenceMoments = []GameEvidenceMoment{
		{ID: "e1", Kind: GameEvidenceOperation, StartMilliseconds: 1000, EndMilliseconds: 1800, Description: "操作", VerifiedCopy: []string{}},
		{ID: "e2", Kind: GameEvidenceResult, StartMilliseconds: 2200, EndMilliseconds: 3200, Description: "反馈", VerifiedCopy: []string{}},
		{ID: "e3", Kind: GameEvidenceGameplay, StartMilliseconds: 3600, EndMilliseconds: 4600, Description: "结果", VerifiedCopy: []string{}},
	}
	workspace.InputSnapshot.AllowedMechanisms = []GameHookMechanism{GameHookChoiceChallenge, GameHookTacticalTradeoff, GameHookWaveEscalation}
	workspace.InputSnapshot.ProhibitedMechanisms = []GameHookMechanism{GameHookMergeUpgrade, GameHookRewardReveal}
	batch, err := planGenericGameCandidateBatch(workspace.InputSnapshot, workspace.InputHash, detail.Task.ID+"_batch", 2, workspace.GenerationConfig, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	workspace.ActiveCandidateBatch, workspace.Candidates, workspace.SelectedCandidateID = &batch, batch.Candidates, batch.Candidates[0].ID
	workspace.Stage = GamePrerollStageCandidateSelected
	repo := service.Repository.(*memoryRepository)
	stored := repo.tasks[detail.Task.ID]
	next := *stored.VideoDraft
	next.GamePreroll = workspace
	repo.tasks[detail.Task.ID] = TaskDetail{Task: stored.Task, Intake: stored.Intake, VideoDraft: &next}
	service.GameEvidenceFrames = gameFrameExtractor{}
	service.DerivedAssets = &gameDerivedWriter{}
	prepared, err := service.PrepareGamePrerollEvidence(context.Background(), rc, "project_1", detail.Task.ID, PrepareGamePrerollEvidenceRequest{ExpectedRevision: next.Revision})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.VideoDraft.GamePreroll.ContractVersion != GamePrerollV2ContractVersion || prepared.VideoDraft.GamePreroll.Stage != GamePrerollStageCandidateSelected || !prepared.VideoDraft.GamePreroll.Readiness.GenerationReady {
		t.Fatalf("V2 evidence preparation corrupted workspace: %#v", prepared.VideoDraft.GamePreroll)
	}
}

func TestUpdateGamePrerollV2GenerationConfigInvalidatesCandidatesAndKeepsConfirmedBrief(t *testing.T) {
	t.Parallel()
	service, rc, detail := testGamePrerollV2Service(t)
	workspace := detail.VideoDraft.GamePreroll
	workspace.Analysis = GamePrerollAnalysis{Status: GamePrerollResourceReady, Revision: 1}
	workspace.ConfirmedBrief = &GameBriefVersion{ID: "brief_1", Version: 1, AnalysisRevision: 1, Fields: []GameBriefField{}, ConfirmedBy: rc.Actor.Principal.ID, ConfirmedAt: time.Now(), ContentHash: "sha256:brief"}
	workspace.ActiveCandidateBatch = &GameCandidateBatch{ID: "old_batch"}
	workspace.Candidates = []GamePrerollCandidate{{ID: "old_candidate"}}
	workspace.SelectedCandidateID = "old_candidate"
	workspace.OutputAsset = &contract.ProjectAssetRef{ProjectID: "project_1", AssetVersion: contract.AssetVersionRef{AssetID: "old_output", Version: 1}}
	workspace.Stage = GamePrerollStageVideoReady
	repo := service.Repository.(*memoryRepository)
	stored := repo.tasks[detail.Task.ID]
	next := *stored.VideoDraft
	next.GamePreroll = workspace
	repo.tasks[detail.Task.ID] = TaskDetail{Task: stored.Task, Intake: stored.Intake, VideoDraft: &next}
	updated, err := service.UpdateGamePrerollV2GenerationConfig(context.Background(), rc.Actor, "project_1", detail.Task.ID, UpdateGamePrerollV2GenerationConfigRequest{ExpectedRevision: next.Revision, Config: GamePrerollGenerationConfig{SubtitleStyle: "high_contrast_dynamic", HookStrength: 5, PaceProfile: "balanced", DurationSeconds: 10, Channel: "douyin", AspectRatio: "9:16", Resolution: "720p", AudioPolicy: "generated", CallToAction: "立即试玩"}})
	if err != nil {
		t.Fatal(err)
	}
	w := updated.VideoDraft.GamePreroll
	if w.ConfirmedBrief == nil || w.Stage != GamePrerollStageBriefConfirmed || w.GenerationConfig.DurationSeconds != 10 || w.InputSnapshot.CallToAction != "立即试玩" || w.ActiveCandidateBatch != nil || len(w.Candidates) != 0 || w.SelectedCandidateID != "" || w.OutputAsset != nil {
		t.Fatalf("config invalidation is incomplete: %#v", w)
	}
}

func TestGamePrerollV2CompilesEverySupportedDuration(t *testing.T) {
	t.Parallel()
	snapshot := GamePrerollInputSnapshot{GameName: "测试游戏", GameplaySummary: "玩家完成操作并得到连续可见反馈", CallToAction: "立即下载", EvidenceMoments: []GameEvidenceMoment{{ID: "e1", Kind: GameEvidenceOperation, StartMilliseconds: 0, EndMilliseconds: 1000, Description: "操作", VerifiedCopy: []string{}}, {ID: "e2", Kind: GameEvidenceResult, StartMilliseconds: 1200, EndMilliseconds: 2200, Description: "反馈", VerifiedCopy: []string{}}, {ID: "e3", Kind: GameEvidenceGameplay, StartMilliseconds: 2400, EndMilliseconds: 3400, Description: "结果", VerifiedCopy: []string{}}}, AllowedMechanisms: []GameHookMechanism{GameHookChoiceChallenge, GameHookTacticalTradeoff, GameHookWaveEscalation}, ProhibitedMechanisms: []GameHookMechanism{GameHookFailureReversal, GameHookMergeUpgrade, GameHookRewardReveal}}
	for duration := 6; duration <= 10; duration++ {
		config := GamePrerollGenerationConfig{SubtitleStyle: "high_contrast_dynamic", HookStrength: 4, PaceProfile: "punchy", DurationSeconds: duration, Channel: "douyin", AspectRatio: "9:16", Resolution: "720p", AudioPolicy: "generated", CallToAction: "立即下载"}
		batch, err := planGenericGameCandidateBatch(snapshot, "sha256:input", fmt.Sprintf("batch_%d", duration), 1, config, time.Now())
		if err != nil {
			t.Fatalf("duration %d: %v", duration, err)
		}
		for _, candidate := range batch.Candidates {
			beats := candidate.Storyboard
			if beats[0].StartMilliseconds != 0 || beats[len(beats)-1].EndMilliseconds != duration*1000 {
				t.Fatalf("duration %d is not fully covered: %#v", duration, beats)
			}
			for index := 1; index < len(beats); index++ {
				if beats[index-1].EndMilliseconds != beats[index].StartMilliseconds {
					t.Fatalf("duration %d has a beat gap: %#v", duration, beats)
				}
			}
		}
	}
}

func TestModelGamePrerollPlannerUsesStructuredOutlinesButServerCompilesPrompts(t *testing.T) {
	t.Parallel()

	structured := json.RawMessage(`{
		"candidates": [
			{
				"hook_mechanism": "tactical_tradeoff",
				"execution_angle": "model_tradeoff",
				"primary_test_variable": "skill_names",
				"variant_hypothesis": "真实技能名能让选择更具体。",
				"hook_line": "获得格子，还是全体加攻？",
				"evidence_moment_ids": ["skill_choice_1", "skill_choice_2", "wave_2"],
				"compiled_prompt": "UNTRUSTED MODEL PROMPT"
			},
			{
				"hook_mechanism": "wave_escalation",
				"execution_angle": "model_wave",
				"primary_test_variable": "wave_pressure",
				"variant_hypothesis": "下一波信息能建立紧迫感。",
				"hook_line": "第 2 波开始前，你选哪个？",
				"evidence_moment_ids": ["wave_2", "skill_choice_2"]
			},
			{
				"hook_mechanism": "choice_challenge",
				"execution_angle": "model_choice",
				"primary_test_variable": "direct_question",
				"variant_hypothesis": "直接提问能邀请用户参与判断。",
				"hook_line": "三个技能只能选一个。",
				"evidence_moment_ids": ["skill_choice_1", "skill_choice_2"]
			}
		]
	}`)
	text := &gamePlannerTextStub{response: provider.SynchronousResponse{
		ProviderCode: "adapter_gateway", ModelAlias: "Seed-2-pro",
		ModelVersion: "doubao-seed-2-0-pro-260215", StructuredOutput: structured,
	}}
	planner := ModelGamePrerollPlanner{Text: text, ModelAlias: "Seed-2-pro"}
	snapshot := testGamePrerollSnapshot()

	batch, err := planner.Plan(
		context.Background(),
		contract.ActorContext{
			OrganizationID: "org_1",
			Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"},
		},
		contract.ProjectContext{
			OrganizationID: "org_1", ProjectID: "project_1",
			BrandID:    func() *contract.BrandID { value := contract.BrandID("brand_1"); return &value }(),
			ProductIDs: []contract.ProductID{}, ProjectContextVersion: 1,
		},
		snapshot,
		"sha256:fixture",
		"game_batch_model",
		1,
		GamePrerollGenerationConfig{SubtitleStyle: "high_contrast_dynamic", HookStrength: 4, PaceProfile: "punchy"},
		time.Unix(2, 0).UTC(),
	)
	if err != nil {
		t.Fatalf("model plan: %v", err)
	}
	if text.request.ModelAlias != "Seed-2-pro" || len(text.request.OutputJSONSchema) == 0 {
		t.Fatalf("text request = %#v", text.request)
	}
	if batch.PlannerVersion != "model:adapter_gateway/doubao-seed-2-0-pro-260215" {
		t.Fatalf("planner version = %q", batch.PlannerVersion)
	}
	if batch.Candidates[0].ExecutionAngle != "model_tradeoff" ||
		batch.Candidates[0].PromptPackage.CompiledPrompt == "UNTRUSTED MODEL PROMPT" ||
		!strings.Contains(batch.Candidates[0].PromptPackage.CompiledPrompt, "严禁虚构") {
		t.Fatalf("server did not compile the model outline safely: %#v", batch.Candidates[0])
	}
}

type gamePlannerTextStub struct {
	request  provider.TextGenerateRequest
	response provider.SynchronousResponse
	err      error
}

func (s *gamePlannerTextStub) GenerateText(_ context.Context, request provider.TextGenerateRequest) (provider.SynchronousResponse, error) {
	s.request = request
	return s.response, s.err
}

func TestFallbackGamePrerollPlannerReportsPrimaryFailure(t *testing.T) {
	t.Parallel()

	primaryErr := errors.New("model output invalid")
	var reported error
	planner := FallbackGamePrerollPlanner{
		Primary:  gamePlannerFailureStub{err: primaryErr},
		Fallback: DeterministicGamePrerollPlanner{},
		OnPrimaryFailure: func(err error) {
			reported = err
		},
	}
	batch, err := planner.Plan(
		context.Background(),
		contract.ActorContext{},
		contract.ProjectContext{},
		testGamePrerollSnapshot(),
		"sha256:fixture",
		"game_batch_fallback",
		1,
		GamePrerollGenerationConfig{SubtitleStyle: "high_contrast_dynamic", HookStrength: 4, PaceProfile: "punchy"},
		time.Unix(3, 0).UTC(),
	)
	if err != nil {
		t.Fatalf("fallback plan: %v", err)
	}
	if !errors.Is(reported, primaryErr) || !strings.HasPrefix(batch.PlannerVersion, "fallback:") {
		t.Fatalf("reported = %v, planner version = %q", reported, batch.PlannerVersion)
	}
}

type gamePlannerFailureStub struct {
	err error
}

func (s gamePlannerFailureStub) Plan(
	context.Context,
	contract.ActorContext,
	contract.ProjectContext,
	GamePrerollInputSnapshot,
	string,
	string,
	int64,
	GamePrerollGenerationConfig,
	time.Time,
) (GameCandidateBatch, error) {
	return GameCandidateBatch{}, s.err
}

func TestManualGamePrerollRequiresHumanSelectionBeforeVideoGeneration(t *testing.T) {
	t.Parallel()

	service := testService()
	service.ViralRemakes = service.Repository.(*memoryRepository)
	sourceVideo := contract.AssetVersionRef{AssetID: "asset_gameplay", Version: 1}
	service.Assets = testAssetReader{snapshot: CreativeAssetSnapshot{
		Ref: sourceVideo, Kind: contract.AssetVideo, MIMEType: "video/mp4", Ready: true,
	}}
	service.GameEvidenceFrames = gameFrameExtractor{}
	service.DerivedAssets = &gameDerivedWriter{}
	rc := testRequestContext()
	manual := testManualGamePrerollInput(sourceVideo)

	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "game-preroll-intake-1", CreateIntakeRequest{
		Source: IntakeSourceManual, Format: FormatVideo, PerformanceMode: PerformanceModeGamePreroll,
		Channel: ChannelDouyin, Objective: "用真实技能选择建立挑战感并引导下载",
		Audience:     "喜欢轻策略塔防和技能搭配的竖屏游戏用户",
		CoreMessage:  "每波技能选择都会改变接下来的塔防决策",
		CallToAction: "立即下载", Concept: "《保卫向日葵》技能选择挑战",
		Tone: []string{"紧张", "可读"}, VisualKeywords: []string{"真实玩法", "技能三选一"},
		Mandatory: []string{"保留真实技能名和波次"}, Prohibited: []string{"不得虚构失败、奖励或升级结果"},
		CreativeRoutes: []CreativeRouteSnapshot{{
			RouteID: ManualGamePrerollRouteID, RouteType: PerformanceModeGamePreroll,
			VideoPurpose: "performance", Channels: []string{"douyin"},
			Reason: "用户确认使用本地游戏前贴固定样例", TargetDurationSeconds: 6,
			AspectRatio: "9:16", SourceAssetRefs: []contract.AssetVersionRef{sourceVideo},
			EvidenceRefs:              []string{"skill_choice_1", "skill_choice_2", "wave_2"},
			RequiresHumanConfirmation: true,
		}},
		ManualGamePreroll: &manual,
	})
	if err != nil {
		t.Fatalf("create game intake: %v", err)
	}
	task, err := service.CreateVideoTask(context.Background(), rc.Actor, "project_1", intake.ID, CreateVideoTaskRequest{
		SelectedRouteID: ManualGamePrerollRouteID, Channel: ChannelDouyin, SourceVideo: sourceVideo,
		Concept: "《保卫向日葵》技能选择挑战", Prompt: "等待人工选择候选",
		CallToAction: "立即下载", Mandatory: []string{"保留真实技能名和波次"},
		Prohibited: []string{"不得虚构失败、奖励或升级结果"}, ConfirmRoute: true,
	})
	if err != nil {
		t.Fatalf("create game task: %v", err)
	}
	detail, err := service.GetTaskDetail(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	game := detail.VideoDraft.GamePreroll
	if game == nil || len(game.Candidates) != 3 || game.Readiness.GenerationReady {
		t.Fatalf("game preroll draft = %#v", game)
	}
	if _, _, err := service.GamePrerollProviderInput(context.Background(), rc.Actor, "project_1", task.ID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("provider input before human selection error = %v, want ErrInvalidState", err)
	}
	prepared, err := service.PrepareGamePrerollEvidence(context.Background(), rc, "project_1", task.ID, PrepareGamePrerollEvidenceRequest{ExpectedRevision: detail.VideoDraft.Revision})
	if err != nil {
		t.Fatalf("prepare evidence: %v", err)
	}
	game = prepared.VideoDraft.GamePreroll
	if game.ContractVersion != "creative-game-preroll-draft/v2" || game.EvidenceAssets == nil || len(game.EvidenceAssets.Frames) != 3 {
		t.Fatalf("prepared evidence = %#v", game.EvidenceAssets)
	}

	selected, err := service.SelectGamePrerollCandidate(context.Background(), rc.Actor, "project_1", task.ID, SelectGamePrerollCandidateRequest{
		ExpectedRevision: prepared.VideoDraft.Revision,
		CandidateID:      game.Candidates[1].ID,
	})
	if err != nil {
		t.Fatalf("select game candidate: %v", err)
	}
	if selected.VideoDraft.GamePreroll.SelectedCandidateID != game.Candidates[1].ID ||
		!selected.VideoDraft.GamePreroll.Readiness.GenerationReady {
		t.Fatalf("selected game draft = %#v", selected.VideoDraft.GamePreroll)
	}
	providerInput, promptHash, err := service.GamePrerollProviderInput(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatalf("build game provider input: %v", err)
	}
	if providerInput.DurationSeconds != 6 || providerInput.AspectRatio != "9:16" ||
		providerInput.Prompt == "" || promptHash == "" || providerInput.InputMode != provider.VideoInputFirstLastFrame || len(providerInput.ConditioningAssets) != 2 {
		t.Fatalf("provider input = %#v, prompt hash = %q", providerInput, promptHash)
	}
	attempt, err := service.RegisterGamePrerollGenerationAttempt(
		context.Background(), rc.Actor, "project_1", task.ID, "provider_job_game_1",
	)
	if err != nil {
		t.Fatalf("register game generation attempt: %v", err)
	}
	restored, err := service.GetLatestGamePrerollWorkspace(context.Background(), rc.Actor, "project_1")
	if err != nil {
		t.Fatalf("restore game workspace: %v", err)
	}
	if len(restored.GamePrerollGenerationAttempts) != 1 ||
		restored.GamePrerollGenerationAttempts[0].ID != attempt.ID ||
		restored.GamePrerollGenerationAttempts[0].CandidateID != game.Candidates[1].ID ||
		restored.GamePrerollGenerationAttempts[0].PromptPackageHash != promptHash {
		t.Fatalf("restored game attempts = %#v", restored.GamePrerollGenerationAttempts)
	}
	regenerated, err := service.RegenerateGamePrerollCandidates(
		context.Background(),
		rc.Actor,
		"project_1",
		task.ID,
		RegenerateGamePrerollCandidatesRequest{
			ExpectedRevision: restored.VideoDraft.Revision,
			GenerationConfig: GamePrerollGenerationConfig{
				SubtitleStyle: "high_contrast_dynamic",
				HookStrength:  4,
				PaceProfile:   "punchy",
			},
		},
	)
	if err != nil {
		t.Fatalf("regenerate game candidates: %v", err)
	}
	if regenerated.VideoDraft.Revision != restored.VideoDraft.Revision+1 ||
		regenerated.VideoDraft.GamePreroll.SelectedCandidateID != "" ||
		regenerated.VideoDraft.GamePreroll.Readiness.GenerationReady ||
		len(regenerated.VideoDraft.GamePreroll.Candidates) != 3 {
		t.Fatalf("regenerated game draft = %#v", regenerated.VideoDraft.GamePreroll)
	}
}

type gameFrameExtractor struct{}

func (gameFrameExtractor) ExtractFrame(_ context.Context, request media.FrameExtractionRequest) (media.ExtractedFrame, error) {
	content := []byte("png")
	return media.ExtractedFrame{Content: io.NopCloser(bytes.NewReader(content)), SizeBytes: int64(len(content)), MIMEType: "image/png", Version: media.EvidenceFrameExtractorVersion}, nil
}

type gameDerivedWriter struct{ count int }

func (w *gameDerivedWriter) IngestDerivedImage(_ context.Context, _ contract.RequestContext, projectID contract.ProjectID, _ string, _ contract.AssetVersionRef, _ io.Reader, _ int64, _ string) (contract.ProjectAssetRef, error) {
	w.count++
	return contract.ProjectAssetRef{ProjectID: projectID, AssetVersion: contract.AssetVersionRef{AssetID: contract.AssetID(fmt.Sprintf("frame_%d", w.count)), Version: 1}}, nil
}

func testManualGamePrerollInput(sourceVideo contract.AssetVersionRef) ManualGamePrerollInput {
	return ManualGamePrerollInput{
		BriefID: "fixture_defend_sunflower_v1", BriefVersion: 1,
		BriefName: "《保卫向日葵》技能选择挑战", GameName: "保卫向日葵",
		GameplaySummary:   "竖屏塔防战斗中，玩家在波次间选择技能来调整后续战斗能力。",
		SourceVideo:       sourceVideo,
		SourceVideoRights: RightsConfirmed,
		EvidenceMoments: []GameEvidenceMoment{
			{ID: "skill_choice_1", Kind: GameEvidenceSkillChoice, StartMilliseconds: 20292, EndMilliseconds: 22250, Description: "第 1 次技能三选一。", VerifiedCopy: []string{"幻系易伤", "怪物易伤", "获得格子"}},
			{ID: "skill_choice_2", Kind: GameEvidenceSkillChoice, StartMilliseconds: 29792, EndMilliseconds: 31375, Description: "第 2 次技能三选一。", VerifiedCopy: []string{"激光弹射", "格子概率", "全体加攻"}},
			{ID: "wave_2", Kind: GameEvidenceWaveProgress, StartMilliseconds: 34000, EndMilliseconds: 35500, Description: "选择后进入第 2/10 波。", VerifiedCopy: []string{"第2/10波"}},
		},
		AllowedMechanisms:    []GameHookMechanism{GameHookChoiceChallenge, GameHookTacticalTradeoff, GameHookWaveEscalation},
		ProhibitedMechanisms: []GameHookMechanism{GameHookFailureReversal, GameHookMergeUpgrade, GameHookRewardReveal},
		SubtitleStyle:        "high_contrast_dynamic", HookStrength: 4, PaceProfile: "punchy",
	}
}

func testGamePrerollSnapshot() GamePrerollInputSnapshot {
	manual := testManualGamePrerollInput(contract.AssetVersionRef{AssetID: "asset_gameplay", Version: 1})
	return GamePrerollInputSnapshot{
		Source: IntakeSourceManual, SelectedRouteID: ManualGamePrerollRouteID,
		BriefID: manual.BriefID, BriefVersion: manual.BriefVersion, BriefName: manual.BriefName,
		GameName: manual.GameName, GameplaySummary: manual.GameplaySummary,
		SourceVideo: manual.SourceVideo, SourceVideoRights: manual.SourceVideoRights,
		CallToAction: "立即下载", EvidenceMoments: manual.EvidenceMoments,
		AllowedMechanisms: manual.AllowedMechanisms, ProhibitedMechanisms: manual.ProhibitedMechanisms,
	}
}
