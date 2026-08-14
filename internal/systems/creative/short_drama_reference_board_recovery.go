package creative

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	referenceBoardAttemptInitial        = "initial"
	referenceBoardAttemptTransientRetry = "transient_retry"
	referenceBoardAttemptPolicyRewrite  = "policy_rewrite"
	referenceBoardAttemptStyleFallback  = "style_fallback"
	referenceBoardPolicyRewriteVersion  = "short-drama-reference-board-policy-safe/v1"
	referenceBoardStyleFallbackVersion  = "short-drama-reference-board-style-fallback/v1"
	referenceBoardMaxAttempts           = 3

	referenceBoardFailureInputPolicy     = "input_policy"
	referenceBoardFailureOutputPolicy    = "output_policy"
	referenceBoardFailureRequestRejected = "policy_or_request_rejected"
	referenceBoardFailureCapacity        = "capacity"
	referenceBoardFailureTransient       = "transient"
	referenceBoardFailureSubmission      = "submission_unknown"
	referenceBoardFailureConfiguration   = "configuration"
	referenceBoardFailureUnknown         = "unknown"
)

type RetryShortDramaReferenceBoardCandidateRequest struct {
	ExpectedRevision int64  `json:"expected_revision"`
	BatchID          string `json:"batch_id"`
	CandidateID      string `json:"candidate_id"`
	FailedAttemptID  string `json:"failed_attempt_id"`
}

func initialReferenceBoardAttempt(candidateID, promptHash, providerJobID string, status ShortDramaV2ResourceStatus, now time.Time) ShortDramaReferenceBoardAttempt {
	return ShortDramaReferenceBoardAttempt{
		ID: candidateID + "_attempt_0", Ordinal: 0, Mode: referenceBoardAttemptInitial,
		PromptHash: promptHash, ProviderJobID: providerJobID, Status: status, CreatedAt: now,
	}
}

func ensureReferenceBoardAttemptHistory(candidate *ShortDramaReferenceBoardCandidate, now time.Time) {
	if len(candidate.Attempts) > 0 {
		return
	}
	attempt := initialReferenceBoardAttempt(candidate.ID, candidate.PromptHash, candidate.ProviderJobID, candidate.Status, now)
	if candidate.Status == ShortDramaV2ResourceFailed || candidate.Status == ShortDramaV2ResourceCancelled {
		completed := now
		attempt.CompletedAt = &completed
		attempt.ProviderErrorCode = candidate.ErrorCode
		attempt.FailureClass = candidate.FailureClass
		attempt.Retryable = candidate.Recoverable
	}
	candidate.Attempts = []ShortDramaReferenceBoardAttempt{attempt}
	candidate.CurrentAttemptID = attempt.ID
}

func classifyReferenceBoardFailure(job contract.ProviderJob) (string, bool, string) {
	code := "REFERENCE_BOARD_GENERATION_FAILED"
	retryable := false
	if job.Error != nil {
		code = strings.TrimSpace(job.Error.Code)
		retryable = job.Error.Retryable
	}
	switch code {
	case "MODEL_INPUT_POLICY_REJECTED":
		return referenceBoardFailureInputPolicy, true, code
	case "MODEL_OUTPUT_POLICY_REJECTED":
		return referenceBoardFailureOutputPolicy, true, code
	case "MODEL_REQUEST_REJECTED":
		return referenceBoardFailureRequestRejected, true, code
	case "MODEL_RATE_LIMITED", "QUOTA_EXCEEDED":
		return referenceBoardFailureCapacity, true, code
	case "MODEL_SUBMISSION_UNKNOWN":
		return referenceBoardFailureSubmission, false, code
	case "MODEL_AUTH_REJECTED", "MODEL_AUTH_UNAVAILABLE", "MODEL_INPUT_UNSUPPORTED", "MODEL_OUTPUT_UNSUPPORTED":
		return referenceBoardFailureConfiguration, false, code
	default:
		if retryable {
			return referenceBoardFailureTransient, true, code
		}
		return referenceBoardFailureUnknown, false, code
	}
}

