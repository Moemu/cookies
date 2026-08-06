package creative

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/media"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type ShortDramaV2FirstFrameJobRequest struct {
	TaskID       string
	BatchID      string
	CandidateID  string
	VariantIndex int
	Prompt       string
	PromptHash   string
}

type ShortDramaV2ImageJobCreator interface {
	CreateFirstFrameJob(context.Context, contract.ActorContext, contract.ProjectContext, ShortDramaV2FirstFrameJobRequest) (contract.ProviderJob, error)
}

type PrepareShortDramaV2OpeningFrameRequest struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

type GenerateShortDramaV2FirstFramesRequest struct {
	ExpectedRevision int64 `json:"expected_revision"`
}

type ReconcileShortDramaV2FirstFrameRequest struct {
	ExpectedRevision int64                `json:"expected_revision"`
	CandidateID      string               `json:"candidate_id"`
	Job              contract.ProviderJob `json:"job"`
}

type SelectShortDramaV2FirstFrameRequest struct {
	ExpectedRevision int64  `json:"expected_revision"`
	BatchID          string `json:"batch_id"`
	CandidateID      string `json:"candidate_id"`
}

type BindShortDramaV2TrustedMaterialsRequest struct {
	ExpectedRevision  int64  `json:"expected_revision"`
	FirstFrameAssetID string `json:"first_frame_asset_id"`
	LastFrameAssetID  string `json:"last_frame_asset_id"`
}

func (s Service) PrepareShortDramaV2OpeningFrame(ctx context.Context, requestContext contract.RequestContext, projectID contract.ProjectID, taskID string, request PrepareShortDramaV2OpeningFrameRequest) (TaskDetail, error) {
	detail, err := s.requireShortDramaV2Workspace(ctx, requestContext.Actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	if s.GameEvidenceFrames == nil || s.DerivedAssets == nil {
		return TaskDetail{}, fmt.Errorf("short drama V2 frame extraction capability is unavailable")
	}
	workspace := detail.VideoDraft.ShortDramaPrerollV2
	extracted, err := s.GameEvidenceFrames.ExtractFrame(ctx, media.FrameExtractionRequest{
		OrganizationID: requestContext.Actor.OrganizationID, ProjectID: projectID,
		SourceVideo: workspace.SourceVideo.AssetVersion, TimestampMS: 0,
	})
	if err != nil {
		return TaskDetail{}, fmt.Errorf("extract short drama opening frame: %w", err)
	}
	derivationHash, err := contract.CanonicalJSONHash(struct {
		Source    contract.AssetVersionRef `json:"source"`
		Timestamp int64                    `json:"timestamp_ms"`
		Version   string                   `json:"extractor_version"`
	}{workspace.SourceVideo.AssetVersion, 0, extracted.Version})
	if err != nil {
		_ = extracted.Content.Close()
		return TaskDetail{}, err
	}
	derivationID := "short-drama-opening-frame-" + derivationHash
	asset, writeErr := s.DerivedAssets.IngestDerivedImage(ctx, requestContext, projectID, derivationID, workspace.SourceVideo.AssetVersion, extracted.Content, extracted.SizeBytes, extracted.MIMEType)
	closeErr := extracted.Content.Close()
	if writeErr != nil {
		return TaskDetail{}, writeErr
	}
	if closeErr != nil {
		return TaskDetail{}, closeErr
	}
	now := s.now()
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *workspace
	updated.Revision = next.Revision
	updated.SourceOpeningFrame = &ShortDramaV2SourceOpeningFrame{
		Status: ShortDramaV2ResourceReady, Asset: &asset, SourceVideo: workspace.SourceVideo,
		TimestampMS: 0, DerivationID: derivationID, ExtractionVersion: extracted.Version,
	}
	updated.GenerationSpec = nil
	updated.LatestVideoAttemptID = ""
	updated.OutputAsset = nil
	updated.UpdatedAt = now
	next.ShortDramaPrerollV2 = &updated
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, requestContext.Actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, TaskInProgress); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, requestContext.Actor.OrganizationID, projectID, taskID)
}

