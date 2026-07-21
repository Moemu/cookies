package contract

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventEnvelopeValidation(t *testing.T) {
	t.Parallel()
	version := int64(3)
	event := EventEnvelope{
		EventID:        "evt_1",
		EventType:      "creative.approved.v1",
		OccurredAt:     time.Date(2026, time.July, 21, 0, 0, 0, 0, time.UTC),
		Producer:       "creative",
		OrganizationID: "org_1",
		ProjectID:      "project_1",
		Subject:        Subject{Type: "creative_version", ID: "cv_1", Version: &version},
		Data:           json.RawMessage(`{"creative_package_id":"cp_1"}`),
		TraceID:        "trace_1",
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestEventEnvelopeRejectsUnversionedNameAndNonObjectData(t *testing.T) {
	t.Parallel()
	event := EventEnvelope{
		EventID:        "evt_1",
		EventType:      "creative.approved",
		OccurredAt:     time.Now(),
		Producer:       "creative",
		OrganizationID: "org_1",
		Subject:        Subject{Type: "creative_version", ID: "cv_1"},
		Data:           json.RawMessage(`[]`),
		TraceID:        "trace_1",
	}
	if err := event.Validate(); err == nil {
		t.Fatal("expected invalid event to be rejected")
	}
}
