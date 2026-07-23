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

type Store interface {
	CreateBrand(context.Context, Brand) error
	CreateProject(context.Context, Project, contract.Principal, []contract.ProductID) error
	GetProject(context.Context, contract.OrganizationID, contract.ProjectID) (Project, error)
	GetContext(context.Context, contract.OrganizationID, contract.ProjectID) (contract.ProjectContext, error)
	ListProjects(context.Context, contract.ActorContext) ([]Project, error)
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
	project := Project{
		ID: contract.ProjectID(id), OrganizationID: actor.OrganizationID, Name: request.Name, Status: status,
		PrimaryBrandID: request.PrimaryBrandID, ProjectContextVersion: 1,
	}
	if err := s.Store.CreateProject(ctx, project, actor.Principal, request.ProductIDs); err != nil {
		return Project{}, err
	}
	return s.Store.GetProject(ctx, actor.OrganizationID, project.ID)
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