func (s Service) GenerateShortDramaV2FirstFrames(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request GenerateShortDramaV2FirstFramesRequest) (TaskDetail, error) {
	detail, err := s.requireShortDramaV2Workspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	workspace := detail.VideoDraft.ShortDramaPrerollV2
	if workspace.PromptDraft == nil || workspace.ActiveStage != ShortDramaV2StagePromptsReady {
		return TaskDetail{}, ErrInvalidState
	}
	if s.ShortDramaV2Images == nil {
		return TaskDetail{}, fmt.Errorf("short drama V2 image generation capability is unavailable")
	}
	project, err := s.Projects.RequireActiveContext(ctx, actor, projectID)
	if err != nil {
		return TaskDetail{}, err
	}
	batchID := fmt.Sprintf("%s_first_frame_batch_%d", taskID, detail.VideoDraft.Revision+1)
	candidates := make([]ShortDramaV2FirstFrameCandidate, 0, 3)
	for index, variation := range []string{"主体近景、正面凝视、压迫感构图", "中景侧身、环境纵深、动作即将发生", "低机位半身、强明暗对比、权力感构图"} {
		candidateID := fmt.Sprintf("%s_candidate_%d", batchID, index+1)
		job, err := s.ShortDramaV2Images.CreateFirstFrameJob(ctx, actor, project, ShortDramaV2FirstFrameJobRequest{
			TaskID: taskID, BatchID: batchID, CandidateID: candidateID, VariantIndex: index + 1,
			Prompt:     workspace.PromptDraft.ImagePrompt + "\n构图变化：" + variation,
			PromptHash: workspace.PromptDraft.ContentHash,
		})
		if err != nil {
			return TaskDetail{}, fmt.Errorf("create first frame job %d: %w", index+1, err)
		}
		candidates = append(candidates, ShortDramaV2FirstFrameCandidate{
			ID: candidateID, VariantIndex: index + 1, ProviderJobID: job.ID, Status: ShortDramaV2ResourceQueued,
		})
	}
	now := s.now()
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *workspace
	updated.Revision = next.Revision
	updated.FirstFrameBatch = &ShortDramaV2FirstFrameBatch{
		ShortDramaV2AsyncResource: ShortDramaV2AsyncResource{Status: ShortDramaV2ResourceQueued},
		ID:                        batchID, Revision: next.Revision, PromptRevision: workspace.PromptDraft.Revision, Candidates: candidates,
	}
	updated.TrustedMaterials = nil
	updated.GenerationSpec = nil
	updated.LatestVideoAttemptID = ""
	updated.OutputAsset = nil
	updated.UpdatedAt = now
	next.ShortDramaPrerollV2 = &updated
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, TaskInProgress); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) ReconcileShortDramaV2FirstFrame(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request ReconcileShortDramaV2FirstFrameRequest) (TaskDetail, error) {
	detail, err := s.requireShortDramaV2Workspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	workspace := detail.VideoDraft.ShortDramaPrerollV2
	if workspace.FirstFrameBatch == nil || request.Job.ProjectID != projectID {
		return TaskDetail{}, ErrInvalidState
	}
	batch := *workspace.FirstFrameBatch
	index := -1
	for i := range batch.Candidates {
		if batch.Candidates[i].ID == request.CandidateID && batch.Candidates[i].ProviderJobID == request.Job.ID {
			index = i
			break
		}
	}
	if index < 0 {
		return TaskDetail{}, fmt.Errorf("short drama V2 first frame job does not match the active batch")
	}
	candidate := batch.Candidates[index]
	switch request.Job.ProviderStatus {
	case contract.ProviderJobSucceeded, contract.ProviderJobPartiallySucceeded:
		if len(request.Job.ProjectAssetRefs) == 0 || request.Job.ProjectAssetRefs[0].ProjectID != projectID {
			return TaskDetail{}, fmt.Errorf("short drama V2 image job completed without a durable project asset")
		}
		candidate.Status = ShortDramaV2ResourceReady
		asset := request.Job.ProjectAssetRefs[0]
		candidate.Asset = &asset
		candidate.ErrorCode, candidate.ErrorMessage = "", ""
	case contract.ProviderJobFailed, contract.ProviderJobCancelled, contract.ProviderJobExpired:
		candidate.Status = ShortDramaV2ResourceFailed
		candidate.ErrorCode = "IMAGE_GENERATION_FAILED"
		if request.Job.Error != nil {
			candidate.ErrorMessage = request.Job.Error.Message
		}
	default:
		return detail, nil
	}
	batch.Candidates[index] = candidate
	ready, failed := 0, 0
	for _, item := range batch.Candidates {
		switch item.Status {
		case ShortDramaV2ResourceReady:
			ready++
		case ShortDramaV2ResourceFailed, ShortDramaV2ResourceCancelled:
			failed++
		}
	}
	switch {
	case ready == len(batch.Candidates):
		batch.Status = ShortDramaV2ResourceReady
	case ready > 0 && ready+failed == len(batch.Candidates):
		batch.Status = ShortDramaV2ResourcePartial
	case failed == len(batch.Candidates):
		batch.Status = ShortDramaV2ResourceFailed
	default:
		batch.Status = ShortDramaV2ResourceRunning
	}
	now := s.now()
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *workspace
	updated.Revision = next.Revision
	updated.FirstFrameBatch = &batch
	if ready > 0 && ready+failed == len(batch.Candidates) {
		updated.ActiveStage = ShortDramaV2StageFramesReady
	}
	updated.UpdatedAt = now
	next.ShortDramaPrerollV2 = &updated
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, TaskInProgress); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) SelectShortDramaV2FirstFrame(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request SelectShortDramaV2FirstFrameRequest) (TaskDetail, error) {
	detail, err := s.requireShortDramaV2Workspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	workspace := detail.VideoDraft.ShortDramaPrerollV2
	batch := workspace.FirstFrameBatch
	if batch == nil || batch.ID != request.BatchID || workspace.PromptDraft == nil ||
		workspace.SourceOpeningFrame == nil || workspace.SourceOpeningFrame.Asset == nil {
		return TaskDetail{}, ErrInvalidState
	}
	var selected *contract.ProjectAssetRef
	for _, candidate := range batch.Candidates {
		if candidate.ID == request.CandidateID && candidate.Status == ShortDramaV2ResourceReady && candidate.Asset != nil {
			asset := *candidate.Asset
			selected = &asset
			break
		}
	}
	if selected == nil {
		return TaskDetail{}, fmt.Errorf("short drama V2 first frame does not belong to the active ready batch")
	}
	now := s.now()
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *workspace
	updated.Revision = next.Revision
	batchCopy := *batch
	batchCopy.SelectedAsset = selected
	updated.FirstFrameBatch = &batchCopy
	updated.TrustedMaterials = nil
	updated.ActiveStage = ShortDramaV2StageFrameSelected
	spec, err := compileShortDramaV2GenerationSpec(updated, projectID, next.Revision)
	if err != nil {
		return TaskDetail{}, err
	}
	updated.GenerationSpec = spec
	updated.LatestVideoAttemptID = ""
	updated.OutputAsset = nil
	updated.UpdatedAt = now
	next.ShortDramaPrerollV2 = &updated
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, TaskReady); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) BindShortDramaV2TrustedMaterials(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request BindShortDramaV2TrustedMaterialsRequest) (TaskDetail, error) {
	detail, err := s.requireShortDramaV2Workspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	workspace := detail.VideoDraft.ShortDramaPrerollV2
	if workspace.PromptDraft == nil || workspace.FirstFrameBatch == nil || workspace.FirstFrameBatch.SelectedAsset == nil ||
		workspace.SourceOpeningFrame == nil || workspace.SourceOpeningFrame.Asset == nil {
		return TaskDetail{}, ErrInvalidState
	}
	binding := ShortDramaV2TrustedMaterialBinding{
		ProviderCode: "ark-video", FirstFrameAssetID: strings.TrimSpace(request.FirstFrameAssetID), LastFrameAssetID: strings.TrimSpace(request.LastFrameAssetID),
	}
	for _, assetID := range []string{binding.FirstFrameAssetID, binding.LastFrameAssetID} {
		if err := (provider.VideoAuthorizedAssetReference{ProviderCode: binding.ProviderCode, AssetID: assetID}).Validate(); err != nil {
			return TaskDetail{}, err
		}
	}
	now := s.now()
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *workspace
	updated.Revision = next.Revision
	updated.ActiveStage = ShortDramaV2StageFrameSelected
	updated.TrustedMaterials = &binding
	updated.LatestVideoAttemptID = ""
	updated.OutputAsset = nil
	updated.UpdatedAt = now
	spec, err := compileShortDramaV2GenerationSpec(updated, projectID, next.Revision)
	if err != nil {
		return TaskDetail{}, err
	}
	updated.GenerationSpec = spec
	next.ShortDramaPrerollV2 = &updated
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, TaskReady); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func compileShortDramaV2GenerationSpec(workspace ShortDramaPrerollV2Workspace, projectID contract.ProjectID, revision int64) (*ShortDramaV2GenerationSpec, error) {
	if workspace.PromptDraft == nil || workspace.FirstFrameBatch == nil || workspace.FirstFrameBatch.SelectedAsset == nil ||
		workspace.SourceOpeningFrame == nil || workspace.SourceOpeningFrame.Asset == nil {
		return nil, ErrInvalidState
	}
	first, last := *workspace.FirstFrameBatch.SelectedAsset, *workspace.SourceOpeningFrame.Asset
	if first.ProjectID != projectID || last.ProjectID != projectID {
		return nil, ErrInvalidState
	}
	spec := ShortDramaV2GenerationSpec{
		ContractVersion: "creative-short-drama-preroll-generation-spec/v2", DraftRevision: revision,
		PromptRevision: workspace.PromptDraft.Revision, DurationSeconds: workspace.PromptDraft.DurationSeconds,
		AspectRatio: "9:16", Resolution: "720p", AudioPolicy: string(provider.VideoAudioGenerated),
		InputMode: string(provider.VideoInputFirstLastFrame), FirstFrameAsset: first, LastFrameAsset: last,
		TrustedMaterials: workspace.TrustedMaterials,
		PromptHash:       workspace.PromptDraft.ContentHash,
	}
	hash, err := contract.CanonicalJSONHash(spec)
	if err != nil {
		return nil, err
	}
	spec.SpecHash = "sha256:" + hash
	return &spec, nil
}

