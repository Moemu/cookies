package strategy

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/knowledge"
)

type KnowledgeDocumentProductEventSink struct {
	Writer ProductEventWriter
	NewID  func() (string, error)
}

func (s KnowledgeDocumentProductEventSink) RecordDocumentEvent(ctx context.Context, input knowledge.DocumentEvent) error {
	if s.Writer == nil || s.NewID == nil {
		return fmt.Errorf("knowledge document product event dependencies are required")
	}
	eventType := ""
	switch input.Kind {
	case knowledge.DocumentEventParseStarted:
		eventType = ProductEventDocumentParseStarted
	case knowledge.DocumentEventReady:
		eventType = ProductEventDocumentReady
	case knowledge.DocumentEventPartial:
		eventType = ProductEventDocumentPartial
	case knowledge.DocumentEventFailed:
		eventType = ProductEventDocumentFailed
	case knowledge.DocumentEventVisionFallback:
		eventType = ProductEventDocumentVisionFallback
	default:
		return fmt.Errorf("unsupported knowledge document event %q", input.Kind)
	}
	id, err := s.NewID()
	if err != nil {
		return err
	}
	attributes := map[string]any{}
	if value := strings.TrimSpace(input.ParseStrategy); value != "" {
		attributes["parse_strategy"] = value
	}
	if value := strings.TrimSpace(input.QualityTier); value != "" {
		attributes["quality_tier"] = value
	}
	if value := strings.TrimSpace(input.Status); value != "" {
		attributes["status"] = value
	}
	event, err := NewProductEvent(NewProductEventInput{
		ID: id,
		Actor: contract.ActorContext{
			OrganizationID: input.OrganizationID,
			Principal:      contract.Principal{Kind: contract.PrincipalService, ID: "knowledge-document-pipeline"},
			Scopes:         []contract.Scope{},
		},
		ProjectID: input.ProjectID, EventType: eventType,
		Resource:   ProductEventResource{Type: "knowledge_document", ID: input.DocumentID},
		DurationMS: input.DurationMS, Outcome: input.Outcome, Attributes: attributes, OccurredAt: input.OccurredAt,
	})
	if err != nil {
		return err
	}
	return s.Writer.AppendProductEvent(ctx, event)
}
