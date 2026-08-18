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

// Industry is deliberately owned by Project rather than Brand: the same brand
// may run different category campaigns in different projects.
type Industry string

const (
	IndustryShortDrama      Industry = "short_drama"
	IndustryGame            Industry = "game"
	IndustryEcommerce       Industry = "ecommerce"
	IndustryAutomotiveBrand Industry = "automotive_brand"
)

func (i Industry) Valid() bool {
	switch i {
	case IndustryShortDrama, IndustryGame, IndustryEcommerce, IndustryAutomotiveBrand:
		return true
	default:
		return false
	}
}

type Project struct {
	ID                      contract.ProjectID      `json:"id"`
	OrganizationID          contract.OrganizationID `json:"organization_id"`
	Name                    string                  `json:"name"`
	Status                  Status                  `json:"status"`
	Industry                Industry                `json:"industry"`
	PrimaryBrandID          *contract.BrandID       `json:"primary_brand_id"`
	PrimaryBrandStatus      string                  `json:"-"`
	BrandGuidelineVersionID string                  `json:"brand_guideline_version_id,omitempty"`
	ProjectContextVersion   int64                   `json:"project_context_version"`
	CreatedAt               time.Time               `json:"created_at"`
	UpdatedAt               time.Time               `json:"updated_at"`
}

