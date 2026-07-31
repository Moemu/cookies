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
	ID             string                     `json:"id"`
	HookStrategy   ShortDramaHookStrategy     `json:"hook_strategy"`
	ExecutionAngle string                     `json:"execution_angle"`
	Score          int                        `json:"score"`
	ScoreMeaning   string                     `json:"score_meaning"`
	Evidence       []string                   `json:"evidence"`
	HookLine       string                     `json:"hook_line"`
	Voiceover      string                     `json:"voiceover"`
	Storyboard     []ShortDramaStoryboardBeat `json:"storyboard"`
	VisualIntent   string                     `json:"visual_intent"`
	TransitionLine string                     `json:"transition_line"`
	PromptPackage  ShortDramaPromptPackage    `json:"prompt_package"`
}

type ShortDramaPromptPackage struct {
	ContractVersion     string            `json:"contract_version"`
	InputSnapshotHash   string            `json:"input_snapshot_hash"`
	CandidateID         string            `json:"candidate_id"`
	DirectorSpec        map[string]string `json:"director_spec"`
	NegativeConstraints []string          `json:"negative_constraints"`
	CompiledPrompt      string            `json:"compiled_prompt"`
	ContentHash         string            `json:"content_hash"`
}

type ShortDramaPrerollDraft struct {
	ContractVersion     string                         `json:"contract_version"`
	TaskID              string                         `json:"task_id"`
	Revision            int64                          `json:"revision"`
	SelectedRouteID     string                         `json:"selected_route_id"`
	InputSnapshot       ShortDramaPrerollInputSnapshot `json:"input_snapshot"`
	InputHash           string                         `json:"input_hash"`
	Readiness           CreativeReadiness              `json:"readiness"`
	Candidates          []ShortDramaPrerollCandidate   `json:"candidates"`
	SelectedCandidateID string                         `json:"selected_candidate_id,omitempty"`
	CreatedAt           time.Time                      `json:"created_at"`
	UpdatedAt           time.Time                      `json:"updated_at"`
}

func (d ShortDramaPrerollDraft) Validate() error {
	if d.ContractVersion != "creative-short-drama-preroll-draft/v1" || strings.TrimSpace(d.TaskID) == "" ||
		d.Revision < 1 || d.SelectedRouteID != ManualShortDramaPrerollRouteID || strings.TrimSpace(d.InputHash) == "" ||
		!d.Readiness.PlanningReady || d.Candidates == nil || d.CreatedAt.IsZero() || d.UpdatedAt.IsZero() {
		return fmt.Errorf("short drama preroll draft is incomplete")
	}
	return nil
}

func planShortDramaCandidates(snapshot ShortDramaPrerollInputSnapshot, inputHash string) ([]ShortDramaPrerollCandidate, error) {
	variants := shortDramaExecutionVariants(snapshot)
	candidates := make([]ShortDramaPrerollCandidate, 0, len(variants))
	for index, variant := range variants {
		id := fmt.Sprintf("short_drama_%s_%d", snapshot.HookStrategy, index+1)
		beats := []ShortDramaStoryboardBeat{
			{StartSeconds: 0, EndSeconds: 2, Visual: variant.OpeningVisual, Copy: variant.HookLine},
			{StartSeconds: 2, EndSeconds: 4, Visual: variant.MiddleVisual, Copy: variant.MiddleCopy},
			{StartSeconds: 4, EndSeconds: 6, Visual: variant.ClosingVisual, Copy: snapshot.CallToAction},
		}
		compiled := compileShortDramaPrompt(snapshot, variant.Label, variant.HookLine, beats)
		prompt := ShortDramaPromptPackage{
			ContractVersion: "creative-short-drama-prompt-package/v1", InputSnapshotHash: inputHash, CandidateID: id,
			DirectorSpec: map[string]string{
				"execution_angle":             variant.Label,
				"subject_and_continuity":      "沿用已确认短剧人物、服装、场景与关系；不得改变人物身份。",
				"selling_point_dramatization": strings.Join(snapshot.ReviewedSellingPoints, "；"),
				"scene_and_tone":              "竖屏都市短剧，情绪紧凑，前一秒建立信息缺口。",
				"camera_language":             variant.CameraLanguage,
				"audio_spec":                  "可生成氛围音；默认静音可理解。",
				"post_production_constraints": "高对比动态字幕，安全区可读，无水印。",
			},
			NegativeConstraints: []string{"不得逐字复制正片首句", "不得虚构未确认剧情事实", "不得泄露完整结局"},
			CompiledPrompt:      compiled,
		}
		hashValue := prompt
		hashValue.ContentHash = ""
		hash, err := contract.CanonicalJSONHash(hashValue)
		if err != nil {
			return nil, err
		}
		prompt.ContentHash = "sha256:" + hash
		candidates = append(candidates, ShortDramaPrerollCandidate{
			ID: id, HookStrategy: snapshot.HookStrategy, ExecutionAngle: variant.ID,
			Score: 88 - index*3, ScoreMeaning: "hook_relevance",
			Evidence: append([]string{}, snapshot.ReviewedSellingPoints...), HookLine: variant.HookLine,
			Voiceover: variant.HookLine + " " + snapshot.CallToAction, Storyboard: beats, VisualIntent: variant.OpeningVisual,
			TransitionLine: variant.TransitionLine, PromptPackage: prompt,
		})
	}
	return candidates, nil
}

