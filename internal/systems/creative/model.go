// Package creative owns the advertising creative vertical's business state.
// It consumes only stable project context and Provider capability seams.
package creative

import (
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	ScopeRead  contract.Scope = "creative.read"
	ScopeWrite contract.Scope = "creative.write"
)

type IntakeSource string

const (
	IntakeSourceManual           IntakeSource = "manual"
	IntakeSourceStrategyPackage  IntakeSource = "strategy_package"
	IntakeSourceUploadedDocument IntakeSource = "uploaded_document"
	IntakeSourceConversation     IntakeSource = "conversation"
)

type IntakeStatus string

const (
	IntakeDraft              IntakeStatus = "draft"
	IntakeNeedsClarification IntakeStatus = "needs_clarification"
	IntakeReady              IntakeStatus = "ready"
	IntakeSuperseded         IntakeStatus = "superseded"
)

type CreativeFormat string

const FormatImageText CreativeFormat = "image_text"

type CreativeChannel string

const ChannelXiaohongshu CreativeChannel = "xiaohongshu"

type TaskStatus string

const (
	TaskDraft      TaskStatus = "draft"
	TaskInProgress TaskStatus = "in_progress"
	TaskReady      TaskStatus = "ready_for_review"
	// TaskArchived is a reversible-looking UI state backed by a retained record.
	// It deliberately does not delete drafts, Provider lineage, or frozen versions.
	TaskArchived TaskStatus = "archived"
)

type CreateIntakeRequest struct {
	Source IntakeSource `json:"source"`
	// StrategyPackage is supplied only for the explicit, user-triggered handoff
	// from an immutable Strategy package. The server reads and validates that
	// package; callers never submit its content as trusted Creative input.
	StrategyPackage *StrategyPackageReference `json:"strategy_package,omitempty"`
	Channel         CreativeChannel           `json:"channel"`
	Objective       string                    `json:"objective"`
	Audience        string                    `json:"audience"`
	CoreMessage     string                    `json:"core_message"`
	CallToAction    string                    `json:"call_to_action"`
	Concept         string                    `json:"concept"`
	Tone            []string                  `json:"tone"`
	VisualKeywords  []string                  `json:"visual_keywords"`
	Mandatory       []string                  `json:"mandatory_elements"`
	Prohibited      []string                  `json:"prohibited_claims"`
}

type StrategyPackageReference struct {
	PackageID           string `json:"package_id"`
	PackageVersion      int64  `json:"package_version"`
	ExpectedContentHash string `json:"expected_content_hash"`
}

func (r StrategyPackageReference) Validate() error {
	if strings.TrimSpace(r.PackageID) == "" || r.PackageVersion < 1 || strings.TrimSpace(r.ExpectedContentHash) == "" {
		return fmt.Errorf("strategy_package package_id, package_version, and expected_content_hash are required")
	}
	return nil
}

func (r CreateIntakeRequest) Validate() error {
	if r.Source == "" {
		return fmt.Errorf("source is required")
	}
	switch r.Source {
	case IntakeSourceManual:
		if r.StrategyPackage != nil {
			return fmt.Errorf("manual intake must not include strategy_package")
		}
	case IntakeSourceStrategyPackage:
		if r.StrategyPackage == nil {
			return fmt.Errorf("strategy_package is required for a strategy intake")
		}
		return r.StrategyPackage.Validate()
	default:
		return fmt.Errorf("unsupported Creative intake source %q", r.Source)
	}
	return r.validateContent()
}

func (r CreateIntakeRequest) validateContent() error {
	if r.Channel != ChannelXiaohongshu {
		return fmt.Errorf("Creative M1 supports the xiaohongshu channel")
	}
	if len(r.Objective) > 500 || len(r.Audience) > 500 || len(r.CoreMessage) > 1000 || len(r.CallToAction) > 300 || len(r.Concept) > 500 {
		return fmt.Errorf("creative input exceeds its maximum length")
	}
	if err := validateStringList("tone", r.Tone, 12, 80); err != nil {
		return err
	}
	if err := validateStringList("visual_keywords", r.VisualKeywords, 16, 120); err != nil {
		return err
	}
	if err := validateStringList("mandatory_elements", r.Mandatory, 20, 200); err != nil {
		return err
	}
	return validateStringList("prohibited_claims", r.Prohibited, 20, 200)
}

