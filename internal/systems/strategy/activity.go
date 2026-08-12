package strategy

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	TaskActivityContractVersion         = "strategy-task-activity/v1"
	TaskActivitySnapshotContractVersion = "strategy-task-activity-snapshot/v1"
	ActivityStallAfter                  = 60 * time.Second
)

type ActivityRound struct {
	Current int `json:"current"`
	Max     int `json:"max"`
}

type ActivityProgress struct {
	Kind    string `json:"kind"`
	Value   *int   `json:"value"`
	Message string `json:"message"`
}

type ActivityConclusion struct {
	ID          string `json:"id"`
	Text        string `json:"text"`
	Status      string `json:"status"`
	SourceCount int    `json:"source_count"`
}

type ActivitySourceScope struct {
	ProjectID      contract.ProjectID `json:"project_id"`
	WorkspaceID    string             `json:"workspace_id,omitempty"`
	ConversationID string             `json:"conversation_id,omitempty"`
}

type ActivityFailure struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// TaskActivity is a read model over existing AgentTask, Job, ResearchRun, and
// Document facts. It is not an executable task or a new workflow authority.
type TaskActivity struct {
	ContractVersion      string                `json:"contract_version"`
	ID                   string                `json:"id"`
	Kind                 string                `json:"kind"`
	Status               string                `json:"status"`
	Phase                string                `json:"phase"`
	Round                *ActivityRound        `json:"round"`
	Progress             ActivityProgress      `json:"progress"`
	Summary              string                `json:"summary"`
	ConfirmedConclusions []ActivityConclusion  `json:"confirmed_conclusions"`
	SourceScope          ActivitySourceScope   `json:"source_scope"`
	ResourceRef          contract.ResourceRef  `json:"resource_ref"`
	ExecutionRef         *contract.ResourceRef `json:"execution_ref,omitempty"`
	Actions              []string              `json:"actions"`
	CancelRequested      bool                  `json:"cancel_requested"`
	Failure              *ActivityFailure      `json:"failure,omitempty"`
	HeartbeatAt          *time.Time            `json:"heartbeat_at"`
	UpdatedAt            time.Time             `json:"updated_at"`
}

type TaskActivitySnapshot struct {
	ContractVersion string         `json:"contract_version"`
	SnapshotID      string         `json:"snapshot_id"`
	CapturedAt      time.Time      `json:"captured_at"`
	Items           []TaskActivity `json:"items"`
}

type activityRow struct {
	ID                     string
	ProjectID              contract.ProjectID
	AgentKind              string
	DomainKind             string
	DomainPurpose          string
	DomainStatus           string
	ExecutionStatus        string
	SourceType             string
	SourceID               string
	WorkspaceID            string
	ConversationID         string
	ResourceType           string
	ResourceID             string
	ResourceVersion        int64
	ExecutionType          string
	ExecutionID            string
	ExecutionVersion       int64
	Progress               int
	ProgressMessage        string
	DocumentPhase          string
	DocumentProgressKind   string
	DocumentProgress       *int
	DocumentProcessedPages *int
	DocumentTotalPages     *int
	Cancellable            bool
	CancelRequested        bool
	AttemptCount           int
	MaxAttempts            int
	HeartbeatAt            *time.Time
	UpdatedAt              time.Time
	FailureCode            string
	FailureMessage         string
	FailureRetryable       bool
}

func (s Service) ListTaskActivities(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	workspaceID string,
	limit int,
) (TaskActivitySnapshot, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return TaskActivitySnapshot{}, err
	}
	if strings.TrimSpace(string(projectID)) == "" {
		return TaskActivitySnapshot{}, ErrInvalidRequest
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return TaskActivitySnapshot{}, err
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := s.listAgentActivityRows(ctx, actor.OrganizationID, projectID, limit*2)
	if err != nil {
		return TaskActivitySnapshot{}, err
	}
	knowledgeRows, err := s.listKnowledgeActivityRows(ctx, actor.OrganizationID, projectID, limit*4)
	if err != nil {
		return TaskActivitySnapshot{}, err
	}
	rows = append(rows, knowledgeRows...)
	return taskActivitySnapshot(rows, s.now(), strings.TrimSpace(workspaceID), limit)
}

