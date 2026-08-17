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
	"path/filepath"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/creative"
)

const gamePrerollV2AnalysisPrompt = `你是效果广告的游戏原视频分析器。只提取视频能证明的玩法、操作、结果、奖励、数值和 UI；推断必须标注 ai_inference，不能伪装成事实。返回单个 JSON：
{"game_name":"无法确认则写未识别游戏","gameplay_summary":"至少十个字","facts":[{"id":"fact_1","label":"核心玩法","value":"内容","provenance":"video_evidence|ai_inference","evidence_refs":["evidence_1"]}],"evidence":[{"id":"evidence_1","kind":"gameplay|operation|result|reward|ui","start_milliseconds":0,"end_milliseconds":1000,"description":"证据描述","verified_copy":["画面可核验文字"]}],"unknowns":["无法确认的内容"],"suggested_brief":[{"id":"objective","key":"objective","label":"广告目标","value":"促进游戏下载","provenance":"ai_inference","evidence_refs":[],"required":true},{"id":"audience","key":"audience","label":"目标受众","value":"受众推断","provenance":"ai_inference","evidence_refs":[],"required":true},{"id":"selling-point","key":"selling_point","label":"主推卖点","value":"视频支持的卖点","provenance":"video_evidence","evidence_refs":["evidence_1"],"required":true},{"id":"cta","key":"cta","label":"CTA","value":"立即下载","provenance":"manual","evidence_refs":[],"required":true}]}
evidence 必须恰好选择三个有先后关系的片段，优先覆盖操作、反馈/结果和收束画面；所有时间必须来自输入帧时间且 end>start。没有奖励或数值就加入 unknowns，不得补写。只返回 JSON，不得返回 Markdown。`

type GamePrerollV2Analyzer struct {
	helper  *ShortDramaV2Analyzer
	sampler *CommercePrerollV2Analyzer
}

func NewGamePrerollV2Analyzer(config ViralAnalyzerConfig) (*GamePrerollV2Analyzer, error) {
	helper, err := NewShortDramaV2Analyzer(config)
	if err != nil {
		return nil, err
	}
	return &GamePrerollV2Analyzer{helper: helper, sampler: &CommercePrerollV2Analyzer{helper: helper}}, nil
}

func (a *GamePrerollV2Analyzer) Analyze(ctx context.Context, actor contract.ActorContext, project contract.ProjectContext, source contract.ProjectAssetRef) (creative.GamePrerollV2AnalysisResult, error) {
	if source.ProjectID != project.ProjectID || source.Validate() != nil {
		return creative.GamePrerollV2AnalysisResult{}, fmt.Errorf("game source video reference is invalid")
	}
	video, _, err := a.helper.config.Assets.OpenPreview(ctx, actor, project.ProjectID, source.AssetVersion)
	if err != nil {
		return creative.GamePrerollV2AnalysisResult{}, fmt.Errorf("game source video cannot be opened: %w", err)
	}
	defer video.Close()
	if err = os.MkdirAll(a.helper.config.WorkRoot, 0o750); err != nil {
		return creative.GamePrerollV2AnalysisResult{}, err
	}
	workDir, err := os.MkdirTemp(a.helper.config.WorkRoot, "game-preroll-v2-*")
	if err != nil {
		return creative.GamePrerollV2AnalysisResult{}, err
	}
	defer os.RemoveAll(workDir)
	videoPath := filepath.Join(workDir, "source.mp4")
	file, err := os.Create(videoPath)
	if err != nil {
		return creative.GamePrerollV2AnalysisResult{}, err
	}
	if _, err = io.Copy(file, video); err != nil {
		_ = file.Close()
		return creative.GamePrerollV2AnalysisResult{}, err
	}
	if err = file.Close(); err != nil {
		return creative.GamePrerollV2AnalysisResult{}, err
	}
	transcript, _ := a.helper.helper.extractTranscript(ctx, videoPath, workDir)
	frames, err := a.sampler.extractFrames(ctx, videoPath, workDir)
	if err != nil || len(frames) < 3 {
		return creative.GamePrerollV2AnalysisResult{}, fmt.Errorf("game source frames cannot be extracted: %w", err)
	}
	result, err := a.callModel(ctx, actor, transcript, frames)
	if err != nil {
		return creative.GamePrerollV2AnalysisResult{}, err
	}
	normalizeGamePrerollAnalysis(&result)
	hash, err := contract.CanonicalJSONHash(struct {
		Source contract.ProjectAssetRef `json:"source"`
		Prompt string                   `json:"prompt"`
	}{source, "game-preroll-analysis/v1"})
	if err != nil {
		return creative.GamePrerollV2AnalysisResult{}, err
	}
	result.InputHash = "sha256:" + hash
	result.PromptVersion = "game-preroll-analysis/v1"
	return result, nil
}

