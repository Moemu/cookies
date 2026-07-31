package strategy

import (
	"context"
	"encoding/json"
	"time"

	"github.com/shikanon/cookies/internal/platform/agent"
	"github.com/shikanon/cookies/internal/platform/contract"
)

// HandleAgentTaskFinalFailure updates domain state only after AgentTask retries
// are exhausted. Transient failures deliberately leave resources in their
// running state so a later attempt can commit its result.
func (s Service) HandleAgentTaskFinalFailure(task agent.Task, _ contract.JobError) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	switch task.Kind {
	case AgentKindDraftGenerate, AgentKindDraftRevise:
		status := "failed"
		var currentRevision int64
		if err := s.DB.QueryRowContext(ctx, `SELECT current_revision FROM strategy_drafts
			WHERE organization_id = ? AND project_id = ? AND id = ?`,
			task.OrganizationID, task.ProjectID, task.SourceID).Scan(&currentRevision); err != nil {
			return
		}
		if currentRevision > 0 {
			status = "draft"
		}
		_, _ = s.DB.ExecContext(ctx, `UPDATE strategy_drafts SET status = ?, version = version + 1,
			updated_at = UTC_TIMESTAMP(6) WHERE organization_id = ? AND project_id = ? AND id = ?
			AND status = 'generating'`, status, task.OrganizationID, task.ProjectID, task.SourceID)
	case AgentKindReviewDeep:
		var input struct {
			AnalysisID string `json:"analysis_id"`
		}
		if json.Unmarshal(task.InputSnapshot, &input) != nil || input.AnalysisID == "" {
			return
		}
		_, _ = s.DB.ExecContext(ctx, `UPDATE strategy_review_analyses
			SET status = 'failed', updated_at = UTC_TIMESTAMP(6)
			WHERE organization_id = ? AND project_id = ? AND id = ? AND status = 'pending'`,
			task.OrganizationID, task.ProjectID, input.AnalysisID)
	case AgentKindCreativeTaskGenerate:
		_, _ = s.DB.ExecContext(ctx, `UPDATE strategy_creative_task_plans
			SET status = 'failed', version = version + 1, updated_at = UTC_TIMESTAMP(6)
			WHERE organization_id = ? AND project_id = ? AND id = ?
			  AND status = 'generating' AND current_agent_task_id = ?`,
			task.OrganizationID, task.ProjectID, task.SourceID, task.ID)
	}
}
