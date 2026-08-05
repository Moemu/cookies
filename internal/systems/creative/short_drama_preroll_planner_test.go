package creative

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

func TestModelShortDramaPrerollPlannerReturnsStoryGroundedStructuredCandidates(t *testing.T) {
	t.Parallel()
	text := &shortDramaPlannerTextStub{response: provider.SynchronousResponse{
		ProviderCode: "adapter_gateway",
		ModelAlias:   "cookies.text.standard",
		ModelVersion: "text-model-v1",
		StructuredOutput: json.RawMessage(`{
			"candidates": [
				{
					"execution_angle": "action_reveal",
					"primary_test_variable": "missing_recording",
					"variant_hypothesis": "用案件卷宗中缺失的录音建立首秒信息缺口。",
					"grounding_quote": "案件卷宗里的录音",
					"hook_line": "案件卷宗里的录音，为什么消失了？",
					"opening_visual": "林夏翻开案件卷宗，录音条目所在位置留下空白。",
					"middle_visual": "镜头推近录音时间与事故记录的冲突标记。",
					"middle_copy": "这份录音，从未出现在卷宗里。",
					"transition_line": "谁删掉了第七份证词？"
				},
				{
					"execution_angle": "result_first",
					"primary_test_variable": "time_conflict",
					"variant_hypothesis": "先展示时间矛盾，再追问记录被修改的原因。",
					"grounding_quote": "六年前事故记录",
					"hook_line": "录音时间，和六年前事故记录对不上。",
					"opening_visual": "两个时间戳并列出现，六年前事故记录被红线圈出。",
					"middle_visual": "林夏暂停录音，画面停在矛盾的日期上。",
					"middle_copy": "有人改过事故发生的时间。",
					"transition_line": "真正隐瞒秘密的人是谁？"
				},
				{
					"execution_angle": "dialogue_confrontation",
					"primary_test_variable": "story_title",
					"variant_hypothesis": "直接点出剧名中的第七份证词，强化记忆与点击意图。",
					"grounding_quote": "消失的第七份证词",
					"hook_line": "消失的第七份证词，刚刚被她找到了。",
					"opening_visual": "林夏拿起父亲遗物中的录音设备。",
					"middle_visual": "镜头切到她听见关键内容后的克制反应。",
					"middle_copy": "但卷宗里，没有这份证词。",
					"transition_line": "点击正片，听见被隐瞒的真相。"
				}
			]
		}`),
	}}
	planner := ModelShortDramaPrerollPlanner{Text: text, ModelAlias: "cookies.text.standard"}
	snapshot := testSuspenseShortDramaSnapshot()

	batch, err := planner.Plan(
		context.Background(),
		contract.ActorContext{
			OrganizationID: "org_1",
			Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"},
		},
		contract.ProjectContext{
			OrganizationID: "org_1", ProjectID: "project_1",
			ProductIDs: []contract.ProductID{}, ProjectContextVersion: 1,
		},
		snapshot,
		"sha256:story",
		"short_drama_batch_model",
		1,
		shortDramaGenerationConfig(snapshot),
		"balanced",
		time.Unix(2, 0).UTC(),
	)
	if err != nil {
		t.Fatalf("model plan: %v", err)
	}
	if len(text.request.OutputJSONSchema) == 0 ||
		!strings.Contains(text.request.Messages[1].Content, snapshot.StoryTitle) {
		t.Fatalf("model request is missing schema or immutable story input: %#v", text.request)
	}
	if !strings.HasPrefix(batch.PlannerVersion, "model:") || len(batch.Candidates) != 3 {
		t.Fatalf("unexpected model batch: %#v", batch)
	}
	for _, candidate := range batch.Candidates {
		if !shortDramaCandidateGroundedInSnapshot(snapshot, candidate) {
			t.Fatalf("candidate is not grounded in story input: %#v", candidate)
		}
		if !strings.Contains(candidate.PromptPackage.CompiledPrompt, snapshot.StoryTitle) {
			t.Fatalf("server-compiled prompt lost story title: %s", candidate.PromptPackage.CompiledPrompt)
		}
	}
}

