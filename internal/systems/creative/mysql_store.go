package creative

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type MySQLStore struct{ DB *sql.DB }

func (s MySQLStore) Create(ctx context.Context, plan Plan) (Plan, error) {
	if s.DB == nil {
		return Plan{}, fmt.Errorf("creative database is required")
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO creative_plans
		(id, organization_id, project_id, strategy_output_id, media_type, variant, prompt, model_alias)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, plan.ID, plan.OrganizationID, plan.ProjectID, plan.StrategyOutputID,
		plan.MediaType, plan.Variant, plan.Prompt, plan.ModelAlias)
	if err != nil {
		return Plan{}, err
	}
	return s.Get(ctx, plan.OrganizationID, plan.ProjectID, plan.ID)
}

func (s MySQLStore) Get(ctx context.Context, org contract.OrganizationID, projectID contract.ProjectID, planID string) (Plan, error) {
	if s.DB == nil {
		return Plan{}, fmt.Errorf("creative database is required")
	}
	var plan Plan
	err := s.DB.QueryRowContext(ctx, `SELECT id, organization_id, project_id, strategy_output_id, media_type, variant, prompt, model_alias, created_at
		FROM creative_plans WHERE organization_id=? AND project_id=? AND id=?`, org, projectID, planID).
		Scan(&plan.ID, &plan.OrganizationID, &plan.ProjectID, &plan.StrategyOutputID, &plan.MediaType, &plan.Variant, &plan.Prompt, &plan.ModelAlias, &plan.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Plan{}, ErrPlanNotFound
	}
	return plan, err
}

var _ Store = MySQLStore{}
