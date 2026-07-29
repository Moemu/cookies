package creative

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

type ViralAnalysisRequest struct {
	TaskID        string
	InputSnapshot ViralRemakeInputSnapshot
}

type ViralAnalysisResult struct {
	Dimensions      []ViralAnalysisDimension
	PreserveRules   []string
	ReplaceRules    []string
	Transcript      string
	Confidence      float64
	EvidenceRefs    []string
	RouteRevisionID string
	PromptVersion   string
}

type ViralReferenceAnalyzer interface {
	Analyze(context.Context, contract.ActorContext, contract.ProjectID, ViralAnalysisRequest) (ViralAnalysisResult, error)
}

type UpdateViralPromptRequest struct {
	ExpectedRevision int64                             `json:"expected_revision"`
	Dimensions       map[ViralPromptDimensionID]string `json:"dimensions"`
}

type ConfirmViralGenerationRequest struct {
	ExpectedRevision            int64 `json:"expected_revision"`
	ConfirmReferenceVideoRights bool  `json:"confirm_reference_video_rights"`
	ConfirmReferenceImageRights bool  `json:"confirm_reference_image_rights"`
}

func (s Service) AnalyzeViralRemake(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string) (TaskDetail, error) {
	detail, err := s.requireViralWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if s.ViralAnalyzer == nil {
		return TaskDetail{}, fmt.Errorf("viral reference analyzer is not configured")
	}
	viral := detail.VideoDraft.ViralRemake
	result, err := s.ViralAnalyzer.Analyze(ctx, actor, projectID, ViralAnalysisRequest{
		TaskID: taskID, InputSnapshot: viral.InputSnapshot,
	})
	if err != nil {
		return TaskDetail{}, err
	}
	if err := validateViralAnalysisResult(result); err != nil {
		return TaskDetail{}, err
	}
	now := s.now()
	snapshot := ViralAnalysisSnapshot{
		ContractVersion: "creative-viral-analysis-snapshot/v1", TaskID: taskID,
		SourceAssetRef: viral.InputSnapshot.ReferenceVideo,
		Dimensions:     append([]ViralAnalysisDimension{}, result.Dimensions...),
		PreserveRules:  append([]string{}, result.PreserveRules...),
		ReplaceRules:   append([]string{}, result.ReplaceRules...),
		Transcript:     strings.TrimSpace(result.Transcript), Confidence: result.Confidence,
		EvidenceRefs: append([]string{}, result.EvidenceRefs...),
		ModelLineage: ViralModelLineage{
			ModelAlias: "cookies.vision.standard", RouteRevisionID: result.RouteRevisionID,
			PromptVersion: result.PromptVersion,
		},
		CreatedAt: now,
	}
	hashValue := snapshot
	hashValue.ContentHash = ""
	hash, err := contract.CanonicalJSONHash(hashValue)
	if err != nil {
		return TaskDetail{}, err
	}
	snapshot.ContentHash = "sha256:" + hash
	dimensions := make(map[ViralPromptDimensionID]string, len(snapshot.Dimensions))
	for _, dimension := range snapshot.Dimensions {
		dimensions[dimension.ID] = dimension.Prompt
	}
	promptDraft := ViralPromptDraft{
		Revision: 1, Dimensions: dimensions,
		CompositePrompt: compileViralCompositePrompt(viral.InputSnapshot, dimensions, snapshot.PreserveRules, snapshot.ReplaceRules),
		UpdatedAt:       now,
	}
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *viral
	updated.Revision = next.Revision
	updated.Status = ViralAnalysisReady
	updated.Analysis = &snapshot
	updated.PromptDraft = &promptDraft
	if updated.Candidates == nil {
		updated.Candidates = []ViralCandidate{}
	}
	updated.Readiness.GenerationReady = false
	updated.Readiness.ProductionReady = false
	updated.Readiness.Blockers = replaceBlockers(updated.Readiness.Blockers, []string{"analysis_snapshot"}, []string{"confirmed_prompt_package"})
	updated.UpdatedAt = now
	next.ViralRemake = &updated
	next.Prompt = promptDraft.CompositePrompt
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, TaskInProgress); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) UpdateViralPrompt(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request UpdateViralPromptRequest) (TaskDetail, error) {
	detail, err := s.requireViralWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	viral := detail.VideoDraft.ViralRemake
	if viral.Analysis == nil || viral.PromptDraft == nil || request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	if err := validateViralDimensionMap(request.Dimensions); err != nil {
		return TaskDetail{}, err
	}
	now := s.now()
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *viral
	updated.Revision = next.Revision
	updated.Status = ViralAnalysisReady
	updated.PromptDraft = &ViralPromptDraft{
		Revision:   viral.PromptDraft.Revision + 1,
		Dimensions: cloneViralDimensions(request.Dimensions),
		CompositePrompt: compileViralCompositePrompt(
			viral.InputSnapshot, request.Dimensions, viral.Analysis.PreserveRules, viral.Analysis.ReplaceRules,
		),
		UpdatedAt: now,
	}
	updated.PromptPackage = nil
	updated.Readiness.GenerationReady = false
	updated.Readiness.ProductionReady = false
	updated.Readiness.Blockers = appendUnique(removeStrings(updated.Readiness.Blockers, "confirmed_prompt_package"), "confirmed_prompt_package")
	updated.UpdatedAt = now
	next.ViralRemake = &updated
	next.Prompt = updated.PromptDraft.CompositePrompt
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, TaskInProgress); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) ConfirmViralGeneration(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request ConfirmViralGenerationRequest) (TaskDetail, error) {
	detail, err := s.requireViralWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	viral := detail.VideoDraft.ViralRemake
	if request.ExpectedRevision != detail.VideoDraft.Revision || viral.Analysis == nil || viral.PromptDraft == nil {
		return TaskDetail{}, ErrVersionConflict
	}
	if !request.ConfirmReferenceVideoRights || (viral.InputSnapshot.ReferenceImage != nil && !request.ConfirmReferenceImageRights) {
		return TaskDetail{}, fmt.Errorf("reference asset rights must be explicitly confirmed")
	}
	now := s.now()
	input := viral.InputSnapshot
	input.ReferenceVideoRights = RightsConfirmed
	if input.ReferenceImage != nil {
		input.ReferenceImageRights = RightsConfirmed
	}
	generation := ViralGenerationSpec{
		ModelAlias: "cookies.video.standard", DurationSeconds: minInt(detail.VideoDraft.DurationSeconds, 15),
		AspectRatio: "9:16", Resolution: "720p", CandidateCount: 1,
	}
	pkg := ViralPromptPackage{
		ContractVersion: "creative-viral-prompt-package/v1", TaskID: taskID,
		PromptVersion:        viral.PromptDraft.Revision,
		AnalysisSnapshotHash: viral.Analysis.ContentHash, InputSnapshotHash: hashRef(viral.InputHash),
		Dimensions:          cloneViralDimensions(viral.PromptDraft.Dimensions),
		PreserveRules:       append([]string{}, viral.Analysis.PreserveRules...),
		ReplaceRules:        append([]string{}, viral.Analysis.ReplaceRules...),
		ProductFacts:        append([]string{input.ProductName}, input.SellingPoints...),
		NegativeConstraints: append([]string{}, input.ProhibitedClaims...),
		GenerationSpec:      generation, CompositePrompt: viral.PromptDraft.CompositePrompt,
		ConfirmedBy: actor.Principal.ID, ConfirmedAt: now,
	}
	hashValue := pkg
	hashValue.ContentHash = ""
	hash, err := contract.CanonicalJSONHash(hashValue)
	if err != nil {
		return TaskDetail{}, err
	}
	pkg.ContentHash = "sha256:" + hash
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	next.Prompt = pkg.CompositePrompt
	next.DurationSeconds = generation.DurationSeconds
	updated := *viral
	updated.Revision = next.Revision
	updated.Status = ViralGenerationReady
	updated.InputSnapshot = input
	updated.PromptPackage = &pkg
	updated.Readiness.GenerationReady = true
	updated.Readiness.ProductionReady = false
	updated.Readiness.Blockers = removeStrings(updated.Readiness.Blockers,
		"confirmed_prompt_package", "reference_video_rights", "reference_image_rights", "provider_video_route")
	updated.UpdatedAt = now
	next.ViralRemake = &updated
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, TaskInProgress); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) ViralProviderInput(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string) (provider.VideoGenerationInput, string, error) {
	detail, err := s.requireViralWorkspace(ctx, actor, projectID, taskID, false)
	if err != nil {
		return provider.VideoGenerationInput{}, "", err
	}
	viral := detail.VideoDraft.ViralRemake
	if !viral.Readiness.GenerationReady || viral.PromptPackage == nil || viral.Status != ViralGenerationReady {
		return provider.VideoGenerationInput{}, "", ErrInvalidState
	}
	input := provider.VideoGenerationInput{
		Prompt:             viral.PromptPackage.CompositePrompt,
		DurationSeconds:    viral.PromptPackage.GenerationSpec.DurationSeconds,
		AspectRatio:        viral.PromptPackage.GenerationSpec.AspectRatio,
		Resolution:         viral.PromptPackage.GenerationSpec.Resolution,
		AudioPolicy:        provider.VideoAudioGenerated,
		InputMode:          provider.VideoInputTextOnly,
		ConditioningAssets: []provider.VideoConditioningAsset{},
	}
	if ref := viral.InputSnapshot.ReferenceImage; ref != nil {
		input.InputMode = provider.VideoInputReferenceImage
		input.ConditioningAssets = []provider.VideoConditioningAsset{{
			Role:      provider.VideoConditioningReferenceImage,
			Reference: contract.ProjectAssetRef{ProjectID: projectID, AssetVersion: *ref},
		}}
	}
	return input, viral.PromptPackage.ContentHash, input.Validate()
}

