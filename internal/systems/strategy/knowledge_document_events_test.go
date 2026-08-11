package strategy

import (
	"context"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/knowledge"
)

func TestKnowledgeDocumentProductEventSinkStoresOnlyBoundedMetrics(t *testing.T) {
	writer := &capturingProductEventWriter{}
	sink := KnowledgeDocumentProductEventSink{Writer: writer, NewID: func() (string, error) { return "event_document_1", nil }}
	duration := int64(4200)
	if err := sink.RecordDocumentEvent(context.Background(), knowledge.DocumentEvent{
		OrganizationID: "org_1", ProjectID: "project_1", DocumentID: "document_1",
		Kind: knowledge.DocumentEventVisionFallback, Outcome: "succeeded", ParseStrategy: "hybrid",
		QualityTier: "medium", Status: "ready", DurationMS: &duration,
		OccurredAt: time.Date(2026, 8, 11, 11, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordDocumentEvent() error = %v", err)
	}
	if writer.event.EventType != ProductEventDocumentVisionFallback || writer.event.DurationMS == nil ||
		writer.event.Attributes["parse_strategy"] != "hybrid" || writer.event.Resource.ID != "document_1" {
		t.Fatalf("document product event = %#v", writer.event)
	}
	if _, exists := writer.event.Attributes["document_text"]; exists {
		t.Fatal("document text must never enter product metrics")
	}
}

type capturingProductEventWriter struct{ event ProductEvent }

func (w *capturingProductEventWriter) AppendProductEvent(_ context.Context, event ProductEvent) error {
	w.event = event
	return nil
}
