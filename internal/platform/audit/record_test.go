package audit

import (
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestRecordRequiresAValidRequestContext(t *testing.T) {
	t.Parallel()
	record := Record{
		ID:         "audit_1",
		OccurredAt: time.Now(),
		Action:     "project.create",
		Target:     contract.ResourceRef{Type: "project", ID: "project_1"},
		Outcome:    OutcomeSucceeded,
	}
	if err := record.Validate(); err == nil {
		t.Fatal("expected an audit record without tenant context to be invalid")
	}
}
