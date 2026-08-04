package creative

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const aiNativeRequirementContract = "creative.ai-native.requirement/v1"

var (
	ErrInvalidAINativeRequirement = errors.New("invalid AI native requirement")
	ErrAINativeProductUnavailable = errors.New("AI native product is unavailable")
)

type AINativeProductImage struct {
	URL      string                    `json:"url"`
	Role     string                    `json:"role"`
	AssetRef *contract.AssetVersionRef `json:"asset_ref,omitempty"`
}

type AINativeProductPrice struct {
	MinRaw             int64  `json:"min_raw"`
	MaxRaw             int64  `json:"max_raw"`
	Currency           string `json:"currency"`
	DisplayUnconfirmed bool   `json:"display_unconfirmed"`
}

type AINativeProductSnapshot struct {
	Source      string                 `json:"source"`
	ProductID   string                 `json:"product_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Images      []AINativeProductImage `json:"images"`
	Price       AINativeProductPrice   `json:"price"`
	Sales       int64                  `json:"sales"`
	SourceURL   string                 `json:"source_url"`
}

type AINativeProductResolver interface {
	Resolve(context.Context, string) (AINativeProductSnapshot, error)
}

type AINativeProductMediaImporter interface {
	ImportProductMedia(context.Context, contract.ActorContext, contract.ProjectID, string, []AINativeRequirementMedia) ([]AINativeRequirementMedia, error)
}

type AnalyzeAINativeRequirementRequest struct {
	ProductLink             string `json:"product_link"`
	SupplementalRequirement string `json:"supplemental_requirement"`
	Channel                 string `json:"channel,omitempty"`
	AspectRatio             string `json:"aspect_ratio,omitempty"`
	DurationSeconds         int    `json:"duration_seconds,omitempty"`
	Language                string `json:"language,omitempty"`
}

func (r AnalyzeAINativeRequirementRequest) normalized() (AnalyzeAINativeRequirementRequest, error) {
	r.ProductLink = strings.TrimSpace(r.ProductLink)
	r.SupplementalRequirement = strings.TrimSpace(r.SupplementalRequirement)
	r.Channel = strings.TrimSpace(r.Channel)
	r.AspectRatio = strings.TrimSpace(r.AspectRatio)
	r.Language = strings.TrimSpace(r.Language)
	if r.Channel == "" {
		r.Channel = "douyin"
	}
	if r.AspectRatio == "" {
		r.AspectRatio = "9:16"
	}
	if r.DurationSeconds == 0 {
		r.DurationSeconds = 20
	}
	if r.Language == "" {
		r.Language = "zh-CN"
	}
	if r.ProductLink == "" {
		return r, fmt.Errorf("%w: product_link is required", ErrInvalidAINativeRequirement)
	}
	if len([]rune(r.SupplementalRequirement)) > 2000 {
		return r, fmt.Errorf("%w: supplemental_requirement must not exceed 2000 characters", ErrInvalidAINativeRequirement)
	}
	if r.Channel != "douyin" {
		return r, fmt.Errorf("%w: only douyin is enabled in P0", ErrInvalidAINativeRequirement)
	}
	if r.AspectRatio != "9:16" {
		return r, fmt.Errorf("%w: only 9:16 is enabled for douyin in P0", ErrInvalidAINativeRequirement)
	}
	if r.DurationSeconds < 15 || r.DurationSeconds > 30 {
		return r, fmt.Errorf("%w: duration_seconds must be between 15 and 30", ErrInvalidAINativeRequirement)
	}
	if r.Language != "zh-CN" {
		return r, fmt.Errorf("%w: only zh-CN is enabled in P0", ErrInvalidAINativeRequirement)
	}
	return r, nil
}

type AINativeEditableText struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type AINativeRequirementMedia struct {
	ID       string                    `json:"id"`
	URL      string                    `json:"url"`
	Role     string                    `json:"role"`
	Source   string                    `json:"source"`
	AssetRef *contract.AssetVersionRef `json:"asset_ref,omitempty"`
}

type AINativeGenerationMetadata struct {
	Mode            string `json:"mode"`
	ModelAlias      string `json:"model_alias"`
	ModelVersion    string `json:"model_version"`
	RouteRevisionID string `json:"route_revision_id,omitempty"`
	PromptVersion   string `json:"prompt_version"`
}

type AINativeRequirementDraft struct {
	ContractVersion         string                     `json:"contract_version"`
	Revision                int64                      `json:"revision"`
	Status                  string                     `json:"status"`
	Product                 AINativeProductSnapshot    `json:"product"`
	ProductName             string                     `json:"product_name"`
	ProductDescription      string                     `json:"product_description"`
	TargetAudiences         []AINativeEditableText     `json:"target_audiences"`
	Media                   []AINativeRequirementMedia `json:"media"`
	CoreSellingPoints       []AINativeEditableText     `json:"core_selling_points"`
	SupplementalRequirement string                     `json:"supplemental_requirement"`
	Channel                 string                     `json:"channel"`
	AspectRatio             string                     `json:"aspect_ratio"`
	DurationSeconds         int                        `json:"duration_seconds"`
	Language                string                     `json:"language"`
	NeedsConfirmation       []string                   `json:"needs_confirmation"`
	Generation              AINativeGenerationMetadata `json:"generation"`
}

func (d AINativeRequirementDraft) Validate() error {
	if d.ContractVersion != aiNativeRequirementContract || d.Revision < 1 || d.Status != "draft" ||
		strings.TrimSpace(d.Product.ProductID) == "" || strings.TrimSpace(d.ProductName) == "" || strings.TrimSpace(d.ProductDescription) == "" ||
		len(d.TargetAudiences) == 0 || len(d.TargetAudiences) > 10 || len(d.CoreSellingPoints) == 0 || len(d.CoreSellingPoints) > 20 ||
		d.Channel != "douyin" || d.AspectRatio != "9:16" || d.DurationSeconds < 15 || d.DurationSeconds > 30 || d.Language != "zh-CN" ||
		len([]rune(d.SupplementalRequirement)) > 2000 || strings.TrimSpace(d.Generation.Mode) == "" ||
		strings.TrimSpace(d.Generation.ModelAlias) == "" || strings.TrimSpace(d.Generation.ModelVersion) == "" || strings.TrimSpace(d.Generation.PromptVersion) == "" {
		return fmt.Errorf("AI native requirement draft is invalid")
	}
	for _, collection := range [][]AINativeEditableText{d.TargetAudiences, d.CoreSellingPoints} {
		seen := map[string]struct{}{}
		for _, item := range collection {
			if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Text) == "" {
				return fmt.Errorf("AI native editable item is invalid")
			}
			if _, exists := seen[item.ID]; exists {
				return fmt.Errorf("AI native editable item ID is duplicated")
			}
			seen[item.ID] = struct{}{}
		}
	}
	if len(d.Media) > 20 {
		return fmt.Errorf("AI native requirement media limit exceeded")
	}
	seenMedia := map[string]struct{}{}
	for _, item := range d.Media {
		parsed, err := url.Parse(strings.TrimSpace(item.URL))
		if strings.TrimSpace(item.ID) == "" || err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" ||
			strings.TrimSpace(item.Role) == "" || strings.TrimSpace(item.Source) == "" {
			return fmt.Errorf("AI native requirement media is invalid")
		}
		if _, exists := seenMedia[item.ID]; exists {
			return fmt.Errorf("AI native requirement media ID is duplicated")
		}
		if item.AssetRef != nil && item.AssetRef.Validate() != nil {
			return fmt.Errorf("AI native requirement media asset_ref is invalid")
		}
		seenMedia[item.ID] = struct{}{}
	}
	return nil
}

func (s Service) AnalyzeAINativeRequirement(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request AnalyzeAINativeRequirementRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeProducts == nil || s.AINativeRequirementPlanner == nil || s.AINativeRequirements == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native requirement dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	normalized, err := request.normalized()
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	project, err := s.Projects.RequireActiveContext(ctx, actor, projectID)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	product, err := s.AINativeProducts.Resolve(ctx, normalized.ProductLink)
	if err != nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%w: %v", ErrAINativeProductUnavailable, err)
	}
	draft, err := s.AINativeRequirementPlanner.Analyze(ctx, actor, project, product, normalized)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if err := draft.Validate(); err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	id, err := s.idGenerator()("ainativeworkspace")
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	intakeID, err := s.idGenerator()("creativeintake")
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	taskID, err := s.idGenerator()("creativetask")
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	if s.AINativeProductMediaImporter != nil && len(draft.Media) > 0 {
		media, importErr := s.AINativeProductMediaImporter.ImportProductMedia(ctx, actor, projectID, id, draft.Media)
		if importErr != nil {
			return AINativeRequirementWorkspace{}, fmt.Errorf("import AI native product media: %w", importErr)
		}
		draft.Media = media
		for imageIndex := range draft.Product.Images {
			for _, item := range media {
				if item.URL == draft.Product.Images[imageIndex].URL {
					draft.Product.Images[imageIndex].AssetRef = item.AssetRef
					break
				}
			}
		}
		if err := draft.Validate(); err != nil {
			return AINativeRequirementWorkspace{}, err
		}
	}
	now := s.now()
	workspace := AINativeRequirementWorkspace{
		WorkspaceID: id, CreativeIntakeID: intakeID, CreativeTaskID: taskID,
		OrganizationID: actor.OrganizationID, ProjectID: projectID,
		Status: AINativeRequirementDraftStatus, CurrentStage: AINativeStageRequirement, WorkspaceVersion: 1,
		CurrentRevision: 1, Requirement: draft,
		CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now,
	}
	return s.AINativeRequirements.CreateAINativeRequirementWorkspace(ctx, workspace)
}
