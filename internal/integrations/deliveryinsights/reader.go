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
	value, metric, plan, err := r.Service.ReadExecutionEvidence(ctx, actor, projectID, executionID)
	if err != nil {
		return insights.DeliveryExecutionSnapshot{}, err
	}
	result := mapExecution(value, plan)
	if metric != nil {
		result.MetricSnapshot = &insights.DeliveryMetricSnapshot{
			ID: metric.ID, Source: metric.Source, IsSimulated: metric.IsSimulated,
			DatasetVersion: metric.DatasetVersion, Currency: metric.Currency,
			RawMetrics: insights.RawMetrics{
				Impressions: metric.RawMetrics.Impressions, Clicks: metric.RawMetrics.Clicks,
				Conversions: metric.RawMetrics.Conversions, SpendCents: metric.RawMetrics.SpendCents,
			},
		}
	}
	return result, nil
}

func (r Reader) ListExecutions(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, limit int) ([]insights.DeliveryExecutionSnapshot, error) {
	values, err := r.Service.ListExecutionEvidence(ctx, actor, projectID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]insights.DeliveryExecutionSnapshot, 0, len(values))
	for _, value := range values {
		snapshot, readErr := r.ReadExecution(ctx, actor, projectID, value.Execution.ID)
		if readErr != nil {
			return nil, readErr
		}
		result = append(result, snapshot)
	}
	return result, nil
}

func mapExecution(value delivery.ExecutionResult, plan delivery.DeliveryPlan) insights.DeliveryExecutionSnapshot {
	return insights.DeliveryExecutionSnapshot{
		ID: value.Execution.ID, ChangeSetID: value.Execution.ChangeSetID, PlanID: value.ChangeSet.PlanID,
		CreativePackageID: plan.CreativePackageID, Mode: value.Execution.Mode,
		EvidenceID: value.Evidence.ID, EvidenceSummary: value.Evidence.Summary,
	}
}
