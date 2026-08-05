package creative

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	PerformanceModeShortDramaPreroll = "short_drama_preroll"
	ManualShortDramaPrerollRouteID   = "route_manual_short_drama_preroll_v1"
)

type ShortDramaHookStrategy string

const (
	ShortDramaConflictReversal   ShortDramaHookStrategy = "conflict_reversal"
	ShortDramaSuspenseReveal     ShortDramaHookStrategy = "suspense_reveal"
	ShortDramaIdentityContrast   ShortDramaHookStrategy = "identity_contrast"
	ShortDramaSellingPointBridge ShortDramaHookStrategy = "selling_point_bridge"
)

func (s ShortDramaHookStrategy) Validate() error {
	switch s {
	case ShortDramaConflictReversal, ShortDramaSuspenseReveal, ShortDramaIdentityContrast, ShortDramaSellingPointBridge:
		return nil
	default:
		return fmt.Errorf("unsupported short drama hook strategy %q", s)
	}
}

// ManualShortDramaPrerollInput is a Creative-owned local Brief. It deliberately
// carries no Strategy package identifiers: it is a temporary source that can
// later be replaced by a strategy-creative-handoff projection.
type ManualShortDramaPrerollInput struct {
	BriefID               string                     `json:"brief_id"`
	BriefVersion          int64                      `json:"brief_version"`
	BriefName             string                     `json:"brief_name"`
	StoryTitle            string                     `json:"story_title"`
	Synopsis              string                     `json:"synopsis"`
	ReviewedSellingPoints []string                   `json:"reviewed_selling_points"`
	OpeningLine           string                     `json:"opening_line,omitempty"`
	HookStrategy          ShortDramaHookStrategy     `json:"hook_strategy"`
	SubtitleStyle         string                     `json:"subtitle_style"`
	Transition            string                     `json:"transition"`
	HookStrength          int                        `json:"hook_strength"`
	PaceProfile           string                     `json:"pace_profile,omitempty"`
	CharacterReferences   []contract.AssetVersionRef `json:"character_references"`
}

func (i ManualShortDramaPrerollInput) Validate() error {
	if strings.TrimSpace(i.BriefID) == "" || i.BriefVersion < 1 || strings.TrimSpace(i.BriefName) == "" ||
		strings.TrimSpace(i.StoryTitle) == "" || utf8.RuneCountInString(strings.TrimSpace(i.StoryTitle)) > 120 ||
		utf8.RuneCountInString(strings.TrimSpace(i.Synopsis)) < 40 || utf8.RuneCountInString(i.Synopsis) > 2000 {
		return fmt.Errorf("manual short drama brief, title, and a synopsis of at least 40 characters are required")
	}
	if err := validateStringList("reviewed_selling_points", i.ReviewedSellingPoints, 12, 300); err != nil || len(i.ReviewedSellingPoints) == 0 {
		if err != nil {
			return err
		}
		return fmt.Errorf("at least one reviewed_selling_point is required")
	}
	if err := i.HookStrategy.Validate(); err != nil {
		return err
	}
	if i.SubtitleStyle != "high_contrast_dynamic" && i.SubtitleStyle != "brand_minimal" {
		return fmt.Errorf("unsupported subtitle_style")
	}
	if i.Transition != "hard_cut" && i.Transition != "action_match" && i.Transition != "audio_bridge" {
		return fmt.Errorf("unsupported transition")
	}
	if i.HookStrength < 1 || i.HookStrength > 5 {
		return fmt.Errorf("hook_strength must be between 1 and 5")
	}
	if err := validateShortDramaPaceProfile(normalizeShortDramaPaceProfile(i.PaceProfile)); err != nil {
		return err
	}
	for _, reference := range i.CharacterReferences {
		if err := reference.Validate(); err != nil {
			return fmt.Errorf("character_reference: %w", err)
		}
	}
	return nil
}

