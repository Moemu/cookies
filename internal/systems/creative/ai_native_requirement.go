package creative

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var (
	ErrInvalidAINativeRequirement     = errors.New("invalid AI native requirement")
	ErrAINativeProductUnavailable     = errors.New("AI native product is unavailable")
	ErrAINativeProductLinkIncomplete  = errors.New("AI native product link is incomplete")
	ErrAINativeProductLinkUnsupported = errors.New("AI native product link is unsupported")
	ErrAINativeProductDetailMissing   = errors.New("AI native product detail is missing")
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
	Source           string                 `json:"source"`
	ProductID        string                 `json:"product_id"`
	Name             string                 `json:"name"`
	Description      string                 `json:"description"`
	Images           []AINativeProductImage `json:"images"`
	Price            AINativeProductPrice   `json:"price"`
	Sales            int64                  `json:"sales"`
	SourceURL        string                 `json:"source_url"`
	ResolutionStatus string                 `json:"resolution_status,omitempty"`
	ResourceType     string                 `json:"resource_type,omitempty"`
	MissingFields    []string               `json:"missing_fields,omitempty"`
}

type AINativeProductResolver interface {
	Resolve(context.Context, string) (AINativeProductSnapshot, error)
}

type AINativeProductMediaImporter interface {
	ImportProductMedia(context.Context, contract.ActorContext, contract.ProjectID, string, []AINativeRequirementMedia) ([]AINativeRequirementMedia, error)
}

type ResolveAINativeProductPreviewRequest struct {
	ProductLink string `json:"product_link"`
}

type AINativeProductPreview struct {
	ProductID     string   `json:"product_id"`
	ProductName   string   `json:"product_name"`
	Source        string   `json:"source"`
	SourceURL     string   `json:"source_url"`
	Status        string   `json:"status"`
	ResourceType  string   `json:"resource_type"`
	MissingFields []string `json:"missing_fields"`
}

type AnalyzeAINativeRequirementRequest struct {
	ProductLink             string                     `json:"product_link"`
	SupplementalRequirement string                     `json:"supplemental_requirement"`
	Channel                 string                     `json:"channel,omitempty"`
	AspectRatio             string                     `json:"aspect_ratio,omitempty"`
	DurationSeconds         int                        `json:"duration_seconds,omitempty"`
	Language                string                     `json:"language,omitempty"`
	OutputPresetID          string                     `json:"output_preset_id,omitempty"`
	DeliveryTreatment       *AINativeDeliveryTreatment `json:"delivery_treatment,omitempty"`
	outputPreset            AINativeOutputPresetSnapshot
}

func (r AnalyzeAINativeRequirementRequest) normalized() (AnalyzeAINativeRequirementRequest, error) {
	return r.normalizedWith(NewOutputPresetRegistry(NewChannelCreativeProfileRegistry()))
}

