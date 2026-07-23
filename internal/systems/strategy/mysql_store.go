package strategy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/strategy/prompts"
)

type MySQLStore struct{ DB *sql.DB }

func (s MySQLStore) CreateProposal(ctx context.Context, proposal Proposal) (Proposal, bool, error) {
	if s.DB == nil {
		return Proposal{}, false, fmt.Errorf("strategy database is required")
	}
	input, err := json.Marshal(proposal.Input)
	if err != nil {
		return Proposal{}, false, err
	}
	result, err := s.DB.ExecContext(ctx, `INSERT INTO strategy_proposals
		(id, organization_id, project_id, input_json, input_hash, template_version, status)
		VALUES (?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE id=id`,
		proposal.ID, proposal.OrganizationID, proposal.ProjectID, input, proposal.InputHash, proposal.TemplateVersion, proposal.Status)
	if err != nil {
		return Proposal{}, false, err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		var existingID string
		if err := s.DB.QueryRowContext(ctx, `SELECT id FROM strategy_proposals WHERE organization_id=? AND project_id=? AND input_hash=? AND template_version=?`,
			proposal.OrganizationID, proposal.ProjectID, proposal.InputHash, proposal.TemplateVersion).Scan(&existingID); err != nil {
			return Proposal{}, false, err
		}
		value, err := s.GetProposal(ctx, proposal.OrganizationID, proposal.ProjectID, existingID)
		return value, true, err
	}
	value, err := s.GetProposal(ctx, proposal.OrganizationID, proposal.ProjectID, proposal.ID)
	return value, false, err
}

func (s MySQLStore) GetProposal(ctx context.Context, org contract.OrganizationID, projectID contract.ProjectID, proposalID string) (Proposal, error) {
	var proposal Proposal
	var input []byte
	err := s.DB.QueryRowContext(ctx, `SELECT id, organization_id, project_id, input_json, input_hash, template_version, status, created_at, updated_at
		FROM strategy_proposals WHERE organization_id=? AND project_id=? AND id=?`, org, projectID, proposalID).
		Scan(&proposal.ID, &proposal.OrganizationID, &proposal.ProjectID, &input, &proposal.InputHash, &proposal.TemplateVersion, &proposal.Status, &proposal.CreatedAt, &proposal.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Proposal{}, ErrProposalNotFound
	}
	if err != nil {
		return Proposal{}, err
	}
	if err := json.Unmarshal(input, &proposal.Input); err != nil {
		return Proposal{}, fmt.Errorf("decode proposal input: %w", err)
	}
	return proposal, nil
}

func (s MySQLStore) CreateStrategy(ctx context.Context, strategy StrategyOutput) (StrategyOutput, error) {
	if s.DB == nil {
		return StrategyOutput{}, fmt.Errorf("strategy database is required")
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO strategy_outputs
		(id, proposal_id, organization_id, project_id, strategy_json, model_alias, model_version, provider_code)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, strategy.ID, strategy.ProposalID, strategy.OrganizationID, strategy.ProjectID,
		strategy.Content, strategy.ModelAlias, strategy.ModelVersion, strategy.ProviderCode)
	if err != nil {
		return StrategyOutput{}, err
	}
	if _, err := s.DB.ExecContext(ctx, `UPDATE strategy_proposals SET status='generated' WHERE organization_id=? AND project_id=? AND id=?`,
		strategy.OrganizationID, strategy.ProjectID, strategy.ProposalID); err != nil {
		return StrategyOutput{}, err
	}
	return s.GetStrategy(ctx, strategy.OrganizationID, strategy.ProjectID, strategy.ID)
}

func (s MySQLStore) GetStrategy(ctx context.Context, org contract.OrganizationID, projectID contract.ProjectID, strategyID string) (StrategyOutput, error) {
	return s.getStrategy(ctx, `id=?`, org, projectID, strategyID)
}

func (s MySQLStore) GetStrategyByProposal(ctx context.Context, org contract.OrganizationID, projectID contract.ProjectID, proposalID string) (StrategyOutput, error) {
	return s.getStrategy(ctx, `proposal_id=?`, org, projectID, proposalID)
}

func (s MySQLStore) getStrategy(ctx context.Context, condition string, org contract.OrganizationID, projectID contract.ProjectID, valueID string) (StrategyOutput, error) {
	var value StrategyOutput
	var approved sql.NullTime
	query := `SELECT id, proposal_id, organization_id, project_id, strategy_json, model_alias, model_version, provider_code, approved_at, created_at
		FROM strategy_outputs WHERE organization_id=? AND project_id=? AND ` + condition + ` ORDER BY created_at DESC LIMIT 1`
	err := s.DB.QueryRowContext(ctx, query, org, projectID, valueID).
		Scan(&value.ID, &value.ProposalID, &value.OrganizationID, &value.ProjectID, &value.Content, &value.ModelAlias, &value.ModelVersion, &value.ProviderCode, &approved, &value.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return StrategyOutput{}, ErrStrategyNotFound
	}
	if err != nil {
		return StrategyOutput{}, err
	}
	if approved.Valid {
		approvedAt := approved.Time.UTC()
		value.ApprovedAt = &approvedAt
	}
	return value, nil
}

func (s MySQLStore) ApproveStrategy(ctx context.Context, org contract.OrganizationID, projectID contract.ProjectID, strategyID string) (StrategyOutput, error) {
	result, err := s.DB.ExecContext(ctx, `UPDATE strategy_outputs SET approved_at=COALESCE(approved_at, ?) WHERE organization_id=? AND project_id=? AND id=?`,
		time.Now().UTC(), org, projectID, strategyID)
	if err != nil {
		return StrategyOutput{}, err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return StrategyOutput{}, ErrStrategyNotFound
	}
	value, err := s.GetStrategy(ctx, org, projectID, strategyID)
	if err != nil {
		return StrategyOutput{}, err
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE strategy_proposals SET status='approved' WHERE organization_id=? AND project_id=? AND id=?`, org, projectID, value.ProposalID)
	return value, err
}

var _ Store = MySQLStore{}
var _ = prompts.TemplateVersion