type ShortDramaPrerollInputSnapshot struct {
	Source                IntakeSource               `json:"source"`
	SelectedRouteID       string                     `json:"selected_route_id"`
	BriefID               string                     `json:"brief_id"`
	BriefVersion          int64                      `json:"brief_version"`
	BriefName             string                     `json:"brief_name"`
	StoryTitle            string                     `json:"story_title"`
	Synopsis              string                     `json:"synopsis"`
	ReviewedSellingPoints []string                   `json:"reviewed_selling_points"`
	OpeningLine           string                     `json:"opening_line,omitempty"`
	HookStrategy          ShortDramaHookStrategy     `json:"hook_strategy"`
	SubtitleStyle         string                     `json:"subtitle_style"`
	Transition            string                     `json:"transition"`
	HookStrength          int                        `json:"hook_strength"`
	PaceProfile           string                     `json:"pace_profile,omitempty"`
	CallToAction          string                     `json:"call_to_action"`
	CharacterReferences   []contract.AssetVersionRef `json:"character_references"`
}

type ShortDramaStoryboardBeat struct {
	StartSeconds int    `json:"start_seconds"`
	EndSeconds   int    `json:"end_seconds"`
	Visual       string `json:"visual"`
	Copy         string `json:"copy"`
}

type ShortDramaPrerollCandidate struct {
	ID                  string                     `json:"id"`
	HookStrategy        ShortDramaHookStrategy     `json:"hook_strategy"`
	ExecutionAngle      string                     `json:"execution_angle"`
	PrimaryTestVariable string                     `json:"primary_test_variable"`
	PacingProfile       string                     `json:"pacing_profile"`
	VisualGrammar       string                     `json:"visual_grammar"`
	VariantHypothesis   string                     `json:"variant_hypothesis"`
	Score               int                        `json:"score"`
	ScoreMeaning        string                     `json:"score_meaning"`
	Evidence            []string                   `json:"evidence"`
	HookLine            string                     `json:"hook_line"`
	Voiceover           string                     `json:"voiceover"`
	Storyboard          []ShortDramaStoryboardBeat `json:"storyboard"`
	VisualIntent        string                     `json:"visual_intent"`
	TransitionLine      string                     `json:"transition_line"`
	PromptPackage       ShortDramaPromptPackage    `json:"prompt_package"`
}

type ShortDramaGenerationConfig struct {
	SubtitleStyle string `json:"subtitle_style"`
	HookStrength  int    `json:"hook_strength"`
	PaceProfile   string `json:"pace_profile"`
}

func (c ShortDramaGenerationConfig) Validate() error {
	if c.SubtitleStyle != "high_contrast_dynamic" && c.SubtitleStyle != "brand_minimal" {
		return fmt.Errorf("unsupported subtitle_style")
	}
	if c.HookStrength < 1 || c.HookStrength > 5 {
		return fmt.Errorf("hook_strength must be between 1 and 5")
	}
	return validateShortDramaPaceProfile(c.PaceProfile)
}

type ShortDramaSubtitleSpec struct {
	Mode             string `json:"mode"`
	MaxLines         int    `json:"max_lines"`
	SafeArea         string `json:"safe_area"`
	KeywordEmphasis  bool   `json:"keyword_emphasis"`
	AnimationDensity string `json:"animation_density"`
	ContrastPolicy   string `json:"contrast_policy"`
}

type ShortDramaPromptPackage struct {
	ContractVersion       string                     `json:"contract_version"`
	PromptCompilerVersion string                     `json:"prompt_compiler_version"`
	InputSnapshotHash     string                     `json:"input_snapshot_hash"`
	CandidateBatchID      string                     `json:"candidate_batch_id"`
	CandidateID           string                     `json:"candidate_id"`
	GenerationConfig      ShortDramaGenerationConfig `json:"generation_config"`
	SubtitleSpec          ShortDramaSubtitleSpec     `json:"subtitle_spec"`
	DirectorSpec          map[string]string          `json:"director_spec"`
	NegativeConstraints   []string                   `json:"negative_constraints"`
	CompiledPrompt        string                     `json:"compiled_prompt"`
	ContentHash           string                     `json:"content_hash"`
}