func (r AnalyzeAINativeRequirementRequest) normalizedWith(registry OutputPresetRegistry) (AnalyzeAINativeRequirementRequest, error) {
	r.ProductLink = strings.TrimSpace(r.ProductLink)
	r.SupplementalRequirement = strings.TrimSpace(r.SupplementalRequirement)
	r.Channel = strings.TrimSpace(r.Channel)
	r.AspectRatio = strings.TrimSpace(r.AspectRatio)
	r.Language = strings.TrimSpace(r.Language)
	r.OutputPresetID = strings.TrimSpace(r.OutputPresetID)
	if r.DurationSeconds == 0 {
		r.DurationSeconds = 20
	}
	if r.Language == "" {
		r.Language = "zh-CN"
	}
	if r.OutputPresetID == "" {
		r.OutputPresetID = AINativeOutputPresetDouyinFeed9x16V1
	}
	preset, err := registry.Resolve(r.OutputPresetID)
	if err != nil {
		return r, fmt.Errorf("%w: output preset is unavailable", ErrInvalidAINativeRequirement)
	}
	if r.Channel != "" && r.Channel != preset.Channel {
		return r, fmt.Errorf("%w: channel conflicts with output_preset_id", ErrInvalidAINativeRequirement)
	}
	if r.AspectRatio != "" && r.AspectRatio != preset.AspectRatio {
		return r, fmt.Errorf("%w: aspect_ratio conflicts with output_preset_id", ErrInvalidAINativeRequirement)
	}
	r.Channel, r.AspectRatio, r.outputPreset = preset.Channel, preset.AspectRatio, preset
	if r.DeliveryTreatment == nil {
		value := DefaultAINativeDeliveryTreatment()
		r.DeliveryTreatment = &value
	}
	if r.ProductLink == "" {
		return r, fmt.Errorf("%w: product_link is required", ErrInvalidAINativeRequirement)
	}
	if len([]rune(r.SupplementalRequirement)) > 2000 {
		return r, fmt.Errorf("%w: supplemental_requirement must not exceed 2000 characters", ErrInvalidAINativeRequirement)
	}
	if r.DurationSeconds < 15 || r.DurationSeconds > 30 {
		return r, fmt.Errorf("%w: duration_seconds must be between 15 and 30", ErrInvalidAINativeRequirement)
	}
	if r.Language != "zh-CN" {
		return r, fmt.Errorf("%w: only zh-CN is enabled in P0", ErrInvalidAINativeRequirement)
	}
	if err := r.DeliveryTreatment.Validate(); err != nil {
		return r, fmt.Errorf("%w: %v", ErrInvalidAINativeRequirement, err)
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
	ContractVersion         string                       `json:"contract_version"`
	Revision                int64                        `json:"revision"`
	Status                  string                       `json:"status"`
	Product                 AINativeProductSnapshot      `json:"product"`
	ProductResolution       AINativeProductResolution    `json:"product_resolution,omitempty"`
	ProductName             string                       `json:"product_name"`
	ProductDescription      string                       `json:"product_description"`
	TargetAudiences         []AINativeEditableText       `json:"target_audiences"`
	Media                   []AINativeRequirementMedia   `json:"media"`
	CoreSellingPoints       []AINativeEditableText       `json:"core_selling_points"`
	SupplementalRequirement string                       `json:"supplemental_requirement"`
	Channel                 string                       `json:"channel"`
	AspectRatio             string                       `json:"aspect_ratio"`
	DurationSeconds         int                          `json:"duration_seconds"`
	Language                string                       `json:"language"`
	OutputPreset            AINativeOutputPresetSnapshot `json:"output_preset,omitempty"`
	DeliveryTreatment       AINativeDeliveryTreatment    `json:"delivery_treatment,omitempty"`
	NeedsConfirmation       []string                     `json:"needs_confirmation"`
	Generation              AINativeGenerationMetadata   `json:"generation"`
}

func (d AINativeRequirementDraft) Validate() error { return d.ValidateStructure() }

func (d AINativeRequirementDraft) ValidateStructure() error {
	legacy := d.ContractVersion == aiNativeRequirementContractV1
	if !legacy && d.ContractVersion != aiNativeRequirementContractV2 {
		return fmt.Errorf("AI native requirement contract is invalid")
	}
	if d.Revision < 1 || d.Status != "draft" || len(d.TargetAudiences) > 10 || len(d.CoreSellingPoints) > 20 ||
		strings.TrimSpace(d.Channel) == "" || strings.TrimSpace(d.AspectRatio) == "" || d.DurationSeconds < 15 || d.DurationSeconds > 30 || d.Language != "zh-CN" ||
		len([]rune(d.SupplementalRequirement)) > 2000 || strings.TrimSpace(d.Generation.Mode) == "" ||
		strings.TrimSpace(d.Generation.ModelAlias) == "" || strings.TrimSpace(d.Generation.ModelVersion) == "" || strings.TrimSpace(d.Generation.PromptVersion) == "" {
		return fmt.Errorf("AI native requirement draft is invalid")
	}
	if legacy {
		if d.Channel != "douyin" || d.AspectRatio != "9:16" || strings.TrimSpace(d.Product.ProductID) == "" || strings.TrimSpace(d.ProductName) == "" || strings.TrimSpace(d.ProductDescription) == "" ||
			len(d.TargetAudiences) == 0 || len(d.CoreSellingPoints) == 0 {
			return fmt.Errorf("legacy AI native requirement draft is invalid")
		}
	} else {
		if !isSupportedAINativeProductSource(d.Product.Source) || strings.TrimSpace(d.Product.SourceURL) == "" || d.ProductResolution.Validate() != nil ||
			d.ProductResolution.Source != d.Product.Source || d.ProductResolution.SourceURL != d.Product.SourceURL ||
			d.OutputPreset.Validate() != nil || d.DeliveryTreatment.Validate() != nil || d.Channel != d.OutputPreset.Channel || d.AspectRatio != d.OutputPreset.AspectRatio {
			return fmt.Errorf("AI native requirement v2 configuration is invalid")
		}
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
		hasExternalURL := err == nil && parsed.Scheme == "https" && parsed.Hostname() != ""
		if strings.TrimSpace(item.ID) == "" || (item.AssetRef == nil && !hasExternalURL) ||
			(item.AssetRef != nil && strings.TrimSpace(item.URL) != "" && !hasExternalURL) || strings.TrimSpace(item.Role) == "" || strings.TrimSpace(item.Source) == "" {
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

func (d *AINativeRequirementDraft) ReconcileProductResolution() {
	if d == nil || d.ContractVersion != aiNativeRequirementContractV2 {
		return
	}
	missing := make([]string, 0, len(d.ProductResolution.MissingFields))
	for _, field := range d.ProductResolution.MissingFields {
		switch field {
		case "product_name":
			if strings.TrimSpace(d.ProductName) != "" {
				continue
			}
		case "images":
			hasAsset := false
			for _, item := range d.Media {
				if item.AssetRef != nil && item.AssetRef.Validate() == nil {
					hasAsset = true
					break
				}
			}
			if hasAsset {
				continue
			}
		case "core_selling_points":
			if len(cleanTextListFromEditable(d.CoreSellingPoints)) > 0 {
				continue
			}
		case "target_audiences":
			if len(cleanTextListFromEditable(d.TargetAudiences)) > 0 {
				continue
			}
		}
		missing = appendUniqueText(missing, field)
	}
	d.ProductResolution.MissingFields = missing
	hasConfirmationBlocker := false
	for _, field := range missing {
		if field == "product_name" || field == "images" || field == "core_selling_points" || field == "target_audiences" {
			hasConfirmationBlocker = true
			break
		}
	}
	if len(missing) == 0 {
		d.ProductResolution.Status = AINativeProductResolutionRecognized
	} else if hasConfirmationBlocker {
		d.ProductResolution.Status = AINativeProductResolutionManualRequired
	} else {
		d.ProductResolution.Status = AINativeProductResolutionPartial
	}
}

func (d AINativeRequirementDraft) ValidateForConfirmation() []AINativeRequirementFieldIssue {
	issues := []AINativeRequirementFieldIssue{}
	appendIssue := func(field, code, message string) {
		issues = append(issues, AINativeRequirementFieldIssue{Field: field, Code: code, Message: message})
	}
	if strings.TrimSpace(d.ProductName) == "" {
		appendIssue("product_name", "PRODUCT_NAME_REQUIRED", "请补充商品名称")
	}
	hasProductAsset := false
	for _, media := range d.Media {
		if media.AssetRef != nil && media.AssetRef.Validate() == nil {
			hasProductAsset = true
			break
		}
	}
	if !hasProductAsset {
		appendIssue("media", "PRODUCT_IMAGE_REQUIRED", "链接没有权限提取，需要用户手动上传")
	}
	if len(cleanTextListFromEditable(d.CoreSellingPoints)) == 0 {
		appendIssue("core_selling_points", "SELLING_POINT_REQUIRED", "请至少确认一条核心卖点")
	}
	if len(cleanTextListFromEditable(d.TargetAudiences)) == 0 {
		appendIssue("target_audiences", "TARGET_AUDIENCE_REQUIRED", "请至少确认一个目标受众")
	}
	return issues
}

func cleanTextListFromEditable(items []AINativeEditableText) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Text) != "" {
			result = append(result, strings.TrimSpace(item.Text))
		}
	}
	return result
}

func (s Service) AnalyzeAINativeRequirement(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request AnalyzeAINativeRequirementRequest) (AINativeRequirementWorkspace, error) {
	if s.Projects == nil || s.AINativeProducts == nil || s.AINativeRequirementPlanner == nil || s.AINativeRequirements == nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("AI native requirement dependencies are incomplete")
	}
	if !actor.HasScope(ScopeWrite) {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%s scope is required", ScopeWrite)
	}
	normalized, err := request.normalizedWith(s.outputPresetRegistry())
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	project, err := s.Projects.RequireActiveContext(ctx, actor, projectID)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	product, err := s.AINativeProducts.Resolve(ctx, normalized.ProductLink)
	if err != nil {
		return AINativeRequirementWorkspace{}, fmt.Errorf("%w: %w", ErrAINativeProductUnavailable, err)
	}
	draft, err := s.AINativeRequirementPlanner.Analyze(ctx, actor, project, product, normalized)
	if err != nil {
		return AINativeRequirementWorkspace{}, err
	}
	draft.ReconcileProductResolution()
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
			draft.Media = media
			if len(media) == 0 {
				draft.Product.Images = []AINativeProductImage{}
				draft.ProductResolution.MissingFields = appendUniqueText(draft.ProductResolution.MissingFields, "images")
			}
			draft.NeedsConfirmation = appendUniqueText(draft.NeedsConfirmation, "链接没有权限提取，需要用户手动上传")
		} else {
			draft.Media = media
		}
		for imageIndex := range draft.Product.Images {
			for _, item := range media {
				if item.URL == draft.Product.Images[imageIndex].URL {
					draft.Product.Images[imageIndex].AssetRef = item.AssetRef
					break
				}
			}
		}
		draft.ReconcileProductResolution()
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

