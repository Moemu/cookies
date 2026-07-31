package creative

import (
	"context"
	"encoding/json"
	"testing"
)

func TestPlanShortDramaCandidatesBuildsThreeExplainableCandidates(t *testing.T) {
	t.Parallel()
	snapshot := ShortDramaPrerollInputSnapshot{
		Source: IntakeSourceManual, SelectedRouteID: ManualShortDramaPrerollRouteID,
		BriefID: "brief_local_urban_reversal_v1", BriefVersion: 1, BriefName: "都市逆袭",
		StoryTitle: "逆光归来", Synopsis: "女主在公司年会上被众人轻视，却在公开场合拿出能改变所有人判断的关键证据。",
		ReviewedSellingPoints: []string{"公开受轻视", "身份信息待正片揭示"}, HookStrategy: ShortDramaConflictReversal,
		SubtitleStyle: "high_contrast_dynamic", Transition: "hard_cut", HookStrength: 4,
		CallToAction: "点击看她如何反转局面",
	}
	candidates, err := planShortDramaCandidates(snapshot, "sha256:test")
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 3 {
		t.Fatalf("candidate count = %d, want 3", len(candidates))
	}
	hookLines := map[string]struct{}{}
	voiceovers := map[string]struct{}{}
	storyboards := map[string]struct{}{}
	middleBeatCopies := map[string]struct{}{}
	executionAngles := map[string]struct{}{}
	for _, candidate := range candidates {
		if candidate.ScoreMeaning != "hook_relevance" || len(candidate.Storyboard) != 3 || candidate.PromptPackage.ContentHash == "" {
			t.Fatalf("candidate is incomplete: %+v", candidate)
		}
		if candidate.PromptPackage.CompiledPrompt == "" {
			t.Fatal("compiled prompt is empty")
		}
		storyboard, err := json.Marshal(candidate.Storyboard)
		if err != nil {
			t.Fatal(err)
		}
		hookLines[candidate.HookLine] = struct{}{}
		voiceovers[candidate.Voiceover] = struct{}{}
		storyboards[string(storyboard)] = struct{}{}
		middleBeatCopies[candidate.Storyboard[1].Copy] = struct{}{}
		executionAngles[candidate.ExecutionAngle] = struct{}{}
	}
	if len(hookLines) != 3 || len(voiceovers) != 3 || len(storyboards) != 3 || len(middleBeatCopies) != 3 || len(executionAngles) != 3 {
		t.Fatalf(
			"candidates must offer three distinct creative executions; hooks=%d voiceovers=%d storyboards=%d middle_beats=%d angles=%d",
			len(hookLines), len(voiceovers), len(storyboards), len(middleBeatCopies), len(executionAngles),
		)
	}
}

func TestManualShortDramaPrerollInputRequiresReviewedFacts(t *testing.T) {
	t.Parallel()
	input := ManualShortDramaPrerollInput{
		BriefID: "brief", BriefVersion: 1, BriefName: "样例", StoryTitle: "标题",
		Synopsis:              "这是一个足够长的都市短剧故事梗概，用于验证人工输入必须包含已审核剧情事实。",
		ReviewedSellingPoints: []string{}, HookStrategy: ShortDramaConflictReversal,
		SubtitleStyle: "high_contrast_dynamic", Transition: "hard_cut", HookStrength: 4,
	}
	if err := input.Validate(); err == nil {
		t.Fatal("expected reviewed selling point validation error")
	}
}

func TestManualShortDramaPrerollFreezesCandidateBeforeVideoGeneration(t *testing.T) {
	t.Parallel()
	service := testService()
	service.ViralRemakes = service.Repository.(*memoryRepository)
	service.Assets = nil
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "short-drama-intake-1", CreateIntakeRequest{
		Source: IntakeSourceManual, Format: FormatVideo, PerformanceMode: PerformanceModeShortDramaPreroll,
		Channel: ChannelDouyin, Objective: "引导用户继续观看短剧正片", Audience: "偏好都市逆袭剧情的竖屏观众",
		CoreMessage: "被轻视后的身份反转", CallToAction: "点击看她如何反转局面", Concept: "短剧正片导流前贴",
		Tone: []string{"紧凑"}, VisualKeywords: []string{"人物连续"}, Mandatory: []string{}, Prohibited: []string{"不得虚构未确认剧情事实"},
		CreativeRoutes: []CreativeRouteSnapshot{{
			RouteID: ManualShortDramaPrerollRouteID, RouteType: PerformanceModeShortDramaPreroll, VideoPurpose: "performance",
			Channels: []string{"douyin"}, Reason: "用户选择短剧前贴本地 Brief", TargetDurationSeconds: 6, AspectRatio: "9:16", RequiresHumanConfirmation: true,
		}},
		ManualShortDramaPreroll: &ManualShortDramaPrerollInput{
			BriefID: "brief_local_urban_reversal_v1", BriefVersion: 1, BriefName: "都市逆袭 · 身份反转拉片",
			StoryTitle: "逆光归来", Synopsis: "女主在公司年会上被众人轻视，却在公开场合拿出能改变所有人判断的关键证据。她没有立即解释，只把所有人的目光引向正片即将揭开的身份真相。",
			ReviewedSellingPoints: []string{"被轻视", "身份反转"}, HookStrategy: ShortDramaConflictReversal,
			SubtitleStyle: "high_contrast_dynamic", Transition: "hard_cut", HookStrength: 4,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.CreateVideoTask(context.Background(), rc.Actor, "project_1", intake.ID, CreateVideoTaskRequest{
		SelectedRouteID: ManualShortDramaPrerollRouteID, Channel: ChannelDouyin,
		Concept: "短剧正片导流前贴", Prompt: "等待人工选择候选", CallToAction: "点击看她如何反转局面",
		Mandatory: []string{}, Prohibited: []string{"不得虚构未确认剧情事实"}, ConfirmRoute: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	detail, err := service.GetTaskDetail(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	short := detail.VideoDraft.ShortDramaPreroll
	if short == nil || len(short.Candidates) != 3 || short.Readiness.GenerationReady {
		t.Fatalf("short drama draft = %#v", short)
	}
	selected, err := service.SelectShortDramaCandidate(context.Background(), rc.Actor, "project_1", task.ID, SelectShortDramaCandidateRequest{
		ExpectedRevision: detail.VideoDraft.Revision, CandidateID: short.Candidates[1].ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if selected.VideoDraft.ShortDramaPreroll.SelectedCandidateID != short.Candidates[1].ID || !selected.VideoDraft.ShortDramaPreroll.Readiness.GenerationReady {
		t.Fatalf("selected short drama draft = %#v", selected.VideoDraft.ShortDramaPreroll)
	}
	providerInput, promptHash, err := service.ShortDramaProviderInput(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if providerInput.DurationSeconds != 6 || providerInput.AspectRatio != "9:16" || providerInput.Prompt == "" || promptHash == "" {
		t.Fatalf("provider input = %#v, hash=%q", providerInput, promptHash)
	}
}