func (s Service) RegisterViralCandidateJob(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID, providerJobID string) (TaskDetail, error) {
	detail, err := s.requireViralWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	viral := detail.VideoDraft.ViralRemake
	if viral.PromptPackage == nil || viral.Status != ViralGenerationReady || strings.TrimSpace(providerJobID) == "" {
		return TaskDetail{}, ErrInvalidState
	}
	for _, candidate := range viral.Candidates {
		if candidate.ProviderJobID == providerJobID {
			return detail, nil
		}
	}
	candidateID, err := s.idGenerator()("viralcandidate")
	if err != nil {
		return TaskDetail{}, err
	}
	now := s.now()
	if err := s.Repository.RegisterProductionJob(ctx, actor.OrganizationID, projectID, taskID, ProductionJob{
		TaskID: taskID, Kind: "viral_candidate_" + candidateID, ProviderJobID: providerJobID, CreatedAt: now,
	}); err != nil {
		return TaskDetail{}, err
	}
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *viral
	updated.Revision = next.Revision
	updated.Status = ViralGenerating
	updated.Readiness.GenerationReady = true
	updated.Readiness.ProductionReady = false
	updated.Candidates = append(append([]ViralCandidate{}, viral.Candidates...), ViralCandidate{
		ID: candidateID, ProviderJobID: providerJobID, PromptHash: viral.PromptPackage.ContentHash,
		Status: ViralCandidateQueued, Checks: []ViralCandidateCheck{}, CreatedAt: now, UpdatedAt: now,
	})
	updated.UpdatedAt = now
	next.ViralRemake = &updated
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, TaskGenerating); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) ReconcileViralCandidate(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, job contract.ProviderJob) (TaskDetail, error) {
	detail, err := s.requireViralWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	viral := detail.VideoDraft.ViralRemake
	index := -1
	for i := range viral.Candidates {
		if viral.Candidates[i].ProviderJobID == job.ID {
			index = i
			break
		}
	}
	if index < 0 {
		return TaskDetail{}, ErrNotFound
	}
	current := viral.Candidates[index]
	status := candidateStatusForProvider(job.ProviderStatus)
	if current.Status == status && (status != ViralCandidateSucceeded || current.OutputAssetRef != nil) {
		return detail, nil
	}
	now := s.now()
	candidate := current
	candidate.Status = status
	candidate.UpdatedAt = now
	if job.Error != nil {
		candidate.ErrorCode, candidate.ErrorMessage = job.Error.Code, job.Error.Message
	}
	if status == ViralCandidateSucceeded {
		if len(job.ProjectAssetRefs) != 1 || job.ProjectAssetRefs[0].ProjectID != projectID {
			return TaskDetail{}, fmt.Errorf("successful viral candidate requires one project asset")
		}
		ref := job.ProjectAssetRefs[0].AssetVersion
		candidate.OutputAssetRef = &ref
		candidate.Checks = viralCandidateChecks(*viral, candidate)
	}
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *viral
	updated.Revision = next.Revision
	updated.Candidates = append([]ViralCandidate{}, viral.Candidates...)
	updated.Candidates[index] = candidate
	switch status {
	case ViralCandidateSucceeded:
		updated.Status = ViralCandidateReady
		updated.Readiness.ProductionReady = allCandidateChecksPassed(candidate.Checks)
	case ViralCandidateFailed:
		updated.Status = ViralProviderFailed
		updated.Readiness.ProductionReady = false
	default:
		updated.Status = ViralGenerating
	}
	updated.UpdatedAt = now
	next.ViralRemake = &updated
	taskStatus := TaskGenerating
	if status == ViralCandidateSucceeded {
		taskStatus = TaskReady
	} else if status == ViralCandidateFailed {
		taskStatus = TaskInProgress
	}
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, taskStatus); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func (s Service) SubmitViralCandidateReview(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID, candidateID string) (TaskDetail, error) {
	detail, err := s.requireViralWorkspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	viral := detail.VideoDraft.ViralRemake
	index := -1
	for i := range viral.Candidates {
		if viral.Candidates[i].ID == candidateID {
			index = i
			break
		}
	}
	if index < 0 || viral.Candidates[index].Status != ViralCandidateSucceeded ||
		!allCandidateChecksPassed(viral.Candidates[index].Checks) {
		return TaskDetail{}, ErrInvalidState
	}
	now := s.now()
	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *viral
	updated.Revision = next.Revision
	updated.Status = ViralReadyForReview
	updated.Candidates = append([]ViralCandidate{}, viral.Candidates...)
	updated.Candidates[index].Status = ViralCandidateReviewed
	updated.Candidates[index].UpdatedAt = now
	updated.Readiness.ProductionReady = true
	updated.UpdatedAt = now
	next.ViralRemake = &updated
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, TaskReady); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}

