package project

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/identity"
	"github.com/shikanon/cookies/internal/platform/ids"
)

type MySQLStore struct{ DB *sql.DB }

func (s MySQLStore) ListProjectMembers(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) ([]ProjectMembership, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("project database is required")
	}
	if err := s.AuthorizeProject(ctx, actor, projectID); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT pm.organization_id, pm.project_id, pm.principal_kind,
		pm.principal_id, COALESCE(u.display_name, si.name, pm.principal_id), pm.role, pm.status,
		pm.created_at, pm.updated_at
		FROM project_memberships pm
		LEFT JOIN users u ON pm.principal_kind = 'user' AND u.id = pm.principal_id
		LEFT JOIN service_identities si ON pm.principal_kind = 'service'
		  AND si.organization_id = pm.organization_id AND si.id = pm.principal_id
		WHERE pm.organization_id = ? AND pm.project_id = ? ORDER BY pm.created_at`,
		actor.OrganizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]ProjectMembership, 0)
	for rows.Next() {
		var value ProjectMembership
		if err := rows.Scan(&value.OrganizationID, &value.ProjectID, &value.PrincipalKind,
			&value.PrincipalID, &value.DisplayName, &value.Role, &value.Status,
			&value.CreatedAt, &value.UpdatedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s MySQLStore) AddProjectMember(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, principal contract.Principal, role string) (ProjectMembership, error) {
	if s.DB == nil {
		return ProjectMembership{}, fmt.Errorf("project database is required")
	}
	if projectID == "" || strings.TrimSpace(principal.ID) == "" ||
		!ValidProjectRoleForPrincipal(principal.Kind, role) {
		return ProjectMembership{}, fmt.Errorf("valid principal and compatible project role are required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ProjectMembership{}, err
	}
	defer tx.Rollback()
	if err := requireProjectOwner(ctx, tx, actor, projectID); err != nil {
		return ProjectMembership{}, err
	}
	var displayName string
	switch principal.Kind {
	case contract.PrincipalUser:
		err = tx.QueryRowContext(ctx, `SELECT u.display_name FROM users u
			JOIN organization_memberships om ON om.user_id = u.id
			WHERE om.organization_id = ? AND om.user_id = ? AND om.status = 'active' AND u.status = 'active'`,
			actor.OrganizationID, principal.ID).Scan(&displayName)
	case contract.PrincipalService:
		err = tx.QueryRowContext(ctx, `SELECT name FROM service_identities
			WHERE organization_id = ? AND id = ? AND status = 'active'`,
			actor.OrganizationID, principal.ID).Scan(&displayName)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectMembership{}, identity.ErrActorInactive
	}
	if err != nil {
		return ProjectMembership{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO project_memberships
		(organization_id, project_id, principal_kind, principal_id, role, status)
		VALUES (?, ?, ?, ?, ?, 'active')`,
		actor.OrganizationID, projectID, principal.Kind, principal.ID, role); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return ProjectMembership{}, ErrMembershipConflict
		}
		return ProjectMembership{}, err
	}
	value, err := scanProjectMembership(ctx, tx, actor.OrganizationID, projectID, principal)
	if err != nil {
		return ProjectMembership{}, err
	}
	value.DisplayName = displayName
	if err := writeProjectMembershipAudit(ctx, tx, actor, projectID, value, "project_member.added"); err != nil {
		return ProjectMembership{}, err
	}
	if err := tx.Commit(); err != nil {
		return ProjectMembership{}, err
	}
	return value, nil
}

