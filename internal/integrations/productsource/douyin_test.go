package productsource

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(request *http.Request) (*http.Response, error) { return f(request) }

func TestDouyinResolverResolvesShareMessage(t *testing.T) {
	detail, err := json.Marshal(map[string]any{
		"title":     "simelo施美乐纯钛保温杯",
		"sales":     3221,
		"min_price": 79900,
		"max_price": 89900,
		"img": map[string]any{"url_list": []string{
			"https://p26-item.ecombdimg.com/img/product.png",
			"https://p3-item.ecombdimg.com/img/product.png",
			"https://untrusted.example/product.png",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	finalURL, err := url.Parse("https://haohuo.jinritemai.com/ecommerce/trade/detail/index.html?id=3802315260866724312&goods_detail=" + url.QueryEscape(string(detail)))
	if err != nil {
		t.Fatal(err)
	}
	resolver := DouyinResolver{Client: doerFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodGet || request.Header.Get("User-Agent") != resolverUserAgent {
			t.Fatalf("unexpected request: method=%s user-agent=%s", request.Method, request.Header.Get("User-Agent"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Request:    &http.Request{URL: finalURL},
		}, nil
	})}

	snapshot, err := resolver.Resolve(context.Background(), "长按复制 https://v.douyin.com/xcYJuBqvxDM/ 打开抖音")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Source != douyinSource || snapshot.ProductID != "3802315260866724312" || snapshot.Name != "simelo施美乐纯钛保温杯" {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if snapshot.Price.MinRaw != 79900 || snapshot.Price.MaxRaw != 89900 || !snapshot.Price.DisplayUnconfirmed || snapshot.Sales != 3221 {
		t.Fatalf("unexpected commerce fields: %#v", snapshot)
	}
	if len(snapshot.Images) != 1 || snapshot.Images[0].Role != "main" || !strings.Contains(snapshot.Images[0].URL, "p26-item") {
		t.Fatalf("only the first trusted CDN mirror should remain: %#v", snapshot.Images)
	}
	if snapshot.SourceURL != "https://v.douyin.com/xcYJuBqvxDM/" {
		t.Fatalf("tracking-free source URL expected, got %q", snapshot.SourceURL)
	}
}

func TestDouyinResolverRejectsUnsupportedHostBeforeRequest(t *testing.T) {
	called := false
	resolver := DouyinResolver{Client: doerFunc(func(request *http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("must not be called")
	})}
	_, err := resolver.Resolve(context.Background(), "https://example.com/product/1")
	if !errors.Is(err, ErrUnsupportedLink) || called {
		t.Fatalf("expected unsupported link without network request, err=%v called=%v", err, called)
	}
}

func TestDouyinResolverRejectsMissingProductPayload(t *testing.T) {
	finalURL, _ := url.Parse("https://haohuo.jinritemai.com/ecommerce/trade/detail/index.html?id=123")
	resolver := DouyinResolver{Client: doerFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Request: &http.Request{URL: finalURL}}, nil
	})}
	_, err := resolver.Resolve(context.Background(), "https://v.douyin.com/example/")
	if !errors.Is(err, ErrProductMissing) {
		t.Fatalf("expected missing product error, got %v", err)
	}
}

func TestDouyinRedirectPolicyRejectsUntrustedHostAndExcessRedirects(t *testing.T) {
	resolver := NewDouyinResolver()
	client, ok := resolver.Client.(*http.Client)
	if !ok {
		t.Fatal("default resolver must use http.Client")
	}
	evil, _ := http.NewRequest(http.MethodGet, "https://example.com/steal", nil)
	if err := client.CheckRedirect(evil, nil); !errors.Is(err, ErrUnsupportedLink) {
		t.Fatalf("expected untrusted redirect rejection, got %v", err)
	}
	allowed, _ := http.NewRequest(http.MethodGet, "https://haohuo.jinritemai.com/product", nil)
	via := make([]*http.Request, maxRedirects)
	if err := client.CheckRedirect(allowed, via); err == nil || !strings.Contains(err.Error(), "redirect limit") {
		t.Fatalf("expected redirect limit error, got %v", err)
	}
}
