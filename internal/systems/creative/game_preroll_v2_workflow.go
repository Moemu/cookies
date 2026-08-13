package creative

import (
	"context"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type GamePrerollV2AnalysisResult struct {
	InputHash       string               `json:"input_hash"`
	PromptVersion   string               `json:"prompt_version"`
	GameName        string               `json:"game_name"`
	GameplaySummary string               `json:"gameplay_summary"`
	Facts           []GameAnalysisFact   `json:"facts"`
	Evidence        []GameEvidenceMoment `json:"evidence"`
	Unknowns        []string             `json:"unknowns"`
	SuggestedBrief  []GameBriefField     `json:"suggested_brief"`
}
type GamePrerollV2Analyzer interface {
	Analyze(context.Context, contract.ActorContext, contract.ProjectContext, contract.ProjectAssetRef) (GamePrerollV2AnalysisResult, error)
}
type GamePrerollV2AnalysisLauncher interface {
	Launch(func())
}
type AnalyzeGamePrerollV2SourceRequest struct {
	ExpectedRevision int64 `json:"expected_revision"`
}
type ConfirmGamePrerollV2BriefRequest struct {
	ExpectedRevision int64            `json:"expected_revision"`
	Fields           []GameBriefField `json:"fields"`
}
type PlanGamePrerollV2CandidatesRequest struct {
	ExpectedRevision int64 `json:"expected_revision"`
}
type UpdateGamePrerollV2GenerationConfigRequest struct {
	ExpectedRevision int64                       `json:"expected_revision"`
	Config           GamePrerollGenerationConfig `json:"config"`
}
type ReconcileGamePrerollV2VideoRequest struct {
	ExpectedRevision int64                `json:"expected_revision"`
	Job              contract.ProviderJob `json:"job"`
}

type CreateGamePrerollV2WorkspaceRequest struct {
	SourceVideo     contract.AssetVersionRef `json:"source_video"`
	RightsConfirmed bool                     `json:"rights_confirmed"`
	DurationSeconds int                      `json:"duration_seconds,omitempty"`
}

func (s Service) RegisterGamePrerollV2VideoJob(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, expected int64, providerJobID string) (TaskDetail, error) {
	if providerJobID == "" {
		return TaskDetail{}, fmt.Errorf("provider_job_id is required")
	}
	detail, err := s.GetGamePrerollV2Workspace(ctx, actor, projectID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}
	if detail.VideoDraft.Revision != expected {
		return TaskDetail{}, ErrVersionConflict
	}
	if _, err = s.RegisterGamePrerollGenerationAttempt(ctx, actor, projectID, taskID, providerJobID); err != nil {
		return TaskDetail{}, err
	}
	return s.reviseGamePrerollV2(ctx, actor, projectID, taskID, expected, func(_ *VideoDraft, w *GamePrerollDraft) error {
		if w.GenerationSpec == nil || !w.Readiness.GenerationReady {
			return ErrInvalidState
		}
		w.LatestVideoAttemptID = providerJobID
		w.Stage = GamePrerollStageVideoGenerating
		w.VideoError = nil
		w.OutputAsset = nil
		return nil
	}, TaskGenerating)
}

func (s Service) ReconcileGamePrerollV2Video(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request ReconcileGamePrerollV2VideoRequest) (TaskDetail, error) {
	detail, err := s.GetGamePrerollV2Workspace(ctx, actor, projectID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}
	if detail.VideoDraft.Revision != request.ExpectedRevision {
		return TaskDetail{}, ErrVersionConflict
	}
	w := detail.VideoDraft.GamePreroll
	if request.Job.ID != w.LatestVideoAttemptID || request.Job.ProjectID != projectID {
		return TaskDetail{}, ErrInvalidState
	}
	if request.Job.ProviderStatus == contract.ProviderJobFailed || request.Job.ProviderStatus == contract.ProviderJobCancelled || request.Job.ProviderStatus == contract.ProviderJobExpired {
		return s.reviseGamePrerollV2(ctx, actor, projectID, taskID, request.ExpectedRevision, func(_ *VideoDraft, current *GamePrerollDraft) error {
			current.Stage = GamePrerollStageCandidateSelected
			current.VideoError = request.Job.Error
			if current.VideoError == nil {
				current.VideoError = &contract.JobError{Code: "VIDEO_GENERATION_FAILED", Message: "游戏前贴生成失败", Retryable: true}
			}
			return nil
		}, TaskReady)
	}
	if request.Job.ProviderStatus != contract.ProviderJobSucceeded && request.Job.ProviderStatus != contract.ProviderJobPartiallySucceeded {
		return detail, nil
	}
	if len(request.Job.ProjectAssetRefs) == 0 || request.Job.ProjectAssetRefs[0].ProjectID != projectID {
		return TaskDetail{}, fmt.Errorf("game preroll video completed without a durable project asset")
	}
	output := request.Job.ProjectAssetRefs[0]
	updated, err := s.reviseGamePrerollV2(ctx, actor, projectID, taskID, request.ExpectedRevision, func(_ *VideoDraft, current *GamePrerollDraft) error {
		current.Stage = GamePrerollStageVideoReady
		current.OutputAsset = &output
		current.VideoError = nil
		current.Readiness.ProductionReady = true
		return nil
	}, TaskGenerated)
	if err != nil {
		return TaskDetail{}, err
	}
	if attempts, ok := s.Repository.(GamePrerollAttemptRepository); ok {
		if err = attempts.CompleteGamePrerollGenerationAttempt(ctx, actor.OrganizationID, projectID, request.Job.ID, output.AssetVersion); err != nil {
			return TaskDetail{}, err
		}
	}
	return updated, nil
}

func (s Service) CreateGamePrerollV2Workspace(ctx context.Context, rc contract.RequestContext, projectID contract.ProjectID, key contract.IdempotencyKey, request CreateGamePrerollV2WorkspaceRequest) (TaskDetail, error) {
	if request.SourceVideo.Validate() != nil || !request.RightsConfirmed {
		return TaskDetail{}, fmt.Errorf("a valid source_video and explicit rights confirmation are required")
	}
	if request.DurationSeconds == 0 {
		request.DurationSeconds = 8
	}
	if request.DurationSeconds < 6 || request.DurationSeconds > 10 {
		return TaskDetail{}, fmt.Errorf("duration_seconds must be between 6 and 10")
	}
	intake, err := s.CreateIntake(ctx, rc, projectID, key, CreateIntakeRequest{
		Source: IntakeSourceManual, Format: FormatVideo, PerformanceMode: PerformanceModeGamePreroll, Channel: ChannelDouyin,
		Objective: "促进游戏下载", Audience: "抖音游戏用户", CoreMessage: "从真实游戏视频事实生成前贴", CallToAction: "立即下载", Concept: "原视频理解驱动的游戏前贴", Tone: []string{"高注意力"}, VisualKeywords: []string{"真实玩法", "游戏UI"}, Mandatory: []string{}, Prohibited: []string{"不得虚构奖励、数值、玩法或结果"},
		CreativeRoutes:      []CreativeRouteSnapshot{{RouteID: ManualGamePrerollV2RouteID, RouteType: PerformanceModeGamePreroll, VideoPurpose: "performance", Channels: []string{string(ChannelDouyin)}, Reason: "source-video-driven game preroll", TargetDurationSeconds: request.DurationSeconds, AspectRatio: "9:16", Resolution: "720p", SourceAssetRefs: []contract.AssetVersionRef{request.SourceVideo}, RequiresHumanConfirmation: true}},
		ManualGamePrerollV2: &ManualGamePrerollV2Input{SourceVideo: request.SourceVideo, SourceVideoRights: RightsConfirmed},
	})
	if err != nil {
		return TaskDetail{}, err
	}
	if existing, existingErr := s.taskForIntake(ctx, rc.Actor, projectID, intake.ID); existingErr == nil {
		return existing, nil
	} else if existingErr != ErrNotFound {
		return TaskDetail{}, existingErr
	}
	task, err := s.CreateVideoTask(ctx, rc.Actor, projectID, intake.ID, CreateVideoTaskRequest{SelectedRouteID: ManualGamePrerollV2RouteID, Channel: ChannelDouyin, SourceVideo: request.SourceVideo, Concept: "原视频理解驱动的游戏前贴", Prompt: "等待游戏原视频理解", CallToAction: "立即下载", Mandatory: []string{}, Prohibited: []string{"不得虚构奖励、数值、玩法或结果"}, ConfirmRoute: true})
	if err != nil {
		return TaskDetail{}, err
	}
	return s.GetGamePrerollV2Workspace(ctx, rc.Actor, projectID, task.ID)
}

func (s Service) GetGamePrerollV2Workspace(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string) (TaskDetail, error) {
	detail, err := s.requireGamePrerollWorkspace(ctx, actor, projectID, taskID, false)
	if err != nil {
		return TaskDetail{}, err
	}
	if detail.VideoDraft.GamePreroll.ContractVersion != GamePrerollV2ContractVersion {
		return TaskDetail{}, ErrNotFound
	}
	return detail, nil
}

func (s Service) reviseGamePrerollV2(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, expected int64, mutate func(*VideoDraft, *GamePrerollDraft) error, status TaskStatus) (TaskDetail, error) {
	detail, err := s.GetGamePrerollV2Workspace(ctx, actor, projectID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}
	if detail.VideoDraft.Revision != expected {
		return TaskDetail{}, ErrVersionConflict
	}
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = s.now()
	workspace := *detail.VideoDraft.GamePreroll
	workspace.Revision = next.Revision
	workspace.UpdatedAt = s.now()
	if err = mutate(&next, &workspace); err != nil {
		return TaskDetail{}, err
	}
	next.GamePreroll = &workspace
	if _, err = s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, expected, next, status); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) AnalyzeGamePrerollV2Source(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request AnalyzeGamePrerollV2SourceRequest) (TaskDetail, error) {
	if s.GamePrerollV2Analyzer == nil {
		return TaskDetail{}, fmt.Errorf("game preroll analysis capability is unavailable")
	}
	project, err := s.Projects.RequireActiveContext(ctx, actor, projectID)
	if err != nil {
		return TaskDetail{}, err
	}
	detail, err := s.GetGamePrerollV2Workspace(ctx, actor, projectID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}
	if detail.VideoDraft.Revision != request.ExpectedRevision {
		return TaskDetail{}, ErrVersionConflict
	}
	running, err := s.reviseGamePrerollV2(ctx, actor, projectID, taskID, request.ExpectedRevision, func(_ *VideoDraft, w *GamePrerollDraft) error {
		w.Analysis = GamePrerollAnalysis{Status: GamePrerollResourceRunning, Revision: w.Analysis.Revision}
		w.Readiness.PlanningReady = false
		w.Readiness.GenerationReady = false
		w.Readiness.ProductionReady = false
		return nil
	}, TaskInProgress)
	if err != nil {
		return TaskDetail{}, err
	}
	analysisContext := context.WithoutCancel(ctx)
	source := contract.ProjectAssetRef{ProjectID: projectID, AssetVersion: detail.VideoDraft.GamePreroll.InputSnapshot.SourceVideo}
	durationMS := detail.VideoDraft.GamePreroll.SourceMetadata.DurationMS
	work := func() {
		s.finishGamePrerollV2Analysis(analysisContext, actor, project, projectID, taskID, running.VideoDraft.Revision, source, durationMS)
	}
	if s.GamePrerollV2AnalysisLauncher != nil {
		s.GamePrerollV2AnalysisLauncher.Launch(work)
	} else {
		go work()
	}
	return running, nil
}

