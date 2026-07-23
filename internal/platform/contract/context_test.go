package contract

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRequestContextRequiresTenantAndPrincipal(t *testing.T) {
	t.Parallel()

	requestContext := RequestContext{RequestID: "req_1", TraceID: "trace_1"}
	if err := requestContext.Validate(); err == nil {
		t.Fatal("expected context without tenant and principal to be invalid")
	}
}

func TestActorContextJSONNeverCarriesProjectID(t *testing.T) {
	t.Parallel()
	payload, err := json.Marshal(ActorContext{
		OrganizationID: "org_1",
		Principal:      Principal{Kind: PrincipalUser, ID: "usr_1"},
		Scopes:         []Scope{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "project_id") {
		t.Fatalf("ActorContext leaked project scope: %s", payload)
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
			Scopes:         []Scope{"strategy.brief.read"},
		},
	}
	got, ok := RequestContextFrom(WithRequestContext(context.Background(), want))
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("context round trip = %#v, %t; want %#v, true", got, ok, want)
	}
}
