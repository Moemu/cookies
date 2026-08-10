package insights

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/integrations/crawler"
	"github.com/shikanon/cookies/internal/platform/contract"
)

type UpdateMiyunConnectionRequest struct {
	Session          string     `json:"session"`
	SessionExpiresAt *time.Time `json:"session_expires_at,omitempty"`
	ExpectedVersion  int64      `json:"expected_version,omitempty"`
}

type VerifyMiyunConnectionRequest struct {
	ExpectedVersion int64 `json:"expected_version"`
}

type MiyunConnectionVerifier interface {
	VerifyMiyunConnection(context.Context, []byte) error
}

type AESGCMMiyunSecretCipher struct {
	AEAD       cipher.AEAD
	KeyVersion string
}

func NewAESGCMMiyunSecretCipher(encodedKey, keyVersion string) (*AESGCMMiyunSecretCipher, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encodedKey))
	if err != nil || len(key) != 32 || strings.TrimSpace(keyVersion) == "" {
		return nil, fmt.Errorf("Miyun master key must be a base64-encoded 32-byte key with a version")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AESGCMMiyunSecretCipher{AEAD: aead, KeyVersion: strings.TrimSpace(keyVersion)}, nil
}

func (c *AESGCMMiyunSecretCipher) Encrypt(plaintext []byte) ([]byte, string, error) {
	if c == nil || c.AEAD == nil || len(plaintext) == 0 {
		return nil, "", fmt.Errorf("Miyun cipher and plaintext are required")
	}
	nonce := make([]byte, c.AEAD.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, "", err
	}
	return c.AEAD.Seal(nonce, nonce, plaintext, nil), c.KeyVersion, nil
}

func (c *AESGCMMiyunSecretCipher) Decrypt(ciphertext []byte, keyVersion string) ([]byte, error) {
	if c == nil || c.AEAD == nil || keyVersion != c.KeyVersion || len(ciphertext) <= c.AEAD.NonceSize() {
		return nil, fmt.Errorf("Miyun ciphertext cannot be decrypted with the configured key version")
	}
	nonceSize := c.AEAD.NonceSize()
	return c.AEAD.Open(nil, ciphertext[:nonceSize], ciphertext[nonceSize:], nil)
}

func (s Service) GetMiyunConnection(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID) (MiyunConnection, error) {
	if err := s.miyunReady(actor, projectID, ScopeRead); err != nil {
		return MiyunConnection{}, err
	}
	return s.Miyun.GetProjectMiyunConnection(ctx, actor.OrganizationID, projectID)
}

func (s Service) UpdateMiyunConnection(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request UpdateMiyunConnectionRequest) (MiyunConnection, error) {
	if err := s.miyunReady(actor, projectID, ScopeWrite); err != nil {
		return MiyunConnection{}, err
	}
	if s.MiyunSecrets == nil || len(request.Session) < 8 || len(request.Session) > 4096 {
		return MiyunConnection{}, ErrInvalidRequest
	}
	if request.SessionExpiresAt != nil && !request.SessionExpiresAt.After(s.now()) {
		return MiyunConnection{}, ErrInvalidRequest
	}
	plaintext := []byte(request.Session)
	ciphertext, keyVersion, err := s.MiyunSecrets.Encrypt(plaintext)
	for index := range plaintext {
		plaintext[index] = 0
	}
	if err != nil {
		return MiyunConnection{}, err
	}
	now := s.now()
	current, err := s.Miyun.GetProjectMiyunConnection(ctx, actor.OrganizationID, projectID)
	if errors.Is(err, ErrNotFound) {
		if request.ExpectedVersion != 0 {
			return MiyunConnection{}, ErrVersionConflict
		}
		id, idErr := s.idGenerator()("miyunconnection")
		if idErr != nil {
			return MiyunConnection{}, idErr
		}
		value := MiyunConnection{ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
			Status: MiyunConnectionUnverified, SessionCiphertext: ciphertext, SessionKeyVersion: keyVersion,
			SessionExpiresAt: request.SessionExpiresAt, Version: 1, CreatedBy: actor.Principal.ID, CreatedAt: now, UpdatedAt: now}
		return s.Miyun.CreateMiyunConnection(ctx, value)
	}
	if err != nil {
		return MiyunConnection{}, err
	}
	if request.ExpectedVersion != current.Version {
		return MiyunConnection{}, ErrVersionConflict
	}
	current.SessionCiphertext, current.SessionKeyVersion, current.SessionExpiresAt = ciphertext, keyVersion, request.SessionExpiresAt
	current.Status, current.CooldownUntil = MiyunConnectionUnverified, nil
	current.LastErrorKind, current.LastErrorCode, current.LastErrorAt, current.UpdatedAt = "", "", nil, now
	return s.Miyun.UpdateMiyunConnection(ctx, current, request.ExpectedVersion)
}

func (s Service) VerifyMiyunConnection(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request VerifyMiyunConnectionRequest) (MiyunConnection, error) {
	if err := s.miyunReady(actor, projectID, ScopeWrite); err != nil {
		return MiyunConnection{}, err
	}
	if s.MiyunSecrets == nil || s.MiyunVerifier == nil || request.ExpectedVersion < 1 {
		return MiyunConnection{}, ErrInvalidState
	}
	current, err := s.Miyun.GetProjectMiyunConnection(ctx, actor.OrganizationID, projectID)
	if err != nil {
		return MiyunConnection{}, err
	}
	if current.Version != request.ExpectedVersion {
		return MiyunConnection{}, ErrVersionConflict
	}
	plaintext, err := s.MiyunSecrets.Decrypt(current.SessionCiphertext, current.SessionKeyVersion)
	if err != nil {
		return MiyunConnection{}, err
	}
	verifyErr := s.MiyunVerifier.VerifyMiyunConnection(ctx, plaintext)
	for index := range plaintext {
		plaintext[index] = 0
	}
	now := s.now()
	current.LastVerifiedAt, current.UpdatedAt = &now, now
	if verifyErr == nil {
		current.Status, current.CooldownUntil = MiyunConnectionReady, nil
		current.LastSuccessfulRequestAt = &now
		current.LastErrorKind, current.LastErrorCode, current.LastErrorAt = "", "", nil
		return s.Miyun.UpdateMiyunConnection(ctx, current, request.ExpectedVersion)
	}
	var upstream *crawler.YouShuError
	if errors.As(verifyErr, &upstream) {
		current.LastErrorKind, current.LastErrorCode, current.LastErrorAt = string(upstream.Kind), upstream.Code, &now
		if current.LastErrorCode == "" {
			current.LastErrorCode = strings.ToUpper(string(upstream.Kind))
		}
		if upstream.Kind == crawler.YouShuAuthRequired {
			current.Status = MiyunConnectionAuthRequired
		} else if upstream.Kind == crawler.YouShuRateLimited {
			until := now.Add(s.miyunCooldown())
			current.CooldownUntil = &until
		}
		updated, updateErr := s.Miyun.UpdateMiyunConnection(ctx, current, request.ExpectedVersion)
		if updateErr != nil {
			return MiyunConnection{}, updateErr
		}
		return updated, verifyErr
	}
	return MiyunConnection{}, verifyErr
}
