package connector

import (
	"context"
	"errors"
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
func (s *accountStoreStub) ClaimOrganizationAccount(_ context.Context, organizationID, projectID, accountID string, _ time.Time) (PlatformAccount, error) {
	return PlatformAccount{ID: accountID, OrganizationID: organizationID, ProjectID: projectID}, nil
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

type accountProbeStub struct {
	external string
	version  int64
	err      error
}
type accountSessionStateStub struct{}

func (accountSessionStateStub) MarkAccountSessionVerified(context.Context, string, string, int64, time.Time) (OceanEngineAccountSession, error) {
	return OceanEngineAccountSession{}, nil
}

func (p *accountProbeStub) Verify(_ context.Context, _, _, _, external string) (int64, error) {
	p.external = external
	return p.version, p.err
}
func TestAccountServiceKeepsExternalIDInsideVerificationBoundary(t *testing.T) {
	store := &accountStoreStub{}
	probe := &accountProbeStub{}
	service := AccountService{Store: store, Probe: probe, Sessions: accountSessionStateStub{}, Now: func() time.Time { return baseTime }}
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

func TestAccountServiceClassifiesProbeFailureWithoutClaimingVersionConflict(t *testing.T) {
	service := AccountService{Store: &accountStoreStub{}, Probe: &accountProbeStub{err: errors.New("upstream failed")}, Sessions: accountSessionStateStub{}}
	_, err := service.Verify(context.Background(), "org_1", "", "oeacct_safe")
	if !errors.Is(err, ErrAccountVerificationUnavailable) || errors.Is(err, ErrImmutableConflict) {
		t.Fatalf("error=%v", err)
	}

	service.Probe = &accountProbeStub{err: ErrAccountSessionInvalid}
	_, err = service.Verify(context.Background(), "org_1", "", "oeacct_safe")
	if !errors.Is(err, ErrAccountSessionInvalid) {
		t.Fatalf("error=%v", err)
	}
}

func TestAccountServiceClaimsLegacyAccountForProject(t *testing.T) {
	service := AccountService{Store: &accountStoreStub{}, Now: func() time.Time { return baseTime }}
	account, err := service.Claim(context.Background(), "org_1", "project_1", "oeacct_safe")
	if err != nil {
		t.Fatal(err)
	}
	if account.ID != "oeacct_safe" || account.ProjectID != "project_1" {
		t.Fatalf("claimed account=%#v", account)
	}
	if _, err = service.Claim(context.Background(), "org_1", "", "oeacct_safe"); !errors.Is(err, ErrInvalidFact) {
		t.Fatalf("empty project must fail, got %v", err)
	}
}
