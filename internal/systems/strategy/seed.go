package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/strategy/prompts"
)

const DefaultVolcadProposalObjectURI = "tos://cookies-assets/demo/volcad/mock_proposal_package.zip"

// SeedPolarisFresh creates a local demonstration proposal exactly once for a
// project. It intentionally contains only reusable mock business facts.
func SeedPolarisFresh(ctx context.Context, service Service, actor contract.ActorContext, project contract.ProjectContext, createPlans func(context.Context, string) error) (Proposal, StrategyOutput, bool, error) {
	proposal, duplicate, err := SeedPolarisFreshProposal(ctx, service, actor, project)
	if err != nil {
		return Proposal{}, StrategyOutput{}, false, err
	}
	if duplicate {
		output, err := service.Store.GetStrategyByProposal(ctx, actor.OrganizationID, project.ProjectID, proposal.ID)
		if err == nil {
			return proposal, output, false, nil
		}
		if err != ErrStrategyNotFound {
			return Proposal{}, StrategyOutput{}, false, err
		}
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
		"creative_directions":  proposal.Input.Directions,
		"compliance_checklist": proposal.Input.Compliance,
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

func SeedPolarisFreshProposal(ctx context.Context, service Service, actor contract.ActorContext, project contract.ProjectContext) (Proposal, bool, error) {
	input := PolarisFreshVolcadProposalInput(DefaultVolcadProposalObjectURI)
	proposal, duplicate, err := service.CreateProposal(ctx, actor, project, input)
	return proposal, duplicate, err
}

func PolarisFreshVolcadProposalInput(objectURI string) prompts.ProposalInput {
	if strings.TrimSpace(objectURI) == "" {
		objectURI = DefaultVolcadProposalObjectURI
	}
	packageData := prompts.VolcadProposalPackage{
		CampaignName: "极地鲜生 618 新品推广",
		Brief: prompts.VolcadBrandBrief{
			BrandName:       "极地鲜生",
			Category:        "食品生鲜",
			ProductName:     "深海鳕鱼柳",
			SellingPoints:   []string{"野生捕捞", "0添加", "高蛋白低脂", "冷链直达"},
			TargetAudience:  "25-40岁注重健康饮食和家庭囤货效率的城市白领家庭",
			Platforms:       []string{"抖音", "小红书", "微信视频号"},
			Tone:            "专业且有亲和力，强调品质感和生活方式感",
			Budget:          "日预算 1 万 - 2 万",
			ROIGoal:         "首月 ROI≥3，新客成交成本控制在 80 元以内",
			ComplianceNotes: "禁用绝对化用语；不得夸大减脂效果；避免医疗功效暗示",
		},
		Options: prompts.VolcadGenerationOptions{
			AssetKinds:    []string{"text", "image", "video"},
			MaterialTypes: []string{"素材二创", "卡点直播"},
			CopyCount:     4,
			ImageCount:    3,
			VideoCount:    1,
			ImageSize:     "2048x2048",
			VideoRatio:    "9:16",
			VideoDuration: 6,
		},
		ExtraRequirements: "需要兼顾内容种草与转化收口，前 3 秒一定要突出“开袋即烹、全家可吃、无负担”。文案要能同时适配短视频口播、直播间切片字幕和信息流投放标题。",
		CreativeKeywords:  []string{"家庭快手晚餐", "高蛋白轻负担", "618 囤货清单", "达人真实口播", "直播间福利转化"},
		ImageDirection: prompts.VolcadImageDirection{
			MainVisual: "香煎鳕鱼柳成品特写 + 北欧厨房背景 + 冷链蓝色辅助光",
			CopySlots:  []string{"首屏大标题", "卖点角标", "618 活动贴片", "到手价信息"},
			MustShow:   []string{"鱼柳纹理", "一袋多吃法", "家庭共食氛围"},
		},
		VideoDirection: prompts.VolcadVideoDirection{
			OpeningHooks: []string{"晚饭不知道吃什么的，真的可以试试这个鳕鱼柳", "618 囤货我最怕踩雷，但这个我已经回购第三次了"},
			ShotKeywords: []string{"开袋", "下锅", "滋滋冒油", "切开展示纹理", "餐桌成品", "优惠字幕"},
			CTA:          "引导用户进直播间领券或点击立即下单",
		},
		ActivityCadence: []string{
			"预热期：6 月 1 日 - 6 月 7 日，重点做人群种草和收藏加购",
			"爆发期：6 月 8 日 - 6 月 18 日，重点打爆品心智和直播间转化",
			"返场期：6 月 19 日 - 6 月 21 日，承接未转化人群，强调库存与优惠尾期",
		},
		CompetitorNotes: []string{"对标高品质冷冻海鲜与家庭快手菜场景，避免纯商超货架感。"},
		UserFeedback:    []string{"用户关注无腥味、烹饪门槛低、肉质紧实和家庭复购价值。"},
	}
	return prompts.ProposalInput{
		Brand:       packageData.Brief.BrandName,
		Product:     packageData.Brief.ProductName,
		Audience:    packageData.Brief.TargetAudience,
		Platform:    strings.Join(packageData.Brief.Platforms, "、"),
		Budget:      packageData.Brief.Budget,
		Timeline:    "618 大促前 21 天",
		Description: packageData.ExtraRequirements,
		Compliance:  []string{"禁用绝对化用语", "不宣称医疗功效", "优惠信息须以实际上架页面为准", "不得夸大减脂效果", "避免医疗功效暗示"},
		Directions:  []string{"冷链鲜度可视化", "十分钟家庭料理", "618 囤货场景", "家庭共食氛围", "达人真实口播"},
		Source: prompts.ProposalSource{
			Type:      "tos",
			ObjectURI: objectURI,
			Archive:   "mock_proposal_package.zip",
			Files: []prompts.ProposalSourceFile{
				{Name: "01_客户简报.txt", Size: 857, Preview: "品牌名称：极地鲜生；主营品类：食品生鲜；核心卖点：野生捕捞，0添加，高蛋白低脂，冷链直达。"},
				{Name: "02_投放要求.json", Size: 583, Preview: "asset_kinds: text/image/video；material_types: 素材二创、卡点直播；video_ratio: 9:16。"},
				{Name: "03_素材偏好.csv", Size: 418, Preview: "家庭轻烹饪 / 健身控卡 / 宝妈囤货；冰蓝高级感、北欧厨房、食材新鲜特写。"},
				{Name: "04_活动节奏.md", Size: 638, Preview: "618 预热、爆发、返场三阶段；视频前 3 秒必须出现问题式钩子。"},
				{Name: "05_落地页说明.html", Size: 626, Preview: "首屏突出野生捕捞深海鳕鱼柳，转化区放置 618 限时券、第二件折扣、直播间福利。"},
			},
		},
		ProposalPackage: &packageData,
	}
}