type ShortDramaCandidateBatch struct {
	ID                      string                       `json:"id"`
	Revision                int64                        `json:"revision"`
	PlannerVersion          string                       `json:"planner_version"`
	PromptCompilerVersion   string                       `json:"prompt_compiler_version"`
	DiversityNonce          string                       `json:"diversity_nonce"`
	GenerationConfig        ShortDramaGenerationConfig   `json:"generation_config"`
	VariationIntent         string                       `json:"variation_intent"`
	GeneratedCandidateCount int                          `json:"generated_candidate_count"`
	Candidates              []ShortDramaPrerollCandidate `json:"candidates"`
	CreatedAt               time.Time                    `json:"created_at"`
}

type ShortDramaPrerollDraft struct {
	ContractVersion      string                         `json:"contract_version"`
	TaskID               string                         `json:"task_id"`
	Revision             int64                          `json:"revision"`
	SelectedRouteID      string                         `json:"selected_route_id"`
	InputSnapshot        ShortDramaPrerollInputSnapshot `json:"input_snapshot"`
	InputHash            string                         `json:"input_hash"`
	Readiness            CreativeReadiness              `json:"readiness"`
	ActiveCandidateBatch *ShortDramaCandidateBatch      `json:"active_candidate_batch,omitempty"`
	Candidates           []ShortDramaPrerollCandidate   `json:"candidates"`
	SelectedCandidateID  string                         `json:"selected_candidate_id,omitempty"`
	CreatedAt            time.Time                      `json:"created_at"`
	UpdatedAt            time.Time                      `json:"updated_at"`
}

func (d ShortDramaPrerollDraft) Validate() error {
	if d.ContractVersion != "creative-short-drama-preroll-draft/v1" || strings.TrimSpace(d.TaskID) == "" ||
		d.Revision < 1 || d.SelectedRouteID != ManualShortDramaPrerollRouteID || strings.TrimSpace(d.InputHash) == "" ||
		!d.Readiness.PlanningReady || d.Candidates == nil || d.CreatedAt.IsZero() || d.UpdatedAt.IsZero() {
		return fmt.Errorf("short drama preroll draft is incomplete")
	}
	if d.ActiveCandidateBatch != nil {
		if d.ActiveCandidateBatch.ID == "" || d.ActiveCandidateBatch.Revision < 1 ||
			d.ActiveCandidateBatch.GeneratedCandidateCount < len(d.ActiveCandidateBatch.Candidates) ||
			len(d.ActiveCandidateBatch.Candidates) != len(d.Candidates) || d.ActiveCandidateBatch.CreatedAt.IsZero() {
			return fmt.Errorf("short drama candidate batch is incomplete")
		}
	}
	return nil
}

func planShortDramaCandidates(snapshot ShortDramaPrerollInputSnapshot, inputHash string) ([]ShortDramaPrerollCandidate, error) {
	batch, err := planShortDramaCandidateBatch(
		snapshot,
		inputHash,
		"short_drama_legacy_batch",
		1,
		shortDramaGenerationConfig(snapshot),
		"balanced",
		time.Unix(1, 0).UTC(),
	)
	if err != nil {
		return nil, err
	}
	return batch.Candidates, nil
}

func planShortDramaCandidateBatch(
	snapshot ShortDramaPrerollInputSnapshot,
	inputHash string,
	batchID string,
	revision int64,
	config ShortDramaGenerationConfig,
	variationIntent string,
	now time.Time,
) (ShortDramaCandidateBatch, error) {
	snapshot.SubtitleStyle = config.SubtitleStyle
	snapshot.HookStrength = config.HookStrength
	snapshot.PaceProfile = config.PaceProfile
	variants := orderShortDramaExecutionVariants(shortDramaExecutionVariants(snapshot), variationIntent, revision)
	return compileShortDramaCandidateBatch(
		snapshot,
		inputHash,
		batchID,
		revision,
		config,
		variationIntent,
		variants,
		"short-drama-deterministic-planner/v3",
		now,
	)
}