func candidateStatusForProvider(status contract.ProviderJobStatus) ViralCandidateStatus {
	switch status {
	case contract.ProviderJobSubmitted:
		return ViralCandidateQueued
	case contract.ProviderJobRunning, contract.ProviderJobOutputsReady, contract.ProviderJobIngesting:
		return ViralCandidateRunning
	case contract.ProviderJobSucceeded, contract.ProviderJobPartiallySucceeded:
		return ViralCandidateSucceeded
	default:
		return ViralCandidateFailed
	}
}

func viralCandidateChecks(viral ViralRemakeDraft, candidate ViralCandidate) []ViralCandidateCheck {
	return []ViralCandidateCheck{
		{Code: "asset_ingested", Passed: candidate.OutputAssetRef != nil, Message: "Provider output is stored as an Assets-owned version."},
		{Code: "rights_confirmed", Passed: viral.InputSnapshot.ReferenceVideoRights == RightsConfirmed &&
			(viral.InputSnapshot.ReferenceImage == nil || viral.InputSnapshot.ReferenceImageRights == RightsConfirmed),
			Message: "Reference asset rights were explicitly confirmed."},
		{Code: "prompt_lineage", Passed: viral.PromptPackage != nil && candidate.PromptHash == viral.PromptPackage.ContentHash,
			Message: "Candidate points to the confirmed immutable PromptPackage."},
		{Code: "originality_guardrail", Passed: viral.Analysis != nil && len(viral.Analysis.ReplaceRules) > 0,
			Message: "Protected expressions are replaced rather than copied."},
	}
}