type shortDramaExecutionVariant struct {
	ID             string
	Label          string
	HookLine       string
	OpeningVisual  string
	MiddleVisual   string
	MiddleCopy     string
	ClosingVisual  string
	TransitionLine string
	CameraLanguage string
}

func shortDramaExecutionVariants(snapshot ShortDramaPrerollInputSnapshot) []shortDramaExecutionVariant {
	hooks := shortDramaHooks(snapshot.HookStrategy)
	return []shortDramaExecutionVariant{
		{
			ID: "dialogue_confrontation", Label: "台词对峙",
			HookLine:       hooks[0],
			OpeningVisual:  "极近景锁定主角直视质疑者的瞬间，以短促正反打建立公开对峙。",
			MiddleVisual:   "镜头保持人物轴线，切到质疑者停顿和主角不退让的表情。",
			MiddleCopy:     "她没有退让，一句反问让质疑声突然停住。",
			ClosingVisual:  "停在主角直视对方的表情，以清晰行动号召收束，不泄露结局。",
			TransitionLine: "她接下来会说什么？",
			CameraLanguage: "0–2 秒用台词和正反打点燃冲突，2–4 秒捕捉对峙停顿，4–6 秒以主角表情和行动号召收束。",
		},
		{
			ID: "action_reveal", Label: "关键动作",
			HookLine:       hooks[1],
			OpeningVisual:  "从手部或关键动作特写切入，不先展示完整结果，让观众追问动作背后的事实。",
			MiddleVisual:   "快速推近主角完成关键动作的瞬间，保持人物、服装与场景连续。",
			MiddleCopy:     "她不争辩，只把关键事实推到镜头中央。",
			ClosingVisual:  "停在事实将揭未揭的画面，以行动号召收束，不新增未经确认的证据。",
			TransitionLine: "那个关键事实究竟是什么？",
			CameraLanguage: "0–2 秒以手部或动作特写制造信息缺口，2–4 秒推近关键动作，4–6 秒在事实揭晓前用行动号召收束。",
		},
		{
			ID: "reaction_escalation", Label: "群体反应",
			HookLine:       hooks[2],
			OpeningVisual:  "中景横移扫过旁观者的轻视表情，再落到保持冷静的主角身上。",
			MiddleVisual:   "连续切换旁观者表情，让态度从轻视转为错愕，暂不解释反转原因。",
			MiddleCopy:     "镜头扫过众人，轻视正在变成惊讶。",
			ClosingVisual:  "停在全场同步看向主角的瞬间，以行动号召收束，不泄露完整结局。",
			TransitionLine: "他们为什么突然改变态度？",
			CameraLanguage: "0–2 秒建立群体压力，2–4 秒用反应镜头放大态度反转，4–6 秒停在集体注视并以行动号召收束。",
		},
	}
}

func shortDramaHooks(strategy ShortDramaHookStrategy) [3]string {
	switch strategy {
	case ShortDramaSuspenseReveal:
		return [3]string{
			"她只说了一句话，现场所有人同时沉默了。",
			"她没有解释，只把那个关键事实留在了镜头前。",
			"真相还没揭开，旁观者的表情却先变了。",
		}
	case ShortDramaIdentityContrast:
		return [3]string{
			"他们刚让她离开，最重要的位置却突然为她空了出来。",
			"没人把她放在眼里，直到最后的决定必须由她来做。",
			"上一秒还在轻视她的人，下一秒全部收起了笑容。",
		}
	case ShortDramaSellingPointBridge:
		return [3]string{
			"她没有争辩，只用一个结果让质疑声停了下来。",
			"所有解释，都不如接下来的这个瞬间更有说服力。",
			"他们以为故事已经结束，真正的答案才刚刚出现。",
		}
	default:
		return [3]string{
			"所有人都等着她低头，她却只问了一句：现在，可以听我说了吗？",
			"所有人都以为她已经输了，直到她把关键事实摆到众人面前。",
			"质疑声还没停，现场所有人的态度却突然变了。",
		}
	}
}

func compileShortDramaPrompt(snapshot ShortDramaPrerollInputSnapshot, executionAngle string, hook string, beats []ShortDramaStoryboardBeat) string {
	parts := []string{
		"短剧前贴效果广告", "标题：" + snapshot.StoryTitle, "已审核剧情事实：" + snapshot.Synopsis,
		"执行角度：" + executionAngle, "钩子：" + hook, "卖点：" + strings.Join(snapshot.ReviewedSellingPoints, "；"),
		"规格：9:16，独立 6 秒广告成片，" + snapshot.SubtitleStyle,
		"限制：人物、服装和场景连续；不得逐字复制正片台词；不得泄露结局；静音可理解。",
	}
	for _, beat := range beats {
		parts = append(parts, fmt.Sprintf("%d-%d 秒：%s；字幕/旁白：%s", beat.StartSeconds, beat.EndSeconds, beat.Visual, beat.Copy))
	}
	return strings.Join(parts, "。")
}
