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

type stubProjectStore struct{ read bool }

func (*stubProjectStore) CreateBrand(context.Context, Brand) error { return nil }
func (*stubProjectStore) CreateProject(context.Context, Project, contract.Principal, []contract.ProductID) error {
	return nil
}
func (s *stubProjectStore) GetProject(context.Context, contract.OrganizationID, contract.ProjectID) (Project, error) {
	s.read = true
	return Project{}, ErrNotFound
}
func (s *stubProjectStore) GetContext(context.Context, contract.OrganizationID, contract.ProjectID) (contract.ProjectContext, error) {
	s.read = true
	return contract.ProjectContext{}, ErrNotFound
}
func (*stubProjectStore) ListProjects(context.Context, contract.ActorContext) ([]Project, error) {
	return nil, nil
}
