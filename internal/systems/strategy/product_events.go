package strategy

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const ProductEventContractV1 = "strategy-product-event/v1"

const (
	ProductEventWorkspaceOpened                = "workspace.opened"
	ProductEventStageViewed                    = "stage.viewed"
	ProductEventAssistantCommandSubmitted      = "assistant.command_submitted"
	ProductEventAssistantFirstAck              = "assistant.first_ack"
	ProductEventAssistantFirstMeaningfulUpdate = "assistant.first_meaningful_update"
	ProductEventAssistantProposalAccepted      = "assistant.proposal_accepted"
	ProductEventAssistantProposalEdited        = "assistant.proposal_edited"
	ProductEventAssistantProposalIgnored       = "assistant.proposal_ignored"
	ProductEventResearchStarted                = "research.started"
	ProductEventResearchFindingVerified        = "research.finding_verified"
	ProductEventResearchCompleted              = "research.completed"
	ProductEventResearchPartial                = "research.partial"
	ProductEventResearchFailed                 = "research.failed"
	ProductEventResearchCancelled              = "research.cancelled"
	ProductEventResearchProposalApplied        = "research.proposal_applied"
	ProductEventResearchProposalStale          = "research.proposal_stale"
	ProductEventDocumentParseStarted           = "document.parse_started"
	ProductEventDocumentReady                  = "document.ready"
	ProductEventDocumentPartial                = "document.partial"
	ProductEventDocumentFailed                 = "document.failed"
	ProductEventDocumentVisionFallback         = "document.vision_fallback"
	ProductEventBriefConfirmed                 = "brief.confirmed"
	ProductEventStrategyConfirmed              = "strategy.confirmed"
	ProductEventReviewSubmitted                = "review.submitted"
	ProductEventReviewApproved                 = "review.approved"
	ProductEventReviewReturned                 = "review.returned"
	ProductEventHandoffCreated                 = "handoff.created"
	ProductEventActivityStalled                = "activity.stalled"
	ProductEventActivityRetried                = "activity.retried"
)

var allowedProductEventTypes = map[string]struct{}{
	ProductEventWorkspaceOpened: {}, ProductEventStageViewed: {},
	ProductEventAssistantCommandSubmitted: {}, ProductEventAssistantFirstAck: {},
	ProductEventAssistantFirstMeaningfulUpdate: {}, ProductEventAssistantProposalAccepted: {},
	ProductEventAssistantProposalEdited: {}, ProductEventAssistantProposalIgnored: {},
	ProductEventResearchStarted: {}, ProductEventResearchFindingVerified: {},
	ProductEventResearchCompleted: {}, ProductEventResearchPartial: {},
	ProductEventResearchFailed: {}, ProductEventResearchCancelled: {},
	ProductEventResearchProposalApplied: {}, ProductEventResearchProposalStale: {},
	ProductEventDocumentParseStarted: {}, ProductEventDocumentReady: {},
	ProductEventDocumentPartial: {}, ProductEventDocumentFailed: {},
	ProductEventDocumentVisionFallback: {}, ProductEventBriefConfirmed: {},
	ProductEventStrategyConfirmed: {}, ProductEventReviewSubmitted: {},
	ProductEventReviewApproved: {}, ProductEventReviewReturned: {},
	ProductEventHandoffCreated: {}, ProductEventActivityStalled: {},
	ProductEventActivityRetried: {},
}

var allowedProductEventStages = map[string]struct{}{
	"": {}, "intake": {}, "brief": {}, "strategy": {}, "review": {}, "handoff": {},
}

var allowedProductEventOutcomes = map[string]struct{}{
	"": {}, "accepted": {}, "viewed": {}, "succeeded": {}, "partial": {},
	"failed": {}, "cancelled": {}, "stalled": {}, "retried": {}, "edited": {},
	"ignored": {}, "stale": {}, "approved": {}, "returned": {},
}

var allowedProductEventAttributeKeys = map[string]struct{}{
	"capability": {}, "finding_count": {}, "mode": {}, "panel": {},
	"parse_strategy": {}, "proposal_kind": {}, "quality_tier": {},
	"reason_code": {}, "retry_count": {}, "review_mode": {}, "round": {},
	"source_count": {}, "status": {},
}

