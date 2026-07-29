package project

import (
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type Status string

const (
	StatusDraft    Status = "draft"
	StatusActive   Status = "active"
	StatusArchived Status = "archived"
)

type Project struct {
	ID                      contract.ProjectID      `json:"id"`
	OrganizationID          contract.OrganizationID `json:"organization_id"`
	Name                    string                  `json:"name"`
	Status                  Status                  `json:"status"`
	PrimaryBrandID          *contract.BrandID       `json:"primary_brand_id"`
	PrimaryBrandStatus      string                  `json:"-"`
	BrandGuidelineVersionID string                  `json:"brand_guideline_version_id,omitempty"`
	ProjectContextVersion   int64                   `json:"project_context_version"`
	CreatedAt               time.Time               `json:"created_at"`
	UpdatedAt               time.Time               `json:"updated_at"`
}

type ProjectDetail struct {
	Project    Project                  `json:"project"`
	Runtime    ProjectRuntime           `json:"runtime"`
	Artifacts  []ProjectArtifactSummary `json:"artifacts"`
	Tasks      []BusinessTask           `json:"tasks"`
	Operations []OperationalRecord      `json:"operations"`
	ChangeSets []ChangeSet              `json:"change_sets"`
}

type ProjectRuntime struct {
	Code           string    `json:"code"`
	Brand          string    `json:"brand,omitempty"`
	Product        string    `json:"product,omitempty"`
	Goal           string    `json:"goal,omitempty"`
	Stage          string    `json:"stage"`
	Progress       int       `json:"progress"`
	Status         string    `json:"status"`
	Owner          string    `json:"owner"`
	Budget         float64   `json:"budget"`
	Currency       string    `json:"currency"`
	Timezone       string    `json:"timezone"`
	KnowledgeCount int       `json:"knowledge_count,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ProjectArtifactSummary struct {
	ID            string                    `json:"id,omitempty"`
	Key           string                    `json:"key"`
	Label         string                    `json:"label"`
	Version       string                    `json:"version"`
	Status        string                    `json:"status"`
	Owner         string                    `json:"owner"`
	UpdatedAt     time.Time                 `json:"updated_at"`
	Summary       string                    `json:"summary"`
	SourceVersion string                    `json:"source_version,omitempty"`
	AssetRef      *contract.ProjectAssetRef `json:"asset_ref,omitempty"`
}

type Brand struct {
	ID             contract.BrandID        `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	Name           string                  `json:"name"`
	Status         string                  `json:"status"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type CreateProjectRequest struct {
	Name           string               `json:"name"`
	PrimaryBrandID *contract.BrandID    `json:"primary_brand_id"`
	ProductIDs     []contract.ProductID `json:"product_ids"`
	Activate       bool                 `json:"activate"`
}

func (r CreateProjectRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" || len(r.Name) > 255 {
		return fmt.Errorf("project name must be between 1 and 255 characters")
	}
	if r.PrimaryBrandID != nil && strings.TrimSpace(string(*r.PrimaryBrandID)) == "" {
		return fmt.Errorf("primary_brand_id must be null or non-empty")
	}
	if r.Activate && r.PrimaryBrandID == nil {
		return fmt.Errorf("active project requires primary_brand_id")
	}
	if r.ProductIDs == nil {
		return fmt.Errorf("product_ids must be an array")
	}
	seen := make(map[contract.ProductID]struct{}, len(r.ProductIDs))
	for _, productID := range r.ProductIDs {
		if strings.TrimSpace(string(productID)) == "" {
			return fmt.Errorf("product_id must not be empty")
		}
		if _, exists := seen[productID]; exists {
			return fmt.Errorf("product_id %q is duplicated", productID)
		}
		seen[productID] = struct{}{}
	}
	return nil
}

type BusinessTaskType string

const (
	BusinessTaskStrategy          BusinessTaskType = "strategy"
	BusinessTaskCreative          BusinessTaskType = "creative"
	BusinessTaskVideo             BusinessTaskType = "video"
	BusinessTaskBrandVideo        BusinessTaskType = "brand_video"
	BusinessTaskShortDramaPreroll BusinessTaskType = "short_drama_preroll"
	BusinessTaskGamePreroll       BusinessTaskType = "game_preroll"
	BusinessTaskCommercePreroll   BusinessTaskType = "commerce_preroll"
	BusinessTaskViralRemake       BusinessTaskType = "viral_remake"
	BusinessTaskVideoEdit         BusinessTaskType = "video_edit"
)

type BusinessTaskStatus string

const (
	BusinessTaskDraft      BusinessTaskStatus = "draft"
	BusinessTaskInProgress BusinessTaskStatus = "in_progress"
	BusinessTaskReady      BusinessTaskStatus = "ready"
	BusinessTaskCompleted  BusinessTaskStatus = "completed"
	BusinessTaskFailed     BusinessTaskStatus = "failed"
)

type BusinessTask struct {
	ID                string                  `json:"id"`
	OrganizationID    contract.OrganizationID `json:"-"`
	ProjectID         contract.ProjectID      `json:"project_id"`
	Type              BusinessTaskType        `json:"type"`
	Name              string                  `json:"name"`
	Objective         string                  `json:"objective"`
	Status            BusinessTaskStatus      `json:"status"`
	SourceTaskIDs     []string                `json:"source_task_ids"`
	SourceArtifactIDs []string                `json:"source_artifact_ids"`
	OutputArtifactIDs []string                `json:"output_artifact_ids"`
	Version           int64                   `json:"version"`
	CreatedAt         time.Time               `json:"created_at"`
	UpdatedAt         time.Time               `json:"updated_at"`
}

type CreateBusinessTaskRequest struct {
	Type              BusinessTaskType `json:"type"`
	Name              string           `json:"name"`
	Objective         string           `json:"objective"`
	SourceTaskIDs     []string         `json:"source_task_ids"`
	SourceArtifactIDs []string         `json:"source_artifact_ids"`
}

type UpdateBusinessTaskRequest struct {
	Name              *string             `json:"name"`
	Objective         *string             `json:"objective"`
	Status            *BusinessTaskStatus `json:"status"`
	SourceTaskIDs     []string            `json:"source_task_ids"`
	SourceArtifactIDs []string            `json:"source_artifact_ids"`
	OutputArtifactIDs []string            `json:"output_artifact_ids"`
	ExpectedVersion   *int64              `json:"expected_version"`
}

type OperationalRecordKind string

const (
	OperationalRecordWorkItem           OperationalRecordKind = "work_item"
	OperationalRecordEvidence           OperationalRecordKind = "evidence"
	OperationalRecordActivity           OperationalRecordKind = "activity"
	OperationalRecordMetric             OperationalRecordKind = "metric"
	OperationalRecordPerformanceAd      OperationalRecordKind = "performance_ad"
	OperationalRecordAudienceMix        OperationalRecordKind = "audience_mix"
	OperationalRecordMethod             OperationalRecordKind = "method"
	OperationalRecordDeliveryDiagnostic OperationalRecordKind = "delivery_diagnostic"
	OperationalRecordDeliveryAction     OperationalRecordKind = "delivery_action"
	OperationalRecordUnifiedRecord      OperationalRecordKind = "unified_record"
)

type OperationalRecord struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"-"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Kind           OperationalRecordKind   `json:"kind"`
	Title          string                  `json:"title"`
	Status         string                  `json:"status"`
	OccurredAt     time.Time               `json:"occurred_at"`
	Fields         map[string]any          `json:"fields"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type UpsertOperationalRecordRequest struct {
	Kind       OperationalRecordKind `json:"kind"`
	Title      string                `json:"title"`
	Status     string                `json:"status"`
	OccurredAt time.Time             `json:"occurred_at"`
	Fields     map[string]any        `json:"fields"`
}

type ChangeSetStatus string

const (
	ChangeSetDraft           ChangeSetStatus = "draft"
	ChangeSetPreflightPassed ChangeSetStatus = "preflight_passed"
	ChangeSetPreflightFailed ChangeSetStatus = "preflight_failed"
	ChangeSetApproved        ChangeSetStatus = "approved"
	ChangeSetRejected        ChangeSetStatus = "rejected"
	ChangeSetExecuting       ChangeSetStatus = "executing"
	ChangeSetExecuted        ChangeSetStatus = "executed"
	ChangeSetRolledBack      ChangeSetStatus = "rolled_back"
)

type ChangeSet struct {
	ID             string                     `json:"id"`
	OrganizationID contract.OrganizationID    `json:"-"`
	ProjectID      contract.ProjectID         `json:"project_id"`
	Name           string                     `json:"name"`
	Status         ChangeSetStatus            `json:"status"`
	ArtifactRefs   []contract.ProjectAssetRef `json:"artifact_refs"`
	BudgetLimit    *float64                   `json:"budget_limit,omitempty"`
	Preflight      *ChangeSetPreflight        `json:"preflight"`
	Execution      *ChangeSetExecution        `json:"execution"`
	Rollback       *ChangeSetRollback         `json:"rollback"`
	AuditEvents    []AuditEvent               `json:"audit_events"`
	Version        int64                      `json:"version"`
	CreatedAt      time.Time                  `json:"created_at"`
	UpdatedAt      time.Time                  `json:"updated_at"`
}

type CreateChangeSetRequest struct {
	Name         string                     `json:"name"`
	ArtifactRefs []contract.ProjectAssetRef `json:"artifact_refs"`
	BudgetLimit  *float64                   `json:"budget_limit"`
}

type ChangeSetApprovalRequest struct {
	Actor string `json:"actor"`
	Role  string `json:"role"`
	Note  string `json:"note"`
}

type RollbackChangeSetRequest struct {
	Actor  string `json:"actor"`
	Reason string `json:"reason"`
}

type ChangeSetPreflight struct {
	Passed    bool             `json:"passed"`
	Checks    []PreflightCheck `json:"checks"`
	CheckedAt time.Time        `json:"checked_at"`
}

type PreflightCheck struct {
	Code    string `json:"code"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
	Repair  string `json:"repair"`
}

type ChangeSetExecution struct {
	Simulated  bool                `json:"simulated"`
	Evidence   []ChangeSetEvidence `json:"evidence"`
	ExecutedAt time.Time           `json:"executed_at"`
}

type ChangeSetRollback struct {
	Simulated    bool      `json:"simulated"`
	Reason       string    `json:"reason"`
	RolledBackAt time.Time `json:"rolled_back_at"`
}

type ChangeSetEvidence struct {
	Step       string    `json:"step"`
	Status     string    `json:"status"`
	Message    string    `json:"message"`
	RecordedAt time.Time `json:"recorded_at"`
}

type ChangeSetEvent struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	ChangeSetID    string                  `json:"change_set_id"`
	EventType      string                  `json:"event_type"`
	Actor          string                  `json:"actor"`
	Payload        map[string]any          `json:"payload"`
	CreatedAt      time.Time               `json:"created_at"`
}

type AuditEntityType string

const (
	AuditEntityProject           AuditEntityType = "project"
	AuditEntityBusinessTask      AuditEntityType = "business_task"
	AuditEntityArtifact          AuditEntityType = "artifact"
	AuditEntityGenerationJob     AuditEntityType = "generation_job"
	AuditEntityChangeSet         AuditEntityType = "change_set"
	AuditEntityOperationalRecord AuditEntityType = "operational_record"
)

type AuditEvent struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"-"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Actor          string                  `json:"actor"`
	Action         string                  `json:"action"`
	EntityType     AuditEntityType         `json:"entity_type"`
	EntityID       string                  `json:"entity_id"`
	Metadata       map[string]any          `json:"metadata"`
	CreatedAt      time.Time               `json:"created_at"`
}
