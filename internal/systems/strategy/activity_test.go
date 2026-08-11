package strategy

import (
	"reflect"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestTaskActivityMarksStaleHeartbeatWithoutMutatingExecutionFact(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	heartbeat := now.Add(-ActivityStallAfter - time.Second)
	row := activityRow{
		ID: "agent_1", ProjectID: "project_1", AgentKind: AgentKindDraftGenerate,
		DomainStatus: "running", ExecutionStatus: "running",
		ResourceType: "strategy_draft", ResourceID: "strategy_1",
		ExecutionType: "strategy_agent_task", ExecutionID: "agent_1", ExecutionVersion: 4,
		Progress: 40, ProgressMessage: "正在生成策略骨架", Cancellable: true,
		HeartbeatAt: &heartbeat, UpdatedAt: heartbeat,
	}

	activity := normalizeTaskActivity(row, now)
	if activity.Status != "stalled" || activity.Failure == nil || activity.Failure.Code != "ACTIVITY_HEARTBEAT_STALE" {
		t.Fatalf("activity=%#v", activity)
	}
	if !reflect.DeepEqual(activity.Actions, []string{"open", "cancel"}) {
		t.Fatalf("actions=%v", activity.Actions)
	}
	if activity.Progress.Value == nil || *activity.Progress.Value != 40 || activity.Phase != "drafting" {
		t.Fatalf("progress=%#v phase=%q", activity.Progress, activity.Phase)
	}
}

func TestTaskActivityDoesNotTreatAnOldQueuedTaskAsAStaleExecution(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	activity := normalizeTaskActivity(activityRow{
		ID: "research_queued", ProjectID: "project_1", DomainKind: "knowledge.research.execute",
		DomainStatus: "queued", ExecutionStatus: "queued",
		ResourceType: "knowledge_research_run", ResourceID: "research_queued",
		ExecutionType: "platform_job", ExecutionID: "job_queued", ExecutionVersion: 1,
		Cancellable: true, UpdatedAt: now.Add(-10 * time.Minute),
	}, now)

	if activity.Status != "queued" || activity.Failure != nil || activity.Phase != "queued" {
		t.Fatalf("activity=%#v", activity)
	}
	if !reflect.DeepEqual(activity.Actions, []string{"open", "cancel"}) {
		t.Fatalf("actions=%v", activity.Actions)
	}
}

func TestTaskActivityUsesDomainFailureOverSuccessfulJob(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	row := activityRow{
		ID: "document_1", ProjectID: "project_1", DomainKind: "knowledge.document.parse",
		DomainStatus: "parse_failed", ExecutionStatus: "succeeded",
		ResourceType: "knowledge_document", ResourceID: "document_1",
		ExecutionType: "platform_job", ExecutionID: "job_1", ExecutionVersion: 3,
		Progress: 70, Cancellable: true, UpdatedAt: now,
		FailureCode: "DOCUMENT_PARSE_EMPTY", FailureMessage: "Parser returned no chunks", FailureRetryable: true,
	}

	activity := normalizeTaskActivity(row, now)
	if activity.Status != "failed" || activity.Progress.Value == nil || *activity.Progress.Value != 70 {
		t.Fatalf("activity=%#v", activity)
	}
	if activity.Failure == nil || activity.Failure.Code != "DOCUMENT_PARSE_EMPTY" || !activity.Failure.Retryable {
		t.Fatalf("failure=%#v", activity.Failure)
	}
	if !reflect.DeepEqual(activity.Actions, []string{"open", "retry"}) {
		t.Fatalf("actions=%v", activity.Actions)
	}
}

func TestTaskActivityReportsCancelRequestedWithoutClaimingTermination(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	row := activityRow{
		ID: "research_1", ProjectID: "project_1", DomainKind: "knowledge.research.execute",
		DomainStatus: "running", ExecutionStatus: "running",
		ResourceType: "knowledge_research_run", ResourceID: "research_1",
		ExecutionType: "platform_job", ExecutionID: "job_1", ExecutionVersion: 5,
		Progress: 35, Cancellable: true, CancelRequested: true,
		HeartbeatAt: timePointer(now), UpdatedAt: now,
	}

	activity := normalizeTaskActivity(row, now)
	if activity.Status != "running" || activity.Phase != "cancelling" || !activity.CancelRequested {
		t.Fatalf("activity=%#v", activity)
	}
	if !reflect.DeepEqual(activity.Actions, []string{"open"}) {
		t.Fatalf("actions=%v", activity.Actions)
	}
}

func TestTaskActivityMapsCancelledDocumentToCancelled(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	activity := normalizeTaskActivity(activityRow{
		ID: "document_1", ProjectID: "project_1", DomainKind: "knowledge.document.parse",
		DomainStatus: "parse_failed", ExecutionStatus: "cancelled",
		ResourceType: "knowledge_document", ResourceID: "document_1",
		ExecutionType: "platform_job", ExecutionID: "job_1", ExecutionVersion: 3,
		FailureCode: "DOCUMENT_PARSE_CANCELLED", UpdatedAt: now,
	}, now)
	if activity.Status != "cancelled" || activity.Failure != nil ||
		!reflect.DeepEqual(activity.Actions, []string{"open", "retry"}) {
		t.Fatalf("activity=%#v", activity)
	}
}

func TestTaskActivityUsesPersistedDocumentPartialAndPageProgress(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	progress, processed, total := 100, 8, 8
	activity := normalizeTaskActivity(activityRow{
		ID: "document_1", ProjectID: "project_1", DomainKind: "knowledge.document.parse",
		DomainStatus: "partial", ExecutionStatus: "succeeded",
		ResourceType: "knowledge_document", ResourceID: "document_1",
		ExecutionType: "platform_job", ExecutionID: "job_1", ExecutionVersion: 3,
		DocumentPhase: "partial", DocumentProgressKind: "pages", DocumentProgress: &progress,
		DocumentProcessedPages: &processed, DocumentTotalPages: &total, UpdatedAt: now,
	}, now)
	if activity.Status != "partially_completed" || activity.Phase != "partial" ||
		activity.Progress.Kind != "pages" || activity.Progress.Value == nil || *activity.Progress.Value != 100 ||
		!reflect.DeepEqual(activity.Actions, []string{"open", "retry"}) {
		t.Fatalf("activity=%#v", activity)
	}
}

func TestTaskActivitySnapshotIsWorkspaceScopedAndContentAddressed(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC)
	rows := []activityRow{
		{
			ID: "agent_other", ProjectID: "project_1", AgentKind: AgentKindBriefExtract,
			DomainStatus: "succeeded", ResourceType: "strategy_task", ResourceID: "task_other",
			ExecutionType: "strategy_agent_task", ExecutionID: "agent_other", ExecutionVersion: 2,
			WorkspaceID: "workspace_other", UpdatedAt: now.Add(-time.Minute),
		},
		{
			ID: "research_project", ProjectID: "project_1", DomainKind: "knowledge.research.execute",
			DomainPurpose: "deep_research", DomainStatus: "running", ExecutionStatus: "running",
			ResourceType: "knowledge_research_run", ResourceID: "research_project",
			ExecutionType: "platform_job", ExecutionID: "job_research", ExecutionVersion: 2,
			Progress: 35, UpdatedAt: now, HeartbeatAt: timePointer(now),
		},
		{
			ID: "agent_current", ProjectID: "project_1", AgentKind: AgentKindDraftGenerate,
			DomainStatus: "running", ExecutionStatus: "running",
			ResourceType: "strategy_draft", ResourceID: "strategy_current",
			ExecutionType: "strategy_agent_task", ExecutionID: "agent_current", ExecutionVersion: 2,
			WorkspaceID: "workspace_current", ConversationID: "conversation_current",
			Progress: 20, UpdatedAt: now, HeartbeatAt: timePointer(now),
		},
	}

	first, err := taskActivitySnapshot(rows, now, "workspace_current", 10)
	if err != nil {
		t.Fatal(err)
	}
	second, err := taskActivitySnapshot(rows, now.Add(time.Second), "workspace_current", 10)
	if err != nil {
		t.Fatal(err)
	}
	if first.SnapshotID != second.SnapshotID || first.SnapshotID[:7] != "sha256:" {
		t.Fatalf("snapshot IDs first=%q second=%q", first.SnapshotID, second.SnapshotID)
	}
	if len(first.Items) != 2 || first.Items[0].SourceScope.ProjectID != contract.ProjectID("project_1") {
		t.Fatalf("items=%#v", first.Items)
	}
	for _, item := range first.Items {
		if item.SourceScope.WorkspaceID == "workspace_other" {
			t.Fatalf("cross-workspace activity leaked: %#v", item)
		}
	}
}

func timePointer(value time.Time) *time.Time { return &value }
