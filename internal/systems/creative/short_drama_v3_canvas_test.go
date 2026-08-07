package creative

import (
	"math"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestDeriveShortDramaCanvasesUsesSourceAspectRatio(t *testing.T) {
	t.Parallel()

	source, model, output, err := deriveShortDramaCanvases(CreativeAssetSnapshot{
		Kind: contract.AssetVideo, Ready: true, WidthPixels: 1920, HeightPixels: 818,
		DurationMS: 182000, FrameRate: "25/1", VideoCodec: "h264", AudioCodec: "aac",
	})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(source.AspectRatio-(1920.0/818.0)) > 0.000001 || model.Ratio != "16:9" || model.Width != 1280 || model.Height != 720 {
		t.Fatalf("source/model canvas = %#v / %#v", source, model)
	}
	if output.Width != 1280 || output.Height != 546 || output.AspectNum != 960 || output.AspectDen != 409 || output.FrameRate != 25 {
		t.Fatalf("output canvas = %#v, want 1280x546 source-ratio output", output)
	}
}

func TestDeriveShortDramaCanvasesCoversStandardOrientations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		width, height    int
		ratio            string
		outputW, outputH int
		imageW, imageH   int
	}{
		{name: "vertical", width: 1080, height: 1920, ratio: "9:16", outputW: 720, outputH: 1280, imageW: 1024, imageH: 1536},
		{name: "square", width: 1080, height: 1080, ratio: "1:1", outputW: 720, outputH: 720, imageW: 1024, imageH: 1024},
		{name: "landscape", width: 1920, height: 1080, ratio: "16:9", outputW: 1280, outputH: 720, imageW: 1536, imageH: 1024},
		{name: "four-three", width: 1440, height: 1080, ratio: "1:1", outputW: 720, outputH: 540, imageW: 1024, imageH: 1024},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, model, output, err := deriveShortDramaCanvases(CreativeAssetSnapshot{
				Kind: contract.AssetVideo, Ready: true, WidthPixels: test.width, HeightPixels: test.height, DurationMS: 1000,
			})
			if err != nil {
				t.Fatal(err)
			}
			if model.Ratio != test.ratio || model.ImageWidth != test.imageW || model.ImageHeight != test.imageH || output.Width != test.outputW || output.Height != test.outputH || output.Width%2 != 0 || output.Height%2 != 0 {
				t.Fatalf("model/output = %#v / %#v", model, output)
			}
		})
	}
}
