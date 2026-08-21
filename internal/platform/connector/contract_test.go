package connector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPointInTimeContractRequiresLineageTimeUnitAndEvidence(t *testing.T) {
	path := filepath.Join("..", "..", "..", "docs", "connector", "oceanengine-point-in-time-v1.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	defs := schema["$defs"].(map[string]any)
	fact := defs["fact"].(map[string]any)
	requiredValues := fact["required"].([]any)
	required := map[string]bool{}
	for _, value := range requiredValues {
		required[value.(string)] = true
	}
	for _, key := range []string{"organization_id", "source_system", "source_ref", "ingest_run_id", "schema_version", "payload_hash", "collected_at", "available_at", "data_through", "valid_from", "quality_status", "evidence_ref"} {
		if !required[key] {
			t.Fatalf("contract does not require %s", key)
		}
	}
	properties := fact["properties"].(map[string]any)
	if _, ok := properties["project_id"]; !ok || required["project_id"] {
		t.Fatal("legacy project_id must be optional for Organization-scoped facts")
	}
	for _, key := range []string{"currency", "amount_unit", "metric_definition_version", "window_start", "window_end", "granularity", "time_zone", "attribution_window"} {
		if _, ok := properties[key]; !ok {
			t.Fatalf("contract does not define %s", key)
		}
	}
	prelaunch := properties["eligible_as_prelaunch_feature"].(map[string]any)
	if value, ok := prelaunch["const"].(bool); !ok || value {
		t.Fatal("diagnosis prelaunch guard is not false")
	}
}

func TestConnectorOpenAPIDoesNotExposeRawPayload(t *testing.T) {
	path := filepath.Join("..", "..", "..", "api", "openapi", "connector-v1.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(data))
	if strings.Contains(text, "            raw:") || strings.Contains(text, "cookie:") || strings.Contains(text, "token:") || strings.Contains(text, "csrf:") {
		t.Fatal("ordinary Connector API exposes raw or credential material")
	}
	if !strings.Contains(text, "prediction_cutoff") || !strings.Contains(text, DatasetVersion) {
		t.Fatal("point-in-time route is incomplete")
	}
	if !strings.Contains(text, "/api/connector/v1/accounts:") || !strings.Contains(text, "writeonly: true") {
		t.Fatal("Organization account API or credential write boundary is missing")
	}
}
