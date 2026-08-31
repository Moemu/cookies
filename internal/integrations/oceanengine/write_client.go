package oceanengine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	SecSDKVersion  = "1.2.22"
	writeUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0"

	ProjectCreatePath   = "/superior/api/v2/project/create"
	ProjectListPath     = "/superior/api/v2/project/list"
	PromotionCreatePath = "/superior/api/v2/promotion/create_promotion"
	PromotionListPath   = "/superior/api/ad/promotion/list"
)

var (
	ErrCSRFTokenInvalid = errors.New("Ocean Engine CSRF token is invalid")
	ErrWriteForbidden   = errors.New("Ocean Engine write endpoint is forbidden")
	ErrResultUnknown    = errors.New("Ocean Engine write result is unknown")
	ErrAuthRequired     = errors.New("Ocean Engine authentication is required")
)

var writeEndpoints = map[Endpoint]struct{}{
	{Method: http.MethodPost, Path: ProjectCreatePath}:   {},
	{Method: http.MethodPost, Path: PromotionCreatePath}: {},
}

type csrfCacheKey struct {
	host           string
	advertiserID   string
	sessionVersion int64
}

type csrfCacheValue struct {
	token     string
	expiresAt time.Time
}

// CSRFTokenCache stores Secsdk tokens in process memory only.
type CSRFTokenCache struct {
	mu     sync.Mutex
	values map[csrfCacheKey]csrfCacheValue
}

func NewCSRFTokenCache() *CSRFTokenCache {
	return &CSRFTokenCache{values: make(map[csrfCacheKey]csrfCacheValue)}
}

type WriteClient struct {
	BaseURL        *url.URL
	HTTPClient     *http.Client
	Session        Session
	AdvertiserID   string
	SessionVersion int64
	UserAgent      string
	TokenCache     *CSRFTokenCache
	Now            func() time.Time
}

type WriteResponse struct {
	StatusCode int
	Body       json.RawMessage
}

func NewWriteClient(rawBaseURL, advertiserID string, sessionVersion int64, session Session, httpClient *http.Client, cache *CSRFTokenCache) (*WriteClient, error) {
	baseURL, err := url.Parse(rawBaseURL)
	if err != nil || baseURL.Scheme != "https" && baseURL.Scheme != "http" || baseURL.Host == "" || strings.TrimSpace(advertiserID) == "" || sessionVersion < 1 || strings.TrimSpace(session.Cookies) == "" {
		return nil, fmt.Errorf("invalid Ocean Engine write client configuration")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	copyClient := *httpClient
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if cache == nil {
		cache = NewCSRFTokenCache()
	}
	return &WriteClient{
		BaseURL: baseURL, HTTPClient: &copyClient, Session: session, AdvertiserID: advertiserID,
		SessionVersion: sessionVersion, UserAgent: writeUserAgent, TokenCache: cache, Now: time.Now,
	}, nil
}

// Close removes plaintext session material from the client.
func (c *WriteClient) Close() {
	c.Session.Cookies = ""
	c.Session.CSRFToken = ""
}

// SubmitJSON sends one protected POST. It never retries the write.
func (c *WriteClient) SubmitJSON(ctx context.Context, path string, payload any) (WriteResponse, error) {
	if _, ok := writeEndpoints[Endpoint{Method: http.MethodPost, Path: path}]; !ok {
		return WriteResponse{}, fmt.Errorf("%w: POST %s", ErrWriteForbidden, path)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return WriteResponse{}, fmt.Errorf("encode Ocean Engine write request: %w", err)
	}
	token, err := c.csrfToken(ctx, path)
	if err != nil {
		return WriteResponse{}, err
	}
	req, err := c.newRequest(ctx, http.MethodPost, path, bytes.NewReader(body))
	if err != nil {
		return WriteResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-secsdk-csrf-token", token)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return WriteResponse{}, fmt.Errorf("%w: write transport failed", ErrResultUnknown)
	}
	defer resp.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if readErr != nil {
		return WriteResponse{}, fmt.Errorf("%w: write response could not be read", ErrResultUnknown)
	}
	result := WriteResponse{StatusCode: resp.StatusCode, Body: json.RawMessage(responseBody)}
	if authResponse(resp) {
		return result, ErrAuthRequired
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return result, fmt.Errorf("%w: HTTP %d", ErrResultUnknown, resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return result, HTTPStatusError{StatusCode: resp.StatusCode}
	}
	return result, nil
}

func (c *WriteClient) csrfToken(ctx context.Context, protectedPath string) (string, error) {
	now := c.Now().UTC()
	key := csrfCacheKey{host: c.BaseURL.Host, advertiserID: c.AdvertiserID, sessionVersion: c.SessionVersion}
	c.TokenCache.mu.Lock()
	if cached, ok := c.TokenCache.values[key]; ok && cached.token != "" && now.Before(cached.expiresAt) {
		c.TokenCache.mu.Unlock()
		return cached.token, nil
	}
	c.TokenCache.mu.Unlock()

	// Secsdk 1.2.22 uses the protected POST path when TOKEN_PATH is undefined.
	req, err := c.newRequest(ctx, http.MethodHead, protectedPath, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("x-secsdk-csrf-request", "1")
	req.Header.Set("x-secsdk-csrf-version", SecSDKVersion)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("fetch Ocean Engine CSRF token: %w", context.DeadlineExceeded)
		}
		return "", fmt.Errorf("fetch Ocean Engine CSRF token: transport failed")
	}
	defer resp.Body.Close()
	if authResponse(resp) {
		return "", ErrAuthRequired
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%w: token HTTP %d", ErrCSRFTokenInvalid, resp.StatusCode)
	}
	parts := strings.Split(resp.Header.Get("x-ware-csrf-token"), ",")
	if len(parts) < 3 || parts[0] != "0" || strings.TrimSpace(parts[1]) == "" || parts[1] == "DOWNGRADE" {
		return "", ErrCSRFTokenInvalid
	}
	maxAgeMS, parseErr := strconv.ParseInt(strings.TrimSpace(parts[2]), 10, 64)
	if parseErr != nil || maxAgeMS <= 0 || maxAgeMS > int64((24*time.Hour)/time.Millisecond) {
		return "", ErrCSRFTokenInvalid
	}
	value := csrfCacheValue{token: parts[1], expiresAt: now.Add(time.Duration(maxAgeMS) * time.Millisecond)}
	c.TokenCache.mu.Lock()
	c.TokenCache.values[key] = value
	c.TokenCache.mu.Unlock()
	return value.token, nil
}

func (c *WriteClient) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "?") {
		return nil, fmt.Errorf("invalid Ocean Engine endpoint path")
	}
	target := *c.BaseURL
	target.Path = path
	target.RawQuery = "aadvid=" + url.QueryEscape(c.AdvertiserID)
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Cookie", c.Session.Cookies)
	if c.Session.CSRFToken != "" {
		req.Header.Set("x-csrftoken", c.Session.CSRFToken)
	}
	req.Header.Set("Origin", c.BaseURL.Scheme+"://"+c.BaseURL.Host)
	req.Header.Set("Referer", c.BaseURL.Scheme+"://"+c.BaseURL.Host+"/superior/")
	req.Header.Set("User-Agent", c.UserAgent)
	return req, nil
}

func authResponse(resp *http.Response) bool {
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return true
	}
	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return false
	}
	location := strings.ToLower(resp.Header.Get("Location"))
	return strings.Contains(location, "login") || strings.Contains(location, "sso") || strings.Contains(location, "auth")
}
