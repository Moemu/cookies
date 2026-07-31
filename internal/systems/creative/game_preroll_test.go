package creative

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

func TestPlanGamePrerollCandidateBatchBuildsThreeEvidenceGroundedCandidates(t *testing.T) {
	t.Parallel()

	snapshot := GamePrerollInputSnapshot{
		Source:          IntakeSourceManual,
		SelectedRouteID: ManualGamePrerollRouteID,
		BriefID:         "fixture_defend_sunflower_v1",
		BriefVersion:    1,
		BriefName:       "《保卫向日葵》技能选择挑战",
		GameName:        "保卫向日葵",
		GameplaySummary: "竖屏塔防战斗中，玩家在波次间选择技能来调整后续战斗能力。",
		CallToAction:    "立即下载",
		EvidenceMoments: []GameEvidenceMoment{
			{
				ID: "skill_choice_1", Kind: GameEvidenceSkillChoice,
				StartMilliseconds: 20292, EndMilliseconds: 22250,
				Description:  "第 1 次技能三选一，画面展示幻系易伤、怪物易伤、获得格子。",
				VerifiedCopy: []string{"幻系易伤", "怪物易伤", "获得格子"},
			},
			{
				ID: "skill_choice_2", Kind: GameEvidenceSkillChoice,
				StartMilliseconds: 29792, EndMilliseconds: 31375,
				Description:  "第 2 次技能三选一，画面展示激光弹射、格子概率、全体加攻。",
				VerifiedCopy: []string{"激光弹射", "格子概率", "全体加攻"},
			},
			{
				ID: "wave_2", Kind: GameEvidenceWaveProgress,
				StartMilliseconds: 34000, EndMilliseconds: 35500,
				Description:  "选择完成后进入第 2/10 波。",
				VerifiedCopy: []string{"第2/10波"},
			},
		},
		AllowedMechanisms: []GameHookMechanism{
			GameHookChoiceChallenge,
			GameHookTacticalTradeoff,
			GameHookWaveEscalation,
		},
		ProhibitedMechanisms: []GameHookMechanism{
			GameHookFailureReversal,
			GameHookMergeUpgrade,
			GameHookRewardReveal,
		},
	}

	batch, err := planGamePrerollCandidateBatch(
		snapshot,
		"sha256:fixture",
		"game_batch_1",
		1,
		GamePrerollGenerationConfig{
			SubtitleStyle: "high_contrast_dynamic",
			HookStrength:  4,
			PaceProfile:   "punchy",
		},
		time.Unix(1, 0).UTC(),
	)
	if err != nil {
		t.Fatalf("plan game preroll candidates: %v", err)
	}
	if len(batch.Candidates) != 3 {
		t.Fatalf("candidate count = %d, want 3", len(batch.Candidates))
	}

	seenMechanisms := map[GameHookMechanism]bool{}
	for _, candidate := range batch.Candidates {
		if candidate.PromptPackage.ContentHash == "" || candidate.PromptPackage.CompiledPrompt == "" {
			t.Fatalf("candidate %q is missing a compiled PromptPackage", candidate.ID)
		}
		if candidate.PromptPackage.InputSnapshotHash != "sha256:fixture" {
			t.Fatalf("candidate %q input snapshot hash = %q", candidate.ID, candidate.PromptPackage.InputSnapshotHash)
		}
		if len(candidate.EvidenceMomentIDs) == 0 {
			t.Fatalf("candidate %q is not grounded in evidence", candidate.ID)
		}
		if !containsGameHookMechanism(snapshot.AllowedMechanisms, candidate.HookMechanism) {
			t.Fatalf("candidate %q used unapproved mechanism %q", candidate.ID, candidate.HookMechanism)
		}
		if containsGameHookMechanism(snapshot.ProhibitedMechanisms, candidate.HookMechanism) {
			t.Fatalf("candidate %q used prohibited mechanism %q", candidate.ID, candidate.HookMechanism)
		}
		if len(candidate.Storyboard) != 3 ||
			candidate.Storyboard[0].StartMilliseconds != 0 ||
			candidate.Storyboard[2].EndMilliseconds != 6000 {
			t.Fatalf("candidate %q storyboard does not cover one 6-second video: %#v", candidate.ID, candidate.Storyboard)
		}
		seenMechanisms[candidate.HookMechanism] = true
	}
	if len(seenMechanisms) != 3 {
		t.Fatalf("candidate mechanisms are not diverse: %#v", seenMechanisms)
	}
}

