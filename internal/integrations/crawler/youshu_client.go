package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
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

type youShuRequest struct {
	OperationName string      `json:"operationName"`
	Query         string      `json:"query"`
	Variables     YouShuQuery `json:"variables"`
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
	b, e := json.Marshal(youShuRequest{op, strings.Replace(leafletMaterialListSelection, "%s", op, 1), q})
	if e != nil {
		return YouShuPage{}, &YouShuError{Kind: YouShuInvalidRequest, Strategy: YouShuCorrectRequest, Source: "encode"}
	}
	r, e := http.NewRequestWithContext(x, http.MethodPost, c.Endpoint, bytes.NewReader(b))
	if e != nil {
		return YouShuPage{}, &YouShuError{Kind: YouShuInvalidRequest, Strategy: YouShuCorrectRequest, Source: "request"}
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json")
	r.Header.Set("X-Operation-Name", op)
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
	Data struct {
		Material []rawMaterial `json:"material"`
	} `json:"data"`
	Total, Limit, MaxTotal, Page int64
}
type rawMaterial struct {
	ID           string          `json:"id"`
	Channel      json.RawMessage `json:"channel"`
	IsCID        bool            `json:"isCidMaterial"`
	MaterialType string          `json:"material_type"`
	Duration     int64           `json:"duration"`
	Score        float64         `json:"material_score"`
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
	o := YouShuPage{Total: r.Total, Limit: r.Limit, MaxTotal: r.MaxTotal, Page: r.Page}
	for _, m := range r.Data.Material {
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
		o.Materials = append(o.Materials, YouShuMaterial{MaterialID: m.ID, IsCID: m.IsCID, ChannelID: id, ChannelName: n, MaterialType: m.MaterialType, Duration: m.Duration, Score: m.Score, FirstSeenAt: f, LastSeenAt: l, PlatformName: namedName(m.Platform), CntAdID: scalarInt(m.Cnt), ImpressionInc2Y: scalarInt(m.Impression), Resource: resourceValue(m.Resource), Slogan: m.Slogan, Social: socialValue(m.Social), BGMTitle: m.BGM.Title, BGMAuthor: m.BGM.Author, FirstLineContent: linesContent(m.Lines)})
	}
	sort.Slice(o.Materials, func(i, j int) bool { return o.Materials[i].MaterialID < o.Materials[j].MaterialID })
	return o, nil
}
func parseYouShuTime(v any) (time.Time, bool) {
	switch value := v.(type) {
	case string:
		for _, format := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
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
func resourceValue(b json.RawMessage) YouShuResource {
	var many []YouShuResource
	if json.Unmarshal(b, &many) == nil && len(many) > 0 {
		return many[0]
	}
	var x YouShuResource
	json.Unmarshal(b, &x)
	return x
}
func socialValue(b json.RawMessage) YouShuSocial {
	var x struct {
		View, Like, Comment, Share, Save struct {
			Value json.RawMessage `json:"value"`
		}
	}
	json.Unmarshal(b, &x)
	return YouShuSocial{
		View: scalarInt(x.View.Value), Like: scalarInt(x.Like.Value), Comment: scalarInt(x.Comment.Value),
		Share: scalarInt(x.Share.Value), Save: scalarInt(x.Save.Value),
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
