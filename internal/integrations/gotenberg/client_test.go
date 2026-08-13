package gotenberg

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/knowledge"
)

func TestConvertUsesBoundedSinglePPTXMultipartAndReturnsValidatedPDF(t *testing.T) {
	t.Parallel()
	source := []byte("pptx fixture")
	pdf := []byte("%PDF-1.7\nfixture\n%%EOF")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != convertEndpointPath {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Gotenberg-Trace"); got != "trace-bad" {
			t.Errorf("Gotenberg-Trace = %q", got)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm() error = %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		files := r.MultipartForm.File["files"]
		if len(files) != 1 || files[0].Filename != "source.pptx" {
			t.Errorf("files = %#v", files)
		}
		file, err := files[0].Open()
		if err != nil {
			t.Errorf("open upload: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer file.Close()
		data, _ := io.ReadAll(file)
		if !bytes.Equal(data, source) {
			t.Errorf("uploaded source = %q", data)
		}
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(pdf)
	}))
	defer server.Close()

	client := Client{BaseURL: server.URL, AllowInsecureHTTP: true, Version: "gotenberg-test", MaxPDFBytes: 1024}
	result, err := client.Convert(context.Background(), knowledge.DocumentVisionInputConversionRequest{
		MIMEType: knowledge.PowerPointOpenXMLMIME, SizeBytes: int64(len(source)), Source: bytes.NewReader(source), TraceID: "trace-\r\nbad",
	})
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}
	if !bytes.Equal(result.PDF, pdf) || result.ConverterCode != converterCode || result.Version != "gotenberg-test" {
		t.Fatalf("Convert() = %#v", result)
	}
}

func TestConvertClassifiesDeterministicAndRetryableFailuresWithoutLeakingBody(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status    int
		wantCode  string
		retryable bool
	}{
		{status: http.StatusBadRequest, wantCode: "DOCUMENT_VISION_CONVERSION_REJECTED", retryable: false},
		{status: http.StatusServiceUnavailable, wantCode: "DOCUMENT_VISION_CONVERTER_UNAVAILABLE", retryable: true},
	}
	for _, item := range tests {
		item := item
		t.Run(http.StatusText(item.status), func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(item.status)
				_, _ = w.Write([]byte("vendor secret internal path"))
			}))
			defer server.Close()
			client := Client{BaseURL: server.URL, AllowInsecureHTTP: true}
			_, err := client.Convert(context.Background(), knowledge.DocumentVisionInputConversionRequest{
				MIMEType: knowledge.PowerPointOpenXMLMIME, SizeBytes: 1, Source: strings.NewReader("x"),
			})
			conversionError, ok := knowledge.AsDocumentVisionInputConversionError(err)
			if !ok || conversionError.Code != item.wantCode || conversionError.Retryable != item.retryable {
				t.Fatalf("error = %#v, ok=%v", conversionError, ok)
			}
			if strings.Contains(conversionError.Message, "vendor secret") {
				t.Fatalf("upstream response leaked: %q", conversionError.Message)
			}
		})
	}
}

func TestConvertRejectsUnexpectedOrOversizedOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		contentType string
		body        []byte
		wantCode    string
	}{
		{name: "non pdf type", contentType: "text/plain", body: []byte("%PDF-1.7\n%%EOF"), wantCode: "DOCUMENT_VISION_CONVERSION_OUTPUT_INVALID"},
		{name: "invalid pdf", contentType: "application/pdf", body: []byte("not a pdf"), wantCode: "DOCUMENT_VISION_CONVERSION_OUTPUT_INVALID"},
		{name: "too large", contentType: "application/pdf", body: []byte("%PDF-1.7\n0123456789\n%%EOF"), wantCode: "DOCUMENT_VISION_CONVERSION_OUTPUT_TOO_LARGE"},
	}
	for _, item := range tests {
		item := item
		t.Run(item.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", item.contentType)
				_, _ = w.Write(item.body)
			}))
			defer server.Close()
			maxBytes := int64(1024)
			if item.name == "too large" {
				maxBytes = 16
			}
			client := Client{BaseURL: server.URL, AllowInsecureHTTP: true, MaxPDFBytes: maxBytes}
			_, err := client.Convert(context.Background(), knowledge.DocumentVisionInputConversionRequest{
				MIMEType: knowledge.PowerPointOpenXMLMIME, SizeBytes: 1, Source: strings.NewReader("x"),
			})
			conversionError, ok := knowledge.AsDocumentVisionInputConversionError(err)
			if !ok || conversionError.Code != item.wantCode || conversionError.Retryable {
				t.Fatalf("error = %#v, ok=%v", conversionError, ok)
			}
		})
	}
}

func TestInspectRequiresExplicitHTTPOptInAndRejectsCredentialURLs(t *testing.T) {
	t.Parallel()
	for _, baseURL := range []string{"http://gotenberg:3000", "https://user:secret@gotenberg.example"} {
		client := Client{BaseURL: baseURL}
		capability, err := client.Inspect(context.Background())
		if err == nil || capability.Available || capability.ReasonCode != "DOCUMENT_VISION_CONVERTER_DISABLED" {
			t.Fatalf("Inspect(%q) = %#v, %v", baseURL, capability, err)
		}
	}
}

func TestPresentationExtensionPreservesLegacyPowerPointFormat(t *testing.T) {
	t.Parallel()
	if !presentationMIME(knowledge.PowerPointLegacyMIME) || presentationExtension(knowledge.PowerPointLegacyMIME) != ".ppt" {
		t.Fatal("legacy PowerPoint must be uploaded with its format-preserving extension")
	}
	if presentationExtension(knowledge.PowerPointOpenXMLMIME) != ".pptx" {
		t.Fatal("OpenXML PowerPoint must use the .pptx extension")
	}
}
