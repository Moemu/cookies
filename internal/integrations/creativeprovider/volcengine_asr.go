package creativeprovider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// VolcengineASR is the reusable recording-file recognition client shared by
// Creative analysis and the platform media-understanding pipeline.
type VolcengineASR struct {
	Config ASRConfig
	Client *http.Client
}

func (a VolcengineASR) Transcribe(ctx context.Context, audio []byte) (string, error) {
	if len(audio) == 0 {
		return "", fmt.Errorf("ASR audio is empty")
	}
	client := a.Client
	if client == nil {
		client = &http.Client{}
	}
	headers := map[string]string{
		"X-Api-Resource-Id": a.Config.ResourceID,
		"X-Api-Request-Id":  fmt.Sprintf("cookies-%d", time.Now().UnixNano()),
		"X-Api-Sequence":    "-1",
	}
	uid := "cookies-media-understanding"
	switch a.Config.AuthMode {
	case "legacy":
		headers["X-Api-App-Key"] = a.Config.AppID
		headers["X-Api-Access-Key"] = a.Config.AccessToken
		uid = a.Config.AppID
	case "api_key":
		headers["X-Api-Key"] = a.Config.APIKey
	default:
		return "", fmt.Errorf("unsupported ASR auth mode")
	}
	body, err := json.Marshal(map[string]any{
		"user":    map[string]string{"uid": uid},
		"audio":   map[string]string{"data": base64.StdEncoding.EncodeToString(audio)},
		"request": map[string]string{"model_name": a.Config.Model},
	})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.Config.Endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	status := response.Header.Get("X-Api-Status-Code")
	if status == "20000003" {
		return "", nil
	}
	if status != "20000000" {
		return "", fmt.Errorf("ASR returned status %s", status)
	}
	var decoded struct {
		Result struct {
			Text string `json:"text"`
		} `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&decoded); err != nil {
		return "", err
	}
	return strings.TrimSpace(decoded.Result.Text), nil
}
