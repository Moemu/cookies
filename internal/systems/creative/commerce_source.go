package creative

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type CreativeSourceKind string

const (
	CreativeSourceConfirmedBrief CreativeSourceKind = "confirmed_brief"
	CreativeSourceStrategy       CreativeSourceKind = "strategy_package"
)

type CreativeSourceReference struct {
	Kind        CreativeSourceKind `json:"kind"`
	ID          string             `json:"id"`
	Version     int64              `json:"version"`
	ContentHash string             `json:"content_hash"`
}

func (r CreativeSourceReference) Validate() error {
	switch r.Kind {
	case CreativeSourceConfirmedBrief, CreativeSourceStrategy:
	default:
		return fmt.Errorf("unsupported creative source kind %q", r.Kind)
	}
	if strings.TrimSpace(r.ID) == "" || r.Version < 1 || strings.TrimSpace(r.ContentHash) == "" {
		return fmt.Errorf("creative source id, version, and content_hash are required")
	}
	return nil
}

type CommerceProductFacts struct {
	BrandName       string                     `json:"brand_name"`
	ProductName     string                     `json:"product_name"`
	ProductCategory string                     `json:"product_category,omitempty"`
	SellingPoints   []string                   `json:"selling_points"`
	Tone            []string                   `json:"tone"`
	VisualKeywords  []string                   `json:"visual_keywords"`
	Mandatory       []string                   `json:"mandatory_elements"`
	Prohibited      []string                   `json:"prohibited_claims"`
	ProductAssets   []contract.AssetVersionRef `json:"product_asset_refs"`
}

type CreativeSourceOption struct {
	SourceRef   CreativeSourceReference `json:"source_ref"`
	Status      string                  `json:"status"`
	Product     CommerceProductFacts    `json:"product"`
	ConfirmedAt time.Time               `json:"confirmed_at"`
	Preferred   bool                    `json:"preferred"`
}

type CreativeSourceSnapshot struct {
	SourceRef CreativeSourceReference
	Product   CommerceProductFacts
}

// CreativeSourceReader is the single Strategy-to-Creative seam for selectable,
// immutable source versions. Implementations authorize every read and return
// Creative vocabulary rather than exposing Strategy persistence models.
type CreativeSourceReader interface {
	ListCreativeSources(context.Context, contract.ActorContext, contract.ProjectID) ([]CreativeSourceOption, error)
	ReadCreativeSource(context.Context, contract.ActorContext, contract.ProjectID, CreativeSourceReference) (CreativeSourceSnapshot, error)
}

type PrepareCommercePrerollRequest struct {
	SourceRef    CreativeSourceReference   `json:"source_ref"`
	Template     TemplateReference         `json:"template_ref"`
	ProductAsset *contract.AssetVersionRef `json:"product_asset_ref,omitempty"`
}

func (r PrepareCommercePrerollRequest) Validate() error {
	if err := r.SourceRef.Validate(); err != nil {
		return err
	}
	if !supportedCommerceTemplate(r.Template.ID) || r.Template.Version != 1 {
		return fmt.Errorf("unsupported commerce preroll template")
	}
	if r.ProductAsset != nil {
		if err := r.ProductAsset.Validate(); err != nil {
			return fmt.Errorf("product_asset_ref: %w", err)
		}
	}
	return nil
}

type CommercePrerollReadiness struct {
	PlanningReady   bool     `json:"planning_ready"`
	GenerationReady bool     `json:"generation_ready"`
	Blockers        []string `json:"blockers"`
	Warnings        []string `json:"warnings"`
}

type PreparedCommercePreroll struct {
	ContractVersion string                   `json:"contract_version"`
	SourceRef       CreativeSourceReference  `json:"source_ref"`
	Product         CommerceProductFacts     `json:"product"`
	Plan            CommercePrerollPlan      `json:"plan"`
	Readiness       CommercePrerollReadiness `json:"readiness"`
	PreparedAt      time.Time                `json:"prepared_at"`
}

func (s Service) ListCommercePrerollSources(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
) ([]CreativeSourceOption, error) {
	if s.Projects == nil || s.Sources == nil {
		return nil, fmt.Errorf("creative source selection is unavailable")
	}
	if !actor.HasScope(ScopeRead) {
		return nil, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.Sources.ListCreativeSources(ctx, actor, projectID)
}

func (s Service) PrepareCommercePreroll(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	request PrepareCommercePrerollRequest,
) (PreparedCommercePreroll, error) {
	if s.Projects == nil || s.Sources == nil {
		return PreparedCommercePreroll{}, fmt.Errorf("creative source selection is unavailable")
	}
	if !actor.HasScope(ScopeRead) {
		return PreparedCommercePreroll{}, fmt.Errorf("%s scope is required", ScopeRead)
	}
	if err := request.Validate(); err != nil {
		return PreparedCommercePreroll{}, err
	}
	if _, err := s.Projects.RequireActiveContext(ctx, actor, projectID); err != nil {
		return PreparedCommercePreroll{}, err
	}
	source, err := s.Sources.ReadCreativeSource(ctx, actor, projectID, request.SourceRef)
	if err != nil {
		return PreparedCommercePreroll{}, err
	}
	if source.SourceRef != request.SourceRef {
		return PreparedCommercePreroll{}, fmt.Errorf("resolved creative source does not match the selected immutable version")
	}
	productAsset := contract.AssetVersionRef{}
	if request.ProductAsset != nil {
		if !containsAssetRef(source.Product.ProductAssets, *request.ProductAsset) {
			return PreparedCommercePreroll{}, fmt.Errorf("product_asset_ref is not part of the selected source snapshot")
		}
		productAsset = *request.ProductAsset
	} else if len(source.Product.ProductAssets) > 0 {
		productAsset = source.Product.ProductAssets[0]
	}
	taskID := fmt.Sprintf("commerce-preview:%s:%d:%s", request.SourceRef.ID, request.SourceRef.Version, request.Template.ID)
	plan, err := (CommercePrerollPlanner{}).Plan(CommercePrerollPlanningInput{
		TaskID:             taskID,
		IntakeVersion:      request.SourceRef.Version,
		TemplateID:         request.Template.ID,
		TemplateVersion:    request.Template.Version,
		BrandName:          source.Product.BrandName,
		ProductName:        source.Product.ProductName,
		ProductCategory:    source.Product.ProductCategory,
		SellingPoints:      append([]string{}, source.Product.SellingPoints...),
		VisualKeywords:     append([]string{}, source.Product.VisualKeywords...),
		ProductAsset:       productAsset,
		DurationSeconds:    6,
		AspectRatio:        "9:16",
		Resolution:         "720p",
		AudioPolicy:        VideoAudioSilent,
		MandatoryElements:  append([]string{}, source.Product.Mandatory...),
		ProhibitedElements: append([]string{}, source.Product.Prohibited...),
	})
	if err != nil {
		return PreparedCommercePreroll{}, err
	}
	blockers := make([]string, 0, 1)
	if productAsset == (contract.AssetVersionRef{}) {
		blockers = append(blockers, "PRODUCT_IMAGE_MISSING")
	}
	return PreparedCommercePreroll{
		ContractVersion: "creative-commerce-preroll-preparation/v1",
		SourceRef:       source.SourceRef,
		Product:         source.Product,
		Plan:            plan,
		Readiness: CommercePrerollReadiness{
			PlanningReady: true, GenerationReady: len(blockers) == 0,
			Blockers: blockers, Warnings: []string{},
		},
		PreparedAt: s.now(),
	}, nil
}

func containsAssetRef(values []contract.AssetVersionRef, target contract.AssetVersionRef) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
