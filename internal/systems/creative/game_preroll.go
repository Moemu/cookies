package creative

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	PerformanceModeGamePreroll   = "game_preroll"
	ManualGamePrerollRouteID     = "route_manual_game_preroll_v1"
	ManualGamePrerollV2RouteID   = "route_manual_game_preroll_v2"
	GamePrerollV2ContractVersion = "creative-game-preroll-workspace/v2"
)

type GamePrerollStage string

const (
	GamePrerollStageSourceReady       GamePrerollStage = "source_ready"
	GamePrerollStageAnalysisReady     GamePrerollStage = "analysis_ready"
	GamePrerollStageBriefConfirmed    GamePrerollStage = "brief_confirmed"
	GamePrerollStageCandidatesReady   GamePrerollStage = "candidates_ready"
	GamePrerollStageCandidateSelected GamePrerollStage = "candidate_selected"
	GamePrerollStageVideoGenerating   GamePrerollStage = "video_generating"
	GamePrerollStageVideoReady        GamePrerollStage = "video_ready"
)

type GamePrerollResourceStatus string

const (
	GamePrerollResourceIdle    GamePrerollResourceStatus = "idle"
	GamePrerollResourceRunning GamePrerollResourceStatus = "running"
	GamePrerollResourceReady   GamePrerollResourceStatus = "ready"
	GamePrerollResourceFailed  GamePrerollResourceStatus = "failed"
)

type ManualGamePrerollV2Input struct {
	SourceVideo       contract.AssetVersionRef `json:"source_video"`
	SourceVideoRights RightsStatus             `json:"source_video_rights"`
}

func (i ManualGamePrerollV2Input) Validate() error {
	if err := i.SourceVideo.Validate(); err != nil {
		return fmt.Errorf("source_video: %w", err)
	}
	if i.SourceVideoRights != RightsConfirmed {
		return fmt.Errorf("source_video_rights must be confirmed")
	}
	return nil
}

type GamePrerollAnalysis struct {
	Status          GamePrerollResourceStatus `json:"status"`
	Revision        int64                     `json:"revision,omitempty"`
	InputHash       string                    `json:"input_hash,omitempty"`
	PromptVersion   string                    `json:"prompt_version,omitempty"`
	ErrorCode       string                    `json:"error_code,omitempty"`
	ErrorMessage    string                    `json:"error_message,omitempty"`
	GameName        string                    `json:"game_name,omitempty"`
	GameplaySummary string                    `json:"gameplay_summary,omitempty"`
	Facts           []GameAnalysisFact        `json:"facts,omitempty"`
	Evidence        []GameEvidenceMoment      `json:"evidence,omitempty"`
	Unknowns        []string                  `json:"unknowns,omitempty"`
	SuggestedBrief  []GameBriefField          `json:"suggested_brief,omitempty"`
}

type GameFactProvenance string

const (
	GameProvenanceVideo  GameFactProvenance = "video_evidence"
	GameProvenanceAI     GameFactProvenance = "ai_inference"
	GameProvenanceManual GameFactProvenance = "manual"
)

type GameAnalysisFact struct {
	ID           string             `json:"id"`
	Label        string             `json:"label"`
	Value        string             `json:"value"`
	Provenance   GameFactProvenance `json:"provenance"`
	EvidenceRefs []string           `json:"evidence_refs"`
}
type GameBriefField struct {
	ID           string             `json:"id"`
	Key          string             `json:"key"`
	Label        string             `json:"label"`
	Value        string             `json:"value"`
	Provenance   GameFactProvenance `json:"provenance"`
	EvidenceRefs []string           `json:"evidence_refs"`
	Required     bool               `json:"required"`
}
type GameBriefVersion struct {
	ID               string           `json:"id"`
	Version          int64            `json:"version"`
	AnalysisRevision int64            `json:"analysis_revision"`
	Fields           []GameBriefField `json:"fields"`
	ConfirmedBy      string           `json:"confirmed_by"`
	ConfirmedAt      time.Time        `json:"confirmed_at"`
	ContentHash      string           `json:"content_hash"`
}

func (d GamePrerollDraft) AnalysisSuggestedBrief() []GameBriefField {
	return append([]GameBriefField{}, d.Analysis.SuggestedBrief...)
}

type GameEvidenceKind string

