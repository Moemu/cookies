package mediaunderstanding

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
	"github.com/shikanon/cookies/internal/platform/media"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const JobKind = "platform.media.understand.v1"

type jobStore interface {
	Enqueue(context.Context, jobruntime.CreateRequest) (contract.Job, bool, error)
}

type JobRuntimeScheduler struct {
	Store jobStore
	NewID func() (string, error)
	Now   func() time.Time
}

func (s JobRuntimeScheduler) Schedule(ctx context.Context, artifact Artifact) (string, error) {
	if s.Store == nil || s.NewID == nil {
		return "", fmt.Errorf("media understanding job scheduler is unavailable")
	}
	jobID, err := s.NewID()
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(struct {
		ArtifactID string `json:"artifact_id"`
	}{ArtifactID: artifact.ID})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	nowFn := s.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn().UTC()
	job, _, err := s.Store.Enqueue(ctx, jobruntime.CreateRequest{
		Job: contract.Job{
			ID: jobID, Kind: JobKind, OrganizationID: artifact.OrganizationID, ProjectID: artifact.ProjectID,
			Status: contract.JobQueued, Cancellable: true, AttemptCount: 0, MaxAttempts: 1,
			Version: 1, CreatedAt: now, UpdatedAt: now,
		},
		Payload: payload, IdempotencyKey: contract.IdempotencyKey("media_understand_" + artifact.ID),
		RequestHash: hex.EncodeToString(sum[:]),
	})
	if err != nil {
		return "", err
	}
	return job.ID, nil
}

func (s Service) HandleJob(ctx context.Context, claim jobruntime.Claim) (jobruntime.Result, error) {
	if claim.Job.Kind != JobKind {
		return jobruntime.Result{}, fmt.Errorf("unsupported media understanding job kind %q", claim.Job.Kind)
	}
	var payload struct {
		ArtifactID string `json:"artifact_id"`
	}
	if json.Unmarshal(claim.Payload, &payload) != nil || strings.TrimSpace(payload.ArtifactID) == "" {
		return jobruntime.Result{}, jobruntime.ExecutionError{JobError: contract.JobError{
			Code: "INVALID_MEDIA_UNDERSTANDING_JOB", Message: "Media understanding job payload is invalid", Retryable: false,
		}}
	}
	artifact, err := s.Store.Get(ctx, claim.Job.OrganizationID, claim.Job.ProjectID, payload.ArtifactID)
	if err != nil {
		return jobruntime.Result{}, err
	}
	ref := contract.ResourceRef{Type: "media_understanding_artifact", ID: artifact.ID}
	if artifact.Status == StatusReady || artifact.Status == StatusPartial || artifact.Status == StatusFailed {
		return jobruntime.Result{Ref: &ref}, nil
	}
	actor := contract.ActorContext{
		OrganizationID: artifact.OrganizationID, Principal: artifact.CreatedBy,
		Scopes: []contract.Scope{"project.read", "assets.read", "assets.write", provider.ScopeVisionUnderstand},
	}
	asset, err := s.Assets.Get(ctx, actor, artifact.ProjectID, artifact.AssetRef.AssetVersion)
	if err == nil {
		switch artifact.AssetKind {
		case contract.AssetImage:
			err = s.analyzeImage(ctx, actor, asset, &artifact)
		case contract.AssetVideo:
			err = s.analyzeVideo(ctx, actor, asset, &artifact)
		default:
			err = ErrUnsupportedProfile
		}
	}
	if err != nil {
		artifact.Status = StatusFailed
		artifact.ErrorCode = mediaFailureCode(err)
		artifact.ErrorMessage = mediaFailureMessage(err)
	}
	artifact, persistErr := s.Store.Complete(ctx, artifact, s.now())
	if persistErr != nil {
		return jobruntime.Result{}, persistErr
	}
	return jobruntime.Result{Ref: &ref}, nil
}

func (s Service) analyzeImage(ctx context.Context, actor contract.ActorContext, asset assets.ProjectAsset, artifact *Artifact) error {
	if artifact == nil {
		return fmt.Errorf("media understanding artifact is required")
	}
	if s.Vision == nil {
		applyTechnicalFallback(asset, artifact, "vision_provider_unavailable")
		return nil
	}
	projectContext, err := s.Projects.GetContext(ctx, actor, artifact.ProjectID)
	if err != nil {
		return err
	}
	response, err := s.Vision.UnderstandVision(ctx, provider.VisionUnderstandRequest{
		Actor: actor, Project: projectContext, ModelAlias: s.modelAlias(),
		Input: provider.VisionUnderstandingInput{
			Instruction:  "分析广告创作素材。严格区分画面可见文字、直接观察、推断、风险和无法判断项；不要把推断写成事实。frame_index 对图片固定为 0。",
			SourceAssets: []contract.ProjectAssetRef{artifact.AssetRef}, OutputJSONSchema: visionOutputSchema(),
		},
	})
	if errors.Is(err, provider.ErrGatewayRouteNotFound) {
		applyTechnicalFallback(asset, artifact, "vision_route_unavailable")
		return nil
	}
	if err != nil {
		return err
	}
	if response.ProviderCode == "fake" {
		applyTechnicalFallback(asset, artifact, "fake_vision_no_semantic_claims")
		artifact.Lineage.ProviderCode, artifact.Lineage.ModelVersion = response.ProviderCode, response.ModelVersion
		return nil
	}
	return applyVisionResponse(artifact, response, nil)
}

