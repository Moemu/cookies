package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/ids"
)

var ErrActorInactive = errors.New("actor is not active in the organization")

type MySQLStore struct{ DB *sql.DB }

func (s MySQLStore) ResolveUserScopes(ctx context.Context, organizationID contract.OrganizationID, userID string) ([]contract.Scope, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("identity database is required")
	}
	var role string
	err := s.DB.QueryRowContext(ctx, `SELECT m.role
		FROM organization_memberships m
		JOIN organizations o ON o.id = m.organization_id
		JOIN users u ON u.id = m.user_id
		WHERE m.organization_id = ? AND m.user_id = ?
		  AND m.status = 'active' AND o.status = 'active' AND u.status = 'active'`,
		organizationID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrActorInactive
	}
	if err != nil {
		return nil, err
	}
	return ScopesForOrganizationRole(role)
}

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

func (s MySQLStore) ListOrganizations(ctx context.Context, actor contract.ActorContext) ([]OrganizationAccess, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("identity database is required")
	}
	if actor.Principal.Kind != contract.PrincipalUser || s.ValidateActor(ctx, actor) != nil {
		return nil, ErrMembershipForbidden
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT o.id, o.name, o.status, o.created_at, o.updated_at,
		m.organization_id, m.user_id, m.role, m.status, m.created_at, m.updated_at
		FROM organizations o JOIN organization_memberships m ON m.organization_id = o.id
		WHERE m.user_id = ? AND m.status = 'active' AND o.status = 'active'
		ORDER BY o.name`, actor.Principal.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]OrganizationAccess, 0)
	for rows.Next() {
		var value OrganizationAccess
		if err := rows.Scan(&value.Organization.ID, &value.Organization.Name, &value.Organization.Status,
			&value.Organization.CreatedAt, &value.Organization.UpdatedAt,
			&value.Membership.OrganizationID, &value.Membership.UserID, &value.Membership.Role,
			&value.Membership.Status, &value.Membership.CreatedAt, &value.Membership.UpdatedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s MySQLStore) UpdateCurrentUser(ctx context.Context, actor contract.ActorContext, displayName string) (User, error) {
	displayName = strings.TrimSpace(displayName)
	if s.DB == nil {
		return User{}, fmt.Errorf("identity database is required")
	}
	if actor.Principal.Kind != contract.PrincipalUser {
		return User{}, ErrMembershipForbidden
	}
	if err := s.ValidateActor(ctx, actor); err != nil {
		return User{}, ErrMembershipForbidden
	}
	if displayName == "" || len([]rune(displayName)) > 80 {
		return User{}, fmt.Errorf("display name must contain 1 to 80 characters")
	}
	if _, err := s.DB.ExecContext(ctx, `UPDATE users SET display_name = ? WHERE id = ? AND status = 'active'`,
		displayName, actor.Principal.ID); err != nil {
		return User{}, err
	}
	current, err := s.GetCurrent(ctx, actor)
	if err != nil || current.User == nil {
		return User{}, err
	}
	return *current.User, nil
}

func (s MySQLStore) ListOrganizationMembers(ctx context.Context, actor contract.ActorContext) ([]OrganizationMember, error) {
	if s.DB == nil {
		return nil, fmt.Errorf("identity database is required")
	}
	if actor.Principal.Kind != contract.PrincipalUser || s.ValidateActor(ctx, actor) != nil {
		return nil, ErrMembershipForbidden
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT u.id, u.display_name, u.status, u.created_at, u.updated_at,
		m.organization_id, m.user_id, m.role, m.status, m.created_at, m.updated_at
		FROM organization_memberships m JOIN users u ON u.id = m.user_id
		WHERE m.organization_id = ? ORDER BY m.created_at`, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make([]OrganizationMember, 0)
	for rows.Next() {
		var value OrganizationMember
		if err := rows.Scan(&value.User.ID, &value.User.DisplayName, &value.User.Status,
			&value.User.CreatedAt, &value.User.UpdatedAt, &value.Membership.OrganizationID,
			&value.Membership.UserID, &value.Membership.Role, &value.Membership.Status,
			&value.Membership.CreatedAt, &value.Membership.UpdatedAt); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s MySQLStore) AddOrganizationMember(ctx context.Context, actor contract.ActorContext, userID, role string) (OrganizationMember, error) {
	if s.DB == nil {
		return OrganizationMember{}, fmt.Errorf("identity database is required")
	}
	if !ValidOrganizationRole(role) || strings.TrimSpace(userID) == "" {
		return OrganizationMember{}, fmt.Errorf("valid user and role are required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return OrganizationMember{}, err
	}
	defer tx.Rollback()
	actorRole, err := membershipRole(ctx, tx, actor.OrganizationID, actor.Principal.ID)
	if err != nil || (actorRole != "owner" && actorRole != "admin") || (role == "owner" && actorRole != "owner") {
		return OrganizationMember{}, ErrMembershipForbidden
	}
	var user User
	if err := tx.QueryRowContext(ctx, `SELECT id, display_name, status, created_at, updated_at
		FROM users WHERE id = ? AND status = 'active'`, userID).Scan(
		&user.ID, &user.DisplayName, &user.Status, &user.CreatedAt, &user.UpdatedAt); errors.Is(err, sql.ErrNoRows) {
		return OrganizationMember{}, ErrUserNotFound
	} else if err != nil {
		return OrganizationMember{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO organization_memberships
		(organization_id, user_id, role, status) VALUES (?, ?, ?, 'active')`,
		actor.OrganizationID, userID, role); err != nil {
		if duplicateKey(err) {
			return OrganizationMember{}, ErrMembershipConflict
		}
		return OrganizationMember{}, err
	}
	member := OrganizationMember{User: user}
	if err := tx.QueryRowContext(ctx, `SELECT organization_id, user_id, role, status, created_at, updated_at
		FROM organization_memberships WHERE organization_id = ? AND user_id = ?`,
		actor.OrganizationID, userID).Scan(&member.Membership.OrganizationID, &member.Membership.UserID,
		&member.Membership.Role, &member.Membership.Status, &member.Membership.CreatedAt,
		&member.Membership.UpdatedAt); err != nil {
		return OrganizationMember{}, err
	}
	if err := writeIdentityAudit(ctx, tx, actor, "organization_membership", userID, "member.added", nil, member.Membership); err != nil {
		return OrganizationMember{}, err
	}
	if err := tx.Commit(); err != nil {
		return OrganizationMember{}, err
	}
	return member, nil
}

func (s MySQLStore) UpdateOrganizationMember(ctx context.Context, actor contract.ActorContext, userID string, request UpdateOrganizationMembershipRequest) (OrganizationMember, error) {
	if s.DB == nil {
		return OrganizationMember{}, fmt.Errorf("identity database is required")
	}
	if strings.TrimSpace(userID) == "" || !ValidOrganizationRole(request.Role) ||
		!ValidMembershipStatus(request.Status) || request.ExpectedUpdatedAt.IsZero() {
		return OrganizationMember{}, fmt.Errorf("valid role, status, and expected_updated_at are required")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return OrganizationMember{}, err
	}
	defer tx.Rollback()
	actorRole, err := membershipRole(ctx, tx, actor.OrganizationID, actor.Principal.ID)
	if err != nil || (actorRole != "owner" && actorRole != "admin") {
		return OrganizationMember{}, ErrMembershipForbidden
	}
	var before OrganizationMembership
	if err := tx.QueryRowContext(ctx, `SELECT organization_id, user_id, role, status, created_at, updated_at
		FROM organization_memberships WHERE organization_id = ? AND user_id = ? FOR UPDATE`,
		actor.OrganizationID, userID).Scan(&before.OrganizationID, &before.UserID, &before.Role,
		&before.Status, &before.CreatedAt, &before.UpdatedAt); errors.Is(err, sql.ErrNoRows) {
		return OrganizationMember{}, ErrMembershipNotFound
	} else if err != nil {
		return OrganizationMember{}, err
	}
	if !before.UpdatedAt.Equal(request.ExpectedUpdatedAt) {
		return OrganizationMember{}, ErrMembershipConflict
	}
	if actorRole == "admin" && (userID == actor.Principal.ID || before.Role == "owner" || request.Role == "owner") {
		return OrganizationMember{}, ErrMembershipForbidden
	}
	removesOwner := before.Role == "owner" && before.Status == "active" &&
		(request.Role != "owner" || request.Status != "active")
	if removesOwner {
		rows, err := tx.QueryContext(ctx, `SELECT user_id FROM organization_memberships
			WHERE organization_id = ? AND role = 'owner' AND status = 'active' FOR UPDATE`, actor.OrganizationID)
		if err != nil {
			return OrganizationMember{}, err
		}
		count := 0
		for rows.Next() {
			count++
		}
		if err := rows.Close(); err != nil {
			return OrganizationMember{}, err
		}
		if count <= 1 {
			return OrganizationMember{}, ErrLastOwner
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE organization_memberships SET role = ?, status = ?
		WHERE organization_id = ? AND user_id = ? AND updated_at = ?`,
		request.Role, request.Status, actor.OrganizationID, userID, request.ExpectedUpdatedAt)
	if err != nil {
		return OrganizationMember{}, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return OrganizationMember{}, err
	}
	if affected != 1 {
		return OrganizationMember{}, ErrMembershipConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE identity_sessions SET revoked_at = CURRENT_TIMESTAMP(6)
		WHERE organization_id = ? AND user_id = ? AND revoked_at IS NULL`, actor.OrganizationID, userID); err != nil {
		return OrganizationMember{}, err
	}
	var member OrganizationMember
	if err := tx.QueryRowContext(ctx, `SELECT u.id, u.display_name, u.status, u.created_at, u.updated_at,
		m.organization_id, m.user_id, m.role, m.status, m.created_at, m.updated_at
		FROM organization_memberships m JOIN users u ON u.id = m.user_id
		WHERE m.organization_id = ? AND m.user_id = ?`, actor.OrganizationID, userID).Scan(
		&member.User.ID, &member.User.DisplayName, &member.User.Status, &member.User.CreatedAt,
		&member.User.UpdatedAt, &member.Membership.OrganizationID, &member.Membership.UserID,
		&member.Membership.Role, &member.Membership.Status, &member.Membership.CreatedAt,
		&member.Membership.UpdatedAt); err != nil {
		return OrganizationMember{}, err
	}
	if err := writeIdentityAudit(ctx, tx, actor, "organization_membership", userID, "member.updated", before, member.Membership); err != nil {
		return OrganizationMember{}, err
	}
	if err := tx.Commit(); err != nil {
		return OrganizationMember{}, err
	}
	return member, nil
}

func membershipRole(ctx context.Context, tx *sql.Tx, organizationID contract.OrganizationID, userID string) (string, error) {
	var role string
	err := tx.QueryRowContext(ctx, `SELECT role FROM organization_memberships
		WHERE organization_id = ? AND user_id = ? AND status = 'active' FOR UPDATE`,
		organizationID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrMembershipForbidden
	}
	return role, err
}

func writeIdentityAudit(ctx context.Context, tx *sql.Tx, actor contract.ActorContext, targetKind, targetID, action string, before, after any) error {
	id, err := ids.New("identity_audit")
	if err != nil {
		return err
	}
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return err
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO identity_audit_events
		(id, organization_id, actor_user_id, target_kind, target_id, action, before_state, after_state)
		VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, 'null'), NULLIF(?, 'null'))`,
		id, actor.OrganizationID, actor.Principal.ID, targetKind, targetID, action, beforeJSON, afterJSON)
	return err
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
