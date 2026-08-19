package project

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/ids"
)

var ErrNotFound = errors.New("project not found")
var ErrNotActive = errors.New("project is not active")
var ErrBrandNotFound = errors.New("brand not found")
var ErrProductNotFound = errors.New("product not found")
var ErrMembershipNotFound = errors.New("project membership not found")
var ErrMembershipForbidden = errors.New("project membership operation forbidden")
var ErrMembershipConflict = errors.New("project membership changed")
var ErrLastOwner = errors.New("project must keep an active owner")
var ErrProductMappingConflict = errors.New("ocean engine product ID is already bound")

type Store interface {
	CreateBrand(context.Context, Brand) error
	CreateProduct(context.Context, Product) error
	ListProducts(context.Context, contract.OrganizationID) ([]Product, error)
	GetProduct(context.Context, contract.OrganizationID, contract.ProductID) (Product, error)
	UpdateProduct(context.Context, Product) error
	ListProductProjects(context.Context, contract.OrganizationID, contract.ProductID) ([]ProductProjectRef, error)
	LinkProductToProject(context.Context, contract.OrganizationID, contract.ProjectID, contract.ProductID) error
	DeleteProduct(context.Context, contract.OrganizationID, contract.ProductID) error
	CreateProject(context.Context, Project, contract.Principal, []contract.ProductID) error
	UpdateProject(context.Context, Project, ProjectRuntime, int64) error
	CreateProjectArtifact(context.Context, ProjectArtifact) error
	ListProjectArtifacts(context.Context, contract.OrganizationID, contract.ProjectID) ([]ProjectArtifact, error)
	GetProjectArtifact(context.Context, contract.OrganizationID, contract.ProjectID, string) (ProjectArtifact, error)
	UpdateProjectArtifact(context.Context, ProjectArtifact, int64) error
	GetProject(context.Context, contract.OrganizationID, contract.ProjectID) (Project, error)
	GetProjectRuntime(context.Context, contract.OrganizationID, contract.ProjectID) (ProjectRuntime, error)
	UpsertProjectRuntime(context.Context, contract.OrganizationID, contract.ProjectID, ProjectRuntime) error
	GetWorkbench(context.Context, contract.OrganizationID, contract.ProjectID) (Workbench, error)
	UpsertWorkbench(context.Context, Workbench) error
	GetContext(context.Context, contract.OrganizationID, contract.ProjectID) (contract.ProjectContext, error)
	ListProjects(context.Context, contract.ActorContext) ([]Project, error)
	CreateBusinessTask(context.Context, BusinessTask) error
	ListBusinessTasks(context.Context, contract.OrganizationID, contract.ProjectID) ([]BusinessTask, error)
	GetBusinessTask(context.Context, contract.OrganizationID, contract.ProjectID, string) (BusinessTask, error)
	UpdateBusinessTask(context.Context, BusinessTask) error
	CreateOperationalRecord(context.Context, OperationalRecord) error
	ListOperationalRecords(context.Context, contract.OrganizationID, contract.ProjectID) ([]OperationalRecord, error)
	GetOperationalRecord(context.Context, contract.OrganizationID, contract.ProjectID, string) (OperationalRecord, error)
	UpdateOperationalRecord(context.Context, OperationalRecord) error
	DeleteOperationalRecord(context.Context, contract.OrganizationID, contract.ProjectID, string) error
	CreateChangeSet(context.Context, ChangeSet) error
	ListChangeSets(context.Context, contract.OrganizationID, contract.ProjectID) ([]ChangeSet, error)
	GetChangeSet(context.Context, contract.OrganizationID, contract.ProjectID, string) (ChangeSet, error)
	UpdateChangeSet(context.Context, ChangeSet) error
	AppendChangeSetEvent(context.Context, ChangeSetEvent) error
	AppendAuditEvent(context.Context, AuditEvent) error
	ListAuditEvents(context.Context, contract.OrganizationID, contract.ProjectID) ([]AuditEvent, error)
}