func refreshReferenceBoardBatchState(batch *ShortDramaReferenceBoardBatch) {
	batch.DesiredCount = len(batch.Candidates)
	batch.ReadyCount, batch.RunningCount, batch.FailedCount, batch.RecoverableFailedCount = 0, 0, 0, 0
	for _, candidate := range batch.Candidates {
		switch candidate.Status {
		case ShortDramaV2ResourceReady:
			batch.ReadyCount++
		case ShortDramaV2ResourceQueued, ShortDramaV2ResourceRunning:
			batch.RunningCount++
		case ShortDramaV2ResourceFailed, ShortDramaV2ResourceCancelled:
			batch.FailedCount++
			if candidate.Recoverable {
				batch.RecoverableFailedCount++
			}
		}
	}
	switch {
	case batch.DesiredCount > 0 && batch.ReadyCount == batch.DesiredCount:
		batch.Status = ShortDramaV2ResourceReady
	case batch.RunningCount > 0:
		batch.Status = ShortDramaV2ResourceRunning
	case batch.ReadyCount > 0:
		batch.Status = ShortDramaV2ResourcePartial
	case batch.FailedCount == batch.DesiredCount:
		batch.Status = ShortDramaV2ResourceFailed
	default:
		batch.Status = ShortDramaV2ResourceQueued
	}
}

func referenceBoardRecoveryMode(candidate ShortDramaReferenceBoardCandidate) (string, string, bool) {
	nextOrdinal := len(candidate.Attempts)
	if nextOrdinal >= referenceBoardMaxAttempts {
		return "", "", false
	}
	if nextOrdinal == 1 && (candidate.FailureClass == referenceBoardFailureCapacity || candidate.FailureClass == referenceBoardFailureTransient) {
		return referenceBoardAttemptTransientRetry, "", true
	}
	if nextOrdinal == 1 {
		return referenceBoardAttemptPolicyRewrite, referenceBoardPolicyRewriteVersion, true
	}
	if candidate.FailureClass == referenceBoardFailureInputPolicy || candidate.FailureClass == referenceBoardFailureOutputPolicy || candidate.FailureClass == referenceBoardFailureRequestRejected {
		return referenceBoardAttemptStyleFallback, referenceBoardStyleFallbackVersion, true
	}
	return "", "", false
}

func compileReferenceBoardRecoveryPrompt(plan ShortDramaReferenceBoardPlan, creativeDirection string, candidate ShortDramaReferenceBoardCandidate, mode string, board ShortDramaBoardCanvas, output ShortDramaOutputCanvas) (string, error) {
	variantInstruction := ""
	for _, variant := range shortDramaReferenceBoardVariants {
		if variant.Key == candidate.PrimaryTestVariable {
			variantInstruction = variant.Instruction
			break
		}
	}
	if variantInstruction == "" {
		return "", fmt.Errorf("unknown reference board test variable %q", candidate.PrimaryTestVariable)
	}
	prompt := compileShortDramaReferenceBoardImagePrompt(plan, creativeDirection, candidate.PrimaryTestVariable, variantInstruction, board, output)
	switch mode {
	case referenceBoardAttemptTransientRetry:
		return prompt, nil
	case referenceBoardAttemptPolicyRewrite:
		return prompt + "\n安全改写要求：保持剧情证据、人物身份、时代、场景、A/B/C/D 四格职责和主要测试变量不变；使用原创虚构角色；将伤害、血腥、威胁或违法动作改为冲突前一刻、秘密即将暴露、紧张对峙等非具象表达；不得模仿真实演员、公众人物、已知角色或品牌 IP。", nil
	case referenceBoardAttemptStyleFallback:
		return prompt + "\n安全风格回退：保持全部剧情事实、四格职责和主要测试变量不变；改用原创国漫半写实电影概念设定板，弱化冲突和伤害，不生成真人摄影感、真实演员相似脸、血腥细节、Logo、水印或可读文字。", nil
	default:
		return "", fmt.Errorf("unsupported reference board recovery mode %q", mode)
	}
}

