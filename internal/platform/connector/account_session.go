package connector

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type AccountSessionStatus string

const (
	AccountSessionUnverified   AccountSessionStatus = "unverified"
	AccountSessionReady        AccountSessionStatus = "ready"
	AccountSessionAuthRequired AccountSessionStatus = "auth_required"
	AccountSessionDisabled     AccountSessionStatus = "disabled"
)

type OceanEngineAccountSession struct {
	ID                   string               `json:"id"`
	OrganizationID       string               `json:"organization_id"`
	AccountID            string               `json:"account_id"`
	Status               AccountSessionStatus `json:"status"`
	CredentialRefPresent bool                 `json:"credential_ref_present"`
	SessionCiphertext    []byte               `json:"-"`
	SessionKeyVersion    string               `json:"-"`
	LastVerifiedAt       *time.Time           `json:"last_verified_at,omitempty"`
	Version              int64                `json:"version"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
}

type AccountSessionCipher interface {
	Encrypt([]byte) ([]byte, string, error)
	Decrypt([]byte, string) ([]byte, error)
}

type AccountSessionStore interface {
	GetAccountSession(context.Context, string, string) (OceanEngineAccountSession, error)
	PutAccountSession(context.Context, OceanEngineAccountSession, int64) (OceanEngineAccountSession, error)
	MarkAccountSessionVerified(context.Context, string, string, int64, time.Time) (OceanEngineAccountSession, error)
}

type AccountSessionService struct {
	Store  AccountSessionStore
	Cipher AccountSessionCipher
	Now    func() time.Time
}

func (s AccountSessionService) Get(ctx context.Context, organizationID, accountID string) (OceanEngineAccountSession, error) {
	if s.Store == nil || strings.TrimSpace(organizationID) == "" || !strings.HasPrefix(strings.TrimSpace(accountID), "oeacct_") {
		return OceanEngineAccountSession{}, ErrInvalidFact
	}
	return s.Store.GetAccountSession(ctx, organizationID, accountID)
}

func (s AccountSessionService) Update(ctx context.Context, organizationID, accountID string, session []byte, expectedVersion int64) (OceanEngineAccountSession, error) {
	if s.Store == nil || s.Cipher == nil || strings.TrimSpace(organizationID) == "" || !strings.HasPrefix(strings.TrimSpace(accountID), "oeacct_") || len(session) == 0 || len(session) > 16<<10 || expectedVersion < 0 {
		return OceanEngineAccountSession{}, ErrInvalidFact
	}
	ciphertext, keyVersion, err := s.Cipher.Encrypt(session)
	if err != nil || len(ciphertext) == 0 || strings.TrimSpace(keyVersion) == "" {
		return OceanEngineAccountSession{}, fmt.Errorf("encrypt Connector account session")
	}
	now := time.Now().UTC()
	if s.Now != nil {
		now = s.Now().UTC()
	}
	value := OceanEngineAccountSession{ID: "oeaccountsession_" + canonicalHash([]string{organizationID, accountID}), OrganizationID: organizationID, AccountID: accountID, Status: AccountSessionUnverified, CredentialRefPresent: true, SessionCiphertext: ciphertext, SessionKeyVersion: keyVersion, Version: 1, CreatedAt: now, UpdatedAt: now}
	return s.Store.PutAccountSession(ctx, value, expectedVersion)
}

func (r MySQLRepository) GetAccountSession(ctx context.Context, organizationID, accountID string) (OceanEngineAccountSession, error) {
	db, err := r.db()
	if err != nil {
		return OceanEngineAccountSession{}, err
	}
	var value OceanEngineAccountSession
	var verified sql.NullTime
	err = db.QueryRowContext(ctx, `SELECT id,organization_id,account_id,status,session_ciphertext,session_key_version,last_verified_at,version,created_at,updated_at FROM connector_ocean_engine_account_sessions WHERE organization_id=? AND account_id=?`, organizationID, accountID).Scan(&value.ID, &value.OrganizationID, &value.AccountID, &value.Status, &value.SessionCiphertext, &value.SessionKeyVersion, &verified, &value.Version, &value.CreatedAt, &value.UpdatedAt)
	if err != nil {
		return OceanEngineAccountSession{}, err
	}
	value.CredentialRefPresent = len(value.SessionCiphertext) > 0
	if verified.Valid {
		value.LastVerifiedAt = &verified.Time
	}
	return value, nil
}

func (r MySQLRepository) PutAccountSession(ctx context.Context, value OceanEngineAccountSession, expectedVersion int64) (OceanEngineAccountSession, error) {
	db, err := r.db()
	if err != nil {
		return OceanEngineAccountSession{}, err
	}
	if expectedVersion == 0 {
		_, err = db.ExecContext(ctx, `INSERT INTO connector_ocean_engine_account_sessions (id,organization_id,account_id,status,session_ciphertext,session_key_version,last_verified_at,version,created_at,updated_at) VALUES (?,?,?,?,?,?,NULL,1,?,?)`, value.ID, value.OrganizationID, value.AccountID, value.Status, value.SessionCiphertext, value.SessionKeyVersion, value.CreatedAt, value.UpdatedAt)
	} else {
		var result sql.Result
		result, err = db.ExecContext(ctx, `UPDATE connector_ocean_engine_account_sessions SET status='unverified',session_ciphertext=?,session_key_version=?,last_verified_at=NULL,version=version+1,updated_at=? WHERE organization_id=? AND account_id=? AND version=?`, value.SessionCiphertext, value.SessionKeyVersion, value.UpdatedAt, value.OrganizationID, value.AccountID, expectedVersion)
		if err == nil {
			count, _ := result.RowsAffected()
			if count != 1 {
				return OceanEngineAccountSession{}, ErrImmutableConflict
			}
		}
	}
	if err != nil {
		return OceanEngineAccountSession{}, err
	}
	return r.GetAccountSession(ctx, value.OrganizationID, value.AccountID)
}

func (r MySQLRepository) MarkAccountSessionVerified(ctx context.Context, organizationID, accountID string, expectedVersion int64, now time.Time) (OceanEngineAccountSession, error) {
	db, err := r.db()
	if err != nil {
		return OceanEngineAccountSession{}, err
	}
	result, err := db.ExecContext(ctx, `UPDATE connector_ocean_engine_account_sessions SET status='ready',last_verified_at=?,version=version+1,updated_at=? WHERE organization_id=? AND account_id=? AND version=? AND status<>'disabled'`, now, now, organizationID, accountID, expectedVersion)
	if err != nil {
		return OceanEngineAccountSession{}, err
	}
	count, _ := result.RowsAffected()
	if count != 1 {
		return OceanEngineAccountSession{}, ErrImmutableConflict
	}
	return r.GetAccountSession(ctx, organizationID, accountID)
}