const (
	GameEvidenceSkillChoice  GameEvidenceKind = "skill_choice"
	GameEvidenceWaveProgress GameEvidenceKind = "wave_progress"
	GameEvidenceBattle       GameEvidenceKind = "battle"
	GameEvidenceGameplay     GameEvidenceKind = "gameplay"
	GameEvidenceOperation    GameEvidenceKind = "operation"
	GameEvidenceResult       GameEvidenceKind = "result"
	GameEvidenceReward       GameEvidenceKind = "reward"
	GameEvidenceUI           GameEvidenceKind = "ui"
)

type GameHookMechanism string

const (
	GameHookChoiceChallenge  GameHookMechanism = "choice_challenge"
	GameHookTacticalTradeoff GameHookMechanism = "tactical_tradeoff"
	GameHookWaveEscalation   GameHookMechanism = "wave_escalation"
	GameHookFailureReversal  GameHookMechanism = "failure_reversal"
	GameHookMergeUpgrade     GameHookMechanism = "merge_upgrade"
	GameHookRewardReveal     GameHookMechanism = "reward_reveal"
)

type GameEvidenceMoment struct {
	ID                string           `json:"id"`
	Kind              GameEvidenceKind `json:"kind"`
	StartMilliseconds int              `json:"start_milliseconds"`
	EndMilliseconds   int              `json:"end_milliseconds"`
	Description       string           `json:"description"`
	VerifiedCopy      []string         `json:"verified_copy"`
}

func (m GameEvidenceMoment) Validate() error {
	if strings.TrimSpace(m.ID) == "" || strings.TrimSpace(m.Description) == "" ||
		m.StartMilliseconds < 0 || m.EndMilliseconds <= m.StartMilliseconds {
		return fmt.Errorf("game evidence moment is incomplete")
	}
	switch m.Kind {
	case GameEvidenceSkillChoice, GameEvidenceWaveProgress, GameEvidenceBattle, GameEvidenceGameplay, GameEvidenceOperation, GameEvidenceResult, GameEvidenceReward, GameEvidenceUI:
	default:
		return fmt.Errorf("unsupported game evidence kind %q", m.Kind)
	}
	if err := validateStringList("verified_copy", m.VerifiedCopy, 12, 120); err != nil {
		return err
	}
	return nil
}

// ManualGamePrerollInput is the temporary Creative-owned fixture contract. A
// Strategy handoff can later project into the same immutable snapshot without
// changing candidate planning or generation.
type ManualGamePrerollInput struct {
	BriefID              string                   `json:"brief_id"`
	BriefVersion         int64                    `json:"brief_version"`
	BriefName            string                   `json:"brief_name"`
	GameName             string                   `json:"game_name"`
	GameplaySummary      string                   `json:"gameplay_summary"`
	SourceVideo          contract.AssetVersionRef `json:"source_video"`
	SourceVideoRights    RightsStatus             `json:"source_video_rights"`
	EvidenceMoments      []GameEvidenceMoment     `json:"evidence_moments"`
	AllowedMechanisms    []GameHookMechanism      `json:"allowed_mechanisms"`
	ProhibitedMechanisms []GameHookMechanism      `json:"prohibited_mechanisms"`
	SubtitleStyle        string                   `json:"subtitle_style"`
	HookStrength         int                      `json:"hook_strength"`
	PaceProfile          string                   `json:"pace_profile"`
}

func (i ManualGamePrerollInput) Validate() error {
	if strings.TrimSpace(i.BriefID) == "" || i.BriefVersion < 1 || strings.TrimSpace(i.BriefName) == "" ||
		strings.TrimSpace(i.GameName) == "" || utf8.RuneCountInString(strings.TrimSpace(i.GameplaySummary)) < 10 {
		return fmt.Errorf("manual game preroll brief, game name, and gameplay summary are required")
	}
	if err := i.SourceVideo.Validate(); err != nil {
		return fmt.Errorf("source_video: %w", err)
	}
	if i.SourceVideoRights != RightsConfirmed {
		return fmt.Errorf("source_video_rights must be confirmed")
	}
	if err := validateGameEvidence(i.EvidenceMoments); err != nil {
		return err
	}
	if err := validateGameMechanismPolicy(i.AllowedMechanisms, i.ProhibitedMechanisms); err != nil {
		return err
	}
	return (GamePrerollGenerationConfig{
		SubtitleStyle: i.SubtitleStyle,
		HookStrength:  i.HookStrength,
		PaceProfile:   i.PaceProfile,
	}).Validate()
}