func (s Service) RetryShortDramaReferenceBoardCandidate(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, taskID string, request RetryShortDramaReferenceBoardCandidateRequest) (TaskDetail, error) {
	detail, err := s.requireShortDramaV2Workspace(ctx, actor, projectID, taskID, true)
	if err != nil {
		return TaskDetail{}, err
	}
	if request.ExpectedRevision != detail.VideoDraft.Revision {
		return TaskDetail{}, ErrVersionConflict
	}
	workspace := detail.VideoDraft.ShortDramaPrerollV2
	if workspace.ReferenceBoardBatch == nil || workspace.BoardCanvas == nil || workspace.OutputCanvas == nil || workspace.PromptDraft == nil || workspace.ReferenceBoardBatch.ID != request.BatchID || s.ShortDramaV2Images == nil {
		return TaskDetail{}, ErrInvalidState
	}
	batch := *workspace.ReferenceBoardBatch
	batch.Candidates = append([]ShortDramaReferenceBoardCandidate(nil), batch.Candidates...)
	index := -1
	for i := range batch.Candidates {
		if batch.Candidates[i].ID == request.CandidateID {
			index = i
			break
		}
	}
	if index < 0 {
		return TaskDetail{}, ErrInvalidState
	}
	now := s.now()
	candidate := batch.Candidates[index]
	ensureReferenceBoardAttemptHistory(&candidate, now)
	if candidate.Status != ShortDramaV2ResourceFailed || !candidate.Recoverable || candidate.CurrentAttemptID != request.FailedAttemptID {
		return TaskDetail{}, ErrInvalidState
	}
	mode, policyVersion, ok := referenceBoardRecoveryMode(candidate)
	if !ok {
		return TaskDetail{}, ErrInvalidState
	}
	prompt, err := compileReferenceBoardRecoveryPrompt(candidate.Plan, workspace.PromptDraft.ImagePrompt, candidate, mode, *workspace.BoardCanvas, *workspace.OutputCanvas)
	if err != nil {
		return TaskDetail{}, err
	}
	promptHashValue, err := contract.CanonicalJSONHash(struct {
		PlanHash string `json:"plan_hash"`
		Variable string `json:"primary_test_variable"`
		Mode     string `json:"mode"`
		Prompt   string `json:"prompt"`
	}{candidate.Plan.ContentHash, candidate.PrimaryTestVariable, mode, prompt})
	if err != nil {
		return TaskDetail{}, err
	}
	promptHash := "sha256:" + promptHashValue
	ordinal := len(candidate.Attempts)
	attempt := ShortDramaReferenceBoardAttempt{
		ID: candidate.ID + fmt.Sprintf("_attempt_%d", ordinal), Ordinal: ordinal, Mode: mode,
		RewritePolicyVersion: policyVersion, SourcePromptHash: candidate.PromptHash, PromptHash: promptHash,
		Status: ShortDramaV2ResourceQueued, CreatedAt: now,
	}
	project, err := s.Projects.RequireActiveContext(ctx, actor, projectID)
	if err != nil {
		return TaskDetail{}, err
	}
	job, err := s.ShortDramaV2Images.CreateFirstFrameJob(ctx, actor, project, ShortDramaV2FirstFrameJobRequest{
		TaskID: taskID, BatchID: batch.ID, CandidateID: candidate.ID, VariantIndex: candidate.VariantIndex,
		Prompt: prompt, PromptHash: promptHash, Width: workspace.BoardCanvas.Width, Height: workspace.BoardCanvas.Height,
	})
	if err != nil {
		return TaskDetail{}, fmt.Errorf("create reference board recovery job: %w", err)
	}
	attempt.ProviderJobID = job.ID
	candidate.Attempts = append(candidate.Attempts, attempt)
	candidate.CurrentAttemptID = attempt.ID
	candidate.ProviderJobID = job.ID
	candidate.PromptHash = promptHash
	candidate.Status = ShortDramaV2ResourceQueued
	candidate.RecoveryState = mode
	candidate.FailureClass = ""
	candidate.Recoverable = false
	candidate.ErrorCode, candidate.ErrorMessage = "", ""
	batch.Candidates[index] = candidate
	refreshReferenceBoardBatchState(&batch)

	next := *detail.VideoDraft
	next.Revision++
	next.CreatedAt = now
	updated := *workspace
	updated.Revision, updated.ReferenceBoardBatch, updated.UpdatedAt = next.Revision, &batch, now
	if batch.SelectedCandidateID == "" {
		updated.ActiveStage = ShortDramaV2StageFramesGenerating
	}
	next.ShortDramaPrerollV2 = &updated
	if _, err := s.ViralRemakes.ReviseVideoDraft(ctx, actor.OrganizationID, projectID, taskID, detail.VideoDraft.Revision, next, TaskGenerating); err != nil {
		return TaskDetail{}, err
	}
	return s.Repository.GetTaskDetail(ctx, actor.OrganizationID, projectID, taskID)
}
