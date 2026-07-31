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

type Store interface {
	CreateBrand(context.Context, Brand) error
	CreateProject(context.Context, Project, contract.Principal, []contract.ProductID) error
	UpdateProject(context.Context, Project, ProjectRuntime, int64) error
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