type GamePrerollInputSnapshot struct {
	Source               IntakeSource             `json:"source"`
	SelectedRouteID      string                   `json:"selected_route_id"`
	BriefID              string                   `json:"brief_id"`
	BriefVersion         int64                    `json:"brief_version"`
	BriefName            string                   `json:"brief_name"`
	GameName             string                   `json:"game_name"`
	GameplaySummary      string                   `json:"gameplay_summary"`
	SourceVideo          contract.AssetVersionRef `json:"source_video,omitempty"`
	SourceVideoRights    RightsStatus             `json:"source_video_rights,omitempty"`
	CallToAction         string                   `json:"call_to_action"`
	EvidenceMoments      []GameEvidenceMoment     `json:"evidence_moments"`
	AllowedMechanisms    []GameHookMechanism      `json:"allowed_mechanisms"`
	ProhibitedMechanisms []GameHookMechanism      `json:"prohibited_mechanisms"`
}

type GamePrerollGenerationConfig struct {
	SubtitleStyle   string `json:"subtitle_style"`
	HookStrength    int    `json:"hook_strength"`
	PaceProfile     string `json:"pace_profile"`
	DurationSeconds int    `json:"duration_seconds,omitempty"`
	Channel         string `json:"channel,omitempty"`
	AspectRatio     string `json:"aspect_ratio,omitempty"`
	Resolution      string `json:"resolution,omitempty"`
	AudioPolicy     string `json:"audio_policy,omitempty"`
	CallToAction    string `json:"call_to_action,omitempty"`
}

func (c GamePrerollGenerationConfig) Validate() error {
	if c.SubtitleStyle != "high_contrast_dynamic" && c.SubtitleStyle != "brand_minimal" {
		return fmt.Errorf("unsupported subtitle_style")
	}
	if c.HookStrength < 1 || c.HookStrength > 5 {
		return fmt.Errorf("hook_strength must be between 1 and 5")
	}
	if c.DurationSeconds != 0 && (c.DurationSeconds < 6 || c.DurationSeconds > 10) {
		return fmt.Errorf("duration_seconds must be between 6 and 10")
	}
	if c.Channel != "" && c.Channel != string(ChannelDouyin) {
		return fmt.Errorf("unsupported game preroll channel")
	}
	if c.AspectRatio != "" && c.AspectRatio != "9:16" {
		return fmt.Errorf("unsupported game preroll aspect_ratio")
	}
	switch c.PaceProfile {
	case "punchy", "balanced":
		return nil
	default:
		return fmt.Errorf("unsupported pace_profile")
	}
}

type GameStoryboardBeat struct {
	StartMilliseconds int    `json:"start_milliseconds"`
	EndMilliseconds   int    `json:"end_milliseconds"`
	Visual            string `json:"visual"`
	Copy              string `json:"copy"`
	EvidenceMomentID  string `json:"evidence_moment_id"`
}

type GamePromptPackage struct {
	ContractVersion       string                      `json:"contract_version"`
	PromptCompilerVersion string                      `json:"prompt_compiler_version"`
	InputSnapshotHash     string                      `json:"input_snapshot_hash"`
	CandidateBatchID      string                      `json:"candidate_batch_id"`
	CandidateID           string                      `json:"candidate_id"`
	GenerationConfig      GamePrerollGenerationConfig `json:"generation_config"`
	DirectorSpec          map[string]string           `json:"director_spec"`
	NegativeConstraints   []string                    `json:"negative_constraints"`
	CompiledPrompt        string                      `json:"compiled_prompt"`
	ContentHash           string                      `json:"content_hash"`
}

type GamePrerollCandidate struct {
	ID                  string               `json:"id"`
	HookMechanism       GameHookMechanism    `json:"hook_mechanism"`
	ExecutionAngle      string               `json:"execution_angle"`
	PrimaryTestVariable string               `json:"primary_test_variable"`
	VariantHypothesis   string               `json:"variant_hypothesis"`
	Score               int                  `json:"score"`
	ScoreMeaning        string               `json:"score_meaning"`
	HookLine            string               `json:"hook_line"`
	EvidenceMomentIDs   []string             `json:"evidence_moment_ids"`
	Storyboard          []GameStoryboardBeat `json:"storyboard"`
	PromptPackage       GamePromptPackage    `json:"prompt_package"`
}

type GameCandidateBatch struct {
	ID                      string                      `json:"id"`
	Revision                int64                       `json:"revision"`
	PlannerVersion          string                      `json:"planner_version"`
	PromptCompilerVersion   string                      `json:"prompt_compiler_version"`
	GenerationConfig        GamePrerollGenerationConfig `json:"generation_config"`
	GeneratedCandidateCount int                         `json:"generated_candidate_count"`
	Candidates              []GamePrerollCandidate      `json:"candidates"`
	CreatedAt               time.Time                   `json:"created_at"`
}