func (s Service) analyzeVideo(ctx context.Context, actor contract.ActorContext, asset assets.ProjectAsset, artifact *Artifact) error {
	if artifact == nil || s.Frames == nil || s.DerivedImages == nil {
		return fmt.Errorf("video frame extraction is unavailable")
	}
	durationMS := asset.Version.DurationMS
	if durationMS == 0 {
		durationMS = int64(asset.Version.Media.DurationSeconds * 1000)
	}
	timestamps := sampleTimestamps(durationMS)
	keyframes := make([]Keyframe, 0, len(timestamps))
	for _, timestamp := range timestamps {
		frame, err := s.Frames.ExtractFrame(ctx, media.FrameExtractionRequest{
			OrganizationID: artifact.OrganizationID, ProjectID: artifact.ProjectID,
			SourceVideo: artifact.AssetRef.AssetVersion, TimestampMS: timestamp,
		})
		if err != nil {
			artifact.Warnings = append(artifact.Warnings, fmt.Sprintf("frame_extraction_failed:%d", timestamp))
			continue
		}
		derivationHash, _ := contract.CanonicalJSONHash(struct {
			Source    contract.AssetVersionRef `json:"source"`
			Timestamp int64                    `json:"timestamp_ms"`
			Version   string                   `json:"extractor_version"`
		}{artifact.AssetRef.AssetVersion, timestamp, frame.Version})
		requestContext := contract.RequestContext{
			RequestID: "media-" + artifact.ID, TraceID: "media-" + artifact.ID, Actor: actor,
		}
		frameRef, persistErr := s.DerivedImages.IngestDerivedImage(
			ctx, requestContext, artifact.ProjectID, "mediaframe_"+derivationHash[:48],
			artifact.AssetRef.AssetVersion, frame.Content, frame.SizeBytes, frame.MIMEType,
		)
		closeErr := frame.Content.Close()
		if persistErr != nil {
			return persistErr
		}
		if closeErr != nil {
			return closeErr
		}
		keyframes = append(keyframes, Keyframe{TimestampMS: timestamp, FrameRef: frameRef})
	}
	if len(keyframes) == 0 {
		return fmt.Errorf("video has no analyzable frames")
	}
	artifact.Keyframes = keyframes
	artifact.Warnings = append(artifact.Warnings, "uniform_time_sampling", "transcript_unavailable")
	if s.Vision == nil {
		applyTechnicalFallback(asset, artifact, "vision_provider_unavailable")
		return nil
	}
	projectContext, err := s.Projects.GetContext(ctx, actor, artifact.ProjectID)
	if err != nil {
		return err
	}
	refs := make([]contract.ProjectAssetRef, 0, len(keyframes))
	for _, frame := range keyframes {
		refs = append(refs, frame.FrameRef)
	}
	response, err := s.Vision.UnderstandVision(ctx, provider.VisionUnderstandRequest{
		Actor: actor, Project: projectContext, ModelAlias: s.modelAlias(),
		Input: provider.VisionUnderstandingInput{
			Instruction:  "这些图片按时间顺序来自同一条广告视频。分析叙事结构、镜头变化、画面可见文字、直接观察、推断、风险和无法判断项；frame_index 必须指向提供的帧序号（从 0 开始），不要把推断写成事实。",
			SourceAssets: refs, OutputJSONSchema: visionOutputSchema(),
		},
	})
	if errors.Is(err, provider.ErrGatewayRouteNotFound) {
		applyTechnicalFallback(asset, artifact, "vision_route_unavailable")
		return nil
	}
	if err != nil {
		return err
	}
	if response.ProviderCode == "fake" {
		applyTechnicalFallback(asset, artifact, "fake_vision_no_semantic_claims")
		artifact.Lineage.ProviderCode, artifact.Lineage.ModelVersion = response.ProviderCode, response.ModelVersion
		return nil
	}
	if err := applyVisionResponse(artifact, response, keyframes); err != nil {
		return err
	}
	artifact.Status = StatusPartial
	return nil
}

