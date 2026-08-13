package creative

import (
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const shortDramaReferenceBoardPromptRules = "输入参考图是一张 2×2 视觉设定板，不是视频分屏画面。左上区域只定义开场构图；右上区域只定义人物身份与服装；左下区域只定义环境、光线与色调；右下区域只定义关键动作或道具。生成的视频必须是正常的单画面连续影像，不得出现宫格、拼贴、边框、标签或说明文字。"

var shortDramaReferenceBoardVariants = []struct {
	Key         string
	Label       string
	Instruction string
}{
	{"character_emotion", "人物情绪", "以人物情绪和身份状态为唯一主要测试变量，突出微表情、姿态变化与心理压力"},
	{"environment_suspense", "环境悬念", "以环境、空间压迫和信息缺口为唯一主要测试变量，人物身份与剧情事实保持锁定"},
	{"action_prop", "动作道具", "以关键动作、道具或事件触发为唯一主要测试变量，避免只更换景别或形容词"},
}

func compileShortDramaVibeIntent(analysis ShortDramaV2Analysis, direction ShortDramaV2HookDirection, duration int) ShortDramaVibeIntent {
	evidence := append([]string(nil), direction.GroundingEvidenceIDs...)
	return ShortDramaVibeIntent{
		Version:       "short-drama-vibe-intent/v1",
		VisualAnchor:  strings.TrimSpace(direction.VisualIntent),
		BehaviorState: strings.TrimSpace(direction.HookCopy),
		LocalTone:     strings.TrimSpace(analysis.Content.Tone + "，" + strings.Join(analysis.Content.VisualKeywords, "、")),
		Theme:         strings.TrimSpace(direction.Title + "，" + analysis.Content.UnresolvedHook),
		HardConstraints: []string{
			fmt.Sprintf("独立前贴，总时长严格为 %d 秒", duration),
			"严格基于当前视频理解事实，不虚构人物、关系、事件或结局",
			"原创虚构人物，不模仿真实演员、公众人物或已知角色",
			"无 Logo、水印、乱码文字、格子标签或额外肢体",
		},
		EvidenceIDs: evidence,
	}
}

func compileShortDramaReferenceBoardPlan(analysis ShortDramaV2Analysis, direction ShortDramaV2HookDirection, duration int) (ShortDramaReferenceBoardPlan, error) {
	intent := compileShortDramaVibeIntent(analysis, direction, duration)
	evidence := append([]string(nil), intent.EvidenceIDs...)
	character := "原创虚构角色，身份、年龄状态、服装、发型和表情与当前剧情事实一致"
	if len(analysis.Content.Characters) > 0 {
		parts := make([]string, 0, len(analysis.Content.Characters))
		for _, item := range analysis.Content.Characters {
			parts = append(parts, strings.TrimSpace(item.Name+"："+item.Description))
		}
		character = strings.Join(parts, "；") + "；仅作为原创角色身份设定，不模仿演员"
	}
	plan := ShortDramaReferenceBoardPlan{
		Version:    "short-drama-reference-board-plan/v1",
		Layout:     "2x2_v1",
		VibeIntent: intent,
		Panels: []ShortDramaReferencePanelPlan{
			{Slot: "A", Role: "opening_composition", Description: "开场主构图与注意力中心：" + direction.VisualIntent, EvidenceIDs: evidence},
			{Slot: "B", Role: "character_identity", Description: "人物身份、服装与表情：" + character, EvidenceIDs: evidence},
			{Slot: "C", Role: "environment_mood", Description: "场景、时代、光线与色调：" + analysis.Content.Tone + "，" + strings.Join(analysis.Content.VisualKeywords, "、"), EvidenceIDs: evidence},
			{Slot: "D", Role: "action_detail", Description: "关键动作、道具或情绪细节：" + direction.HookCopy, EvidenceIDs: evidence},
		},
		GlobalStyle: strings.TrimSpace(analysis.Content.Tone + "，统一世界观、人物造型、材质、光线和色彩"),
		NegativeRules: []string{
			"不得在画面中生成文字、编号、Logo 或水印",
			"不得出现互相矛盾的人脸、服装、时代或道具",
			"不得复刻真实演员或已知角色",
		},
	}
	hash, err := contract.CanonicalJSONHash(plan)
	if err != nil {
		return ShortDramaReferenceBoardPlan{}, err
	}
	plan.ContentHash = "sha256:" + hash
	return plan, validateShortDramaReferenceBoardPlan(analysis, plan)
}

func validateShortDramaReferenceBoardPlan(analysis ShortDramaV2Analysis, plan ShortDramaReferenceBoardPlan) error {
	if plan.Version != "short-drama-reference-board-plan/v1" || plan.Layout != "2x2_v1" || len(plan.Panels) != 4 || strings.TrimSpace(plan.ContentHash) == "" {
		return fmt.Errorf("short drama reference board plan is incomplete")
	}
	intent := plan.VibeIntent
	if strings.TrimSpace(intent.VisualAnchor) == "" || strings.TrimSpace(intent.BehaviorState) == "" || strings.TrimSpace(intent.LocalTone) == "" || strings.TrimSpace(intent.Theme) == "" || len(intent.EvidenceIDs) == 0 {
		return fmt.Errorf("short drama vibe intent is incomplete")
	}
	validEvidence := make(map[string]struct{}, len(analysis.Content.Evidence))
	for _, item := range analysis.Content.Evidence {
		validEvidence[item.ID] = struct{}{}
	}
	expected := map[string]string{"A": "opening_composition", "B": "character_identity", "C": "environment_mood", "D": "action_detail"}
	seen := make(map[string]struct{}, 4)
	for _, panel := range plan.Panels {
		if expected[panel.Slot] != panel.Role || strings.TrimSpace(panel.Description) == "" || len(panel.EvidenceIDs) == 0 {
			return fmt.Errorf("short drama reference board panel %q is invalid", panel.Slot)
		}
		if _, exists := seen[panel.Slot]; exists {
			return fmt.Errorf("short drama reference board panel %q is duplicated", panel.Slot)
		}
		seen[panel.Slot] = struct{}{}
		for _, id := range panel.EvidenceIDs {
			if _, ok := validEvidence[id]; !ok {
				return fmt.Errorf("short drama reference board uses unknown evidence %q", id)
			}
		}
	}
	return nil
}

func compileShortDramaReferenceBoardImagePrompt(plan ShortDramaReferenceBoardPlan, creativeDirection, variantKey, instruction string, board ShortDramaBoardCanvas, output ShortDramaOutputCanvas) string {
	panels := make([]string, 0, len(plan.Panels))
	for _, panel := range plan.Panels {
		panels = append(panels, fmt.Sprintf("%s 区（%s）：%s", panel.Slot, panel.Role, panel.Description))
	}
	return fmt.Sprintf(
		"生成一张完整的 2×2 短剧前贴视觉设定板，外层画布 %s、%d×%d，四个区域面积均衡且全部完整保留。用户可编辑的创作方向：%s。%s。%s。四格属于同一世界、同一人物设定和统一画风。%s。全局风格：%s。主要测试变量：%s。最终视频输出为 %d:%d，人物脸、关键动作和道具应位于各格中央安全区。禁止在像素中写入 A/B/C/D、中文说明、标题、边框标签、Logo 或水印。%s",
		board.AspectRatio, board.Width, board.Height, strings.TrimSpace(creativeDirection), strings.Join(panels, "；"), strings.Join(plan.VibeIntent.HardConstraints, "；"), strings.Join(plan.NegativeRules, "；"), plan.GlobalStyle, variantKey+"："+instruction, output.AspectNum, output.AspectDen, shortDramaReferenceBoardPromptRules,
	)
}

func compileShortDramaReferenceBoardVideoPrompt(base string, duration int, candidate ShortDramaReferenceBoardCandidate, output *ShortDramaOutputCanvas) string {
	canvasInstruction := "人物面部、关键动作和字幕始终位于中央安全区"
	if output != nil {
		canvasInstruction = fmt.Sprintf("最终视频将确定性归一化为 %d:%d（%d×%d），人物面部、关键动作和字幕始终位于中央安全区", output.AspectNum, output.AspectDen, output.Width, output.Height)
	}
	panelLines := make([]string, 0, len(candidate.Plan.Panels))
	for _, panel := range candidate.Plan.Panels {
		panelLines = append(panelLines, fmt.Sprintf("%s=%s", panel.Slot, panel.Description))
	}
	return strings.TrimSpace(base) + fmt.Sprintf(
		"\n\n%s\n所选视觉方案的主要测试变量：%s。面板语义：%s。创作意图：视觉锚点=%s；行为状态=%s；局部调性=%s；主题=%s。总时长严格为 %d 秒，生成独立短剧前贴，不拼接正片。%s。不得虚构当前视频理解之外的人物、关系、事件或结局，不得生成水印、Logo、乱码文字或额外肢体。",
		shortDramaReferenceBoardPromptRules, candidate.PrimaryTestVariable, strings.Join(panelLines, "；"), candidate.Plan.VibeIntent.VisualAnchor, candidate.Plan.VibeIntent.BehaviorState, candidate.Plan.VibeIntent.LocalTone, candidate.Plan.VibeIntent.Theme, duration, canvasInstruction,
	)
}
