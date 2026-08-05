package strategy

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func (s Service) ExportCreativeTaskStrategyMarkdown(
	ctx context.Context,
	actor contract.ActorContext,
	planID string,
	versionNumber int64,
) (string, error) {
	version, err := s.GetCreativeTaskStrategyVersion(ctx, actor, planID, versionNumber)
	if err != nil {
		return "", err
	}
	document := version.Document
	var builder strings.Builder
	builder.WriteString("# 创意任务策略\n\n")
	builder.WriteString(fmt.Sprintf("- 业务：`%s`\n", document.BusinessRef.BusinessCode))
	builder.WriteString(fmt.Sprintf("- 任务策略版本：`%d`\n", version.Version))
	builder.WriteString(fmt.Sprintf("- 内容 Hash：`%s`\n", version.ContentHash))
	builder.WriteString(fmt.Sprintf("- Brief：`%s@%d`\n\n", document.Lineage.BriefID, document.Lineage.BriefVersion))
	writeMarkdownSection(&builder, "目标", []string{document.Objective})
	writeMarkdownSection(&builder, "目标人群", append([]string{document.Audience.Primary}, document.Audience.Insights...))
	writeMarkdownSection(&builder, "核心主张", []string{document.CoreMessage})
	writeMarkdownSection(&builder, "信息优先级", document.MessageHierarchy)
	builder.WriteString("## 业务策略\n\n")
	for key, value := range document.BusinessStrategy {
		builder.WriteString(fmt.Sprintf("- `%s`：%v\n", key, value))
	}
	builder.WriteString("\n")
	if len(document.Hypotheses) > 0 {
		builder.WriteString("## 验证假设\n\n")
		for _, hypothesis := range document.Hypotheses {
			builder.WriteString(fmt.Sprintf("- %s；变量：%s；指标：%s\n",
				hypothesis.Statement, hypothesis.Variable, hypothesis.Metric))
		}
		builder.WriteString("\n")
	}
	writeMarkdownSection(&builder, "事实与证据", document.ClaimsAndEvidence)
	if len(document.Media.Items) > 0 {
		builder.WriteString("## 素材实际使用情况\n\n")
		for _, item := range document.Media.Items {
			builder.WriteString(fmt.Sprintf("- `%s@%d`（%s / %s）：%s\n",
				item.AssetRef.AssetID, item.AssetRef.Version, item.Role, item.Kind,
				creativeMediaUsefulnessLabel(item.Usefulness)))
			for _, observation := range item.Observations {
				builder.WriteString("  - 已读取观察：" + observation + "\n")
			}
			for _, limitation := range item.Limitations {
				builder.WriteString("  - 限制：" + limitation + "\n")
			}
		}
		builder.WriteString("\n")
	}
	writeMarkdownSection(&builder, "约束", document.Guardrails)
	if len(document.AssetRequirements) > 0 {
		builder.WriteString("## 素材要求\n\n")
		for _, requirement := range document.AssetRequirements {
			builder.WriteString(fmt.Sprintf("- [%s] %s\n", requirement.RequiredStage, requirement.Requirement))
		}
		builder.WriteString("\n")
	}
	writeMarkdownSection(&builder, "待确认问题", document.OpenQuestions)
	builder.WriteString("## 来源\n\n")
	builder.WriteString(fmt.Sprintf("- Business Profile：`%s@%s`（`%s`）\n",
		document.BusinessRef.BusinessCode, document.BusinessRef.Version,
		document.BusinessRef.ContentHash))
	builder.WriteString(fmt.Sprintf("- Strategy Skill：`%s@%s`（`%s`）\n",
		document.Lineage.SkillName, document.Lineage.SkillVersion,
		document.Lineage.SkillContentHash))
	return builder.String(), nil
}

func creativeMediaUsefulnessLabel(value string) string {
	switch value {
	case CreativeMediaSemantic:
		return "已读取语义特征，可作为策略依据"
	case CreativeMediaProductionOnly:
		return "仅核验存在和规格，未读取内容"
	default:
		return "当前不可用"
	}
}

func writeMarkdownSection(builder *strings.Builder, title string, values []string) {
	if len(values) == 0 {
		return
	}
	builder.WriteString("## ")
	builder.WriteString(title)
	builder.WriteString("\n\n")
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			builder.WriteString("- ")
			builder.WriteString(strings.TrimSpace(value))
			builder.WriteString("\n")
		}
	}
	builder.WriteString("\n")
}
