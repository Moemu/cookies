package project

import (
	"context"
	"errors"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
)

func TestRequireActiveContextFailsClosedBeforeReadingProject(t *testing.T) {
	store := &stubProjectStore{}
	service := Service{Store: store, Authorizer: denyingAuthorizer{}}
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "user_1"}, Scopes: []contract.Scope{}}
	_, err := service.RequireActiveContext(context.Background(), actor, "project_1")
	if !errors.Is(err, identity.ErrProjectAccessDenied) {
		t.Fatalf("error=%v", err)
	}
	if store.read {
		t.Fatal("project data was read before authorization")
	}
}

type denyingAuthorizer struct{}

func (denyingAuthorizer) AuthorizeProject(context.Context, contract.ActorContext, contract.ProjectID) error {
	return identity.ErrProjectAccessDenied
}
func (denyingAuthorizer) AuthorizeProjectAction(context.Context, contract.ActorContext, contract.ProjectID, string) error {
	return identity.ErrProjectAccessDenied
}

type stubProjectStore struct {
	read      bool
	workbench Workbench
	saved     Workbench
}

func (*stubProjectStore) CreateBrand(context.Context, Brand) error     { return nil }
func (*stubProjectStore) CreateProduct(context.Context, Product) error { return nil }
func (*stubProjectStore) ListProducts(context.Context, contract.OrganizationID) ([]Product, error) {
	return nil, nil
}
func (*stubProjectStore) GetProduct(context.Context, contract.OrganizationID, contract.ProductID) (Product, error) {
	return Product{}, ErrNotFound
}
func (*stubProjectStore) UpdateProduct(context.Context, Product) error { return nil }
func (*stubProjectStore) ListProductProjects(context.Context, contract.OrganizationID, contract.ProductID) ([]ProductProjectRef, error) {
	return nil, nil
}
func (*stubProjectStore) LinkProductToProject(context.Context, contract.OrganizationID, contract.ProjectID, contract.ProductID) error {
	return nil
}
func (*stubProjectStore) DeleteProduct(context.Context, contract.OrganizationID, contract.ProductID) error {
	return nil
}
func (*stubProjectStore) CreateProject(context.Context, Project, contract.Principal, []contract.ProductID) error {
	return nil
}
func (*stubProjectStore) UpdateProject(context.Context, Project, ProjectRuntime, int64) error {
	return nil
}
func (*stubProjectStore) CreateProjectArtifact(context.Context, ProjectArtifact) error { return nil }
func (*stubProjectStore) ListProjectArtifacts(context.Context, contract.OrganizationID, contract.ProjectID) ([]ProjectArtifact, error) {
	return nil, nil
}
func (*stubProjectStore) GetProjectArtifact(context.Context, contract.OrganizationID, contract.ProjectID, string) (ProjectArtifact, error) {
	return ProjectArtifact{}, ErrNotFound
}
func (*stubProjectStore) UpdateProjectArtifact(context.Context, ProjectArtifact, int64) error {
	return nil
}
func (s *stubProjectStore) GetProject(context.Context, contract.OrganizationID, contract.ProjectID) (Project, error) {
	s.read = true
	return Project{}, ErrNotFound
}
func (*stubProjectStore) GetProjectRuntime(context.Context, contract.OrganizationID, contract.ProjectID) (ProjectRuntime, error) {
	return ProjectRuntime{}, ErrNotFound
}
func (*stubProjectStore) UpsertProjectRuntime(context.Context, contract.OrganizationID, contract.ProjectID, ProjectRuntime) error {
	return nil
}
func (s *stubProjectStore) GetWorkbench(context.Context, contract.OrganizationID, contract.ProjectID) (Workbench, error) {
	if s.workbench.Project.ProjectID == "" {
		return Workbench{}, ErrNotFound
	}
	return s.workbench, nil
}
func (s *stubProjectStore) UpsertWorkbench(_ context.Context, value Workbench) error {
	s.saved = value
	s.workbench = value
	return nil
}
func (s *stubProjectStore) GetContext(context.Context, contract.OrganizationID, contract.ProjectID) (contract.ProjectContext, error) {
	s.read = true
	return contract.ProjectContext{}, ErrNotFound
}
func (*stubProjectStore) ListProjects(context.Context, contract.ActorContext) ([]Project, error) {
	return nil, nil
}
func (*stubProjectStore) CreateBusinessTask(context.Context, BusinessTask) error { return nil }
func (*stubProjectStore) ListBusinessTasks(context.Context, contract.OrganizationID, contract.ProjectID) ([]BusinessTask, error) {
	return nil, nil
}
func (*stubProjectStore) GetBusinessTask(context.Context, contract.OrganizationID, contract.ProjectID, string) (BusinessTask, error) {
	return BusinessTask{}, nil
}
func (*stubProjectStore) UpdateBusinessTask(context.Context, BusinessTask) error { return nil }
func (*stubProjectStore) CreateOperationalRecord(context.Context, OperationalRecord) error {
	return nil
}
func (*stubProjectStore) ListOperationalRecords(context.Context, contract.OrganizationID, contract.ProjectID) ([]OperationalRecord, error) {
	return nil, nil
}
func (*stubProjectStore) GetOperationalRecord(context.Context, contract.OrganizationID, contract.ProjectID, string) (OperationalRecord, error) {
	return OperationalRecord{}, nil
}
func (*stubProjectStore) UpdateOperationalRecord(context.Context, OperationalRecord) error {
	return nil
}
func (*stubProjectStore) DeleteOperationalRecord(context.Context, contract.OrganizationID, contract.ProjectID, string) error {
	return nil
}
func (*stubProjectStore) CreateChangeSet(context.Context, ChangeSet) error { return nil }
func (*stubProjectStore) ListChangeSets(context.Context, contract.OrganizationID, contract.ProjectID) ([]ChangeSet, error) {
	return nil, nil
}
func (*stubProjectStore) GetChangeSet(context.Context, contract.OrganizationID, contract.ProjectID, string) (ChangeSet, error) {
	return ChangeSet{}, nil
}
func (*stubProjectStore) UpdateChangeSet(context.Context, ChangeSet) error { return nil }
func (*stubProjectStore) AppendChangeSetEvent(context.Context, ChangeSetEvent) error {
	return nil
}
func (*stubProjectStore) AppendAuditEvent(context.Context, AuditEvent) error { return nil }
func (*stubProjectStore) ListAuditEvents(context.Context, contract.OrganizationID, contract.ProjectID) ([]AuditEvent, error) {
	return nil, nil
}
