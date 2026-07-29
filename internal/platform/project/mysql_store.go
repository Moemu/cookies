package project

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
)

type MySQLStore struct{ DB *sql.DB }

// EnsureLocalProject creates deterministic local-development seed data after
// migrations have run. It is never called outside the local environment.
func (s MySQLStore) EnsureLocalProject(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) error {
	if s.DB == nil || actor.Validate() != nil || projectID == "" {
		return fmt.Errorf("valid local project seed is required")
	}
	digest := sha256.Sum256([]byte(string(actor.OrganizationID) + "/" + string(projectID)))
	brandID := contract.BrandID("brand_local_" + hex.EncodeToString(digest[:8]))
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO brands (id, organization_id, name, status) VALUES (?, ?, 'Local Brand', 'active') ON DUPLICATE KEY UPDATE status='active'`, brandID, actor.OrganizationID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO projects (id, organization_id, name, status, primary_brand_id, project_context_version) VALUES (?, ?, 'Local Project', 'active', ?, 1) ON DUPLICATE KEY UPDATE status='active', primary_brand_id=VALUES(primary_brand_id)`, projectID, actor.OrganizationID, brandID); err != nil {
		return err
	}
	role := "owner"
	if actor.Principal.Kind == contract.PrincipalService {
		role = "worker"
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO project_memberships (organization_id, project_id, principal_kind, principal_id, role, status) VALUES (?, ?, ?, ?, ?, 'active') ON DUPLICATE KEY UPDATE role=VALUES(role), status='active'`, actor.OrganizationID, projectID, actor.Principal.Kind, actor.Principal.ID, role); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO project_context_versions (organization_id, project_id, version, brand_id, product_ids) VALUES (?, ?, 1, ?, JSON_ARRAY()) ON DUPLICATE KEY UPDATE brand_id=VALUES(brand_id)`, actor.OrganizationID, projectID, brandID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s MySQLStore) AuthorizeProject(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) error {
	if s.DB == nil || actor.Validate() != nil || projectID == "" {
		return identity.ErrProjectAccessDenied
	}
	var exists int
	var err error
	switch actor.Principal.Kind {
	case contract.PrincipalUser:
		err = s.DB.QueryRowContext(ctx, `SELECT 1
			FROM projects p
			JOIN project_memberships pm ON pm.organization_id = p.organization_id AND pm.project_id = p.id
			JOIN organization_memberships om ON om.organization_id = p.organization_id AND om.user_id = pm.principal_id
			JOIN organizations o ON o.id = p.organization_id
			JOIN users u ON u.id = pm.principal_id
			WHERE p.organization_id = ? AND p.id = ?
			  AND pm.principal_kind = 'user' AND pm.principal_id = ? AND pm.status = 'active'
			  AND om.status = 'active' AND o.status = 'active' AND u.status = 'active'`,
			actor.OrganizationID, projectID, actor.Principal.ID).Scan(&exists)
	case contract.PrincipalService:
		err = s.DB.QueryRowContext(ctx, `SELECT 1
			FROM projects p
			JOIN project_memberships pm ON pm.organization_id = p.organization_id AND pm.project_id = p.id
			JOIN service_identities si ON si.organization_id = p.organization_id AND si.id = pm.principal_id
			JOIN organizations o ON o.id = p.organization_id
			WHERE p.organization_id = ? AND p.id = ?
			  AND pm.principal_kind = 'service' AND pm.principal_id = ? AND pm.status = 'active'
			  AND si.status = 'active' AND o.status = 'active'`,
			actor.OrganizationID, projectID, actor.Principal.ID).Scan(&exists)
	default:
		return identity.ErrProjectAccessDenied
	}
	if err == sql.ErrNoRows {
		return identity.ErrProjectAccessDenied
	}
	if err != nil {
		return fmt.Errorf("authorize project: %w", err)
	}
	return nil
}

