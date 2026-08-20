package connector

import (
	"context"
	"strings"
	"testing"
	"time"
)

type accountStoreStub struct {
	request  RegisterAccountRequest
	verified string
}

func (s *accountStoreStub) RegisterAccount(_ context.Context, r RegisterAccountRequest) (PlatformAccount, error) {
	s.request = r
	return PlatformAccount{ID: "oeacct_safe", OrganizationID: r.OrganizationID, ProjectID: r.ProjectID, Platform: SourceSystem, Status: "pending", CredentialRefPresent: true}, nil
}
func (s *accountStoreStub) ListAccounts(context.Context, string, string) ([]PlatformAccount, error) {
	return []PlatformAccount{{ID: "oeacct_safe"}}, nil
}
func (s *accountStoreStub) MarkAccountVerified(_ context.Context, _, _, id string, _ time.Time) (PlatformAccount, error) {
	s.verified = id
	return PlatformAccount{ID: id, Status: "verified"}, nil
}
func (s *accountStoreStub) ResolveAnyExternalAccountID(context.Context, string, string, string) (string, error) {
	return "raw-platform-id", nil
}
func (s *accountStoreStub) RevokeAccount(context.Context, string, string, string, time.Time) (PlatformAccount, error) {
	return PlatformAccount{ID: "oeacct_safe", Status: "revoked"}, nil
}

type accountProbeStub struct{ external string }

func (p *accountProbeStub) Verify(_ context.Context, _, _, external string) error {
	p.external = external
	return nil
}
func TestAccountServiceKeepsExternalIDInsideVerificationBoundary(t *testing.T) {
	store := &accountStoreStub{}
	probe := &accountProbeStub{}
	service := AccountService{Store: store, Probe: probe, Now: func() time.Time { return baseTime }}
	account, err := service.Register(context.Background(), RegisterAccountRequest{OrganizationID: "org_1", ProjectID: "project_1", ExternalID: "raw-platform-id", CredentialRef: "insights-session://compat"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(account.ID, "raw-platform-id") {
		t.Fatal("local account ID exposes platform identity")
	}
	verified, err := service.Verify(context.Background(), "org_1", "project_1", account.ID)
	if err != nil || verified.Status != "verified" || probe.external != "raw-platform-id" {
		t.Fatalf("verified=%#v external=%s error=%v", verified, probe.external, err)
	}
}