func (s MySQLStore) UpdateProjectMember(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, principal contract.Principal, request UpdateProjectMembershipRequest) (ProjectMembership, error) {
	if s.DB == nil {
		return ProjectMembership{}, fmt.Errorf("project database is required")
	}
	if projectID == "" || strings.TrimSpace(principal.ID) == "" ||
		!ValidProjectRoleForPrincipal(principal.Kind, request.Role) ||
		!identity.ValidMembershipStatus(request.Status) ||
		request.ExpectedUpdatedAt.IsZero() {
		return ProjectMembership{}, fmt.Errorf("valid principal, compatible role, status, and expected_updated_at are required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return ProjectMembership{}, err
	}
	defer tx.Rollback()
	if err := requireProjectOwner(ctx, tx, actor, projectID); err != nil {
		return ProjectMembership{}, err
	}
	before, err := scanProjectMembership(ctx, tx, actor.OrganizationID, projectID, principal)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectMembership{}, ErrMembershipNotFound
	}
	if err != nil {
		return ProjectMembership{}, err
	}
	if !before.UpdatedAt.Equal(request.ExpectedUpdatedAt) {
		return ProjectMembership{}, ErrMembershipConflict
	}
	removesOwner := before.Role == "owner" && before.Status == "active" &&
		(request.Role != "owner" || request.Status != "active")
	if removesOwner {
		rows, err := tx.QueryContext(ctx, `SELECT principal_id FROM project_memberships
			WHERE organization_id = ? AND project_id = ? AND principal_kind = 'user'
			  AND role = 'owner' AND status = 'active' FOR UPDATE`, actor.OrganizationID, projectID)
		if err != nil {
			return ProjectMembership{}, err
		}
		count := 0
		for rows.Next() {
			count++
		}
		if err := rows.Close(); err != nil {
			return ProjectMembership{}, err
		}
		if count <= 1 {
			return ProjectMembership{}, ErrLastOwner
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE project_memberships SET role = ?, status = ?
		WHERE organization_id = ? AND project_id = ? AND principal_kind = ?
		  AND principal_id = ? AND updated_at = ?`,
		request.Role, request.Status, actor.OrganizationID, projectID, principal.Kind,
		principal.ID, request.ExpectedUpdatedAt)
	if err != nil {
		return ProjectMembership{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		return ProjectMembership{}, ErrMembershipConflict
	}
	value, err := scanProjectMembership(ctx, tx, actor.OrganizationID, projectID, principal)
	if err != nil {
		return ProjectMembership{}, err
	}
	if err := writeProjectMembershipAudit(ctx, tx, actor, projectID, value, "project_member.updated"); err != nil {
		return ProjectMembership{}, err
	}
	if err := tx.Commit(); err != nil {
		return ProjectMembership{}, err
	}
	return value, nil
}

func requireProjectOwner(ctx context.Context, tx *sql.Tx, actor contract.ActorContext, projectID contract.ProjectID) error {
	if actor.Principal.Kind != contract.PrincipalUser {
		return ErrMembershipForbidden
	}
	var role string
	err := tx.QueryRowContext(ctx, `SELECT role FROM project_memberships
		WHERE organization_id = ? AND project_id = ? AND principal_kind = 'user'
		  AND principal_id = ? AND status = 'active' FOR UPDATE`,
		actor.OrganizationID, projectID, actor.Principal.ID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) || role != "owner" {
		return ErrMembershipForbidden
	}
	return err
}

func scanProjectMembership(ctx context.Context, tx *sql.Tx, organizationID contract.OrganizationID, projectID contract.ProjectID, principal contract.Principal) (ProjectMembership, error) {
	var value ProjectMembership
	err := tx.QueryRowContext(ctx, `SELECT pm.organization_id, pm.project_id, pm.principal_kind,
		pm.principal_id, COALESCE(u.display_name, si.name, pm.principal_id), pm.role, pm.status,
		pm.created_at, pm.updated_at
		FROM project_memberships pm
		LEFT JOIN users u ON pm.principal_kind = 'user' AND u.id = pm.principal_id
		LEFT JOIN service_identities si ON pm.principal_kind = 'service'
		  AND si.organization_id = pm.organization_id AND si.id = pm.principal_id
		WHERE pm.organization_id = ? AND pm.project_id = ?
		  AND pm.principal_kind = ? AND pm.principal_id = ?`,
		organizationID, projectID, principal.Kind, principal.ID).Scan(
		&value.OrganizationID, &value.ProjectID, &value.PrincipalKind, &value.PrincipalID,
		&value.DisplayName, &value.Role, &value.Status, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}

func writeProjectMembershipAudit(ctx context.Context, tx *sql.Tx, actor contract.ActorContext, projectID contract.ProjectID, value ProjectMembership, action string) error {
	id, err := ids.New("identity_audit")
	if err != nil {
		return err
	}
	after, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO identity_audit_events
		(id, organization_id, actor_user_id, target_kind, target_id, action, after_state)
		VALUES (?, ?, ?, 'project_membership', ?, ?, ?)`,
		id, actor.OrganizationID, actor.Principal.ID,
		fmt.Sprintf("%s:%s:%s", projectID, value.PrincipalKind, value.PrincipalID), action, after)
	return err
}

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
	if _, err := tx.ExecContext(ctx, `INSERT INTO projects (id, organization_id, name, status, industry, primary_brand_id, project_context_version) VALUES (?, ?, 'Local Project', 'active', 'ecommerce', ?, 1) ON DUPLICATE KEY UPDATE status='active', primary_brand_id=VALUES(primary_brand_id)`, projectID, actor.OrganizationID, brandID); err != nil {
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
	if err := upsertProjectRuntime(ctx, tx, actor.OrganizationID, projectID, ProjectRuntime{Code: string(projectID), Brand: "Local Brand", Product: "尚未关联产品", Goal: "尚未设定项目目标", Stage: "项目执行", Progress: 10, Status: "active", Owner: "系统", Budget: 0, Currency: "CNY", Timezone: "Asia/Shanghai", KnowledgeCount: 0, UpdatedAt: time.Now().UTC()}); err != nil {
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

func (s MySQLStore) AuthorizeProjectAction(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, action string) error {
	if err := s.AuthorizeProject(ctx, actor, projectID); err != nil {
		return err
	}
	var role string
	err := s.DB.QueryRowContext(ctx, `SELECT role FROM project_memberships
		WHERE organization_id = ? AND project_id = ? AND principal_kind = ?
		  AND principal_id = ? AND status = 'active'`,
		actor.OrganizationID, projectID, actor.Principal.Kind, actor.Principal.ID).Scan(&role)
	if err != nil {
		return identity.ErrProjectAccessDenied
	}
	if !projectRoleAllowsAction(actor.Principal.Kind, role, action) {
		return identity.ErrProjectAccessDenied
	}
	return nil
}

func projectRoleAllowsAction(kind contract.PrincipalKind, role, action string) bool {
	switch kind {
	case contract.PrincipalUser:
		return role == "owner" ||
			(role == "editor" && (action == "read" || action == "write")) ||
			(role == "viewer" && action == "read")
	case contract.PrincipalService:
		return role == "worker" && (action == "read" || action == "write")
	default:
		return false
	}
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
		(id, organization_id, name, status, industry, primary_brand_id, brand_guideline_version_id, project_context_version)
		VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?)`, project.ID, project.OrganizationID, project.Name, project.Status, project.Industry,
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
	brandName := "未指定品牌"
	if project.PrimaryBrandID != nil {
		if err := tx.QueryRowContext(ctx, `SELECT name FROM brands WHERE organization_id=? AND id=?`, project.OrganizationID, *project.PrimaryBrandID).Scan(&brandName); err != nil {
			return err
		}
	}
	productName := "尚未关联产品"
	if len(productIDs) > 0 {
		if err := tx.QueryRowContext(ctx, `SELECT GROUP_CONCAT(name ORDER BY name SEPARATOR '、') FROM products WHERE organization_id=? AND id IN (`+placeholders(len(productIDs))+`)`, append([]any{project.OrganizationID}, productIDArgs(productIDs)...)...).Scan(&productName); err != nil {
			return err
		}
		if productName == "" {
			productName = "尚未关联产品"
		}
	}
	status := "active"
	if project.Status == StatusDraft {
		status = "blocked"
	}
	if project.Status == StatusArchived {
		status = "completed"
	}
	if err := upsertProjectRuntime(ctx, tx, project.OrganizationID, project.ID, ProjectRuntime{Code: string(project.ID), Brand: brandName, Product: productName, Goal: "尚未设定项目目标", Stage: string(project.Status), Progress: 0, Status: status, Owner: string(principal.Kind) + ":" + principal.ID, Budget: 0, Currency: "CNY", Timezone: "Asia/Shanghai", KnowledgeCount: 0, UpdatedAt: time.Now().UTC()}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s MySQLStore) UpdateProject(ctx context.Context, project Project, runtime ProjectRuntime, expectedContextVersion int64) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	if expectedContextVersion < 1 {
		return fmt.Errorf("expected project context version must be positive")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE projects SET name=?, industry=?, project_context_version=project_context_version+1
		WHERE organization_id=? AND id=? AND project_context_version=?`,
		project.Name, project.Industry, project.OrganizationID, project.ID, expectedContextVersion)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		var exists int
		err = tx.QueryRowContext(ctx, `SELECT 1 FROM projects WHERE organization_id=? AND id=?`, project.OrganizationID, project.ID).Scan(&exists)
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		return ErrVersionConflict
	}
	if err := upsertProjectRuntime(ctx, tx, project.OrganizationID, project.ID, runtime); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO project_context_versions
		(organization_id, project_id, version, brand_id, brand_guideline_version_id, product_ids)
		SELECT organization_id, project_id, ?, brand_id, brand_guideline_version_id, product_ids
		FROM project_context_versions WHERE organization_id=? AND project_id=? AND version=?`,
		expectedContextVersion+1, project.OrganizationID, project.ID, expectedContextVersion); err != nil {
		return err
	}
	return tx.Commit()
}

func (s MySQLStore) CreateProjectArtifact(ctx context.Context, artifact ProjectArtifact) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO platform_project_artifacts
		(organization_id, project_id, id, kind, status, content, source_job_id, version, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?)`,
		artifact.OrganizationID, artifact.ProjectID, artifact.ID, artifact.Kind, artifact.Status, artifact.Content,
		artifact.SourceJobID, artifact.Version, artifact.CreatedAt, artifact.UpdatedAt)
	return err
}

