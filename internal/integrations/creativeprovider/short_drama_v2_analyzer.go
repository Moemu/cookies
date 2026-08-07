package creativeprovider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/creative"
)

const shortDramaV2AnalysisPrompt = `你是短剧视频理解编导。根据全片间隔抽帧和 ASR 转写，提炼能够支撑前贴创作的真实剧情事实。不得猜测未出现的人物、关系、事件或结局。

只返回一个 JSON 对象，并严格包含以下字段：
{
  "title": "根据画面和转写识别的短剧标题；若原片未明确展示标题，则用真实剧情事实概括命名",
  "episode": "集数，无法确认时返回空字符串",
  "synopsis": "不少于 40 个汉字的全片剧情梗概",
  "opening_beat": "开场已经发生的动作或信息",
  "core_conflict": "全片真实呈现的核心冲突",
  "unresolved_hook": "适合引流且未剧透结局的信息缺口",
  "tone": "主要情绪与类型",
  "characters": [{"name":"人物称谓或姓名","description":"只写可观察事实","relationship":"可确认关系，无法确认时留空"}],
  "visual_keywords": ["可观察的场景、服饰、时代或动作关键词"],
  "evidence": [{"id":"frame_1","timestamp_ms":0,"transcript":"该证据实际支持的简短事实"}]
}

title、synopsis、opening_beat、core_conflict、unresolved_hook 和 evidence 不得为空。evidence.id 只能使用输入提供的 transcript_1、frame_1 至 frame_8；timestamp_ms 必须与输入给出的秒数对应。不要输出 Markdown。`

type ShortDramaV2Analyzer struct {
	config ViralAnalyzerConfig
	helper ViralAnalyzer
}

func NewShortDramaV2Analyzer(config ViralAnalyzerConfig) (*ShortDramaV2Analyzer, error) {
	helper, err := NewViralAnalyzer(config)
	if err != nil {
		return nil, err
	}
	return &ShortDramaV2Analyzer{config: helper.config, helper: *helper}, nil
}

func (a *ShortDramaV2Analyzer) Analyze(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, source contract.ProjectAssetRef) (creative.ShortDramaV2AnalysisResult, error) {
	if source.ProjectID != project.ProjectID || source.Validate() != nil {
		return creative.ShortDramaV2AnalysisResult{}, fmt.Errorf("short drama source video is invalid")
	}
	video, _, err := a.config.Assets.OpenPreview(ctx, actor, project.ProjectID, source.AssetVersion)
	if err != nil {
		return creative.ShortDramaV2AnalysisResult{}, fmt.Errorf("open short drama source: %w", err)
	}
	defer video.Close()
	if err := os.MkdirAll(a.config.WorkRoot, 0o750); err != nil {
		return creative.ShortDramaV2AnalysisResult{}, err
	}
	workDir, err := os.MkdirTemp(a.config.WorkRoot, "short-drama-v2-*")
	if err != nil {
		return creative.ShortDramaV2AnalysisResult{}, err
	}
	defer os.RemoveAll(workDir)
	videoPath := filepath.Join(workDir, "source.mp4")
	file, err := os.Create(videoPath)
	if err != nil {
		return creative.ShortDramaV2AnalysisResult{}, err
	}
	if _, err = io.Copy(file, video); err != nil {
		_ = file.Close()
		return creative.ShortDramaV2AnalysisResult{}, err
	}
	if err := file.Close(); err != nil {
		return creative.ShortDramaV2AnalysisResult{}, err
	}
	transcript, _ := a.helper.extractTranscript(ctx, videoPath, workDir)
	frames, err := a.extractWholeVideoFrames(ctx, videoPath, workDir)
	if err != nil || len(frames) == 0 {
		return creative.ShortDramaV2AnalysisResult{}, fmt.Errorf("extract short drama frames: %w", err)
	}
	result, err := a.callModel(ctx, actor, source, transcript, frames)
	if err != nil {
		return creative.ShortDramaV2AnalysisResult{}, err
	}
	hash, err := contract.CanonicalJSONHash(struct {
		Source        contract.ProjectAssetRef `json:"source"`
		PromptVersion string                   `json:"prompt_version"`
	}{source, "short-drama-analysis/v1"})
	if err != nil {
		return creative.ShortDramaV2AnalysisResult{}, err
	}
	result.InputHash = "sha256:" + hash
	result.PromptVersion = "short-drama-analysis/v1"
	return result, nil
}