func TestModelGamePrerollPlannerUsesStructuredOutlinesButServerCompilesPrompts(t *testing.T) {
	t.Parallel()

	structured := json.RawMessage(`{
		"candidates": [
			{
				"hook_mechanism": "tactical_tradeoff",
				"execution_angle": "model_tradeoff",
				"primary_test_variable": "skill_names",
				"variant_hypothesis": "真实技能名能让选择更具体。",
				"hook_line": "获得格子，还是全体加攻？",
				"evidence_moment_ids": ["skill_choice_1", "skill_choice_2", "wave_2"],
				"compiled_prompt": "UNTRUSTED MODEL PROMPT"
			},
			{
				"hook_mechanism": "wave_escalation",
				"execution_angle": "model_wave",
				"primary_test_variable": "wave_pressure",
				"variant_hypothesis": "下一波信息能建立紧迫感。",
				"hook_line": "第 2 波开始前，你选哪个？",
				"evidence_moment_ids": ["wave_2", "skill_choice_2"]
			},
			{
				"hook_mechanism": "choice_challenge",
				"execution_angle": "model_choice",
				"primary_test_variable": "direct_question",
				"variant_hypothesis": "直接提问能邀请用户参与判断。",
				"hook_line": "三个技能只能选一个。",
				"evidence_moment_ids": ["skill_choice_1", "skill_choice_2"]
			}
		]
	}`)
	text := &gamePlannerTextStub{response: provider.SynchronousResponse{
		ProviderCode: "adapter_gateway", ModelAlias: "Seed-2-pro",
		ModelVersion: "doubao-seed-2-0-pro-260215", StructuredOutput: structured,
	}}
	planner := ModelGamePrerollPlanner{Text: text, ModelAlias: "Seed-2-pro"}
	snapshot := testGamePrerollSnapshot()

	batch, err := planner.Plan(
		context.Background(),
		contract.ActorContext{
			OrganizationID: "org_1",
			Principal:      contract.Principal{Kind: contract.PrincipalUser, ID: "usr_1"},
		},
		contract.ProjectContext{
			OrganizationID: "org_1", ProjectID: "project_1",
			BrandID:    func() *contract.BrandID { value := contract.BrandID("brand_1"); return &value }(),
			ProductIDs: []contract.ProductID{}, ProjectContextVersion: 1,
		},
		snapshot,
		"sha256:fixture",
		"game_batch_model",
		1,
		GamePrerollGenerationConfig{SubtitleStyle: "high_contrast_dynamic", HookStrength: 4, PaceProfile: "punchy"},
		time.Unix(2, 0).UTC(),
	)
	if err != nil {
		t.Fatalf("model plan: %v", err)
	}
	if text.request.ModelAlias != "Seed-2-pro" || len(text.request.OutputJSONSchema) == 0 {
		t.Fatalf("text request = %#v", text.request)
	}
	if batch.PlannerVersion != "model:adapter_gateway/doubao-seed-2-0-pro-260215" {
		t.Fatalf("planner version = %q", batch.PlannerVersion)
	}
	if batch.Candidates[0].ExecutionAngle != "model_tradeoff" ||
		batch.Candidates[0].PromptPackage.CompiledPrompt == "UNTRUSTED MODEL PROMPT" ||
		!strings.Contains(batch.Candidates[0].PromptPackage.CompiledPrompt, "严禁虚构") {
		t.Fatalf("server did not compile the model outline safely: %#v", batch.Candidates[0])
	}
}

type gamePlannerTextStub struct {
	request  provider.TextGenerateRequest
	response provider.SynchronousResponse
	err      error
}

func (s *gamePlannerTextStub) GenerateText(_ context.Context, request provider.TextGenerateRequest) (provider.SynchronousResponse, error) {
	s.request = request
	return s.response, s.err
}

func TestFallbackGamePrerollPlannerReportsPrimaryFailure(t *testing.T) {
	t.Parallel()

	primaryErr := errors.New("model output invalid")
	var reported error
	planner := FallbackGamePrerollPlanner{
		Primary:  gamePlannerFailureStub{err: primaryErr},
		Fallback: DeterministicGamePrerollPlanner{},
		OnPrimaryFailure: func(err error) {
			reported = err
		},
	}
	batch, err := planner.Plan(
		context.Background(),
		contract.ActorContext{},
		contract.ProjectContext{},
		testGamePrerollSnapshot(),
		"sha256:fixture",
		"game_batch_fallback",
		1,
		GamePrerollGenerationConfig{SubtitleStyle: "high_contrast_dynamic", HookStrength: 4, PaceProfile: "punchy"},
		time.Unix(3, 0).UTC(),
	)
	if err != nil {
		t.Fatalf("fallback plan: %v", err)
	}
	if !errors.Is(reported, primaryErr) || !strings.HasPrefix(batch.PlannerVersion, "fallback:") {
		t.Fatalf("reported = %v, planner version = %q", reported, batch.PlannerVersion)
	}
}

