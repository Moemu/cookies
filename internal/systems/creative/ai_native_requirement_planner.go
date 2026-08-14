package creative

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/provider"
)

const aiNativeRequirementPromptVersion = "ai-native-requirement/v2"

type AINativeRequirementTextGenerator interface {
	GenerateText(context.Context, provider.TextGenerateRequest) (provider.SynchronousResponse, error)
}

type AINativeRequirementPlanner interface {
	Analyze(context.Context, contract.ActorContext, contract.ProjectContext, AINativeProductSnapshot, AnalyzeAINativeRequirementRequest) (AINativeRequirementDraft, error)
}

type DeterministicAINativeRequirementPlanner struct{}

func (DeterministicAINativeRequirementPlanner) Analyze(_ context.Context, _ contract.ActorContext, _ contract.ProjectContext, product AINativeProductSnapshot, request AnalyzeAINativeRequirementRequest) (AINativeRequirementDraft, error) {
	audiences := []string{}
	if name := strings.TrimSpace(product.Name); name != "" {
		audiences = []string{
			"正在比较" + name + "的潜在消费者",
			"关注" + name + "日常使用体验的人群",
			"有相关品类购买需求的人群",
		}
	}
	sellingPoints := titleBackedSellingPoints(product.Name)
	description := ""
	if strings.TrimSpace(product.Name) != "" {
		description = product.Name
	}
	return buildAINativeRequirementDraft(product, request, description, audiences, sellingPoints, AINativeGenerationMetadata{
		Mode: "deterministic_fallback", ModelAlias: "fixture.deterministic", ModelVersion: "title-facts-v1", PromptVersion: aiNativeRequirementPromptVersion,
	}), nil
}

func titleBackedSellingPoints(title string) []string {
	points := make([]string, 0, 3)
	if strings.TrimSpace(title) == "" {
		return points
	}
	if strings.Contains(title, "纯钛") || strings.Contains(title, "钛杯") {
		points = append(points, "商品标题明确标注纯钛材质")
	}
	if strings.Contains(title, "保温杯") || strings.Contains(title, "咖啡杯") {
		points = append(points, "兼顾保温杯与随行咖啡杯使用场景")
	}
	if strings.Contains(title, "便携") || strings.Contains(title, "随行") {
		points = append(points, "强调便携随行的使用定位")
	}
	return points
}

type ModelAINativeRequirementPlanner struct {
	Text       AINativeRequirementTextGenerator
	ModelAlias string
}

type modelAINativeRequirement struct {
	ProductDescription string   `json:"product_description"`
	TargetAudiences    []string `json:"target_audiences"`
	CoreSellingPoints  []string `json:"core_selling_points"`
}

var aiNativeRequirementSchema = json.RawMessage(`{
  "type":"object",
  "additionalProperties":false,
  "required":["product_description","target_audiences","core_selling_points"],
  "properties":{
    "product_description":{"type":"string"},
    "target_audiences":{"type":"array","minItems":1,"maxItems":10,"items":{"type":"string"}},
    "core_selling_points":{"type":"array","minItems":1,"maxItems":20,"items":{"type":"string"}}
  }
}`)

func (p ModelAINativeRequirementPlanner) Analyze(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, product AINativeProductSnapshot, request AnalyzeAINativeRequirementRequest) (AINativeRequirementDraft, error) {
	if p.Text == nil || strings.TrimSpace(p.ModelAlias) == "" {
		return AINativeRequirementDraft{}, fmt.Errorf("AI native requirement text model is unavailable")
	}
	if strings.TrimSpace(product.Name) == "" {
		return AINativeRequirementDraft{}, fmt.Errorf("AI native requirement needs a user-confirmed product name before model planning")
	}
	input, err := json.Marshal(map[string]any{"product": product, "supplemental_requirement": request.SupplementalRequirement, "output_preset": request.outputPreset})
	if err != nil {
		return AINativeRequirementDraft{}, err
	}
	response, err := p.Text.GenerateText(ctx, provider.TextGenerateRequest{
		Actor: actor, Project: project, ModelAlias: p.ModelAlias,
		Messages: []provider.TextMessage{
			{Role: provider.TextRoleSystem, Content: "你是效果广告需求分析员。只使用输入商品快照、冻结的投放预设和用户补充需求，输出可编辑的商品描述、目标受众和核心卖点。不得补造材质、容量、保温时长、价格优惠或功效。卖点不需要输出来源字段。只输出符合 JSON Schema 的 JSON。"},
			{Role: provider.TextRoleUser, Content: string(input)},
		},
		OutputJSONSchema: aiNativeRequirementSchema,
	})
	if err != nil {
		return AINativeRequirementDraft{}, err
	}
	raw := response.StructuredOutput
	if len(raw) == 0 {
		raw = json.RawMessage(strings.TrimSpace(response.Text))
	}
	var generated modelAINativeRequirement
	if err := json.Unmarshal(raw, &generated); err != nil {
		return AINativeRequirementDraft{}, fmt.Errorf("decode AI native requirement output: %w", err)
	}
	if strings.TrimSpace(generated.ProductDescription) == "" || len(cleanTextList(generated.TargetAudiences, 10)) == 0 || len(cleanTextList(generated.CoreSellingPoints, 20)) == 0 {
		return AINativeRequirementDraft{}, fmt.Errorf("AI native requirement output is incomplete")
	}
	return buildAINativeRequirementDraft(product, request, generated.ProductDescription, generated.TargetAudiences, generated.CoreSellingPoints, AINativeGenerationMetadata{
		Mode: "model", ModelAlias: p.ModelAlias, ModelVersion: response.ModelVersion, RouteRevisionID: response.RouteRevisionID, PromptVersion: aiNativeRequirementPromptVersion,
	}), nil
}

