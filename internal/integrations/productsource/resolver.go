package productsource

import (
	"context"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	ResolutionRecognized     = "recognized"
	ResolutionPartial        = "partial"
	ResolutionManualRequired = "manual_required"
	ResourceProduct          = "product"
)

var (
	digitsOnly               = regexp.MustCompile(`^[0-9]+$`)
	commerceTargetURLPattern = regexp.MustCompile(`https?://[^\s<>"']+`)
	metaTagPattern           = regexp.MustCompile(`(?is)<meta\s+[^>]*>`)
	metaAttributePattern     = regexp.MustCompile(`(?is)([a-zA-Z_:][a-zA-Z0-9_:.\-]*)\s*=\s*["']([^"']*)["']`)
	htmlTitlePattern         = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	quotedProductNamePattern = regexp.MustCompile(`「([^」]{2,120})」`)
)

// Resolver is the single commerce-source seam used by Creative. Alibaba
// commerce pages are read only as a best-effort fallback; no platform API or
// merchant credential is used here.
type Resolver struct {
	Client HTTPDoer
}

func NewResolver() Resolver {
	return Resolver{Client: &http.Client{
		Timeout: defaultHTTPTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("commerce redirect limit exceeded")
			}
			return validateApprovedCommerceURL(request.URL, true)
		},
	}}
}

func (r Resolver) Resolve(ctx context.Context, input string) (ProductSnapshot, error) {
	productURL, err := extractProductURL(input)
	if err != nil {
		return ProductSnapshot{}, err
	}
	host := normalizedHost(productURL)
	if isDouyinHost(host) {
		douyin := DouyinResolver{Client: r.Client}
		if douyin.Client == nil {
			douyin = NewDouyinResolver()
		}
		return douyin.Resolve(ctx, input)
	}
	if err := validateApprovedCommerceURL(productURL, true); err != nil {
		return ProductSnapshot{}, err
	}

	if isMainCommerceHost(host) {
		snapshot, snapshotErr := snapshotFromCommerceURL(input, productURL)
		if snapshotErr != nil {
			return ProductSnapshot{}, snapshotErr
		}
		// A valid product identity must remain usable even if the public page is
		// blocked, requires login, or changes markup.
		if body, finalURL, fetchErr := r.fetchPage(ctx, productURL); fetchErr == nil && isMainCommerceHost(normalizedHost(finalURL)) {
			if resolved, resolvedErr := snapshotFromCommerceURL(input, finalURL); resolvedErr == nil {
				snapshot = resolved
			}
			return enrichCommerceSnapshot(snapshot, body), nil
		}
		return snapshot, nil
	}

	body, finalURL, err := r.fetchPage(ctx, productURL)
	if err != nil {
		return ProductSnapshot{}, fmt.Errorf("resolve commerce short link: %w", err)
	}
	targetURL := finalURL
	if !isMainCommerceHost(normalizedHost(targetURL)) {
		targetURL = commerceTargetFromHTML(body)
	}
	if targetURL == nil || !isMainCommerceHost(normalizedHost(targetURL)) {
		return ProductSnapshot{}, fmt.Errorf("%w: short link did not expose a supported product target", ErrProductMissing)
	}
	if err := validateApprovedCommerceURL(targetURL, false); err != nil {
		return ProductSnapshot{}, err
	}
	snapshot, err := snapshotFromCommerceURL(input, targetURL)
	if err != nil {
		return ProductSnapshot{}, err
	}
	return enrichCommerceSnapshot(snapshot, body), nil
}

func (r Resolver) fetchPage(ctx context.Context, target *url.URL) ([]byte, *url.URL, error) {
	if r.Client == nil {
		return nil, nil, fmt.Errorf("commerce product resolver client is required")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("create commerce product request: %w", err)
	}
	request.Header.Set("User-Agent", resolverUserAgent)
	response, err := r.Client.Do(request)
	if err != nil {
		return nil, nil, err
	}
	if response == nil {
		return nil, nil, fmt.Errorf("empty response")
	}
	if response.Body == nil {
		response.Body = http.NoBody
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, nil, fmt.Errorf("unexpected status %d", response.StatusCode)
	}
	finalURL := target
	if response.Request != nil && response.Request.URL != nil {
		finalURL = response.Request.URL
	}
	if err := validateApprovedCommerceURL(finalURL, true); err != nil {
		return nil, nil, err
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBody))
	if err != nil {
		return nil, nil, fmt.Errorf("read commerce response: %w", err)
	}
	return body, finalURL, nil
}

