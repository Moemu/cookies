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

type AudioMetadata struct {
	DurationMS int64
	Codec      string
	Channels   int
	SampleRate int
	BitrateBPS int64
}

func (m AudioMetadata) Validate() error {
	if m.DurationMS < 1 || strings.TrimSpace(m.Codec) == "" || m.Channels < 1 || m.SampleRate < 8000 {
		return fmt.Errorf("audio metadata requires duration, codec, channels, and sample rate")
	}
	return nil
}

type AudioMetadataProbe interface {
	Probe(context.Context, []byte, string) (AudioMetadata, error)
}

// FFprobeAudioProbe validates actual audio bytes and rejects containers that
// unexpectedly contain a video stream. Temporary inputs are always removed.
type FFprobeAudioProbe struct {
	Path     string
	WorkRoot string
}

func (p FFprobeAudioProbe) Probe(ctx context.Context, contents []byte, mimeType string) (AudioMetadata, error) {
	if strings.TrimSpace(p.Path) == "" || len(contents) == 0 {
		return AudioMetadata{}, fmt.Errorf("FFprobe path and audio contents are required")
	}
	extension := ".bin"
	switch mimeType {
	case "audio/wav":
		extension = ".wav"
	case "audio/mpeg":
		extension = ".mp3"
	case "audio/aac":
		extension = ".aac"
	}
	workRoot := strings.TrimSpace(p.WorkRoot)
	if workRoot == "" {
		workRoot = filepath.Join(".data", "audio-work")
	}
	if err := os.MkdirAll(workRoot, 0o700); err != nil {
		return AudioMetadata{}, fmt.Errorf("create audio probe work directory: %w", err)
	}
	file, err := os.CreateTemp(workRoot, "probe-*"+extension)
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("create audio probe input: %w", err)
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return AudioMetadata{}, err
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return AudioMetadata{}, err
	}
	if err := file.Close(); err != nil {
		return AudioMetadata{}, err
	}
	output, err := exec.CommandContext(ctx, p.Path, "-v", "error", "-show_streams", "-show_format", "-of", "json", name).Output()
	if err != nil {
		return AudioMetadata{}, fmt.Errorf("FFprobe rejected audio: %w", err)
	}
	var decoded struct {
		Streams []struct {
			CodecType  string `json:"codec_type"`
			CodecName  string `json:"codec_name"`
			Channels   int    `json:"channels"`
			SampleRate string `json:"sample_rate"`
			BitRate    string `json:"bit_rate"`
			Duration   string `json:"duration"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
	}
	if err := json.Unmarshal(output, &decoded); err != nil {
		return AudioMetadata{}, fmt.Errorf("decode FFprobe audio output: %w", err)
	}
	metadata := AudioMetadata{}
	duration := decoded.Format.Duration
	for _, stream := range decoded.Streams {
		if stream.CodecType == "video" {
			return AudioMetadata{}, fmt.Errorf("audio asset must not contain video")
		}
		if stream.CodecType != "audio" || metadata.Codec != "" {
			continue
		}
		metadata.Codec, metadata.Channels = stream.CodecName, stream.Channels
		metadata.SampleRate, _ = strconv.Atoi(stream.SampleRate)
		metadata.BitrateBPS, _ = strconv.ParseInt(stream.BitRate, 10, 64)
		if duration == "" {
			duration = stream.Duration
		}
	}
	seconds, err := strconv.ParseFloat(duration, 64)
	if err != nil || seconds <= 0 {
		return AudioMetadata{}, fmt.Errorf("FFprobe returned invalid audio duration")
	}
	metadata.DurationMS = int64(seconds*1000 + 0.5)
	if metadata.BitrateBPS == 0 {
		metadata.BitrateBPS, _ = strconv.ParseInt(decoded.Format.BitRate, 10, 64)
	}
	if err := metadata.Validate(); err != nil {
		return AudioMetadata{}, err
	}
	return metadata, nil
}

func detectAudioMIME(contents []byte) (string, bool) {
	if len(contents) >= 12 && string(contents[:4]) == "RIFF" && string(contents[8:12]) == "WAVE" {
		return "audio/wav", true
	}
	if len(contents) >= 3 && string(contents[:3]) == "ID3" {
		return "audio/mpeg", true
	}
	if len(contents) >= 2 && contents[0] == 0xff && contents[1]&0xe0 == 0xe0 {
		if contents[1]&0x16 == 0x10 {
			return "audio/aac", true
		}
		return "audio/mpeg", true
	}
	return "", false
}
