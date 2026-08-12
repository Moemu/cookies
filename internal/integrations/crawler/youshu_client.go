package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const leafletMaterialListSelection = `query %s(
  $materialIds: [String] $category: NumStr $channel: NumStr $startDate: LocalDate
  $endDate: LocalDate $platform: NumStr $keyword: String $city: NumStr $format: NumStr
  $mtype: NumStr $special: Special $page: Int $order: MaterialListSort $isLiveMaterial: Int
  $videoTime: NumStr $resolution: NumStr $is_aigc: Int $isExact: Boolean $productId: [String]
  $sellerCompanyId: String $shopId: String $uid: NumID $liveId: NumID $site: NumStr
  $brandId: NumID $linesDigest: MixID $isProductAd: Int $ratios: NumStr $materialTag: NumStr
  $words: String $tpl: [String] $material_score_gte: Int $material_score_lte: Int
  $searchField: LeafletSearchField $min_price: NumStr $max_price: NumStr $targetingAudience: NumStr
  $searchDsl: [SearchDsl] $shopType: NumStr $imgKey: String $isSearchAiScene: Int $accountType: [AccountTypeField]
) {
  leafletMaterialList(
    materialIds: $materialIds category: $category channel: $channel platform: $platform
    keyword: $keyword city: $city format: $format mtype: $mtype special: $special
    isLiveMaterial: $isLiveMaterial page: $page startDate: $startDate endDate: $endDate
    order: $order videoTime: $videoTime resolution: $resolution is_aigc: $is_aigc
    isExact: $isExact productId: $productId sellerCompanyId: $sellerCompanyId shopId: $shopId
    uid: $uid liveId: $liveId site: $site brandId: $brandId linesDigest: $linesDigest
    isProductAd: $isProductAd ratios: $ratios materialTagIds: $materialTag words: $words
    tpl: $tpl material_score_gte: $material_score_gte material_score_lte: $material_score_lte
    searchField: $searchField min_price: $min_price max_price: $max_price targetingAudience: $targetingAudience
    searchDsl: $searchDsl shopType: $shopType imgKey: $imgKey isSearchAiScene: $isSearchAiScene
    accountType: $accountType
  ) {
    data { material {
      id channel { id name } isCidMaterial material_type duration material_score firstTime lastTime
      platform { name } cnt_ad_id impression_inc_2y resource { id url poster width height duration type size }
      slogan socialInfo { view { value } like { value } comment { value } share { value } save { value } }
      bgm { title author_name } lines { content lines_style }
    } }
    total limit maxTotal page
  }
}`

const (
	// YouShuConsoleOrigin is public request context required by the upstream
	// console's GraphQL endpoint. It is not derived from, or tied to, a user's
	// browser session.
	YouShuConsoleOrigin  = "https://console.youshu.youcloud.com"
	youShuConsoleReferer = YouShuConsoleOrigin + "/leaflet"
	youShuUserAgent      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/151.0.0.0 Safari/537.36 Edg/151.0.0.0"
)

type youShuRequest struct {
	OperationName string `json:"operationName"`
	Query         string `json:"query"`
	Variables     any    `json:"variables"`
}
type youShuResponse struct {
	Data struct {
		Leaflet json.RawMessage `json:"leafletMaterialList"`
	} `json:"data"`
	Errors []struct {
		Extensions struct {
			Code string `json:"c"`
		} `json:"extensions"`
	} `json:"errors"`
}
type YouShuClient struct {
	Endpoint   string
	HTTPClient *http.Client
	Gate       *YouShuGate
	gate       YouShuGate
	// sessionCookie is deliberately unexported so normal serialization and
	// formatting cannot accidentally disclose an upstream session.
	sessionCookie string
}

// MarshalJSON intentionally omits the HTTP transport, gate, and session.
// A crawler client is runtime configuration rather than a persisted DTO.
func (c *YouShuClient) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Endpoint string `json:"endpoint"`
	}{Endpoint: c.Endpoint})
}

// NewYouShuClient creates the production client.  The endpoint is constrained
// to HTTPS and must not carry credentials or query parameters.  sessionCookie
// is sent only as the request Cookie header and is never retained in errors.
//
// Tests that replay a local HTTP server may still construct YouShuClient
// directly; production code should use this constructor.
func NewYouShuClient(endpoint, sessionCookie string, httpClient *http.Client) (*YouShuClient, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, &YouShuError{Kind: YouShuInvalidRequest, Strategy: YouShuCorrectRequest, Source: "endpoint"}
	}
	if strings.TrimSpace(sessionCookie) == "" {
		return nil, &YouShuError{Kind: YouShuInvalidRequest, Strategy: YouShuCorrectRequest, Source: "session"}
	}
	return &YouShuClient{Endpoint: u.String(), HTTPClient: httpClient, sessionCookie: sessionCookie}, nil
}

