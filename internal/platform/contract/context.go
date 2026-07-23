// Package contract defines the stable, business-agnostic data shapes shared
// by the platform and the four vertical systems. It must not contain domain
// state machines or vendor-specific fields.
package contract

import (
	"context"
	"fmt"
	"strings"
)

type OrganizationID string
type ProjectID string
type Scope string

type PrincipalKind string

const (
	PrincipalUser    PrincipalKind = "user"
	PrincipalService PrincipalKind = "service"
)

type Principal struct {
	Kind PrincipalKind `json:"kind"`
	ID   string        `json:"id"`
}

type ActorContext struct {
	OrganizationID OrganizationID `json:"organization_id"`
	Principal      Principal      `json:"principal"`
	Scopes         []Scope        `json:"scopes"`
}

func (a ActorContext) Validate() error {
	if strings.TrimSpace(string(a.OrganizationID)) == "" {
		return fmt.Errorf("organization_id is required")
	}
	if a.Principal.Kind != PrincipalUser && a.Principal.Kind != PrincipalService {
		return fmt.Errorf("principal kind must be user or service")
	}
	if strings.TrimSpace(a.Principal.ID) == "" {
		return fmt.Errorf("principal ID is required")
	}
	seen := make(map[Scope]struct{}, len(a.Scopes))
	for _, scope := range a.Scopes {
		if strings.TrimSpace(string(scope)) == "" {
			return fmt.Errorf("scope must not be empty")
		}
		if _, exists := seen[scope]; exists {
			return fmt.Errorf("scope %q is duplicated", scope)
		}
		seen[scope] = struct{}{}
	}
	return nil
}

func (a ActorContext) HasScope(scope Scope) bool {
	for _, granted := range a.Scopes {
		if granted == scope {
			return true
		}
	}
	return false
}

func ScopesFromStrings(values []string) []Scope {
	result := make([]Scope, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, Scope(trimmed))
		}
	}
	return result
}

type RequestContext struct {
	RequestID string       `json:"request_id"`
	TraceID   string       `json:"trace_id"`
	Actor     ActorContext `json:"actor"`
}

func (c RequestContext) Validate() error {
	if strings.TrimSpace(c.RequestID) == "" {
		return fmt.Errorf("request_id is required")
	}
	if strings.TrimSpace(c.TraceID) == "" {
		return fmt.Errorf("trace_id is required")
	}
	if err := c.Actor.Validate(); err != nil {
		return fmt.Errorf("invalid actor context: %w", err)
	}
	return nil
}

type requestContextKey struct{}

func WithRequestContext(ctx context.Context, requestContext RequestContext) context.Context {
	return context.WithValue(ctx, requestContextKey{}, requestContext)
}

func RequestContextFrom(ctx context.Context) (RequestContext, bool) {
	requestContext, ok := ctx.Value(requestContextKey{}).(RequestContext)
	return requestContext, ok
}
