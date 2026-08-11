package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"sort"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const P0MetricsContractV1 = "strategy-p0-metrics/v1"

type P0MetricsWindow struct {
	Days int       `json:"days"`
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type P0MetricsFunnel struct {
	ConversationsStarted  int64 `json:"conversations_started"`
	ConversationsEngaged  int64 `json:"conversations_engaged"`
	RequirementsConfirmed int64 `json:"requirements_confirmed"`
	StrategiesStarted     int64 `json:"strategies_started"`
	PackagesPublished     int64 `json:"packages_published"`
	CreativeTasksCreated  int64 `json:"creative_tasks_created"`
}

type P0MetricsTurns struct {
	UserTurns        int64 `json:"user_turns"`
	AssistantTurns   int64 `json:"assistant_turns"`
	FailedAgentTurns int64 `json:"failed_agent_turns"`
	DeepTurns        int64 `json:"deep_turns"`
	WebSearchTurns   int64 `json:"web_search_turns"`
	DocumentRefTurns int64 `json:"document_ref_turns"`
	MediaRefTurns    int64 `json:"media_ref_turns"`
	ResearchRefTurns int64 `json:"research_ref_turns"`
}

type P0MetricsPaths struct {
	QuickIntakes      int64 `json:"quick_intakes"`
	QuickReadyIntakes int64 `json:"quick_ready_intakes"`
	FullIntakes       int64 `json:"full_intakes"`
	FullReadyIntakes  int64 `json:"full_ready_intakes"`
}

type P0MetricsTimings struct {
	RequirementSamples              int      `json:"requirement_samples"`
	MedianSecondsToRequirement      *int64   `json:"median_seconds_to_requirement"`
	AverageUserTurnsToRequirement   *float64 `json:"average_user_turns_to_requirement"`
	QuickTaskSamples                int      `json:"quick_task_samples"`
	MedianSecondsToQuickTask        *int64   `json:"median_seconds_to_quick_task"`
	PublishedPackageSamples         int      `json:"published_package_samples"`
	MedianSecondsToPublishedPackage *int64   `json:"median_seconds_to_published_package"`
}

type P0MetricsFeedback struct {
	Responses    int64    `json:"responses"`
	Useful       int64    `json:"useful"`
	PartlyUseful int64    `json:"partly_useful"`
	NotUseful    int64    `json:"not_useful"`
	UsefulRate   *float64 `json:"useful_rate"`
}

type P0Metrics struct {
	ContractVersion string            `json:"contract_version"`
	Window          P0MetricsWindow   `json:"window"`
	Funnel          P0MetricsFunnel   `json:"funnel"`
	Turns           P0MetricsTurns    `json:"turns"`
	Paths           P0MetricsPaths    `json:"paths"`
	Timings         P0MetricsTimings  `json:"timings"`
	Feedback        P0MetricsFeedback `json:"feedback"`
	Interpretation  string            `json:"interpretation"`
}

func (s Service) GetP0Metrics(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	days int,
) (P0Metrics, error) {
	if err := requireScope(actor, ScopeRead); err != nil {
		return P0Metrics{}, err
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return P0Metrics{}, err
	}
	if s.DB == nil {
		return P0Metrics{}, ErrInvalidState
	}
	if days == 0 {
		days = 30
	}
	if days < 1 || days > 90 {
		return P0Metrics{}, ErrInvalidRequest
	}
	to := s.now()
	from := to.AddDate(0, 0, -days)
	value := P0Metrics{
		ContractVersion: P0MetricsContractV1,
		Window:          P0MetricsWindow{Days: days, From: from, To: to},
		Interpretation:  "observed_activity_not_causal_effect",
	}

	counts := []struct {
		target *int64
		query  string
	}{
		{&value.Funnel.ConversationsStarted, `SELECT COUNT(*) FROM strategy_conversations WHERE organization_id = ? AND project_id = ? AND created_at >= ? AND created_at < ?`},
		{&value.Funnel.ConversationsEngaged, `SELECT COUNT(DISTINCT conversation_id) FROM strategy_messages WHERE organization_id = ? AND project_id = ? AND role = 'user' AND created_at >= ? AND created_at < ?`},
		{&value.Funnel.RequirementsConfirmed, `SELECT COUNT(DISTINCT brief_id) FROM strategy_brief_versions WHERE organization_id = ? AND project_id = ? AND confirmed_at >= ? AND confirmed_at < ?`},
		{&value.Funnel.StrategiesStarted, `SELECT COUNT(*) FROM strategy_drafts WHERE organization_id = ? AND project_id = ? AND created_at >= ? AND created_at < ?`},
		{&value.Funnel.PackagesPublished, `SELECT COUNT(*) FROM strategy_package_versions WHERE organization_id = ? AND project_id = ? AND published_at >= ? AND published_at < ?`},
		{&value.Funnel.CreativeTasksCreated, `SELECT COUNT(*) FROM creative_tasks WHERE organization_id = ? AND project_id = ? AND created_at >= ? AND created_at < ?`},
		{&value.Turns.FailedAgentTurns, `SELECT COUNT(*) FROM platform_agent_tasks WHERE organization_id = ? AND project_id = ? AND kind = 'strategy.brief.extract' AND status = 'failed' AND created_at >= ? AND created_at < ?`},
	}
	for _, count := range counts {
		if err := s.DB.QueryRowContext(ctx, count.query, actor.OrganizationID, projectID, from, to).Scan(count.target); err != nil {
			return P0Metrics{}, err
		}
	}
	if err := s.loadP0TurnMetrics(ctx, actor, projectID, from, to, &value.Turns); err != nil {
		return P0Metrics{}, err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT
		COALESCE(SUM(source_type = 'requirement_snapshot'), 0),
		COALESCE(SUM(source_type = 'requirement_snapshot' AND status = 'ready'), 0),
		COALESCE(SUM(source_type IN ('strategy_package', 'task_strategy')), 0),
		COALESCE(SUM(source_type IN ('strategy_package', 'task_strategy') AND status = 'ready'), 0)
		FROM creative_intakes
		WHERE organization_id = ? AND project_id = ? AND created_at >= ? AND created_at < ?`,
		actor.OrganizationID, projectID, from, to,
	).Scan(&value.Paths.QuickIntakes, &value.Paths.QuickReadyIntakes, &value.Paths.FullIntakes, &value.Paths.FullReadyIntakes); err != nil {
		return P0Metrics{}, err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT
		COUNT(*),
		COALESCE(SUM(rating = 'useful'), 0),
		COALESCE(SUM(rating = 'partly_useful'), 0),
		COALESCE(SUM(rating = 'not_useful'), 0)
		FROM strategy_feedback
		WHERE organization_id = ? AND project_id = ? AND created_at >= ? AND created_at < ?`,
		actor.OrganizationID, projectID, from, to,
	).Scan(&value.Feedback.Responses, &value.Feedback.Useful, &value.Feedback.PartlyUseful, &value.Feedback.NotUseful); err != nil {
		return P0Metrics{}, err
	}
	if value.Feedback.Responses > 0 {
		rate := roundedMetric(float64(value.Feedback.Useful) / float64(value.Feedback.Responses))
		value.Feedback.UsefulRate = &rate
	}
	if err := s.loadP0TimingMetrics(ctx, actor, projectID, from, to, &value.Timings); err != nil {
		return P0Metrics{}, err
	}
	return value, nil
}

func (s Service) loadP0TurnMetrics(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	from, to time.Time,
	metrics *P0MetricsTurns,
) error {
	rows, err := s.DB.QueryContext(ctx, `SELECT role, content_blocks, requested_policy
		FROM strategy_messages
		WHERE organization_id = ? AND project_id = ? AND created_at >= ? AND created_at < ?`,
		actor.OrganizationID, projectID, from, to)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var role string
		var blocksJSON, policyJSON []byte
		if err := rows.Scan(&role, &blocksJSON, &policyJSON); err != nil {
			return err
		}
		if role == "assistant" {
			metrics.AssistantTurns++
			continue
		}
		if role != "user" {
			continue
		}
		metrics.UserTurns++
		var policy MessageRequestedPolicy
		if len(policyJSON) > 0 && json.Unmarshal(policyJSON, &policy) == nil {
			if policy.ReasoningMode == "deep" {
				metrics.DeepTurns++
			}
			if policy.WebSearch == "allowed" {
				metrics.WebSearchTurns++
			}
		}
		var blocks []MessageContentBlock
		if len(blocksJSON) == 0 || json.Unmarshal(blocksJSON, &blocks) != nil {
			continue
		}
		var hasDocument, hasMedia, hasResearch bool
		for _, block := range blocks {
			switch block.Type {
			case "document_ref":
				hasDocument = true
			case "asset_ref":
				hasMedia = true
			case "research_ref":
				hasResearch = true
			}
		}
		if hasDocument {
			metrics.DocumentRefTurns++
		}
		if hasMedia {
			metrics.MediaRefTurns++
		}
		if hasResearch {
			metrics.ResearchRefTurns++
		}
	}
	return rows.Err()
}