func applyVisionResponse(artifact *Artifact, response provider.SynchronousResponse, keyframes []Keyframe) error {
	raw := response.StructuredOutput
	if len(raw) == 0 && json.Valid([]byte(strings.TrimSpace(response.Text))) {
		raw = json.RawMessage(strings.TrimSpace(response.Text))
	}
	var output visionOutput
	if len(raw) == 0 || json.Unmarshal(raw, &output) != nil || strings.TrimSpace(output.Summary) == "" {
		return fmt.Errorf("vision provider returned invalid structured output")
	}
	artifact.Summary = strings.TrimSpace(output.Summary)
	if len([]rune(artifact.Summary)) > 800 {
		artifact.Summary = string([]rune(artifact.Summary)[:800])
	}
	var err error
	artifact.VisibleText, err = candidatesToEvidence(artifact, "visible", output.VisibleText, keyframes)
	if err != nil {
		return err
	}
	artifact.Observations, err = candidatesToEvidence(artifact, "observation", output.Observations, keyframes)
	if err != nil {
		return err
	}
	artifact.Inferences, err = candidatesToEvidence(artifact, "inference", output.Inferences, keyframes)
	if err != nil {
		return err
	}
	artifact.Risks, err = candidatesToEvidence(artifact, "risk", output.Risks, keyframes)
	if err != nil {
		return err
	}
	artifact.Unknowns, err = candidatesToEvidence(artifact, "unknown", output.Unknowns, keyframes)
	if err != nil {
		return err
	}
	artifact.Lineage.ProviderCode = response.ProviderCode
	artifact.Lineage.ModelVersion = response.ModelVersion
	artifact.Lineage.RouteRevisionID = response.RouteRevisionID
	artifact.Status = StatusReady
	return nil
}

func candidatesToEvidence(artifact *Artifact, prefix string, candidates []visionCandidate, keyframes []Keyframe) ([]Evidence, error) {
	if len(candidates) > 24 {
		return nil, fmt.Errorf("vision provider returned too many evidence items")
	}
	result := make([]Evidence, 0, len(candidates))
	for index, candidate := range candidates {
		candidate.Text = strings.TrimSpace(candidate.Text)
		if candidate.Text == "" || len([]rune(candidate.Text)) > 500 || candidate.Confidence < 0 || candidate.Confidence > 1 {
			return nil, fmt.Errorf("vision provider returned invalid evidence")
		}
		locator := Locator{Kind: "image", AssetRef: artifact.AssetRef}
		if artifact.AssetKind == contract.AssetVideo {
			if candidate.FrameIndex < 0 || candidate.FrameIndex >= len(keyframes) {
				return nil, fmt.Errorf("vision provider evidence points outside sampled frames")
			}
			frame := keyframes[candidate.FrameIndex]
			timestamp := frame.TimestampMS
			frameRef := frame.FrameRef
			locator = Locator{Kind: "video_frame", AssetRef: artifact.AssetRef, TimestampMS: &timestamp, FrameRef: &frameRef}
		}
		result = append(result, Evidence{
			ID: fmt.Sprintf("%s_%02d", prefix, index+1), Text: candidate.Text,
			Confidence: candidate.Confidence, Locator: locator,
		})
	}
	return result, nil
}

func applyTechnicalFallback(asset assets.ProjectAsset, artifact *Artifact, warning string) {
	description := fmt.Sprintf("素材技术校验通过：%s，%d × %d", asset.Version.MIMEType, asset.Version.WidthPixels, asset.Version.HeightPixels)
	if asset.Asset.Kind == contract.AssetVideo {
		durationMS := asset.Version.DurationMS
		if durationMS == 0 {
			durationMS = int64(asset.Version.Media.DurationSeconds * 1000)
		}
		description = fmt.Sprintf("视频技术校验通过：%.1f 秒，%d × %d，已抽取 %d 个可追溯帧", float64(durationMS)/1000, asset.Version.WidthPixels, asset.Version.HeightPixels, len(artifact.Keyframes))
	}
	artifact.Summary = "素材已经完成安全与技术校验；当前环境未产出可采信的语义理解。"
	artifact.Observations = []Evidence{{
		ID: "observation_01", Text: description, Confidence: 1,
		Locator: Locator{Kind: string(asset.Asset.Kind), AssetRef: artifact.AssetRef},
	}}
	artifact.Warnings = append(artifact.Warnings, warning)
	artifact.Status = StatusPartial
}

func sampleTimestamps(durationMS int64) []int64 {
	// Container duration usually points just beyond the final decodable frame.
	// Staying 500ms inside the tail is stable across common short-form frame rates.
	tail := durationMS - 500
	if tail < 0 {
		tail = 0
	}
	values := []int64{0, durationMS / 4, durationMS / 2, durationMS * 3 / 4, tail}
	result := make([]int64, 0, len(values))
	seen := map[int64]struct{}{}
	for _, value := range values {
		if value < 0 {
			value = 0
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func mediaFailureCode(err error) string {
	if errors.Is(err, ErrUnsupportedProfile) {
		return "UNSUPPORTED_FOR_PROFILE"
	}
	if strings.Contains(strings.ToLower(err.Error()), "frame") {
		return "MEDIA_FRAME_EXTRACTION_FAILED"
	}
	return "MEDIA_UNDERSTANDING_FAILED"
}

func mediaFailureMessage(err error) string {
	if errors.Is(err, ErrUnsupportedProfile) {
		return "素材不符合 15–90 秒图片/短视频理解范围"
	}
	return "素材理解失败；原始素材仍然保留，可重试或仅作为创作输入使用"
}
