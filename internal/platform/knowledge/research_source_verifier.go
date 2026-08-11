package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const maxResearchSourceBytes int64 = 2 * 1024 * 1024

var researchHTMLTag = regexp.MustCompile(`(?s)<[^>]*>`)

// SafeHTTPResearchSourceVerifier is intentionally narrower than a browser. It
// only reads bounded public HTTP(S) text, rejects private/link-local targets on
// every dial and redirect, and never executes page instructions or scripts.
type SafeHTTPResearchSourceVerifier struct {
	Timeout  time.Duration
	Resolver *net.Resolver
}

func (v SafeHTTPResearchSourceVerifier) Verify(ctx context.Context, rawURL, excerpt string) (VerifiedResearchSource, error) {
	canonical, _, err := canonicalResearchURL(rawURL)
	if err != nil {
		return VerifiedResearchSource{}, err
	}
	excerpt = strings.TrimSpace(excerpt)
	if len([]rune(excerpt)) < 12 || len([]rune(excerpt)) > 800 {
		return VerifiedResearchSource{}, fmt.Errorf("research source excerpt is outside the verification bounds")
	}
	timeout := v.Timeout
	if timeout <= 0 || timeout > 20*time.Second {
		timeout = 8 * time.Second
	}
	resolver := v.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          8,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   timeout,
		ResponseHeaderTimeout: timeout,
		DialContext: func(dialCtx context.Context, network, address string) (net.Conn, error) {
			host, port, splitErr := net.SplitHostPort(address)
			if splitErr != nil {
				return nil, splitErr
			}
			addresses, lookupErr := resolver.LookupIPAddr(dialCtx, host)
			if lookupErr != nil || len(addresses) == 0 {
				return nil, fmt.Errorf("research source host could not be resolved")
			}
			for _, address := range addresses {
				if !publicResearchIP(address.IP) {
					return nil, fmt.Errorf("research source resolved to a non-public address")
				}
			}
			return dialer.DialContext(dialCtx, network, net.JoinHostPort(addresses[0].IP.String(), port))
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 4 {
				return fmt.Errorf("research source redirected too many times")
			}
			_, _, redirectErr := canonicalResearchURL(request.URL.String())
			return redirectErr
		},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, canonical, nil)
	if err != nil {
		return VerifiedResearchSource{}, err
	}
	request.Header.Set("Accept", "text/html, text/plain, application/json;q=0.8")
	request.Header.Set("User-Agent", "cookies-research-verifier/1.0")
	response, err := client.Do(request)
	if err != nil {
		return VerifiedResearchSource{}, fmt.Errorf("research source body could not be read")
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return VerifiedResearchSource{}, fmt.Errorf("research source returned HTTP %d", response.StatusCode)
	}
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]))
	if mediaType != "" && mediaType != "text/html" && mediaType != "text/plain" && mediaType != "application/json" && !strings.HasSuffix(mediaType, "+json") {
		return VerifiedResearchSource{}, fmt.Errorf("research source content type is not verifiable text")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResearchSourceBytes+1))
	if err != nil || int64(len(body)) > maxResearchSourceBytes {
		return VerifiedResearchSource{}, fmt.Errorf("research source body exceeded the verification limit")
	}
	sum := sha256.Sum256(body)
	plain := html.UnescapeString(researchHTMLTag.ReplaceAllString(string(body), " "))
	found := strings.Contains(normalizedVerificationText(plain), normalizedVerificationText(excerpt))
	return VerifiedResearchSource{ContentHash: hex.EncodeToString(sum[:]), ExcerptFound: found}, nil
}

func publicResearchIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsMulticast() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
		return false
	}
	return ip.IsGlobalUnicast()
}

func normalizedVerificationText(value string) string {
	var normalized strings.Builder
	for _, character := range strings.ToLower(value) {
		if unicode.IsLetter(character) || unicode.IsNumber(character) {
			normalized.WriteRune(character)
		}
	}
	return normalized.String()
}
