package contract

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Subject is kept as an alias for the event vocabulary used in the product
// documentation. ResourceRef is the shared implementation type.
type Subject = ResourceRef

type EventEnvelope struct {
	EventID        string          `json:"event_id"`
	EventType      string          `json:"event_type"`
	OccurredAt     time.Time       `json:"occurred_at"`
	Producer       string          `json:"producer"`
	OrganizationID OrganizationID  `json:"organization_id"`
	ProjectID      ProjectID       `json:"project_id,omitempty"`
	Subject        Subject         `json:"subject"`
	Data           json.RawMessage `json:"data"`
	TraceID        string          `json:"trace_id"`
}

func (e EventEnvelope) Validate() error {
	if strings.TrimSpace(e.EventID) == "" {
		return fmt.Errorf("event_id is required")
	}
	if !validVersionedName(e.EventType) {
		return fmt.Errorf("event_type must use a versioned name such as creative.approved.v1")
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("occurred_at is required")
	}
	if strings.TrimSpace(e.Producer) == "" {
		return fmt.Errorf("producer is required")
	}
	if strings.TrimSpace(string(e.OrganizationID)) == "" {
		return fmt.Errorf("organization_id is required")
	}
	if err := e.Subject.Validate(); err != nil {
		return fmt.Errorf("invalid subject: %w", err)
	}
	if !validJSONObject(e.Data) {
		return fmt.Errorf("data must be a valid JSON object")
	}
	if strings.TrimSpace(e.TraceID) == "" {
		return fmt.Errorf("trace_id is required")
	}
	return nil
}

func validVersionedName(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) < 3 || len(parts[len(parts)-1]) < 2 || parts[len(parts)-1][0] != 'v' {
		return false
	}
	for index, part := range parts {
		if part == "" || (index < len(parts)-1 && !validNamePart(part)) {
			return false
		}
	}
	version := parts[len(parts)-1][1:]
	if version[0] == '0' {
		return false
	}
	for _, character := range version {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validNamePart(value string) bool {
	for index, character := range value {
		if index == 0 && (character < 'a' || character > 'z') {
			return false
		}
		if !((character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') ||
			character == '_') {
			return false
		}
	}
	return true
}

func validJSONObject(value json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(value))
	return len(trimmed) >= 2 && trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}' && json.Valid(value)
}
