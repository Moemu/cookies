package knowledge

import (
	"context"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	DocumentEventParseStarted   = "parse_started"
	DocumentEventReady          = "ready"
	DocumentEventPartial        = "partial"
	DocumentEventFailed         = "failed"
	DocumentEventVisionFallback = "vision_fallback"
)

type DocumentEvent struct {
	OrganizationID contract.OrganizationID
	ProjectID      contract.ProjectID
	DocumentID     string
	Kind           string
	Outcome        string
	ParseStrategy  string
	QualityTier    string
	Status         string
	DurationMS     *int64
	OccurredAt     time.Time
}

type DocumentEventSink interface {
	RecordDocumentEvent(context.Context, DocumentEvent) error
}

func (s Service) recordDocumentEvent(ctx context.Context, document Document, kind, outcome, status string, duration *time.Duration) {
	if s.DocumentEvents == nil {
		return
	}
	var durationMS *int64
	if duration != nil {
		value := max(int64(0), duration.Milliseconds())
		durationMS = &value
	}
	_ = s.DocumentEvents.RecordDocumentEvent(ctx, DocumentEvent{
		OrganizationID: document.OrganizationID, ProjectID: document.ProjectID, DocumentID: document.ID,
		Kind: kind, Outcome: outcome, ParseStrategy: document.ParseStrategy,
		QualityTier: document.QualityTier, Status: status, DurationMS: durationMS, OccurredAt: s.now(),
	})
}