type GameEvidenceFrameAsset struct {
	EvidenceMomentID        string                   `json:"evidence_moment_id"`
	SourceStartMilliseconds int                      `json:"source_start_milliseconds"`
	SourceEndMilliseconds   int                      `json:"source_end_milliseconds"`
	RepresentativeFrameMS   int                      `json:"representative_frame_milliseconds"`
	FrameAsset              contract.ProjectAssetRef `json:"frame_asset"`
	ExtractionVersion       string                   `json:"extraction_version"`
}

type GameEvidenceAssetSet struct {
	SourceVideo contract.AssetVersionRef `json:"source_video"`
	Status      string                   `json:"status"`
	Frames      []GameEvidenceFrameAsset `json:"frames"`
	ContentHash string                   `json:"content_hash"`
}

type GameVideoConditioningAsset struct {
	Role             string                   `json:"role"`
	EvidenceMomentID string                   `json:"evidence_moment_id"`
	Reference        contract.ProjectAssetRef `json:"reference"`
}

type GamePrerollGenerationSpec struct {
	ContractVersion    string                       `json:"contract_version"`
	TaskID             string                       `json:"task_id"`
	DraftRevision      int64                        `json:"draft_revision"`
	InputSnapshotHash  string                       `json:"input_snapshot_hash"`
	CandidateBatchID   string                       `json:"candidate_batch_id"`
	CandidateID        string                       `json:"candidate_id"`
	PromptPackageHash  string                       `json:"prompt_package_hash"`
	InputMode          string                       `json:"input_mode"`
	ConditioningAssets []GameVideoConditioningAsset `json:"conditioning_assets"`
	DurationSeconds    int                          `json:"duration_seconds"`
	AspectRatio        string                       `json:"aspect_ratio"`
	Resolution         string                       `json:"resolution"`
	AudioPolicy        string                       `json:"audio_policy"`
	Hash               string                       `json:"hash"`
}

type GamePrerollDraft struct {
	ContractVersion      string                      `json:"contract_version"`
	TaskID               string                      `json:"task_id"`
	Revision             int64                       `json:"revision"`
	SelectedRouteID      string                      `json:"selected_route_id"`
	InputSnapshot        GamePrerollInputSnapshot    `json:"input_snapshot"`
	InputHash            string                      `json:"input_hash"`
	Readiness            CreativeReadiness           `json:"readiness"`
	ActiveCandidateBatch *GameCandidateBatch         `json:"active_candidate_batch,omitempty"`
	Candidates           []GamePrerollCandidate      `json:"candidates"`
	SelectedCandidateID  string                      `json:"selected_candidate_id,omitempty"`
	EvidenceAssets       *GameEvidenceAssetSet       `json:"evidence_assets,omitempty"`
	GenerationSpec       *GamePrerollGenerationSpec  `json:"generation_spec,omitempty"`
	CreatedAt            time.Time                   `json:"created_at"`
	UpdatedAt            time.Time                   `json:"updated_at"`
	Stage                GamePrerollStage            `json:"stage,omitempty"`
	SourceMetadata       CreativeAssetSnapshot       `json:"source_metadata,omitempty"`
	SourceVideoRights    RightsConfirmation          `json:"source_video_rights,omitempty"`
	Analysis             GamePrerollAnalysis         `json:"analysis,omitempty"`
	GenerationConfig     GamePrerollGenerationConfig `json:"generation_config,omitempty"`
	OutputAsset          *contract.ProjectAssetRef   `json:"output_asset,omitempty"`
	ConfirmedBrief       *GameBriefVersion           `json:"confirmed_brief,omitempty"`
	LatestVideoAttemptID string                      `json:"latest_video_attempt_id,omitempty"`
	VideoError           *contract.JobError          `json:"video_error,omitempty"`
}

