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

type CreativeDirection struct {
	Concept        string   `json:"concept"`
	Tone           []string `json:"tone"`
	VisualKeywords []string `json:"visual_keywords"`
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

type ImagePlanItem struct {
	Order       int    `json:"order"`
	Purpose     string `json:"purpose"`
	VisualBrief string `json:"visual_brief"`
	Caption     string `json:"caption"`
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
