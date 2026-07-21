// Package audit contains the append-only audit record contract. Persistence is
// intentionally deferred until the platform database and migration ADR are in
// place; callers can already depend on this safe, content-free shape.
package audit

import (
	"fmt"
	"strings"
	"time"

	"github.com/Cecillia803/cookies/internal/platform/contract"
)

type Outcome string

const (
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeDenied    Outcome = "denied"
	OutcomeFailed    Outcome = "failed"
)

type Record struct {
	ID         string                  `json:"id"`
	OccurredAt time.Time               `json:"occurred_at"`
	Request    contract.RequestContext `json:"request"`
	Action     string                  `json:"action"`
	Target     contract.ResourceRef    `json:"target"`
	Outcome    Outcome                 `json:"outcome"`
	ReasonCode string                  `json:"reason_code,omitempty"`
}

func (r Record) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("audit record ID is required")
	}
	if r.OccurredAt.IsZero() {
		return fmt.Errorf("audit occurred_at is required")
	}
	if err := r.Request.Validate(); err != nil {
		return fmt.Errorf("invalid audit request context: %w", err)
	}
	if strings.TrimSpace(r.Action) == "" {
		return fmt.Errorf("audit action is required")
	}
	if err := r.Target.Validate(); err != nil {
		return fmt.Errorf("invalid audit target: %w", err)
	}
	switch r.Outcome {
	case OutcomeSucceeded, OutcomeDenied, OutcomeFailed:
		return nil
	default:
		return fmt.Errorf("audit outcome is invalid")
	}
}
