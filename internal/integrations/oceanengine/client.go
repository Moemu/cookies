package oceanengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrForbiddenEndpoint = errors.New("oceanengine endpoint is not read-only")
var ErrSessionInvalid = errors.New("oceanengine session is invalid")

type Endpoint struct {
	Method string
	Path   string
}

var readOnlyEndpoints = map[Endpoint]struct{}{
	{http.MethodPost, "/ad/api/promotion/ads/list"}:                        {},
	{http.MethodGet, "/superior/api/ad/promotion/detail"}:                  {},
	{http.MethodGet, "/ad/api/promotion/ads/get_promotion_detail"}:         {},
	{http.MethodGet, "/ad/api/promotion/ads/detail"}:                       {},
	{http.MethodPost, "/ad/api/promotion/ads/attribute/list"}:              {},
	{http.MethodPost, "/report/api/tool/agw/statistics_sophonx/statQuery"}: {},
	{http.MethodPost, "/ad/api/agw/statistics_sophonx/statQuery"}:          {},
	{http.MethodGet, "/ad/api/account/info"}:                               {},
	{http.MethodGet, "/superior/api/v2/account/info"}:                      {},
	{http.MethodGet, "/ad/api/account/conf"}:                               {},
	{http.MethodGet, "/api/ebp/ebp_info/get_global_info"}:                  {},
}

type Session struct {
	Cookies   string
	CSRFToken string
}

type Client struct {
	BaseURL      *url.URL
	HTTPClient   *http.Client
	Session      Session
	UserAgent    string
	AdvertiserID string
	Delay        time.Duration
}

func NewClient(rawBaseURL, advertiserID string, session Session, httpClient *http.Client) (*Client, error) {
	client, err := newClient(rawBaseURL, advertiserID, session, httpClient)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(advertiserID) == "" {
		return nil, fmt.Errorf("advertiser ID and in-memory cookie session are required")
	}
	return client, nil
}

// NewSessionClient creates a read-only client for session verification.
func NewSessionClient(rawBaseURL string, session Session, httpClient *http.Client) (*Client, error) {
	return newClient(rawBaseURL, "", session, httpClient)
}

func newClient(rawBaseURL, advertiserID string, session Session, httpClient *http.Client) (*Client, error) {
	base, err := url.Parse(strings.TrimRight(rawBaseURL, "/"))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("invalid Ocean Engine base URL")
	}
	if strings.TrimSpace(session.Cookies) == "" {
		return nil, fmt.Errorf("in-memory cookie session is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if strings.TrimSpace(session.CSRFToken) == "" {
		session.CSRFToken = cookieValue(session.Cookies, "csrftoken")
	}
	return &Client{BaseURL: base, HTTPClient: httpClient, Session: session, AdvertiserID: advertiserID, UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36 Edg/126.0.0.0", Delay: 500 * time.Millisecond}, nil
}

func cookieValue(header, name string) string {
	for _, part := range strings.Split(header, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), name) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, contentType string) (map[string]any, error) {
	endpointPath := path
	endpointQuery := ""
	if index := strings.IndexByte(path, '?'); index >= 0 {
		endpointPath, endpointQuery = path[:index], path[index+1:]
	}
	if _, ok := readOnlyEndpoints[Endpoint{method, endpointPath}]; !ok {
		return nil, fmt.Errorf("%w: %s %s", ErrForbiddenEndpoint, method, path)
	}
	if c.Delay > 0 {
		timer := time.NewTimer(c.Delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	u := *c.BaseURL
	u.Path = strings.TrimRight(u.Path, "/") + endpointPath
	query := u.Query()
	if endpointQuery != "" {
		parsed, err := url.ParseQuery(endpointQuery)
		if err != nil {
			return nil, fmt.Errorf("invalid endpoint query: %w", err)
		}
		for key, values := range parsed {
			for _, value := range values {
				query.Add(key, value)
			}
		}
	}
	if c.AdvertiserID != "" {
		query.Set("aadvid", c.AdvertiserID)
	}
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	origin := c.BaseURL.Scheme + "://" + c.BaseURL.Host
	req.Header.Set("Origin", origin)
	if c.AdvertiserID != "" {
		req.Header.Set("Referer", origin+"/promotion/promote-manage/ad?aadvid="+url.QueryEscape(c.AdvertiserID))
	} else if strings.EqualFold(c.BaseURL.Host, "business.oceanengine.com") {
		req.Header.Set("Referer", origin+"/")
	}
	req.Header.Set("Cookie", c.Session.Cookies)
	if c.Session.CSRFToken != "" {
		req.Header.Set("x-csrftoken", c.Session.CSRFToken)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("oceanengine request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("oceanengine HTTP status %d", resp.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode Ocean Engine response: %w", err)
	}
	if code, ok := payload["code"].(float64); ok && code != 0 {
		return nil, fmt.Errorf("%w: business code %.0f", ErrSessionInvalid, code)
	}
	return payload, nil
}
