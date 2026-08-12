package strategy

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestNewProductEventPseudonymizesActorAndNormalizesTime(t *testing.T) {
	t.Parallel()
	version, duration := int64(4), int64(823)
	actor := contract.ActorContext{
		OrganizationID: "org_1",
		Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "user@example.com"},
		Scopes:         []contract.Scope{ScopeRead},
	}
	event, err := NewProductEvent(NewProductEventInput{
		ID: "productevent_1", Actor: actor, ProjectID: "project_1", WorkspaceID: "workspace_1",
		EventType: ProductEventAssistantFirstAck, Stage: "brief",
		Resource:   ProductEventResource{Type: "agent_task", ID: "task_1", Version: &version},
		DurationMS: &duration, Outcome: "accepted", Attributes: map[string]any{"mode": "standard"},
		OccurredAt: time.Date(2026, 8, 10, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60)),
	})
	if err != nil {
		t.Fatalf("new product event: %v", err)
	}
	if event.ActorIDHash == actor.Principal.ID || strings.Contains(event.ActorIDHash, "example") || len(event.ActorIDHash) != 64 {
		t.Fatalf("actor hash leaked identity: %q", event.ActorIDHash)
	}
	if event.OccurredAt.Location() != time.UTC || event.OccurredAt.Hour() != 0 {
		t.Fatalf("occurred_at was not normalized to UTC: %s", event.OccurredAt)
	}
	if event.Attributes["mode"] != "standard" || event.Resource.Version == nil || *event.Resource.Version != 4 {
		t.Fatalf("event lost bounded metadata: %#v", event)
	}
}

func TestProductEventRejectsUnboundedOrSensitiveAttributes(t *testing.T) {
	t.Parallel()
	base := validProductEventForTest()
	tests := []struct {
		name       string
		attributes map[string]any
	}{
		{name: "prompt", attributes: map[string]any{"prompt": "sensitive text"}},
		{name: "nested", attributes: map[string]any{"status": map[string]any{"raw": "payload"}}},
		{name: "long string", attributes: map[string]any{"reason_code": strings.Repeat("x", 129)}},
		{name: "free text", attributes: map[string]any{"status": "contains user supplied prose"}},
		{name: "counter string", attributes: map[string]any{"source_count": "many"}},
		{name: "negative counter", attributes: map[string]any{"round": -1}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate := base
			candidate.Attributes = test.attributes
			if err := candidate.Validate(); err == nil {
				t.Fatalf("attributes were accepted: %#v", test.attributes)
			}
		})
	}
}

func TestProductEventRejectsUnknownSemanticsAndInvalidMeasurements(t *testing.T) {
	t.Parallel()
	negative, zero := int64(-1), int64(0)
	tests := []ProductEvent{
		func() ProductEvent {
			value := validProductEventForTest()
			value.EventType = "user.raw_prompt"
			return value
		}(),
		func() ProductEvent { value := validProductEventForTest(); value.Stage = "research"; return value }(),
		func() ProductEvent { value := validProductEventForTest(); value.Outcome = "maybe"; return value }(),
		func() ProductEvent { value := validProductEventForTest(); value.DurationMS = &negative; return value }(),
		func() ProductEvent {
			value := validProductEventForTest()
			value.Resource = ProductEventResource{Type: "brief", ID: "brief_1", Version: &zero}
			return value
		}(),
	}
	for index, candidate := range tests {
		if err := candidate.Validate(); err == nil {
			t.Fatalf("invalid candidate %d was accepted: %#v", index, candidate)
		}
	}
}

func TestMySQLProductEventWriterRequiresDatabase(t *testing.T) {
	t.Parallel()
	if err := (MySQLProductEventWriter{}).AppendProductEvent(context.Background(), validProductEventForTest()); err == nil {
		t.Fatal("writer accepted a missing database")
	}
}

func validProductEventForTest() ProductEvent {
	return ProductEvent{
		ID: "productevent_1", OrganizationID: "org_1", ProjectID: "project_1",
		EventType: ProductEventWorkspaceOpened, ActorKind: contract.PrincipalUser,
		ActorIDHash: strings.Repeat("a", 64), Attributes: map[string]any{},
		OccurredAt: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
	}
}