func normalizeGamePrerollAnalysis(result *creative.GamePrerollV2AnalysisResult) {
	for index := range result.SuggestedBrief {
		if result.SuggestedBrief[index].Key != "cta" {
			continue
		}
		result.SuggestedBrief[index].Value = "立即下载"
		result.SuggestedBrief[index].Provenance = creative.GameProvenanceManual
		result.SuggestedBrief[index].EvidenceRefs = []string{}
		result.SuggestedBrief[index].Required = true
		return
	}
	result.SuggestedBrief = append(result.SuggestedBrief, creative.GameBriefField{
		ID: "cta", Key: "cta", Label: "CTA", Value: "立即下载",
		Provenance: creative.GameProvenanceManual, EvidenceRefs: []string{}, Required: true,
	})
}

func (a *GamePrerollV2Analyzer) callModel(ctx context.Context, actor contract.ActorContext, transcript string, frames []commerceSampledFrame) (creative.GamePrerollV2AnalysisResult, error) {
	route, err := a.helper.config.Routes.ResolveTextRoute(ctx, actor.OrganizationID, a.helper.config.ModelAlias)
	if err != nil {
		return creative.GamePrerollV2AnalysisResult{}, err
	}
	if err = route.ValidateTextWithPolicy(a.helper.config.AllowInsecureHTTP); err != nil {
		return creative.GamePrerollV2AnalysisResult{}, err
	}
	token, err := a.helper.config.Credentials.ResolveGatewayCredential(ctx, route.CredentialID, route.CredentialVersion)
	if err != nil {
		return creative.GamePrerollV2AnalysisResult{}, err
	}
	content := []any{map[string]any{"type": "text", "text": "ASR 转写（只作证据，可能为空）：" + transcript}}
	for i, frame := range frames {
		content = append(content, map[string]any{"type": "text", "text": fmt.Sprintf("frame_%d timestamp_ms=%d", i+1, frame.TimestampMS)}, map[string]any{"type": "image_url", "image_url": map[string]string{"url": "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(frame.Content)}})
	}
	payload := map[string]any{"model": route.UpstreamModel, "messages": []any{map[string]any{"role": "system", "content": gamePrerollV2AnalysisPrompt}, map[string]any{"role": "user", "content": content}}}
	if err = applyShortDramaTextRouteConstraints(payload, route); err != nil {
		return creative.GamePrerollV2AnalysisResult{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return creative.GamePrerollV2AnalysisResult{}, err
	}
	timeout, cancel := context.WithTimeout(ctx, time.Duration(route.TimeoutSeconds)*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(timeout, http.MethodPost, route.ChatCompletionsEndpoint(), bytes.NewReader(body))
	if err != nil {
		return creative.GamePrerollV2AnalysisResult{}, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	response, err := a.helper.doModelRequestWithRetry(timeout, request, body)
	if err != nil {
		return creative.GamePrerollV2AnalysisResult{}, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, route.MaxResponseBytes+1))
	if err != nil || int64(len(responseBody)) > route.MaxResponseBytes || response.StatusCode < 200 || response.StatusCode >= 300 {
		return creative.GamePrerollV2AnalysisResult{}, fmt.Errorf("game analysis model response is unavailable")
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(responseBody, &envelope) != nil || len(envelope.Choices) == 0 {
		return creative.GamePrerollV2AnalysisResult{}, fmt.Errorf("game analysis model response envelope is invalid")
	}
	text := strings.TrimSpace(envelope.Choices[0].Message.Content)
	text = strings.TrimPrefix(strings.TrimPrefix(text, "```json"), "```")
	text = strings.TrimSuffix(text, "```")
	var result creative.GamePrerollV2AnalysisResult
	if err = json.Unmarshal([]byte(strings.TrimSpace(text)), &result); err != nil {
		return creative.GamePrerollV2AnalysisResult{}, fmt.Errorf("game analysis model response is invalid: %w", err)
	}
	return result, nil
}

var _ creative.GamePrerollV2Analyzer = (*GamePrerollV2Analyzer)(nil)
