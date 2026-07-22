package project

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

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

func nullableBrandID(value *contract.BrandID) any {
	if value == nil {
		return nil
	}
	return *value
}

func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }
