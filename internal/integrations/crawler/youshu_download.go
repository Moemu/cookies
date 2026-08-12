package crawler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/shikanon/cookies/internal/platform/assets"
)

// YouShuDownloadErrorKind describes a download policy failure without
// retaining an upstream URL, response body, request headers, or session.
type YouShuDownloadErrorKind string

const (
	YouShuDownloadInvalidURL     YouShuDownloadErrorKind = "invalid_url"
	YouShuDownloadForbiddenHost  YouShuDownloadErrorKind = "forbidden_host"
	YouShuDownloadExpiredURL     YouShuDownloadErrorKind = "expired_url"
	YouShuDownloadHTTPError      YouShuDownloadErrorKind = "http_error"
	YouShuDownloadServerError    YouShuDownloadErrorKind = "server_error"
	YouShuDownloadTimeout        YouShuDownloadErrorKind = "timeout"
	YouShuDownloadTransport      YouShuDownloadErrorKind = "transport_error"
	YouShuDownloadNotMP4         YouShuDownloadErrorKind = "not_mp4"
	YouShuDownloadTooLarge       YouShuDownloadErrorKind = "too_large"
	YouShuDownloadLengthMismatch YouShuDownloadErrorKind = "length_mismatch"
)

type YouShuDownloadError struct {
	Kind   YouShuDownloadErrorKind
	Source string
	Status int
}

func (e *YouShuDownloadError) Error() string {
	return fmt.Sprintf("youshu download %s (%s, status=%d)", e.Kind, e.Source, e.Status)
}

// YouShuDownloadedVideo is the verified result of a successful stream.
type YouShuDownloadedVideo struct {
	SizeBytes int64
	SHA256    string
}

// YouShuDownloader applies the network and media policy for an already
// authorized upstream media URL. AllowedHosts must be explicitly populated;
// each item is a hostname (an optional port is ignored for matching).
type YouShuDownloader struct {
	HTTPClient   *http.Client
	AllowedHosts []string
	MaxRedirects int
}

// Download streams a verified MP4 to dst. expectedSize is an optional size
// advertised by the upstream GraphQL payload; zero means unknown.
func (d *YouShuDownloader) Download(ctx context.Context, rawURL string, expectedSize int64, dst io.Writer) (YouShuDownloadedVideo, error) {
	if dst == nil || expectedSize < 0 {
		return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadInvalidURL, Source: "request"}
	}
	if len(d.AllowedHosts) == 0 {
		return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadForbiddenHost, Source: "allowlist"}
	}
	u, err := url.Parse(rawURL)
	if err != nil || !d.allowedURL(u) {
		return YouShuDownloadedVideo{}, d.urlError(u, err)
	}
	if expectedSize > assets.MaxVideoBytes {
		return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadTooLarge, Source: "expected_size"}
	}

	client := d.httpClient()
	redirects := d.MaxRedirects
	if redirects <= 0 {
		redirects = 5
	}
	for redirect := 0; ; redirect++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadInvalidURL, Source: "request"}
		}
		resp, err := client.Do(req)
		if err != nil {
			return YouShuDownloadedVideo{}, downloadTransportError(ctx, err)
		}
		if isRedirect(resp.StatusCode) {
			location, parseErr := resp.Location()
			resp.Body.Close()
			if parseErr != nil {
				return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadInvalidURL, Source: "redirect"}
			}
			u = u.ResolveReference(location)
			if !d.allowedURL(u) {
				return YouShuDownloadedVideo{}, d.urlError(u, nil)
			}
			if redirect >= redirects {
				return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadHTTPError, Source: "redirect_limit", Status: resp.StatusCode}
			}
			continue
		}
		return d.copyResponse(resp, expectedSize, dst)
	}
}

func (d *YouShuDownloader) httpClient() *http.Client {
	base := d.HTTPClient
	if base == nil {
		base = http.DefaultClient
	}
	copy := *base
	// Redirects are followed explicitly so every target is validated.
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &copy
}

func (d *YouShuDownloader) allowedURL(u *url.URL) bool {
	if u == nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, value := range d.AllowedHosts {
		candidate := strings.TrimSpace(strings.ToLower(value))
		if parsed, err := url.Parse("//" + candidate); err == nil {
			candidate = parsed.Hostname()
		}
		if candidate != "" && candidate == host {
			return true
		}
	}
	return false
}

func (d *YouShuDownloader) urlError(u *url.URL, parseErr error) error {
	if parseErr != nil || u == nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
		return &YouShuDownloadError{Kind: YouShuDownloadInvalidURL, Source: "url"}
	}
	return &YouShuDownloadError{Kind: YouShuDownloadForbiddenHost, Source: "host"}
}

func (d *YouShuDownloader) copyResponse(resp *http.Response, expectedSize int64, dst io.Writer) (YouShuDownloadedVideo, error) {
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadExpiredURL, Source: "http", Status: resp.StatusCode}
	}
	if resp.StatusCode >= 500 {
		return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadServerError, Source: "http", Status: resp.StatusCode}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadHTTPError, Source: "http", Status: resp.StatusCode}
	}
	if !isMP4ContentType(resp.Header.Get("Content-Type")) {
		return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadNotMP4, Source: "content_type", Status: resp.StatusCode}
	}
	if resp.ContentLength > assets.MaxVideoBytes || (expectedSize > 0 && resp.ContentLength >= 0 && resp.ContentLength != expectedSize) {
		kind := YouShuDownloadLengthMismatch
		if resp.ContentLength > assets.MaxVideoBytes {
			kind = YouShuDownloadTooLarge
		}
		return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: kind, Source: "content_length", Status: resp.StatusCode}
	}

	prefix := make([]byte, 32)
	n, readErr := io.ReadFull(resp.Body, prefix)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadLengthMismatch, Source: "body", Status: resp.StatusCode}
	}
	prefix = prefix[:n]
	if !isMP4Container(prefix) {
		return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadNotMP4, Source: "container", Status: resp.StatusCode}
	}
	hash := sha256.New()
	reader := io.MultiReader(bytes.NewReader(prefix), resp.Body)
	// Do not write an overflow byte to the caller's destination.  Probe for it
	// only after the capped stream has been copied.
	written, copyErr := io.Copy(io.MultiWriter(dst, hash), io.LimitReader(reader, assets.MaxVideoBytes))
	if copyErr != nil {
		return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadLengthMismatch, Source: "body", Status: resp.StatusCode}
	}
	var overflow [1]byte
	overflowBytes, overflowErr := reader.Read(overflow[:])
	if overflowBytes > 0 {
		return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadTooLarge, Source: "stream", Status: resp.StatusCode}
	}
	if overflowErr != nil && !errors.Is(overflowErr, io.EOF) {
		return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadLengthMismatch, Source: "body", Status: resp.StatusCode}
	}
	if (resp.ContentLength >= 0 && written != resp.ContentLength) || (expectedSize > 0 && written != expectedSize) {
		return YouShuDownloadedVideo{}, &YouShuDownloadError{Kind: YouShuDownloadLengthMismatch, Source: "stream", Status: resp.StatusCode}
	}
	return YouShuDownloadedVideo{SizeBytes: written, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}

func isRedirect(status int) bool { return status >= 300 && status <= 399 }

func isMP4ContentType(value string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	return mediaType == "video/mp4"
}

// ISO BMFF MP4 files start with a box header whose type is ftyp.  Requiring
// that signature supplements (rather than replaces) the declared MIME type.
func isMP4Container(prefix []byte) bool {
	if len(prefix) < 12 || !bytes.Equal(prefix[4:8], []byte("ftyp")) {
		return false
	}
	size := int64(uint32(prefix[0])<<24 | uint32(prefix[1])<<16 | uint32(prefix[2])<<8 | uint32(prefix[3]))
	return size == 0 || size == 1 || size >= 8
}

func downloadTransportError(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return &YouShuDownloadError{Kind: YouShuDownloadTimeout, Source: "transport"}
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) && errors.Is(urlErr.Err, context.DeadlineExceeded) {
		return &YouShuDownloadError{Kind: YouShuDownloadTimeout, Source: "transport"}
	}
	return &YouShuDownloadError{Kind: YouShuDownloadTransport, Source: "transport"}
}

// ParseExpectedYouShuSize is a small helper for callers whose source field is
// textual. It intentionally reveals no upstream URL on invalid input.
func ParseExpectedYouShuSize(value string) (int64, error) {
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil || n < 0 || n > assets.MaxVideoBytes {
		return 0, &YouShuDownloadError{Kind: YouShuDownloadInvalidURL, Source: "expected_size"}
	}
	return n, nil
}
