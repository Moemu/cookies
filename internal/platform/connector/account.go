package connector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type PlatformAccount struct {
	ID                   string     `json:"id"`
	OrganizationID       string     `json:"organization_id"`
	ProjectID            string     `json:"project_id"`
	Platform             string     `json:"platform"`
	DisplayLabel         string     `json:"display_label"`
	Status               string     `json:"status"`
	VerifiedAt           *time.Time `json:"verified_at,omitempty"`
	LastCheckedAt        *time.Time `json:"last_checked_at,omitempty"`
	CredentialRefPresent bool       `json:"credential_ref_present"`
}
type RegisterAccountRequest struct {
	OrganizationID string
	ProjectID      string
	ExternalID     string
	DisplayLabel   string
	CredentialRef  string
}
type AccountProbe interface {
	Verify(context.Context, string, string, string) error
}
type AccountStore interface {
	RegisterAccount(context.Context, RegisterAccountRequest) (PlatformAccount, error)
	ListAccounts(context.Context, string, string) ([]PlatformAccount, error)
	MarkAccountVerified(context.Context, string, string, string, time.Time) (PlatformAccount, error)
	ResolveAnyExternalAccountID(context.Context, string, string, string) (string, error)
	RevokeAccount(context.Context, string, string, string, time.Time) (PlatformAccount, error)
}
type AccountService struct {
	Store AccountStore
	Probe AccountProbe
	Now   func() time.Time
}

