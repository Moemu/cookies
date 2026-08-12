package delivery

import (
	"os"
	"strings"
	"testing"
)

func TestControlledAuthorityMigrationPreservesLegacyMockAuthority(t *testing.T) {
	payload, err := os.ReadFile("../../../migrations/delivery/20260812131000_delivery_controlled_execution_authority.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(payload)
	for _, required := range []string{"delivery_controlled_change_sets", "delivery_remote_write_approvals", "controlled_remote_write", "delivery_controlled_executions", "computer_use_run_id", "delivery_platform_entity_mappings", "result_evidence_id", "list_evidence_id"} {
		if !strings.Contains(sql, required) {
			t.Errorf("migration omitted %q", required)
		}
	}
	lower := strings.ToLower(sql)
	for _, forbidden := range []string{"update delivery_approvals", "drop table delivery_approvals", "drop constraint chk_delivery_approval", "execute_mock'"} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("migration changes legacy approval authority with %q", forbidden)
		}
	}
}

func TestControlledObjectFingerprintMigrationPreventsDuplicateTargets(t *testing.T) {
	payload, err := os.ReadFile("../../../migrations/delivery/20260812132000_delivery_controlled_object_fingerprint.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(payload))
	for _, required := range []string{"generated always", "object_fingerprint", "json_extract", "unique key", "organization_id, project_id, object_fingerprint", "distinct_evidence", "result_evidence_id <> list_evidence_id"} {
		if !strings.Contains(sql, required) {
			t.Errorf("object fingerprint migration omitted %q", required)
		}
	}
	for _, forbidden := range []string{"update delivery_approvals", "execute_mock", "drop table"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("object fingerprint migration changes legacy authority with %q", forbidden)
		}
	}
}