func (s Service) ShortDramaV2ProviderInput(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string) (provider.VideoGenerationInput, string, string, error) {
	detail, err := s.requireShortDramaV2Workspace(ctx, actor, projectID, taskID, false)
	if err != nil {
		return provider.VideoGenerationInput{}, "", "", err
	}
	workspace := detail.VideoDraft.ShortDramaPrerollV2
	if workspace.GenerationSpec == nil || workspace.PromptDraft == nil || workspace.ActiveStage != ShortDramaV2StageFrameSelected {
		return provider.VideoGenerationInput{}, "", "", ErrInvalidState
	}
	spec := workspace.GenerationSpec
	if spec.PromptHash != workspace.PromptDraft.ContentHash || strings.TrimSpace(spec.SpecHash) == "" {
		return provider.VideoGenerationInput{}, "", "", ErrInvalidState
	}
	input := provider.VideoGenerationInput{
		Prompt: workspace.PromptDraft.VideoPrompt, DurationSeconds: spec.DurationSeconds,
		AspectRatio: spec.AspectRatio, Resolution: spec.Resolution,
		AudioPolicy: provider.VideoAudioPolicy(spec.AudioPolicy), InputMode: provider.VideoInputMode(spec.InputMode),
		ConditioningAssets: []provider.VideoConditioningAsset{
			{Role: provider.VideoConditioningFirstFrame, Reference: spec.FirstFrameAsset},
			{Role: provider.VideoConditioningLastFrame, Reference: spec.LastFrameAsset},
		},
	}
	if spec.TrustedMaterials != nil {
		input.ConditioningAssets[0].AuthorizedAsset = &provider.VideoAuthorizedAssetReference{ProviderCode: spec.TrustedMaterials.ProviderCode, AssetID: spec.TrustedMaterials.FirstFrameAssetID}
		input.ConditioningAssets[1].AuthorizedAsset = &provider.VideoAuthorizedAssetReference{ProviderCode: spec.TrustedMaterials.ProviderCode, AssetID: spec.TrustedMaterials.LastFrameAssetID}
	}
	if err := input.Validate(); err != nil {
		return provider.VideoGenerationInput{}, "", "", err
	}
	return input, spec.PromptHash, spec.SpecHash, nil
}

