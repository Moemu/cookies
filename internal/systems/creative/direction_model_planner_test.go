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
	if !strings.Contains(string(generator.requests[0].InvocationKey), "_v3_0") ||
		!strings.Contains(generator.requests[1].Messages[len(generator.requests[1].Messages)-1].Content, "未通过质量校验") {
		t.Fatalf("repair request did not carry the v3 identity and remediation instruction: %+v", generator.requests)
	}
}

func TestModelCreativeDirectionPlannerRepairsUtilityHeavyBrandBatch(t *testing.T) {
	utility := func(concept string) DirectionCandidate {
		return DirectionCandidate{
			Concept: concept, CreativeRationale: "用知识步骤解释工程判断", DirectionMode: "utility",
			MessagePlan: []string{"先问题后方法"}, ExecutionOutline: []string{"清单式讲解"}, GuardrailTrace: []string{"不虚构结论"},
			EmotionalArc: "从疑惑到理解", VisualGrammar: "信息卡片", BrandMemoryDevice: "海军蓝标尺", HumanMoment: "工程师核对图纸",
		}
	}
	generator := &directionModelTextStub{responses: []provider.SynchronousResponse{
		creativeDirectionModelResponse(t, []DirectionCandidate{utility("打样避坑指南"), utility("供应商核验清单"), utility("三步判断工具")}),
		creativeDirectionModelResponse(t, []DirectionCandidate{
			{Concept: "图纸沉默之后", CreativeRationale: "把不确定转成被看见的工程判断", DirectionMode: "emotional", MessagePlan: []string{"先焦虑后笃定"}, ExecutionOutline: []string{"空旷办公室切到机床微光"}, GuardrailTrace: []string{"只展示可核验过程"}, EmotionalArc: "从独自承担到有人共同判断", VisualGrammar: "低照度长镜头与微距金属反光", BrandMemoryDevice: "海军蓝校准线与一次清脆归零声", HumanMoment: "研发负责人按下发送前停顿，工程师回传标注"},
			{Concept: "毫米之间，有人回答", CreativeRationale: "用人物接力建立工程伙伴认知", DirectionMode: "cinematic", MessagePlan: []string{"问题跨越空间被接住"}, ExecutionOutline: []string{"图纸线条匹配两地人物动作"}, GuardrailTrace: []string{"不承诺具体精度"}, EmotionalArc: "从悬而未决到获得回应", VisualGrammar: "动作匹配剪辑与克制工业声场", BrandMemoryDevice: "银色测量光带贯穿转场", HumanMoment: "采购与工程师隔屏同时指向同一处标注"},
			{Concept: "一次判断的来路", CreativeRationale: "展示判断如何形成", DirectionMode: "utility", MessagePlan: []string{"展示依据而非教学步骤"}, ExecutionOutline: []string{"证据在桌面逐层聚合"}, GuardrailTrace: []string{"不虚构数据"}, EmotionalArc: "从模糊到清晰", VisualGrammar: "俯拍证据与留白字幕", BrandMemoryDevice: "蓝色验证印记", HumanMoment: "工程师签下注释后抬头确认"},
		}),
	}}
	planner := ModelCreativeDirectionPlanner{Text: generator, ModelAlias: "cookies.text.standard"}

	result, err := planner.Generate(context.Background(), contract.ActorContext{}, contract.ProjectContext{}, CreativePlanningContext{
		InputIdentityHash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		SelectedRoute:     CreativeRouteSnapshot{RouteType: CreativeRouteBrandVideo},
	}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 3 || result.PromptVersion != creativeDirectionPromptVersion {
		t.Fatalf("unexpected repaired brand batch: %+v", result)
	}
	if !strings.Contains(generator.requests[0].Messages[0].Content, "品牌视频") ||
		!strings.Contains(generator.requests[1].Messages[len(generator.requests[1].Messages)-1].Content, "utility-led") {
		t.Fatalf("brand quality contract was not carried into prompt and repair: %+v", generator.requests)
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