func allCandidateChecksPassed(checks []ViralCandidateCheck) bool {
	if len(checks) != 4 {
		return false
	}
	for _, check := range checks {
		if !check.Passed {
			return false
		}
	}
	return true
}

func (s Service) requireViralWorkspace(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, write bool) (TaskDetail, error) {
	if s.Repository == nil || s.ViralRemakes == nil || s.Projects == nil {
		return TaskDetail{}, fmt.Errorf("viral remake dependencies are incomplete")
	}
	required := ScopeRead
	if write {
		required = ScopeWrite
	}
	if !actor.HasScope(required) {
		return TaskDetail{}, fmt.Errorf("%s scope is required", required)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return TaskDetail{}, err
	}
	detail, err := s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
	if err != nil {
		return TaskDetail{}, err
	}
	if detail.Task.Format != FormatVideo || detail.Task.PerformanceMode != PerformanceModeViralRemake ||
		detail.VideoDraft == nil || detail.VideoDraft.ViralRemake == nil || detail.Task.Status == TaskArchived {
		return TaskDetail{}, ErrInvalidState
	}
	return detail, nil
}

func validateViralAnalysisResult(result ViralAnalysisResult) error {
	if len(result.Dimensions) != len(viralDimensionOrder) || strings.TrimSpace(result.RouteRevisionID) == "" ||
		strings.TrimSpace(result.PromptVersion) == "" || result.Confidence < 0 || result.Confidence > 1 {
		return fmt.Errorf("viral analysis result is incomplete")
	}
	seen := make(map[ViralPromptDimensionID]bool, len(result.Dimensions))
	for _, dimension := range result.Dimensions {
		if strings.TrimSpace(dimension.Prompt) == "" || dimension.Source != "ai_extracted" ||
			dimension.Confidence < 0 || dimension.Confidence > 1 {
			return fmt.Errorf("viral analysis dimension is invalid")
		}
		seen[dimension.ID] = true
	}
	for _, id := range viralDimensionOrder {
		if !seen[id] {
			return fmt.Errorf("viral analysis dimension %q is required", id)
		}
	}
	return nil
}