func (s Service) RegisterShortDramaV2VideoJob(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID, providerJobID string) (TaskDetail, error) {
	detail, err := s.requireShortDramaV2Workspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if strings.TrimSpace(providerJobID) == "" || detail.VideoDraft.ShortDramaPrerollV2.GenerationSpec == nil {
		return TaskDetail{}, ErrInvalidState
	}
	now := s.now()
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *detail.VideoDraft.ShortDramaPrerollV2
	updated.Revision, updated.ActiveStage, updated.LatestVideoAttemptID = next.Revision, ShortDramaV2StageVideoGenerating, providerJobID
	updated.OutputAsset, updated.UpdatedAt = nil, now
	next.ShortDramaPrerollV2 = &updated
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, TaskGenerating); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

type ReconcileShortDramaV2VideoRequest struct {
	ExpectedRevision int64                `json:"expected_revision"`
	Job              contract.ProviderJob `json:"job"`
}

func (s Service) ReconcileShortDramaV2Video(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request ReconcileShortDramaV2VideoRequest) (TaskDetail, error) {
	detail, err := s.requireShortDramaV2Workspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	workspace := detail.VideoDraft.ShortDramaPrerollV2
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	if request.Job.ID != workspace.LatestVideoAttemptID || request.Job.ProjectID != projectID {
		return TaskDetail{}, ErrInvalidState
	}
	if request.Job.ProviderStatus != contract.ProviderJobSucceeded && request.Job.ProviderStatus != contract.ProviderJobPartiallySucceeded {
		return detail, nil
	}
	if len(request.Job.ProjectAssetRefs) == 0 || request.Job.ProjectAssetRefs[0].ProjectID != projectID {
		return TaskDetail{}, fmt.Errorf("short drama V2 video completed without a durable project asset")
	}
	asset := request.Job.ProjectAssetRefs[0]
	now := s.now()
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *workspace
	updated.Revision, updated.ActiveStage, updated.OutputAsset, updated.UpdatedAt = next.Revision, ShortDramaV2StageCompleted, &asset, now
	next.ShortDramaPrerollV2 = &updated
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, TaskGenerated); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}