func (a *ShortDramaV2Analyzer) extractWholeVideoFrames(ctx context.Context, videoPath, workDir string) ([][]byte, error) {
	pattern := filepath.Join(workDir, "story-frame-%02d.jpg")
	// One frame every 24 seconds covers the provided 182-second short drama with
	// eight representative observations instead of only sampling its opening.
	command := exec.CommandContext(ctx, a.config.FFmpegPath, "-hide_banner", "-loglevel", "error", "-y", "-i", videoPath,
		"-vf", "fps=1/24,scale=640:-2", "-q:v", "4", "-frames:v", "8", pattern)
	if output, err := command.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(string(output)))
	}
	paths, err := filepath.Glob(filepath.Join(workDir, "story-frame-*.jpg"))
	if err != nil {
		return nil, err
	}
	frames := make([][]byte, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		frames = append(frames, content)
	}
	return frames, nil
}

func (a *ShortDramaV2Analyzer) callModel(ctx context.Context, actor contract.ActorContext, source contract.ProjectAssetRef, transcript string, frames [][]byte) (creative.ShortDramaV2AnalysisResult, error) {
	route, err := a.config.Routes.ResolveTextRoute(ctx, actor.OrganizationID, a.config.ModelAlias)
	if err != nil {
		return creative.ShortDramaV2AnalysisResult{}, err
	}
	if err := route.ValidateTextWithPolicy(a.config.AllowInsecureHTTP); err != nil {
		return creative.ShortDramaV2AnalysisResult{}, err
	}
	token, err := a.config.Credentials.ResolveGatewayCredential(ctx, route.CredentialID, route.CredentialVersion)
	if err != nil {
		return creative.ShortDramaV2AnalysisResult{}, err
	}
	content := []any{map[string]any{"type": "text", "text": "evidence id=transcript_1，ASR转写：" + transcript}}
	for index, frame := range frames {
		content = append(content,
			map[string]any{"type": "text", "text": fmt.Sprintf("下一张图的 evidence id=frame_%d，约位于全片第%d秒", index+1, index*24)},
			map[string]any{"type": "image_url", "image_url": map[string]string{"url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(frame)}},
		)
	}
	payload := map[string]any{"model": route.UpstreamModel, "messages": []any{
		map[string]any{"role": "system", "content": shortDramaV2AnalysisPrompt},
		map[string]any{"role": "user", "content": content},
	}, "temperature": 0.2, "response_format": map[string]any{"type": "json_object"}}
	body, err := json.Marshal(payload)
	if err != nil {
		return creative.ShortDramaV2AnalysisResult{}, err
	}
	endpoint := strings.TrimRight(route.BaseURL, "/")
	if strings.HasSuffix(endpoint, "/v1") {
		endpoint += "/chat/completions"
	} else {
		endpoint += "/v1/chat/completions"
	}
	timeout, cancel := context.WithTimeout(ctx, time.Duration(route.TimeoutSeconds)*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(timeout, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return creative.ShortDramaV2AnalysisResult{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := a.config.Client.Do(request)
	if err != nil {
		return creative.ShortDramaV2AnalysisResult{}, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, route.MaxResponseBytes+1))
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return creative.ShortDramaV2AnalysisResult{}, fmt.Errorf("short drama model request failed with status %d", response.StatusCode)
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &envelope); err != nil || len(envelope.Choices) == 0 {
		return creative.ShortDramaV2AnalysisResult{}, fmt.Errorf("short drama model response is invalid")
	}
	text := strings.TrimSpace(envelope.Choices[0].Message.Content)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	var result creative.ShortDramaV2AnalysisResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &result.Content); err != nil {
		return creative.ShortDramaV2AnalysisResult{}, fmt.Errorf("decode short drama analysis: %w", err)
	}
	return result, nil
}

var _ creative.ShortDramaV2Analyzer = (*ShortDramaV2Analyzer)(nil)