func (r CreateIntakeRequest) missingFields() []string {
	missing := make([]string, 0, 3)
	if strings.TrimSpace(r.Objective) == "" {
		missing = append(missing, "objective")
	}
	if strings.TrimSpace(r.Audience) == "" {
		missing = append(missing, "audience")
	}
	if strings.TrimSpace(r.CoreMessage) == "" {
		missing = append(missing, "core_message")
	}
	return missing
}

type CreativeIntake struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Source         IntakeSource            `json:"source"`
	Status         IntakeStatus            `json:"status"`
	Request        CreateIntakeRequest     `json:"request"`
	MissingFields  []string                `json:"missing_fields"`
	Warnings       []string                `json:"warnings"`
	ConfirmedBy    string                  `json:"confirmed_by,omitempty"`
	Principal      contract.Principal      `json:"-"`
	IdempotencyKey contract.IdempotencyKey `json:"-"`
	RequestHash    string                  `json:"-"`
	Version        int64                   `json:"version"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type CreativeTask struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	IntakeID       string                  `json:"intake_id"`
	Format         CreativeFormat          `json:"format"`
	Channel        CreativeChannel         `json:"channel"`
	Status         TaskStatus              `json:"status"`
	Direction      CreativeDirection       `json:"direction"`
	Version        int64                   `json:"version"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type CreativeContentType string

const (
	ContentTypeLifestyle             CreativeContentType = "lifestyle"
	ContentTypeIngredientExplanation CreativeContentType = "ingredient_explanation"
	ContentTypeUsageScenario         CreativeContentType = "usage_scenario"
	ContentTypeListGuide             CreativeContentType = "list_guide"
	ContentTypeComparison            CreativeContentType = "comparison"
	ContentTypeCustom                CreativeContentType = "custom"
)

// CreateTaskRequest is the explicit second-stage brief that differentiates
// several Creative tasks produced from one approved Strategy package.
type CreateTaskRequest struct {
	ContentType  CreativeContentType `json:"content_type"`
	Focus        string              `json:"focus"`
	Audience     string              `json:"audience,omitempty"`
	CoreMessage  string              `json:"core_message,omitempty"`
	CallToAction string              `json:"call_to_action,omitempty"`
}

func (r CreateTaskRequest) Validate() error {
	switch r.ContentType {
	case ContentTypeLifestyle, ContentTypeIngredientExplanation, ContentTypeUsageScenario, ContentTypeListGuide, ContentTypeComparison, ContentTypeCustom:
	default:
		return fmt.Errorf("unsupported Creative content_type %q", r.ContentType)
	}
	if len(strings.TrimSpace(r.Focus)) == 0 || len(r.Focus) > 300 || len(r.Audience) > 500 || len(r.CoreMessage) > 1000 || len(r.CallToAction) > 300 {
		return fmt.Errorf("creative task focus is required or task input exceeds its maximum length")
	}
	return nil
}

type CreativeDirection struct {
	ContentType    CreativeContentType `json:"content_type"`
	Focus          string              `json:"focus"`
	Audience       string              `json:"audience"`
	CoreMessage    string              `json:"core_message"`
	CallToAction   string              `json:"call_to_action"`
	Concept        string              `json:"concept"`
	Tone           []string            `json:"tone"`
	VisualKeywords []string            `json:"visual_keywords"`
}

type ImageTextDraft struct {
	TaskID          string          `json:"task_id"`
	Version         int64           `json:"version"`
	Status          string          `json:"status"`
	TitleCandidates []string        `json:"title_candidates"`
	Body            string          `json:"body"`
	Topics          []string        `json:"topics"`
	CoverCopy       string          `json:"cover_copy"`
	ImagePlan       []ImagePlanItem `json:"image_plan"`
	CreatedAt       time.Time       `json:"created_at"`
}

// ReviseDraftRequest replaces the editable content of the current draft. The
// expected version is an optimistic-lock boundary: a stale browser must reload
// rather than silently overwriting someone else's Creative revision.
type ReviseDraftRequest struct {
	ExpectedVersion int64           `json:"expected_version"`
	TitleCandidates []string        `json:"title_candidates"`
	Body            string          `json:"body"`
	Topics          []string        `json:"topics"`
	CoverCopy       string          `json:"cover_copy"`
	ImagePlan       []ImagePlanItem `json:"image_plan"`
}