func (d GamePrerollDraft) Validate() error {
	if d.ContractVersion == GamePrerollV2ContractVersion && d.Stage == GamePrerollStageSourceReady {
		analysisCanRemainAtSource := d.Analysis.Status == GamePrerollResourceIdle || d.Analysis.Status == GamePrerollResourceRunning || d.Analysis.Status == GamePrerollResourceFailed
		if strings.TrimSpace(d.TaskID) == "" || d.Revision < 1 || d.SelectedRouteID != ManualGamePrerollV2RouteID || d.InputSnapshot.SourceVideo.Validate() != nil || d.SourceVideoRights.Validate() != nil || !analysisCanRemainAtSource || d.GenerationConfig.Validate() != nil || d.CreatedAt.IsZero() || d.UpdatedAt.IsZero() {
			return fmt.Errorf("game preroll V2 source workspace is incomplete")
		}
		return nil
	}
	if d.ContractVersion == GamePrerollV2ContractVersion {
		if strings.TrimSpace(d.TaskID) == "" || d.Revision < 1 || d.SelectedRouteID != ManualGamePrerollV2RouteID || d.SourceVideoRights.Validate() != nil || d.GenerationConfig.Validate() != nil || d.CreatedAt.IsZero() || d.UpdatedAt.IsZero() {
			return fmt.Errorf("game preroll V2 workspace is incomplete")
		}
		return nil
	}
	if (d.ContractVersion != "creative-game-preroll-draft/v1" && d.ContractVersion != "creative-game-preroll-draft/v2") || strings.TrimSpace(d.TaskID) == "" ||
		d.Revision < 1 || d.SelectedRouteID != ManualGamePrerollRouteID || strings.TrimSpace(d.InputHash) == "" ||
		!d.Readiness.PlanningReady || len(d.Candidates) != 3 || d.CreatedAt.IsZero() || d.UpdatedAt.IsZero() {
		return fmt.Errorf("game preroll draft is incomplete")
	}
	if d.ActiveCandidateBatch == nil || d.ActiveCandidateBatch.ID == "" ||
		d.ActiveCandidateBatch.Revision < 1 || len(d.ActiveCandidateBatch.Candidates) != len(d.Candidates) ||
		d.ActiveCandidateBatch.CreatedAt.IsZero() {
		return fmt.Errorf("game candidate batch is incomplete")
	}
	return nil
}

type gameCandidateOutline struct {
	Mechanism           GameHookMechanism
	ExecutionAngle      string
	PrimaryTestVariable string
	VariantHypothesis   string
	HookLine            string
	EvidenceMomentIDs   []string
	Beats               []GameStoryboardBeat
}

func planGamePrerollCandidateBatch(
	snapshot GamePrerollInputSnapshot,
	inputHash string,
	batchID string,
	revision int64,
	config GamePrerollGenerationConfig,
	now time.Time,
) (GameCandidateBatch, error) {
	outlines := defaultGameCandidateOutlines(snapshot)
	if !gameEvidenceIDsExist(snapshot.EvidenceMoments, []string{"skill_choice_1", "skill_choice_2", "wave_2"}) {
		outlines = genericGameCandidateOutlines(snapshot, config.DurationSeconds)
	}
	return compileGamePrerollCandidateBatch(
		snapshot,
		inputHash,
		batchID,
		revision,
		config,
		outlines,
		"game-preroll-deterministic-fallback/v1",
		now,
	)
}

