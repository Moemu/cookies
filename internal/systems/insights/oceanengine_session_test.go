package insights

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type oceanEngineSessionRepositoryStub struct {
	value   OceanEngineSession
	getErr  error
	created bool
}

func (r *oceanEngineSessionRepositoryStub) CreateOceanEngineSession(_ context.Context, value OceanEngineSession) (OceanEngineSession, error) {
	r.value, r.created = value, true
	return value, nil
}
func (r *oceanEngineSessionRepositoryStub) GetProjectOceanEngineSession(_ context.Context, org contract.OrganizationID, project contract.ProjectID) (OceanEngineSession, error) {
	if r.getErr != nil {
		return OceanEngineSession{}, r.getErr
	}
	if r.value.OrganizationID != org || r.value.ProjectID != project || r.value.ID == "" {
		return OceanEngineSession{}, ErrNotFound
	}
	return r.value, nil
}
func (r *oceanEngineSessionRepositoryStub) UpdateOceanEngineSession(_ context.Context, value OceanEngineSession, expected int64) (OceanEngineSession, error) {
	if r.value.Version != expected {
		return OceanEngineSession{}, ErrVersionConflict
	}
	value.Version = expected + 1
	r.value = value
	return value, nil
}

type oceanEngineSessionCipherStub struct{}

func (oceanEngineSessionCipherStub) Encrypt(value []byte) ([]byte, string, error) {
	return append([]byte("cipher:"), value...), "key-v1", nil
}
func (oceanEngineSessionCipherStub) Decrypt(value []byte, _ string) ([]byte, error) {
	return append([]byte(nil), value[len("cipher:"):]...), nil
}

type oceanEngineSessionVerifierStub struct {
	err   error
	calls int
}

func (v *oceanEngineSessionVerifierStub) VerifyOceanEngineSession(context.Context, []byte) error {
	v.calls++
	return v.err
}

func oceanEngineSessionTestService(repo *oceanEngineSessionRepositoryStub, verifier *oceanEngineSessionVerifierStub) Service {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	return Service{OceanEngineSessions: repo, OceanEngineVerifier: verifier, SessionSecrets: oceanEngineSessionCipherStub{}, Now: func() time.Time { return now }, NewID: func(prefix string) (string, error) { return prefix + "_1", nil }}
}

func oceanEngineSessionActor() contract.ActorContext {
	return contract.ActorContext{OrganizationID: "org_1", Principal: contract.Principal{Kind: contract.PrincipalUser, ID: "operator_1"}, Scopes: []contract.Scope{ScopeRead, ScopeWrite}}
}

func TestOceanEngineSessionUpdateDoesNotReturnPlaintext(t *testing.T) {
	repo := &oceanEngineSessionRepositoryStub{}
	service := oceanEngineSessionTestService(repo, &oceanEngineSessionVerifierStub{})
	value, err := service.UpdateOceanEngineSession(context.Background(), oceanEngineSessionActor(), "project_1", UpdateOceanEngineSessionRequest{Session: "synthetic-cookie-value"})
	if err != nil {
		t.Fatal(err)
	}
	if !repo.created || !value.CredentialRefPresent || value.Status != OceanEngineSessionUnverified {
		t.Fatalf("saved session=%#v", value)
	}
	if string(value.SessionCiphertext) == "synthetic-cookie-value" || value.SessionKeyVersion == "" {
		t.Fatal("session was not protected")
	}
	if encoded := string(mustJSON(t, value)); containsSecret(encoded, "synthetic-cookie-value") {
		t.Fatal("response contains plaintext session")
	}
}

func TestOceanEngineSessionVerifyPersistsSafeFailure(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	repo := &oceanEngineSessionRepositoryStub{value: OceanEngineSession{ID: "oceanenginesession_1", OrganizationID: "org_1", ProjectID: "project_1", Status: OceanEngineSessionUnverified, CredentialRefPresent: true, SessionCiphertext: []byte("cipher:synthetic-cookie-value"), SessionKeyVersion: "key-v1", Version: 1, CreatedBy: "operator_1", CreatedAt: now, UpdatedAt: now}}
	verifier := &oceanEngineSessionVerifierStub{err: errors.New("upstream response contained sensitive details")}
	service := oceanEngineSessionTestService(repo, verifier)
	value, err := service.VerifyOceanEngineSession(context.Background(), oceanEngineSessionActor(), "project_1", VerifyOceanEngineSessionRequest{ExpectedVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if verifier.calls != 1 || value.Status != OceanEngineSessionAuthRequired || value.LastErrorCode != "OCEAN_ENGINE_VERIFY_FAILED" {
		t.Fatalf("verification state=%#v calls=%d", value, verifier.calls)
	}
	if value.LastErrorKind != "verification_failed" {
		t.Fatalf("error kind=%q", value.LastErrorKind)
	}
}

func mustJSON(t *testing.T, value OceanEngineSession) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
func containsSecret(value, secret string) bool {
	return len(secret) > 0 && strings.Contains(value, secret)
}