// BindImageAssetRequest binds an already-ready project asset to one planned
// image. The Asset context owns the asset itself; Creative retains only the
// immutable asset-version reference in its next Draft revision.
type BindImageAssetRequest struct {
	ExpectedDraftVersion int64                    `json:"expected_draft_version"`
	ImagePlanOrder       int                      `json:"image_plan_order"`
	AssetRef             contract.AssetVersionRef `json:"asset_ref"`
}

func (r BindImageAssetRequest) Validate() error {
	if r.ExpectedDraftVersion < 1 || r.ImagePlanOrder < 1 || r.ImagePlanOrder > 12 {
		return fmt.Errorf("expected_draft_version and image_plan_order are invalid")
	}
	return r.AssetRef.Validate()
}

// CreateImageJobRequest is deliberately scoped to an image-plan position.
// A retry for one failed image never recreates the other images in the group.
type CreateImageJobRequest struct {
	ImagePlanOrder int    `json:"image_plan_order"`
	ModelAlias     string `json:"model_alias"`
}

func (r CreateImageJobRequest) Validate() error {
	if r.ImagePlanOrder < 1 || r.ImagePlanOrder > 12 {
		return fmt.Errorf("image_plan_order must be between 1 and 12")
	}
	if len(r.ModelAlias) > 128 {
		return fmt.Errorf("model_alias is too long")
	}
	return nil
}

func (r ReviseDraftRequest) Validate() error {
	if r.ExpectedVersion < 1 {
		return fmt.Errorf("expected_version must be positive")
	}
	if len(r.TitleCandidates) < 3 || len(r.TitleCandidates) > 8 {
		return fmt.Errorf("title_candidates must contain between 3 and 8 candidates")
	}
	if len(strings.TrimSpace(r.Body)) == 0 || len(r.Body) > 5000 {
		return fmt.Errorf("body is required and must not exceed 5000 characters")
	}
	if len(strings.TrimSpace(r.CoverCopy)) == 0 || len([]rune(r.CoverCopy)) > 30 {
		return fmt.Errorf("cover_copy is required and must not exceed 30 characters")
	}
	if len(r.Topics) > 12 || len(r.ImagePlan) < 1 || len(r.ImagePlan) > 12 {
		return fmt.Errorf("topics or image_plan is outside the supported range")
	}
	for _, title := range r.TitleCandidates {
		if len(strings.TrimSpace(title)) == 0 || len([]rune(title)) > 80 {
			return fmt.Errorf("title_candidates contains an invalid value")
		}
	}
	for _, topic := range r.Topics {
		if len(strings.TrimSpace(topic)) == 0 || len([]rune(topic)) > 80 {
			return fmt.Errorf("topics contains an invalid value")
		}
	}
	for index, item := range r.ImagePlan {
		if item.Order != index+1 || strings.TrimSpace(item.Purpose) == "" || strings.TrimSpace(item.VisualBrief) == "" || strings.TrimSpace(item.Caption) == "" {
			return fmt.Errorf("image_plan must have ordered, complete items")
		}
		if item.AssetRef != nil {
			if err := item.AssetRef.Validate(); err != nil {
				return fmt.Errorf("image_plan asset_ref: %w", err)
			}
		}
	}
	return nil
}

func (r ReviseDraftRequest) Draft(taskID string, version int64, now time.Time) ImageTextDraft {
	return ImageTextDraft{TaskID: taskID, Version: version, Status: "draft", TitleCandidates: append([]string{}, r.TitleCandidates...), Body: r.Body,
		Topics: append([]string{}, r.Topics...), CoverCopy: r.CoverCopy, ImagePlan: append([]ImagePlanItem{}, r.ImagePlan...), CreatedAt: now}
}

// CreativeVersion is the immutable Creative-owned snapshot that downstream
// systems may reference. A draft remains editable; a CreativeVersion never is.
type CreativeVersionStatus string

const (
	CreativeVersionCreated    CreativeVersionStatus = "created"
	CreativeVersionChecked    CreativeVersionStatus = "checked"
	CreativeVersionApproved   CreativeVersionStatus = "approved"
	CreativeVersionSuperseded CreativeVersionStatus = "superseded"
)

