package productsource

import (
	"context"
	"fmt"
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
	ResourceNote             = "note"
)

var digitsOnly = regexp.MustCompile(`^[0-9]+$`)

// Resolver is the single commerce-source seam used by Creative. Platform URL
// rules and redirect policy remain local to this package.
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
			return validateApprovedCommerceURL(request.URL, false)
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
	resolvedURL := productURL
	if isCommerceShortHost(host) {
		if r.Client == nil {
			return ProductSnapshot{}, fmt.Errorf("commerce product resolver client is required")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, productURL.String(), nil)
		if err != nil {
			return ProductSnapshot{}, fmt.Errorf("create commerce product request: %w", err)
		}
		request.Header.Set("User-Agent", resolverUserAgent)
		response, err := r.Client.Do(request)
		if err != nil {
			return ProductSnapshot{}, fmt.Errorf("resolve commerce short link: %w", err)
		}
		if response == nil {
			return ProductSnapshot{}, fmt.Errorf("resolve commerce short link: empty response")
		}
		if response.Body != nil {
			defer response.Body.Close()
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxResponseBody))
		}
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || response.Request == nil || response.Request.URL == nil {
			return ProductSnapshot{}, fmt.Errorf("resolve commerce short link: invalid response")
		}
		resolvedURL = response.Request.URL
		if err := validateApprovedCommerceURL(resolvedURL, false); err != nil {
			return ProductSnapshot{}, err
		}
	}
	return snapshotFromCommerceURL(input, resolvedURL)
}

func snapshotFromCommerceURL(input string, resolvedURL *url.URL) (ProductSnapshot, error) {
	host := normalizedHost(resolvedURL)
	source, resourceType, externalID, canonicalURL := "", ResourceProduct, "", ""
	switch {
	case host == "item.taobao.com":
		source, externalID = "taobao", strings.TrimSpace(resolvedURL.Query().Get("id"))
		canonicalURL = "https://item.taobao.com/item.htm?id=" + url.QueryEscape(externalID)
	case host == "detail.tmall.com":
		source, externalID = "tmall", strings.TrimSpace(resolvedURL.Query().Get("id"))
		canonicalURL = "https://detail.tmall.com/item.htm?id=" + url.QueryEscape(externalID)
	case host == "detail.1688.com":
		parts := strings.Split(strings.Trim(resolvedURL.Path, "/"), "/")
		if len(parts) == 2 && parts[0] == "offer" {
			externalID = strings.TrimSuffix(parts[1], ".html")
		}
		source, canonicalURL = "1688", "https://detail.1688.com/offer/"+url.PathEscape(externalID)+".html"
	case isXiaohongshuHost(host):
		source = "xiaohongshu"
		parts := strings.Split(strings.Trim(resolvedURL.Path, "/"), "/")
		if len(parts) >= 3 && parts[0] == "discovery" && parts[1] == "item" {
			resourceType, externalID = ResourceNote, parts[2]
			canonicalURL = "https://www.xiaohongshu.com/explore/" + url.PathEscape(externalID)
		} else if len(parts) >= 2 && (parts[0] == "explore" || parts[0] == "discovery" || parts[0] == "note") {
			resourceType, externalID = ResourceNote, parts[1]
			canonicalURL = "https://www.xiaohongshu.com/explore/" + url.PathEscape(externalID)
		} else if len(parts) >= 2 && (parts[0] == "goods-detail" || parts[0] == "goods") {
			resourceType, externalID = ResourceProduct, parts[1]
			canonicalURL = "https://www.xiaohongshu.com/goods-detail/" + url.PathEscape(externalID)
		}
	}
	if source == "" || strings.TrimSpace(externalID) == "" || (source != "xiaohongshu" && !digitsOnly.MatchString(externalID)) {
		return ProductSnapshot{}, fmt.Errorf("%w: product or content id is absent", ErrProductMissing)
	}
	name := shareTextProductName(input)
	missing := []string{"images", "description", "core_selling_points"}
	status := ResolutionManualRequired
	if name == "" {
		missing = append([]string{"product_name"}, missing...)
	}
	return ProductSnapshot{
		Source: source, ProductID: externalID, Name: name, Images: []ProductImage{},
		Price: ProductPrice{Currency: "CNY", DisplayUnconfirmed: true}, SourceURL: canonicalURL,
		ResolutionStatus: status, ResourceType: resourceType, MissingFields: missing,
	}, nil
}

func shareTextProductName(input string) string {
	location := httpsURLPattern.FindStringIndex(input)
	if location == nil {
		return ""
	}
	value := strings.TrimSpace(input[:location[0]])
	for _, marker := range []string{"【淘宝】", "【天猫】", "【抖音商城】", "【小红书】", "【1688】"} {
		value = strings.ReplaceAll(value, marker, "")
	}
	value = strings.TrimSpace(strings.Trim(value, "：:，,。.!！?？#"))
	if len([]rune(value)) > 120 || strings.Contains(value, "长按复制") {
		return ""
	}
	return value
}

func validateApprovedCommerceURL(value *url.URL, allowShort bool) error {
	if value == nil || value.Scheme != "https" || value.User != nil || value.Port() != "" {
		return fmt.Errorf("%w: only HTTPS commerce links without credentials or custom ports are allowed", ErrUnsupportedLink)
	}
	host := normalizedHost(value)
	if isMainCommerceHost(host) || (allowShort && isCommerceShortHost(host)) {
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
func isXiaohongshuHost(host string) bool {
	return host == "xiaohongshu.com" || host == "www.xiaohongshu.com"
}
func isMainCommerceHost(host string) bool {
	return isDouyinHost(host) || host == "item.taobao.com" || host == "detail.tmall.com" || host == "detail.1688.com" || isXiaohongshuHost(host)
}
func isCommerceShortHost(host string) bool {
	return host == "m.tb.cn" || host == "s.tb.cn" || host == "xhslink.com" || host == "www.xhslink.com" || host == "qr.1688.com"
}
