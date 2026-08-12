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

// AES-GCM adds a nonce and authentication tag to the stored value. Keep the
// plaintext below the 16 KiB database envelope so a complete Cookie header is
// not silently truncated before it can be verified.
const maxMiyunSessionBytes = 16*1024 - 28

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
	if s.MiyunSecrets == nil {
		return MiyunConnection{}, fmt.Errorf("%w: 米云会话加密尚未在服务端启用，请联系管理员配置 COOKIES_MIYUN_ENABLED 和会话加密密钥", ErrInvalidState)
	}
	if len(request.Session) < 8 {
		return MiyunConnection{}, fmt.Errorf("%w: Cookie 值不完整，请从请求标头中复制完整的 Cookie 值", ErrInvalidRequest)
	}
	if len(request.Session) > maxMiyunSessionBytes {
		return MiyunConnection{}, fmt.Errorf("%w: Cookie 值超过 16 KiB，无法安全保存；请确认只复制 Cookie 的值", ErrInvalidRequest)
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
	// 服务端没启用米云和「传了个不合法的版本号」是两回事，但原来都回同一个裸
	// ErrInvalidState。前者人怎么重试都没用，得说清楚是环境没配，跟保存会话那条
	// 保持同一句话。
	if s.MiyunSecrets == nil || s.MiyunVerifier == nil {
		return MiyunConnection{}, fmt.Errorf("%w: 米云会话加密尚未在服务端启用，请联系管理员配置 COOKIES_MIYUN_ENABLED 和会话加密密钥", ErrInvalidState)
	}
	if request.ExpectedVersion < 1 {
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
		} else {
			// A non-authentication failure must not preserve a stale
			// auth_required state from an earlier verification attempt.
			if current.Status == MiyunConnectionAuthRequired {
				current.Status = MiyunConnectionUnverified
			}
			if upstream.Kind == crawler.YouShuRateLimited {
				until := now.Add(s.miyunCooldown())
				current.CooldownUntil = &until
			}
		}
	} else {
		// Never expose an unclassified upstream error (it can include transport
		// details). Persist a safe outcome so the API can still return the
		// authoritative connection state instead of an opaque 500.
		current.LastErrorKind, current.LastErrorCode, current.LastErrorAt = string(crawler.YouShuTransport), "UNCLASSIFIED", &now
	}
	updated, updateErr := s.Miyun.UpdateMiyunConnection(ctx, current, request.ExpectedVersion)
	if updateErr != nil {
		return MiyunConnection{}, updateErr
	}
	return updated, nil
}
