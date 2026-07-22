package identity

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestRejectingResolverFailsClosed(t *testing.T) {
	t.Parallel()
	_, err := (RejectingResolver{}).Authenticate(context.Background(), httptest.NewRequest("GET", "/", nil))
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate() error = %v, want ErrUnauthenticated", err)
	}
}

func TestStaticResolverRejectsInvalidActor(t *testing.T) {
	t.Parallel()
	_, err := NewStaticResolver(contract.ActorContext{})
	if err == nil {
		t.Fatal("expected invalid actor to be rejected")
	}
}

func TestStaticProjectAuthorizerDoesNotTrustActorProjectData(t *testing.T) {
	t.Parallel()
	actor := contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"}}
	authorizer := StaticProjectAuthorizer{ProjectID: "project_1"}
	if err := authorizer.AuthorizeProject(context.Background(), actor, "project_1"); err != nil {
		t.Fatalf("AuthorizeProject() error = %v", err)
	}
	if err := authorizer.AuthorizeProject(context.Background(), actor, "project_2"); !errors.Is(err, ErrProjectAccessDenied) {
		t.Fatalf("AuthorizeProject() error = %v, want ErrProjectAccessDenied", err)
	}
}
