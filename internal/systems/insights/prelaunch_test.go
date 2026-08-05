package insights

import (
	"errors"
	"testing"
)

func TestRecommendationWithoutActionIsRejected(t *testing.T) {
	t.Parallel()
	err := CreateExperienceRequest{
		Conclusion: "把首图换成单一利益点。",
		CardType:   CardRecommendation,
		Confidence: ConfidenceDirectional,
	}.Validate()
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("「建议」缺建议动作应当被拒，得到 %v", err)
	}
}

// 「事实」和「统计观察」是能被下游当证据引用的两类卡。没有数据依据的
// 「事实」其实是假设，放进去会让引用它的人以为背后有数据。
func TestEvidenceGradeCardNeedsDataBasis(t *testing.T) {
	t.Parallel()
	for _, cardType := range []InsightCardType{CardFact, CardStatistic} {
		err := CreateExperienceRequest{
			Conclusion: "竖版素材的完播率高于横版。",
			CardType:   cardType, Confidence: ConfidenceSufficient,
		}.Validate()
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("%s 缺数据依据应当被拒，得到 %v", cardType.Label(), err)
		}
		err = CreateExperienceRequest{
			Conclusion: "竖版素材的完播率高于横版。",
			CardType:   cardType, Confidence: ConfidenceSufficient,
			DataBasis: DataBasis{SampleSize: 12000, Metrics: []string{"完播率"}},
		}.Validate()
		if err != nil {
			t.Fatalf("%s 带数据依据应当通过，得到 %v", cardType.Label(), err)
		}
	}
}

func TestHypothesisCannotClaimSufficientConfidence(t *testing.T) {
	t.Parallel()
	err := CreateExperienceRequest{
		Conclusion: "也许开头三秒露脸能提升完播。",
		CardType:   CardHypothesis, Confidence: ConfidenceSufficient,
	}.Validate()
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("假设不可能「充分」，得到 %v", err)
	}
}

// 不填类型和置信要落到最保守的一格。老调用方（复盘报告沉淀）没有这两个字段，
// 默认成「事实/充分」等于替录入的人做了他没做过的判断。
func TestMissingCardTypeDefaultsToTheMostConservativeCell(t *testing.T) {
	t.Parallel()
	filled := CreateExperienceRequest{Conclusion: "首图保持单一利益点。"}.withCardDefaults()
	if filled.CardType != CardHypothesis || filled.Confidence != ConfidenceDirectional {
		t.Fatalf("默认应为 假设 + 方向性，得到 %s / %s", filled.CardType, filled.Confidence)
	}
}

func TestFilterExcludesUnscopedExperiencesButReportsHowMany(t *testing.T) {
	t.Parallel()
	scoped := Experience{Applicability: Applicability{Channels: []string{"douyin"}}}
	unscoped := Experience{}

	match, unknown := matchesPreLaunchFilter(scoped, PreLaunchFilter{Channel: "douyin"})
	if !match || unknown {
		t.Fatalf("命中渠道的经验应当返回：match=%v unscoped=%v", match, unknown)
	}
	match, unknown = matchesPreLaunchFilter(unscoped, PreLaunchFilter{Channel: "douyin"})
	if match || !unknown {
		t.Fatalf("没写适用范围的经验应当被单独计数：match=%v unscoped=%v", match, unknown)
	}
	// 不带筛选时，没写适用范围的经验照常出现——它只是没说自己适用于哪里。
	if match, unknown := matchesPreLaunchFilter(unscoped, PreLaunchFilter{}); !match || unknown {
		t.Fatalf("无筛选时不应排除：match=%v unscoped=%v", match, unknown)
	}
}

func TestCardReportsMissingFieldsInsteadOfHidingTheCard(t *testing.T) {
	t.Parallel()
	card := buildInsightCard(Experience{
		Conclusion: "首图保持单一利益点。", CardType: CardHypothesis, Confidence: ConfidenceDirectional,
	}, 0)
	want := map[string]bool{"适用范围": true, "数据依据": true, "内容依据": true, "风险与反例": true, "建议动作": true}
	if len(card.MissingFields) != len(want) {
		t.Fatalf("缺失字段 = %v，想要 %d 个", card.MissingFields, len(want))
	}
	for _, field := range card.MissingFields {
		if !want[field] {
			t.Fatalf("意外的缺失字段 %s", field)
		}
	}
	if card.Conclusion == "" {
		t.Fatal("不合格的卡也要返回，不能藏起来")
	}
}

// 历史模式按出现次数排，但取的是「最强置信」而不是平均——一条充分证据
// 和一条样本不足的观察加在一起不该被平均成方向性。
func TestFeaturePatternsCountAppearancesAndKeepStrongestConfidence(t *testing.T) {
	t.Parallel()
	patterns := buildFeaturePatterns([]Experience{
		{Confidence: ConfidenceLowSample, Applicability: Applicability{Channels: []string{"douyin"}},
			ContentBasis: ContentBasis{Features: []string{"开头露脸", "字幕加粗"}}},
		{Confidence: ConfidenceSufficient, Applicability: Applicability{Channels: []string{"xiaohongshu"}},
			ContentBasis: ContentBasis{Features: []string{"开头露脸"}}},
	})
	if len(patterns) != 2 || patterns[0].Feature != "开头露脸" || patterns[0].CardCount != 2 {
		t.Fatalf("模式 = %#v", patterns)
	}
	if patterns[0].BestConfidence != ConfidenceSufficient {
		t.Fatalf("最强置信 = %s，想要 %s", patterns[0].BestConfidence, ConfidenceSufficient)
	}
	if len(patterns[0].Channels) != 2 {
		t.Fatalf("跨渠道出现的特征要把渠道都列出来：%v", patterns[0].Channels)
	}
}
