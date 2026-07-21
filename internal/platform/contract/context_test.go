package contract

import (
	"context"
	"reflect"
	"testing"
)

func TestRequestContextRequiresTenantAndPrincipal(t *testing.T) {
	t.Parallel()

	requestContext := RequestContext{RequestID: "req_1", TraceID: "trace_1"}
	if err := requestContext.Validate(); err == nil {
		t.Fatal("expected context without tenant and principal to be invalid")
	}
}

func TestRequestContextRoundTripsThroughContext(t *testing.T) {
	t.Parallel()

	want := RequestContext{
		RequestID: "req_1",
		TraceID:   "trace_1",
		Actor: ActorContext{
			OrganizationID: "org_1",
			Principal:      Principal{Kind: PrincipalUser, ID: "usr_1"},
			ProjectID:      "project_1",
			Scopes:         []Scope{"strategy.brief.read"},
		},
	}
	got, ok := RequestContextFrom(WithRequestContext(context.Background(), want))
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("context round trip = %#v, %t; want %#v, true", got, ok, want)
	}
}
