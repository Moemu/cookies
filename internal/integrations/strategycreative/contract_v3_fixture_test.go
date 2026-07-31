package strategycreative

import (
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestFrozenV3FixtureLineage(t *testing.T) {
	t.Parallel()

	intake := readFixture(t, "creative-intake-v3-ready.json")
	handoff, ok := intake["base_handoff"].(map[string]any)
	if !ok {
		t.Fatal("v3 intake base_handoff is missing")
	}
	assertHandoffHash(t, handoff)
	assertSameJSONValue(t, "v3 intake strategy_package_ref", intake["strategy_package_ref"], intakePackageRef(t, handoff))

	selectedRouteID := requiredString(t, intake, "selected_route_id")
	assertRouteExists(t, handoff, selectedRouteID)
	assertReadyIntakeState(t, intake)

	plan := readFixture(t, "strategy-creative-task-plan-v2-ready.json")
	strategy := readFixture(t, "strategy-creative-task-strategy-v2-ready.json")
	overlay := readFixture(t, "strategy-creative-task-overlay-v1-ready.json")
	create := readFixture(t, "creative-intake-create-v3-overlay.json")
	planning := readFixture(t, "creative-planning-context-v1-enhanced.json")
	direction := readFixture(t, "creative-direction-v1-candidate.json")
	batch := readFixture(t, "creative-direction-candidate-batch-v1-ready.json")
	assertDirectionHash(t, direction)

	for name, value := range map[string]map[string]any{
		"plan": plan, "strategy": strategy, "overlay": overlay, "create": create,
	} {
		if requiredString(t, value, "selected_route_id") != "route_xhs_image_text" {
			t.Fatalf("%s fixture does not preserve the selected route", name)
		}
	}
	assertSameJSONValue(t, "plan package_ref", plan["package_ref"], overlay["package_ref"])
	assertSameJSONValue(t, "strategy package_ref", strategy["package_ref"], overlay["package_ref"])
	assertSameJSONValue(t, "create package_ref", create["strategy_package_ref"], overlay["package_ref"])

	identityHash := requiredString(t, planning, "input_identity_hash")
	if requiredString(t, direction, "input_identity_hash") != identityHash ||
		requiredString(t, batch, "input_identity_hash") != identityHash {
		t.Fatal("planning context, direction, and candidate batch identity hashes differ")
	}
	if requiredString(t, direction, "batch_id") != requiredString(t, batch, "batch_id") {
		t.Fatal("direction does not belong to the frozen candidate batch")
	}
	candidates, ok := batch["candidates"].([]any)
	if !ok || len(candidates) != 1 {
		t.Fatal("candidate batch must contain its frozen direction")
	}
	assertSameJSONValue(t, "batch candidate", candidates[0], direction)
}

func assertDirectionHash(t *testing.T, value map[string]any) {
	t.Helper()
	expected := requiredString(t, value, "content_hash")
	hashInput := cloneJSONMap(t, value)
	hashInput["content_hash"] = ""
	actual, err := contract.NewContentHash(hashInput)
	if err != nil {
		t.Fatalf("canonicalize direction: %v", err)
	}
	if string(actual) != expected {
		t.Fatalf("direction content_hash mismatch: fixture=%s calculated=%s", expected, actual)
	}
}
