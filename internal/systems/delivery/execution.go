package delivery

import (
	"context"
	"fmt"
)

const MockOceanEngineAdapter = "mock_ocean_engine"

// PlatformAdapter is intentionally small so a real platform implementation can
// share the state model without changing the controlled execution contract.
type PlatformAdapter interface {
	Source() Source
	ExecuteStep(context.Context, PlatformStepRequest) (PlatformStepResult, error)
}

type PlatformStepRequest struct {
	ExecutionID string
	Scenario    ExecutionScenario
	Action      string
	Sequence    int
}
type PlatformStepResult struct {
	Status                       StepStatus
	Effect, Summary, EvidenceRef string
}

type DeterministicMockAdapter struct{}

func (DeterministicMockAdapter) Source() Source { return SourceMock }
func (DeterministicMockAdapter) ExecuteStep(_ context.Context, request PlatformStepRequest) (PlatformStepResult, error) {
	result := PlatformStepResult{Status: StepSucceeded, Effect: "confirmed_applied"}
	switch request.Action {
	case "create_platform_project":
		result.Summary, result.EvidenceRef = "approval and payload verified", "mock://execution/project"
		if request.Scenario == ExecutionScenarioFailed {
			result.Status, result.Effect = StepFailed, "confirmed_not_applied"
			result.Summary, result.EvidenceRef = "fixture rejected before any target effect", "mock://execution/rejected"
		}
	case "create_promotion":
		result.Summary, result.EvidenceRef = "fixture delivery created", "mock://execution/promotion"
		if request.Scenario == ExecutionScenarioPartial {
			result.Status, result.Effect = StepFailed, "confirmed_not_applied"
			result.Summary, result.EvidenceRef = "remaining fixture operation did not complete", "mock://execution/partial"
		}
		if request.Scenario == ExecutionScenarioResultUnknown {
			result.Status, result.Effect = StepResultUnknown, "unknown"
			result.Summary, result.EvidenceRef = "fixture response was interrupted; effect cannot be verified", "mock://execution/unknown"
		}
	case "verify_platform_state":
		result.Summary, result.EvidenceRef = "fixture result verified", "mock://execution/verification"
		if request.Scenario == ExecutionScenarioPartial {
			result.Summary = "completed scope verified"
		}
		if request.Scenario == ExecutionScenarioResultUnknown {
			result.Status, result.Effect = StepResultUnknown, "unknown"
			result.Summary, result.EvidenceRef = "query or reconcile before any retry", "mock://execution/reconcile"
		}
	default:
		return PlatformStepResult{}, fmt.Errorf("%w: unknown step %q", ErrInvalidRequest, request.Action)
	}
	return result, nil
}

func (s Service) platformAdapter() PlatformAdapter {
	if s.Adapter != nil {
		return s.Adapter
	}
	return DeterministicMockAdapter{}
}

func executionOutcome(scenario ExecutionScenario) (ExecutionStatus, string, string, []string) {
	switch scenario {
	case ExecutionScenarioSuccess:
		return ExecutionSucceeded, "none", "", []string{}
	case ExecutionScenarioFailed:
		return ExecutionFailed, "create_new_change_set", "no target effect was produced", []string{}
	case ExecutionScenarioPartial:
		return ExecutionPartial, "review_and_compensate", "some target effects completed; compensation requires a new controlled action", []string{"remove_mock_delivery"}
	case ExecutionScenarioResultUnknown:
		return ExecutionResultUnknown, "query_and_reconcile", "result is unknown; blind retry is prohibited", []string{}
	default:
		return ExecutionResultUnknown, "query_and_reconcile", "adapter result is unknown; blind retry is prohibited", []string{}
	}
}

func validExecutionTransition(from, to ExecutionStatus) bool {
	switch from {
	case ExecutionQueued:
		return to == ExecutionValidatingApproval || to == ExecutionCancelled
	case ExecutionValidatingApproval:
		return to == ExecutionExecuting || to == ExecutionCancelled
	case ExecutionExecuting:
		return to == ExecutionVerifying || to == ExecutionResultUnknown
	case ExecutionVerifying:
		return to == ExecutionSucceeded || to == ExecutionFailed || to == ExecutionPartial || to == ExecutionResultUnknown
	default:
		return false
	}
}

func validStepTransition(from, to StepStatus) bool {
	return (from == StepPending && (to == StepRunning || to == StepSkipped)) || (from == StepRunning && (to == StepSucceeded || to == StepFailed || to == StepResultUnknown))
}