func TestFallbackShortDramaPrerollPlannerRejectsUngroundedModelOutput(t *testing.T) {
	t.Parallel()
	text := &shortDramaPlannerTextStub{response: provider.SynchronousResponse{
		ProviderCode: "adapter_gateway",
		ModelVersion: "text-model-v1",
		StructuredOutput: json.RawMessage(`{
			"candidates": [
				{"execution_angle":"action_reveal","primary_test_variable":"generic_1","variant_hypothesis":"通用悬念","grounding_quote":"豪门继承人","hook_line":"豪门继承人出现了。","opening_visual":"众人震惊","middle_visual":"身份揭晓","middle_copy":"她的身份不简单","transition_line":"点击观看"},
				{"execution_angle":"result_first","primary_test_variable":"generic_2","variant_hypothesis":"通用反转","grounding_quote":"豪门继承人","hook_line":"所有人都沉默了。","opening_visual":"所有人沉默","middle_visual":"主角回头","middle_copy":"真相即将出现","transition_line":"点击观看"},
				{"execution_angle":"dialogue_confrontation","primary_test_variable":"generic_3","variant_hypothesis":"通用冲突","grounding_quote":"豪门继承人","hook_line":"她只说了一句话。","opening_visual":"人物对峙","middle_visual":"众人反应","middle_copy":"没有人敢回答","transition_line":"点击观看"}
			]
		}`),
	}}
	var primaryFailure error
	planner := FallbackShortDramaPrerollPlanner{
		Primary:  ModelShortDramaPrerollPlanner{Text: text, ModelAlias: "cookies.text.standard"},
		Fallback: DeterministicShortDramaPrerollPlanner{},
		OnPrimaryFailure: func(err error) {
			primaryFailure = err
		},
	}
	snapshot := testSuspenseShortDramaSnapshot()

	batch, err := planner.Plan(
		context.Background(),
		contract.ActorContext{},
		contract.ProjectContext{},
		snapshot,
		"sha256:story",
		"short_drama_batch_fallback",
		1,
		shortDramaGenerationConfig(snapshot),
		"balanced",
		time.Unix(3, 0).UTC(),
	)
	if err != nil {
		t.Fatalf("fallback plan: %v", err)
	}
	if primaryFailure == nil || !strings.Contains(primaryFailure.Error(), "grounding_quote") {
		t.Fatalf("ungrounded model output was not rejected: %v", primaryFailure)
	}
	if !strings.HasPrefix(batch.PlannerVersion, "fallback:") {
		t.Fatalf("planner version = %q", batch.PlannerVersion)
	}
	for _, candidate := range batch.Candidates {
		if !shortDramaCandidateGroundedInSnapshot(snapshot, candidate) {
			t.Fatalf("fallback candidate is not grounded: %#v", candidate)
		}
	}
}

type shortDramaPlannerTextStub struct {
	request  provider.TextGenerateRequest
	response provider.SynchronousResponse
	err      error
}

func (s *shortDramaPlannerTextStub) GenerateText(_ context.Context, request provider.TextGenerateRequest) (provider.SynchronousResponse, error) {
	s.request = request
	return s.response, s.err
}

type shortDramaPlannerFailureStub struct {
	err error
}

func (s shortDramaPlannerFailureStub) Plan(
	context.Context,
	contract.ActorContext,
	contract.ProjectContext,
	ShortDramaPrerollInputSnapshot,
	string,
	string,
	int64,
	ShortDramaGenerationConfig,
	string,
	time.Time,
) (ShortDramaCandidateBatch, error) {
	return ShortDramaCandidateBatch{}, s.err
}

type shortDramaPlannerSpy struct {
	delegate            ShortDramaPrerollPlanner
	calls               int
	lastSnapshot        ShortDramaPrerollInputSnapshot
	lastVariationIntent string
}

func (s *shortDramaPlannerSpy) Plan(
	ctx context.Context,
	actor contract.ActorContext,
	project contract.ProjectContext,
	snapshot ShortDramaPrerollInputSnapshot,
	inputHash string,
	batchID string,
	revision int64,
	config ShortDramaGenerationConfig,
	variationIntent string,
	now time.Time,
) (ShortDramaCandidateBatch, error) {
	s.calls++
	s.lastSnapshot = snapshot
	s.lastVariationIntent = variationIntent
	return s.delegate.Plan(
		ctx,
		actor,
		project,
		snapshot,
		inputHash,
		batchID,
		revision,
		config,
		variationIntent,
		now,
	)
}

func testSuspenseShortDramaSnapshot() ShortDramaPrerollInputSnapshot {
	return ShortDramaPrerollInputSnapshot{
		Source: IntakeSourceManual, SelectedRouteID: ManualShortDramaPrerollRouteID,
		BriefID: "brief_suspense_truth_v1", BriefVersion: 1, BriefName: "悬疑真相",
		StoryTitle:            "消失的第七份证词",
		Synopsis:              "林夏整理父亲遗物时，发现一份未出现在案件卷宗里的录音。录音时间与六年前事故记录完全矛盾。",
		ReviewedSellingPoints: []string{"关键录音出现", "事故时间矛盾"},
		HookStrategy:          ShortDramaSuspenseReveal, SubtitleStyle: "high_contrast_dynamic",
		Transition: "hard_cut", HookStrength: 4, PaceProfile: "suspense_hold",
		CallToAction: "点击正片揭开真相",
	}
}
