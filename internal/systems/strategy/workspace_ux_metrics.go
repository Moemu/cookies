package strategy

import (
	"context"
	"database/sql"
	"math"
	"sort"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const WorkspaceUXMetricsContractV1 = "strategy-workspace-ux-metrics/v1"

type WorkspaceUXMetricsWindow struct {
	Days int       `json:"days"`
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type WorkspaceUXLatencyMetric struct {
	Samples int    `json:"samples"`
	P50MS   *int64 `json:"p50_ms"`
	P95MS   *int64 `json:"p95_ms"`
}

type WorkspaceUXAssistantMetrics struct {
	Commands                 int64                    `json:"commands"`
	MissingAcknowledgements  int64                    `json:"missing_acknowledgements"`
	MissingMeaningfulUpdates int64                    `json:"missing_meaningful_updates"`
	FirstAcknowledgement     WorkspaceUXLatencyMetric `json:"first_acknowledgement"`
	FirstMeaningfulUpdate    WorkspaceUXLatencyMetric `json:"first_meaningful_update"`
}

type WorkspaceUXResearchMetrics struct {
	Runs                 int64    `json:"runs"`
	Completed            int64    `json:"completed"`
	PartiallyCompleted   int64    `json:"partially_completed"`
	Failed               int64    `json:"failed"`
	Cancelled            int64    `json:"cancelled"`
	VerifiedFindings     int64    `json:"verified_findings"`
	ConflictingFindings  int64    `json:"conflicting_findings"`
	ProposalsApplied     int64    `json:"proposals_applied"`
	ProposalsStale       int64    `json:"proposals_stale"`
	ProposalAdoptionRate *float64 `json:"proposal_adoption_rate"`
}

type WorkspaceUXDocumentMetrics struct {
	ParseStarted         int64                    `json:"parse_started"`
	Ready                int64                    `json:"ready"`
	Partial              int64                    `json:"partial"`
	Failed               int64                    `json:"failed"`
	VisionAttempts       int64                    `json:"vision_attempts"`
	VisionSucceeded      int64                    `json:"vision_succeeded"`
	VisionPartial        int64                    `json:"vision_partial"`
	VisionFailed         int64                    `json:"vision_failed"`
	BillableVisionPages  int64                    `json:"billable_vision_pages"`
	TerminalParseLatency WorkspaceUXLatencyMetric `json:"terminal_parse_latency"`
	VisionLatency        WorkspaceUXLatencyMetric `json:"vision_latency"`
}

type WorkspaceUXRecoveryMetrics struct {
	Stalled int64 `json:"stalled"`
	Retried int64 `json:"retried"`
}

type WorkspaceUXMetrics struct {
	ContractVersion    string                      `json:"contract_version"`
	Window             WorkspaceUXMetricsWindow    `json:"window"`
	Assistant          WorkspaceUXAssistantMetrics `json:"assistant"`
	Research           WorkspaceUXResearchMetrics  `json:"research"`
	Documents          WorkspaceUXDocumentMetrics  `json:"documents"`
	Recovery           WorkspaceUXRecoveryMetrics  `json:"recovery"`
	Interpretation     string                      `json:"interpretation"`
	TimeSavingMeasured bool                        `json:"time_saving_measured"`
	TimeSavingReason   string                      `json:"time_saving_reason"`
}

func (s Service) GetWorkspaceUXMetrics(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	days int,
) (WorkspaceUXMetrics, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return WorkspaceUXMetrics{}, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return WorkspaceUXMetrics{}, err
	}
	if s.DB == nil {
		return WorkspaceUXMetrics{}, ErrInvalidState
	}
	if days == 0 {
		days = 30
	}
	if days < 1 || days > 90 {
		return WorkspaceUXMetrics{}, ErrInvalidRequest
	}
	to := s.now()
	from := to.AddDate(0, 0, -days)
	value := WorkspaceUXMetrics{
		ContractVersion:    WorkspaceUXMetricsContractV1,
		Window:             WorkspaceUXMetricsWindow{Days: days, From: from, To: to},
		Interpretation:     "observed_workflow_evidence_not_causal_effect",
		TimeSavingMeasured: false,
		TimeSavingReason:   "human_correction_baseline_not_collected",
	}
	if err := s.loadWorkspaceUXProductEvents(ctx, actor.OrganizationID, projectID, from, to, &value); err != nil {
		return WorkspaceUXMetrics{}, err
	}
	if err := s.loadWorkspaceUXResearch(ctx, actor.OrganizationID, projectID, from, to, &value.Research); err != nil {
		return WorkspaceUXMetrics{}, err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COALESCE(SUM(billable_pages), 0)
		FROM platform_knowledge_document_vision_tasks
		WHERE organization_id = ? AND project_id = ? AND updated_at >= ? AND updated_at < ?`,
		actor.OrganizationID, projectID, from, to,
	).Scan(&value.Documents.BillableVisionPages); err != nil {
		return WorkspaceUXMetrics{}, err
	}
	return value, nil
}

func (s Service) loadWorkspaceUXProductEvents(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	from, to time.Time,
	metrics *WorkspaceUXMetrics,
) error {
	rows, err := s.DB.QueryContext(ctx, `SELECT event_type, resource_id, duration_ms, COALESCE(outcome, '')
		FROM strategy_product_events
		WHERE organization_id = ? AND project_id = ? AND occurred_at >= ? AND occurred_at < ?
		  AND event_type IN (
			'assistant.command_submitted', 'assistant.first_ack', 'assistant.first_meaningful_update',
			'document.parse_started', 'document.ready', 'document.partial', 'document.failed',
			'document.vision_fallback', 'activity.stalled', 'activity.retried'
		  )`, organizationID, projectID, from, to)
	if err != nil {
		return err
	}
	defer rows.Close()
	commands, acknowledgements, meaningful := map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}
	ackDurations, meaningfulDurations := []int64{}, []int64{}
	parseDurations, visionDurations := []int64{}, []int64{}
	for rows.Next() {
		var eventType, resourceID, outcome string
		var duration sql.NullInt64
		if err := rows.Scan(&eventType, &resourceID, &duration, &outcome); err != nil {
			return err
		}
		switch eventType {
		case ProductEventAssistantCommandSubmitted:
			commands[resourceID] = struct{}{}
		case ProductEventAssistantFirstAck:
			acknowledgements[resourceID] = struct{}{}
			appendValidMetricDuration(&ackDurations, duration)
		case ProductEventAssistantFirstMeaningfulUpdate:
			meaningful[resourceID] = struct{}{}
			appendValidMetricDuration(&meaningfulDurations, duration)
		case ProductEventDocumentParseStarted:
			metrics.Documents.ParseStarted++
		case ProductEventDocumentReady:
			metrics.Documents.Ready++
			appendValidMetricDuration(&parseDurations, duration)
		case ProductEventDocumentPartial:
			metrics.Documents.Partial++
			appendValidMetricDuration(&parseDurations, duration)
		case ProductEventDocumentFailed:
			metrics.Documents.Failed++
			appendValidMetricDuration(&parseDurations, duration)
		case ProductEventDocumentVisionFallback:
			metrics.Documents.VisionAttempts++
			appendValidMetricDuration(&visionDurations, duration)
			switch outcome {
			case "succeeded":
				metrics.Documents.VisionSucceeded++
			case "partial":
				metrics.Documents.VisionPartial++
			case "failed":
				metrics.Documents.VisionFailed++
			}
		case ProductEventActivityStalled:
			metrics.Recovery.Stalled++
		case ProductEventActivityRetried:
			metrics.Recovery.Retried++
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	metrics.Assistant.Commands = int64(len(commands))
	metrics.Assistant.MissingAcknowledgements = missingMetricResources(commands, acknowledgements)
	metrics.Assistant.MissingMeaningfulUpdates = missingMetricResources(commands, meaningful)
	metrics.Assistant.FirstAcknowledgement = latencyMetric(ackDurations)
	metrics.Assistant.FirstMeaningfulUpdate = latencyMetric(meaningfulDurations)
	metrics.Documents.TerminalParseLatency = latencyMetric(parseDurations)
	metrics.Documents.VisionLatency = latencyMetric(visionDurations)
	return nil
}

func (s Service) loadWorkspaceUXResearch(
	ctx context.Context,
	organizationID contract.OrganizationID,
	projectID contract.ProjectID,
	from, to time.Time,
	metrics *WorkspaceUXResearchMetrics,
) error {
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*),
		COALESCE(SUM(status = 'completed'), 0), COALESCE(SUM(status = 'partially_completed'), 0),
		COALESCE(SUM(status = 'failed'), 0), COALESCE(SUM(status = 'cancelled'), 0)
		FROM platform_research_runs
		WHERE organization_id = ? AND project_id = ? AND created_at >= ? AND created_at < ?`,
		organizationID, projectID, from, to,
	).Scan(&metrics.Runs, &metrics.Completed, &metrics.PartiallyCompleted, &metrics.Failed, &metrics.Cancelled); err != nil {
		return err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(finding.status = 'verified'), 0), COALESCE(SUM(finding.status = 'conflicting'), 0)
		FROM platform_research_findings finding
		JOIN platform_research_runs run
		  ON run.organization_id = finding.organization_id AND run.project_id = finding.project_id
		 AND run.id = finding.research_run_id
		WHERE run.organization_id = ? AND run.project_id = ? AND run.created_at >= ? AND run.created_at < ?`,
		organizationID, projectID, from, to,
	).Scan(&metrics.VerifiedFindings, &metrics.ConflictingFindings); err != nil {
		return err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(proposal.status = 'applied'), 0), COALESCE(SUM(proposal.status = 'stale'), 0)
		FROM strategy_artifact_proposals proposal
		JOIN platform_research_runs run
		  ON run.organization_id = proposal.organization_id AND run.project_id = proposal.project_id
		 AND run.id = proposal.source_research_run_id
		WHERE run.organization_id = ? AND run.project_id = ? AND run.created_at >= ? AND run.created_at < ?`,
		organizationID, projectID, from, to,
	).Scan(&metrics.ProposalsApplied, &metrics.ProposalsStale); err != nil {
		return err
	}
	decided := metrics.ProposalsApplied + metrics.ProposalsStale
	if decided > 0 {
		rate := roundedMetric(float64(metrics.ProposalsApplied) / float64(decided))
		metrics.ProposalAdoptionRate = &rate
	}
	return nil
}

func appendValidMetricDuration(target *[]int64, value sql.NullInt64) {
	if value.Valid && value.Int64 >= 0 {
		*target = append(*target, value.Int64)
	}
}

func missingMetricResources(expected, observed map[string]struct{}) int64 {
	var missing int64
	for resourceID := range expected {
		if _, ok := observed[resourceID]; !ok {
			missing++
		}
	}
	return missing
}

func latencyMetric(values []int64) WorkspaceUXLatencyMetric {
	return WorkspaceUXLatencyMetric{Samples: len(values), P50MS: percentileMetric(values, 0.50), P95MS: percentileMetric(values, 0.95)}
}

func percentileMetric(values []int64, percentile float64) *int64 {
	if len(values) == 0 || percentile <= 0 || percentile > 1 {
		return nil
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(left, right int) bool { return sorted[left] < sorted[right] })
	index := int(math.Ceil(percentile*float64(len(sorted)))) - 1
	value := sorted[max(0, min(index, len(sorted)-1))]
	return &value
}