func (s Service) ResolveAINativeProductPreview(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request ResolveAINativeProductPreviewRequest) (AINativeProductPreview, error) {
	if s.Projects == nil || s.AINativeProducts == nil {
		return AINativeProductPreview{}, fmt.Errorf("AI native product preview dependencies are incomplete")
	}
	if !actor.HasScope(ScopeRead) {
		return AINativeProductPreview{}, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return AINativeProductPreview{}, err
	}
	product, err := s.AINativeProducts.Resolve(ctx, strings.TrimSpace(request.ProductLink))
	if err != nil {
		return AINativeProductPreview{}, fmt.Errorf("%w: %w", ErrAINativeProductUnavailable, err)
	}
	if strings.TrimSpace(product.ProductID) == "" || strings.TrimSpace(product.Source) == "" || strings.TrimSpace(product.SourceURL) == "" {
		return AINativeProductPreview{}, fmt.Errorf("%w: product identity is required", ErrAINativeProductUnavailable)
	}
	status := product.ResolutionStatus
	if status == "" {
		status = AINativeProductResolutionRecognized
	}
	resourceType := product.ResourceType
	if resourceType == "" {
		resourceType = AINativeProductResourceProduct
	}
	return AINativeProductPreview{
		ProductID:   product.ProductID,
		ProductName: product.Name,
		Source:      product.Source,
		SourceURL:   product.SourceURL,
		Status:      status, ResourceType: resourceType, MissingFields: append([]string{}, product.MissingFields...),
	}, nil
}

func appendUniqueText(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
