package eventoutbox

import (
	"testing"
	"time"
)

func TestEventValidationRequiresTenantAndImmutableSubject(t *testing.T) {
	t.Parallel()
	event := Event{
		ID: "event_1", OrganizationID: "org_1", ProjectID: "project_1",
		Type: "strategy.approved.v1", SubjectType: "strategy_package",
		SubjectID: "package_1", SubjectVersion: 1, Payload: []byte(`{"event_type":"strategy.approved.v1"}`),
		CreatedAt: time.Now().UTC(),
	}
	if err := event.Validate(); err != nil {
		t.Fatal(err)
	}
	event.SubjectVersion = 0
	if err := event.Validate(); err == nil {
		t.Fatal("zero subject version unexpectedly validated")
	}
}
