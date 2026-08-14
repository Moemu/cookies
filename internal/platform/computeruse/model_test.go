package computeruse

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestRunStateTransitionsFreezeSensitiveBoundary(t *testing.T) {
	allowed := [][2]RunState{{RunQueued, RunEnvironmentCheck}, {RunPreparing, RunAwaitingConfirmation}, {RunAwaitingConfirmation, RunSubmitting}, {RunSubmitting, RunVerifying}, {RunVerifying, RunSucceeded}}
	for _, transition := range allowed {
		if !CanTransition(transition[0], transition[1]) {
			t.Fatalf("expected %s -> %s to be allowed", transition[0], transition[1])
		}
	}
	for _, transition := range [][2]RunState{{RunQueued, RunSubmitting}, {RunPreparing, RunSubmitting}, {RunResultUnknown, RunSubmitting}, {RunSucceeded, RunPreparing}} {
		if CanTransition(transition[0], transition[1]) {
			t.Fatalf("unsafe transition %s -> %s was allowed", transition[0], transition[1])
		}
	}
}

func TestSessionLeaseRequiresLiveExclusiveFence(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	lease := SessionLease{ID: "lease_1", RunID: "run_1", FencingToken: 4, Version: 1, ExpiresAt: now.Add(time.Minute), HeartbeatDeadline: now.Add(30 * time.Second)}
	if !lease.ValidAt(now) {
		t.Fatal("expected live lease")
	}
	released := now
	lease.ReleasedAt = &released
	if lease.ValidAt(now) {
		t.Fatal("released lease remained valid")
	}
}

func TestSitePolicyRequiresExactHTTPSHostPageAndProject(t *testing.T) {
	policy := SitePolicy{AllowedProtocols: []string{"https"}, AllowedHosts: []string{"business.oceanengine.com"}, AllowedPageKinds: []string{"project_create"}, AllowedPlatformProjects: []string{"project_test_1"}}
	if !policy.Allows("https://business.oceanengine.com/project/create", "project_create", "project_test_1") {
		t.Fatal("expected exact allowlist match")
	}
	for _, rawURL := range []string{"http://business.oceanengine.com/project/create", "https://evil.example/project/create", "https://business.oceanengine.com.evil.example/project/create"} {
		if policy.Allows(rawURL, "project_create", "project_test_1") {
			t.Fatalf("unexpected allowlist match for %s", rawURL)
		}
	}
}

func TestFinalConfirmationCannotBeReplayedOrUsedAfterExpiry(t *testing.T) {
	now := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	confirmation := FinalConfirmation{SchemaVersion: ConfirmationSchemaV1, ID: "confirmation_1", RunID: "run_1", BindingHash: hash64(), TokenDigest: hash64(), IssuedAt: now, ExpiresAt: now.Add(time.Minute), Version: 1}
	if !confirmation.UsableAt(now) {
		t.Fatal("fresh confirmation should be usable")
	}
	confirmation.ConsumedAt = &now
	if confirmation.UsableAt(now) {
		t.Fatal("consumed confirmation must not be reusable")
	}
	confirmation.ConsumedAt = nil
	if confirmation.UsableAt(now.Add(2 * time.Minute)) {
		t.Fatal("expired confirmation must not be usable")
	}
}

func TestAuthorityRejectsUnknownPromotionMutationAction(t *testing.T) {
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	authority := validRun(now).Authority
	authority.Action = "update_promotion_schedule"
	if err := authority.Validate(); err != ErrInvalidContract {
		t.Fatalf("project-owned schedule action passed the promotion authority contract: %v", err)
	}
}

func TestPlatformOpenAPIExposesControlledRunWithoutCompileTimeBlockReason(t *testing.T) {
	contents, err := os.ReadFile("../../../api/openapi/platform-v1.yaml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(contents)
	for _, fragment := range []string{"createComputerUseRun", "issueComputerUseFinalConfirmation", "ComputerUseAuthorityBinding:", "ComputerUsePromotionControl:", "ComputerUsePromotionRestart:", "FINAL_CONFIRMATION_REQUIRED", "RESULT_RECONCILIATION_REQUIRED"} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("platform OpenAPI missing %q", fragment)
		}
	}
	start := strings.Index(text, "ComputerUseBlockingReason:")
	if start < 0 {
		t.Fatal("could not locate Computer Use blocking-reason contract")
	}
	end := strings.Index(text[start:], "ComputerUseRunState:")
	if end < 0 {
		t.Fatal("could not locate end of Computer Use blocking-reason contract")
	}
	if strings.Contains(text[start:start+end], "PHASE_C_REMOTE_WRITE_PROHIBITED") {
		t.Fatal("run-time blocking reasons reused the Phase C compile-time prohibition")
	}
	if strings.Contains(text, "update_promotion_schedule") {
		t.Fatal("platform OpenAPI assigns the parent-project schedule to a promotion authority")
	}
}

func hash64() string { return "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" }