func (s MySQLStore) ListProjectArtifacts(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]ProjectArtifact, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("project database is required")
	}
	rows, err := s.DB.QueryContext(ctx, projectArtifactSelect+` WHERE organization_id=? AND project_id=? ORDER BY updated_at DESC, id`, organizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	artifacts := make([]ProjectArtifact, 0)
	for rows.Next() {
		artifact, err := scanProjectArtifact(rows)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

func (s MySQLStore) GetProjectArtifact(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, artifactID string) (ProjectArtifact, error) {
	if s.DB == nil {
		return ProjectArtifact{}, fmt.Errorf("project database is required")
	}
	return scanProjectArtifact(s.DB.QueryRowContext(ctx, projectArtifactSelect+` WHERE organization_id=? AND project_id=? AND id=?`, organizationID, projectID, artifactID))
}

func (s MySQLStore) UpdateProjectArtifact(ctx context.Context, artifact ProjectArtifact, expectedVersion int64) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_project_artifacts SET content=?, status=?, source_job_id=NULLIF(?, ''),
		version=version+1, updated_at=? WHERE organization_id=? AND project_id=? AND id=? AND version=?`,
		artifact.Content, artifact.Status, artifact.SourceJobID, artifact.UpdatedAt,
		artifact.OrganizationID, artifact.ProjectID, artifact.ID, expectedVersion)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 1 {
		return nil
	}
	var exists int
	err = s.DB.QueryRowContext(ctx, `SELECT 1 FROM platform_project_artifacts WHERE organization_id=? AND project_id=? AND id=?`, artifact.OrganizationID, artifact.ProjectID, artifact.ID).Scan(&exists)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return ErrVersionConflict
}

func (s MySQLStore) GetProjectRuntime(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) (ProjectRuntime, error) {
	if s.DB == nil {
		return ProjectRuntime{}, fmt.Errorf("project database is required")
	}
	var runtime ProjectRuntime
	err := s.DB.QueryRowContext(ctx, `SELECT code, brand, product, goal, stage, progress, status, owner, budget, currency, timezone, knowledge_count, updated_at FROM platform_project_runtimes WHERE organization_id=? AND project_id=?`, organizationID, projectID).Scan(&runtime.Code, &runtime.Brand, &runtime.Product, &runtime.Goal, &runtime.Stage, &runtime.Progress, &runtime.Status, &runtime.Owner, &runtime.Budget, &runtime.Currency, &runtime.Timezone, &runtime.KnowledgeCount, &runtime.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectRuntime{}, ErrNotFound
	}
	return runtime, err
}

func (s MySQLStore) UpsertProjectRuntime(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, runtime ProjectRuntime) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	return upsertProjectRuntime(ctx, s.DB, organizationID, projectID, runtime)
}

func upsertProjectRuntime(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, organizationID contract.OrganizationID, projectID contract.ProjectID, runtime ProjectRuntime) error {
	runtime.UpdatedAt = defaultTimestamp(runtime.UpdatedAt)
	_, err := exec.ExecContext(ctx, `INSERT INTO platform_project_runtimes (organization_id, project_id, code, brand, product, goal, stage, progress, status, owner, budget, currency, timezone, knowledge_count, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE code=VALUES(code), brand=VALUES(brand), product=VALUES(product), goal=VALUES(goal), stage=VALUES(stage), progress=VALUES(progress), status=VALUES(status), owner=VALUES(owner), budget=VALUES(budget), currency=VALUES(currency), timezone=VALUES(timezone), knowledge_count=VALUES(knowledge_count), updated_at=VALUES(updated_at)`, organizationID, projectID, runtime.Code, runtime.Brand, runtime.Product, runtime.Goal, runtime.Stage, runtime.Progress, runtime.Status, runtime.Owner, runtime.Budget, runtime.Currency, runtime.Timezone, runtime.KnowledgeCount, runtime.UpdatedAt)
	return err
}

func (s MySQLStore) GetWorkbench(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) (Workbench, error) {
	if s.DB == nil {
		return Workbench{}, fmt.Errorf("project database is required")
	}
	var value Workbench
	var productLines []byte
	err := s.DB.QueryRowContext(ctx, `SELECT organization_code, organization_name, organization_owner, client_id, client_code, client_name, client_industry, brand_id, brand_code, brand_name, brand_category, product_lines, guideline_status, stage, stage_label, stage_percent, task_percent, risk_status, COALESCE(blocker, ''), updated_at FROM platform_project_workbenches WHERE organization_id=? AND project_id=?`, organizationID, projectID).Scan(&value.Organization.Code, &value.Organization.Name, &value.Organization.Owner, &value.Client.ID, &value.Client.Code, &value.Client.Name, &value.Client.Industry, &value.Brand.ID, &value.Brand.Code, &value.Brand.Name, &value.Brand.Category, &productLines, &value.Brand.GuidelineStatus, &value.Project.Stage, &value.Project.StageLabel, &value.Project.StagePercent, &value.Project.TaskPercent, &value.Project.RiskStatus, &value.Project.Blocker, &value.Project.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		value, err = s.defaultWorkbench(ctx, organizationID, projectID)
		productLines = nil
	}
	if err != nil {
		return Workbench{}, err
	}
	if productLines != nil {
		if err := decodeJSON(productLines, &value.Brand.ProductLines); err != nil {
			return Workbench{}, fmt.Errorf("decode workbench product lines: %w", err)
		}
	}
	if value.Brand.ProductLines == nil {
		value.Brand.ProductLines = []string{}
	}
	if value.AdAccountBindings == nil {
		value.AdAccountBindings = []WorkbenchAdAccountBinding{}
	}
	if value.QualityCheckRuns == nil {
		value.QualityCheckRuns = []WorkbenchQualityCheckRun{}
	}
	if value.MaterialConfirmations == nil {
		value.MaterialConfirmations = []WorkbenchMaterialConfirmation{}
	}
	if value.AssetVersionPointers == nil {
		value.AssetVersionPointers = []WorkbenchAssetVersionPointer{}
	}
	value.Organization.ID, value.Organization.Currency, value.Organization.Timezone, value.Organization.UpdatedAt = string(organizationID), "CNY", "Asia/Shanghai", value.Project.UpdatedAt
	value.Client.OrganizationID, value.Client.Owner, value.Client.HealthStatus, value.Client.UpdatedAt = string(organizationID), value.Organization.Owner, "healthy", value.Project.UpdatedAt
	value.Brand.OrganizationID, value.Brand.ClientID, value.Brand.Owner, value.Brand.UpdatedAt = string(organizationID), value.Client.ID, value.Organization.Owner, value.Project.UpdatedAt
	value.Project.ProjectID, value.Project.OrganizationID, value.Project.ClientID, value.Project.BrandID = string(projectID), string(organizationID), value.Client.ID, value.Brand.ID

	accounts, err := s.listWorkbenchAdAccounts(ctx, organizationID, projectID)
	if err != nil {
		return Workbench{}, err
	}
	qualityChecks, err := s.listWorkbenchQualityChecks(ctx, organizationID, projectID)
	if err != nil {
		return Workbench{}, err
	}
	confirmations, err := s.listWorkbenchMaterialConfirmations(ctx, organizationID, projectID)
	if err != nil {
		return Workbench{}, err
	}
	pointers, err := s.listWorkbenchAssetPointers(ctx, organizationID, projectID)
	if err != nil {
		return Workbench{}, err
	}
	assetPointers, err := s.listProjectAssetPointers(ctx, organizationID, projectID)
	if err != nil {
		return Workbench{}, err
	}
	value.AdAccountBindings, value.QualityCheckRuns, value.MaterialConfirmations = accounts, qualityChecks, confirmations
	value.AssetVersionPointers = mergeWorkbenchAssetPointers(pointers, assetPointers)
	return value, nil
}

func (s MySQLStore) defaultWorkbench(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) (Workbench, error) {
	projectValue, err := s.GetProject(ctx, organizationID, projectID)
	if err != nil {
		return Workbench{}, err
	}
	runtime, err := s.GetProjectRuntime(ctx, organizationID, projectID)
	if err != nil {
		return Workbench{}, err
	}
	organizationName := string(organizationID)
	if err := s.DB.QueryRowContext(ctx, `SELECT name FROM organizations WHERE id=?`, organizationID).Scan(&organizationName); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Workbench{}, err
	}
	brandID := string(projectID)
	if projectValue.PrimaryBrandID != nil {
		brandID = string(*projectValue.PrimaryBrandID)
	}
	// Until Identity owns real client records, use the Project ID as the
	// stable fallback identity. A shared "client_unassigned" value makes
	// unrelated projects collapse into one React list item and one apparent
	// workbench context.
	clientID := string(projectID)
	stage := "intake"
	if runtime.Progress >= 100 || runtime.Status == "completed" {
		stage = "completed"
	} else if runtime.Progress >= 70 {
		stage = "delivery"
	} else if runtime.Progress >= 45 {
		stage = "quality_check"
	} else if runtime.Progress >= 20 {
		stage = "creative"
	}
	risk := "healthy"
	if runtime.Status == "blocked" {
		risk = "blocked"
	}
	productLines := []string{}
	if product := strings.TrimSpace(runtime.Product); product != "" && product != "尚未关联产品" {
		productLines = []string{product}
	}
	updatedAt := runtime.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = projectValue.UpdatedAt
	}
	return Workbench{
		Organization:      WorkbenchOrganization{ID: string(organizationID), Code: string(organizationID), Name: organizationName, Owner: runtime.Owner, Currency: "CNY", Timezone: "Asia/Shanghai", UpdatedAt: updatedAt},
		Client:            WorkbenchClient{ID: clientID, OrganizationID: string(organizationID), Code: clientID, Name: "客户未分配", Industry: string(projectValue.Industry), Owner: runtime.Owner, HealthStatus: "healthy", UpdatedAt: updatedAt},
		Brand:             WorkbenchBrand{ID: brandID, OrganizationID: string(organizationID), ClientID: clientID, Code: brandID, Name: runtime.Brand, Category: string(projectValue.Industry), ProductLines: productLines, Owner: runtime.Owner, GuidelineStatus: "missing", UpdatedAt: updatedAt},
		Project:           WorkbenchProject{ProjectID: string(projectID), OrganizationID: string(organizationID), ClientID: clientID, BrandID: brandID, Stage: stage, StageLabel: runtime.Stage, StagePercent: runtime.Progress, TaskPercent: runtime.Progress, RiskStatus: risk, UpdatedAt: updatedAt},
		AdAccountBindings: []WorkbenchAdAccountBinding{}, QualityCheckRuns: []WorkbenchQualityCheckRun{},
		MaterialConfirmations: []WorkbenchMaterialConfirmation{}, AssetVersionPointers: []WorkbenchAssetVersionPointer{},
	}, nil
}

func (s MySQLStore) listProjectAssetPointers(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]WorkbenchAssetVersionPointer, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT pa.asset_id, pa.asset_version, a.owner_system, av.source_type,
		COALESCE(av.provider_job_id, ''), COALESCE(av.render_job_id, ''), av.created_at, a.updated_at
		FROM project_assets pa
		JOIN assets a ON a.organization_id=pa.organization_id AND a.id=pa.asset_id
		JOIN asset_versions av ON av.organization_id=pa.organization_id AND av.asset_id=pa.asset_id AND av.version=pa.asset_version
		WHERE pa.organization_id=? AND pa.project_id=? AND pa.status='active' AND a.status<>'archived'
		ORDER BY pa.asset_id, pa.asset_version DESC`, organizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byAsset := map[string]*WorkbenchAssetVersionPointer{}
	order := []string{}
	for rows.Next() {
		var assetID, owner, sourceType, providerJobID, renderJobID string
		var version int
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&assetID, &version, &owner, &sourceType, &providerJobID, &renderJobID, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		pointer := byAsset[assetID]
		if pointer == nil {
			pointer = &WorkbenchAssetVersionPointer{
				ID: assetID, OrganizationID: string(organizationID), ProjectID: string(projectID), AssetID: assetID,
				WorkingVersion: version, Versions: []WorkbenchAssetVersion{},
				Authorization:  WorkbenchAssetAuthorization{Platforms: []string{}, Regions: []string{}},
				DeliveryTarget: WorkbenchDeliveryTarget{}, Owner: owner, UpdatedAt: updatedAt,
			}
			byAsset[assetID] = pointer
			order = append(order, assetID)
		}
		sourceTaskID := providerJobID
		if sourceTaskID == "" {
			sourceTaskID = renderJobID
		}
		sourceLabel := "项目素材入库"
		versionSourceType := "manual_edit"
		if providerJobID != "" || renderJobID != "" {
			sourceLabel = "模型生成素材入库"
			versionSourceType = "model_generation"
		}
		pointer.Versions = append(pointer.Versions, WorkbenchAssetVersion{
			Version: version, CreatedBy: owner, SourceTaskID: sourceTaskID, SourceType: versionSourceType,
			SourceLabel: sourceLabel, CreatedAt: createdAt, ChangeSummary: sourceType,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]WorkbenchAssetVersionPointer, 0, len(order))
	for _, assetID := range order {
		result = append(result, *byAsset[assetID])
	}
	return result, nil
}

func mergeWorkbenchAssetPointers(persisted, discovered []WorkbenchAssetVersionPointer) []WorkbenchAssetVersionPointer {
	result := make([]WorkbenchAssetVersionPointer, 0, len(persisted)+len(discovered))
	positions := make(map[string]int, len(persisted)+len(discovered))
	for _, pointer := range persisted {
		positions[pointer.AssetID] = len(result)
		result = append(result, pointer)
	}
	for _, source := range discovered {
		position, exists := positions[source.AssetID]
		if !exists {
			positions[source.AssetID] = len(result)
			result = append(result, source)
			continue
		}
		target := &result[position]
		seen := make(map[int]struct{}, len(target.Versions))
		for _, version := range target.Versions {
			seen[version.Version] = struct{}{}
		}
		for _, version := range source.Versions {
			if _, exists := seen[version.Version]; !exists {
				target.Versions = append(target.Versions, version)
			}
		}
		if source.WorkingVersion > target.WorkingVersion {
			target.WorkingVersion = source.WorkingVersion
		}
		if source.UpdatedAt.After(target.UpdatedAt) {
			target.UpdatedAt = source.UpdatedAt
		}
		if target.Owner == "" {
			target.Owner = source.Owner
		}
	}
	return result
}

func (s MySQLStore) UpsertWorkbench(ctx context.Context, value Workbench) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	organizationID, projectID := contract.OrganizationID(value.Project.OrganizationID), contract.ProjectID(value.Project.ProjectID)
	if organizationID == "" || projectID == "" {
		return fmt.Errorf("workbench organization_id and project_id are required")
	}
	lines, err := jsonArray(value.Brand.ProductLines)
	if err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	updatedAt := defaultTimestamp(value.Project.UpdatedAt)
	if _, err = tx.ExecContext(ctx, `INSERT INTO platform_project_workbenches (organization_id, project_id, organization_code, organization_name, organization_owner, client_id, client_code, client_name, client_industry, brand_id, brand_code, brand_name, brand_category, product_lines, guideline_status, stage, stage_label, stage_percent, task_percent, risk_status, blocker, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?) ON DUPLICATE KEY UPDATE organization_code=VALUES(organization_code), organization_name=VALUES(organization_name), organization_owner=VALUES(organization_owner), client_id=VALUES(client_id), client_code=VALUES(client_code), client_name=VALUES(client_name), client_industry=VALUES(client_industry), brand_id=VALUES(brand_id), brand_code=VALUES(brand_code), brand_name=VALUES(brand_name), brand_category=VALUES(brand_category), product_lines=VALUES(product_lines), guideline_status=VALUES(guideline_status), stage=VALUES(stage), stage_label=VALUES(stage_label), stage_percent=VALUES(stage_percent), task_percent=VALUES(task_percent), risk_status=VALUES(risk_status), blocker=VALUES(blocker), updated_at=VALUES(updated_at)`, organizationID, projectID, value.Organization.Code, value.Organization.Name, value.Organization.Owner, value.Client.ID, value.Client.Code, value.Client.Name, value.Client.Industry, value.Brand.ID, value.Brand.Code, value.Brand.Name, value.Brand.Category, lines, value.Brand.GuidelineStatus, value.Project.Stage, value.Project.StageLabel, value.Project.StagePercent, value.Project.TaskPercent, value.Project.RiskStatus, value.Project.Blocker, updatedAt); err != nil {
		return err
	}
	for _, account := range value.AdAccountBindings {
		if err := upsertWorkbenchAdAccount(ctx, tx, organizationID, projectID, account); err != nil {
			return err
		}
	}
	for _, run := range value.QualityCheckRuns {
		if err := upsertWorkbenchQualityCheck(ctx, tx, organizationID, projectID, run); err != nil {
			return err
		}
	}
	for _, confirmation := range value.MaterialConfirmations {
		if err := upsertWorkbenchMaterialConfirmation(ctx, tx, organizationID, projectID, confirmation); err != nil {
			return err
		}
	}
	for _, pointer := range value.AssetVersionPointers {
		if err := upsertWorkbenchAssetPointer(ctx, tx, organizationID, projectID, pointer); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func upsertWorkbenchAdAccount(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, organizationID contract.OrganizationID, projectID contract.ProjectID, value WorkbenchAdAccountBinding) error {
	assets, err := jsonArray(value.BoundAssetIDs)
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx, `INSERT INTO platform_project_workbench_ad_accounts (organization_id, project_id, id, client_id, brand_id, platform, account_name, account_display_id, currency, timezone, permission_status, login_status, tracking_status, owner, bound_asset_ids, last_synced_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE client_id=VALUES(client_id), brand_id=VALUES(brand_id), platform=VALUES(platform), account_name=VALUES(account_name), account_display_id=VALUES(account_display_id), currency=VALUES(currency), timezone=VALUES(timezone), permission_status=VALUES(permission_status), login_status=VALUES(login_status), tracking_status=VALUES(tracking_status), owner=VALUES(owner), bound_asset_ids=VALUES(bound_asset_ids), last_synced_at=VALUES(last_synced_at)`, organizationID, projectID, value.ID, value.ClientID, value.BrandID, value.Platform, value.AccountName, value.AccountDisplayID, value.Currency, value.Timezone, value.PermissionStatus, value.LoginStatus, value.TrackingStatus, value.Owner, assets, defaultTimestamp(value.LastSyncedAt))
	return err
}
func upsertWorkbenchQualityCheck(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, organizationID contract.OrganizationID, projectID contract.ProjectID, value WorkbenchQualityCheckRun) error {
	issues, err := jsonArray(value.Issues)
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx, `INSERT INTO platform_project_workbench_quality_checks (organization_id, project_id, id, asset_id, asset_version, status, model, rule_version, prompt_version, summary, issues, created_at, completed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE asset_id=VALUES(asset_id), asset_version=VALUES(asset_version), status=VALUES(status), model=VALUES(model), rule_version=VALUES(rule_version), prompt_version=VALUES(prompt_version), summary=VALUES(summary), issues=VALUES(issues), created_at=VALUES(created_at), completed_at=VALUES(completed_at)`, organizationID, projectID, value.ID, value.AssetID, value.AssetVersion, value.Status, value.Model, value.RuleVersion, value.PromptVersion, value.Summary, issues, defaultTimestamp(value.CreatedAt), value.CompletedAt)
	return err
}
func upsertWorkbenchMaterialConfirmation(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, organizationID contract.OrganizationID, projectID contract.ProjectID, value WorkbenchMaterialConfirmation) error {
	_, err := exec.ExecContext(ctx, `INSERT INTO platform_project_workbench_material_confirmations (organization_id, project_id, id, quality_check_run_id, asset_id, asset_version, status, scope, confirmed_by, note, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE quality_check_run_id=VALUES(quality_check_run_id), asset_id=VALUES(asset_id), asset_version=VALUES(asset_version), status=VALUES(status), scope=VALUES(scope), confirmed_by=VALUES(confirmed_by), note=VALUES(note), created_at=VALUES(created_at)`, organizationID, projectID, value.ID, value.QualityCheckRunID, value.AssetID, value.AssetVersion, value.Status, value.Scope, value.ConfirmedBy, value.Note, defaultTimestamp(value.CreatedAt))
	return err
}
func upsertWorkbenchAssetPointer(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, organizationID contract.OrganizationID, projectID contract.ProjectID, value WorkbenchAssetVersionPointer) error {
	versions, err := jsonArray(value.Versions)
	if err != nil {
		return err
	}
	platforms, err := jsonArray(value.Authorization.Platforms)
	if err != nil {
		return err
	}
	regions, err := jsonArray(value.Authorization.Regions)
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx, `INSERT INTO platform_project_workbench_asset_pointers (organization_id, project_id, id, asset_id, working_version, quality_checked_version, human_confirmed_version, delivery_version, versions, authorization_platforms, authorization_regions, rights_holder, expires_at, authorization_note, delivery_platform, delivery_region, owner, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE asset_id=VALUES(asset_id), working_version=VALUES(working_version), quality_checked_version=VALUES(quality_checked_version), human_confirmed_version=VALUES(human_confirmed_version), delivery_version=VALUES(delivery_version), versions=VALUES(versions), authorization_platforms=VALUES(authorization_platforms), authorization_regions=VALUES(authorization_regions), rights_holder=VALUES(rights_holder), expires_at=VALUES(expires_at), authorization_note=VALUES(authorization_note), delivery_platform=VALUES(delivery_platform), delivery_region=VALUES(delivery_region), owner=VALUES(owner), updated_at=VALUES(updated_at)`, organizationID, projectID, value.ID, value.AssetID, value.WorkingVersion, value.QualityCheckedVersion, value.HumanConfirmedVersion, value.DeliveryVersion, versions, platforms, regions, value.Authorization.RightsHolder, defaultTimestamp(value.Authorization.ExpiresAt), value.Authorization.Note, value.DeliveryTarget.Platform, value.DeliveryTarget.Region, value.Owner, defaultTimestamp(value.UpdatedAt))
	return err
}

func (s MySQLStore) listWorkbenchAdAccounts(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]WorkbenchAdAccountBinding, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, client_id, brand_id, platform, account_name, account_display_id, currency, timezone, permission_status, login_status, tracking_status, owner, bound_asset_ids, last_synced_at FROM platform_project_workbench_ad_accounts WHERE organization_id=? AND project_id=? ORDER BY id`, organizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WorkbenchAdAccountBinding{}
	for rows.Next() {
		var item WorkbenchAdAccountBinding
		var ids []byte
		if err := rows.Scan(&item.ID, &item.ClientID, &item.BrandID, &item.Platform, &item.AccountName, &item.AccountDisplayID, &item.Currency, &item.Timezone, &item.PermissionStatus, &item.LoginStatus, &item.TrackingStatus, &item.Owner, &ids, &item.LastSyncedAt); err != nil {
			return nil, err
		}
		if err := decodeJSON(ids, &item.BoundAssetIDs); err != nil {
			return nil, err
		}
		item.OrganizationID = string(organizationID)
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s MySQLStore) listWorkbenchQualityChecks(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]WorkbenchQualityCheckRun, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, asset_id, asset_version, status, model, rule_version, prompt_version, summary, issues, created_at, completed_at FROM platform_project_workbench_quality_checks WHERE organization_id=? AND project_id=? ORDER BY created_at DESC, id`, organizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WorkbenchQualityCheckRun{}
	for rows.Next() {
		var item WorkbenchQualityCheckRun
		var issues []byte
		if err := rows.Scan(&item.ID, &item.AssetID, &item.AssetVersion, &item.Status, &item.Model, &item.RuleVersion, &item.PromptVersion, &item.Summary, &issues, &item.CreatedAt, &item.CompletedAt); err != nil {
			return nil, err
		}
		if err := decodeJSON(issues, &item.Issues); err != nil {
			return nil, err
		}
		item.OrganizationID, item.ProjectID = string(organizationID), string(projectID)
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s MySQLStore) listWorkbenchMaterialConfirmations(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]WorkbenchMaterialConfirmation, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, quality_check_run_id, asset_id, asset_version, status, scope, confirmed_by, note, created_at FROM platform_project_workbench_material_confirmations WHERE organization_id=? AND project_id=? ORDER BY created_at DESC, id`, organizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WorkbenchMaterialConfirmation{}
	for rows.Next() {
		var item WorkbenchMaterialConfirmation
		if err := rows.Scan(&item.ID, &item.QualityCheckRunID, &item.AssetID, &item.AssetVersion, &item.Status, &item.Scope, &item.ConfirmedBy, &item.Note, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.OrganizationID, item.ProjectID = string(organizationID), string(projectID)
		result = append(result, item)
	}
	return result, rows.Err()
}
func (s MySQLStore) listWorkbenchAssetPointers(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) ([]WorkbenchAssetVersionPointer, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, asset_id, working_version, quality_checked_version, human_confirmed_version, delivery_version, versions, authorization_platforms, authorization_regions, rights_holder, expires_at, authorization_note, delivery_platform, delivery_region, owner, updated_at FROM platform_project_workbench_asset_pointers WHERE organization_id=? AND project_id=? ORDER BY id`, organizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []WorkbenchAssetVersionPointer{}
	for rows.Next() {
		var item WorkbenchAssetVersionPointer
		var versions, platforms, regions []byte
		if err := rows.Scan(&item.ID, &item.AssetID, &item.WorkingVersion, &item.QualityCheckedVersion, &item.HumanConfirmedVersion, &item.DeliveryVersion, &versions, &platforms, &regions, &item.Authorization.RightsHolder, &item.Authorization.ExpiresAt, &item.Authorization.Note, &item.DeliveryTarget.Platform, &item.DeliveryTarget.Region, &item.Owner, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if err := decodeJSON(versions, &item.Versions); err != nil {
			return nil, err
		}
		if err := decodeJSON(platforms, &item.Authorization.Platforms); err != nil {
			return nil, err
		}
		if err := decodeJSON(regions, &item.Authorization.Regions); err != nil {
			return nil, err
		}
		item.OrganizationID, item.ProjectID = string(organizationID), string(projectID)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s MySQLStore) DeleteOperationalRecord(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, recordID string) error {
	if s.DB == nil {
		return fmt.Errorf("project database is required")
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM platform_project_operations WHERE organization_id=? AND project_id=? AND id=?`, organizationID, projectID, recordID)
	return err
}

func placeholders(count int) string { return strings.TrimRight(strings.Repeat("?,", count), ",") }
func productIDArgs(ids []contract.ProductID) []any {
	values := make([]any, len(ids))
	for i, id := range ids {
		values[i] = id
	}
	return values
}

func (s MySQLStore) GetProject(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID) (Project, error) {
	if s.DB == nil {
		return Project{}, fmt.Errorf("project database is required")
	}
	var project Project
	var brandID, guidelineID sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT p.id, p.organization_id, p.name, p.status, p.industry, p.primary_brand_id,
		p.brand_guideline_version_id, p.project_context_version, p.created_at, p.updated_at, COALESCE(b.status, '')
		FROM projects p LEFT JOIN brands b ON b.organization_id=p.organization_id AND b.id=p.primary_brand_id
		WHERE p.organization_id = ? AND p.id = ?`, organizationID, projectID).Scan(
		&project.ID, &project.OrganizationID, &project.Name, &project.Status, &project.Industry, &brandID,
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
	rows, err := s.DB.QueryContext(ctx, `SELECT p.id, p.organization_id, p.name, p.status, p.industry, p.primary_brand_id,
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
		if err := rows.Scan(&project.ID, &project.OrganizationID, &project.Name, &project.Status, &project.Industry, &brandID,
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

const projectArtifactSelect = `SELECT organization_id, project_id, id, kind, status, content, source_job_id,
	version, created_at, updated_at FROM platform_project_artifacts`

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

func scanProjectArtifact(row projectScanner) (ProjectArtifact, error) {
	var artifact ProjectArtifact
	var sourceJobID sql.NullString
	err := row.Scan(&artifact.OrganizationID, &artifact.ProjectID, &artifact.ID, &artifact.Kind, &artifact.Status,
		&artifact.Content, &sourceJobID, &artifact.Version, &artifact.CreatedAt, &artifact.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectArtifact{}, ErrNotFound
	}
	if err != nil {
		return ProjectArtifact{}, err
	}
	artifact.SourceJobID = sourceJobID.String
	return artifact, nil
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