func compileShortDramaCandidateBatch(
	snapshot ShortDramaPrerollInputSnapshot,
	inputHash string,
	batchID string,
	revision int64,
	config ShortDramaGenerationConfig,
	variationIntent string,
	variants []shortDramaExecutionVariant,
	plannerVersion string,
	now time.Time,
) (ShortDramaCandidateBatch, error) {
	snapshot.SubtitleStyle = config.SubtitleStyle
	snapshot.HookStrength = config.HookStrength
	snapshot.PaceProfile = config.PaceProfile
	generated := make([]ShortDramaPrerollCandidate, 0, len(variants))
	for index, variant := range variants {
		id := fmt.Sprintf("%s_candidate_%d", batchID, index+1)
		beats := []ShortDramaStoryboardBeat{
			{StartSeconds: 0, EndSeconds: 2, Visual: variant.OpeningVisual, Copy: variant.HookLine},
			{StartSeconds: 2, EndSeconds: 4, Visual: variant.MiddleVisual, Copy: variant.MiddleCopy},
			{StartSeconds: 4, EndSeconds: 6, Visual: variant.ClosingVisual, Copy: snapshot.CallToAction},
		}
		compiled := compileShortDramaPrompt(snapshot, variant.Label, variant.HookLine, beats)
		generationConfig := shortDramaGenerationConfig(snapshot)
		generationConfig.PaceProfile = config.PaceProfile
		subtitleSpec := shortDramaSubtitleSpec(snapshot.SubtitleStyle)
		prompt := ShortDramaPromptPackage{
			ContractVersion: "creative-short-drama-prompt-package/v1", PromptCompilerVersion: "short-drama-prompt-compiler/v2",
			InputSnapshotHash: inputHash, CandidateBatchID: batchID, CandidateID: id,
			GenerationConfig: generationConfig,
			SubtitleSpec:     subtitleSpec,
			DirectorSpec: map[string]string{
				"execution_angle":             variant.Label,
				"subject_and_continuity":      "沿用已确认短剧人物、服装、场景与关系；不得改变人物身份。",
				"selling_point_dramatization": strings.Join(snapshot.ReviewedSellingPoints, "；"),
				"scene_and_tone":              "竖屏都市短剧，情绪紧凑，前一秒建立信息缺口。",
				"camera_language":             variant.CameraLanguage,
				"pacing_profile":              shortDramaPaceInstruction(config.PaceProfile),
				"audio_spec":                  "可生成氛围音；默认静音可理解。",
				"post_production_constraints": shortDramaSubtitleInstruction(subtitleSpec),
			},
			NegativeConstraints: []string{"不得逐字复制正片首句", "不得虚构未确认剧情事实", "不得泄露完整结局"},
			CompiledPrompt:      compiled,
		}
		hashValue := prompt
		hashValue.ContentHash = ""
		hash, err := contract.CanonicalJSONHash(hashValue)
		if err != nil {
			return ShortDramaCandidateBatch{}, err
		}
		prompt.ContentHash = "sha256:" + hash
		evidence := append([]string{}, snapshot.ReviewedSellingPoints...)
		if strings.TrimSpace(variant.GroundingQuote) != "" {
			evidence = append(evidence, variant.GroundingQuote)
		}
		generated = append(generated, ShortDramaPrerollCandidate{
			ID: id, HookStrategy: snapshot.HookStrategy, ExecutionAngle: variant.ID,
			PrimaryTestVariable: variant.PrimaryTestVariable, PacingProfile: variant.PacingProfile,
			VisualGrammar: variant.VisualGrammar, VariantHypothesis: variant.VariantHypothesis,
			Score: 88 - index*3, ScoreMeaning: "editorial_quality_heuristic",
			Evidence: evidence, HookLine: variant.HookLine,
			Voiceover: variant.HookLine + " " + snapshot.CallToAction, Storyboard: beats, VisualIntent: variant.OpeningVisual,
			TransitionLine: variant.TransitionLine, PromptPackage: prompt,
		})
	}
	const visibleCandidateCount = 3
	if len(generated) < visibleCandidateCount {
		return ShortDramaCandidateBatch{}, fmt.Errorf("short drama planner produced fewer than three qualified candidates")
	}
	candidates := append([]ShortDramaPrerollCandidate{}, generated[:visibleCandidateCount]...)
	return ShortDramaCandidateBatch{
		ID: batchID, Revision: revision,
		PlannerVersion: plannerVersion, PromptCompilerVersion: "short-drama-prompt-compiler/v2",
		DiversityNonce: batchID, GenerationConfig: config, VariationIntent: variationIntent,
		GeneratedCandidateCount: len(generated), Candidates: candidates, CreatedAt: now,
	}, nil
}

