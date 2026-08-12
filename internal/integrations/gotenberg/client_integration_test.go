package gotenberg

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/knowledge"
)

func TestConfiguredGotenbergConvertsPresentation(t *testing.T) {
	baseURL := os.Getenv("COOKIES_TEST_GOTENBERG_URL")
	fixturePath := os.Getenv("COOKIES_TEST_GOTENBERG_PPTX")
	if baseURL == "" || fixturePath == "" {
		t.Skip("COOKIES_TEST_GOTENBERG_URL and COOKIES_TEST_GOTENBERG_PPTX are not configured")
	}
	source, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	result, err := (Client{
		BaseURL: baseURL, Version: "gotenberg-integration", Timeout: 90 * time.Second,
		MaxPDFBytes: 8 * 1024 * 1024, AllowInsecureHTTP: true,
	}).Convert(ctx, knowledge.DocumentVisionInputConversionRequest{
		MIMEType: knowledge.PowerPointOpenXMLMIME, SizeBytes: int64(len(source)), Source: bytes.NewReader(source), TraceID: "gotenberg-integration-test",
	})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if !validPDF(result.PDF) || len(result.PDF) < 1024 {
		t.Fatalf("converted PDF is unexpectedly small or invalid: %d bytes", len(result.PDF))
	}
}