type Service struct {
	Store      Store
	Authorizer identity.ProjectAuthorizer
	NewID      ids.Generator
}

func (s Service) CreateBrand(ctx context.Context, actor contract.ActorContext, name string) (Brand, error) {
	if s.Store == nil {
		return Brand{}, fmt.Errorf("project store is required")
	}
	if err := actor.Validate(); err != nil {
		return Brand{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 255 {
		return Brand{}, fmt.Errorf("brand name must be between 1 and 255 characters")
	}
	newID := s.NewID
	if newID == nil {
		newID = ids.New
	}
	id, err := newID("brand")
	if err != nil {
		return Brand{}, err
	}
	brand := Brand{ID: contract.BrandID(id), OrganizationID: actor.OrganizationID, Name: name, Status: "active"}
	if err := s.Store.CreateBrand(ctx, brand); err != nil {
		return Brand{}, err
	}
	return brand, nil
}

// CreateProduct creates an organization-level product object. Category
// selects the OceanEngine product kind (ordinary product or promotional
// activity); product-only and activity-only fields are validated against it.
// The OceanEngine mapping is bound later by the launch pipeline, so a new
// product always starts without a platform ID.
func (s Service) CreateProduct(ctx context.Context, actor contract.ActorContext, request CreateProductRequest) (Product, error) {
	if s.Store == nil {
		return Product{}, fmt.Errorf("project store is required")
	}
	if err := actor.Validate(); err != nil {
		return Product{}, err
	}
	name := strings.TrimSpace(request.Name)
	if name == "" || len(name) > 255 {
		return Product{}, fmt.Errorf("product name must be between 1 and 255 characters")
	}
	category := contract.ProductCategory(strings.TrimSpace(string(request.Category)))
	if category == "" {
		category = contract.ProductCategoryProduct
	}
	if category != contract.ProductCategoryProduct && category != contract.ProductCategoryActivity {
		return Product{}, fmt.Errorf("product category must be product or activity")
	}
	if request.BrandType != "" && request.BrandType != contract.BrandTypeStandard && request.BrandType != contract.BrandTypeCustom {
		return Product{}, fmt.Errorf("brand_type must be standard or custom")
	}
	if request.PriceBand != "" && !validPriceBand(request.PriceBand) {
		return Product{}, fmt.Errorf("price_band is not a supported OceanEngine price tier")
	}
	if category == contract.ProductCategoryActivity {
		activityType := strings.TrimSpace(request.ActivityType)
		if activityType == "" {
			return Product{}, fmt.Errorf("activity_type is required for activity products")
		}
		if activityType != "red_packet" {
			return Product{}, fmt.Errorf("activity_type currently supports only red_packet")
		}
	}
	for field, value := range map[string]string{
		"activity_type": request.ActivityType,
		"activity_name": request.ActivityName,
		"brand_name":    request.BrandName,
		"description":   request.Description,
		"product_image": request.ProductImage,
	} {
		if len(value) > 2000 {
			return Product{}, fmt.Errorf("%s must not exceed 2000 characters", field)
		}
	}
	newID := s.NewID
	if newID == nil {
		newID = ids.New
	}
	id, err := newID("product")
	if err != nil {
		return Product{}, err
	}
	product := Product{
		ID: contract.ProductID(id), OrganizationID: actor.OrganizationID, Name: name, Category: category, Status: "active",
		ProductImage: strings.TrimSpace(request.ProductImage), PriceBand: request.PriceBand,
		ActivityType: strings.TrimSpace(request.ActivityType), ActivityName: strings.TrimSpace(request.ActivityName),
		BrandType: request.BrandType, BrandName: strings.TrimSpace(request.BrandName), Description: strings.TrimSpace(request.Description),
		OceanEngineProductID: strings.TrimSpace(request.OceanEngineProductID),
	}
	if err := s.Store.CreateProduct(ctx, product); err != nil {
		return Product{}, err
	}
	created, err := s.Store.GetProduct(ctx, actor.OrganizationID, product.ID)
	if err != nil {
		return Product{}, err
	}
	return created, nil
}

func validPriceBand(value contract.ProductPriceBand) bool {
	switch value {
	case contract.PriceBand0To9, contract.PriceBand10To99, contract.PriceBand100To999,
		contract.PriceBand1000To9999, contract.PriceBand10000To99999, contract.PriceBand100000Plus:
		return true
	default:
		return false
	}
}

func (s Service) ListProducts(ctx context.Context, actor contract.ActorContext) ([]Product, error) {
	if s.Store == nil {
		return nil, fmt.Errorf("project store is required")
	}
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	return s.Store.ListProducts(ctx, actor.OrganizationID)
}

func (s Service) GetProduct(ctx context.Context, actor contract.ActorContext, productID contract.ProductID) (Product, error) {
	if s.Store == nil {
		return Product{}, fmt.Errorf("project store is required")
	}
	if err := actor.Validate(); err != nil {
		return Product{}, err
	}
	if productID == "" {
		return Product{}, fmt.Errorf("product id must not be empty")
	}
	return s.Store.GetProduct(ctx, actor.OrganizationID, productID)
}

// UpdateProduct applies optional field updates. Empty strings clear the
// corresponding optional column; nil pointers keep the existing value.
func (s Service) UpdateProduct(ctx context.Context, actor contract.ActorContext, productID contract.ProductID, request UpdateProductRequest) (Product, error) {
	if s.Store == nil {
		return Product{}, fmt.Errorf("project store is required")
	}
	if err := actor.Validate(); err != nil {
		return Product{}, err
	}
	if productID == "" {
		return Product{}, fmt.Errorf("product id must not be empty")
	}
	product, err := s.Store.GetProduct(ctx, actor.OrganizationID, productID)
	if err != nil {
		return Product{}, err
	}
	if request.Name != nil {
		product.Name = strings.TrimSpace(*request.Name)
	}
	if request.Category != nil {
		category := contract.ProductCategory(strings.TrimSpace(string(*request.Category)))
		if category != contract.ProductCategoryProduct && category != contract.ProductCategoryActivity {
			return Product{}, fmt.Errorf("product category must be product or activity")
		}
		product.Category = category
	}
	if request.Status != nil {
		status := strings.TrimSpace(*request.Status)
		if status != "active" && status != "archived" {
			return Product{}, fmt.Errorf("product status must be active or archived")
		}
		product.Status = status
	}
	if request.ProductImage != nil {
		product.ProductImage = strings.TrimSpace(*request.ProductImage)
	}
	if request.PriceBand != nil {
		band := contract.ProductPriceBand(strings.TrimSpace(string(*request.PriceBand)))
		if band != "" && !validPriceBand(band) {
			return Product{}, fmt.Errorf("price_band is not a supported OceanEngine price tier")
		}
		product.PriceBand = band
	}
	if request.ActivityType != nil {
		activityType := strings.TrimSpace(*request.ActivityType)
		if product.Category == contract.ProductCategoryActivity && activityType != "" && activityType != "red_packet" {
			return Product{}, fmt.Errorf("activity_type currently supports only red_packet")
		}
		product.ActivityType = activityType
	}
	if request.ActivityName != nil {
		product.ActivityName = strings.TrimSpace(*request.ActivityName)
	}
	if request.BrandType != nil {
		brandType := contract.BrandType(strings.TrimSpace(string(*request.BrandType)))
		if brandType != "" && brandType != contract.BrandTypeStandard && brandType != contract.BrandTypeCustom {
			return Product{}, fmt.Errorf("brand_type must be standard or custom")
		}
		product.BrandType = brandType
	}
	if request.BrandName != nil {
		product.BrandName = strings.TrimSpace(*request.BrandName)
	}
	if request.Description != nil {
		product.Description = strings.TrimSpace(*request.Description)
	}
	if request.OceanEngineProductID != nil {
		product.OceanEngineProductID = strings.TrimSpace(*request.OceanEngineProductID)
	}
	if product.Name == "" {
		return Product{}, fmt.Errorf("product name must not be empty")
	}
	if err := s.Store.UpdateProduct(ctx, product); err != nil {
		return Product{}, err
	}
	return product, nil
}

func (s Service) ListProductProjects(ctx context.Context, actor contract.ActorContext, productID contract.ProductID) ([]ProductProjectRef, error) {
	if s.Store == nil {
		return nil, fmt.Errorf("project store is required")
	}
	if err := actor.Validate(); err != nil {
		return nil, err
	}
	if productID == "" {
		return nil, fmt.Errorf("product id must not be empty")
	}
	return s.Store.ListProductProjects(ctx, actor.OrganizationID, productID)
}

// LinkProductToProject associates an organization-level product with a
// project so the project's delivery forms can offer it in the cookies
// product dropdown. The association is idempotent.
func (s Service) LinkProductToProject(ctx context.Context, actor contract.ActorContext, productID contract.ProductID, projectID contract.ProjectID) error {
	if s.Store == nil {
		return fmt.Errorf("project store is required")
	}
	if err := actor.Validate(); err != nil {
		return err
	}
	if productID == "" || projectID == "" {
		return fmt.Errorf("product id and project id must not be empty")
	}
	if _, err := s.Store.GetProduct(ctx, actor.OrganizationID, productID); err != nil {
		return err
	}
	if _, err := s.Store.GetProject(ctx, actor.OrganizationID, projectID); err != nil {
		return err
	}
	return s.Store.LinkProductToProject(ctx, actor.OrganizationID, projectID, productID)
}

func (s Service) DeleteProduct(ctx context.Context, actor contract.ActorContext, productID contract.ProductID) error {
	if s.Store == nil {
		return fmt.Errorf("project store is required")
	}
	if err := actor.Validate(); err != nil {
		return err
	}
	if productID == "" {
		return fmt.Errorf("product id must not be empty")
	}
	return s.Store.DeleteProduct(ctx, actor.OrganizationID, productID)
}

func (s Service) CreateProject(ctx context.Context, actor contract.ActorContext, request CreateProjectRequest) (Project, error) {
	if s.Store == nil {
		return Project{}, fmt.Errorf("project store is required")
	}
	if err := actor.Validate(); err != nil {
		return Project{}, err
	}
	if err := request.Validate(); err != nil {
		return Project{}, err
	}
	request.Name = strings.TrimSpace(request.Name)
	newID := s.NewID
	if newID == nil {
		newID = ids.New
	}
	id, err := newID("project")
	if err != nil {
		return Project{}, err
	}
	status := StatusDraft
	if request.Activate {
		status = StatusActive
	}
	industry := request.Industry
	if industry == "" {
		industry = IndustryEcommerce
	}
	project := Project{
		ID: contract.ProjectID(id), OrganizationID: actor.OrganizationID, Name: request.Name, Status: status,
		Industry: industry, PrimaryBrandID: request.PrimaryBrandID, ProjectContextVersion: 1,
	}
	if err := s.Store.CreateProject(ctx, project, actor.Principal, request.ProductIDs); err != nil {
		return Project{}, err
	}
	if strings.TrimSpace(request.Brand) != "" || strings.TrimSpace(request.Goal) != "" {
		runtime, err := s.Store.GetProjectRuntime(ctx, actor.OrganizationID, project.ID)
		if err != nil {
			return Project{}, err
		}
		if strings.TrimSpace(request.Brand) != "" {
			runtime.Brand = strings.TrimSpace(request.Brand)
		}
		if strings.TrimSpace(request.Goal) != "" {
			runtime.Goal = strings.TrimSpace(request.Goal)
		}
		if err := s.Store.UpsertProjectRuntime(ctx, actor.OrganizationID, project.ID, runtime); err != nil {
			return Project{}, err
		}
	}
	return s.Store.GetProject(ctx, actor.OrganizationID, project.ID)
}

func (s Service) UpdateProject(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request UpdateProjectRequest) (Project, error) {
	if err := s.authorizeWorkflow(ctx, actor, projectID); err != nil {
		return Project{}, err
	}
	if err := request.Validate(); err != nil {
		return Project{}, err
	}
	current, err := s.Store.GetProject(ctx, actor.OrganizationID, projectID)
	if err != nil {
		return Project{}, err
	}
	if request.ExpectedContextVersion != nil && *request.ExpectedContextVersion != current.ProjectContextVersion {
		return Project{}, ErrVersionConflict
	}
	runtime, err := s.Store.GetProjectRuntime(ctx, actor.OrganizationID, projectID)
	if err != nil {
		return Project{}, err
	}
	if request.Name != nil {
		current.Name = strings.TrimSpace(*request.Name)
	}
	if request.Industry != nil {
		current.Industry = *request.Industry
	}
	if request.Brand != nil {
		runtime.Brand = strings.TrimSpace(*request.Brand)
	}
	if request.Goal != nil {
		runtime.Goal = strings.TrimSpace(*request.Goal)
	}
	if err := s.Store.UpdateProject(ctx, current, runtime, current.ProjectContextVersion); err != nil {
		return Project{}, err
	}
	return s.Store.GetProject(ctx, actor.OrganizationID, projectID)
}

func (s Service) GetContext(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectContext, error) {
	if s.Store == nil {
		return contract.ProjectContext{}, fmt.Errorf("project store is required")
	}
	if s.Authorizer == nil {
		return contract.ProjectContext{}, identity.ErrProjectAccessDenied
	}
	if err := s.Authorizer.AuthorizeProject(ctx, actor, projectID); err != nil {
		return contract.ProjectContext{}, err
	}
	return s.Store.GetContext(ctx, actor.OrganizationID, projectID)
}

// GetBusinessContext returns names only for cross-artifact compatibility
// checks. Authorization is identical to GetContext and the immutable IDs in
// ProjectContext remain the source of lineage truth.
func (s Service) GetBusinessContext(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectBusinessContext, error) {
	if s.Store == nil || s.Authorizer == nil {
		return contract.ProjectBusinessContext{}, identity.ErrProjectAccessDenied
	}
	if err := s.Authorizer.AuthorizeProject(ctx, actor, projectID); err != nil {
		return contract.ProjectBusinessContext{}, err
	}
	reader, ok := s.Store.(interface {
		GetBusinessContext(context.Context, contract.OrganizationID, contract.ProjectID) (contract.ProjectBusinessContext, error)
	})
	if !ok {
		return contract.ProjectBusinessContext{}, fmt.Errorf("project business context reader is required")
	}
	return reader.GetBusinessContext(ctx, actor.OrganizationID, projectID)
}

func (s Service) RequireActiveContext(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) (contract.ProjectContext, error) {
	if s.Store == nil || s.Authorizer == nil {
		return contract.ProjectContext{}, identity.ErrProjectAccessDenied
	}
	if err := s.Authorizer.AuthorizeProject(ctx, actor, projectID); err != nil {
		return contract.ProjectContext{}, err
	}
	project, err := s.Store.GetProject(ctx, actor.OrganizationID, projectID)
	if err != nil {
		return contract.ProjectContext{}, err
	}
	if project.Status != StatusActive || project.PrimaryBrandID == nil || project.PrimaryBrandStatus != "active" {
		return contract.ProjectContext{}, ErrNotActive
	}
	contextValue, err := s.Store.GetContext(ctx, actor.OrganizationID, projectID)
	if err != nil {
		return contract.ProjectContext{}, err
	}
	if err := contextValue.ValidateBrandBound(); err != nil {
		return contract.ProjectContext{}, ErrNotActive
	}
	return contextValue, nil
}

func (s Service) ListProjects(ctx context.Context, actor contract.ActorContext) ([]Project, error) {
	if s.Store == nil {
		return nil, fmt.Errorf("project store is required")
	}
	return s.Store.ListProjects(ctx, actor)
}