func orderShortDramaExecutionVariants(
	variants []shortDramaExecutionVariant,
	variationIntent string,
	revision int64,
) []shortDramaExecutionVariant {
	byID := make(map[string]shortDramaExecutionVariant, len(variants))
	for _, variant := range variants {
		byID[variant.ID] = variant
	}
	var order []string
	switch variationIntent {
	case "more_visual":
		order = []string{"action_reveal", "result_first", "reaction_escalation", "dialogue_confrontation"}
	case "more_dialogue":
		order = []string{"dialogue_confrontation", "reaction_escalation", "result_first", "action_reveal"}
	case "more_suspense":
		order = []string{"reaction_escalation", "result_first", "action_reveal", "dialogue_confrontation"}
	default:
		order = []string{"dialogue_confrontation", "action_reveal", "reaction_escalation", "result_first"}
		if revision > 1 {
			offset := int((revision - 1) % int64(len(order)))
			order = append(append([]string{}, order[offset:]...), order[:offset]...)
		}
	}
	ordered := make([]shortDramaExecutionVariant, 0, len(order))
	for _, id := range order {
		if variant, ok := byID[id]; ok {
			ordered = append(ordered, variant)
		}
	}
	return ordered
}

type shortDramaExecutionVariant struct {
	ID                  string
	Label               string
	HookLine            string
	OpeningVisual       string
	MiddleVisual        string
	MiddleCopy          string
	ClosingVisual       string
	TransitionLine      string
	CameraLanguage      string
	PrimaryTestVariable string
	PacingProfile       string
	VisualGrammar       string
	VariantHypothesis   string
	GroundingQuote      string
}

