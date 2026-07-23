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