type ProjectMembership struct {
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	PrincipalKind  contract.PrincipalKind  `json:"principal_kind"`
	PrincipalID    string                  `json:"principal_id"`
	DisplayName    string                  `json:"display_name"`
	Role           string                  `json:"role"`
	Status         string                  `json:"status"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type UpdateProjectMembershipRequest struct {
	Role              string    `json:"role"`
	Status            string    `json:"status"`
	ExpectedUpdatedAt time.Time `json:"expected_updated_at"`
}

func ValidProjectRole(value string) bool {
	switch value {
	case "owner", "editor", "viewer", "worker":
		return true
	default:
		return false
	}
}

func ValidProjectRoleForPrincipal(kind contract.PrincipalKind, role string) bool {
	switch kind {
	case contract.PrincipalUser:
		return role == "owner" || role == "editor" || role == "viewer"
	case contract.PrincipalService:
		return role == "worker"
	default:
		return false
	}
}

type ProjectDetail struct {
	Project    Project                  `json:"project"`
	Runtime    ProjectRuntime           `json:"runtime"`
	Artifacts  []ProjectArtifactSummary `json:"artifacts"`
	Tasks      []BusinessTask           `json:"tasks"`
	Operations []OperationalRecord      `json:"operations"`
	ChangeSets []ChangeSet              `json:"change_sets"`
}

// ProductProjectRef is a lightweight project reference for the product
// catalog's "used by projects" view.
type ProductProjectRef struct {
	ProjectID contract.ProjectID `json:"project_id"`
	Name      string             `json:"name"`
}

type ProjectRuntime struct {
	Code           string    `json:"code"`
	Brand          string    `json:"brand"`
	Product        string    `json:"product"`
	Goal           string    `json:"goal"`
	Stage          string    `json:"stage"`
	Progress       int       `json:"progress"`
	Status         string    `json:"status"`
	Owner          string    `json:"owner"`
	Budget         float64   `json:"budget"`
	Currency       string    `json:"currency"`
	Timezone       string    `json:"timezone"`
	KnowledgeCount int       `json:"knowledge_count"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Workbench is the typed, project-scoped read model used by the agency
// workbench. It intentionally does not reuse OperationalRecord.Fields.
type Workbench struct {
	Organization          WorkbenchOrganization             `json:"organization"`
	Client                WorkbenchClient                   `json:"client"`
	Brand                 WorkbenchBrand                    `json:"brand"`
	Project               WorkbenchProject                  `json:"project"`
	Products              []contract.ProjectBusinessProduct `json:"products"`
	AdAccountBindings     []WorkbenchAdAccountBinding       `json:"ad_account_bindings"`
	QualityCheckRuns      []WorkbenchQualityCheckRun        `json:"quality_check_runs"`
	MaterialConfirmations []WorkbenchMaterialConfirmation   `json:"material_confirmations"`
	AssetVersionPointers  []WorkbenchAssetVersionPointer    `json:"asset_version_pointers"`
}

type WorkbenchOrganization struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Owner     string    `json:"owner"`
	Currency  string    `json:"currency"`
	Timezone  string    `json:"timezone"`
	UpdatedAt time.Time `json:"updated_at"`
}
type WorkbenchClient struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	Industry       string    `json:"industry"`
	Owner          string    `json:"owner"`
	HealthStatus   string    `json:"health_status"`
	UpdatedAt      time.Time `json:"updated_at"`
}
type WorkbenchBrand struct {
	ID              string    `json:"id"`
	OrganizationID  string    `json:"organization_id"`
	ClientID        string    `json:"client_id"`
	Code            string    `json:"code"`
	Name            string    `json:"name"`
	Category        string    `json:"category"`
	ProductLines    []string  `json:"product_lines"`
	Owner           string    `json:"owner"`
	GuidelineStatus string    `json:"guideline_status"`
	UpdatedAt       time.Time `json:"updated_at"`
}
type WorkbenchProject struct {
	ProjectID      string    `json:"project_id"`
	OrganizationID string    `json:"organization_id"`
	ClientID       string    `json:"client_id"`
	BrandID        string    `json:"brand_id"`
	Stage          string    `json:"stage"`
	StageLabel     string    `json:"stage_label"`
	StagePercent   int       `json:"stage_percent"`
	TaskPercent    int       `json:"task_percent"`
	RiskStatus     string    `json:"risk_status"`
	Blocker        string    `json:"blocker"`
	UpdatedAt      time.Time `json:"updated_at"`
}
type WorkbenchAdAccountBinding struct {
	ID               string    `json:"id"`
	OrganizationID   string    `json:"organization_id"`
	ClientID         string    `json:"client_id"`
	BrandID          string    `json:"brand_id"`
	Platform         string    `json:"platform"`
	AccountName      string    `json:"account_name"`
	AccountDisplayID string    `json:"account_display_id"`
	Currency         string    `json:"currency"`
	Timezone         string    `json:"timezone"`
	PermissionStatus string    `json:"permission_status"`
	LoginStatus      string    `json:"login_status"`
	TrackingStatus   string    `json:"tracking_status"`
	Owner            string    `json:"owner"`
	BoundAssetIDs    []string  `json:"bound_asset_ids"`
	LastSyncedAt     time.Time `json:"last_synced_at"`
}
type WorkbenchQualityCheckRun struct {
	ID             string                       `json:"id"`
	OrganizationID string                       `json:"organization_id"`
	ProjectID      string                       `json:"project_id"`
	AssetID        string                       `json:"asset_id"`
	AssetVersion   int                          `json:"asset_version"`
	Status         string                       `json:"status"`
	Model          string                       `json:"model"`
	RuleVersion    string                       `json:"rule_version"`
	PromptVersion  string                       `json:"prompt_version"`
	Summary        string                       `json:"summary"`
	Issues         []WorkbenchQualityCheckIssue `json:"issues"`
	CreatedAt      time.Time                    `json:"created_at"`
	CompletedAt    *time.Time                   `json:"completed_at"`
}
type WorkbenchQualityCheckIssue struct {
	ID         string `json:"id"`
	Severity   string `json:"severity"`
	Rule       string `json:"rule"`
	Evidence   string `json:"evidence"`
	Suggestion string `json:"suggestion"`
}
type WorkbenchMaterialConfirmation struct {
	ID                string    `json:"id"`
	OrganizationID    string    `json:"organization_id"`
	ProjectID         string    `json:"project_id"`
	QualityCheckRunID string    `json:"quality_check_run_id"`
	AssetID           string    `json:"asset_id"`
	AssetVersion      int       `json:"asset_version"`
	Status            string    `json:"status"`
	Scope             string    `json:"scope"`
	ConfirmedBy       string    `json:"confirmed_by"`
	Note              string    `json:"note"`
	CreatedAt         time.Time `json:"created_at"`
}
type WorkbenchAssetVersionPointer struct {
	ID                    string                      `json:"id"`
	OrganizationID        string                      `json:"organization_id"`
	ProjectID             string                      `json:"project_id"`
	AssetID               string                      `json:"asset_id"`
	OceanEngineMaterialID string                      `json:"ocean_engine_material_id,omitempty"`
	WorkingVersion        int                         `json:"working_version"`
	QualityCheckedVersion *int                        `json:"quality_checked_version"`
	HumanConfirmedVersion *int                        `json:"human_confirmed_version"`
	DeliveryVersion       *int                        `json:"delivery_version"`
	Versions              []WorkbenchAssetVersion     `json:"versions"`
	Authorization         WorkbenchAssetAuthorization `json:"authorization"`
	DeliveryTarget        WorkbenchDeliveryTarget     `json:"delivery_target"`
	Owner                 string                      `json:"owner"`
	UpdatedAt             time.Time                   `json:"updated_at"`
}
type WorkbenchAssetVersion struct {
	Version       int       `json:"version"`
	CreatedBy     string    `json:"created_by"`
	SourceTaskID  string    `json:"source_task_id"`
	SourceType    string    `json:"source_type"`
	SourceLabel   string    `json:"source_label"`
	CreatedAt     time.Time `json:"created_at"`
	ChangeSummary string    `json:"change_summary"`
}
type WorkbenchAssetAuthorization struct {
	Platforms    []string  `json:"platforms"`
	Regions      []string  `json:"regions"`
	RightsHolder string    `json:"rights_holder"`
	ExpiresAt    time.Time `json:"expires_at"`
	Note         string    `json:"note"`
}
type WorkbenchDeliveryTarget struct {
	Platform string `json:"platform"`
	Region   string `json:"region"`
}

