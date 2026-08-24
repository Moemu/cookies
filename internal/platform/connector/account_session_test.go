package connector

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

type accountSessionCipherStub struct{}

func (accountSessionCipherStub) Encrypt(value []byte) ([]byte, string, error) {
	result := append([]byte("cipher:"), value...)
	return result, "key-v1", nil
}
func (accountSessionCipherStub) Decrypt(value []byte, _ string) ([]byte, error) {
	return append([]byte(nil), value[len("cipher:"):]...), nil
}

type accountSessionStoreStub struct {
	value           OceanEngineAccountSession
	expectedVersion int64
}

func (s *accountSessionStoreStub) GetAccountSession(context.Context, string, string) (OceanEngineAccountSession, error) {
	return s.value, nil
}
func (s *accountSessionStoreStub) PutAccountSession(_ context.Context, value OceanEngineAccountSession, expectedVersion int64) (OceanEngineAccountSession, error) {
	s.expectedVersion = expectedVersion
	s.value = value
	return value, nil
}
func (s *accountSessionStoreStub) MarkAccountSessionVerified(_ context.Context, _, _ string, expectedVersion int64, now time.Time) (OceanEngineAccountSession, error) {
	s.expectedVersion = expectedVersion
	s.value.Status, s.value.LastVerifiedAt, s.value.UpdatedAt = AccountSessionReady, &now, now
	return s.value, nil
}

func TestAccountSessionServiceEncryptsAndReturnsOnlySafeMetadata(t *testing.T) {
	store := &accountSessionStoreStub{}
	service := AccountSessionService{Store: store, Cipher: accountSessionCipherStub{}, Now: func() time.Time { return baseTime }}
	value, err := service.Update(context.Background(), "org_1", "oeacct_safe", []byte("synthetic-cookie"), 0)
	if err != nil {
		t.Fatal(err)
	}
	if string(store.value.SessionCiphertext) != "cipher:synthetic-cookie" || store.value.SessionKeyVersion != "key-v1" {
		t.Fatalf("session was not encrypted: %#v", store.value)
	}
	encoded, _ := json.Marshal(value)
	if string(encoded) == "" || containsSensitiveText(string(encoded)) || string(encoded) == "synthetic-cookie" {
		t.Fatalf("unsafe session response %s", encoded)
	}
}

func TestOrganizationAccountVerificationUsesMatchingSessionVersion(t *testing.T) {
	accounts := &accountStoreStub{}
	sessions := &accountSessionStoreStub{value: OceanEngineAccountSession{Version: 4}}
	probe := &accountProbeStub{version: 4}
	service := AccountService{Store: accounts, Probe: probe, Sessions: sessions, Now: func() time.Time { return baseTime }}
	value, err := service.Verify(context.Background(), "org_1", "", "oeacct_safe")
	if err != nil {
		t.Fatal(err)
	}
	if value.Status != "verified" || sessions.expectedVersion != 4 || sessions.value.Status != AccountSessionReady {
		t.Fatalf("account=%#v session=%#v", value, sessions.value)
	}
}