func snapshotFromCommerceURL(input string, resolvedURL *url.URL) (ProductSnapshot, error) {
	host := normalizedHost(resolvedURL)
	source, externalID, canonicalURL := "", "", ""
	switch {
	case isTaobaoMainHost(host):
		source, externalID = "taobao", commerceQueryID(resolvedURL)
		canonicalURL = "https://item.taobao.com/item.htm?id=" + url.QueryEscape(externalID)
	case isTmallMainHost(host):
		source, externalID = "tmall", commerceQueryID(resolvedURL)
		canonicalURL = "https://detail.tmall.com/item.htm?id=" + url.QueryEscape(externalID)
	case is1688MainHost(host):
		externalID = offerIDFromURL(resolvedURL)
		source, canonicalURL = "1688", "https://detail.1688.com/offer/"+url.PathEscape(externalID)+".html"
	}
	if source == "" || !digitsOnly.MatchString(strings.TrimSpace(externalID)) {
		return ProductSnapshot{}, fmt.Errorf("%w: product id is absent", ErrProductMissing)
	}
	name := shareTextProductName(input)
	missing := []string{"images", "description", "core_selling_points"}
	if name == "" {
		missing = append([]string{"product_name"}, missing...)
	}
	return ProductSnapshot{
		Source: source, ProductID: externalID, Name: name, Images: []ProductImage{},
		Price: ProductPrice{Currency: "CNY", DisplayUnconfirmed: true}, SourceURL: canonicalURL,
		ResolutionStatus: ResolutionManualRequired, ResourceType: ResourceProduct, MissingFields: missing,
	}, nil
}

func commerceQueryID(value *url.URL) string {
	if value == nil {
		return ""
	}
	query := value.Query()
	return firstNonEmpty(query.Get("id"), query.Get("itemId"), query.Get("item_id"), query.Get("item_id_num"))
}

func offerIDFromURL(value *url.URL) string {
	if value == nil {
		return ""
	}
	parts := strings.Split(strings.Trim(value.Path, "/"), "/")
	for index := range parts {
		if parts[index] == "offer" && index+1 < len(parts) {
			return strings.TrimSuffix(parts[index+1], ".html")
		}
	}
	return firstNonEmpty(value.Query().Get("offerId"), value.Query().Get("offer_id"), value.Query().Get("id"))
}

func shareTextProductName(input string) string {
	if match := quotedProductNamePattern.FindStringSubmatch(input); len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	location := httpsURLPattern.FindStringIndex(input)
	if location == nil {
		return ""
	}
	value := strings.TrimSpace(input[:location[0]])
	for _, marker := range []string{"【淘宝】", "【天猫】", "【抖音商城】", "【1688】"} {
		value = strings.ReplaceAll(value, marker, "")
	}
	value = strings.TrimSpace(strings.Trim(value, "：:，,。.!！?？#"))
	if len([]rune(value)) > 120 || strings.Contains(value, "长按复制") {
		return ""
	}
	return value
}

func commerceTargetFromHTML(body []byte) *url.URL {
	values := []string{html.UnescapeString(string(body))}
	values[0] = strings.ReplaceAll(values[0], `\/`, `/`)
	for decodePass := 0; decodePass < 2; decodePass++ {
		decoded, err := url.QueryUnescape(values[len(values)-1])
		if err != nil || decoded == values[len(values)-1] {
			break
		}
		values = append(values, decoded)
	}
	for _, value := range values {
		for _, match := range commerceTargetURLPattern.FindAllString(value, -1) {
			candidate := strings.TrimRight(match, `,.;:!?)]}\\`)
			parsed, err := url.Parse(candidate)
			if err == nil && validateApprovedCommerceURL(parsed, false) == nil && isMainCommerceHost(normalizedHost(parsed)) {
				return parsed
			}
		}
	}
	return nil
}