func shortDramaExecutionVariants(snapshot ShortDramaPrerollInputSnapshot) []shortDramaExecutionVariant {
	hooks := shortDramaHooks(snapshot)
	variants := []shortDramaExecutionVariant{
		{
			ID: "dialogue_confrontation", Label: "台词对峙",
			HookLine:            hooks[0],
			OpeningVisual:       "极近景锁定主角直视质疑者的瞬间，以短促正反打建立公开对峙。",
			MiddleVisual:        "镜头保持人物轴线，切到质疑者停顿和主角不退让的表情。",
			MiddleCopy:          "她没有退让，一句反问让质疑声突然停住。",
			ClosingVisual:       "停在主角直视对方的表情，以清晰行动号召收束，不泄露结局。",
			TransitionLine:      "她接下来会说什么？",
			CameraLanguage:      "0–2 秒用台词和正反打点燃冲突，2–4 秒捕捉对峙停顿，4–6 秒以主角表情和行动号召收束。",
			PrimaryTestVariable: "conflict_dialogue", PacingProfile: "punchy",
			VisualGrammar: "dialogue_shot_reverse_shot", VariantHypothesis: "直接语言冲突能在首秒快速建立注意力。",
		},
		{
			ID: "action_reveal", Label: "关键动作",
			HookLine:            hooks[1],
			OpeningVisual:       "从手部或关键动作特写切入，不先展示完整结果，让观众追问动作背后的事实。",
			MiddleVisual:        "快速推近主角完成关键动作的瞬间，保持人物、服装与场景连续。",
			MiddleCopy:          "她不争辩，只把关键事实推到镜头中央。",
			ClosingVisual:       "停在事实将揭未揭的画面，以行动号召收束，不新增未经确认的证据。",
			TransitionLine:      "那个关键事实究竟是什么？",
			CameraLanguage:      "0–2 秒以手部或动作特写制造信息缺口，2–4 秒推近关键动作，4–6 秒在事实揭晓前用行动号召收束。",
			PrimaryTestVariable: "action_reveal", PacingProfile: "balanced",
			VisualGrammar: "action_macro_push", VariantHypothesis: "关键动作比解释性台词更容易形成可执行的视觉悬念。",
		},
		{
			ID: "reaction_escalation", Label: "群体反应",
			HookLine:            hooks[2],
			OpeningVisual:       "中景横移扫过旁观者的轻视表情，再落到保持冷静的主角身上。",
			MiddleVisual:        "连续切换旁观者表情，让态度从轻视转为错愕，暂不解释反转原因。",
			MiddleCopy:          "镜头扫过众人，轻视正在变成惊讶。",
			ClosingVisual:       "停在全场同步看向主角的瞬间，以行动号召收束，不泄露完整结局。",
			TransitionLine:      "他们为什么突然改变态度？",
			CameraLanguage:      "0–2 秒建立群体压力，2–4 秒用反应镜头放大态度反转，4–6 秒停在集体注视并以行动号召收束。",
			PrimaryTestVariable: "social_reaction", PacingProfile: "suspense_hold",
			VisualGrammar: "group_reaction_montage", VariantHypothesis: "旁观者态度变化能够放大身份反转的信息缺口。",
		},
		{
			ID: "result_first", Label: "结果先行",
			HookLine:            hooks[3],
			OpeningVisual:       "先展示众人态度已经反转的结果，再快速回到主角被轻视的前一刻，形成原因缺口。",
			MiddleVisual:        "用一个明确但不泄露结局的结果画面，与主角此前处境形成强反差。",
			MiddleCopy:          "他们已经改变了态度，但真正的原因还没有揭开。",
			ClosingVisual:       "停在结果与原因之间的悬念点，以点击观看短剧的行动号召收束。",
			TransitionLine:      "她究竟做了什么？",
			CameraLanguage:      "0–1 秒先给态度反转结果，1–4 秒回切此前冲突，4–6 秒停在原因揭晓前并给出行动号召。",
			PrimaryTestVariable: "result_first", PacingProfile: "punchy",
			VisualGrammar: "result_before_cause", VariantHypothesis: "先呈现反转结果能够让观众追问原因并点击观看。",
		},
	}
	point := firstShortDramaSellingPoint(snapshot)
	storyMoment := shortDramaSynopsisMoment(snapshot.Synopsis)
	variants[0].OpeningVisual = fmt.Sprintf("以已审核剧情事实“%s”对应的人物行动切入，不新增输入之外的角色或关系。", point)
	variants[0].MiddleVisual = fmt.Sprintf("保持同一人物与场景，呈现“%s”引发的直接反应，但不揭露结局。", point)
	variants[0].MiddleCopy = fmt.Sprintf("%s已经发生，真正原因还没有说出。", point)
	variants[1].OpeningVisual = fmt.Sprintf("以“%s”中的关键物件或动作特写切入，只使用梗概已确认的事实。", storyMoment)
	variants[1].MiddleVisual = fmt.Sprintf("镜头推进“%s”对应的线索，不补写未经审核的证据。", point)
	variants[1].MiddleCopy = fmt.Sprintf("%s，这条线索究竟指向哪里？", storyMoment)
	variants[2].OpeningVisual = fmt.Sprintf("从“%s”发生后的克制反应切入，避免套用无依据的围观者或身份反转。", point)
	variants[2].MiddleVisual = fmt.Sprintf("用连续反应镜头强调“%s”造成的信息缺口，不虚构剧情结果。", point)
	variants[2].MiddleCopy = fmt.Sprintf("%s之后，真相反而更难解释。", shortDramaPointSubject(point))
	variants[3].OpeningVisual = fmt.Sprintf("先呈现“%s”已经发生的结果，再回到它出现之前的剧情状态。", point)
	variants[3].MiddleVisual = fmt.Sprintf("以《%s》的已审核剧情事实制造原因缺口，不提前公布答案。", snapshot.StoryTitle)
	variants[3].MiddleCopy = fmt.Sprintf("%s已经出现，但真正原因仍未揭开。", shortDramaPointSubject(point))
	return variants
}

