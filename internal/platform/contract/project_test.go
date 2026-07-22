package contract

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProjectRefUsesProjectContextVersion(t *testing.T) {
	t.Parallel()
	ref := ProjectRef{OrganizationID: "org_1", ProjectID: "project_1", ProjectContextVersion: 7}
	if err := ref.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDraftProjectContextSerializesNullBrandAndFrozenVersionName(t *testing.T) {
	t.Parallel()
	context := ProjectContext{OrganizationID: "org_1", ProjectID: "project_1", ProductIDs: []ProductID{}, ProjectContextVersion: 7}
	if err := context.Validate(); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(context)
	if err != nil {
		t.Fatal(err)
	}
	jsonText := string(payload)
	if !strings.Contains(jsonText, `"brand_id":null`) || !strings.Contains(jsonText, `"project_context_version":7`) || strings.Contains(jsonText, `"version":`) {
		t.Fatalf("unexpected ProjectContext JSON: %s", payload)
	}
}
