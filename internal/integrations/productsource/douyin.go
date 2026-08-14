// Package productsource resolves externally shared commerce links into a
// provider-neutral product snapshot. It does not own Creative workflow state.
package productsource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	douyinSource       = "douyin_mall"
	maxRedirects       = 5
	maxResponseBody    = 1 << 20
	maxProductLinkSize = 64 << 10
	resolverUserAgent  = "cookies-product-resolver/1.0"
	defaultHTTPTimeout = 12 * time.Second
)

var (
	httpsURLPattern    = regexp.MustCompile(`https://[^\s]+`)
	ErrUnsupportedLink = errors.New("unsupported product link")
	ErrIncompleteLink  = errors.New("incomplete product link")
	ErrProductMissing  = errors.New("product information is missing")
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type ProductImage struct {
	URL  string `json:"url"`
	Role string `json:"role"`
}

type ProductPrice struct {
	MinRaw             int64  `json:"min_raw"`
	MaxRaw             int64  `json:"max_raw"`
	Currency           string `json:"currency"`
	DisplayUnconfirmed bool   `json:"display_unconfirmed"`
}

type ProductSnapshot struct {
	Source           string         `json:"source"`
	ProductID        string         `json:"product_id"`
	Name             string         `json:"name"`
	Description      string         `json:"description"`
	Images           []ProductImage `json:"images"`
	Price            ProductPrice   `json:"price"`
	Sales            int64          `json:"sales"`
	SourceURL        string         `json:"source_url"`
	ResolutionStatus string         `json:"resolution_status"`
	ResourceType     string         `json:"resource_type"`
	MissingFields    []string       `json:"missing_fields"`
}

// DouyinResolver follows a Douyin share link and extracts the product snapshot
// embedded in the resulting Douyin Mall URL. Client is injectable so callers
// and tests cross the same interface.
type DouyinResolver struct {
	Client HTTPDoer
}

func NewDouyinResolver() DouyinResolver {
	return DouyinResolver{Client: &http.Client{
		Timeout: defaultHTTPTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("douyin product redirect limit exceeded")
			}
			return validateDouyinURL(request.URL)
		},
	}}
}

func (r DouyinResolver) Resolve(ctx context.Context, input string) (ProductSnapshot, error) {
	productURL, err := extractProductURL(input)
	if err != nil {
		return ProductSnapshot{}, err
	}
	if err := validateDouyinURL(productURL); err != nil {
		return ProductSnapshot{}, err
	}
	if r.Client == nil {
		return ProductSnapshot{}, fmt.Errorf("douyin product resolver client is required")
	}
	if snapshot, snapshotErr := snapshotFromResolvedURL(productURL, productURL); snapshotErr == nil {
		return snapshot, nil
	} else if errors.Is(snapshotErr, ErrIncompleteLink) {
		return ProductSnapshot{}, snapshotErr
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, productURL.String(), nil)
	if err != nil {
		return ProductSnapshot{}, fmt.Errorf("create product request: %w", err)
	}
	request.Header.Set("User-Agent", resolverUserAgent)
	response, err := r.Client.Do(request)
	if err != nil {
		return ProductSnapshot{}, fmt.Errorf("resolve douyin product link: %w", err)
	}
	if response == nil {
		return ProductSnapshot{}, fmt.Errorf("resolve douyin product link: empty response")
	}
	if response.Body != nil {
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBody))
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ProductSnapshot{}, fmt.Errorf("resolve douyin product link: unexpected status %d", response.StatusCode)
	}

	resolvedURL := productURL
	if response.Request != nil && response.Request.URL != nil {
		resolvedURL = response.Request.URL
	}
	if err := validateDouyinURL(resolvedURL); err != nil {
		return ProductSnapshot{}, err
	}
	return snapshotFromResolvedURL(productURL, resolvedURL)
}

func extractProductURL(input string) (*url.URL, error) {
	trimmed := strings.TrimSpace(input)
	if len(trimmed) == 0 || len(trimmed) > maxProductLinkSize {
		return nil, fmt.Errorf("%w: product link is empty or too long", ErrUnsupportedLink)
	}
	match := httpsURLPattern.FindString(trimmed)
	if match == "" {
		return nil, fmt.Errorf("%w: an https link is required", ErrUnsupportedLink)
	}
	match = strings.TrimRight(match, "，。！？；：,.!?;:)]}>'\"")
	parsed, err := url.Parse(match)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsupportedLink, err)
	}
	return parsed, nil
}