func (s Service) loadP0TimingMetrics(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	from, to time.Time,
	metrics *P0MetricsTimings,
) error {
	requirementDurations := make([]int64, 0)
	userTurns := make([]int64, 0)
	rows, err := s.DB.QueryContext(ctx, `SELECT
		TIMESTAMPDIFF(SECOND, conversation.created_at, confirmation.confirmed_at),
		(SELECT COUNT(*) FROM strategy_messages message
		 WHERE message.organization_id = conversation.organization_id
		   AND message.project_id = conversation.project_id
		   AND message.conversation_id = conversation.id
		   AND message.role = 'user'
		   AND message.created_at <= confirmation.confirmed_at)
		FROM strategy_conversations conversation
		JOIN strategy_tasks task
		  ON task.organization_id = conversation.organization_id
		 AND task.project_id = conversation.project_id
		 AND task.conversation_id = conversation.id
		JOIN (
		  SELECT organization_id, project_id, brief_id, MIN(confirmed_at) AS confirmed_at
		  FROM strategy_brief_versions
		  WHERE organization_id = ? AND project_id = ?
		  GROUP BY organization_id, project_id, brief_id
		) confirmation
		  ON confirmation.organization_id = task.organization_id
		 AND confirmation.project_id = task.project_id
		 AND confirmation.brief_id = task.brief_id
		WHERE conversation.organization_id = ? AND conversation.project_id = ?
		  AND conversation.created_at >= ? AND conversation.created_at < ?
		  AND confirmation.confirmed_at >= conversation.created_at`,
		actor.OrganizationID, projectID, actor.OrganizationID, projectID, from, to)
	if err != nil {
		return err
	}
	for rows.Next() {
		var duration, turns int64
		if err := rows.Scan(&duration, &turns); err != nil {
			rows.Close()
			return err
		}
		requirementDurations = append(requirementDurations, duration)
		userTurns = append(userTurns, turns)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	metrics.RequirementSamples = len(requirementDurations)
	metrics.MedianSecondsToRequirement = medianMetric(requirementDurations)
	if len(userTurns) > 0 {
		var total int64
		for _, turns := range userTurns {
			total += turns
		}
		average := roundedMetric(float64(total) / float64(len(userTurns)))
		metrics.AverageUserTurnsToRequirement = &average
	}

	quickDurations, err := s.loadP0DurationSamples(ctx, actor, projectID, from, to, `SELECT
		TIMESTAMPDIFF(SECOND, conversation.created_at, MIN(creative_task.created_at))
		FROM strategy_conversations conversation
		JOIN strategy_tasks strategy_task
		  ON strategy_task.organization_id = conversation.organization_id
		 AND strategy_task.project_id = conversation.project_id
		 AND strategy_task.conversation_id = conversation.id
		JOIN creative_intakes intake
		  ON intake.organization_id = strategy_task.organization_id
		 AND intake.project_id = strategy_task.project_id
		 AND intake.requirement_brief_id = strategy_task.brief_id
		 AND intake.source_type = 'requirement_snapshot'
		JOIN creative_tasks creative_task
		  ON creative_task.organization_id = intake.organization_id
		 AND creative_task.project_id = intake.project_id
		 AND creative_task.intake_id = intake.id
		WHERE conversation.organization_id = ? AND conversation.project_id = ?
		  AND conversation.created_at >= ? AND conversation.created_at < ?
		GROUP BY conversation.id, conversation.created_at`)
	if err != nil {
		return err
	}
	metrics.QuickTaskSamples = len(quickDurations)
	metrics.MedianSecondsToQuickTask = medianMetric(quickDurations)

	publishedDurations, err := s.loadP0DurationSamples(ctx, actor, projectID, from, to, `SELECT
		TIMESTAMPDIFF(SECOND, conversation.created_at, MIN(package_version.published_at))
		FROM strategy_conversations conversation
		JOIN strategy_tasks task
		  ON task.organization_id = conversation.organization_id
		 AND task.project_id = conversation.project_id
		 AND task.conversation_id = conversation.id
		JOIN strategy_drafts strategy
		  ON strategy.organization_id = task.organization_id
		 AND strategy.project_id = task.project_id
		 AND strategy.task_id = task.id
		JOIN strategy_package_versions package_version
		  ON package_version.organization_id = strategy.organization_id
		 AND package_version.project_id = strategy.project_id
		 AND package_version.strategy_id = strategy.id
		WHERE conversation.organization_id = ? AND conversation.project_id = ?
		  AND conversation.created_at >= ? AND conversation.created_at < ?
		GROUP BY conversation.id, conversation.created_at`)
	if err != nil {
		return err
	}
	metrics.PublishedPackageSamples = len(publishedDurations)
	metrics.MedianSecondsToPublishedPackage = medianMetric(publishedDurations)
	return nil
}

func (s Service) loadP0DurationSamples(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	from, to time.Time,
	query string,
) ([]int64, error) {
	rows, err := s.DB.QueryContext(ctx, query, actor.OrganizationID, projectID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]int64, 0)
	for rows.Next() {
		var value sql.NullInt64
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		if value.Valid && value.Int64 >= 0 {
			values = append(values, value.Int64)
		}
	}
	return values, rows.Err()
}

func medianMetric(values []int64) *int64 {
	if len(values) == 0 {
		return nil
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(left, right int) bool { return sorted[left] < sorted[right] })
	middle := len(sorted) / 2
	value := sorted[middle]
	if len(sorted)%2 == 0 {
		value = (sorted[middle-1] + sorted[middle]) / 2
	}
	return &value
}

func roundedMetric(value float64) float64 {
	return math.Round(value*1000) / 1000
}
