package main

import (
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/systems/creative"
)

func TestShortDramaV2FirstFrameInputUsesRequestedSupportedDimensions(t *testing.T) {
	t.Parallel()

	input := shortDramaV2FirstFrameInput("武则天站在无字碑前", 1536, 1024)
	if input.Width != 1536 || input.Height != 1024 {
		t.Fatalf("short drama V2 first-frame size = %dx%d, want source-oriented 1536x1024", input.Width, input.Height)
	}
	if input.Prompt != "武则天站在无字碑前" {
		t.Fatalf("short drama V2 first-frame prompt changed: %q", input.Prompt)
	}
}

func TestShortDramaV2FirstFrameSourceTaskIDDoesNotIncludeLongCandidateLineage(t *testing.T) {
	t.Parallel()

	request := creative.ShortDramaV2FirstFrameJobRequest{
		TaskID: "creativetask_1234567890", CandidateID: strings.Repeat("candidate-lineage-", 12), VariantIndex: 1,
	}
	value := shortDramaV2FirstFrameSourceTaskID(request)
	if value != request.TaskID || len(value) > 96 {
		t.Fatalf("provider source_task_id = %q (%d bytes), want stable task ID within VARCHAR(96)", value, len(value))
	}
}
