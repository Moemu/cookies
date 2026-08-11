package promptkit

import (
	"strings"
	"testing"
)

func TestRegistryResolvesEverySupportedStage(t *testing.T) {
	t.Parallel()
	values := []struct {
		stage   Stage
		version string
	}{
		{StageConversation, ConversationV4},
		{StageConversation, ConversationV5},
		{StageConversation, ConversationV6},
		{StageGenerate, GenerateV3},
		{StageGenerate, GenerateV4},
		{StageGenerate, GenerateV5},
		{StageRevise, ReviseV3},
		{StageRevise, ReviseV4},
		{StageReview, ReviewV2},
		{StageRepair, RepairV2},
		{StageRepair, RepairV3},
	}
	for _, value := range values {
		definition, err := Resolve(value.stage, value.version)
		if err != nil {
			t.Fatal(err)
		}
		if definition.System == "" || definition.Version != value.version {
			t.Fatalf("definition = %#v", definition)
		}
	}
}

func TestGenerateV4DemandsDecisionReadyCreativeDirections(t *testing.T) {
	t.Parallel()
	definition, err := Resolve(StageGenerate, GenerateV4)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"方向名｜触发场景｜内容机制｜证据或缺口｜预期动作",
		"禁止把同一想法换词重复",
		"每项只允许一个主要变量",
		"80–180 个汉字",
		"精确数字都必须逐字存在于证据上下文",
	} {
		if !strings.Contains(definition.System, expected) {
			t.Fatalf("GenerateV4 prompt omitted %q", expected)
		}
	}
}

func TestConversationV6OffersBoundedContextualCandidatesWithoutWritingThem(t *testing.T) {
	t.Parallel()
	definition, err := Resolve(StageConversation, ConversationV6)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{
		"2—3 个差异明确的候选",
		"不得输出通用模板墙",
		"不得生成 Brief operation",
	} {
		if !strings.Contains(definition.System, expected) {
			t.Fatalf("ConversationV6 prompt omitted %q", expected)
		}
	}
}

func TestRegistryRejectsUnknownVersion(t *testing.T) {
	t.Parallel()
	if _, err := Resolve(StageGenerate, "strategy.generate.unknown"); err == nil {
		t.Fatal("expected unknown prompt version to fail")
	}
}
