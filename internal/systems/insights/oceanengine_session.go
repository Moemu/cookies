package insights

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type OceanEngineSessionStatus string

const (
	OceanEngineSessionUnverified   OceanEngineSessionStatus = "unverified"
	OceanEngineSessionReady        OceanEngineSessionStatus = "ready"
	OceanEngineSessionAuthRequired OceanEngineSessionStatus = "auth_required"
	OceanEngineSessionDisabled     OceanEngineSessionStatus = "disabled"
)

type OceanEngineSession struct {
	ID                      string                   `json:"id"`
	OrganizationID          contract.OrganizationID  `json:"organization_id"`
	ProjectID               contract.ProjectID       `json:"project_id"`
	Status                  OceanEngineSessionStatus `json:"status"`
	CredentialRefPresent    bool                     `json:"credential_ref_present"`
	SessionCiphertext       []byte                   `json:"-"`
	SessionKeyVersion       string                   `json:"-"`
	LastVerifiedAt          *time.Time               `json:"last_verified_at,omitempty"`
	LastSuccessfulRequestAt *time.Time               `json:"last_successful_request_at,omitempty"`
	LastErrorKind           string                   `json:"last_error_kind,omitempty"`
	LastErrorCode           string                   `json:"last_error_code,omitempty"`
	LastErrorAt             *time.Time               `json:"last_error_at,omitempty"`
	Version                 int64                    `json:"version"`
	CreatedBy               string                   `json:"created_by"`
	CreatedAt               time.Time                `json:"created_at"`
	UpdatedAt               time.Time                `json:"updated_at"`
}

type OceanEngineSessionRepository interface {
	CreateOceanEngineSession(context.Context, OceanEngineSession) (OceanEngineSession, error)
	GetProjectOceanEngineSession(context.Context, contract.OrganizationID, contract.ProjectID) (OceanEngineSession, error)
	UpdateOceanEngineSession(context.Context, OceanEngineSession, int64) (OceanEngineSession, error)
}

type OceanEngineSessionVerifier interface {
	VerifyOceanEngineSession(context.Context, []byte) error
}

type UpdateOceanEngineSessionRequest struct {
	Session         string `json:"session"`
	ExpectedVersion int64  `json:"expected_version,omitempty"`
}
type VerifyOceanEngineSessionRequest struct {
	ExpectedVersion int64 `json:"expected_version"`
}

func (s OceanEngineSession) Validate() error {
	if strings.TrimSpace(s.ID) == "" || strings.TrimSpace(string(s.OrganizationID)) == "" || strings.TrimSpace(string(s.ProjectID)) == "" || strings.TrimSpace(s.CreatedBy) == "" || s.Version < 1 || s.CreatedAt.IsZero() || s.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: 巨量引擎会话身份或版本无效", ErrInvalidRequest)
	}
	switch s.Status {
	case OceanEngineSessionUnverified, OceanEngineSessionReady, OceanEngineSessionAuthRequired, OceanEngineSessionDisabled:
	default:
		return fmt.Errorf("%w: 巨量引擎会话状态无效", ErrInvalidRequest)
	}
	if len(s.SessionCiphertext) == 0 || strings.TrimSpace(s.SessionKeyVersion) == "" {
		return fmt.Errorf("%w: 巨量引擎会话密文缺失", ErrInvalidRequest)
	}
	if (s.LastErrorKind == "") != (s.LastErrorCode == "") {
		return fmt.Errorf("%w: 巨量引擎会话错误摘要不完整", ErrInvalidRequest)
	}
	return nil
}

func (r UpdateOceanEngineSessionRequest) Validate() error {
	if len(strings.TrimSpace(r.Session)) < 8 || len(r.Session) > maxMiyunSessionBytes || r.ExpectedVersion < 0 {
		return ErrInvalidRequest
	}
	return nil
}

func (r VerifyOceanEngineSessionRequest) Validate() error {
	if r.ExpectedVersion < 1 {
		return ErrInvalidRequest
	}
	return nil
}

func (s Service) GetOceanEngineSession(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) (OceanEngineSession, error) {
	if err := s.oceanEngineSessionReady(actor, projectID, ScopeRead); err != nil {
		return OceanEngineSession{}, err
	}
	return s.OceanEngineSessions.GetProjectOceanEngineSession(ctx, actor.OrganizationID, projectID)
}