func compileGamePrerollCandidateBatch(
	snapshot GamePrerollInputSnapshot,
	inputHash string,
	batchID string,
	revision int64,
	config GamePrerollGenerationConfig,
	outlines []gameCandidateOutline,
	plannerVersion string,
	now time.Time,
) (GameCandidateBatch, error) {
	if strings.TrimSpace(inputHash) == "" || strings.TrimSpace(batchID) == "" || revision < 1 || now.IsZero() {
		return GameCandidateBatch{}, fmt.Errorf("game candidate batch context is incomplete")
	}
	if strings.TrimSpace(plannerVersion) == "" {
		return GameCandidateBatch{}, fmt.Errorf("game planner version is required")
	}
	if err := config.Validate(); err != nil {
		return GameCandidateBatch{}, err
	}
	if err := validateGameEvidence(snapshot.EvidenceMoments); err != nil {
		return GameCandidateBatch{}, err
	}
	if err := validateGameMechanismPolicy(snapshot.AllowedMechanisms, snapshot.ProhibitedMechanisms); err != nil {
		return GameCandidateBatch{}, err
	}
	candidates := make([]GamePrerollCandidate, 0, 3)
	for _, outline := range outlines {
		if len(candidates) == 3 {
			break
		}
		if !containsGameHookMechanism(snapshot.AllowedMechanisms, outline.Mechanism) ||
			containsGameHookMechanism(snapshot.ProhibitedMechanisms, outline.Mechanism) ||
			!gameEvidenceIDsExist(snapshot.EvidenceMoments, outline.EvidenceMomentIDs) ||
			len(outline.Beats) < 2 || outline.Beats[0].EvidenceMomentID == outline.Beats[len(outline.Beats)-1].EvidenceMomentID {
			continue
		}
		candidateID := fmt.Sprintf("%s_candidate_%d", batchID, len(candidates)+1)
		compiledPrompt := compileGamePrerollPrompt(snapshot, outline, config)
		promptPackage := GamePromptPackage{
			ContractVersion:       "creative-game-preroll-prompt-package/v1",
			PromptCompilerVersion: "game-preroll-prompt-compiler/v1",
			InputSnapshotHash:     inputHash,
			CandidateBatchID:      batchID,
			CandidateID:           candidateID,
			GenerationConfig:      config,
			DirectorSpec: map[string]string{
				"truth_source":   "仅使用当前上传视频中有证据支持的真实玩法、界面、数值和结果",
				"composition":    "输出 9:16 竖屏；玩法主体、关键操作、结果反馈和核心 UI 保持可读",
				"continuity":     "三个 Beat 按当前证据时间关系连续表达，不虚构中间事件",
				"cta":            snapshot.CallToAction,
				"hook_mechanism": string(outline.Mechanism),
			},
			NegativeConstraints: []string{
				"不得生成当前素材中没有出现的失败、复活、合成、升级或奖励结果",
				"不得改写游戏名称、技能名称、数值、波次和核心 UI",
				"不得伪造试玩操作或宣称选择产生了素材未证明的结果",
			},
			CompiledPrompt: compiledPrompt,
		}
		hashValue := promptPackage
		hashValue.ContentHash = ""
		hash, err := contract.CanonicalJSONHash(hashValue)
		if err != nil {
			return GameCandidateBatch{}, err
		}
		promptPackage.ContentHash = "sha256:" + hash
		candidates = append(candidates, GamePrerollCandidate{
			ID: candidateID, HookMechanism: outline.Mechanism, ExecutionAngle: outline.ExecutionAngle,
			PrimaryTestVariable: outline.PrimaryTestVariable, VariantHypothesis: outline.VariantHypothesis,
			Score: 92 - len(candidates)*3, ScoreMeaning: "evidence_grounded_hook_relevance",
			HookLine: outline.HookLine, EvidenceMomentIDs: append([]string{}, outline.EvidenceMomentIDs...),
			Storyboard: append([]GameStoryboardBeat{}, outline.Beats...), PromptPackage: promptPackage,
		})
	}
	if len(candidates) != 3 {
		return GameCandidateBatch{}, fmt.Errorf("game planner produced fewer than three evidence-grounded candidates")
	}
	return GameCandidateBatch{
		ID: batchID, Revision: revision,
		PlannerVersion:        plannerVersion,
		PromptCompilerVersion: "game-preroll-prompt-compiler/v1",
		GenerationConfig:      config, GeneratedCandidateCount: len(candidates),
		Candidates: candidates, CreatedAt: now,
	}, nil
}