func (s MySQLStore) CreateBrand(ctx context.Context, brand Brand) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO brands (id, organization_id, name, status) VALUES (?, ?, ?, ?)`,
		brand.ID, brand.OrganizationID, brand.Name, brand.Status)
	return err
}

func (s MySQLStore) CreateProject(ctx context.Context, project Project, principal contract.Principal, productIDs []contract.ProductID) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if project.PrimaryBrandID != nil {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM brands WHERE organization_id = ? AND id = ? AND status = 'active'`,
			project.OrganizationID, *project.PrimaryBrandID).Scan(&exists); err != nil {
			if err == sql.ErrNoRows {
				return ErrBrandNotFound
			}
			return err
		}
	}
	for _, productID := range productIDs {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT 1 FROM products WHERE organization_id=? AND id=? AND status='active'`, project.OrganizationID, productID).Scan(&exists); err != nil {
			if err == sql.ErrNoRows {
				return ErrProductNotFound
			}
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO projects
		(id, organization_id, name, status, primary_brand_id, brand_guideline_version_id, project_context_version)
		VALUES (?, ?, ?, ?, ?, NULLIF(?, ''), ?)`, project.ID, project.OrganizationID, project.Name, project.Status,
		nullableBrandID(project.PrimaryBrandID), project.BrandGuidelineVersionID, project.ProjectContextVersion); err != nil {
		return err
	}
	role := "owner"
	if principal.Kind == contract.PrincipalService {
		role = "worker"
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO project_memberships
		(organization_id, project_id, principal_kind, principal_id, role, status) VALUES (?, ?, ?, ?, ?, 'active')`,
		project.OrganizationID, project.ID, principal.Kind, principal.ID, role); err != nil {
		return err
	}
	for _, productID := range productIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_products (organization_id, project_id, product_id) VALUES (?, ?, ?)`,
			project.OrganizationID, project.ID, productID); err != nil {
			return err
		}
	}
	productJSON, err := json.Marshal(productIDs)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO project_context_versions
		(organization_id, project_id, version, brand_id, brand_guideline_version_id, product_ids)
		VALUES (?, ?, ?, ?, NULLIF(?, ''), ?)`, project.OrganizationID, project.ID, project.ProjectContextVersion,
		nullableBrandID(project.PrimaryBrandID), project.BrandGuidelineVersionID, productJSON); err != nil {
		return err
	}
	return tx.Commit()
}

func (s MySQLStore) GetProject(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) (Project, error) {
	if s.DB == nil {
		return Project{}, fmt.Errorf("project database is required")
	}
	var project Project
	var brandID, guidelineID sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT p.id, p.organization_id, p.name, p.status, p.primary_brand_id,
		p.brand_guideline_version_id, p.project_context_version, p.created_at, p.updated_at, COALESCE(b.status, '')
		FROM projects p LEFT JOIN brands b ON b.organization_id=p.organization_id AND b.id=p.primary_brand_id
		WHERE p.organization_id = ? AND p.id = ?`, organizationID, projectID).Scan(
		&project.ID, &project.OrganizationID, &project.Name, &project.Status, &brandID,
		&guidelineID, &project.ProjectContextVersion, &project.CreatedAt, &project.UpdatedAt, &project.PrimaryBrandStatus)
	if err == sql.ErrNoRows {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, err
	}
	if brandID.Valid {
		value := contract.BrandID(brandID.String)
		project.PrimaryBrandID = &value
	}
	project.BrandGuidelineVersionID = guidelineID.String
	return project, nil
}