type FreezeVersionRequest struct {
	DraftVersion int64 `json:"draft_version"`
}

func (r FreezeVersionRequest) Validate() error {
	if r.DraftVersion < 1 {
		return fmt.Errorf("draft_version must be positive")
	}
	return nil
}

type CreativeVersion struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	TaskID         string                  `json:"creative_task_id"`
	Version        int64                   `json:"version"`
	DraftVersion   int64                   `json:"draft_version"`
	Status         CreativeVersionStatus   `json:"status"`
	Snapshot       ImageTextDraft          `json:"snapshot"`
	ContentHash    contract.ContentHash    `json:"content_hash"`
	CreatedBy      string                  `json:"created_by"`
	CreatedAt      time.Time               `json:"created_at"`
	Check          *CreativeCheck          `json:"check,omitempty"`
	Approval       *CreativeApproval       `json:"approval,omitempty"`
	IdempotencyKey contract.IdempotencyKey `json:"-"`
	RequestHash    string                  `json:"-"`
}

// CreativeCheck is an auditable, deterministic Phase-1 gate. It records why
// a frozen snapshot cannot proceed rather than silently changing its content.
type CreativeCheck struct {
	Passed    bool      `json:"passed"`
	Blockers  []string  `json:"blockers"`
	Warnings  []string  `json:"warnings"`
	CheckedBy string    `json:"checked_by"`
	CheckedAt time.Time `json:"checked_at"`
}

type CreativeApproval struct {
	ApprovedBy string    `json:"approved_by"`
	ApprovedAt time.Time `json:"approved_at"`
}

// CreativePackage is the stable output consumed by Delivery and Insights. It
// references one approved immutable CreativeVersion and never a mutable task.
type CreativePackage struct {
	ID                string                  `json:"id"`
	OrganizationID    contract.OrganizationID `json:"organization_id"`
	ProjectID         contract.ProjectID      `json:"project_id"`
	CreativeVersionID string                  `json:"creative_version_id"`
	ContentHash       contract.ContentHash    `json:"content_hash"`
	Snapshot          ImageTextDraft          `json:"snapshot"`
	CreatedBy         string                  `json:"created_by"`
	CreatedAt         time.Time               `json:"created_at"`
}

func (v CreativeVersion) Validate() error {
	if strings.TrimSpace(v.ID) == "" || strings.TrimSpace(v.TaskID) == "" || v.Version < 1 || v.DraftVersion < 1 ||
		v.OrganizationID == "" || v.ProjectID == "" || v.ContentHash.Validate() != nil || v.CreatedAt.IsZero() {
		return fmt.Errorf("creative version is incomplete")
	}
	if v.Status != CreativeVersionCreated && v.Status != CreativeVersionChecked && v.Status != CreativeVersionApproved && v.Status != CreativeVersionSuperseded {
		return fmt.Errorf("creative version status is invalid")
	}
	if v.Snapshot.TaskID != v.TaskID || v.Snapshot.Version != v.DraftVersion {
		return fmt.Errorf("creative version snapshot does not match its draft reference")
	}
	return nil
}

type ImagePlanItem struct {
	Order       int                       `json:"order"`
	Purpose     string                    `json:"purpose"`
	VisualBrief string                    `json:"visual_brief"`
	Caption     string                    `json:"caption"`
	AssetRef    *contract.AssetVersionRef `json:"asset_ref,omitempty"`
}

type ProductionJob struct {
	TaskID        string    `json:"task_id"`
	Kind          string    `json:"kind"`
	ProviderJobID string    `json:"provider_job_id"`
	CreatedAt     time.Time `json:"created_at"`
}

type TaskDetail struct {
	Task           CreativeTask    `json:"task"`
	Intake         CreativeIntake  `json:"intake"`
	Draft          ImageTextDraft  `json:"draft"`
	ProductionJobs []ProductionJob `json:"production_jobs"`
}

func validateStringList(name string, values []string, maxItems, maxLength int) error {
	if values == nil {
		return fmt.Errorf("%s must be an array", name)
	}
	if len(values) > maxItems {
		return fmt.Errorf("%s has too many values", name)
	}
	for _, value := range values {
		if len(strings.TrimSpace(value)) == 0 || len(value) > maxLength {
			return fmt.Errorf("%s contains an invalid value", name)
		}
	}
	return nil
}
