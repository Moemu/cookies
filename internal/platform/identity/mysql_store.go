package identity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var ErrActorInactive = errors.New("actor is not active in the organization")

type MySQLStore struct{ DB *sql.DB }

func (s MySQLStore) ValidateActor(ctx context.Context, actor contract.ActorContext) error {
	if s.DB == nil {
		return fmt.Errorf("identity database is required")
	}
	if err := actor.Validate(); err != nil {
		return ErrActorInactive
	}
	switch actor.Principal.Kind {
	case contract.PrincipalUser:
		var exists int
		err := s.DB.QueryRowContext(ctx, `SELECT 1
			FROM organizations o
			JOIN organization_memberships m ON m.organization_id = o.id
			JOIN users u ON u.id = m.user_id
			WHERE o.id = ? AND o.status = 'active' AND m.user_id = ? AND m.status = 'active' AND u.status = 'active'`,
			actor.OrganizationID, actor.Principal.ID).Scan(&exists)
		if err == sql.ErrNoRows {
			return ErrActorInactive
		}
		return err
	case contract.PrincipalService:
		var exists int
		err := s.DB.QueryRowContext(ctx, `SELECT 1
			FROM organizations o
			JOIN service_identities s ON s.organization_id = o.id
			WHERE o.id = ? AND o.status = 'active' AND s.id = ? AND s.status = 'active'`,
			actor.OrganizationID, actor.Principal.ID).Scan(&exists)
		if err == sql.ErrNoRows {
			return ErrActorInactive
		}
		if err != nil {
			return err
		}
		for _, scope := range actor.Scopes {
			err := s.DB.QueryRowContext(ctx, `SELECT 1 FROM service_identity_scopes
				WHERE organization_id = ? AND service_identity_id = ? AND scope = ?`,
				actor.OrganizationID, actor.Principal.ID, scope).Scan(&exists)
			if err == sql.ErrNoRows {
				return ErrActorInactive
			}
			if err != nil {
				return err
			}
		}
		return nil
	default:
		return ErrActorInactive
	}
}

func (s MySQLStore) GetCurrent(ctx context.Context, actor contract.ActorContext) (CurrentIdentity, error) {
	if err := s.ValidateActor(ctx, actor); err != nil {
		return CurrentIdentity{}, err
	}
	current := CurrentIdentity{Actor: actor}
	if err := s.DB.QueryRowContext(ctx, `SELECT id, name, status, created_at, updated_at
		FROM organizations WHERE id = ?`, actor.OrganizationID).Scan(
		&current.Organization.ID, &current.Organization.Name, &current.Organization.Status,
		&current.Organization.CreatedAt, &current.Organization.UpdatedAt); err != nil {
		return CurrentIdentity{}, err
	}
	if actor.Principal.Kind == contract.PrincipalUser {
		user := &User{}
		membership := &OrganizationMembership{}
		if err := s.DB.QueryRowContext(ctx, `SELECT u.id, u.display_name, u.status, u.created_at, u.updated_at,
			m.organization_id, m.user_id, m.role, m.status, m.created_at, m.updated_at
			FROM users u JOIN organization_memberships m ON m.user_id = u.id
			WHERE m.organization_id = ? AND u.id = ?`, actor.OrganizationID, actor.Principal.ID).Scan(
			&user.ID, &user.DisplayName, &user.Status, &user.CreatedAt, &user.UpdatedAt,
			&membership.OrganizationID, &membership.UserID, &membership.Role, &membership.Status,
			&membership.CreatedAt, &membership.UpdatedAt); err != nil {
			return CurrentIdentity{}, err
		}
		current.User = user
		current.Membership = membership
	}
	return current, nil
}

// EnsureLocalActor creates only local-development seed data. Production
// identities must be provisioned through an external identity adapter.
func (s MySQLStore) EnsureLocalActor(ctx context.Context, actor contract.ActorContext) error {
	if s.DB == nil {
		return fmt.Errorf("identity database is required")
	}
	if err := actor.Validate(); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO organizations (id, name, status) VALUES (?, ?, 'active')
		ON DUPLICATE KEY UPDATE name = VALUES(name)`, actor.OrganizationID, "Local Organization"); err != nil {
		return err
	}
	switch actor.Principal.Kind {
	case contract.PrincipalUser:
		if _, err := tx.ExecContext(ctx, `INSERT INTO users (id, display_name, status) VALUES (?, ?, 'active')
			ON DUPLICATE KEY UPDATE display_name = VALUES(display_name)`, actor.Principal.ID, "Local User"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO organization_memberships (organization_id, user_id, role, status)
			VALUES (?, ?, 'owner', 'active') ON DUPLICATE KEY UPDATE role = 'owner', status = 'active'`,
			actor.OrganizationID, actor.Principal.ID); err != nil {
			return err
		}
	case contract.PrincipalService:
		if _, err := tx.ExecContext(ctx, `INSERT INTO service_identities (id, organization_id, name, status)
			VALUES (?, ?, ?, 'active') ON DUPLICATE KEY UPDATE name = VALUES(name), status = 'active'`,
			actor.Principal.ID, actor.OrganizationID, "Local Service"); err != nil {
			return err
		}
		for _, scope := range actor.Scopes {
			if _, err := tx.ExecContext(ctx, `INSERT IGNORE INTO service_identity_scopes (organization_id, service_identity_id, scope)
				VALUES (?, ?, ?)`, actor.OrganizationID, actor.Principal.ID, scope); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
