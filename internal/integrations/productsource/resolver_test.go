package productsource

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestResolverRecognizesMainlandCommerceLinksWithoutInventingDetails(t *testing.T) {
	tests := []struct {
		name, input, source, resourceType, productID, canonicalURL, productName string
	}{
		{
			name: "taobao", input: "【淘宝】轻量通勤双肩包 https://item.taobao.com/item.htm?id=123456789&spm=a21n57.1", source: "taobao", resourceType: ResourceProduct,
			productID: "123456789", canonicalURL: "https://item.taobao.com/item.htm?id=123456789", productName: "轻量通勤双肩包",
		},
		{
			name: "tmall", input: "【天猫】纯钛随行杯 https://detail.tmall.com/item.htm?id=22334455&skuId=1", source: "tmall", resourceType: ResourceProduct,
			productID: "22334455", canonicalURL: "https://detail.tmall.com/item.htm?id=22334455", productName: "纯钛随行杯",
		},
		{
			name: "1688", input: "商务双肩包 https://detail.1688.com/offer/99887766.html?spm=a260k.1", source: "1688", resourceType: ResourceProduct,
			productID: "99887766", canonicalURL: "https://detail.1688.com/offer/99887766.html", productName: "商务双肩包",
		},
		{
			name: "xiaohongshu note", input: "通勤好物分享 https://www.xiaohongshu.com/explore/66aa11bb22cc33dd44ee55ff?xsec_token=tracking", source: "xiaohongshu", resourceType: ResourceNote,
			productID: "66aa11bb22cc33dd44ee55ff", canonicalURL: "https://www.xiaohongshu.com/explore/66aa11bb22cc33dd44ee55ff", productName: "通勤好物分享",
		},
		{
			name: "xiaohongshu discovery item", input: "https://www.xiaohongshu.com/discovery/item/77bb22cc33dd44ee55ff66aa?xsec_token=tracking", source: "xiaohongshu", resourceType: ResourceNote,
			productID: "77bb22cc33dd44ee55ff66aa", canonicalURL: "https://www.xiaohongshu.com/explore/77bb22cc33dd44ee55ff66aa",
		},
		{
			name: "xiaohongshu product", input: "轻量双肩包 https://www.xiaohongshu.com/goods-detail/778899?xsec_source=app_share", source: "xiaohongshu", resourceType: ResourceProduct,
			productID: "778899", canonicalURL: "https://www.xiaohongshu.com/goods-detail/778899", productName: "轻量双肩包",
		},
	}

	resolver := NewResolver()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := resolver.Resolve(context.Background(), tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if value.Source != tt.source || value.ResourceType != tt.resourceType || value.ProductID != tt.productID || value.SourceURL != tt.canonicalURL || value.Name != tt.productName {
				t.Fatalf("unexpected resolution: %#v", value)
			}
			if value.ResolutionStatus != ResolutionManualRequired || len(value.Images) != 0 || !containsString(value.MissingFields, "images") {
				t.Fatalf("link-only resolution must request a product image: %#v", value)
			}
		})
	}
}

func TestResolverFollowsControlledShortLinkAndRevalidatesFinalPlatform(t *testing.T) {
	finalURL, _ := url.Parse("https://item.taobao.com/item.htm?id=13579&spm=tracking")
	resolver := Resolver{Client: doerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Request: &http.Request{URL: finalURL}}, nil
	})}

	value, err := resolver.Resolve(context.Background(), "【淘宝】旅行背包 https://m.tb.cn/h.test")
	if err != nil {
		t.Fatal(err)
	}
	if value.Source != "taobao" || value.ProductID != "13579" || value.SourceURL != "https://item.taobao.com/item.htm?id=13579" {
		t.Fatalf("unexpected short-link resolution: %#v", value)
	}
}

func TestResolverRejectsShortLinkThatEscapesApprovedCommerceHosts(t *testing.T) {
	evilURL, _ := url.Parse("https://example.com/product/1")
	resolver := Resolver{Client: doerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Request: &http.Request{URL: evilURL}}, nil
	})}

	_, err := resolver.Resolve(context.Background(), "https://xhslink.com/a/test")
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("untrusted redirect must be rejected, got %v", err)
	}
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
