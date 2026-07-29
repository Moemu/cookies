// Package identity defines the seam between cookies and an identity provider.
// The bootstrap intentionally has no header-based development identity: a
// client must not be able to impersonate an organization by sending a header.
package identity

import (
	"context"
	"errors"
	"net/http"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var ErrUnauthenticated = errors.New("request is not authenticated")
var ErrProjectAccessDenied = errors.New("project access is denied")

type Resolver interface {
	Authenticate(context.Context, *http.Request) (contract.ActorContext, error)
}

type ActorValidator interface {
	ValidateActor(context.Context, contract.ActorContext) error
}

// ValidatingResolver re-checks persisted membership/service status after a
// trusted credential has been resolved. Revocation therefore takes effect on
// the next request instead of waiting for a long-lived token to expire.
type ValidatingResolver struct {
	Delegate  Resolver
	Validator ActorValidator
}

func (r ValidatingResolver) Authenticate(ctx context.Context, request *http.Request) (contract.ActorContext, error) {
	if r.Delegate == nil || r.Validator == nil {
		return contract.ActorContext{}, ErrUnauthenticated
	}
	actor, err := r.Delegate.Authenticate(ctx, request)
	if err != nil {
		return contract.ActorContext{}, err
	}
	if err := r.Validator.ValidateActor(ctx, actor); err != nil {
		if errors.Is(err, ErrActorInactive) {
			return contract.ActorContext{}, ErrUnauthenticated
		}
		return contract.ActorContext{}, err
	}
	return actor, nil
}

// ProjectAuthorizer is implemented by the Project module. Shared HTTP code
// never queries Project tables directly.
type ProjectAuthorizer interface {
	AuthorizeProject(context.Context, contract.ActorContext, contract.ProjectID) error
	AuthorizeProjectAction(context.Context, contract.ActorContext, contract.ProjectID, string) error
}

type RejectingResolver struct{}

func (RejectingResolver) Authenticate(context.Context, *http.Request) (contract.ActorContext, error) {
	return contract.ActorContext{}, ErrUnauthenticated
}

type StaticResolver struct {
	actor contract.ActorContext
}

func NewStaticResolver(actor contract.ActorContext) (StaticResolver, error) {
	if actor.Scopes == nil {
		actor.Scopes = []contract.Scope{}
	}
	if err := actor.Validate(); err != nil {
		return StaticResolver{}, err
	}
	return StaticResolver{actor: actor}, nil
}

func (r StaticResolver) Authenticate(context.Context, *http.Request) (contract.ActorContext, error) {
	return r.actor, nil
}

type RejectingProjectAuthorizer struct{}

func (RejectingProjectAuthorizer) AuthorizeProject(context.Context, contract.ActorContext, contract.ProjectID) error {
	return ErrProjectAccessDenied
}
func (RejectingProjectAuthorizer) AuthorizeProjectAction(context.Context, contract.ActorContext, contract.ProjectID, string) error {
	return ErrProjectAccessDenied
}

// StaticProjectAuthorizer is local-development-only support. Production is
// expected to receive an authorizer backed by the Project module.
type StaticProjectAuthorizer struct {
	ProjectID contract.ProjectID
}

func (a StaticProjectAuthorizer) AuthorizeProject(_ context.Context, actor contract.ActorContext, projectID contract.ProjectID) error {
	if actor.Validate() == nil && a.ProjectID != "" && a.ProjectID == projectID {
		return nil
	}
	return ErrProjectAccessDenied
}

func (a StaticProjectAuthorizer) AuthorizeProjectAction(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, _ string) error {
	return a.AuthorizeProject(ctx, actor, projectID)
}
