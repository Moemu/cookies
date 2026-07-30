package delivery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type MySQLStore struct{ DB *sql.DB }

func (s MySQLStore) Create(ctx context.Context, plan DeliveryPlan, version DeliveryPlanVersion) (DeliveryPlan, error) {
	if s.DB == nil {
		return DeliveryPlan{}, fmt.Errorf("delivery database is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return DeliveryPlan{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO delivery_plans
		(organization_id, id, project_id, status, platform, source, scenario, current_version, created_by_kind, created_by_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		plan.OrganizationID, plan.ID, plan.ProjectID, plan.Status, plan.Platform, plan.Source, plan.Scenario,
		version.VersionNumber, plan.CreatedBy.Kind, plan.CreatedBy.ID, plan.CreatedAt, plan.UpdatedAt)
	if err != nil {
		return DeliveryPlan{}, err
	}
	if err := insertVersion(ctx, tx, version); err != nil {
		return DeliveryPlan{}, err
	}
	if err := tx.Commit(); err != nil {
		return DeliveryPlan{}, err
	}
	return s.Get(ctx, plan.OrganizationID, plan.ID)
}

func (s MySQLStore) Get(ctx context.Context, organizationID contract.OrganizationID, planID string) (DeliveryPlan, error) {
	if s.DB == nil {
		return DeliveryPlan{}, fmt.Errorf("delivery database is required")
	}
	var plan DeliveryPlan
	err := s.DB.QueryRowContext(ctx, `SELECT id, organization_id, project_id, status, platform, source, scenario,
		current_version, created_by_kind, created_by_id, created_at, updated_at
		FROM delivery_plans WHERE organization_id=? AND id=?`, organizationID, planID).
		Scan(&plan.ID, &plan.OrganizationID, &plan.ProjectID, &plan.Status, &plan.Platform, &plan.Source, &plan.Scenario,
			&plan.CurrentVersionNumber, &plan.CreatedBy.Kind, &plan.CreatedBy.ID, &plan.CreatedAt, &plan.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DeliveryPlan{}, ErrNotFound
	}
	if err != nil {
		return DeliveryPlan{}, err
	}
	versions, err := s.loadVersions(ctx, organizationID, plan.ID)
	if err != nil {
		return DeliveryPlan{}, err
	}
	plan.Versions = versions
	for _, version := range versions {
		if version.VersionNumber == plan.CurrentVersionNumber {
			plan.CurrentVersion = version
			break
		}
	}
	if plan.CurrentVersion.VersionNumber == 0 {
		return DeliveryPlan{}, fmt.Errorf("delivery plan current version is missing")
	}
	return plan, nil
}

func (s MySQLStore) List(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]DeliveryPlan, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("delivery database is required")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM delivery_plans
		WHERE organization_id=? AND project_id=? ORDER BY updated_at, id`, organizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	plans := make([]DeliveryPlan, 0, len(ids))
	for _, id := range ids {
		plan, err := s.Get(ctx, organizationID, id)
		if err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	return plans, nil
}

func (s MySQLStore) Update(ctx context.Context, organizationID contract.OrganizationID, planID string, expectedVersion int, version DeliveryPlanVersion) (DeliveryPlan, error) {
	if s.DB == nil {
		return DeliveryPlan{}, fmt.Errorf("delivery database is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return DeliveryPlan{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE delivery_plans
		SET current_version=?, scenario=?, updated_at=?
		WHERE organization_id=? AND id=? AND current_version=?`,
		version.VersionNumber, version.Scenario, version.CreatedAt, organizationID, planID, expectedVersion)
	if err != nil {
		return DeliveryPlan{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return DeliveryPlan{}, err
	}
	if affected == 0 {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM delivery_plans WHERE organization_id=? AND id=?`, organizationID, planID).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
			return DeliveryPlan{}, ErrNotFound
		} else if err != nil {
			return DeliveryPlan{}, err
		}
		return DeliveryPlan{}, ErrVersionConflict
	}
	if err := insertVersion(ctx, tx, version); err != nil {
		return DeliveryPlan{}, err
	}
	if err := tx.Commit(); err != nil {
		return DeliveryPlan{}, err
	}
	return s.Get(ctx, organizationID, planID)
}

func insertVersion(ctx context.Context, executor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, version DeliveryPlanVersion) error {
	config, err := json.Marshal(draftFromVersion(version))
	if err != nil {
		return err
	}
	_, err = executor.ExecContext(ctx, `INSERT INTO delivery_plan_versions
		(organization_id, project_id, plan_id, version_number, config_json, source, scenario, created_by_kind, created_by_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		version.OrganizationID, version.ProjectID, version.PlanID, version.VersionNumber, config,
		version.Source, version.Scenario, version.CreatedBy.Kind, version.CreatedBy.ID, version.CreatedAt)
	return err
}

func (s MySQLStore) loadVersions(ctx context.Context, organizationID contract.OrganizationID, planID string) ([]DeliveryPlanVersion, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT project_id, version_number, config_json, source, scenario,
		created_by_kind, created_by_id, created_at
		FROM delivery_plan_versions WHERE organization_id=? AND plan_id=? ORDER BY version_number`,
		organizationID, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var versions []DeliveryPlanVersion
	for rows.Next() {
		var version DeliveryPlanVersion
		var config []byte
		version.PlanID = planID
		version.OrganizationID = organizationID
		if err := rows.Scan(&version.ProjectID, &version.VersionNumber, &config, &version.Source, &version.Scenario,
			&version.CreatedBy.Kind, &version.CreatedBy.ID, &version.CreatedAt); err != nil {
			return nil, err
		}
		var draft PlanDraft
		if err := json.Unmarshal(config, &draft); err != nil {
			return nil, fmt.Errorf("decode delivery plan version: %w", err)
		}
		version.Name = draft.Name
		version.Objective = draft.Objective
		version.Advertiser = MockAdvertiser{
			ID: draft.Advertiser.ID, Name: draft.Advertiser.Name, Platform: draft.Advertiser.Platform,
			Source: version.Source, Scenario: version.Scenario,
		}
		version.Budget = draft.Budget
		version.Schedule = draft.Schedule
		version.Tracking = draft.Tracking
		version.CreativeReferences = draft.CreativeReferences
		version.SourceStrategyVersion = draft.SourceStrategyVersion
		versions = append(versions, version)
	}
	return versions, rows.Err()
}

var _ Store = MySQLStore{}