func defaultGameCandidateOutlines(snapshot GamePrerollInputSnapshot) []gameCandidateOutline {
	if !gameEvidenceIDsExist(snapshot.EvidenceMoments, []string{"skill_choice_1", "skill_choice_2", "wave_2"}) {
		return genericGameCandidateOutlines(snapshot, 6)
	}
	cta := snapshot.CallToAction
	return []gameCandidateOutline{
		{
			Mechanism: GameHookChoiceChallenge, ExecutionAngle: "skill_choice_countdown",
			PrimaryTestVariable: "choice_question", VariantHypothesis: "首秒直接提出三选一问题，能让观众立即理解玩法并参与判断。",
			HookLine:          "三个技能只能选一个，你会怎么选？",
			EvidenceMomentIDs: []string{"skill_choice_1", "skill_choice_2", "wave_2"},
			Beats: []GameStoryboardBeat{
				{StartMilliseconds: 0, EndMilliseconds: 2000, Visual: "从真实战斗压力快速切入第一次技能三选一。", Copy: "三个技能只能选一个", EvidenceMomentID: "skill_choice_1"},
				{StartMilliseconds: 2000, EndMilliseconds: 4000, Visual: "放大已核验技能名，保留原界面与数值。", Copy: "你会怎么选？", EvidenceMomentID: "skill_choice_2"},
				{StartMilliseconds: 4000, EndMilliseconds: 6000, Visual: "回到真实战斗并进入第 2/10 波，叠加 CTA。", Copy: cta, EvidenceMomentID: "wave_2"},
			},
		},
		{
			Mechanism: GameHookTacticalTradeoff, ExecutionAngle: "offense_or_capacity",
			PrimaryTestVariable: "named_skill_tradeoff", VariantHypothesis: "使用真实技能名建立策略取舍，比抽象挑战文案更能传达玩法深度。",
			HookLine:          "先加格子，还是先全体加攻？",
			EvidenceMomentIDs: []string{"skill_choice_1", "skill_choice_2", "wave_2"},
			Beats: []GameStoryboardBeat{
				{StartMilliseconds: 0, EndMilliseconds: 2000, Visual: "展示第一轮选择中的“获得格子”。", Copy: "先加格子？", EvidenceMomentID: "skill_choice_1"},
				{StartMilliseconds: 2000, EndMilliseconds: 4000, Visual: "动作匹配切到第二轮选择中的“全体加攻”。", Copy: "还是全体加攻？", EvidenceMomentID: "skill_choice_2"},
				{StartMilliseconds: 4000, EndMilliseconds: 6000, Visual: "用第 2/10 波真实画面结束，保留选择悬念并显示 CTA。", Copy: cta, EvidenceMomentID: "wave_2"},
			},
		},
		{
			Mechanism: GameHookWaveEscalation, ExecutionAngle: "next_wave_pressure",
			PrimaryTestVariable: "wave_pressure", VariantHypothesis: "先给下一波压力再回看技能选择，能把选择和继续闯关自然连起来。",
			HookLine:          "第 2 波马上开始，这个技能怎么选？",
			EvidenceMomentIDs: []string{"wave_2", "skill_choice_2", "skill_choice_1"},
			Beats: []GameStoryboardBeat{
				{StartMilliseconds: 0, EndMilliseconds: 2000, Visual: "用真实第 2/10 波画面建立下一波压力。", Copy: "第 2 波马上开始", EvidenceMomentID: "wave_2"},
				{StartMilliseconds: 2000, EndMilliseconds: 4000, Visual: "回切第二次真实技能三选一，突出可读选项。", Copy: "这个技能怎么选？", EvidenceMomentID: "skill_choice_2"},
				{StartMilliseconds: 4000, EndMilliseconds: 6000, Visual: "回扣第一次真实技能三选一并显示 CTA，不暗示未经验证的胜负。", Copy: cta, EvidenceMomentID: "skill_choice_1"},
			},
		},
	}
}

func planGenericGameCandidateBatch(snapshot GamePrerollInputSnapshot, inputHash, batchID string, revision int64, config GamePrerollGenerationConfig, now time.Time) (GameCandidateBatch, error) {
	if len(snapshot.EvidenceMoments) < 3 {
		return GameCandidateBatch{}, fmt.Errorf("three evidence moments are required for game candidate planning")
	}
	duration := config.DurationSeconds
	if duration == 0 {
		duration = 8
	}
	return compileGamePrerollCandidateBatch(snapshot, inputHash, batchID, revision, config, genericGameCandidateOutlines(snapshot, duration), "game-preroll-evidence-fallback/v2", now)
}

func genericGameCandidateOutlines(snapshot GamePrerollInputSnapshot, duration int) []gameCandidateOutline {
	e := snapshot.EvidenceMoments
	if len(e) < 3 {
		return nil
	}
	if duration < 1 {
		duration = 6
	}
	hookEnd := duration * 1000 / 4
	if hookEnd < 1000 {
		hookEnd = 1000
	}
	changeEnd := duration*1000 - 2000
	return []gameCandidateOutline{
		{Mechanism: GameHookChoiceChallenge, ExecutionAngle: "suspense_question", PrimaryTestVariable: "question", VariantHypothesis: "真实操作问题建立参与感。", HookLine: "这一步你会怎么选？", EvidenceMomentIDs: []string{e[0].ID, e[1].ID, e[2].ID}, Beats: []GameStoryboardBeat{{0, hookEnd, "展示真实操作入口", "你会怎么选？", e[0].ID}, {hookEnd, changeEnd, "展示操作后的真实反馈", "结果马上出现", e[1].ID}, {changeEnd, duration * 1000, "用真实结果和 CTA 收束", snapshot.CallToAction, e[2].ID}}},
		{Mechanism: GameHookTacticalTradeoff, ExecutionAngle: "operation_feedback", PrimaryTestVariable: "key_operation", VariantHypothesis: "真实操作与反馈的因果邻接建立信息缺口。", HookLine: "关键就在这一步，你看懂了吗？", EvidenceMomentIDs: []string{e[0].ID, e[1].ID, e[2].ID}, Beats: []GameStoryboardBeat{{0, hookEnd, "先展示真实反馈片段", "关键在哪一步？", e[1].ID}, {hookEnd, changeEnd, "回到真实操作过程", "注意这次操作", e[0].ID}, {changeEnd, duration * 1000, "展示后续真实反馈并收束", snapshot.CallToAction, e[2].ID}}},
		{Mechanism: GameHookWaveEscalation, ExecutionAngle: "visual_impact", PrimaryTestVariable: "impact", VariantHypothesis: "强反馈画面能在静音环境建立注意力。", HookLine: "这一操作，画面立刻变了！", EvidenceMomentIDs: []string{e[0].ID, e[1].ID, e[2].ID}, Beats: []GameStoryboardBeat{{0, hookEnd, "用最强真实反馈开场", "画面立刻变了", e[1].ID}, {hookEnd, changeEnd, "展示触发反馈的真实操作", "就是这一步", e[0].ID}, {changeEnd, duration * 1000, "保留真实 UI 并展示 CTA", snapshot.CallToAction, e[2].ID}}},
	}
}