func (s AccountService) Register(ctx context.Context, request RegisterAccountRequest) (PlatformAccount, error) {
	externalID, credentialRef := strings.TrimSpace(request.ExternalID), strings.TrimSpace(request.CredentialRef)
	if s.Store == nil || request.OrganizationID == "" || request.ProjectID == "" || externalID == "" || len(externalID) > 191 || credentialRef == "" || len(credentialRef) > 191 || strings.ContainsAny(externalID, "\r\n\t") || containsSensitiveText(credentialRef) {
		return PlatformAccount{}, ErrInvalidFact
	}
	request.ExternalID, request.CredentialRef = externalID, credentialRef
	return s.Store.RegisterAccount(ctx, request)
}
func (s AccountService) List(ctx context.Context, organizationID, projectID string) ([]PlatformAccount, error) {
	if s.Store == nil {
		return nil, ErrInvalidFact
	}
	return s.Store.ListAccounts(ctx, organizationID, projectID)
}
func (s AccountService) Verify(ctx context.Context, organizationID, projectID, accountID string) (PlatformAccount, error) {
	if s.Store == nil || s.Probe == nil {
		return PlatformAccount{}, ErrInvalidFact
	}
	externalID, err := s.Store.ResolveAnyExternalAccountID(ctx, organizationID, projectID, accountID)
	if err != nil {
		return PlatformAccount{}, err
	}
	if err = s.Probe.Verify(ctx, organizationID, projectID, externalID); err != nil {
		return PlatformAccount{}, err
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	return s.Store.MarkAccountVerified(ctx, organizationID, projectID, accountID, now)
}
func (s AccountService) Revoke(ctx context.Context, organizationID, projectID, accountID string) (PlatformAccount, error) {
	if s.Store == nil {
		return PlatformAccount{}, ErrInvalidFact
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	return s.Store.RevokeAccount(ctx, organizationID, projectID, accountID, now)
}

func (r MySQLRepository) RegisterAccount(ctx context.Context, request RegisterAccountRequest) (PlatformAccount, error) {
	db, err := r.db()
	if err != nil {
		return PlatformAccount{}, err
	}
	now := time.Now().UTC()
	id := "oeacct_" + canonicalHash([]string{request.OrganizationID, request.ProjectID, request.ExternalID})
	connectionID := "oeconn_" + canonicalHash([]string{request.OrganizationID, request.ProjectID, id})
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return PlatformAccount{}, err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO platform_accounts (id,organization_id,platform,external_id,display_label,status,created_at,updated_at) VALUES (?,?,'ocean_engine',?,?, 'pending',?,?) ON DUPLICATE KEY UPDATE display_label=VALUES(display_label),updated_at=VALUES(updated_at)`, id, request.OrganizationID, request.ExternalID, request.DisplayLabel, now, now)
	if err != nil {
		return PlatformAccount{}, fmt.Errorf("register platform account: %w", err)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO platform_account_connections (id,organization_id,project_id,account_id,connection_type,credential_ref,status,created_at,updated_at) VALUES (?,?,?,?,'web_api',?,'pending',?,?) ON DUPLICATE KEY UPDATE credential_ref=VALUES(credential_ref),updated_at=VALUES(updated_at)`, connectionID, request.OrganizationID, request.ProjectID, id, request.CredentialRef, now, now)
	if err != nil {
		return PlatformAccount{}, fmt.Errorf("register platform connection: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return PlatformAccount{}, err
	}
	values, err := r.ListAccounts(ctx, request.OrganizationID, request.ProjectID)
	if err != nil {
		return PlatformAccount{}, err
	}
	for _, value := range values {
		if value.ID == id {
			return value, nil
		}
	}
	return PlatformAccount{}, sql.ErrNoRows
}
func (r MySQLRepository) ListAccounts(ctx context.Context, organizationID, projectID string) ([]PlatformAccount, error) {
	db, err := r.db()
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT a.id,a.display_label,a.status,a.verified_at,a.last_checked_at,(c.credential_ref<>'') FROM platform_accounts a JOIN platform_account_connections c ON c.organization_id=a.organization_id AND c.account_id=a.id WHERE a.organization_id=? AND c.project_id=? AND a.platform='ocean_engine' ORDER BY a.created_at,a.id`, organizationID, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PlatformAccount{}
	for rows.Next() {
		var value PlatformAccount
		var verified, last sql.NullTime
		value.OrganizationID, value.ProjectID, value.Platform = organizationID, projectID, SourceSystem
		if err := rows.Scan(&value.ID, &value.DisplayLabel, &value.Status, &verified, &last, &value.CredentialRefPresent); err != nil {
			return nil, err
		}
		if verified.Valid {
			value.VerifiedAt = &verified.Time
		}
		if last.Valid {
			value.LastCheckedAt = &last.Time
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
func (r MySQLRepository) ResolveAnyExternalAccountID(ctx context.Context, organizationID, projectID, accountID string) (string, error) {
	db, err := r.db()
	if err != nil {
		return "", err
	}
	var value string
	err = db.QueryRowContext(ctx, `SELECT a.external_id FROM platform_accounts a JOIN platform_account_connections c ON c.organization_id=a.organization_id AND c.account_id=a.id WHERE a.organization_id=? AND c.project_id=? AND a.id=? AND a.platform='ocean_engine' AND a.status<>'revoked' AND c.status<>'revoked'`, organizationID, projectID, accountID).Scan(&value)
	return value, err
}
func (r MySQLRepository) MarkAccountVerified(ctx context.Context, organizationID, projectID, accountID string, now time.Time) (PlatformAccount, error) {
	db, err := r.db()
	if err != nil {
		return PlatformAccount{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return PlatformAccount{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE platform_accounts SET status='verified',verified_at=?,last_checked_at=?,updated_at=? WHERE organization_id=? AND id=? AND status<>'revoked'`, now, now, now, organizationID, accountID)
	if err != nil {
		return PlatformAccount{}, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return PlatformAccount{}, sql.ErrNoRows
	}
	result, err = tx.ExecContext(ctx, `UPDATE platform_account_connections SET status='verified',last_verified_at=?,updated_at=? WHERE organization_id=? AND project_id=? AND account_id=? AND status<>'revoked'`, now, now, organizationID, projectID, accountID)
	if err != nil {
		return PlatformAccount{}, err
	}
	count, _ = result.RowsAffected()
	if count != 1 {
		return PlatformAccount{}, sql.ErrNoRows
	}
	if err = tx.Commit(); err != nil {
		return PlatformAccount{}, err
	}
	values, err := r.ListAccounts(ctx, organizationID, projectID)
	if err != nil {
		return PlatformAccount{}, err
	}
	for _, value := range values {
		if value.ID == accountID {
			return value, nil
		}
	}
	return PlatformAccount{}, sql.ErrNoRows
}
func (r MySQLRepository) RevokeAccount(ctx context.Context, organizationID, projectID, accountID string, now time.Time) (PlatformAccount, error) {
	db, err := r.db()
	if err != nil {
		return PlatformAccount{}, err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return PlatformAccount{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE platform_accounts SET status='revoked',updated_at=? WHERE organization_id=? AND id=? AND status<>'revoked'`, now, organizationID, accountID)
	if err != nil {
		return PlatformAccount{}, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return PlatformAccount{}, sql.ErrNoRows
	}
	result, err = tx.ExecContext(ctx, `UPDATE platform_account_connections SET status='revoked',updated_at=? WHERE organization_id=? AND project_id=? AND account_id=? AND status<>'revoked'`, now, organizationID, projectID, accountID)
	if err != nil {
		return PlatformAccount{}, err
	}
	count, _ = result.RowsAffected()
	if count != 1 {
		return PlatformAccount{}, sql.ErrNoRows
	}
	if err = tx.Commit(); err != nil {
		return PlatformAccount{}, err
	}
	values, err := r.ListAccounts(ctx, organizationID, projectID)
	if err != nil {
		return PlatformAccount{}, err
	}
	for _, value := range values {
		if value.ID == accountID {
			return value, nil
		}
	}
	return PlatformAccount{}, sql.ErrNoRows
}
