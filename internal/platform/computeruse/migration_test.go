package computeruse

import (
	"os"
	"strings"
	"testing"
)

func TestControlPlaneMigrationFreezesSafetyPrimitives(t *testing.T) {
	payload, err := os.ReadFile("../../../migrations/platform/20260812130000_platform_computer_use_control_plane.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(payload)
	for _, required := range []string{"computer_use_session_leases", "active_lock_key", "fencing_token", "computer_use_kill_switches", "computer_use_final_confirmations", "token_digest", "computer_use_controlled_action_attempts", "uq_computer_use_attempt_confirmation", "result_unknown"} {
		if !strings.Contains(sql, required) {
			t.Errorf("migration omitted %q", required)
		}
	}
	if strings.Contains(strings.ToLower(sql), "remote_write_enabled") {
		t.Fatal("Platform control-plane migration must not alter Delivery remote-write authority")
	}
}
