package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/strategy/creativecatalog"
)

const (
	CreativeMediaSemantic       = "semantic"
	CreativeMediaProductionOnly = "production_only"
	CreativeMediaUnavailable    = "unavailable"
)

type CreativeMediaInput struct {
	AssetRef        contract.AssetVersionRef `json:"asset_ref"`
	Role            string                   `json:"role"`
	Origin          string                   `json:"origin"`
	Kind            string                   `json:"kind"`
	MIMEType        string                   `json:"mime_type,omitempty"`
	Status          string                   `json:"status"`
	WidthPixels     int                      `json:"width_pixels,omitempty"`
	HeightPixels    int                      `json:"height_pixels,omitempty"`
	DurationSeconds float64                  `json:"duration_seconds,omitempty"`
	Usefulness      string                   `json:"usefulness"`
	StrategyUses    []string                 `json:"strategy_uses"`
	Observations    []string                 `json:"observations"`
	Limitations     []string                 `json:"limitations"`
}

type CreativeMediaAssessment struct {
	Items               []CreativeMediaInput `json:"items"`
	SemanticCount       int                  `json:"semantic_count"`
	ProductionOnlyCount int                  `json:"production_only_count"`
	UnavailableCount    int                  `json:"unavailable_count"`
	Warnings            []string             `json:"warnings"`
}

type creativeMediaCandidate struct {
	Ref    contract.AssetVersionRef
	Role   string
	Origin string
}

func briefMediaCandidates(brief BriefDocument) []creativeMediaCandidate {
	result := make([]creativeMediaCandidate, 0, len(brief.Product.AssetRefs))
	for _, ref := range brief.Product.AssetRefs {
		result = append(result, creativeMediaCandidate{
			Ref: ref, Role: "product_asset", Origin: "brief.product.asset_refs",
		})
	}
	return result
}

func planMediaCandidates(
	brief BriefDocument,
	profile creativecatalog.Profile,
	answers map[string]json.RawMessage,
) []creativeMediaCandidate {
	result := briefMediaCandidates(brief)
	for _, question := range profile.Questions {
		if question.Type != "asset_ref" {
			continue
		}
		raw, found := answers[question.ID]
		if !found {
			continue
		}
		var ref contract.AssetVersionRef
		if json.Unmarshal(raw, &ref) != nil || ref.Validate() != nil {
			continue
		}
		result = append(result, creativeMediaCandidate{
			Ref: ref, Role: question.ID, Origin: "plan.answers." + question.ID,
		})
	}
	return uniqueCreativeMediaCandidates(result)
}