func (s MySQLStore) GetContext(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) (contract.ProjectContext, error) {
	if s.DB == nil {
		return contract.ProjectContext{}, fmt.Errorf("project database is required")
	}
	var result contract.ProjectContext
	var brandID, guidelineID sql.NullString
	var productsJSON []byte
	err := s.DB.QueryRowContext(ctx, `SELECT organization_id, project_id, brand_id, brand_guideline_version_id,
		product_ids, version FROM project_context_versions
		WHERE organization_id = ? AND project_id = ? ORDER BY version DESC LIMIT 1`, organizationID, projectID).Scan(
		&result.OrganizationID, &result.ProjectID, &brandID, &guidelineID, &productsJSON, &result.ProjectContextVersion)
	if err == sql.ErrNoRows {
		return contract.ProjectContext{}, ErrNotFound
	}
	if err != nil {
		return contract.ProjectContext{}, err
	}
	if brandID.Valid {
		value := contract.BrandID(brandID.String)
		result.BrandID = &value
	}
	result.BrandGuidelineVersionID = guidelineID.String
	if err := json.Unmarshal(productsJSON, &result.ProductIDs); err != nil {
		return contract.ProjectContext{}, fmt.Errorf("decode project products: %w", err)
	}
	if result.ProductIDs == nil {
		result.ProductIDs = []contract.ProductID{}
	}
	if err := result.Validate(); err != nil {
		return contract.ProjectContext{}, err
	}
	return result, nil
}

func (s MySQLStore) ListProjects(ctx context.Context, actor contract.ActorContext) ([]Project, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("project database is required")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT p.id, p.organization_id, p.name, p.status, p.primary_brand_id,
		p.brand_guideline_version_id, p.project_context_version, p.created_at, p.updated_at
		FROM projects p JOIN project_memberships pm
		  ON pm.organization_id = p.organization_id AND pm.project_id = p.id
		WHERE p.organization_id = ? AND pm.principal_kind = ? AND pm.principal_id = ? AND pm.status = 'active'
		ORDER BY p.updated_at DESC`, actor.OrganizationID, actor.Principal.Kind, actor.Principal.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := make([]Project, 0)
	for rows.Next() {
		var project Project
		var brandID, guidelineID sql.NullString
		if err := rows.Scan(&project.ID, &project.OrganizationID, &project.Name, &project.Status, &brandID,
			&guidelineID, &project.ProjectContextVersion, &project.CreatedAt, &project.UpdatedAt); err != nil {
			return nil, err
		}
		if brandID.Valid {
			value := contract.BrandID(brandID.String)
			project.PrimaryBrandID = &value
		}
		project.BrandGuidelineVersionID = guidelineID.String
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (s MySQLStore) CreateBusinessTask(ctx context.Context, task BusinessTask) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	now := defaultTimestamp(task.CreatedAt)
	task.CreatedAt = now
	task.UpdatedAt = defaultTimestamp(task.UpdatedAt)
	if task.Version < 1 {
		task.Version = 1
	}
	sourceTasks, err := jsonArray(task.SourceTaskIDs)
	if err != nil {
		return fmt.Errorf("encode source task ids: %w", err)
	}
	sourceArtifacts, err := jsonArray(task.SourceArtifactIDs)
	if err != nil {
		return fmt.Errorf("encode source artifact ids: %w", err)
	}
	outputArtifacts, err := jsonArray(task.OutputArtifactIDs)
	if err != nil {
		return fmt.Errorf("encode output artifact ids: %w", err)
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO platform_project_tasks (
		organization_id, project_id, id, type, name, objective, status,
		source_task_ids, source_artifact_ids, output_artifact_ids, version, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.OrganizationID, task.ProjectID, task.ID, task.Type, task.Name, task.Objective, task.Status,
		sourceTasks, sourceArtifacts, outputArtifacts, task.Version, task.CreatedAt, task.UpdatedAt)
	return err
}

func (s MySQLStore) ListBusinessTasks(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]BusinessTask, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("project database is required")
	}
	rows, err := s.DB.QueryContext(ctx, businessTaskSelect+` WHERE organization_id=? AND project_id=? ORDER BY updated_at DESC, id`, organizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]BusinessTask, 0)
	for rows.Next() {
		task, err := scanBusinessTask(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, task)
	}
	return result, rows.Err()
}

func (s MySQLStore) GetBusinessTask(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, taskID string) (BusinessTask, error) {
	if s.DB == nil {
		return BusinessTask{}, fmt.Errorf("project database is required")
	}
	return scanBusinessTask(s.DB.QueryRowContext(ctx, businessTaskSelect+` WHERE organization_id=? AND project_id=? AND id=?`, organizationID, projectID, taskID))
}

func (s MySQLStore) UpdateBusinessTask(ctx context.Context, task BusinessTask) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	sourceTasks, err := jsonArray(task.SourceTaskIDs)
	if err != nil {
		return fmt.Errorf("encode source task ids: %w", err)
	}
	sourceArtifacts, err := jsonArray(task.SourceArtifactIDs)
	if err != nil {
		return fmt.Errorf("encode source artifact ids: %w", err)
	}
	outputArtifacts, err := jsonArray(task.OutputArtifactIDs)
	if err != nil {
		return fmt.Errorf("encode output artifact ids: %w", err)
	}
	nextVersion := task.Version + 1
	if nextVersion < 1 {
		nextVersion = 1
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_project_tasks SET
		type=?, name=?, objective=?, status=?, source_task_ids=?, source_artifact_ids=?,
		output_artifact_ids=?, version=?, updated_at=?
		WHERE organization_id=? AND project_id=? AND id=?`,
		task.Type, task.Name, task.Objective, task.Status, sourceTasks, sourceArtifacts,
		outputArtifacts, nextVersion, defaultTimestamp(task.UpdatedAt),
		task.OrganizationID, task.ProjectID, task.ID)
	return requireAffected(result, err)
}