type FallbackAINativeRequirementPlanner struct {
	Primary          AINativeRequirementPlanner
	Fallback         AINativeRequirementPlanner
	OnPrimaryFailure func(error)
}

func (p FallbackAINativeRequirementPlanner) Analyze(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, product AINativeProductSnapshot, request AnalyzeAINativeRequirementRequest) (AINativeRequirementDraft, error) {
	if p.Primary != nil {
		value, err := p.Primary.Analyze(ctx, actor, project, product, request)
		if err == nil {
			return value, nil
		}
		if p.OnPrimaryFailure != nil {
			p.OnPrimaryFailure(err)
		}
	}
	if p.Fallback == nil {
		return AINativeRequirementDraft{}, fmt.Errorf("AI native requirement planner is unavailable")
	}
	return p.Fallback.Analyze(ctx, actor, project, product, request)
}

func buildAINativeRequirementDraft(product AINativeProductSnapshot, request AnalyzeAINativeRequirementRequest, description string, audiences, sellingPoints []string, generation AINativeGenerationMetadata) AINativeRequirementDraft {
	media := make([]AINativeRequirementMedia, 0, len(product.Images))
	for index, image := range product.Images {
		media = append(media, AINativeRequirementMedia{ID: fmt.Sprintf("media_%d", index+1), URL: image.URL, Role: image.Role, Source: product.Source})
	}
	confirmations := []string{"商品详情和核心卖点需由用户核对"}
	if product.Price.DisplayUnconfirmed {
		confirmations = append(confirmations, "商品价格单位与展示口径尚未确认")
	}
	if strings.TrimSpace(product.Description) == "" {
		confirmations = append(confirmations, "当前分享链接未提供完整商品详情描述")
	}
	resolutionStatus := product.ResolutionStatus
	if resolutionStatus == "" {
		resolutionStatus = AINativeProductResolutionRecognized
	}
	missingFields := append([]string{}, product.MissingFields...)
	if strings.TrimSpace(product.Name) == "" {
		missingFields = appendUniqueText(missingFields, "product_name")
	}
	if len(product.Images) == 0 {
		missingFields = appendUniqueText(missingFields, "images")
	}
	hasConfirmationBlocker := false
	for _, field := range missingFields {
		if field == "product_name" || field == "images" || field == "core_selling_points" || field == "target_audiences" {
			hasConfirmationBlocker = true
			break
		}
	}
	if hasConfirmationBlocker {
		resolutionStatus = AINativeProductResolutionManualRequired
	} else if len(missingFields) > 0 {
		resolutionStatus = AINativeProductResolutionPartial
	}
	resourceType := product.ResourceType
	if resourceType == "" {
		resourceType = AINativeProductResourceProduct
	}
	delivery := DefaultAINativeDeliveryTreatment()
	if request.DeliveryTreatment != nil {
		delivery = *request.DeliveryTreatment
	}
	return AINativeRequirementDraft{
		ContractVersion: aiNativeRequirementContract, Revision: 1, Status: "draft", Product: product,
		ProductResolution: AINativeProductResolution{Status: resolutionStatus, Source: product.Source, ResourceType: resourceType, ExternalID: product.ProductID, SourceURL: product.SourceURL, MissingFields: missingFields},
		ProductName:       product.Name, ProductDescription: strings.TrimSpace(description),
		TargetAudiences: editableItems("audience", cleanTextList(audiences, 10)), Media: media,
		CoreSellingPoints:       editableItems("selling_point", cleanTextList(sellingPoints, 20)),
		SupplementalRequirement: request.SupplementalRequirement, Channel: request.Channel, AspectRatio: request.AspectRatio,
		DurationSeconds: request.DurationSeconds, Language: request.Language, OutputPreset: request.outputPreset, DeliveryTreatment: delivery,
		NeedsConfirmation: confirmations, Generation: generation,
	}
}

func cleanTextList(values []string, limit int) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
		if len(result) == limit {
			break
		}
	}
	return result
}

func editableItems(prefix string, values []string) []AINativeEditableText {
	result := make([]AINativeEditableText, 0, len(values))
	for index, value := range values {
		result = append(result, AINativeEditableText{ID: fmt.Sprintf("%s_%d", prefix, index+1), Text: value})
	}
	return result
}
