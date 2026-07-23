package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDispatchPayloadUsesCanonicalJSONObject(t *testing.T) {
	t.Parallel()
	payload, hash, err := dispatchPayload("agenttask_1")
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(payload) || len(hash) != 64 || strings.Trim(hash, "0123456789abcdef") != "" {
		t.Fatalf("payload=%s hash=%q", payload, hash)
	}
	var decoded struct {
		AgentTaskID string `json:"agent_task_id"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil || decoded.AgentTaskID != "agenttask_1" {
		t.Fatalf("decoded=%#v err=%v", decoded, err)
	}
}