func (s Service) listAgentActivityRows(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	limit int,
) ([]activityRow, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT
		t.id, t.kind, t.status, t.source_type, t.source_id, t.version, t.updated_at,
		COALESCE(direct_task.workspace_id, draft_task.workspace_id, review_task.workspace_id, ''),
		COALESCE(direct_task.conversation_id, draft_task.conversation_id, review_task.conversation_id, ''),
		COALESCE(job.id, ''), COALESCE(job.status, ''), COALESCE(job.progress, 0),
		COALESCE(job.progress_message, ''), COALESCE(job.cancellable, FALSE),
		COALESCE(job.cancel_requested_at IS NOT NULL, FALSE),
		COALESCE(job.version, 0), COALESCE(job.attempt_count, 0), COALESCE(job.max_attempts, 1),
		job.locked_at, job.updated_at,
		COALESCE(t.error_code, job.error_code, ''),
		COALESCE(t.error_message, job.error_message, ''), COALESCE(job.retryable, FALSE)
	FROM platform_agent_tasks AS t
	LEFT JOIN platform_jobs AS job
	  ON job.organization_id = t.organization_id AND job.project_id = t.project_id AND job.id = t.job_id
	LEFT JOIN strategy_tasks AS direct_task
	  ON t.source_type = 'strategy_task' AND direct_task.organization_id = t.organization_id
	 AND direct_task.project_id = t.project_id AND direct_task.id = t.source_id
	LEFT JOIN strategy_drafts AS source_draft
	  ON t.source_type = 'strategy_draft' AND source_draft.organization_id = t.organization_id
	 AND source_draft.project_id = t.project_id AND source_draft.id = t.source_id
	LEFT JOIN strategy_tasks AS draft_task
	  ON draft_task.organization_id = source_draft.organization_id
	 AND draft_task.project_id = source_draft.project_id AND draft_task.id = source_draft.task_id
	LEFT JOIN strategy_reviews AS source_review
	  ON t.source_type = 'strategy_review' AND source_review.organization_id = t.organization_id
	 AND source_review.project_id = t.project_id AND source_review.id = t.source_id
	LEFT JOIN strategy_drafts AS review_draft
	  ON review_draft.organization_id = source_review.organization_id
	 AND review_draft.project_id = source_review.project_id AND review_draft.id = source_review.strategy_id
	LEFT JOIN strategy_tasks AS review_task
	  ON review_task.organization_id = review_draft.organization_id
	 AND review_task.project_id = review_draft.project_id AND review_task.id = review_draft.task_id
	WHERE t.organization_id = ? AND t.project_id = ? AND t.source_system = 'strategy'
	  AND t.kind IN (?, ?, ?, ?)
	ORDER BY t.updated_at DESC LIMIT ?`, organizationID, projectID,
		AgentKindBriefExtract, AgentKindDraftGenerate, AgentKindDraftRevise, AgentKindReviewDeep, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]activityRow, 0)
	for rows.Next() {
		var value activityRow
		var taskVersion int64
		var jobID, jobStatus, progressMessage string
		var jobVersion int64
		var heartbeat, jobUpdated sql.NullTime
		if err := rows.Scan(
			&value.ID, &value.AgentKind, &value.DomainStatus, &value.SourceType, &value.SourceID,
			&taskVersion, &value.UpdatedAt, &value.WorkspaceID, &value.ConversationID,
			&jobID, &jobStatus, &value.Progress, &progressMessage, &value.Cancellable,
			&value.CancelRequested,
			&jobVersion, &value.AttemptCount, &value.MaxAttempts, &heartbeat, &jobUpdated,
			&value.FailureCode, &value.FailureMessage, &value.FailureRetryable,
		); err != nil {
			return nil, err
		}
		value.ResourceType, value.ResourceID = activityResourceForAgent(value.SourceType, value.SourceID)
		value.ProjectID = projectID
		value.ResourceVersion = 0
		value.ExecutionType, value.ExecutionID, value.ExecutionVersion = "strategy_agent_task", value.ID, taskVersion
		value.ExecutionStatus = jobStatus
		value.ProgressMessage = strings.TrimSpace(progressMessage)
		if heartbeat.Valid {
			stamp := heartbeat.Time.UTC()
			value.HeartbeatAt = &stamp
		}
		if jobUpdated.Valid && jobUpdated.Time.After(value.UpdatedAt) {
			value.UpdatedAt = jobUpdated.Time.UTC()
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s Service) listKnowledgeActivityRows(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	limit int,
) ([]activityRow, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT
		job.id, job.kind, job.status, job.progress, COALESCE(job.progress_message, ''),
		job.cancellable, job.cancel_requested_at IS NOT NULL,
		job.version, job.attempt_count, job.max_attempts,
		COALESCE(document.heartbeat_at, research.heartbeat_at, job.locked_at), job.updated_at,
		COALESCE(research.id, document.id, ''),
		COALESCE(research.purpose, ''), COALESCE(research.status, document.status, ''),
		COALESCE(document.parse_phase, ''), COALESCE(document.progress_kind, ''),
		document.parse_progress, document.processed_pages, document.total_pages,
		COALESCE(research.error_code, NULLIF(document.vision_error_code, ''), document.parse_error_code, job.error_code, ''),
		COALESCE(research.error_message, NULLIF(document.vision_error_message, ''), document.parse_error_message, job.error_message, ''),
		COALESCE(job.retryable, TRUE),
		COALESCE(conversation.workspace_id, ''), COALESCE(message.conversation_id, ''),
		COALESCE(research.updated_at, document.updated_at, job.updated_at)
	FROM platform_jobs AS job
	LEFT JOIN platform_research_runs AS research
	  ON job.kind = 'knowledge.research.execute' AND research.organization_id = job.organization_id
	 AND research.project_id = job.project_id
	 AND research.id = JSON_UNQUOTE(JSON_EXTRACT(job.payload, '$.research_run_id'))
	LEFT JOIN platform_knowledge_documents AS document
	  ON job.kind IN ('knowledge.document.parse', 'knowledge.document.vision_fallback') AND document.organization_id = job.organization_id
	 AND document.project_id = job.project_id
	 AND document.id = JSON_UNQUOTE(JSON_EXTRACT(job.payload, '$.document_id'))
	LEFT JOIN strategy_messages AS message
	  ON research.source_type = 'strategy_message' AND message.organization_id = research.organization_id
	 AND message.project_id = research.project_id AND message.id = research.source_id
	LEFT JOIN strategy_conversations AS conversation
	  ON conversation.organization_id = message.organization_id AND conversation.project_id = message.project_id
	 AND conversation.id = message.conversation_id
	WHERE job.organization_id = ? AND job.project_id = ?
	  AND job.kind IN ('knowledge.research.execute', 'knowledge.document.parse', 'knowledge.document.vision_fallback')
	ORDER BY job.created_at DESC LIMIT ?`, organizationID, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]activityRow, 0)
	seen := map[string]struct{}{}
	for rows.Next() {
		var value activityRow
		var jobKind, resourceID string
		var heartbeat sql.NullTime
		var documentProgress, documentProcessedPages, documentTotalPages sql.NullInt64
		var domainUpdated time.Time
		if err := rows.Scan(
			&value.ExecutionID, &jobKind, &value.ExecutionStatus, &value.Progress,
			&value.ProgressMessage, &value.Cancellable, &value.CancelRequested, &value.ExecutionVersion,
			&value.AttemptCount, &value.MaxAttempts, &heartbeat, &value.UpdatedAt,
			&resourceID, &value.DomainPurpose, &value.DomainStatus,
			&value.DocumentPhase, &value.DocumentProgressKind,
			&documentProgress, &documentProcessedPages, &documentTotalPages,
			&value.FailureCode, &value.FailureMessage, &value.FailureRetryable,
			&value.WorkspaceID, &value.ConversationID, &domainUpdated,
		); err != nil {
			return nil, err
		}
		if resourceID == "" {
			continue
		}
		value.DomainKind = jobKind
		if documentProgress.Valid {
			progress := int(documentProgress.Int64)
			value.DocumentProgress = &progress
		}
		if documentProcessedPages.Valid {
			processed := int(documentProcessedPages.Int64)
			value.DocumentProcessedPages = &processed
		}
		if documentTotalPages.Valid {
			total := int(documentTotalPages.Int64)
			value.DocumentTotalPages = &total
		}
		value.ProjectID = projectID
		value.ExecutionType = "platform_job"
		if jobKind == "knowledge.research.execute" {
			value.ResourceType = "knowledge_research_run"
		} else {
			value.ResourceType = "knowledge_document"
		}
		value.ID, value.ResourceID = resourceID, resourceID
		key := value.ResourceType + ":" + resourceID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		if heartbeat.Valid {
			stamp := heartbeat.Time.UTC()
			value.HeartbeatAt = &stamp
		}
		if domainUpdated.After(value.UpdatedAt) {
			value.UpdatedAt = domainUpdated.UTC()
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func taskActivitySnapshot(rows []activityRow, now time.Time, workspaceID string, limit int) (TaskActivitySnapshot, error) {
	items := make([]TaskActivity, 0, len(rows))
	for _, row := range rows {
		if workspaceID != "" && row.WorkspaceID != "" && row.WorkspaceID != workspaceID {
			continue
		}
		items = append(items, normalizeTaskActivity(row, now.UTC()))
	}
	sort.SliceStable(items, func(left, right int) bool {
		leftActive := activityIsActive(items[left].Status)
		rightActive := activityIsActive(items[right].Status)
		if leftActive != rightActive {
			return leftActive
		}
		return items[left].UpdatedAt.After(items[right].UpdatedAt)
	})
	if len(items) > limit {
		items = items[:limit]
	}
	hash, err := contract.CanonicalJSONHash(items)
	if err != nil {
		return TaskActivitySnapshot{}, err
	}
	return TaskActivitySnapshot{
		ContractVersion: TaskActivitySnapshotContractVersion,
		SnapshotID:      "sha256:" + hash,
		CapturedAt:      now.UTC(),
		Items:           items,
	}, nil
}

func normalizeTaskActivity(row activityRow, now time.Time) TaskActivity {
	kind := activityKind(row)
	status := activityStatus(row)
	stalledMessage := ""
	// Only active execution owns a heartbeat. A queued task can legitimately
	// wait longer than the worker lease and must not be presented as an
	// interrupted execution merely because its ordinary updated_at is old.
	if status == "running" {
		lastSignal := row.UpdatedAt
		if row.HeartbeatAt != nil {
			lastSignal = *row.HeartbeatAt
		}
		if lastSignal.IsZero() || now.Sub(lastSignal) > ActivityStallAfter {
			status = "stalled"
			if row.HeartbeatAt == nil {
				stalledMessage = "运行中的任务长时间没有获得执行心跳，Worker 可能已经中断。"
			} else {
				stalledMessage = "超过 60 秒未收到 Worker 心跳，任务可能已经中断。"
			}
		}
	}
	phase := activityPhase(kind, status, row.Progress, row.AgentKind)
	if kind == "document_parse" && row.DocumentPhase != "" {
		phase = row.DocumentPhase
	}
	summary := activitySummary(kind, status, row.ProgressMessage)
	if row.CancelRequested && activityIsActive(status) {
		phase = "cancelling"
		summary = "取消请求已发送；正在等待当前调用安全结束。"
	}
	if stalledMessage != "" {
		if row.CancelRequested {
			summary = "取消请求已发送；执行心跳已超时，系统正在恢复任务状态。"
		} else {
			summary = stalledMessage
		}
	}
	progressKind := "milestone"
	var progressValue *int
	if kind == "document_parse" && row.DocumentProgress != nil {
		value := *row.DocumentProgress
		progressValue = &value
		if row.DocumentProgressKind != "" {
			progressKind = row.DocumentProgressKind
		}
		if progressKind == "pages" && row.DocumentProcessedPages != nil && row.DocumentTotalPages != nil {
			summary = fmt.Sprintf("%s（%d / %d 页）", summary, *row.DocumentProcessedPages, *row.DocumentTotalPages)
		}
	} else if row.ExecutionID != "" {
		value := row.Progress
		if status == "succeeded" {
			value = 100
		}
		progressValue = &value
	} else {
		progressKind = "indeterminate"
	}
	resource := contract.ResourceRef{Type: row.ResourceType, ID: row.ResourceID}
	if row.ResourceVersion > 0 {
		version := row.ResourceVersion
		resource.Version = &version
	}
	var execution *contract.ResourceRef
	if row.ExecutionID != "" {
		execution = &contract.ResourceRef{Type: row.ExecutionType, ID: row.ExecutionID}
		if row.ExecutionVersion > 0 {
			version := row.ExecutionVersion
			execution.Version = &version
		}
	}
	var failure *ActivityFailure
	if status == "failed" && row.FailureCode != "" {
		failure = &ActivityFailure{Code: row.FailureCode, Message: trimActivityText(row.FailureMessage, 500), Retryable: row.FailureRetryable}
	}
	if status == "stalled" {
		failure = &ActivityFailure{Code: "ACTIVITY_HEARTBEAT_STALE", Message: stalledMessage, Retryable: false}
	}
	return TaskActivity{
		ContractVersion: TaskActivityContractVersion,
		ID:              row.ID, Kind: kind, Status: status, Phase: phase, Round: nil,
		Progress: ActivityProgress{Kind: progressKind, Value: progressValue, Message: trimActivityText(summary, 240)},
		Summary:  summary, ConfirmedConclusions: []ActivityConclusion{},
		SourceScope: ActivitySourceScope{ProjectID: row.ProjectID, WorkspaceID: row.WorkspaceID, ConversationID: row.ConversationID},
		ResourceRef: resource, ExecutionRef: execution,
		Actions: activityActions(row, status), CancelRequested: row.CancelRequested, Failure: failure,
		HeartbeatAt: row.HeartbeatAt, UpdatedAt: row.UpdatedAt.UTC(),
	}
}

func activityResourceForAgent(sourceType, sourceID string) (string, string) {
	switch sourceType {
	case "strategy_draft":
		return "strategy_draft", sourceID
	case "strategy_review":
		return "strategy_review", sourceID
	default:
		return "strategy_task", sourceID
	}
}

func activityKind(row activityRow) string {
	switch row.AgentKind {
	case AgentKindBriefExtract:
		return "assistant"
	case AgentKindDraftGenerate, AgentKindDraftRevise:
		return "strategy_generation"
	case AgentKindReviewDeep:
		return "deep_review"
	}
	if row.DomainKind == "knowledge.document.parse" || row.DomainKind == "knowledge.document.vision_fallback" {
		return "document_parse"
	}
	if row.DomainPurpose == "conversation_web_search" {
		return "quick_research"
	}
	return "deep_research"
}

func activityStatus(row activityRow) string {
	domain := strings.TrimSpace(row.DomainStatus)
	if row.DomainKind == "knowledge.document.parse" || row.DomainKind == "knowledge.document.vision_fallback" {
		switch domain {
		case "ready":
			return "succeeded"
		case "partial":
			return "partially_completed"
		case "parse_failed":
			if row.FailureCode == "DOCUMENT_PARSE_CANCELLED" {
				return "cancelled"
			}
			return "failed"
		}
	}
	if row.DomainKind == "knowledge.research.execute" {
		switch domain {
		case "succeeded", "completed":
			return "succeeded"
		case "partially_completed":
			return "partially_completed"
		case "failed", "unavailable":
			return "failed"
		case "cancelled":
			return "cancelled"
		}
	}
	if row.AgentKind != "" {
		switch domain {
		case "succeeded", "failed", "cancelled":
			return domain
		}
	}
	switch strings.TrimSpace(row.ExecutionStatus) {
	case "running":
		return "running"
	case "succeeded":
		return "succeeded"
	case "failed":
		return "failed"
	case "cancelled":
		return "cancelled"
	default:
		return "queued"
	}
}

func activityPhase(kind, status string, progress int, agentKind string) string {
	switch status {
	case "succeeded", "partially_completed":
		return "completed"
	case "failed":
		return "failed"
	case "cancelled":
		return "cancelled"
	}
	if status == "queued" {
		return "queued"
	}
	switch kind {
	case "document_parse":
		if progress < 20 {
			return "scanning"
		}
		if progress < 80 {
			return "extracting"
		}
		return "chunking"
	case "quick_research", "deep_research":
		if progress < 20 {
			return "planning"
		}
		if progress < 35 {
			return "reading"
		}
		if progress < 90 {
			return "searching"
		}
		return "saving"
	case "strategy_generation":
		if progress >= 90 {
			return "saving"
		}
		if agentKind == AgentKindDraftRevise {
			return "revising"
		}
		return "drafting"
	case "deep_review":
		if progress >= 90 {
			return "saving"
		}
		return "reviewing"
	default:
		if progress >= 90 {
			return "saving"
		}
		return "understanding"
	}
}

func activitySummary(kind, status, progressMessage string) string {
	if value := strings.TrimSpace(progressMessage); value != "" {
		return trimActivityText(value, 500)
	}
	label := map[string]string{
		"assistant":           "AI 正在理解需求并更新 Brief",
		"quick_research":      "正在进行联网搜索",
		"deep_research":       "正在执行研究任务",
		"document_parse":      "正在解析文档并生成可引用内容",
		"brief_generation":    "正在补全 Brief",
		"strategy_generation": "正在生成或修订策略",
		"deep_review":         "AI 正在提供第二视角",
	}[kind]
	switch status {
	case "queued":
		return label + "，等待执行资源。"
	case "succeeded":
		return label + "，已完成。"
	case "partially_completed":
		return label + "，已保留部分可用结果。"
	case "failed":
		return label + "，当前阶段未完成。"
	case "cancelled":
		return label + "，已取消。"
	default:
		return label + "。"
	}
}

func activityActions(row activityRow, status string) []string {
	actions := []string{"open"}
	if row.DomainPurpose == "conversation_web_search" {
		return actions
	}
	if (activityIsActive(status) || status == "stalled") && row.Cancellable && !row.CancelRequested {
		return append(actions, "cancel")
	}
	if status == "failed" || status == "cancelled" || status == "partially_completed" {
		if row.ResourceType == "knowledge_document" || row.ResourceType == "knowledge_research_run" ||
			(row.ResourceType == "strategy_draft" && row.AgentKind == AgentKindDraftGenerate) {
			return append(actions, "retry")
		}
	}
	return actions
}

func activityIsActive(status string) bool {
	return status == "queued" || status == "running" || status == "waiting_user" || status == "stalled"
}

func trimActivityText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