func (s Service) finishGamePrerollV2Analysis(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, projectID contract.ProjectID, taskID string, expectedRevision int64, source contract.ProjectAssetRef, durationMS int64) {
	result, err := s.GamePrerollV2Analyzer.Analyze(ctx, actor, project, source)
	if err != nil {
		_, _ = s.reviseGamePrerollV2(ctx, actor, projectID, taskID, expectedRevision, func(_ *VideoDraft, w *GamePrerollDraft) error {
			w.Analysis.Status = GamePrerollResourceFailed
			w.Analysis.ErrorCode = "GAME_SOURCE_ANALYSIS_FAILED"
			w.Analysis.ErrorMessage = "游戏原视频分析失败，可重试"
			w.Stage = GamePrerollStageSourceReady
			return nil
		}, TaskInProgress)
		return
	}
	if err = validateGameAnalysisResult(result, durationMS); err != nil {
		_, _ = s.reviseGamePrerollV2(ctx, actor, projectID, taskID, expectedRevision, func(_ *VideoDraft, w *GamePrerollDraft) error {
			w.Analysis.Status = GamePrerollResourceFailed
			w.Analysis.ErrorCode = "GAME_SOURCE_ANALYSIS_INVALID"
			w.Analysis.ErrorMessage = "游戏原视频分析结果不完整，可重试"
			w.Stage = GamePrerollStageSourceReady
			return nil
		}, TaskInProgress)
		return
	}
	_, _ = s.reviseGamePrerollV2(ctx, actor, projectID, taskID, expectedRevision, func(d *VideoDraft, w *GamePrerollDraft) error {
		w.Analysis = GamePrerollAnalysis{Status: GamePrerollResourceReady, Revision: w.Analysis.Revision + 1, InputHash: result.InputHash, PromptVersion: result.PromptVersion, GameName: result.GameName, GameplaySummary: result.GameplaySummary, Facts: result.Facts, Evidence: result.Evidence, Unknowns: result.Unknowns, SuggestedBrief: result.SuggestedBrief}
		w.Stage = GamePrerollStageAnalysisReady
		w.InputSnapshot.GameName = result.GameName
		w.InputSnapshot.GameplaySummary = result.GameplaySummary
		w.InputSnapshot.EvidenceMoments = append([]GameEvidenceMoment{}, result.Evidence...)
		w.ConfirmedBrief = nil
		w.ActiveCandidateBatch = nil
		w.Candidates = nil
		w.SelectedCandidateID = ""
		w.EvidenceAssets = nil
		w.GenerationSpec = nil
		w.OutputAsset = nil
		w.LatestVideoAttemptID = ""
		w.VideoError = nil
		w.Readiness = CreativeReadiness{PlanningReady: false, GenerationReady: false, ProductionReady: false, Blockers: []string{"confirmed_brief", "selected_candidate", "evidence_assets"}}
		d.Prompt = "游戏原视频理解已完成，等待确认 Brief"
		return nil
	}, TaskInProgress)
}

