package crawler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/assets"
)

func testMP4() []byte {
	// A minimal ISO BMFF ftyp box followed by synthetic payload. The downloader
	// checks the container signature, not decoder-level media semantics.
	return append([]byte{0, 0, 0, 16, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm', 0, 0, 0, 0}, []byte("test-video")...)
}

func tlsDownloader(server *httptest.Server) *YouShuDownloader {
	u, _ := url.Parse(server.URL)
	return &YouShuDownloader{HTTPClient: server.Client(), AllowedHosts: []string{u.Hostname()}}
}

func requireDownloadError(t *testing.T, err error, kind YouShuDownloadErrorKind) {
	t.Helper()
	var got *YouShuDownloadError
	if !errors.As(err, &got) || got.Kind != kind {
		t.Fatalf("error=%v, want %s", err, kind)
	}
}

func TestYouShuDownloaderRejectsRedirectToForbiddenHost(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://untrusted.invalid/video.mp4?token=secret", http.StatusFound)
	}))
	defer server.Close()

	_, err := tlsDownloader(server).Download(context.Background(), server.URL+"/start?signature=private", 0, &bytes.Buffer{})
	requireDownloadError(t, err, YouShuDownloadForbiddenHost)
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "signature") {
		t.Fatalf("error leaked URL details: %v", err)
	}
}

func TestYouShuDownloaderRevalidatesAllowedRedirect(t *testing.T) {
	body := testMP4()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/video.mp4", http.StatusTemporaryRedirect)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(body)
	}))
	defer server.Close()
	var got bytes.Buffer
	result, err := tlsDownloader(server).Download(context.Background(), server.URL+"/start", int64(len(body)), &got)
	if err != nil || result.SizeBytes != int64(len(body)) || !bytes.Equal(got.Bytes(), body) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestYouShuDownloaderRequiresExplicitAllowlistAndHTTPS(t *testing.T) {
	d := &YouShuDownloader{}
	_, err := d.Download(context.Background(), "https://media.example/video.mp4", 0, &bytes.Buffer{})
	requireDownloadError(t, err, YouShuDownloadForbiddenHost)

	d.AllowedHosts = []string{"media.example"}
	_, err = d.Download(context.Background(), "http://media.example/video.mp4", 0, &bytes.Buffer{})
	requireDownloadError(t, err, YouShuDownloadInvalidURL)
}

func TestYouShuDownloaderRejectsNonMP4(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("not a media container"))
	}))
	defer server.Close()
	_, err := tlsDownloader(server).Download(context.Background(), server.URL, 0, &bytes.Buffer{})
	requireDownloadError(t, err, YouShuDownloadNotMP4)
}

func TestYouShuDownloaderRejectsOversizeDeclaredLength(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Length", "209715201")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	_, err := tlsDownloader(server).Download(context.Background(), server.URL, 0, &bytes.Buffer{})
	requireDownloadError(t, err, YouShuDownloadTooLarge)
}

func TestYouShuDownloaderRejectsLengthMismatch(t *testing.T) {
	body := testMP4()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(body)
	}))
	defer server.Close()
	_, err := tlsDownloader(server).Download(context.Background(), server.URL, int64(len(body)+1), &bytes.Buffer{})
	requireDownloadError(t, err, YouShuDownloadLengthMismatch)
}

func TestYouShuDownloaderClassifiesExpiredURL(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	_, err := tlsDownloader(server).Download(context.Background(), server.URL+"?expires=private", 0, &bytes.Buffer{})
	requireDownloadError(t, err, YouShuDownloadExpiredURL)
}

func TestYouShuDownloaderStreamsVerifiedHash(t *testing.T) {
	body := testMP4()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4; charset=binary")
		_, _ = w.Write(body)
	}))
	defer server.Close()
	var got bytes.Buffer
	result, err := tlsDownloader(server).Download(context.Background(), server.URL, int64(len(body)), &got)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(body)
	if result.SizeBytes != int64(len(body)) || result.SHA256 != stringHex(wantHash[:]) || !bytes.Equal(got.Bytes(), body) {
		t.Fatalf("result=%+v bytes=%d", result, got.Len())
	}
}

func stringHex(value []byte) string {
	const digits = "0123456789abcdef"
	var out strings.Builder
	out.Grow(len(value) * 2)
	for _, b := range value {
		out.WriteByte(digits[b>>4])
		out.WriteByte(digits[b&15])
	}
	return out.String()
}

func TestYouShuDownloaderRejectsExpectedOversize(t *testing.T) {
	d := &YouShuDownloader{AllowedHosts: []string{"media.example"}}
	_, err := d.Download(context.Background(), "https://media.example/video.mp4", assets.MaxVideoBytes+1, &bytes.Buffer{})
	requireDownloadError(t, err, YouShuDownloadTooLarge)
}