func validateDouyinURL(value *url.URL) error {
	if value == nil || value.Scheme != "https" || value.User != nil {
		return fmt.Errorf("%w: only https Douyin links are allowed", ErrUnsupportedLink)
	}
	host := strings.ToLower(strings.TrimSuffix(value.Hostname(), "."))
	if host != "v.douyin.com" && host != "haohuo.jinritemai.com" {
		return fmt.Errorf("%w: host %q is not allowed", ErrUnsupportedLink, host)
	}
	return nil
}

type douyinGoodsDetail struct {
	Title    string `json:"title"`
	Sales    int64  `json:"sales"`
	MinPrice int64  `json:"min_price"`
	MaxPrice int64  `json:"max_price"`
	Image    struct {
		URLs []string `json:"url_list"`
	} `json:"img"`
}

func snapshotFromResolvedURL(sourceURL, resolvedURL *url.URL) (ProductSnapshot, error) {
	query := resolvedURL.Query()
	payload := strings.TrimSpace(query.Get("goods_detail"))
	if payload == "" {
		return ProductSnapshot{}, fmt.Errorf("%w: goods_detail is absent", ErrProductMissing)
	}
	var detail douyinGoodsDetail
	if err := json.Unmarshal([]byte(payload), &detail); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || strings.Contains(strings.ToLower(err.Error()), "unexpected end") {
			return ProductSnapshot{}, fmt.Errorf("%w: goods_detail ended before the product data was complete", ErrIncompleteLink)
		}
		return ProductSnapshot{}, fmt.Errorf("%w: invalid goods_detail: %v", ErrProductMissing, err)
	}
	productID := firstNonEmpty(query.Get("id"), query.Get("product_id"), query.Get("promotion_id"), productIDFromDetailSchema(query.Get("detail_schema")))
	if strings.TrimSpace(detail.Title) == "" || strings.TrimSpace(productID) == "" {
		return ProductSnapshot{}, fmt.Errorf("%w: product id and title are required", ErrProductMissing)
	}

	images := make([]ProductImage, 0, 1)
	for _, rawImageURL := range detail.Image.URLs {
		if !validDouyinImageURL(rawImageURL) {
			continue
		}
		// Douyin uses url_list for CDN mirrors of the same main image, not for
		// distinct gallery assets. Keep one canonical media item here.
		images = append(images, ProductImage{URL: rawImageURL, Role: "main"})
		break
	}
	return ProductSnapshot{
		Source:      douyinSource,
		ProductID:   productID,
		Name:        strings.TrimSpace(detail.Title),
		Description: "",
		Images:      images,
		Price: ProductPrice{
			MinRaw:             detail.MinPrice,
			MaxRaw:             detail.MaxPrice,
			Currency:           "CNY",
			DisplayUnconfirmed: true,
		},
		Sales:     detail.Sales,
		SourceURL: canonicalSourceURL(sourceURL, productID),
		ResolutionStatus: func() string {
			if len(images) == 0 {
				return ResolutionManualRequired
			}
			return ResolutionPartial
		}(),
		ResourceType: ResourceProduct,
		MissingFields: func() []string {
			if len(images) == 0 {
				return []string{"images", "description", "core_selling_points"}
			}
			return []string{"description", "core_selling_points"}
		}(),
	}, nil
}

func productIDFromDetailSchema(raw string) string {
	value := strings.TrimSpace(raw)
	for range 3 {
		if value == "" {
			return ""
		}
		parsed, err := url.Parse(value)
		if err == nil {
			query := parsed.Query()
			if productID := firstNonEmpty(query.Get("product_id"), query.Get("promotion_id"), query.Get("id")); productID != "" {
				return productID
			}
		}
		decoded, err := url.QueryUnescape(value)
		if err != nil || decoded == value {
			break
		}
		value = decoded
	}
	return ""
}

func validDouyinImageURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.User != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	return host == "ecombdimg.com" || strings.HasSuffix(host, ".ecombdimg.com")
}

func canonicalSourceURL(sourceURL *url.URL, productID string) string {
	if sourceURL != nil && strings.EqualFold(sourceURL.Hostname(), "v.douyin.com") {
		copy := *sourceURL
		copy.RawQuery = ""
		copy.Fragment = ""
		return copy.String()
	}
	return "https://haohuo.jinritemai.com/ecommerce/trade/detail/index.html?id=" + url.QueryEscape(productID)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