func (s MySQLStore) CreateOperationalRecord(ctx context.Context, record OperationalRecord) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	record.CreatedAt = defaultTimestamp(record.CreatedAt)
	record.UpdatedAt = defaultTimestamp(record.UpdatedAt)
	record.OccurredAt = defaultTimestamp(record.OccurredAt)
	fields, err := jsonObject(record.Fields)
	if err != nil {
		return fmt.Errorf("encode operational fields: %w", err)
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO platform_project_operations (
		organization_id, project_id, id, kind, title, status, occurred_at, fields, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.OrganizationID, record.ProjectID, record.ID, record.Kind, record.Title, record.Status,
		record.OccurredAt, fields, record.CreatedAt, record.UpdatedAt)
	return err
}

func (s MySQLStore) ListOperationalRecords(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]OperationalRecord, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("project database is required")
	}
	rows, err := s.DB.QueryContext(ctx, operationalRecordSelect+` WHERE organization_id=? AND project_id=? ORDER BY occurred_at DESC, id`, organizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]OperationalRecord, 0)
	for rows.Next() {
		record, err := scanOperationalRecord(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, rows.Err()
}

func (s MySQLStore) GetOperationalRecord(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, recordID string) (OperationalRecord, error) {
	if s.DB == nil {
		return OperationalRecord{}, fmt.Errorf("project database is required")
	}
	return scanOperationalRecord(s.DB.QueryRowContext(ctx, operationalRecordSelect+` WHERE organization_id=? AND project_id=? AND id=?`, organizationID, projectID, recordID))
}

func (s MySQLStore) UpdateOperationalRecord(ctx context.Context, record OperationalRecord) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	fields, err := jsonObject(record.Fields)
	if err != nil {
		return fmt.Errorf("encode operational fields: %w", err)
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_project_operations SET
		kind=?, title=?, status=?, occurred_at=?, fields=?, updated_at=?
		WHERE organization_id=? AND project_id=? AND id=?`,
		record.Kind, record.Title, record.Status, defaultTimestamp(record.OccurredAt), fields, defaultTimestamp(record.UpdatedAt),
		record.OrganizationID, record.ProjectID, record.ID)
	return requireAffected(result, err)
}

func (s MySQLStore) CreateChangeSet(ctx context.Context, changeSet ChangeSet) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	changeSet.CreatedAt = defaultTimestamp(changeSet.CreatedAt)
	changeSet.UpdatedAt = defaultTimestamp(changeSet.UpdatedAt)
	if changeSet.Version < 1 {
		changeSet.Version = 1
	}
	artifactRefs, err := jsonArray(changeSet.ArtifactRefs)
	if err != nil {
		return fmt.Errorf("encode change set artifact refs: %w", err)
	}
	preflight, err := optionalJSON(changeSet.Preflight)
	if err != nil {
		return fmt.Errorf("encode change set preflight: %w", err)
	}
	execution, err := optionalJSON(changeSet.Execution)
	if err != nil {
		return fmt.Errorf("encode change set execution: %w", err)
	}
	rollback, err := optionalJSON(changeSet.Rollback)
	if err != nil {
		return fmt.Errorf("encode change set rollback: %w", err)
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO platform_change_sets (
		organization_id, project_id, id, name, status, artifact_refs, budget_limit,
		preflight, execution, rollback, version, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		changeSet.OrganizationID, changeSet.ProjectID, changeSet.ID, changeSet.Name, changeSet.Status,
		artifactRefs, changeSet.BudgetLimit, preflight, execution, rollback, changeSet.Version,
		changeSet.CreatedAt, changeSet.UpdatedAt)
	return err
}

func (s MySQLStore) ListChangeSets(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]ChangeSet, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("project database is required")
	}
	rows, err := s.DB.QueryContext(ctx, changeSetSelect+` WHERE organization_id=? AND project_id=? ORDER BY updated_at DESC, id`, organizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]ChangeSet, 0)
	for rows.Next() {
		changeSet, err := scanChangeSet(rows)
		if err != nil {
			return nil, err
		}
		changeSet.AuditEvents, err = s.listAuditEventsForEntity(ctx, organizationID, projectID, AuditEntityChangeSet, changeSet.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, changeSet)
	}
	return result, rows.Err()
}

func (s MySQLStore) GetChangeSet(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, changeSetID string) (ChangeSet, error) {
	if s.DB == nil {
		return ChangeSet{}, fmt.Errorf("project database is required")
	}
	changeSet, err := scanChangeSet(s.DB.QueryRowContext(ctx, changeSetSelect+` WHERE organization_id=? AND project_id=? AND id=?`, organizationID, projectID, changeSetID))
	if err != nil {
		return ChangeSet{}, err
	}
	changeSet.AuditEvents, err = s.listAuditEventsForEntity(ctx, organizationID, projectID, AuditEntityChangeSet, changeSet.ID)
	if err != nil {
		return ChangeSet{}, err
	}
	return changeSet, nil
}

func (s MySQLStore) UpdateChangeSet(ctx context.Context, changeSet ChangeSet) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	artifactRefs, err := jsonArray(changeSet.ArtifactRefs)
	if err != nil {
		return fmt.Errorf("encode change set artifact refs: %w", err)
	}
	preflight, err := optionalJSON(changeSet.Preflight)
	if err != nil {
		return fmt.Errorf("encode change set preflight: %w", err)
	}
	execution, err := optionalJSON(changeSet.Execution)
	if err != nil {
		return fmt.Errorf("encode change set execution: %w", err)
	}
	rollback, err := optionalJSON(changeSet.Rollback)
	if err != nil {
		return fmt.Errorf("encode change set rollback: %w", err)
	}
	nextVersion := changeSet.Version + 1
	if nextVersion < 1 {
		nextVersion = 1
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_change_sets SET
		name=?, status=?, artifact_refs=?, budget_limit=?, preflight=?, execution=?, rollback=?,
		version=?, updated_at=?
		WHERE organization_id=? AND project_id=? AND id=?`,
		changeSet.Name, changeSet.Status, artifactRefs, changeSet.BudgetLimit, preflight, execution, rollback,
		nextVersion, defaultTimestamp(changeSet.UpdatedAt), changeSet.OrganizationID, changeSet.ProjectID, changeSet.ID)
	return requireAffected(result, err)
}

