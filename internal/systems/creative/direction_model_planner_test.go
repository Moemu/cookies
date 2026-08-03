package creative

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

func TestModelCreativeDirectionPlannerRepairsUnsafeProviderOutput(t *testing.T) {
	generator := &directionModelTextStub{responses: []provider.SynchronousResponse{
		creativeDirectionModelResponse(t, []DirectionCandidate{
			{Concept: "永久清爽", CreativeRationale: "突出通勤场景", MessagePlan: []string{"先场景后产品"}, ExecutionOutline: []string{"通勤实拍"}, GuardrailTrace: []string{"避免绝对化表达"}},
			{Concept: "青柠通勤记录", CreativeRationale: "记录产品信息", MessagePlan: []string{"先产品后行动"}, ExecutionOutline: []string{"桌面静物"}, GuardrailTrace: []string{"不扩写功效"}},
		}),
		creativeDirectionModelResponse(t, []DirectionCandidate{
			{Concept: "通勤清爽时刻", CreativeRationale: "用真实通勤场景承载产品信息", MessagePlan: []string{"先场景后产品"}, ExecutionOutline: []string{"通勤实拍"}, GuardrailTrace: []string{"不扩写产品功效"}},
			{Concept: "青柠随行清单", CreativeRationale: "以物品清单组织核心信息", MessagePlan: []string{"先清单后行动"}, ExecutionOutline: []string{"俯拍静物"}, GuardrailTrace: []string{"仅使用输入中的产品信息"}},
		}),
	}}
	planner := ModelCreativeDirectionPlanner{Text: generator, ModelAlias: "cookies.text.standard"}

	result, err := planner.Generate(
		context.Background(), contract.ActorContext{}, contract.ProjectContext{},
		CreativePlanningContext{InputIdentityHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		2,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 2 || result.Candidates[0].Concept != "通勤清爽时刻" {
		t.Fatalf("planner did not return the repaired candidates: %+v", result)
	}
	if len(generator.requests) != 2 || generator.requests[0].InvocationKey == generator.requests[1].InvocationKey {
		t.Fatalf("repair must use a distinct provider invocation: %+v", generator.requests)
	}
	if !strings.Contains(string(generator.requests[0].InvocationKey), "_v2_0") ||
		!strings.Contains(generator.requests[1].Messages[len(generator.requests[1].Messages)-1].Content, "未通过广告主张校验") {
		t.Fatalf("repair request did not carry the v2 identity and remediation instruction: %+v", generator.requests)
	}
}

func creativeDirectionModelResponse(t *testing.T, candidates []DirectionCandidate) provider.SynchronousResponse {
	t.Helper()
	payload, err := json.Marshal(modelCreativeDirectionOutput{Candidates: candidates})
	if err != nil {
		t.Fatal(err)
	}
	return provider.SynchronousResponse{
		ProviderCode: "test-provider", ModelVersion: "test-model", StructuredOutput: payload,
	}
}

type directionModelTextStub struct {
	responses []provider.SynchronousResponse
	requests  []provider.TextGenerateRequest
}

func (stub *directionModelTextStub) GenerateText(_ context.Context, request provider.TextGenerateRequest) (provider.SynchronousResponse, error) {
	stub.requests = append(stub.requests, request)
	response := stub.responses[0]
	stub.responses = stub.responses[1:]
	return response, nil
}