func shortDramaHooks(snapshot ShortDramaPrerollInputSnapshot) [4]string {
	title := strings.TrimSpace(snapshot.StoryTitle)
	point := firstShortDramaSellingPoint(snapshot)
	pointSubject := shortDramaPointSubject(point)
	storyMoment := shortDramaSynopsisMoment(snapshot.Synopsis)
	switch snapshot.HookStrategy {
	case ShortDramaSuspenseReveal:
		return [4]string{
			fmt.Sprintf("《%s》里，%s为何此时出现？", title, pointSubject),
			fmt.Sprintf("%s，这条线索意味着什么？", storyMoment),
			fmt.Sprintf("%s出现后，真相反而更难解释。", pointSubject),
			fmt.Sprintf("《%s》的答案，就藏在下一幕。", title),
		}
	case ShortDramaIdentityContrast:
		return [4]string{
			fmt.Sprintf("%s发生后，所有人重新看向了主角。", point),
			fmt.Sprintf("%s，真正被隐瞒的身份即将揭开。", storyMoment),
			fmt.Sprintf("《%s》里，上一秒的轻视为什么突然停止？", title),
			fmt.Sprintf("%s已经出现，但主角的真正身份还没公开。", point),
		}
	case ShortDramaSellingPointBridge:
		return [4]string{
			fmt.Sprintf("%s，正是《%s》的关键转折。", point, title),
			fmt.Sprintf("%s，下一秒的结果比解释更直接。", storyMoment),
			fmt.Sprintf("%s出现后，故事才真正开始。", point),
			fmt.Sprintf("《%s》已经给出线索，答案还差最后一步。", title),
		}
	default:
		return [4]string{
			fmt.Sprintf("%s发生后，所有人的判断都变了。", point),
			fmt.Sprintf("%s，关键事实终于摆到了众人面前。", storyMoment),
			fmt.Sprintf("《%s》里，%s会带来怎样的反转？", title, point),
			fmt.Sprintf("%s已经出现，真正的反转还没有开始。", point),
		}
	}
}

func shortDramaPointSubject(point string) string {
	value := strings.TrimSpace(point)
	for _, suffix := range []string{"已经出现", "首次出现", "突然出现", "出现"} {
		if trimmed := strings.TrimSpace(strings.TrimSuffix(value, suffix)); trimmed != value && trimmed != "" {
			return trimmed
		}
	}
	return value
}

func firstShortDramaSellingPoint(snapshot ShortDramaPrerollInputSnapshot) string {
	for _, point := range snapshot.ReviewedSellingPoints {
		if value := strings.TrimSpace(point); value != "" {
			return value
		}
	}
	return strings.TrimSpace(snapshot.StoryTitle)
}

func shortDramaSynopsisMoment(synopsis string) string {
	value := strings.TrimSpace(synopsis)
	normalized := strings.NewReplacer("。", "，", "！", "，", "？", "，", "；", "，").Replace(value)
	parts := strings.Split(normalized, "，")
	for _, part := range parts {
		candidate := strings.TrimSpace(part)
		if len([]rune(candidate)) > len([]rune(value)) || value == strings.TrimSpace(synopsis) {
			value = candidate
		}
	}
	const maxRunes = 24
	runes := []rune(value)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes])
	}
	return value
}

