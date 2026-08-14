package creative

import (
	"fmt"
	"math"
)

const (
	ShortDramaPrerollV3ContractVersion = "creative-short-drama-preroll-workspace/v3"
	ShortDramaGenerationSpecV3         = "creative-short-drama-preroll-generation-spec/v3"
	ShortDramaPrerollV4ContractVersion = "creative-short-drama-preroll-workspace/v4"
	ShortDramaGenerationSpecV4         = "creative-short-drama-preroll-generation-spec/v4"
)

type ShortDramaSourceCanvas struct {
	WidthPixels  int     `json:"width_pixels"`
	HeightPixels int     `json:"height_pixels"`
	AspectRatio  float64 `json:"aspect_ratio"`
	DurationMS   int64   `json:"duration_ms"`
	FrameRate    string  `json:"frame_rate,omitempty"`
	VideoCodec   string  `json:"video_codec,omitempty"`
	AudioCodec   string  `json:"audio_codec,omitempty"`
	ProbeStatus  string  `json:"probe_status"`
}

type ShortDramaModelCanvas struct {
	Ratio       string `json:"ratio"`
	Resolution  string `json:"resolution"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	ImageWidth  int    `json:"image_width"`
	ImageHeight int    `json:"image_height"`
}

type ShortDramaOutputCanvas struct {
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	AspectNum     int    `json:"aspect_num"`
	AspectDen     int    `json:"aspect_den"`
	FrameRate     int    `json:"frame_rate"`
	VideoCodec    string `json:"video_codec"`
	PixelFormat   string `json:"pixel_format"`
	AudioCodec    string `json:"audio_codec"`
	NormalizeMode string `json:"normalize_mode"`
}

// ShortDramaBoardCanvas preserves the complete 2x2 reference board. Unlike an
// opening frame, a board must never be cover-cropped to the source ratio or a
// panel may disappear before it reaches the video model.
type ShortDramaBoardCanvas struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	AspectRatio string `json:"aspect_ratio"`
	Layout      string `json:"layout"`
	FitMode     string `json:"fit_mode"`
	SafeInsetPX int    `json:"safe_inset_px"`
}

func deriveShortDramaBoardCanvas(model ShortDramaModelCanvas) ShortDramaBoardCanvas {
	inset := model.ImageWidth / 64
	if inset < 12 {
		inset = 12
	}
	return ShortDramaBoardCanvas{
		Width: model.ImageWidth, Height: model.ImageHeight, AspectRatio: model.Ratio,
		Layout: "2x2_v1", FitMode: "contain_panels", SafeInsetPX: inset,
	}
}

func deriveShortDramaCanvases(metadata CreativeAssetSnapshot) (ShortDramaSourceCanvas, ShortDramaModelCanvas, ShortDramaOutputCanvas, error) {
	if metadata.Kind != "video" || !metadata.Ready || metadata.WidthPixels < 2 || metadata.HeightPixels < 2 || metadata.DurationMS < 1 {
		return ShortDramaSourceCanvas{}, ShortDramaModelCanvas{}, ShortDramaOutputCanvas{}, fmt.Errorf("short drama source video metadata is incomplete")
	}
	sourceRatio := float64(metadata.WidthPixels) / float64(metadata.HeightPixels)
	type candidate struct {
		ratio       string
		value       float64
		width       int
		height      int
		imageWidth  int
		imageHeight int
	}
	candidates := []candidate{
		{ratio: "9:16", value: 9.0 / 16.0, width: 720, height: 1280, imageWidth: 1024, imageHeight: 1536},
		{ratio: "1:1", value: 1, width: 720, height: 720, imageWidth: 1024, imageHeight: 1024},
		{ratio: "16:9", value: 16.0 / 9.0, width: 1280, height: 720, imageWidth: 1536, imageHeight: 1024},
	}
	selected := candidates[0]
	best := math.Abs(math.Log(sourceRatio / selected.value))
	for _, value := range candidates[1:] {
		score := math.Abs(math.Log(sourceRatio / value.value))
		if score < best {
			selected, best = value, score
		}
	}
	outputWidth, outputHeight := selected.width, selected.height
	if sourceRatio >= selected.value {
		outputHeight = roundToEven(float64(outputWidth) / sourceRatio)
	} else {
		outputWidth = roundToEven(float64(outputHeight) * sourceRatio)
	}
	if outputWidth < 2 || outputHeight < 2 || outputWidth > selected.width || outputHeight > selected.height {
		return ShortDramaSourceCanvas{}, ShortDramaModelCanvas{}, ShortDramaOutputCanvas{}, fmt.Errorf("short drama source aspect ratio is unsupported")
	}
	divisor := greatestCommonDivisor(metadata.WidthPixels, metadata.HeightPixels)
	source := ShortDramaSourceCanvas{
		WidthPixels: metadata.WidthPixels, HeightPixels: metadata.HeightPixels, AspectRatio: sourceRatio,
		DurationMS: metadata.DurationMS, FrameRate: metadata.FrameRate, VideoCodec: metadata.VideoCodec,
		AudioCodec: metadata.AudioCodec, ProbeStatus: "ready",
	}
	model := ShortDramaModelCanvas{
		Ratio: selected.ratio, Resolution: "720p", Width: selected.width, Height: selected.height,
		ImageWidth: selected.imageWidth, ImageHeight: selected.imageHeight,
	}
	output := ShortDramaOutputCanvas{
		Width: outputWidth, Height: outputHeight, AspectNum: metadata.WidthPixels / divisor, AspectDen: metadata.HeightPixels / divisor,
		FrameRate: normalizedShortDramaFrameRate(metadata.FrameRate), VideoCodec: "h264", PixelFormat: "yuv420p",
		AudioCodec: "aac", NormalizeMode: "cover_crop",
	}
	return source, model, output, nil
}

func roundToEven(value float64) int {
	rounded := int(math.Round(value))
	if rounded%2 != 0 {
		rounded++
	}
	return rounded
}

func greatestCommonDivisor(left, right int) int {
	for right != 0 {
		left, right = right, left%right
	}
	if left < 1 {
		return 1
	}
	return left
}

func normalizedShortDramaFrameRate(value string) int {
	switch value {
	case "24/1", "24":
		return 24
	case "25/1", "25":
		return 25
	case "30/1", "30000/1001", "30":
		return 30
	default:
		return 30
	}
}
