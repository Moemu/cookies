package assets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type VideoMetadata struct {
	DurationMS   int64
	WidthPixels  int
	HeightPixels int
	FrameRate    string
	VideoCodec   string
	AudioCodec   string
}

func (m VideoMetadata) Validate() error {
	if m.DurationMS < 1 || m.WidthPixels < 1 || m.HeightPixels < 1 || strings.TrimSpace(m.FrameRate) == "" || strings.TrimSpace(m.VideoCodec) == "" {
		return fmt.Errorf("video metadata requires duration, dimensions, frame rate, and video codec")
	}
	return nil
}

type VideoMetadataProbe interface {
	Probe(context.Context, []byte) (VideoMetadata, error)
}

// FFprobeVideoProbe validates real media bytes without exposing local paths to
// Assets callers. Temporary files are removed before Probe returns.
type FFprobeVideoProbe struct {
	Path     string
	WorkRoot string
}

func (p FFprobeVideoProbe) Probe(ctx context.Context, contents []byte) (VideoMetadata, error) {
	if strings.TrimSpace(p.Path) == "" || len(contents) == 0 {
		return VideoMetadata{}, fmt.Errorf("FFprobe path and video contents are required")
	}
	workRoot := strings.TrimSpace(p.WorkRoot)
	if workRoot == "" {
		workRoot = filepath.Join(".data", "video-work")
	}
	if err := os.MkdirAll(workRoot, 0o700); err != nil {
		return VideoMetadata{}, fmt.Errorf("create video probe work directory: %w", err)
	}
	file, err := os.CreateTemp(workRoot, "probe-*.mp4")
	if err != nil {
		return VideoMetadata{}, fmt.Errorf("create video probe input: %w", err)
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return VideoMetadata{}, err
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return VideoMetadata{}, err
	}
	if err := file.Close(); err != nil {
		return VideoMetadata{}, err
	}
	output, err := exec.CommandContext(ctx, p.Path, "-v", "error", "-show_streams", "-show_format", "-of", "json", name).Output()
	if err != nil {
		return VideoMetadata{}, fmt.Errorf("FFprobe rejected video: %w", err)
	}
	var decoded struct {
		Streams []struct {
			CodecType    string `json:"codec_type"`
			CodecName    string `json:"codec_name"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
			AvgFrameRate string `json:"avg_frame_rate"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(output, &decoded); err != nil {
		return VideoMetadata{}, fmt.Errorf("decode FFprobe output: %w", err)
	}
	seconds, err := strconv.ParseFloat(decoded.Format.Duration, 64)
	if err != nil || seconds <= 0 {
		return VideoMetadata{}, fmt.Errorf("FFprobe returned invalid duration")
	}
	metadata := VideoMetadata{DurationMS: int64(seconds*1000 + 0.5)}
	for _, stream := range decoded.Streams {
		switch stream.CodecType {
		case "video":
			if metadata.VideoCodec == "" {
				metadata.VideoCodec, metadata.WidthPixels, metadata.HeightPixels, metadata.FrameRate = stream.CodecName, stream.Width, stream.Height, stream.AvgFrameRate
			}
		case "audio":
			if metadata.AudioCodec == "" {
				metadata.AudioCodec = stream.CodecName
			}
		}
	}
	if err := metadata.Validate(); err != nil {
		return VideoMetadata{}, err
	}
	return metadata, nil
}
