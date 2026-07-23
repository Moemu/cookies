package strategy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/strategy/prompts"
)

// SeedPolarisFresh creates a local demonstration proposal exactly once for a
// project. It intentionally contains only reusable mock business facts.
func SeedPolarisFresh(ctx context.Context, service Service, actor contract.ActorContext, project contract.ProjectContext, createPlans func(context.Context, string) error) (Proposal, StrategyOutput, bool, error) {
	input := prompts.ProposalInput{
		Brand: "极地鲜生", Product: "深海鳕鱼柳", Audience: "重视食材品质的城市家庭",
		Platform: "抖音与电商详情页", Budget: "200000 CNY", Timeline: "618 大促前 21 天",
		Description: "围绕冷链鲜度、便捷烹饪和家庭共享场景建立可信赖的海鲜心智。",
		Compliance:  []string{"禁用绝对化用语", "不宣称医疗功效", "优惠信息须以实际上架页面为准"},
		Directions:  []string{"冷链鲜度可视化", "十分钟家庭料理", "618 囤货场景"},
	}
	proposal, duplicate, err := service.CreateProposal(ctx, actor, project, input)
	if err != nil {
		return Proposal{}, StrategyOutput{}, false, err
	}
	if duplicate {
		output, err := service.Store.GetStrategyByProposal(ctx, actor.OrganizationID, project.ProjectID, proposal.ID)
		return proposal, output, false, err
	}
	id, err := service.newID()("strategy")
	if err != nil {
		return Proposal{}, StrategyOutput{}, false, err
	}
	content, _ := json.Marshal(map[string]any{
		"insight":              "家庭消费者希望在大促囤货时兼顾食材品质与烹饪便利。",
		"proposition":          "把冷链鲜度带到每一餐。",
		"strategy":             "以可信赖的冷链过程和可复现的家庭料理场景建立购买理由。",
		"channels":             []string{"抖音短视频", "电商详情页"},
		"creative_directions":  input.Directions,
		"compliance_checklist": input.Compliance,
	})
	output, err := service.Store.CreateStrategy(ctx, StrategyOutput{
		ID: id, ProposalID: proposal.ID, OrganizationID: actor.OrganizationID, ProjectID: project.ProjectID,
		Content: content, ModelAlias: "seeded", ModelVersion: prompts.TemplateVersion, ProviderCode: "seed", CreatedAt: service.now(),
	})
	if err != nil {
		return Proposal{}, StrategyOutput{}, false, err
	}
	output, err = service.ApproveStrategy(ctx, actor, project, output.ID)
	if err != nil {
		return Proposal{}, StrategyOutput{}, false, err
	}
	if createPlans != nil {
		if err := createPlans(ctx, output.ID); err != nil {
			return Proposal{}, StrategyOutput{}, false, fmt.Errorf("seed creative plans: %w", err)
		}
	}
	return proposal, output, true, nil
}