func (c *YouShuClient) Product(x context.Context, q YouShuQuery) (YouShuPage, error) {
	return c.list(x, YouShuProductOperation, q)
}
func (c *YouShuClient) CID(x context.Context, q YouShuQuery) (YouShuPage, error) {
	return c.list(x, YouShuCIDOperation, q)
}
func (c *YouShuClient) list(x context.Context, op string, q YouShuQuery) (YouShuPage, error) {
	if strings.TrimSpace(c.Endpoint) == "" {
		return YouShuPage{}, &YouShuError{Kind: YouShuInvalidRequest, Strategy: YouShuCorrectRequest, Source: "client"}
	}
	if e := q.validate(); e != nil {
		return YouShuPage{}, e
	}
	g := c.Gate
	if g == nil {
		g = &c.gate
	}
	if e := g.Acquire(x); e != nil {
		return YouShuPage{}, e
	}
	success, graphqlRate := false, false
	defer func() { g.Done(success, graphqlRate) }()
	variables, e := youShuVariables(op, q)
	if e != nil {
		return YouShuPage{}, &YouShuError{Kind: YouShuInvalidRequest, Strategy: YouShuCorrectRequest, Source: "encode"}
	}
	b, e := json.Marshal(youShuRequest{op, strings.Replace(leafletMaterialListSelection, "%s", op, 1), variables})
	if e != nil {
		return YouShuPage{}, &YouShuError{Kind: YouShuInvalidRequest, Strategy: YouShuCorrectRequest, Source: "encode"}
	}
	r, e := http.NewRequestWithContext(x, http.MethodPost, c.Endpoint, bytes.NewReader(b))
	if e != nil {
		return YouShuPage{}, &YouShuError{Kind: YouShuInvalidRequest, Strategy: YouShuCorrectRequest, Source: "request"}
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/plain, */*")
	r.Header.Set("X-Operation-Name", op)
	// The console validates the request context in addition to the authorized
	// session. These fixed public values keep the server-side integration in the
	// same application context without copying browser headers or state.
	r.Header.Set("Origin", YouShuConsoleOrigin)
	r.Header.Set("Referer", youShuConsoleReferer)
	r.Header.Set("User-Agent", youShuUserAgent)
	if c.sessionCookie != "" {
		r.Header.Set("Cookie", c.sessionCookie)
	}
	h := c.HTTPClient
	if h == nil {
		h = http.DefaultClient
	}
	resp, e := h.Do(r)
	if e != nil {
		return YouShuPage{}, transportError(x, e)
	}
	defer resp.Body.Close()
	b, e = io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if e != nil {
		return YouShuPage{}, &YouShuError{Kind: YouShuMalformed, Strategy: YouShuRetry, Source: "body", Status: resp.StatusCode}
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return YouShuPage{}, &YouShuError{Kind: YouShuAuthRequired, Strategy: YouShuReauthenticate, Source: "http", Status: resp.StatusCode}
	}
	if resp.StatusCode == 429 {
		return YouShuPage{}, &YouShuError{Kind: YouShuRateLimited, Strategy: YouShuRetryLater, Source: "http", Status: 429}
	}
	if resp.StatusCode >= 500 {
		return YouShuPage{}, &YouShuError{Kind: YouShuServerError, Strategy: YouShuRetry, Source: "http", Status: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return YouShuPage{}, &YouShuError{Kind: YouShuHTTPError, Strategy: YouShuDoNotRetry, Source: "http", Status: resp.StatusCode}
	}
	var d youShuResponse
	if json.Unmarshal(b, &d) != nil {
		return YouShuPage{}, &YouShuError{Kind: YouShuMalformed, Strategy: YouShuRetry, Source: "json", Status: resp.StatusCode}
	}
	if len(d.Errors) > 0 {
		z := graphqlError(d.Errors[0].Extensions.Code)
		graphqlRate = z.Kind == YouShuRateLimited
		return YouShuPage{}, z
	}
	p, e := normalizePage(d.Data.Leaflet)
	if e != nil {
		return YouShuPage{}, e
	}
	success = true
	return p, nil
}

func youShuVariables(operation string, query YouShuQuery) (any, error) {
	if operation == YouShuCIDOperation {
		// CID requires the complete variable shape, including explicit nulls.
		return query, nil
	}
	encoded, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}
	variables := map[string]any{}
	if err := json.Unmarshal(encoded, &variables); err != nil {
		return nil, err
	}
	for key, value := range variables {
		if value == nil {
			delete(variables, key)
			continue
		}
		if values, ok := value.([]any); ok && len(values) == 0 {
			delete(variables, key)
		}
	}
	return variables, nil
}
func graphqlError(c string) *YouShuError {
	switch c {
	case "00:400998":
		return &YouShuError{Kind: YouShuRateLimited, Strategy: YouShuRetryLater, Source: "graphql", Code: c}
	case "00:403005":
		return &YouShuError{Kind: YouShuAuthRequired, Strategy: YouShuReauthenticate, Source: "graphql", Code: c}
	case "00:400999":
		return &YouShuError{Kind: YouShuInvalidRequest, Strategy: YouShuCorrectRequest, Source: "graphql", Code: c}
	default:
		return &YouShuError{Kind: YouShuGraphQLError, Strategy: YouShuDoNotRetry, Source: "graphql", Code: c}
	}
}
func transportError(x context.Context, e error) error {
	if errors.Is(x.Err(), context.DeadlineExceeded) || errors.Is(e, context.DeadlineExceeded) {
		return &YouShuError{Kind: YouShuTimeout, Strategy: YouShuRetry, Source: "transport"}
	}
	return &YouShuError{Kind: YouShuTransport, Strategy: YouShuRetry, Source: "transport"}
}

type rawPage struct {
	Data                         json.RawMessage `json:"data"`
	Total, Limit, MaxTotal, Page int64
}
type rawMaterial struct {
	ID           string          `json:"id"`
	Channel      json.RawMessage `json:"channel"`
	IsCID        bool            `json:"isCidMaterial"`
	MaterialType json.RawMessage `json:"material_type"`
	Duration     json.RawMessage `json:"duration"`
	Score        json.RawMessage `json:"material_score"`
	First        any             `json:"firstTime"`
	Last         any             `json:"lastTime"`
	Platform     json.RawMessage `json:"platform"`
	Cnt          json.RawMessage `json:"cnt_ad_id"`
	Impression   json.RawMessage `json:"impression_inc_2y"`
	Resource     json.RawMessage `json:"resource"`
	Slogan       string          `json:"slogan"`
	Social       json.RawMessage `json:"socialInfo"`
	BGM          struct {
		Title  string `json:"title"`
		Author string `json:"author_name"`
	} `json:"bgm"`
	Lines json.RawMessage `json:"lines"`
}

func normalizePage(b json.RawMessage) (YouShuPage, error) {
	var r rawPage
	if len(b) == 0 || json.Unmarshal(b, &r) != nil {
		return YouShuPage{}, &YouShuError{Kind: YouShuMalformed, Strategy: YouShuRetry, Source: "payload"}
	}
	var materials []rawMaterial
	var wrapped struct {
		Material []rawMaterial `json:"material"`
	}
	if json.Unmarshal(r.Data, &wrapped) == nil {
		materials = wrapped.Material
	} else {
		var rows []struct {
			Material rawMaterial `json:"material"`
		}
		if json.Unmarshal(r.Data, &rows) != nil {
			return YouShuPage{}, &YouShuError{Kind: YouShuMalformed, Strategy: YouShuRetry, Source: "payload_data"}
		}
		materials = make([]rawMaterial, 0, len(rows))
		for _, row := range rows {
			materials = append(materials, row.Material)
		}
	}
	o := YouShuPage{Total: r.Total, Limit: r.Limit, MaxTotal: r.MaxTotal, Page: r.Page}
	for _, m := range materials {
		if m.ID == "" {
			return YouShuPage{}, &YouShuError{Kind: YouShuMalformed, Strategy: YouShuRetry, Source: "material_id"}
		}
		f, ok := parseYouShuTime(m.First)
		if !ok {
			return YouShuPage{}, &YouShuError{Kind: YouShuMalformed, Strategy: YouShuRetry, Source: "first_time"}
		}
		l, ok := parseYouShuTime(m.Last)
		if !ok {
			return YouShuPage{}, &YouShuError{Kind: YouShuMalformed, Strategy: YouShuRetry, Source: "last_time"}
		}
		id, n := named(m.Channel)
		o.Materials = append(o.Materials, YouShuMaterial{MaterialID: m.ID, IsCID: m.IsCID, ChannelID: id, ChannelName: n, MaterialType: scalarString(m.MaterialType), Duration: scalarInt(m.Duration), Score: scalarFloat(m.Score), FirstSeenAt: f, LastSeenAt: l, PlatformName: namedName(m.Platform), CntAdID: scalarCount(m.Cnt), ImpressionInc2Y: scalarCount(m.Impression), ImpressionRaw: scalarText(m.Impression), Resource: resourceValue(m.Resource), Slogan: m.Slogan, Social: socialValue(m.Social), BGMTitle: m.BGM.Title, BGMAuthor: m.BGM.Author, FirstLineContent: linesContent(m.Lines)})
	}
	sort.Slice(o.Materials, func(i, j int) bool { return o.Materials[i].MaterialID < o.Materials[j].MaterialID })
	return o, nil
}
func parseYouShuTime(v any) (time.Time, bool) {
	switch value := v.(type) {
	case string:
		for _, format := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
			if parsed, err := time.Parse(format, value); err == nil {
				return parsed.UTC(), true
			}
		}
	case float64:
		seconds := int64(value)
		if seconds > 1_000_000_000_000 {
			seconds /= 1000
		}
		return time.Unix(seconds, 0).UTC(), true
	}
	return time.Time{}, false
}
func named(b json.RawMessage) (string, string) {
	var many []struct {
		ID   json.RawMessage `json:"id"`
		Name string          `json:"name"`
	}
	if json.Unmarshal(b, &many) == nil && len(many) > 0 {
		return scalarString(many[0].ID), many[0].Name
	}
	var x struct {
		ID   json.RawMessage `json:"id"`
		Name string          `json:"name"`
	}
	json.Unmarshal(b, &x)
	return scalarString(x.ID), x.Name
}
func namedName(b json.RawMessage) string    { _, n := named(b); return n }
func scalarString(b json.RawMessage) string { return strings.Trim(string(b), "\"") }
func scalarText(b json.RawMessage) string {
	var value string
	if json.Unmarshal(b, &value) == nil {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(string(b))
}
func scalarInt(b json.RawMessage) int64 {
	var n int64
	if json.Unmarshal(b, &n) != nil {
		var s string
		if json.Unmarshal(b, &s) == nil {
			n, _ = strconv.ParseInt(s, 10, 64)
		}
	}
	return n
}
func scalarCount(b json.RawMessage) int64 {
	value := scalarText(b)
	value = strings.NewReplacer(",", "", "，", "", " ", "").Replace(value)
	value = strings.TrimSuffix(value, "+")
	multiplier := float64(1)
	for _, unit := range []struct {
		suffix     string
		multiplier float64
	}{
		{"亿", 100_000_000}, {"万", 10_000}, {"w", 10_000}, {"W", 10_000},
		{"千", 1_000}, {"k", 1_000}, {"K", 1_000}, {"m", 1_000_000}, {"M", 1_000_000},
	} {
		if strings.HasSuffix(value, unit.suffix) {
			value = strings.TrimSuffix(value, unit.suffix)
			multiplier = unit.multiplier
			break
		}
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 {
		return 0
	}
	normalized := math.Round(number * multiplier)
	if normalized > float64(^uint64(0)>>1) {
		return 0
	}
	return int64(normalized)
}
func scalarFloat(b json.RawMessage) float64 {
	var n float64
	if json.Unmarshal(b, &n) != nil {
		var s string
		if json.Unmarshal(b, &s) == nil {
			n, _ = strconv.ParseFloat(s, 64)
		}
	}
	return n
}
func resourceValue(b json.RawMessage) YouShuResource {
	type rawResource struct {
		ID, URL, Poster, Type         string
		Width, Height, Duration, Size json.RawMessage
	}
	convert := func(value rawResource) YouShuResource {
		return YouShuResource{ID: value.ID, URL: value.URL, Poster: value.Poster, Width: scalarInt(value.Width), Height: scalarInt(value.Height), Duration: scalarInt(value.Duration), Type: value.Type, Size: scalarInt(value.Size)}
	}
	var many []rawResource
	if json.Unmarshal(b, &many) == nil && len(many) > 0 {
		return convert(many[0])
	}
	var x rawResource
	json.Unmarshal(b, &x)
	return convert(x)
}
func socialValue(b json.RawMessage) YouShuSocial {
	var x struct {
		View, Like, Comment, Share, Save struct {
			Value json.RawMessage `json:"value"`
		}
	}
	json.Unmarshal(b, &x)
	return YouShuSocial{
		View: scalarCount(x.View.Value), Like: scalarCount(x.Like.Value), Comment: scalarCount(x.Comment.Value),
		Share: scalarCount(x.Share.Value), Save: scalarCount(x.Save.Value),
	}
}
func linesContent(b json.RawMessage) string {
	var x []struct{ Content string }
	if json.Unmarshal(b, &x) == nil && len(x) > 0 {
		return strings.Split(x[0].Content, "\n")[0]
	}
	var y struct{ Content string }
	json.Unmarshal(b, &y)
	return strings.Split(y.Content, "\n")[0]
}
