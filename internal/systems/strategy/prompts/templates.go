// Package prompts owns the versioned, reusable advertising prompt templates.
package prompts

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/provider"
)

const TemplateVersion = "volcad-v1"

type ProposalInput struct {
	Brand       string   `json:"brand"`
	Product     string   `json:"product"`
	Audience    string   `json:"audience"`
	Platform    string   `json:"platform"`
	Budget      string   `json:"budget"`
	Timeline    string   `json:"timeline"`
	Compliance  []string `json:"compliance"`
	Directions  []string `json:"directions"`
	Description string   `json:"description"`
}

func (i ProposalInput) Validate() error {
	if strings.TrimSpace(i.Brand) == "" || strings.TrimSpace(i.Product) == "" {
		return fmt.Errorf("brand and product are required")
	}
	if i.Compliance == nil || i.Directions == nil {
		return fmt.Errorf("compliance and directions must be arrays")
	}
	return nil
}

func BuildProposalStrategyMessages(input ProposalInput) []provider.TextMessage {
	return []provider.TextMessage{
		{Role: provider.TextRoleSystem, Content: "你是资深品牌电商广告策划。仅输出符合给定 JSON Schema 的 JSON；不得使用绝对化、医疗功效或无法验证的承诺，并逐项遵守合规限制。"},
		{Role: provider.TextRoleUser, Content: proposalContext(input) + "\n请产出洞察、核心主张、传播策略、渠道节奏、创意方向和合规检查清单。"},
	}
}

func BuildCopyMessages(input ProposalInput, materialType string, count int) []provider.TextMessage {
	return []provider.TextMessage{
		{Role: provider.TextRoleSystem, Content: "你是电商广告文案策划。仅输出符合给定 JSON Schema 的 JSON；严禁绝对化用语、虚假优惠、医疗功效和规避审核的表达。"},
		{Role: provider.TextRoleUser, Content: proposalContext(input) + fmt.Sprintf("\n为%s生成%d条可审核广告文案，包含标题、正文和合规说明。", materialType, count)},
	}
}

func BuildImagePrompt(input ProposalInput, variant int) string {
	return fmt.Sprintf("Commercial food photography for %s %s, variant %d. %s. Premium frozen seafood texture, clean cold-chain visual cues, ecommerce key visual, no text overlay, compliant advertising, high detail.",
		input.Brand, input.Product, variant, direction(input, variant))
}

func BuildVideoPrompt(input ProposalInput, variant int) string {
	return fmt.Sprintf("15-second ecommerce video for %s %s, variant %d. %s. Show frozen-to-pan preparation, family meal moment, cold-chain freshness cues, natural lighting, no text overlay, compliant advertising.",
		input.Brand, input.Product, variant, direction(input, variant))
}

func StrategyJSONSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","required":["insight","proposition","strategy","channels","creative_directions","compliance_checklist"],"properties":{"insight":{"type":"string"},"proposition":{"type":"string"},"strategy":{"type":"string"},"channels":{"type":"array","items":{"type":"string"}},"creative_directions":{"type":"array","items":{"type":"string"}},"compliance_checklist":{"type":"array","items":{"type":"string"}}},"additionalProperties":false}`)
}

func proposalContext(input ProposalInput) string {
	return fmt.Sprintf("模板版本：%s\n品牌：%s\n产品：%s\n受众：%s\n平台：%s\n预算：%s\n时间：%s\n需求：%s\n合规限制：%s\n创意方向：%s",
		TemplateVersion, input.Brand, input.Product, input.Audience, input.Platform, input.Budget, input.Timeline,
		input.Description, strings.Join(input.Compliance, "；"), strings.Join(input.Directions, "；"))
}

func direction(input ProposalInput, variant int) string {
	if len(input.Directions) == 0 {
		return "Highlight product quality and convenient home cooking"
	}
	return input.Directions[(variant-1)%len(input.Directions)]
}
