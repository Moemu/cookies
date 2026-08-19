package rparunner

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func testRunner(t *testing.T, mode string) Runner {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", "fake-runner.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	return Runner{
		Command:        []string{"node", abs},
		ScriptPath:     mode,
		PrepareTimeout: 30 * time.Second,
		SubmitTimeout:  30 * time.Second,
	}
}

func basePlan(mode string) RpaPlan {
	return RpaPlan{
		SchemaVersion: PlanSchemaV2,
		Browser:       "msedge",
		Mode:          mode,
		AccountID:     "account_test",
		Steps: []RpaStep{{
			ID:       "identify_account_and_object",
			Kind:     "identify_page",
			PageKind: "promotion_list",
		}},
	}
}

func TestRunnerRoundTripsPlanOverStdinAndParsesResult(t *testing.T) {
	runner := testRunner(t, "success")
	result, err := runner.Run(context.Background(), basePlan("prepare"))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if result.SchemaVersion != ResultSchemaV1 || result.Outcome != OutcomeSuccess || result.ErrorCode != CodeOK {
		t.Fatalf("unexpected result %+v", result)
	}
	if len(result.Steps) != 1 || result.Steps[0].Readback["object_id"] != "promotion_test" {
		t.Fatalf("steps not forwarded: %+v", result.Steps)
	}
	if result.Steps[0].Readback["plan_mode"] != "prepare" {
		t.Fatalf("plan was not delivered over stdin: %+v", result.Steps[0].Readback)
	}
}

func TestRunnerReportsBusinessFailuresWithoutInfrastructureError(t *testing.T) {
	runner := testRunner(t, "page-drift")
	result, err := runner.Run(context.Background(), basePlan("prepare"))
	if err != nil {
		t.Fatalf("business failure must not be an infrastructure error: %v", err)
	}
	if result.Outcome != OutcomeFailed || result.ErrorCode != CodePageDrift {
		t.Fatalf("unexpected result %+v", result)
	}
}

func TestRunnerTreatsUnparseableOutputAsInfrastructureFailure(t *testing.T) {
	runner := testRunner(t, "garbage")
	_, err := runner.Run(context.Background(), basePlan("prepare"))
	if !errors.Is(err, ErrRunnerInfrastructure) {
		t.Fatalf("expected infrastructure failure, got %v", err)
	}
}

func TestRunnerRejectsUnknownResultSchema(t *testing.T) {
	runner := testRunner(t, "wrong-schema")
	_, err := runner.Run(context.Background(), basePlan("prepare"))
	if !errors.Is(err, ErrRunnerInfrastructure) {
		t.Fatalf("expected infrastructure failure for unknown schema, got %v", err)
	}
}

func TestRunnerMissingCommandIsInfrastructureFailure(t *testing.T) {
	runner := Runner{ScriptPath: "x"}
	_, err := runner.Run(context.Background(), basePlan("prepare"))
	if !errors.Is(err, ErrRunnerInfrastructure) {
		t.Fatalf("expected infrastructure failure, got %v", err)
	}
}
