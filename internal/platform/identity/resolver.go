// Package identity defines the seam between cookies and an identity provider.
// The bootstrap intentionally has no header-based development identity: a
// client must not be able to impersonate an organization by sending a header.
package identity

import (
	"context"
	"errors"
	"net/http"

	"github.com/Cecillia803/cookies/internal/platform/contract"
)

var ErrUnauthenticated = errors.New("request is not authenticated")

type Resolver interface {
	Authenticate(context.Context, *http.Request) (contract.ActorContext, error)
}

type RejectingResolver struct{}

func (RejectingResolver) Authenticate(context.Context, *http.Request) (contract.ActorContext, error) {
	return contract.ActorContext{}, ErrUnauthenticated
}

type StaticResolver struct {
	actor contract.ActorContext
}

func NewStaticResolver(actor contract.ActorContext) (StaticResolver, error) {
	if err := actor.Validate(); err != nil {
		return StaticResolver{}, err
	}
	return StaticResolver{actor: actor}, nil
}

func (r StaticResolver) Authenticate(context.Context, *http.Request) (contract.ActorContext, error) {
	return r.actor, nil
}
