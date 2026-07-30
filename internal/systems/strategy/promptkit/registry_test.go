package promptkit

import "testing"

func TestRegistryResolvesEverySupportedStage(t *testing.T) {
	t.Parallel()
	values := []struct {
		stage   Stage
		version string
	}{
		{StageConversation, ConversationV4},
		{StageGenerate, GenerateV3},
		{StageRevise, ReviseV3},
		{StageReview, ReviewV2},
		{StageRepair, RepairV2},
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

func TestRegistryRejectsUnknownVersion(t *testing.T) {
	t.Parallel()
	if _, err := Resolve(StageGenerate, "strategy.generate.unknown"); err == nil {
		t.Fatal("expected unknown prompt version to fail")
	}
}