var productEventIdentifier = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,127}$`)

var numericProductEventAttributeKeys = map[string]struct{}{
	"finding_count": {}, "retry_count": {}, "round": {}, "source_count": {},
}

type ProductEventResource struct {
	Type    string
	ID      string
	Version *int64
}

type ProductEvent struct {
	ID             string
	OrganizationID contract.OrganizationID
	ProjectID      contract.ProjectID
	WorkspaceID    string
	EventType      string
	Stage          string
	ActorKind      contract.PrincipalKind
	ActorIDHash    string
	Resource       ProductEventResource
	DurationMS     *int64
	Outcome        string
	Attributes     map[string]any
	OccurredAt     time.Time
}

type NewProductEventInput struct {
	ID          string
	Actor       contract.ActorContext
	ProjectID   contract.ProjectID
	WorkspaceID string
	EventType   string
	Stage       string
	Resource    ProductEventResource
	DurationMS  *int64
	Outcome     string
	Attributes  map[string]any
	OccurredAt  time.Time
}

func NewProductEvent(input NewProductEventInput) (ProductEvent, error) {
	if err := input.Actor.Validate(); err != nil {
		return ProductEvent{}, fmt.Errorf("invalid product event actor: %w", err)
	}
	value := ProductEvent{
		ID: input.ID, OrganizationID: input.Actor.OrganizationID, ProjectID: input.ProjectID,
		WorkspaceID: input.WorkspaceID, EventType: input.EventType, Stage: input.Stage,
		ActorKind: input.Actor.Principal.Kind, ActorIDHash: productEventActorHash(input.Actor),
		Resource: input.Resource, DurationMS: input.DurationMS, Outcome: input.Outcome,
		Attributes: input.Attributes, OccurredAt: input.OccurredAt.UTC(),
	}
	if value.Attributes == nil {
		value.Attributes = map[string]any{}
	}
	if err := value.Validate(); err != nil {
		return ProductEvent{}, err
	}
	return value, nil
}

func (e ProductEvent) Validate() error {
	if strings.TrimSpace(e.ID) == "" || len(e.ID) > 96 || strings.TrimSpace(string(e.OrganizationID)) == "" || strings.TrimSpace(string(e.ProjectID)) == "" {
		return fmt.Errorf("product event identity and scope are required")
	}
	if len(e.WorkspaceID) > 96 {
		return fmt.Errorf("product event workspace_id is too long")
	}
	if _, ok := allowedProductEventTypes[e.EventType]; !ok {
		return fmt.Errorf("product event type is unsupported")
	}
	if _, ok := allowedProductEventStages[e.Stage]; !ok {
		return fmt.Errorf("product event stage is unsupported")
	}
	if e.ActorKind != contract.PrincipalUser && e.ActorKind != contract.PrincipalService {
		return fmt.Errorf("product event actor kind is unsupported")
	}
	if !isLowerHexSHA256(e.ActorIDHash) {
		return fmt.Errorf("product event actor hash is invalid")
	}
	if err := e.Resource.validate(); err != nil {
		return err
	}
	if e.DurationMS != nil && *e.DurationMS < 0 {
		return fmt.Errorf("product event duration must not be negative")
	}
	if _, ok := allowedProductEventOutcomes[e.Outcome]; !ok {
		return fmt.Errorf("product event outcome is unsupported")
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("product event occurred_at is required")
	}
	return validateProductEventAttributes(e.Attributes)
}

func (r ProductEventResource) validate() error {
	typeValue, idValue := strings.TrimSpace(r.Type), strings.TrimSpace(r.ID)
	if typeValue == "" && idValue == "" && r.Version == nil {
		return nil
	}
	if typeValue == "" || idValue == "" || len(typeValue) > 48 || len(idValue) > 96 {
		return fmt.Errorf("product event resource type and ID must be supplied together")
	}
	if r.Version != nil && *r.Version < 1 {
		return fmt.Errorf("product event resource version must be positive")
	}
	return nil
}

func validateProductEventAttributes(attributes map[string]any) error {
	if len(attributes) > 12 {
		return fmt.Errorf("product event attributes exceed the key limit")
	}
	for key, value := range attributes {
		if _, ok := allowedProductEventAttributeKeys[key]; !ok {
			return fmt.Errorf("product event attribute %q is not allowed", key)
		}
		_, numeric := numericProductEventAttributeKeys[key]
		switch typed := value.(type) {
		case int:
			if !numeric || typed < 0 {
				return fmt.Errorf("product event attribute %q must be a non-negative counter", key)
			}
		case int32:
			if !numeric || typed < 0 {
				return fmt.Errorf("product event attribute %q must be a non-negative counter", key)
			}
		case int64:
			if !numeric || typed < 0 {
				return fmt.Errorf("product event attribute %q must be a non-negative counter", key)
			}
		case uint, uint32, uint64:
			if !numeric {
				return fmt.Errorf("product event attribute %q must be an identifier", key)
			}
		case string:
			if numeric || !productEventIdentifier.MatchString(typed) {
				return fmt.Errorf("product event attribute %q must be a bounded identifier", key)
			}
		case float32:
			if !numeric || typed < 0 || typed != float32(math.Trunc(float64(typed))) || math.IsNaN(float64(typed)) || math.IsInf(float64(typed), 0) {
				return fmt.Errorf("product event attribute %q must be a non-negative integer", key)
			}
		case float64:
			if !numeric || typed < 0 || typed != math.Trunc(typed) || math.IsNaN(typed) || math.IsInf(typed, 0) {
				return fmt.Errorf("product event attribute %q must be a non-negative integer", key)
			}
		case json.Number:
			parsed, err := typed.Int64()
			if !numeric || err != nil || parsed < 0 {
				return fmt.Errorf("product event attribute %q must be a non-negative integer", key)
			}
		default:
			return fmt.Errorf("product event attribute %q has an unsupported value", key)
		}
	}
	encoded, err := json.Marshal(attributes)
	if err != nil || len(encoded) > 2048 {
		return fmt.Errorf("product event attributes exceed the encoded size limit")
	}
	return nil
}

func productEventActorHash(actor contract.ActorContext) string {
	digest := sha256.Sum256([]byte(string(actor.OrganizationID) + "\x00" + string(actor.Principal.Kind) + "\x00" + actor.Principal.ID))
	return hex.EncodeToString(digest[:])
}

func isLowerHexSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

type ProductEventWriter interface {
	AppendProductEvent(context.Context, ProductEvent) error
}

type TransactionalProductEventWriter interface {
	ProductEventWriter
	AppendProductEventIn(context.Context, *sql.Tx, ProductEvent) error
}

type MySQLProductEventWriter struct {
	DB *sql.DB
}

func (w MySQLProductEventWriter) AppendProductEvent(ctx context.Context, event ProductEvent) error {
	if w.DB == nil {
		return fmt.Errorf("product event database is required")
	}
	return appendMySQLProductEvent(ctx, w.DB, event)
}

func (w MySQLProductEventWriter) AppendProductEventIn(ctx context.Context, tx *sql.Tx, event ProductEvent) error {
	if tx == nil {
		return fmt.Errorf("product event transaction is required")
	}
	return appendMySQLProductEvent(ctx, tx, event)
}

type productEventExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func appendMySQLProductEvent(ctx context.Context, execer productEventExecer, event ProductEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	attributes, err := json.Marshal(event.Attributes)
	if err != nil {
		return fmt.Errorf("encode product event attributes: %w", err)
	}
	_, err = execer.ExecContext(ctx, `INSERT INTO strategy_product_events
		(id, organization_id, project_id, workspace_id, event_type, stage,
		 actor_kind, actor_id_hash, resource_type, resource_id, resource_version,
		 duration_ms, outcome, attributes_json, occurred_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), ?, ?, NULLIF(?, ''),
		 NULLIF(?, ''), ?, ?, NULLIF(?, ''), ?, ?)`,
		event.ID, event.OrganizationID, event.ProjectID, event.WorkspaceID,
		event.EventType, event.Stage, event.ActorKind, event.ActorIDHash,
		event.Resource.Type, event.Resource.ID, event.Resource.Version,
		event.DurationMS, event.Outcome, attributes, event.OccurredAt.UTC())
	return err
}

func (s Service) appendProductEventIn(ctx context.Context, tx *sql.Tx, input NewProductEventInput) error {
	if s.ProductEvents == nil {
		return nil
	}
	writer, ok := s.ProductEvents.(TransactionalProductEventWriter)
	if !ok {
		return fmt.Errorf("strategy product event writer must support transactions")
	}
	event, err := NewProductEvent(input)
	if err != nil {
		return err
	}
	return writer.AppendProductEventIn(ctx, tx, event)
}
