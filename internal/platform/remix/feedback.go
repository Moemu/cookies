package remix

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	FeedbackEventRating          FeedbackEventType = "rating"
	FeedbackEventComment         FeedbackEventType = "comment"
	FeedbackEventAssetSelected   FeedbackEventType = "asset_selected"
	FeedbackEventRenderSucceeded FeedbackEventType = "render_succeeded"

	FeedbackTargetRemixPlan FeedbackTargetType = "remix_plan"
	FeedbackTargetRenderJob FeedbackTargetType = "render_job"
	FeedbackTargetAsset     FeedbackTargetType = "asset"
)

type FeedbackEventType string
type FeedbackTargetType string

type CreateFeedbackEventRequest struct {
	EventType    FeedbackEventType         `json:"event_type"`
	TargetType   FeedbackTargetType        `json:"target_type"`
	TargetID     string                    `json:"target_id"`
	AssetVersion *contract.AssetVersionRef `json:"asset_version,omitempty"`
	Rating       int                       `json:"rating,omitempty"`
	Comment      string                    `json:"comment,omitempty"`
}

type FeedbackEvent struct {
	ID             string                    `json:"id"`
	OrganizationID contract.OrganizationID   `json:"organization_id"`
	ProjectID      contract.ProjectID        `json:"project_id"`
	EventType      FeedbackEventType         `json:"event_type"`
	TargetType     FeedbackTargetType        `json:"target_type"`
	TargetID       string                    `json:"target_id"`
	AssetVersion   *contract.AssetVersionRef `json:"asset_version,omitempty"`
	Rating         int                       `json:"rating,omitempty"`
	Comment        string                    `json:"comment,omitempty"`
	CreatedBy      contract.Principal        `json:"created_by"`
	CreatedAt      time.Time                 `json:"created_at"`
}

type FeedbackEventFilter struct {
	TargetType FeedbackTargetType `json:"target_type,omitempty"`
	TargetID   string             `json:"target_id,omitempty"`
	Limit      int                `json:"limit,omitempty"`
}

type AssetPerformance struct {
	AssetVersion         contract.AssetVersionRef `json:"asset_version"`
	SelectedCount        int                      `json:"selected_count"`
	RenderSucceededCount int                      `json:"render_succeeded_count"`
	FeedbackCount        int                      `json:"feedback_count"`
	AverageRating        float64                  `json:"average_rating"`
	UpdatedAt            time.Time                `json:"updated_at"`
}

type PlannerWeightSnapshot struct {
	ID             string                  `json:"id"`
	OrganizationID contract.OrganizationID `json:"organization_id"`
	ProjectID      contract.ProjectID      `json:"project_id"`
	AssetWeights   []PlannerAssetWeight    `json:"asset_weights"`
	CreatedBy      contract.Principal      `json:"created_by"`
	CreatedAt      time.Time               `json:"created_at"`
}

type PlannerAssetWeight struct {
	AssetVersion contract.AssetVersionRef `json:"asset_version"`
	Weight       float64                  `json:"weight"`
	Reasons      []string                 `json:"reasons"`
}

type FeedbackEventStore interface {
	CreateFeedbackEvent(context.Context, FeedbackEvent) (FeedbackEvent, error)
	ListFeedbackEvents(context.Context, contract.OrganizationID, contract.ProjectID, FeedbackEventFilter) ([]FeedbackEvent, error)
}

type MemoryFeedbackEventStore struct {
	mu     sync.RWMutex
	events []FeedbackEvent
}

func NewMemoryFeedbackEventStore() *MemoryFeedbackEventStore {
	return &MemoryFeedbackEventStore{}
}

func (s *MemoryFeedbackEventStore) CreateFeedbackEvent(_ context.Context, event FeedbackEvent) (FeedbackEvent, error) {
	if err := event.Validate(); err != nil {
		return FeedbackEvent{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, cloneFeedbackEvent(event))
	return cloneFeedbackEvent(event), nil
}

func (s *MemoryFeedbackEventStore) ListFeedbackEvents(_ context.Context, org contract.OrganizationID, project contract.ProjectID, filter FeedbackEventFilter) ([]FeedbackEvent, error) {
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 50
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	events := make([]FeedbackEvent, 0, len(s.events))
	for _, event := range s.events {
		if event.OrganizationID != org || event.ProjectID != project {
			continue
		}
		if filter.TargetType != "" && event.TargetType != filter.TargetType {
			continue
		}
		if filter.TargetID != "" && event.TargetID != filter.TargetID {
			continue
		}
		events = append(events, cloneFeedbackEvent(event))
	}
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].CreatedAt.After(events[j].CreatedAt)
	})
	if len(events) > filter.Limit {
		events = events[:filter.Limit]
	}
	return events, nil
}

func (r CreateFeedbackEventRequest) Validate() error {
	if r.EventType != FeedbackEventRating && r.EventType != FeedbackEventComment && r.EventType != FeedbackEventAssetSelected && r.EventType != FeedbackEventRenderSucceeded {
		return fmt.Errorf("event_type is invalid")
	}
	if r.TargetType != FeedbackTargetRemixPlan && r.TargetType != FeedbackTargetRenderJob && r.TargetType != FeedbackTargetAsset {
		return fmt.Errorf("target_type is invalid")
	}
	if strings.TrimSpace(r.TargetID) == "" || len(r.TargetID) > 160 {
		return fmt.Errorf("target_id must be between 1 and 160 characters")
	}
	if r.AssetVersion != nil {
		if err := r.AssetVersion.Validate(); err != nil {
			return err
		}
	}
	if r.EventType == FeedbackEventRating && (r.Rating < 1 || r.Rating > 5) {
		return fmt.Errorf("rating must be between 1 and 5")
	}
	if r.EventType == FeedbackEventComment && strings.TrimSpace(r.Comment) == "" {
		return fmt.Errorf("comment is required")
	}
	if len(r.Comment) > 1000 {
		return fmt.Errorf("comment is too long")
	}
	return nil
}

func (e FeedbackEvent) Validate() error {
	if strings.TrimSpace(e.ID) == "" {
		return fmt.Errorf("feedback event id is required")
	}
	if strings.TrimSpace(string(e.OrganizationID)) == "" || strings.TrimSpace(string(e.ProjectID)) == "" {
		return fmt.Errorf("feedback event scope is incomplete")
	}
	return CreateFeedbackEventRequest{
		EventType:    e.EventType,
		TargetType:   e.TargetType,
		TargetID:     e.TargetID,
		AssetVersion: e.AssetVersion,
		Rating:       e.Rating,
		Comment:      e.Comment,
	}.Validate()
}

func cloneFeedbackEvent(event FeedbackEvent) FeedbackEvent {
	if event.AssetVersion != nil {
		ref := *event.AssetVersion
		event.AssetVersion = &ref
	}
	return event
}

func cloneAssetPerformance(values []AssetPerformance) []AssetPerformance {
	return append([]AssetPerformance(nil), values...)
}

func clonePlannerWeightSnapshot(snapshot PlannerWeightSnapshot) PlannerWeightSnapshot {
	snapshot.AssetWeights = append([]PlannerAssetWeight(nil), snapshot.AssetWeights...)
	for index := range snapshot.AssetWeights {
		snapshot.AssetWeights[index].Reasons = append([]string(nil), snapshot.AssetWeights[index].Reasons...)
	}
	return snapshot
}
