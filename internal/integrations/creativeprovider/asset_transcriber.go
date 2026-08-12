package creativeprovider

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/shikanon/cookies/internal/platform/media"
)

type AssetTranscriber struct {
	Assets     ViralAssetOpener
	FFmpegPath string
	WorkRoot   string
	ASR        VolcengineASR
}

func (t AssetTranscriber) Transcribe(ctx context.Context, request media.TranscriptionRequest) (media.Transcription, error) {
	if request.Validate() != nil || t.Assets == nil || strings.TrimSpace(t.FFmpegPath) == "" {
		return media.Transcription{}, fmt.Errorf("asset transcription capability is unavailable")
	}
	video, _, err := t.Assets.OpenPreview(ctx, request.Actor, request.ProjectID, request.SourceVideo)
	if err != nil {
		return media.Transcription{}, err
	}
	defer video.Close()
	workRoot := strings.TrimSpace(t.WorkRoot)
	if workRoot == "" {
		workRoot = filepath.Join(".data", "video-work")
	}
	if err := os.MkdirAll(workRoot, 0o700); err != nil {
		return media.Transcription{}, err
	}
	workDir, err := os.MkdirTemp(workRoot, "transcription-*")
	if err != nil {
		return media.Transcription{}, err
	}
	defer os.RemoveAll(workDir)
	videoPath := filepath.Join(workDir, "source.mp4")
	file, err := os.OpenFile(videoPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err == nil {
		_, err = io.Copy(file, video)
	}
	if file != nil {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		return media.Transcription{}, fmt.Errorf("stage transcription source: %w", err)
	}
	audioPath := filepath.Join(workDir, "audio.wav")
	command := exec.CommandContext(ctx, t.FFmpegPath, "-hide_banner", "-loglevel", "error", "-y",
		"-i", videoPath, "-vn", "-ac", "1", "-ar", "16000", "-c:a", "pcm_s16le", audioPath)
	if output, runErr := command.CombinedOutput(); runErr != nil {
		return media.Transcription{}, fmt.Errorf("extract transcription audio: %w: %s", runErr, strings.TrimSpace(string(output)))
	}
	audio, err := os.ReadFile(audioPath)
	if err != nil || len(audio) == 0 {
		return media.Transcription{}, fmt.Errorf("video has no transcribable audio")
	}
	text, err := t.ASR.Transcribe(ctx, audio)
	if err != nil {
		return media.Transcription{}, err
	}
	result := media.Transcription{Text: text, ProviderCode: "volcengine_asr", ModelVersion: t.ASR.Config.Model}
	if err := result.Validate(); err != nil {
		return media.Transcription{}, err
	}
	return result, nil
}
