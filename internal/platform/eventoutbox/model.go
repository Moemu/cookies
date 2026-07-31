// Package eventoutbox provides transactional, at-least-once delivery of
// cross-module fact notifications. Consumers must be idempotent.
package eventoutbox

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type Event struct {
	ID             string                  `json:"event_id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	Type           string                  `json:"event_type"`
	SubjectType    string                  `json:"subject_type"`
	SubjectID      string                  `json:"subject_id"`
	SubjectVersion int64                   `json:"subject_version"`
	Payload        json.RawMessage         `json:"payload"`
	CreatedAt      time.Time               `json:"created_at"`
}

func (e Event) Validate() error {
	if strings.TrimSpace(e.ID) == "" || e.OrganizationID == "" || e.ProjectID == "" ||
		strings.TrimSpace(e.Type) == "" || strings.TrimSpace(e.SubjectType) == "" ||
		strings.TrimSpace(e.SubjectID) == "" || e.SubjectVersion < 1 || e.CreatedAt.IsZero() {
		return fmt.Errorf("outbox event identity, scope, subject version, and timestamp are required")
	}
	if !json.Valid(e.Payload) {
		return fmt.Errorf("outbox event payload must be valid JSON")
	}
	return nil
}

type Publisher interface {
	Publish(context.Context, Event) error
}