func validateGameAnalysisResult(r GamePrerollV2AnalysisResult, duration int64) error {
	if r.InputHash == "" || r.PromptVersion == "" || r.GameName == "" || len([]rune(r.GameplaySummary)) < 10 || len(r.Evidence) < 2 || len(r.SuggestedBrief) < 4 {
		return fmt.Errorf("game preroll analysis result is incomplete")
	}
	for _, e := range r.Evidence {
		if err := e.Validate(); err != nil {
			return err
		}
		if duration > 0 && int64(e.EndMilliseconds) > duration {
			return fmt.Errorf("game evidence exceeds source duration")
		}
	}
	return nil
}

func (s Service) ConfirmGamePrerollV2Brief(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request ConfirmGamePrerollV2BriefRequest) (TaskDetail, error) {
	if len(request.Fields) < 4 {
		return TaskDetail{}, fmt.Errorf("game brief requires objective, audience, selling point and CTA")
	}
	for _, f := range request.Fields {
		if f.Required && f.Value == "" {
			return TaskDetail{}, fmt.Errorf("required game brief field is empty")
		}
	}
	return s.reviseGamePrerollV2(ctx, actor, projectID, taskID, request.ExpectedRevision, func(d *VideoDraft, w *GamePrerollDraft) error {
		if w.Analysis.Status != GamePrerollResourceReady {
			return ErrInvalidState
		}
		hash, _ := contract.CanonicalJSONHash(request.Fields)
		version := int64(1)
		if w.ConfirmedBrief != nil {
			version = w.ConfirmedBrief.Version + 1
		}
		w.ConfirmedBrief = &GameBriefVersion{ID: fmt.Sprintf("%s_brief_%d", taskID, version), Version: version, AnalysisRevision: w.Analysis.Revision, Fields: append([]GameBriefField{}, request.Fields...), ConfirmedBy: actor.Principal.ID, ConfirmedAt: s.now(), ContentHash: "sha256:" + hash}
		w.Stage = GamePrerollStageBriefConfirmed
		w.ActiveCandidateBatch = nil
		w.Candidates = nil
		w.SelectedCandidateID = ""
		w.GenerationSpec = nil
		w.Readiness.PlanningReady = true
		w.Readiness.Blockers = []string{"selected_candidate", "evidence_assets"}
		for _, f := range request.Fields {
			if f.Key == "cta" {
				w.InputSnapshot.CallToAction = f.Value
				w.GenerationConfig.CallToAction = f.Value
			}
		}
		d.Prompt = "Brief 已确认，等待规划候选"
		return nil
	}, TaskInProgress)
}

