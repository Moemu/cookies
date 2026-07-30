package provider

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestProviderJobInputRoundTripsByOperation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		record    JobRecord
		assertion func(*testing.T, JobRecord)
	}{
		{
			name:   "image",
			record: JobRecord{Operation: imageGenerateOperation, Input: ImageGenerationInput{Prompt: "poster", Width: 1024, Height: 1024}},
			assertion: func(t *testing.T, got JobRecord) {
				t.Helper()
				if !reflect.DeepEqual(got.Input, ImageGenerationInput{Prompt: "poster", Width: 1024, Height: 1024}) {
					t.Fatalf("image input = %+v", got.Input)
				}
			},
		},
		{
			name:   "video",
			record: JobRecord{Operation: videoGenerateOperation, VideoInput: VideoGenerationInput{Prompt: "pre-roll", DurationSeconds: 5, AspectRatio: "9:16", Resolution: "720p"}},
			assertion: func(t *testing.T, got JobRecord) {
				t.Helper()
				if !reflect.DeepEqual(got.VideoInput, VideoGenerationInput{Prompt: "pre-roll", DurationSeconds: 5, AspectRatio: "9:16", Resolution: "720p"}) {
					t.Fatalf("video input = %+v", got.VideoInput)
				}
				if !reflect.DeepEqual(got.Input, ImageGenerationInput{}) {
					t.Fatalf("video record unexpectedly populated image input: %+v", got.Input)
				}
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			payload, err := marshalProviderInput(test.record)
			if err != nil {
				t.Fatalf("marshalProviderInput() error = %v", err)
			}
			var got JobRecord
			got.Operation = test.record.Operation
			if err := unmarshalProviderInput(json.RawMessage(payload), &got); err != nil {
				t.Fatalf("unmarshalProviderInput() error = %v", err)
			}
			test.assertion(t, got)
		})
	}
}