func uniqueCreativeMediaCandidates(values []creativeMediaCandidate) []creativeMediaCandidate {
	result := make([]creativeMediaCandidate, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		key := fmt.Sprintf("%s:%d:%s", value.Ref.AssetID, value.Ref.Version, value.Role)
		if value.Ref.Validate() != nil || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func (s Service) assessCreativeMedia(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	candidates []creativeMediaCandidate,
) CreativeMediaAssessment {
	result := CreativeMediaAssessment{
		Items: []CreativeMediaInput{}, Warnings: []string{},
	}
	if len(candidates) == 0 {
		return result
	}
	if s.CreativeAssets == nil {
		result.Warnings = append(result.Warnings,
			"已引用素材，但当前环境未配置素材读取能力；策略不会假装已经看过图片或视频")
		for _, candidate := range candidates {
			result.Items = append(result.Items, CreativeMediaInput{
				AssetRef: candidate.Ref, Role: candidate.Role, Origin: candidate.Origin,
				Status: "unresolved", Usefulness: CreativeMediaUnavailable,
				StrategyUses: []string{}, Observations: []string{},
				Limitations: []string{"无法验证素材是否属于当前 Project，也无法读取媒体类型和分析结果"},
			})
			result.UnavailableCount++
		}
		return result
	}

	features, featureErr := s.CreativeAssets.ListFeatures(ctx, actor, projectID, 200)
	if featureErr != nil {
		result.Warnings = append(result.Warnings,
			"素材语义分析暂不可用；仍会核验素材类型和可用状态")
	}
	featureByRef := latestCreativeFeatures(features)
	for _, candidate := range candidates {
		item := CreativeMediaInput{
			AssetRef: candidate.Ref, Role: candidate.Role, Origin: candidate.Origin,
			StrategyUses: []string{}, Observations: []string{}, Limitations: []string{},
		}
		projectAsset, err := s.CreativeAssets.Get(ctx, actor, projectID, candidate.Ref)
		if err != nil {
			item.Status = "unavailable"
			item.Usefulness = CreativeMediaUnavailable
			item.Limitations = append(item.Limitations,
				"素材不存在、已移出当前 Project，或当前用户无权访问")
			result.UnavailableCount++
			result.Items = append(result.Items, item)
			continue
		}
		item.Kind = string(projectAsset.Asset.Kind)
		item.MIMEType = projectAsset.Version.MIMEType
		item.Status = string(projectAsset.Version.Status)
		item.WidthPixels = projectAsset.Version.WidthPixels
		item.HeightPixels = projectAsset.Version.HeightPixels
		item.DurationSeconds = projectAsset.Version.Media.DurationSeconds
		if projectAsset.Version.Status != assets.AssetReady {
			item.Usefulness = CreativeMediaUnavailable
			item.Limitations = append(item.Limitations, "素材尚未 ready，不能作为当前策略依据")
			result.UnavailableCount++
			result.Items = append(result.Items, item)
			continue
		}
		reader, _, previewErr := s.CreativeAssets.OpenPreview(ctx, actor, projectID, candidate.Ref)
		if previewErr != nil {
			item.Status = "unavailable"
			item.Usefulness = CreativeMediaUnavailable
			item.Limitations = append(item.Limitations,
				"素材元数据存在，但原始文件当前不可读取，不能用于策略或生产")
			result.UnavailableCount++
			result.Items = append(result.Items, item)
			continue
		}
		_ = reader.Close()

		feature, analyzed := featureByRef[assetRefKey(candidate.Ref)]
		if analyzed {
			item.Usefulness = CreativeMediaSemantic
			item.StrategyUses = creativeFeatureUses(projectAsset, feature)
			item.Observations = creativeFeatureObservations(feature)
			if len(item.Observations) == 0 {
				item.Limitations = append(item.Limitations, "分析记录存在，但没有可展示的语义观察")
			}
			result.SemanticCount++
		} else {
			item.Usefulness = CreativeMediaProductionOnly
			item.StrategyUses = creativeMetadataUses(projectAsset)
			item.Limitations = append(item.Limitations,
				"当前只核验了格式和可用性，未读取画面、声音、文案或叙事内容")
			result.ProductionOnlyCount++
		}
		result.Items = append(result.Items, item)
	}
	return result
}

func (s Service) validateCreativeTaskAssetAnswers(
	ctx context.Context,
	actor contract.ActorContext,
	projectID contract.ProjectID,
	profile creativecatalog.Profile,
	answers map[string]json.RawMessage,
) error {
	if s.CreativeAssets == nil {
		return nil
	}
	for _, question := range profile.Questions {
		if question.Type != "asset_ref" {
			continue
		}
		raw, found := answers[question.ID]
		if !found {
			continue
		}
		var ref contract.AssetVersionRef
		if json.Unmarshal(raw, &ref) != nil || ref.Validate() != nil {
			return ErrInvalidRequest
		}
		value, err := s.CreativeAssets.Get(ctx, actor, projectID, ref)
		if err != nil || value.Version.Status != assets.AssetReady {
			return ErrInvalidRequest
		}
		reader, _, err := s.CreativeAssets.OpenPreview(ctx, actor, projectID, ref)
		if err != nil {
			return fmt.Errorf("%w: selected asset content is unavailable", ErrInvalidRequest)
		}
		_ = reader.Close()
	}
	return nil
}

func latestCreativeFeatures(values []assets.AssetFeature) map[string]assets.AssetFeature {
	sort.SliceStable(values, func(i, j int) bool {
		return values[i].UpdatedAt.After(values[j].UpdatedAt)
	})
	result := map[string]assets.AssetFeature{}
	for _, value := range values {
		key := assetRefKey(value.Ref())
		if _, found := result[key]; !found {
			result[key] = value
		}
	}
	return result
}

func assetRefKey(ref contract.AssetVersionRef) string {
	return fmt.Sprintf("%s:%d", ref.AssetID, ref.Version)
}

func creativeMetadataUses(value assets.ProjectAsset) []string {
	switch value.Asset.Kind {
	case contract.AssetImage:
		return []string{"确认图片可用于后续商品保真和画幅规划"}
	case contract.AssetVideo:
		return []string{"确认视频时长和编码可用于后续交付规格规划"}
	default:
		return []string{}
	}
}

func creativeFeatureUses(value assets.ProjectAsset, feature assets.AssetFeature) []string {
	result := creativeMetadataUses(value)
	if feature.ProductVisibility > 0 {
		result = append(result, "判断产品可见度与商品展示约束")
	}
	if len(feature.SceneTags) > 0 || len(feature.ActionTags) > 0 {
		result = append(result, "辅助判断场景与动作表达方向")
	}
	if len(feature.SellingPoints) > 0 || len(feature.Evidence) > 0 {
		result = append(result, "核对素材中已出现的卖点和可见证据")
	}
	if feature.HookStrength > 0 {
		result = append(result, "辅助判断开篇吸引力，但不直接生成具体 Hook")
	}
	return normalizedUnique(result)
}

func creativeFeatureObservations(feature assets.AssetFeature) []string {
	result := []string{}
	if len(feature.SceneTags) > 0 {
		result = append(result, "场景："+strings.Join(feature.SceneTags, "、"))
	}
	if len(feature.ActionTags) > 0 {
		result = append(result, "动作："+strings.Join(feature.ActionTags, "、"))
	}
	if len(feature.ProductTags) > 0 {
		result = append(result, "产品元素："+strings.Join(feature.ProductTags, "、"))
	}
	if len(feature.SellingPoints) > 0 {
		result = append(result, "识别到的卖点："+strings.Join(feature.SellingPoints, "、"))
	}
	result = append(result, feature.Evidence...)
	if feature.SimilarityRisk != "" && feature.SimilarityRisk != assets.AssetFeatureRiskLow {
		result = append(result, "相似性风险："+string(feature.SimilarityRisk))
	}
	return normalizedUnique(result)
}