type gamePlannerFailureStub struct {
	err error
}

func (s gamePlannerFailureStub) Plan(
	context.Context,
	contract.ActorContext,
	contract.ProjectContext,
	GamePrerollInputSnapshot,
	string,
	string,
	int64,
	GamePrerollGenerationConfig,
	time.Time,
) (GameCandidateBatch, error) {
	return GameCandidateBatch{}, s.err
}

func TestManualGamePrerollRequiresHumanSelectionBeforeVideoGeneration(t *testing.T) {
	t.Parallel()

	service := testService()
	service.ViralRemakes = service.Repository.(*memoryRepository)
	sourceVideo := contract.AssetVersionRef{AssetID: "asset_gameplay", Version: 1}
	service.Assets = testAssetReader{snapshot: CreativeAssetSnapshot{
		Ref: sourceVideo, Kind: contract.AssetVideo, MIMEType: "video/mp4", Ready: true,
	}}
	rc := testRequestContext()
	manual := testManualGamePrerollInput(sourceVideo)

	intake, err := service.CreateIntake(context.Background(), rc, "project_1", "game-preroll-intake-1", CreateIntakeRequest{
		Source: IntakeSourceManual, Format: FormatVideo, PerformanceMode: PerformanceModeGamePreroll,
		Channel: ChannelDouyin, Objective: "用真实技能选择建立挑战感并引导下载",
		Audience:     "喜欢轻策略塔防和技能搭配的竖屏游戏用户",
		CoreMessage:  "每波技能选择都会改变接下来的塔防决策",
		CallToAction: "立即下载", Concept: "《保卫向日葵》技能选择挑战",
		Tone: []string{"紧张", "可读"}, VisualKeywords: []string{"真实玩法", "技能三选一"},
		Mandatory: []string{"保留真实技能名和波次"}, Prohibited: []string{"不得虚构失败、奖励或升级结果"},
		CreativeRoutes: []CreativeRouteSnapshot{{
			RouteID: ManualGamePrerollRouteID, RouteType: PerformanceModeGamePreroll,
			VideoPurpose: "performance", Channels: []string{"douyin"},
			Reason: "用户确认使用本地游戏前贴固定样例", TargetDurationSeconds: 6,
			AspectRatio: "9:16", SourceAssetRefs: []contract.AssetVersionRef{sourceVideo},
			EvidenceRefs:              []string{"skill_choice_1", "skill_choice_2", "wave_2"},
			RequiresHumanConfirmation: true,
		}},
		ManualGamePreroll: &manual,
	})
	if err != nil {
		t.Fatalf("create game intake: %v", err)
	}
	task, err := service.CreateVideoTask(context.Background(), rc.Actor, "project_1", intake.ID, CreateVideoTaskRequest{
		SelectedRouteID: ManualGamePrerollRouteID, Channel: ChannelDouyin, SourceVideo: sourceVideo,
		Concept: "《保卫向日葵》技能选择挑战", Prompt: "等待人工选择候选",
		CallToAction: "立即下载", Mandatory: []string{"保留真实技能名和波次"},
		Prohibited: []string{"不得虚构失败、奖励或升级结果"}, ConfirmRoute: true,
	})
	if err != nil {
		t.Fatalf("create game task: %v", err)
	}
	detail, err := service.GetTaskDetail(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatal(err)
	}
	game := detail.VideoDraft.GamePreroll
	if game == nil || len(game.Candidates) != 3 || game.Readiness.GenerationReady {
		t.Fatalf("game preroll draft = %#v", game)
	}
	if _, _, err := service.GamePrerollProviderInput(context.Background(), rc.Actor, "project_1", task.ID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("provider input before human selection error = %v, want ErrInvalidState", err)
	}

	selected, err := service.SelectGamePrerollCandidate(context.Background(), rc.Actor, "project_1", task.ID, SelectGamePrerollCandidateRequest{
		ExpectedRevision: detail.VideoDraft.Revision,
		CandidateID:      game.Candidates[1].ID,
	})
	if err != nil {
		t.Fatalf("select game candidate: %v", err)
	}
	if selected.VideoDraft.GamePreroll.SelectedCandidateID != game.Candidates[1].ID ||
		!selected.VideoDraft.GamePreroll.Readiness.GenerationReady {
		t.Fatalf("selected game draft = %#v", selected.VideoDraft.GamePreroll)
	}
	providerInput, promptHash, err := service.GamePrerollProviderInput(context.Background(), rc.Actor, "project_1", task.ID)
	if err != nil {
		t.Fatalf("build game provider input: %v", err)
	}
	if providerInput.DurationSeconds != 6 || providerInput.AspectRatio != "9:16" ||
		providerInput.Prompt == "" || promptHash == "" {
		t.Fatalf("provider input = %#v, prompt hash = %q", providerInput, promptHash)
	}
	attempt, err := service.RegisterGamePrerollGenerationAttempt(
		context.Background(), rc.Actor, "project_1", task.ID, "provider_job_game_1",
	)
	if err != nil {
		t.Fatalf("register game generation attempt: %v", err)
	}
	restored, err := service.GetLatestGamePrerollWorkspace(context.Background(), rc.Actor, "project_1")
	if err != nil {
		t.Fatalf("restore game workspace: %v", err)
	}
	if len(restored.GamePrerollGenerationAttempts) != 1 ||
		restored.GamePrerollGenerationAttempts[0].ID != attempt.ID ||
		restored.GamePrerollGenerationAttempts[0].CandidateID != game.Candidates[1].ID ||
		restored.GamePrerollGenerationAttempts[0].PromptPackageHash != promptHash {
		t.Fatalf("restored game attempts = %#v", restored.GamePrerollGenerationAttempts)
	}
	regenerated, err := service.RegenerateGamePrerollCandidates(
		context.Background(),
		rc.Actor,
		"project_1",
		task.ID,
		RegenerateGamePrerollCandidatesRequest{
			ExpectedRevision: restored.VideoDraft.Revision,
			GenerationConfig: GamePrerollGenerationConfig{
				SubtitleStyle: "high_contrast_dynamic",
				HookStrength:  4,
				PaceProfile:   "punchy",
			},
		},
	)
	if err != nil {
		t.Fatalf("regenerate game candidates: %v", err)
	}
	if regenerated.VideoDraft.Revision != restored.VideoDraft.Revision+1 ||
		regenerated.VideoDraft.GamePreroll.SelectedCandidateID != "" ||
		regenerated.VideoDraft.GamePreroll.Readiness.GenerationReady ||
		len(regenerated.VideoDraft.GamePreroll.Candidates) != 3 {
		t.Fatalf("regenerated game draft = %#v", regenerated.VideoDraft.GamePreroll)
	}
}