func compileGamePrerollPrompt(snapshot GamePrerollInputSnapshot, outline gameCandidateOutline, config GamePrerollGenerationConfig) string {
	duration := config.DurationSeconds
	if duration == 0 {
		duration = 6
	}
	parts := []string{
		"效果广告游戏前贴，游戏：《" + snapshot.GameName + "》。",
		fmt.Sprintf("目标：制作独立 %d 秒、9:16、720p 的真实玩法前贴。", duration),
		"玩法事实：" + snapshot.GameplaySummary,
		"钩子：" + outline.HookLine,
		"执行：必须以授权实录为视觉事实源，保持游戏 UI、技能名、数值和波次可读；只允许剪辑、节奏、字幕、音效和 CTA 包装。",
		fmt.Sprintf("生成配置：字幕=%s，钩子强度=%d，节奏=%s。", config.SubtitleStyle, config.HookStrength, config.PaceProfile),
	}
	for _, beat := range outline.Beats {
		parts = append(parts, fmt.Sprintf(
			"%0.1f-%0.1f 秒：%s 字幕：%s；证据片段=%s。",
			float64(beat.StartMilliseconds)/1000,
			float64(beat.EndMilliseconds)/1000,
			beat.Visual,
			beat.Copy,
			beat.EvidenceMomentID,
		))
	}
	parts = append(parts,
		"结尾 CTA："+snapshot.CallToAction+"。",
		"严禁虚构当前视频没有证据支持的失败反转、合成升级、奖励、胜利或选择结果。",
	)
	return strings.Join(parts, "\n")
}

func validateGameEvidence(values []GameEvidenceMoment) error {
	if len(values) < 2 {
		return fmt.Errorf("at least two verified game evidence moments are required")
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if err := value.Validate(); err != nil {
			return err
		}
		if _, exists := seen[value.ID]; exists {
			return fmt.Errorf("game evidence moment %q is duplicated", value.ID)
		}
		seen[value.ID] = struct{}{}
	}
	return nil
}

func validateGameMechanismPolicy(allowed []GameHookMechanism, prohibited []GameHookMechanism) error {
	if len(allowed) < 3 {
		return fmt.Errorf("at least three allowed game hook mechanisms are required")
	}
	for _, mechanism := range allowed {
		if !validGameHookMechanism(mechanism) {
			return fmt.Errorf("unsupported game hook mechanism %q", mechanism)
		}
		if containsGameHookMechanism(prohibited, mechanism) {
			return fmt.Errorf("game hook mechanism %q cannot be both allowed and prohibited", mechanism)
		}
	}
	for _, mechanism := range prohibited {
		if !validGameHookMechanism(mechanism) {
			return fmt.Errorf("unsupported game hook mechanism %q", mechanism)
		}
	}
	return nil
}

func validGameHookMechanism(value GameHookMechanism) bool {
	switch value {
	case GameHookChoiceChallenge, GameHookTacticalTradeoff, GameHookWaveEscalation,
		GameHookFailureReversal, GameHookMergeUpgrade, GameHookRewardReveal:
		return true
	default:
		return false
	}
}

func containsGameHookMechanism(values []GameHookMechanism, target GameHookMechanism) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func gameEvidenceIDsExist(values []GameEvidenceMoment, ids []string) bool {
	known := make(map[string]struct{}, len(values))
	for _, value := range values {
		known[value.ID] = struct{}{}
	}
	for _, id := range ids {
		if _, exists := known[id]; !exists {
			return false
		}
	}
	return len(ids) > 0
}
