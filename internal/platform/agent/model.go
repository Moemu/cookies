// Package agent owns durable, domain-neutral agent tasks and version-pinned
// skill runs. Domain modules own the input and result semantics.
package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type TaskStatus string

const (
	TaskDispatchPending TaskStatus = "dispatch_pending"
	TaskQueued          TaskStatus = "queued"
	TaskRunning         TaskStatus = "running"
	TaskSucceeded       TaskStatus = "succeeded"
	TaskFailed          TaskStatus = "failed"
	TaskCancelled       TaskStatus = "cancelled"
)

type Task struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	SourceSystem   string                  `json:"source_system"`
	SourceType     string                  `json:"source_type"`
	SourceID       string                  `json:"source_id"`
	Kind           string                  `json:"kind"`
	Status         TaskStatus              `json:"status"`
	JobID          string                  `json:"job_id,omitempty"`
	Version        int64                   `json:"version"`
	InputSnapshot  json.RawMessage         `json:"-"`
	ResultRef      *contract.ResourceRef   `json:"result_ref,omitempty"`
	Error          *contract.JobError      `json:"error,omitempty"`
	CreatedBy      contract.Principal      `json:"created_by"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

func (t Task) Validate() error {
	if strings.TrimSpace(t.ID) == "" || t.OrganizationID == "" || t.ProjectID == "" ||
		strings.TrimSpace(t.SourceSystem) == "" || strings.TrimSpace(t.SourceType) == "" ||
		strings.TrimSpace(t.SourceID) == "" || strings.TrimSpace(t.Kind) == "" {
		return fmt.Errorf("agent task identity, source, and kind are required")
	}
	if !json.Valid(t.InputSnapshot) {
		return fmt.Errorf("agent task input snapshot must be valid JSON")
	}
	if t.Version < 1 || t.CreatedAt.IsZero() || t.UpdatedAt.IsZero() {
		return fmt.Errorf("agent task version and timestamps are required")
	}
	return nil
}

type CreateRequest struct {
	Task Task
}

const (
	ScopeRunRead  contract.Scope = "agent.read"
	ScopeRunWrite contract.Scope = "agent.write"

	WorkflowRenderDiagnosis Workflow = "render_diagnosis"

	RunQueued    RunStatus = "queued"
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
	RunCancelled RunStatus = "cancelled"

	ToolPending   ToolCallStatus = "pending"
	ToolRunning   ToolCallStatus = "running"
	ToolSucceeded ToolCallStatus = "succeeded"
	ToolFailed    ToolCallStatus = "failed"

	SpanRunning   SpanStatus = "running"
	SpanSucceeded SpanStatus = "succeeded"
	SpanFailed    SpanStatus = "failed"
	SpanCancelled SpanStatus = "cancelled"

	SpanKindAgent SpanKind = "agent"
	SpanKindTool  SpanKind = "tool"
	SpanKindModel SpanKind = "model"
)

type Workflow string
type RunStatus string
type ToolCallStatus string
type SpanStatus string
type SpanKind string

type CreateRunRequest struct {
	Workflow Workflow        `json:"workflow"`
	Target   DiagnosisTarget `json:"target"`
}

type DiagnosisTarget struct {
	RenderJobID string `json:"render_job_id"`
}

type AgentRun struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Workflow       Workflow                `json:"workflow"`
	Status         RunStatus               `json:"status"`
	Target         DiagnosisTarget         `json:"target"`
	Steps          []AgentStep             `json:"steps"`
	ToolCalls      []ToolCall              `json:"tool_calls"`
	TraceSpans     []TraceSpan             `json:"trace_spans"`
	Output         map[string]any          `json:"output,omitempty"`
	ErrorMessage   string                  `json:"error_message,omitempty"`
	CreatedBy      contract.Principal      `json:"created_by"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
	CompletedAt    *time.Time              `json:"completed_at,omitempty"`
}

type AgentStep struct {
	ID        string     `json:"id"`
	Label     string     `json:"label"`
	Status    RunStatus  `json:"status"`
	Summary   string     `json:"summary"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

type ToolCall struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Status       ToolCallStatus         `json:"status"`
	Input        map[string]any         `json:"input"`
	Output       map[string]any         `json:"output,omitempty"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	References   []contract.ResourceRef `json:"references,omitempty"`
	StartedAt    time.Time              `json:"started_at"`
	EndedAt      *time.Time             `json:"ended_at,omitempty"`
}

type TraceSpan struct {
	ID           string     `json:"id"`
	ParentID     string     `json:"parent_id,omitempty"`
	Name         string     `json:"name"`
	Kind         SpanKind   `json:"kind"`
	Status       SpanStatus `json:"status"`
	Model        string     `json:"model,omitempty"`
	InputTokens  int        `json:"input_tokens,omitempty"`
	OutputTokens int        `json:"output_tokens,omitempty"`
	ErrorMessage string     `json:"error_message,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at,omitempty"`
}

func (r CreateRunRequest) Validate() error {
	if r.Workflow != WorkflowRenderDiagnosis {
		return fmt.Errorf("workflow must be render_diagnosis")
	}
	if strings.TrimSpace(r.Target.RenderJobID) == "" || len(r.Target.RenderJobID) > 160 {
		return fmt.Errorf("target.render_job_id must be between 1 and 160 characters")
	}
	return nil
}

func (r AgentRun) Validate() error {
	if strings.TrimSpace(r.ID) == "" || strings.TrimSpace(string(r.OrganizationID)) == "" || strings.TrimSpace(string(r.ProjectID)) == "" {
		return fmt.Errorf("agent run identity is incomplete")
	}
	if r.Workflow != WorkflowRenderDiagnosis {
		return fmt.Errorf("agent run workflow is invalid")
	}
	if !validRunStatus(r.Status) {
		return fmt.Errorf("agent run status is invalid")
	}
	if err := (CreateRunRequest{Workflow: r.Workflow, Target: r.Target}).Validate(); err != nil {
		return err
	}
	spanIDs := make(map[string]struct{}, len(r.TraceSpans))
	for _, span := range r.TraceSpans {
		if strings.TrimSpace(span.ID) == "" {
			return fmt.Errorf("trace span id is required")
		}
		if _, exists := spanIDs[span.ID]; exists {
			return fmt.Errorf("trace span %q is duplicated", span.ID)
		}
		spanIDs[span.ID] = struct{}{}
	}
	for _, span := range r.TraceSpans {
		if span.ParentID != "" {
			if _, exists := spanIDs[span.ParentID]; !exists {
				return fmt.Errorf("trace span %q references missing parent %q", span.ID, span.ParentID)
			}
		}
		if !validSpanStatus(span.Status) || !validSpanKind(span.Kind) {
			return fmt.Errorf("trace span %q is invalid", span.ID)
		}
	}
	return nil
}

func validRunStatus(status RunStatus) bool {
	return status == RunQueued || status == RunRunning || status == RunSucceeded || status == RunFailed || status == RunCancelled
}

func validSpanStatus(status SpanStatus) bool {
	return status == SpanRunning || status == SpanSucceeded || status == SpanFailed || status == SpanCancelled
}

func validSpanKind(kind SpanKind) bool {
	return kind == SpanKindAgent || kind == SpanKindTool || kind == SpanKindModel
}
