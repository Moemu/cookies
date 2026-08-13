package gotenberg

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/knowledge"
)

const (
	converterCode       = "gotenberg_libreoffice"
	defaultVersion      = "gotenberg-8"
	defaultMaxPDFBytes  = int64(32 * 1024 * 1024)
	maxErrorBodyBytes   = int64(4 * 1024)
	convertEndpointPath = "/forms/libreoffice/convert"
)

type Client struct {
	BaseURL           string
	Version           string
	Timeout           time.Duration
	MaxPDFBytes       int64
	AllowInsecureHTTP bool
	HTTPClient        *http.Client
}

func (c Client) Inspect(_ context.Context) (knowledge.DocumentVisionInputConversionCapability, error) {
	if _, err := c.endpoint(); err != nil {
		return knowledge.DocumentVisionInputConversionCapability{
			ConverterCode: converterCode, Version: c.version(), ReasonCode: "DOCUMENT_VISION_CONVERTER_DISABLED",
		}, err
	}
	return knowledge.DocumentVisionInputConversionCapability{
		Available: true, ConverterCode: converterCode, Version: c.version(),
	}, nil
}

func (c Client) Convert(ctx context.Context, input knowledge.DocumentVisionInputConversionRequest) (knowledge.DocumentVisionInputConversionResult, error) {
	endpoint, err := c.endpoint()
	if err != nil {
		return knowledge.DocumentVisionInputConversionResult{}, knowledge.NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERTER_DISABLED", "document converter is not configured", false,
		)
	}
	if input.Source == nil || input.SizeBytes < 1 || input.SizeBytes > knowledge.MaxDocumentBytes || !presentationMIME(input.MIMEType) {
		return knowledge.DocumentVisionInputConversionResult{}, knowledge.NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_INPUT_INVALID", "document conversion input is invalid", false,
		)
	}

	reader, contentType, err := multipartBody(input.Source, input.SizeBytes, presentationExtension(input.MIMEType))
	if err != nil {
		return knowledge.DocumentVisionInputConversionResult{}, knowledge.NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_SOURCE_UNAVAILABLE", "document conversion source could not be read", true,
		)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), reader)
	if err != nil {
		return knowledge.DocumentVisionInputConversionResult{}, err
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Accept", "application/pdf")
	request.Header.Set("Gotenberg-Output-Filename", "document-vision-input")
	if trace := safeTraceID(input.TraceID); trace != "" {
		request.Header.Set("Gotenberg-Trace", trace)
	}

	response, err := c.httpClient().Do(request)
	if err != nil {
		return knowledge.DocumentVisionInputConversionResult{}, knowledge.NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERTER_UNAVAILABLE", "document converter request failed", true,
		)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxErrorBodyBytes))
		retryable := response.StatusCode >= 500 || response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusRequestTimeout
		code := "DOCUMENT_VISION_CONVERSION_REJECTED"
		if retryable {
			code = "DOCUMENT_VISION_CONVERTER_UNAVAILABLE"
		}
		return knowledge.DocumentVisionInputConversionResult{}, knowledge.NewDocumentVisionInputConversionError(
			code, fmt.Sprintf("document converter returned HTTP %d", response.StatusCode), retryable,
		)
	}
	mediaType, _, _ := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if !strings.EqualFold(mediaType, "application/pdf") {
		return knowledge.DocumentVisionInputConversionResult{}, knowledge.NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_OUTPUT_INVALID", "document converter returned a non-PDF response", false,
		)
	}
	limit := c.maxPDFBytes()
	data, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return knowledge.DocumentVisionInputConversionResult{}, knowledge.NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERTER_UNAVAILABLE", "document converter response could not be read", true,
		)
	}
	if int64(len(data)) > limit {
		return knowledge.DocumentVisionInputConversionResult{}, knowledge.NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_OUTPUT_TOO_LARGE", "converted PDF exceeds the configured size limit", false,
		)
	}
	if !validPDF(data) {
		return knowledge.DocumentVisionInputConversionResult{}, knowledge.NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_OUTPUT_INVALID", "document converter returned an invalid PDF", false,
		)
	}
	return knowledge.DocumentVisionInputConversionResult{PDF: data, ConverterCode: converterCode, Version: c.version()}, nil
}

func multipartBody(source io.Reader, expectedSize int64, extension string) (io.Reader, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("files", "source"+extension)
	if err != nil {
		return nil, "", err
	}
	written, err := io.Copy(part, io.LimitReader(source, expectedSize+1))
	if err != nil {
		return nil, "", err
	}
	if written != expectedSize {
		return nil, "", fmt.Errorf("document source size does not match the declared size")
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &body, writer.FormDataContentType(), nil
}

func presentationMIME(value string) bool {
	value = strings.TrimSpace(value)
	return strings.EqualFold(value, knowledge.PowerPointOpenXMLMIME) || strings.EqualFold(value, knowledge.PowerPointLegacyMIME)
}

func presentationExtension(mimeType string) string {
	if strings.EqualFold(strings.TrimSpace(mimeType), knowledge.PowerPointLegacyMIME) {
		return ".ppt"
	}
	return ".pptx"
}

func (c Client) endpoint() (*url.URL, error) {
	base, err := url.Parse(strings.TrimSpace(c.BaseURL))
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return nil, fmt.Errorf("Gotenberg base URL must be an absolute HTTP(S) URL without credentials, query, or fragment")
	}
	if base.Scheme == "http" && !c.AllowInsecureHTTP {
		return nil, fmt.Errorf("Gotenberg HTTP requires explicit insecure transport opt-in")
	}
	base.Path = strings.TrimRight(base.Path, "/") + convertEndpointPath
	return base, nil
}

func (c Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	return &http.Client{Timeout: timeout}
}

func (c Client) maxPDFBytes() int64 {
	if c.MaxPDFBytes > 0 {
		return c.MaxPDFBytes
	}
	return defaultMaxPDFBytes
}

func (c Client) version() string {
	if value := strings.TrimSpace(c.Version); value != "" {
		return value
	}
	return defaultVersion
}

func validPDF(data []byte) bool {
	return len(data) >= 8 && bytes.HasPrefix(data, []byte("%PDF-")) && bytes.Contains(data[len(data)-min(len(data), 2048):], []byte("%%EOF"))
}

func safeTraceID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 96 {
		value = value[:96]
	}
	return strings.Map(func(character rune) rune {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' || strings.ContainsRune("-_./", character) {
			return character
		}
		return -1
	}, value)
}