type RunWorkbenchQualityCheckRequest struct {
	AssetID      string `json:"-"`
	AssetVersion int    `json:"-"`
}

type RecordWorkbenchMaterialConfirmationRequest struct {
	AssetID      string `json:"-"`
	AssetVersion int    `json:"-"`
	Status       string `json:"status"`
	Scope        string `json:"scope"`
	Note         string `json:"note"`
}

type UpdateWorkbenchAssetPointerRequest struct {
	AssetID         string `json:"-"`
	DeliveryVersion *int   `json:"delivery_version"`
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

type ArtifactKind string

const (
	ArtifactKindBrief    ArtifactKind = "brief"
	ArtifactKindImage    ArtifactKind = "image"
	ArtifactKindVideo    ArtifactKind = "video"
	ArtifactKindDocument ArtifactKind = "document"
)

func (k ArtifactKind) Valid() bool {
	switch k {
	case ArtifactKindBrief, ArtifactKindImage, ArtifactKindVideo, ArtifactKindDocument:
		return true
	default:
		return false
	}
}

type ArtifactStatus string

const (
	ArtifactStatusDraft    ArtifactStatus = "draft"
	ArtifactStatusReady    ArtifactStatus = "ready"
	ArtifactStatusArchived ArtifactStatus = "archived"
)

func (s ArtifactStatus) Valid() bool {
	switch s {
	case ArtifactStatusDraft, ArtifactStatusReady, ArtifactStatusArchived:
		return true
	default:
		return false
	}
}

// ProjectArtifact is the Project-scoped, versioned text or media reference
// used by the MVP workflow. Binary media remains owned by the assets service;
// this entity records the business artifact and its source job reference.
type ProjectArtifact struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"-"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Kind           ArtifactKind            `json:"kind"`
	Status         ArtifactStatus          `json:"status"`
	Content        string                  `json:"content"`
	SourceJobID    string                  `json:"source_job_id,omitempty"`
	Version        int64                   `json:"version"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type CreateProjectArtifactRequest struct {
	Kind        ArtifactKind   `json:"kind"`
	Content     string         `json:"content"`
	Status      ArtifactStatus `json:"status"`
	SourceJobID string         `json:"source_job_id,omitempty"`
}

func (r CreateProjectArtifactRequest) Validate() error {
	if !r.Kind.Valid() {
		return fmt.Errorf("artifact kind is invalid")
	}
	if !r.Status.Valid() {
		return fmt.Errorf("artifact status is invalid")
	}
	if strings.TrimSpace(r.Content) == "" || len(r.Content) > 1<<20 {
		return fmt.Errorf("artifact content must be between 1 and 1048576 bytes")
	}
	if len(strings.TrimSpace(r.SourceJobID)) > 96 {
		return fmt.Errorf("source_job_id must be at most 96 characters")
	}
	return nil
}

type UpdateProjectArtifactRequest struct {
	Content         *string         `json:"content,omitempty"`
	Status          *ArtifactStatus `json:"status,omitempty"`
	SourceJobID     *string         `json:"source_job_id,omitempty"`
	ExpectedVersion *int64          `json:"expected_version,omitempty"`
}

func (r UpdateProjectArtifactRequest) Validate() error {
	if r.Content == nil && r.Status == nil && r.SourceJobID == nil {
		return fmt.Errorf("at least one mutable artifact field is required")
	}
	if r.Content != nil && (strings.TrimSpace(*r.Content) == "" || len(*r.Content) > 1<<20) {
		return fmt.Errorf("artifact content must be between 1 and 1048576 bytes")
	}
	if r.Status != nil && !r.Status.Valid() {
		return fmt.Errorf("artifact status is invalid")
	}
	if r.SourceJobID != nil && len(strings.TrimSpace(*r.SourceJobID)) > 96 {
		return fmt.Errorf("source_job_id must be at most 96 characters")
	}
	if r.ExpectedVersion != nil && *r.ExpectedVersion < 1 {
		return fmt.Errorf("expected_version must be positive")
	}
	return nil
}

type Brand struct {
	ID             contract.BrandID        `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	Name           string                  `json:"name"`
	Status         string                  `json:"status"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

// Product is the organization-level catalog object. OceanEngineProductID is
// a platform mapping that is bound after the product is created on the
// platform by the launch pipeline; until then the product is treated as not
// present on OceanEngine.
type Product struct {
	ID                   contract.ProductID      `json:"id"`
	OrganizationID       contract.OrganizationID `json:"organization_id"`
	Name                 string                  `json:"name"`
	Status               string                  `json:"status"`
	ActivityType         string                  `json:"activity_type,omitempty"`
	ActivityName         string                  `json:"activity_name,omitempty"`
	BrandName            string                  `json:"brand_name,omitempty"`
	OceanEngineProductID string                  `json:"ocean_engine_product_id,omitempty"`
	CreatedAt            time.Time               `json:"created_at"`
	UpdatedAt            time.Time               `json:"updated_at"`
}

// CreateProductRequest is the input for creating an organization-level
// product object. The OceanEngine mapping is intentionally absent: it is
// bound later by the launch pipeline after the product exists on the platform.
type CreateProductRequest struct {
	Name         string `json:"name"`
	ActivityType string `json:"activity_type,omitempty"`
	ActivityName string `json:"activity_name,omitempty"`
	BrandName    string `json:"brand_name,omitempty"`
}

// UpdateProductRequest carries optional field updates. A nil pointer keeps
// the existing value; an empty string clears the field.
type UpdateProductRequest struct {
	Name                 *string `json:"name,omitempty"`
	Status               *string `json:"status,omitempty"`
	ActivityType         *string `json:"activity_type,omitempty"`
	ActivityName         *string `json:"activity_name,omitempty"`
	BrandName            *string `json:"brand_name,omitempty"`
	OceanEngineProductID *string `json:"ocean_engine_product_id,omitempty"`
}

type CreateProjectRequest struct {
	Name           string               `json:"name"`
	Brand          string               `json:"brand,omitempty"`
	Goal           string               `json:"goal,omitempty"`
	Industry       Industry             `json:"industry"`
	PrimaryBrandID *contract.BrandID    `json:"primary_brand_id"`
	ProductIDs     []contract.ProductID `json:"product_ids"`
	Activate       bool                 `json:"activate"`
}

func (r CreateProjectRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" || len(r.Name) > 255 {
		return fmt.Errorf("project name must be between 1 and 255 characters")
	}
	if strings.TrimSpace(r.Brand) != "" && len(r.Brand) > 255 {
		return fmt.Errorf("brand must be at most 255 characters")
	}
	if len(strings.TrimSpace(r.Goal)) > 4000 {
		return fmt.Errorf("goal must be at most 4000 characters")
	}
	if r.Industry != "" && !r.Industry.Valid() {
		return fmt.Errorf("industry must be one of short_drama, game, ecommerce, automotive_brand")
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

// UpdateProjectRequest permits only Project-owned display and planning fields.
// Brand and product bindings remain owned by their dedicated versioned APIs.
type UpdateProjectRequest struct {
	Name                   *string   `json:"name,omitempty"`
	Brand                  *string   `json:"brand,omitempty"`
	Goal                   *string   `json:"goal,omitempty"`
	Industry               *Industry `json:"industry,omitempty"`
	ExpectedContextVersion *int64    `json:"expected_context_version,omitempty"`
}

func (r UpdateProjectRequest) Validate() error {
	if r.Name == nil && r.Brand == nil && r.Goal == nil && r.Industry == nil {
		return fmt.Errorf("at least one mutable project field is required")
	}
	if r.Name != nil && (strings.TrimSpace(*r.Name) == "" || len(*r.Name) > 255) {
		return fmt.Errorf("name must be between 1 and 255 characters")
	}
	if r.Brand != nil && (strings.TrimSpace(*r.Brand) == "" || len(*r.Brand) > 255) {
		return fmt.Errorf("brand must be between 1 and 255 characters")
	}
	if r.Goal != nil && len(strings.TrimSpace(*r.Goal)) > 4000 {
		return fmt.Errorf("goal must be at most 4000 characters")
	}
	if r.Industry != nil && !r.Industry.Valid() {
		return fmt.Errorf("industry must be one of short_drama, game, ecommerce, automotive_brand")
	}
	if r.ExpectedContextVersion != nil && *r.ExpectedContextVersion < 1 {
		return fmt.Errorf("expected_context_version must be positive")
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