func compileShortDramaPrompt(snapshot ShortDramaPrerollInputSnapshot, executionAngle string, hook string, beats []ShortDramaStoryboardBeat) string {
	subtitleSpec := shortDramaSubtitleSpec(snapshot.SubtitleStyle)
	parts := []string{
		"短剧前贴效果广告", "标题：" + snapshot.StoryTitle, "已审核剧情事实：" + snapshot.Synopsis,
		"执行角度：" + executionAngle, "钩子：" + hook, "卖点：" + strings.Join(snapshot.ReviewedSellingPoints, "；"),
		"规格：9:16，独立 6 秒广告成片。",
		"钩子强度：" + shortDramaHookStrengthInstruction(snapshot.HookStrength),
		"节奏：" + shortDramaPaceInstruction(snapshot.PaceProfile),
		"字幕：" + shortDramaSubtitleInstruction(subtitleSpec),
		"限制：人物、服装和场景连续；不得逐字复制正片台词；不得泄露结局；静音可理解。",
	}
	for _, beat := range beats {
		parts = append(parts, fmt.Sprintf("%d-%d 秒：%s；字幕/旁白：%s", beat.StartSeconds, beat.EndSeconds, beat.Visual, beat.Copy))
	}
	return strings.Join(parts, "。")
}

func shortDramaGenerationConfig(snapshot ShortDramaPrerollInputSnapshot) ShortDramaGenerationConfig {
	return ShortDramaGenerationConfig{
		SubtitleStyle: snapshot.SubtitleStyle,
		HookStrength:  snapshot.HookStrength,
		PaceProfile:   normalizeShortDramaPaceProfile(snapshot.PaceProfile),
	}
}

func normalizeShortDramaPaceProfile(profile string) string {
	if strings.TrimSpace(profile) == "" {
		return "auto"
	}
	return profile
}

func validateShortDramaPaceProfile(profile string) error {
	switch profile {
	case "auto", "punchy", "balanced", "suspense_hold":
		return nil
	default:
		return fmt.Errorf("unsupported pace_profile")
	}
}

func shortDramaPaceInstruction(profile string) string {
	switch normalizeShortDramaPaceProfile(profile) {
	case "punchy":
		return "强节奏快切，前两秒连续推进信息，每个镜头只承载一个动作或一句短字幕"
	case "balanced":
		return "三段均衡推进，冲突、信息缺口和 CTA 各自保留清晰阅读时间"
	case "suspense_hold":
		return "悬念停顿，在关键事实揭晓前保留短暂停顿与反应镜头，再用 CTA 收束"
	default:
		return "根据候选的主测试变量自动匹配快切、均衡推进或悬念停顿"
	}
}

func shortDramaSubtitleSpec(style string) ShortDramaSubtitleSpec {
	if style == "brand_minimal" {
		return ShortDramaSubtitleSpec{
			Mode: "brand_minimal", MaxLines: 1, SafeArea: "vertical_center_safe",
			KeywordEmphasis: false, AnimationDensity: "restrained", ContrastPolicy: "brand_token_readable",
		}
	}
	return ShortDramaSubtitleSpec{
		Mode: "high_contrast_dynamic", MaxLines: 2, SafeArea: "vertical_center_safe",
		KeywordEmphasis: true, AnimationDensity: "high_in_first_two_seconds", ContrastPolicy: "high_contrast_outline",
	}
}

func shortDramaSubtitleInstruction(spec ShortDramaSubtitleSpec) string {
	if spec.Mode == "brand_minimal" {
		return "单行克制字幕，使用统一品牌色与高可读对比，保持竖屏安全区，不遮挡人物面部和关键动作"
	}
	return "最多两行高对比动态字幕，关键词分段强调，前两秒动画密度较高，保持竖屏安全区，不遮挡人物面部和关键动作"
}

func shortDramaHookStrengthInstruction(strength int) string {
	switch strength {
	case 1:
		return "1.5 秒内建立信息缺口，克制铺垫，使用 2 至 3 个镜头和暗示式文案"
	case 2:
		return "1.2 秒内建立信息缺口，稳步加速，使用 3 个镜头和较直接文案"
	case 3:
		return "1.0 秒内建立信息缺口，三段均衡推进，文案直接但保留悬念"
	case 5:
		return "0.5 秒内建立强信息缺口，使用 4 至 5 个短镜头和强声音重音，不放宽事实与合规限制"
	default:
		return "0.8 秒内直接建立冲突，使用 3 至 4 个快切镜头、直接文案和关键音效"
	}
}
