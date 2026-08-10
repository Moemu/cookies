package crawler

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	YouShuProductOperation = "getLeafletMaterialList"
	YouShuCIDOperation     = "getDrainageMaterialList"
)

// YouShuQuery is deliberately a fixed, serializable representation of the
// upstream filter.  The service has historically treated omitted fields and
// null fields differently, so CID requests always emit every field below.
type YouShuQuery struct {
	MaterialIDs       []string          `json:"materialIds"`
	Category          *string           `json:"category"`
	Channel           *string           `json:"channel"`
	StartDate         string            `json:"startDate"`
	EndDate           string            `json:"endDate"`
	Platform          *string           `json:"platform"`
	Keyword           string            `json:"keyword"`
	City              *string           `json:"city"`
	Format            *string           `json:"format"`
	MType             *string           `json:"mtype"`
	Special           *string           `json:"special"`
	Page              int               `json:"page"`
	Order             string            `json:"order"`
	IsLiveMaterial    *int              `json:"isLiveMaterial"`
	VideoTime         *string           `json:"videoTime"`
	Resolution        *string           `json:"resolution"`
	IsAIGC            *int              `json:"is_aigc"`
	IsExact           *bool             `json:"isExact"`
	ProductID         []string          `json:"productId"`
	SellerCompanyID   *string           `json:"sellerCompanyId"`
	ShopID            *string           `json:"shopId"`
	UID               *string           `json:"uid"`
	LiveID            *string           `json:"liveId"`
	Site              *string           `json:"site"`
	BrandID           *string           `json:"brandId"`
	LinesDigest       *string           `json:"linesDigest"`
	IsProductAd       *int              `json:"isProductAd"`
	Ratios            *string           `json:"ratios"`
	MaterialTag       *string           `json:"materialTag"`
	Words             *string           `json:"words"`
	Tpl               []string          `json:"tpl"`
	MaterialScoreGTE  *int              `json:"material_score_gte"`
	MaterialScoreLTE  *int              `json:"material_score_lte"`
	SearchField       string            `json:"searchField"`
	MinPrice          *string           `json:"min_price"`
	MaxPrice          *string           `json:"max_price"`
	TargetingAudience *string           `json:"targetingAudience"`
	SearchDSL         []json.RawMessage `json:"searchDsl"`
	ShopType          *string           `json:"shopType"`
	ImgKey            *string           `json:"imgKey"`
	IsSearchAiScene   *int              `json:"isSearchAiScene"`
	AccountType       []string          `json:"accountType"`
}

func (q YouShuQuery) validate() error {
	if strings.TrimSpace(q.Keyword) == "" || strings.TrimSpace(q.StartDate) == "" ||
		strings.TrimSpace(q.EndDate) == "" || strings.TrimSpace(q.Order) == "" || q.Page < 1 ||
		strings.TrimSpace(q.SearchField) == "" || q.IsExact == nil || q.IsSearchAiScene == nil {
		return &YouShuError{Kind: YouShuInvalidRequest, Strategy: YouShuCorrectRequest, Source: "validation", Code: "MISSING_QUERY_VARIABLE"}
	}
	return nil
}

// YouShuBool marks a required protocol boolean as explicitly supplied.
func YouShuBool(value bool) *bool { return &value }
func YouShuInt(value int) *int    { return &value }

// YouShuMaterial is the normalized, deliberately small public response model.
type YouShuMaterial struct {
	MaterialID       string
	IsCID            bool
	ChannelID        string
	ChannelName      string
	MaterialType     string
	Duration         int64
	Score            float64
	FirstSeenAt      time.Time
	LastSeenAt       time.Time
	PlatformName     string
	CntAdID          int64
	ImpressionInc2Y  int64
	Resource         YouShuResource
	Slogan           string
	Social           YouShuSocial
	BGMTitle         string
	BGMAuthor        string
	FirstLineContent string
}

type YouShuResource struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	Poster   string `json:"poster"`
	Width    int64  `json:"width"`
	Height   int64  `json:"height"`
	Duration int64  `json:"duration"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
}

type YouShuSocial struct {
	View    int64 `json:"view"`
	Like    int64 `json:"like"`
	Comment int64 `json:"comment"`
	Share   int64 `json:"share"`
	Save    int64 `json:"save"`
}

type YouShuPage struct {
	Materials []YouShuMaterial
	Total     int64
	Limit     int64
	MaxTotal  int64
	Page      int64
}

type YouShuErrorKind string
type YouShuStrategy string

const (
	YouShuRateLimited    YouShuErrorKind = "rate_limited"
	YouShuAuthRequired   YouShuErrorKind = "auth_required"
	YouShuInvalidRequest YouShuErrorKind = "invalid_request"
	YouShuHTTPError      YouShuErrorKind = "http_error"
	YouShuServerError    YouShuErrorKind = "server_error"
	YouShuMalformed      YouShuErrorKind = "malformed_response"
	YouShuTransport      YouShuErrorKind = "transport_error"
	YouShuTimeout        YouShuErrorKind = "timeout"
	YouShuThrottled      YouShuErrorKind = "throttled"
	YouShuGraphQLError   YouShuErrorKind = "graphql_error"
	YouShuRetryLater     YouShuStrategy  = "retry_later"
	YouShuReauthenticate YouShuStrategy  = "reauthenticate"
	YouShuCorrectRequest YouShuStrategy  = "correct_request"
	YouShuRetry          YouShuStrategy  = "retry"
	YouShuDoNotRetry     YouShuStrategy  = "do_not_retry"
)

// YouShuError holds only protocol metadata. It intentionally never retains
// headers, request bodies, or upstream messages, which may contain sessions.
type YouShuError struct {
	Kind         YouShuErrorKind
	Strategy     YouShuStrategy
	Source, Code string
	Status       int
}

func (e *YouShuError) Error() string {
	return fmt.Sprintf("youshu %s (%s, status=%d, code=%s)", e.Kind, e.Source, e.Status, e.Code)
}
