package creative

import (
	"context"
	"encoding/json"
	"strings"
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
	totalScore := 0
	highestScore := 0
	for _, candidate := range candidates {
		if candidate.ScoreMeaning != "editorial_quality_heuristic" || candidate.Score < 70 ||
			len(candidate.Storyboard) != 3 || candidate.PromptPackage.ContentHash == "" {
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
		totalScore += candidate.Score
		if candidate.Score > highestScore {
			highestScore = candidate.Score
		}
	}
	if len(hookLines) != 3 || len(voiceovers) != 3 || len(storyboards) != 3 || len(middleBeatCopies) != 3 || len(executionAngles) != 3 {
		t.Fatalf(
			"candidates must offer three distinct creative executions; hooks=%d voiceovers=%d storyboards=%d middle_beats=%d angles=%d",
			len(hookLines), len(voiceovers), len(storyboards), len(middleBeatCopies), len(executionAngles),
		)
	}
	if highestScore < 85 || totalScore/len(candidates) < 78 {
		t.Fatalf("candidate quality gate failed: highest=%d average=%d", highestScore, totalScore/len(candidates))
	}
}

func TestShortDramaCandidatesAreGroundedInUserStoryFacts(t *testing.T) {
	t.Parallel()
	snapshot := ShortDramaPrerollInputSnapshot{
		Source: IntakeSourceManual, SelectedRouteID: ManualShortDramaPrerollRouteID,
		BriefID: "brief_suspense_truth_v1", BriefVersion: 1, BriefName: "悬疑真相",
		StoryTitle:            "消失的第七份证词",
		Synopsis:              "林夏整理父亲遗物时，发现一份未出现在案件卷宗里的录音。录音时间与六年前事故记录完全矛盾。",
		ReviewedSellingPoints: []string{"关键录音出现", "事故时间矛盾"},
		HookStrategy:          ShortDramaSuspenseReveal, SubtitleStyle: "high_contrast_dynamic",
		Transition: "hard_cut", HookStrength: 4, PaceProfile: "suspense_hold",
		CallToAction: "点击正片揭开真相",
	}

	candidates, err := planShortDramaCandidates(snapshot, "sha256:suspense-story")
	if err != nil {
		t.Fatal(err)
	}
	anchors := []string{"消失的第七份证词", "林夏", "父亲遗物", "案件卷宗", "录音", "六年前事故", "关键录音", "事故时间矛盾"}
	for _, candidate := range candidates {
		visible := candidate.HookLine + " " + candidate.Voiceover + " " +
			candidate.VisualIntent + " " + candidate.TransitionLine
		for _, beat := range candidate.Storyboard {
			visible += " " + beat.Visual + " " + beat.Copy
		}
		grounded := false
		for _, anchor := range anchors {
			if strings.Contains(visible, anchor) {
				grounded = true
				break
			}
		}
		if !grounded {
			t.Fatalf("candidate %q is unrelated to the user story: %s", candidate.ID, visible)
		}
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
			SubtitleStyle: "brand_minimal", Transition: "hard_cut", HookStrength: 1, PaceProfile: "suspense_hold",
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
	promptPackage := short.Candidates[0].PromptPackage
	if promptPackage.GenerationConfig.SubtitleStyle != "brand_minimal" ||
		promptPackage.GenerationConfig.HookStrength != 1 ||
		promptPackage.GenerationConfig.PaceProfile != "suspense_hold" ||
		promptPackage.SubtitleSpec.Mode != "brand_minimal" ||
		!strings.Contains(promptPackage.CompiledPrompt, "1.5 秒内") ||
		!strings.Contains(promptPackage.CompiledPrompt, "悬念停顿") {
		t.Fatalf("generation controls were not compiled into PromptPackage: %#v", promptPackage)
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
	firstAttempt, err := service.RegisterShortDramaGenerationAttempt(
		context.Background(), rc.Actor, "project_1", task.ID, "provider_job_short_drama_1",
	)
	if err != nil {
		t.Fatal(err)
	}
	secondAttempt, err := service.RegisterShortDramaGenerationAttempt(
		context.Background(), rc.Actor, "project_1", task.ID, "provider_job_short_drama_2",
	)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := service.GetTaskDetail(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if firstAttempt.ID == secondAttempt.ID || len(restored.ShortDramaGenerationAttempts) != 2 ||
		restored.ShortDramaGenerationAttempts[0].CandidateID != short.Candidates[1].ID ||
		restored.ShortDramaGenerationAttempts[1].PromptPackageHash != promptHash {
		t.Fatalf("generation attempts were not persisted independently: %#v", restored.ShortDramaGenerationAttempts)
	}
}

func TestRegenerateShortDramaCandidatesAppendsANewDiverseBatchAndClearsSelection(t *testing.T) {
	t.Parallel()
	service := testService()
	service.ViralRemakes = service.Repository.(*memoryRepository)
	planner := &shortDramaPlannerSpy{delegate: DeterministicShortDramaPrerollPlanner{}}
	service.ShortDramaPrerollPlanner = planner
	rc := testRequestContext()
	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "short-drama-regenerate-intake", CreateIntakeRequest{
		Source: IntakeSourceManual, Format: FormatVideo, PerformanceMode: PerformanceModeShortDramaPreroll,
		Channel: ChannelDouyin, Objective: "引导用户点击观看短剧", Audience: "偏好都市逆袭剧情的竖屏观众",
		CoreMessage: "被轻视后的身份反转", CallToAction: "点击看她如何翻盘", Concept: "独立六秒短剧引流前贴",
		Tone: []string{"紧凑"}, VisualKeywords: []string{"竖屏"}, Mandatory: []string{}, Prohibited: []string{"不得虚构未确认剧情事实"},
		CreativeRoutes: []CreativeRouteSnapshot{{
			RouteID: ManualShortDramaPrerollRouteID, RouteType: PerformanceModeShortDramaPreroll, VideoPurpose: "performance",
			Channels: []string{"douyin"}, Reason: "用户选择短剧前贴本地 Brief", TargetDurationSeconds: 6, AspectRatio: "9:16", RequiresHumanConfirmation: true,
		}},
		ManualShortDramaPreroll: &ManualShortDramaPrerollInput{
			BriefID: "brief_local_urban_reversal_v1", BriefVersion: 1, BriefName: "都市逆袭 · 身份反转拉片",
			StoryTitle: "逆光归来", Synopsis: "女主在公开场合被所有人轻视，却握有足以改变众人判断的身份事实。前贴只建立悬念，不泄露完整结局，并引导观众点击观看短剧。",
			ReviewedSellingPoints: []string{"公开被轻视", "身份反转"}, HookStrategy: ShortDramaConflictReversal,
			SubtitleStyle: "high_contrast_dynamic", Transition: "hard_cut", HookStrength: 4,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	task, err := service.CreateVideoTask(context.Background(), rc.Actor, "project_1", intake.ID, CreateVideoTaskRequest{
		SelectedRouteID: ManualShortDramaPrerollRouteID, Channel: ChannelDouyin,
		Concept: "独立六秒短剧引流前贴", Prompt: "等待人工选择候选", CallToAction: "点击看她如何翻盘",
		Mandatory: []string{}, Prohibited: []string{"不得虚构未确认剧情事实"}, ConfirmRoute: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := service.GetTaskDetail(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := service.SelectShortDramaCandidate(context.Background(), rc.Actor, "project_1", task.ID, SelectShortDramaCandidateRequest{
		ExpectedRevision: before.VideoDraft.Revision,
		CandidateID:      before.VideoDraft.ShortDramaPreroll.Candidates[0].ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	previousIDs := map[string]struct{}{}
	for _, candidate := range selected.VideoDraft.ShortDramaPreroll.Candidates {
		previousIDs[candidate.ID] = struct{}{}
	}

	regenerated, err := service.RegenerateShortDramaCandidates(
		context.Background(), rc.Actor, "project_1", task.ID,
		RegenerateShortDramaCandidatesRequest{
			ExpectedRevision: selected.VideoDraft.Revision,
			GenerationConfig: ShortDramaGenerationConfig{
				SubtitleStyle: "brand_minimal",
				HookStrength:  5,
				PaceProfile:   "auto",
			},
			VariationIntent: "more_visual",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	short := regenerated.VideoDraft.ShortDramaPreroll
	if regenerated.VideoDraft.Revision != selected.VideoDraft.Revision+1 ||
		short.SelectedCandidateID != "" || short.Readiness.GenerationReady {
		t.Fatalf("regenerated draft did not append a revision and clear selection: %#v", regenerated.VideoDraft)
	}
	if short.ActiveCandidateBatch.GeneratedCandidateCount != 4 || len(short.Candidates) != 3 ||
		short.ActiveCandidateBatch.GenerationConfig.SubtitleStyle != "brand_minimal" ||
		short.ActiveCandidateBatch.GenerationConfig.HookStrength != 5 {
		t.Fatalf("candidate batch = %#v", short.ActiveCandidateBatch)
	}
	primaryVariables := map[string]struct{}{}
	executionAngles := make([]string, 0, len(short.Candidates))
	for _, candidate := range short.Candidates {
		if _, exists := previousIDs[candidate.ID]; exists {
			t.Fatalf("regeneration reused candidate id %q", candidate.ID)
		}
		primaryVariables[candidate.PrimaryTestVariable] = struct{}{}
		executionAngles = append(executionAngles, candidate.ExecutionAngle)
	}
	if len(primaryVariables) != 3 {
		t.Fatalf("primary test variables = %#v, want three distinct mechanisms", primaryVariables)
	}
	if got := strings.Join(executionAngles, ","); got != "action_reveal,result_first,reaction_escalation" {
		t.Fatalf("more_visual regeneration angles = %q, want a visual-first batch", got)
	}
	if planner.calls != 2 ||
		planner.lastSnapshot.StoryTitle != short.InputSnapshot.StoryTitle ||
		planner.lastVariationIntent != "more_visual" {
		t.Fatalf("short drama planner was not used by create and regenerate: %#v", planner)
	}
}