func validateViralDimensionMap(values map[ViralPromptDimensionID]string) error {
	if len(values) != len(viralDimensionOrder) {
		return fmt.Errorf("all five viral prompt dimensions are required")
	}
	for _, id := range viralDimensionOrder {
		if strings.TrimSpace(values[id]) == "" {
			return fmt.Errorf("viral prompt dimension %q is required", id)
		}
	}
	return nil
}

func compileViralCompositePrompt(input ViralRemakeInputSnapshot, dimensions map[ViralPromptDimensionID]string, preserve, replace []string) string {
	lines := []string{
		"生成一条原创效果广告，只复用参考视频的抽象节奏与镜头功能，不复制人物、商标、字幕、音乐或受保护表达。",
		"目标产品：" + input.ProductName,
		"产品卖点：" + strings.Join(input.SellingPoints, "；"),
		"CTA：" + input.CallToAction,
		"用户指令：" + input.UserInstruction,
	}
	for _, id := range viralDimensionOrder {
		lines = append(lines, string(id)+"："+strings.TrimSpace(dimensions[id]))
	}
	lines = append(lines, "保留规则："+strings.Join(preserve, "；"), "替换规则："+strings.Join(replace, "；"))
	return strings.Join(lines, "\n")
}

func cloneViralDimensions(values map[ViralPromptDimensionID]string) map[ViralPromptDimensionID]string {
	cloned := make(map[ViralPromptDimensionID]string, len(values))
	for key, value := range values {
		cloned[key] = strings.TrimSpace(value)
	}
	return cloned
}

func replaceBlockers(values, remove, add []string) []string {
	result := removeStrings(values, remove...)
	for _, value := range add {
		result = appendUnique(result, value)
	}
	return result
}

func removeStrings(values []string, removals ...string) []string {
	removed := make(map[string]bool, len(removals))
	for _, value := range removals {
		removed[value] = true
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if !removed[value] {
			result = appendUnique(result, value)
		}
	}
	return result
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func hashRef(value string) string {
	value = strings.TrimPrefix(strings.TrimSpace(value), "sha256:")
	return "sha256:" + value
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