func (s Service) UpdateOceanEngineSession(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request UpdateOceanEngineSessionRequest) (OceanEngineSession, error) {
	if err := s.oceanEngineSessionReady(actor, projectID, ScopeWrite); err != nil {
		return OceanEngineSession{}, err
	}
	cipher := s.SessionSecrets
	if cipher == nil {
		return OceanEngineSession{}, fmt.Errorf("%w: 巨量引擎会话加密尚未启用", ErrInvalidState)
	}
	if err := request.Validate(); err != nil {
		return OceanEngineSession{}, err
	}
	plaintext := []byte(request.Session)
	ciphertext, keyVersion, err := cipher.Encrypt(plaintext)
	for i := range plaintext {
		plaintext[i] = 0
	}
	if err != nil {
		return OceanEngineSession{}, err
	}
	now := s.now()
	current, err := s.OceanEngineSessions.GetProjectOceanEngineSession(ctx, actor.OrganizationID, projectID)
	if err == ErrNotFound {
		if request.ExpectedVersion != 0 {
			return OceanEngineSession{}, ErrVersionConflict
		}
		id, idErr := s.idGenerator()("oceanenginesession")
		if idErr != nil {
			return OceanEngineSession{}, idErr
		}
		return s.OceanEngineSessions.CreateOceanEngineSession(ctx, OceanEngineSession{ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID, Status: OceanEngineSessionUnverified, CredentialRefPresent: true, SessionCiphertext: ciphertext, SessionKeyVersion: keyVersion, Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now})
	}
	if err != nil {
		return OceanEngineSession{}, err
	}
	if current.Version != request.ExpectedVersion {
		return OceanEngineSession{}, ErrVersionConflict
	}
	current.SessionCiphertext, current.SessionKeyVersion, current.CredentialRefPresent = ciphertext, keyVersion, true
	current.Status, current.LastErrorKind, current.LastErrorCode, current.LastErrorAt, current.UpdatedAt = OceanEngineSessionUnverified, "", "", nil, now
	return s.OceanEngineSessions.UpdateOceanEngineSession(ctx, current, request.ExpectedVersion)
}

func (s Service) VerifyOceanEngineSession(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request VerifyOceanEngineSessionRequest) (OceanEngineSession, error) {
	if err := s.oceanEngineSessionReady(actor, projectID, ScopeWrite); err != nil {
		return OceanEngineSession{}, err
	}
	cipher := s.SessionSecrets
	if cipher == nil || s.OceanEngineVerifier == nil {
		return OceanEngineSession{}, fmt.Errorf("%w: 巨量引擎只读验证尚未启用", ErrInvalidState)
	}
	if err := request.Validate(); err != nil {
		return OceanEngineSession{}, err
	}
	current, err := s.OceanEngineSessions.GetProjectOceanEngineSession(ctx, actor.OrganizationID, projectID)
	if err != nil {
		return OceanEngineSession{}, err
	}
	if current.Version != request.ExpectedVersion {
		return OceanEngineSession{}, ErrVersionConflict
	}
	plaintext, err := cipher.Decrypt(current.SessionCiphertext, current.SessionKeyVersion)
	if err != nil {
		return OceanEngineSession{}, err
	}
	verifyErr := s.OceanEngineVerifier.VerifyOceanEngineSession(ctx, plaintext)
	for i := range plaintext {
		plaintext[i] = 0
	}
	now := s.now()
	current.LastVerifiedAt, current.UpdatedAt = &now, now
	if verifyErr == nil {
		current.Status, current.LastSuccessfulRequestAt, current.LastErrorKind, current.LastErrorCode, current.LastErrorAt = OceanEngineSessionReady, &now, "", "", nil
	} else {
		current.Status, current.LastErrorKind, current.LastErrorCode, current.LastErrorAt = OceanEngineSessionAuthRequired, "verification_failed", "OCEAN_ENGINE_VERIFY_FAILED", &now
	}
	return s.OceanEngineSessions.UpdateOceanEngineSession(ctx, current, request.ExpectedVersion)
}

func (s Service) oceanEngineSessionReady(actor contract.ActorContext, projectID contract.ProjectID, scope contract.Scope) error {
	if s.OceanEngineSessions == nil {
		return fmt.Errorf("巨量引擎会话服务尚未启用")
	}
	if err := actor.Validate(); err != nil || strings.TrimSpace(string(projectID)) == "" {
		return ErrInvalidRequest
	}
	if !actor.HasScope(scope) {
		return fmt.Errorf("%s scope is required", scope)
	}
	return nil
}