func enrichCommerceSnapshot(snapshot ProductSnapshot, body []byte) ProductSnapshot {
	metadata := commerceMetadataFromHTML(body)
	if snapshot.Name == "" && !isGenericCommerceTitle(metadata.title) {
		snapshot.Name = metadata.title
	}
	if metadata.description != "" {
		snapshot.Description = metadata.description
	}
	if metadata.image != "" && validPublicMediaURL(metadata.image) {
		snapshot.Images = []ProductImage{{URL: metadata.image, Role: "main"}}
	}
	missing := []string{"core_selling_points"}
	if snapshot.Name == "" {
		missing = append([]string{"product_name"}, missing...)
	}
	if len(snapshot.Images) == 0 {
		missing = append(missing, "images")
	}
	if snapshot.Description == "" {
		missing = append(missing, "description")
	}
	snapshot.MissingFields = missing
	if snapshot.Name == "" || len(snapshot.Images) == 0 {
		snapshot.ResolutionStatus = ResolutionManualRequired
	} else {
		snapshot.ResolutionStatus = ResolutionPartial
	}
	return snapshot
}

type commerceMetadata struct {
	title, description, image string
}

func commerceMetadataFromHTML(body []byte) commerceMetadata {
	metadata := commerceMetadata{}
	for _, tag := range metaTagPattern.FindAllString(string(body), -1) {
		attributes := map[string]string{}
		for _, match := range metaAttributePattern.FindAllStringSubmatch(tag, -1) {
			attributes[strings.ToLower(match[1])] = strings.TrimSpace(html.UnescapeString(match[2]))
		}
		key := strings.ToLower(firstNonEmpty(attributes["property"], attributes["name"], attributes["itemprop"]))
		content := attributes["content"]
		switch key {
		case "og:title", "twitter:title":
			if metadata.title == "" {
				metadata.title = content
			}
		case "og:description", "twitter:description", "description":
			if metadata.description == "" {
				metadata.description = content
			}
		case "og:image", "twitter:image", "image":
			if metadata.image == "" {
				metadata.image = content
			}
		}
	}
	if metadata.title == "" {
		if match := htmlTitlePattern.FindStringSubmatch(string(body)); len(match) == 2 {
			metadata.title = strings.TrimSpace(html.UnescapeString(match[1]))
		}
	}
	return metadata
}

func isGenericCommerceTitle(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "" || value == "淘宝网 - 淘！我喜欢" || value == "天猫tmall.com--理想生活上天猫" || value == "1688"
}

func validPublicMediaURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Port() != "" || parsed.Hostname() == "" {
		return false
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
		return false
	}
	return true
}

func validateApprovedCommerceURL(value *url.URL, allowShort bool) error {
	if value == nil || value.Scheme != "https" || value.User != nil || value.Port() != "" {
		return fmt.Errorf("%w: only HTTPS commerce links without credentials or custom ports are allowed", ErrUnsupportedLink)
	}
	host := normalizedHost(value)
	if isMainCommerceHost(host) || isCommerceLandingHost(host) || (allowShort && isCommerceShortHost(host)) {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast()) {
		return fmt.Errorf("%w: private network target is not allowed", ErrUnsupportedLink)
	}
	return fmt.Errorf("%w: commerce host %q is not allowed", ErrUnsupportedLink, host)
}

func normalizedHost(value *url.URL) string {
	if value == nil {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(value.Hostname(), "."))
}

func isDouyinHost(host string) bool { return host == "v.douyin.com" || host == "haohuo.jinritemai.com" }
func isTaobaoMainHost(host string) bool {
	return host == "item.taobao.com" || host == "h5.m.taobao.com" || host == "item.m.taobao.com" || host == "main.m.taobao.com"
}
func isTmallMainHost(host string) bool {
	return host == "detail.tmall.com" || host == "detail.m.tmall.com" || host == "detail.tmall.hk" || host == "chaoshi.detail.tmall.com"
}
func is1688MainHost(host string) bool { return host == "detail.1688.com" || host == "m.1688.com" }
func isMainCommerceHost(host string) bool {
	return isDouyinHost(host) || isTaobaoMainHost(host) || isTmallMainHost(host) || is1688MainHost(host)
}
func isCommerceShortHost(host string) bool {
	return host == "e.tb.cn" || host == "m.tb.cn" || host == "s.tb.cn" || host == "qr.1688.com"
}
func isCommerceLandingHost(host string) bool {
	return host == "market.m.taobao.com" || host == "pages.tmall.com"
}
func isSupportedProductHost(host string) bool {
	return isMainCommerceHost(host) || isCommerceShortHost(host)
}
