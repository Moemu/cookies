// Package deliveryinsights adapts Delivery's execution evidence to the narrow
// immutable snapshot consumed by Insights.
package deliveryinsights

import (
	"context"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/delivery"
	"github.com/shikanon/cookies/internal/systems/insights"
)

type Reader struct {
	Service *delivery.Service
}

func (r Reader) ReadExecution(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, executionID string) (insights.DeliveryExecutionSnapshot, error) {
	values, err := r.ListExecutions(ctx, actor, projectID, 100)
	if err != nil {
		return insights.DeliveryExecutionSnapshot{}, err
	}
	for _, value := range values {
		if value.ID == executionID {
			return value, nil
		}
	}
	return insights.DeliveryExecutionSnapshot{}, insights.ErrNotFound
}

func (r Reader) ListExecutions(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]insights.DeliveryExecutionSnapshot, error) {
	values, err := r.Service.ListExecutionEvidence(ctx, actor, projectID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]insights.DeliveryExecutionSnapshot, 0, len(values))
	for _, value := range values {
		result = append(result, insights.DeliveryExecutionSnapshot{
			ID: value.Execution.ID, ChangeSetID: value.Execution.ChangeSetID, PlanID: value.ChangeSet.PlanID,
			Mode: value.Execution.Mode, EvidenceID: value.Evidence.ID, EvidenceSummary: value.Evidence.Summary,
		})
	}
	return result, nil
}
