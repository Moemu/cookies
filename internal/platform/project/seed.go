package project

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type CanonicalDemoProject struct {
	ProjectID   contract.ProjectID
	BrandID     contract.BrandID
	ProductID   contract.ProductID
	Name        string
	Industry    Industry
	BrandName   string
	ProductName string
}

func (s MySQLStore) EnsureCanonicalDemoProject(ctx context.Context, actor contract.ActorContext, seed CanonicalDemoProject) (Project, error) {
	if s.DB == nil {
		return Project{}, fmt.Errorf("project database is required")
	}
	if err := actor.Validate(); err != nil {
		return Project{}, err
	}
	if strings.TrimSpace(string(seed.ProjectID)) == "" || strings.TrimSpace(string(seed.BrandID)) == "" ||
		strings.TrimSpace(string(seed.ProductID)) == "" || strings.TrimSpace(seed.Name) == "" ||
		strings.TrimSpace(seed.BrandName) == "" || strings.TrimSpace(seed.ProductName) == "" || !seed.Industry.Valid() {
		return Project{}, fmt.Errorf("canonical demo project seed is incomplete")
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return Project{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO brands (id, organization_id, name, status)
		VALUES (?, ?, ?, 'active')
		ON DUPLICATE KEY UPDATE name=VALUES(name), status='active'`,
		seed.BrandID, actor.OrganizationID, seed.BrandName); err != nil {
		return Project{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO products (id, organization_id, name, status)
		VALUES (?, ?, ?, 'active')
		ON DUPLICATE KEY UPDATE name=VALUES(name), status='active'`,
		seed.ProductID, actor.OrganizationID, seed.ProductName); err != nil {
		return Project{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO projects
		(id, organization_id, name, status, industry, primary_brand_id, project_context_version)
		VALUES (?, ?, ?, 'active', ?, ?, 1)
		ON DUPLICATE KEY UPDATE name=VALUES(name), status='active', industry=VALUES(industry), primary_brand_id=VALUES(primary_brand_id), project_context_version=1`,
		seed.ProjectID, actor.OrganizationID, seed.Name, seed.Industry, seed.BrandID); err != nil {
		return Project{}, err
	}
	role := "owner"
	if actor.Principal.Kind == contract.PrincipalService {
		role = "worker"
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO project_memberships
		(organization_id, project_id, principal_kind, principal_id, role, status)
		VALUES (?, ?, ?, ?, ?, 'active')
		ON DUPLICATE KEY UPDATE role=VALUES(role), status='active'`,
		actor.OrganizationID, seed.ProjectID, actor.Principal.Kind, actor.Principal.ID, role); err != nil {
		return Project{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO project_products (organization_id, project_id, product_id)
		VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE product_id=VALUES(product_id)`,
		actor.OrganizationID, seed.ProjectID, seed.ProductID); err != nil {
		return Project{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO project_context_versions
		(organization_id, project_id, version, brand_id, product_ids, created_at)
		VALUES (?, ?, 1, ?, JSON_ARRAY(?), ?)
		ON DUPLICATE KEY UPDATE brand_id=VALUES(brand_id), product_ids=VALUES(product_ids)`,
		actor.OrganizationID, seed.ProjectID, seed.BrandID, seed.ProductID, time.Now().UTC()); err != nil {
		return Project{}, err
	}
	if err := upsertProjectRuntime(ctx, tx, actor.OrganizationID, seed.ProjectID, ProjectRuntime{
		Code:           string(seed.ProjectID),
		Brand:          seed.BrandName,
		Product:        seed.ProductName,
		Goal:           seed.Name,
		Stage:          "需求与策略",
		Progress:       0,
		Status:         "active",
		Owner:          string(actor.Principal.Kind) + ":" + actor.Principal.ID,
		Budget:         0,
		Currency:       "CNY",
		Timezone:       "Asia/Shanghai",
		KnowledgeCount: 0,
		UpdatedAt:      time.Now().UTC(),
	}); err != nil {
		return Project{}, err
	}
	if err := tx.Commit(); err != nil {
		return Project{}, err
	}
	return s.GetProject(ctx, actor.OrganizationID, seed.ProjectID)
}