func (s MySQLStore) AppendChangeSetEvent(ctx context.Context, event ChangeSetEvent) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	event.CreatedAt = defaultTimestamp(event.CreatedAt)
	payload, err := jsonObject(event.Payload)
	if err != nil {
		return fmt.Errorf("encode change set event payload: %w", err)
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO platform_change_set_events (
		organization_id, project_id, change_set_id, id, event_type, actor, payload, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		event.OrganizationID, event.ProjectID, event.ChangeSetID, event.ID, event.EventType, event.Actor, payload, event.CreatedAt)
	return err
}

func (s MySQLStore) AppendAuditEvent(ctx context.Context, event AuditEvent) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	event.CreatedAt = defaultTimestamp(event.CreatedAt)
	metadata, err := jsonObject(event.Metadata)
	if err != nil {
		return fmt.Errorf("encode audit metadata: %w", err)
	}
	_, err = s.DB.ExecContext(ctx, `INSERT INTO platform_audit_events (
		organization_id, project_id, id, actor, action, entity_type, entity_id, metadata, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.OrganizationID, event.ProjectID, event.ID, event.Actor, event.Action, event.EntityType,
		event.EntityID, metadata, event.CreatedAt)
	return err
}

func (s MySQLStore) ListAuditEvents(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]AuditEvent, error) {
	return s.listAuditEvents(ctx, organizationID, projectID, "", "")
}

func (s MySQLStore) listAuditEventsForEntity(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, entityType AuditEntityType, entityID string) ([]AuditEvent, error) {
	return s.listAuditEvents(ctx, organizationID, projectID, entityType, entityID)
}

func (s MySQLStore) listAuditEvents(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, entityType AuditEntityType, entityID string) ([]AuditEvent, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("project database is required")
	}
	query := auditEventSelect + ` WHERE organization_id=? AND project_id=?`
	args := []any{organizationID, projectID}
	if entityType != "" {
		query += ` AND entity_type=? AND entity_id=?`
		args = append(args, entityType, entityID)
	}
	query += ` ORDER BY created_at ASC, id`
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]AuditEvent, 0)
	for rows.Next() {
		event, err := scanAuditEvent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, rows.Err()
}

func nullableBrandID(value *contract.BrandID) any {
	if value == nil {
		return nil
	}
	return *value
}

func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

const businessTaskSelect = `SELECT organization_id, project_id, id, type, name, objective, status,
	source_task_ids, source_artifact_ids, output_artifact_ids, version, created_at, updated_at
	FROM platform_project_tasks`

const operationalRecordSelect = `SELECT organization_id, project_id, id, kind, title, status,
	occurred_at, fields, created_at, updated_at FROM platform_project_operations`

const changeSetSelect = `SELECT organization_id, project_id, id, name, status, artifact_refs, budget_limit,
	preflight, execution, rollback, version, created_at, updated_at FROM platform_change_sets`

const auditEventSelect = `SELECT organization_id, project_id, id, actor, action, entity_type, entity_id,
	metadata, created_at FROM platform_audit_events`

type projectScanner interface{ Scan(...any) error }

func scanBusinessTask(row projectScanner) (BusinessTask, error) {
	var task BusinessTask
	var sourceTasks, sourceArtifacts, outputArtifacts []byte
	err := row.Scan(&task.OrganizationID, &task.ProjectID, &task.ID, &task.Type, &task.Name, &task.Objective,
		&task.Status, &sourceTasks, &sourceArtifacts, &outputArtifacts, &task.Version, &task.CreatedAt, &task.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return BusinessTask{}, ErrNotFound
	}
	if err != nil {
		return BusinessTask{}, err
	}
	if err := decodeJSON(sourceTasks, &task.SourceTaskIDs); err != nil {
		return BusinessTask{}, fmt.Errorf("decode source task ids: %w", err)
	}
	if err := decodeJSON(sourceArtifacts, &task.SourceArtifactIDs); err != nil {
		return BusinessTask{}, fmt.Errorf("decode source artifact ids: %w", err)
	}
	if err := decodeJSON(outputArtifacts, &task.OutputArtifactIDs); err != nil {
		return BusinessTask{}, fmt.Errorf("decode output artifact ids: %w", err)
	}
	return task, nil
}

func scanOperationalRecord(row projectScanner) (OperationalRecord, error) {
	var record OperationalRecord
	var fields []byte
	err := row.Scan(&record.OrganizationID, &record.ProjectID, &record.ID, &record.Kind, &record.Title,
		&record.Status, &record.OccurredAt, &fields, &record.CreatedAt, &record.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return OperationalRecord{}, ErrNotFound
	}
	if err != nil {
		return OperationalRecord{}, err
	}
	if err := decodeJSON(fields, &record.Fields); err != nil {
		return OperationalRecord{}, fmt.Errorf("decode operational fields: %w", err)
	}
	if record.Fields == nil {
		record.Fields = map[string]any{}
	}
	return record, nil
}

func scanChangeSet(row projectScanner) (ChangeSet, error) {
	var changeSet ChangeSet
	var artifactRefs []byte
	var budget sql.NullFloat64
	var preflight, execution, rollback sql.NullString
	err := row.Scan(&changeSet.OrganizationID, &changeSet.ProjectID, &changeSet.ID, &changeSet.Name,
		&changeSet.Status, &artifactRefs, &budget, &preflight, &execution, &rollback,
		&changeSet.Version, &changeSet.CreatedAt, &changeSet.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ChangeSet{}, ErrNotFound
	}
	if err != nil {
		return ChangeSet{}, err
	}
	if err := decodeJSON(artifactRefs, &changeSet.ArtifactRefs); err != nil {
		return ChangeSet{}, fmt.Errorf("decode change set artifact refs: %w", err)
	}
	if budget.Valid {
		changeSet.BudgetLimit = &budget.Float64
	}
	if preflight.Valid {
		changeSet.Preflight = &ChangeSetPreflight{}
		if err := decodeJSON([]byte(preflight.String), changeSet.Preflight); err != nil {
			return ChangeSet{}, fmt.Errorf("decode change set preflight: %w", err)
		}
	}
	if execution.Valid {
		changeSet.Execution = &ChangeSetExecution{}
		if err := decodeJSON([]byte(execution.String), changeSet.Execution); err != nil {
			return ChangeSet{}, fmt.Errorf("decode change set execution: %w", err)
		}
	}
	if rollback.Valid {
		changeSet.Rollback = &ChangeSetRollback{}
		if err := decodeJSON([]byte(rollback.String), changeSet.Rollback); err != nil {
			return ChangeSet{}, fmt.Errorf("decode change set rollback: %w", err)
		}
	}
	if changeSet.ArtifactRefs == nil {
		changeSet.ArtifactRefs = []contract.ProjectAssetRef{}
	}
	return changeSet, nil
}

func scanAuditEvent(row projectScanner) (AuditEvent, error) {
	var event AuditEvent
	var metadata []byte
	err := row.Scan(&event.OrganizationID, &event.ProjectID, &event.ID, &event.Actor, &event.Action,
		&event.EntityType, &event.EntityID, &metadata, &event.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AuditEvent{}, ErrNotFound
	}
	if err != nil {
		return AuditEvent{}, err
	}
	if err := decodeJSON(metadata, &event.Metadata); err != nil {
		return AuditEvent{}, fmt.Errorf("decode audit metadata: %w", err)
	}
	if event.Metadata == nil {
		event.Metadata = map[string]any{}
	}
	return event, nil
}

func jsonArray[T any](value []T) ([]byte, error) {
	if value == nil {
		value = []T{}
	}
	return json.Marshal(value)
}

func jsonObject(value map[string]any) ([]byte, error) {
	if value == nil {
		value = map[string]any{}
	}
	return json.Marshal(value)
}

func optionalJSON(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func decodeJSON(data []byte, target any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, target)
}

func defaultTimestamp(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value
}

func requireAffected(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return ErrNotFound
	}
	return err
}
