package delivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func (r MySQLRepository) CreateOrGetMechanisticSimulation(ctx context.Context, value MechanisticSimulationResult, fingerprint string) (MechanisticSimulationResult, bool, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return MechanisticSimulationResult{}, false, err
	}
	result, err := r.DB.ExecContext(ctx, `INSERT IGNORE INTO delivery_mechanistic_simulation_runs
		(id,organization_id,project_id,plan_id,plan_version,fingerprint,input_snapshot_hash,model_version,prior_set_version,stable_seed,result_json)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`, value.ID, value.OrganizationID, value.ProjectID, value.PlanID, value.PlanVersion, fingerprint, value.InputSnapshotHash, value.ModelVersion, value.PriorSetVersion, value.StableSeed, payload)
	if err != nil {
		return MechanisticSimulationResult{}, false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return MechanisticSimulationResult{}, false, err
	}
	if affected == 1 {
		return value, false, nil
	}
	stored, err := scanMechanisticSimulation(r.DB.QueryRowContext(ctx, `SELECT result_json FROM delivery_mechanistic_simulation_runs WHERE organization_id=? AND project_id=? AND fingerprint=?`, value.OrganizationID, value.ProjectID, fingerprint))
	return stored, true, err
}

func (r MySQLRepository) GetMechanisticSimulation(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, id string) (MechanisticSimulationResult, error) {
	value, err := scanMechanisticSimulation(r.DB.QueryRowContext(ctx, `SELECT result_json FROM delivery_mechanistic_simulation_runs WHERE organization_id=? AND project_id=? AND id=?`, organizationID, projectID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return MechanisticSimulationResult{}, ErrNotFound
	}
	return value, err
}

func scanMechanisticSimulation(row rowScanner) (MechanisticSimulationResult, error) {
	var payload []byte
	if err := row.Scan(&payload); err != nil {
		return MechanisticSimulationResult{}, err
	}
	var value MechanisticSimulationResult
	if err := json.Unmarshal(payload, &value); err != nil {
		return MechanisticSimulationResult{}, fmt.Errorf("decode mechanistic simulation: %w", err)
	}
	return value, nil
}
