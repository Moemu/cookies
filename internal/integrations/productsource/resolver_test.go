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
			name: "taobao mobile", input: "旅行包 https://h5.m.taobao.com/awp/core/detail.htm?id=778899", source: "taobao", resourceType: ResourceProduct,
			productID: "778899", canonicalURL: "https://item.taobao.com/item.htm?id=778899", productName: "旅行包",
		},
		{
			name: "1688 mobile", input: "钛保温杯 https://m.1688.com/offer/445566.html", source: "1688", resourceType: ResourceProduct,
			productID: "445566", canonicalURL: "https://detail.1688.com/offer/445566.html", productName: "钛保温杯",
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

func TestResolverExtractsTaobaoTargetFromHTMLShortLink(t *testing.T) {
	shortURL, _ := url.Parse("https://e.tb.cn/h.example?tk=token")
	body := `<html><script>var url = 'https://item.taobao.com/item.htm?id=1013712033785&amp;spm=tracking';</script></html>`
	resolver := Resolver{Client: doerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: &http.Request{URL: shortURL}}, nil
	})}

	value, err := resolver.Resolve(context.Background(), "【淘宝】7天无理由退货 https://e.tb.cn/h.example?tk=token MF278 「Unique blue波点托特包」")
	if err != nil {
		t.Fatal(err)
	}
	if value.Source != "taobao" || value.ProductID != "1013712033785" || value.Name != "Unique blue波点托特包" {
		t.Fatalf("unexpected HTML short-link resolution: %#v", value)
	}
}

func TestResolverEnrichesDirectCommerceLinkFromPublicMetadata(t *testing.T) {
	productURL, _ := url.Parse("https://detail.1688.com/offer/99887766.html")
	body := `<html><head><meta property="og:title" content="商务通勤双肩包"><meta property="og:image" content="https://cbu01.alicdn.com/img/ibank/example.jpg"><meta name="description" content="耐磨面料，可扩容设计"></head></html>`
	resolver := Resolver{Client: doerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: &http.Request{URL: productURL}}, nil
	})}

	value, err := resolver.Resolve(context.Background(), productURL.String())
	if err != nil {
		t.Fatal(err)
	}
	if value.Name != "商务通勤双肩包" || value.Description != "耐磨面料，可扩容设计" || len(value.Images) != 1 || value.ResolutionStatus != ResolutionPartial {
		t.Fatalf("unexpected public metadata enrichment: %#v", value)
	}
}

func TestResolverChoosesSupportedProductURLWhenShareTextContainsSeveralLinks(t *testing.T) {
	productURL, _ := url.Parse("https://item.taobao.com/item.htm?id=24680")
	resolver := Resolver{Client: doerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Request: &http.Request{URL: productURL}}, nil
	})}

	value, err := resolver.Resolve(context.Background(), "活动页 https://example.com/promo 商品 https://item.taobao.com/item.htm?id=24680")
	if err != nil {
		t.Fatal(err)
	}
	if value.Source != "taobao" || value.ProductID != "24680" {
		t.Fatalf("unexpected product selection: %#v", value)
	}
}

func TestResolverRejectsXiaohongshuProductSource(t *testing.T) {
	_, err := NewResolver().Resolve(context.Background(), "https://www.xiaohongshu.com/goods-detail/778899")
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("xiaohongshu must not remain an AI native product source, got %v", err)
	}
}

func TestResolverRejectsShortLinkThatEscapesApprovedCommerceHosts(t *testing.T) {
	evilURL, _ := url.Parse("https://example.com/product/1")
	resolver := Resolver{Client: doerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Request: &http.Request{URL: evilURL}}, nil
	})}

	_, err := resolver.Resolve(context.Background(), "https://e.tb.cn/h.test")
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