func (s Service) PlanGamePrerollV2Candidates(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request PlanGamePrerollV2CandidatesRequest) (TaskDetail, error) {
	detail, err := s.GetGamePrerollV2Workspace(ctx, actor, projectID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}
	if detail.VideoDraft.Revision != request.ExpectedRevision {
		return TaskDetail{}, ErrVersionConflict
	}
	w := detail.VideoDraft.GamePreroll
	if w.ConfirmedBrief == nil {
		return TaskDetail{}, ErrInvalidState
	}
	snapshot := w.InputSnapshot
	snapshot.AllowedMechanisms = []GameHookMechanism{GameHookChoiceChallenge, GameHookTacticalTradeoff, GameHookWaveEscalation}
	snapshot.ProhibitedMechanisms = []GameHookMechanism{GameHookFailureReversal, GameHookMergeUpgrade, GameHookRewardReveal}
	project, err := s.Projects.RequireActiveContext(ctx, actor, projectID)
	if err != nil {
		return TaskDetail{}, err
	}
	planner := s.GamePrerollPlanner
	if planner == nil {
		planner = FallbackGamePrerollPlanner{Fallback: GenericGamePrerollPlanner{}}
	}
	batch, err := planner.Plan(ctx, actor, project, snapshot, w.InputHash, taskID+"_batch_"+fmt.Sprint(request.ExpectedRevision+1), request.ExpectedRevision+1, w.GenerationConfig, s.now())
	if err != nil {
		return TaskDetail{}, err
	}
	return s.reviseGamePrerollV2(ctx, actor, projectID, taskID, request.ExpectedRevision, func(d *VideoDraft, current *GamePrerollDraft) error {
		current.InputSnapshot = snapshot
		current.ActiveCandidateBatch = &batch
		current.Candidates = batch.Candidates
		current.Stage = GamePrerollStageCandidatesReady
		current.SelectedCandidateID = ""
		current.GenerationSpec = nil
		d.Prompt = batch.Candidates[0].PromptPackage.CompiledPrompt
		return nil
	}, TaskInProgress)
}