func testManualGamePrerollInput(sourceVideo contract.AssetVersionRef) ManualGamePrerollInput {
	return ManualGamePrerollInput{
		BriefID: "fixture_defend_sunflower_v1", BriefVersion: 1,
		BriefName: "《保卫向日葵》技能选择挑战", GameName: "保卫向日葵",
		GameplaySummary:   "竖屏塔防战斗中，玩家在波次间选择技能来调整后续战斗能力。",
		SourceVideo:       sourceVideo,
		SourceVideoRights: RightsConfirmed,
		EvidenceMoments: []GameEvidenceMoment{
			{ID: "skill_choice_1", Kind: GameEvidenceSkillChoice, StartMilliseconds: 20292, EndMilliseconds: 22250, Description: "第 1 次技能三选一。", VerifiedCopy: []string{"幻系易伤", "怪物易伤", "获得格子"}},
			{ID: "skill_choice_2", Kind: GameEvidenceSkillChoice, StartMilliseconds: 29792, EndMilliseconds: 31375, Description: "第 2 次技能三选一。", VerifiedCopy: []string{"激光弹射", "格子概率", "全体加攻"}},
			{ID: "wave_2", Kind: GameEvidenceWaveProgress, StartMilliseconds: 34000, EndMilliseconds: 35500, Description: "选择后进入第 2/10 波。", VerifiedCopy: []string{"第2/10波"}},
		},
		AllowedMechanisms:    []GameHookMechanism{GameHookChoiceChallenge, GameHookTacticalTradeoff, GameHookWaveEscalation},
		ProhibitedMechanisms: []GameHookMechanism{GameHookFailureReversal, GameHookMergeUpgrade, GameHookRewardReveal},
		SubtitleStyle:        "high_contrast_dynamic", HookStrength: 4, PaceProfile: "punchy",
	}
}

func testGamePrerollSnapshot() GamePrerollInputSnapshot {
	manual := testManualGamePrerollInput(contract.AssetVersionRef{AssetID: "asset_gameplay", Version: 1})
	return GamePrerollInputSnapshot{
		Source: IntakeSourceManual, SelectedRouteID: ManualGamePrerollRouteID,
		BriefID: manual.BriefID, BriefVersion: manual.BriefVersion, BriefName: manual.BriefName,
		GameName: manual.GameName, GameplaySummary: manual.GameplaySummary,
		SourceVideo: manual.SourceVideo, SourceVideoRights: manual.SourceVideoRights,
		CallToAction: "立即下载", EvidenceMoments: manual.EvidenceMoments,
		AllowedMechanisms: manual.AllowedMechanisms, ProhibitedMechanisms: manual.ProhibitedMechanisms,
	}
}