func (s Service) UpdateGamePrerollV2GenerationConfig(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request UpdateGamePrerollV2GenerationConfigRequest) (TaskDetail, error) {
	if err := request.Config.Validate(); err != nil {
		return TaskDetail{}, err
	}
	if request.Config.DurationSeconds == 0 || request.Config.Channel == "" || request.Config.AspectRatio == "" || request.Config.Resolution == "" || request.Config.AudioPolicy == "" || request.Config.CallToAction == "" {
		return TaskDetail{}, fmt.Errorf("complete game preroll generation config is required")
	}
	return s.reviseGamePrerollV2(ctx, actor, projectID, taskID, request.ExpectedRevision, func(_ *VideoDraft, w *GamePrerollDraft) error {
		w.GenerationConfig = request.Config
		w.InputSnapshot.CallToAction = request.Config.CallToAction
		w.ActiveCandidateBatch = nil
		w.Candidates = nil
		w.SelectedCandidateID = ""
		w.EvidenceAssets = nil
		w.GenerationSpec = nil
		w.OutputAsset = nil
		w.LatestVideoAttemptID = ""
		w.VideoError = nil
		w.Readiness.GenerationReady = false
		w.Readiness.ProductionReady = false
		w.Readiness.Blockers = appendUnique(removeStrings(w.Readiness.Blockers, "selected_candidate", "evidence_assets"), "candidate_batch")
		if w.ConfirmedBrief != nil {
			w.Stage = GamePrerollStageBriefConfirmed
		}
		return nil
	}, TaskInProgress)
}

type GenericGamePrerollPlanner struct{}

func (GenericGamePrerollPlanner) Plan(_ context.Context, _ contract.ActorContext, _ contract.ProjectContext, snapshot GamePrerollInputSnapshot, inputHash, batchID string, revision int64, config GamePrerollGenerationConfig, now time.Time) (GameCandidateBatch, error) {
	return planGenericGameCandidateBatch(snapshot, inputHash, batchID, revision, config, now)
}
