package knowledge

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const PowerPointOpenXMLMIME = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
const PowerPointLegacyMIME = "application/vnd.ms-powerpoint"

type DocumentVisionInputConversionCapability struct {
	Available     bool
	ConverterCode string
	Version       string
	ReasonCode    string
}

type DocumentVisionInputConversionRequest struct {
	OrganizationID contract.OrganizationID
	ProjectID      contract.ProjectID
	DocumentID     string
	AttemptID      string
	Filename       string
	MIMEType       string
	SizeBytes      int64
	Source         io.Reader
	TraceID        string
}

type DocumentVisionInputConversionResult struct {
	PDF           []byte
	ConverterCode string
	Version       string
}

type DocumentVisionInputConverter interface {
	Inspect(context.Context) (DocumentVisionInputConversionCapability, error)
	Convert(context.Context, DocumentVisionInputConversionRequest) (DocumentVisionInputConversionResult, error)
}

type DocumentVisionInputConversionError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e DocumentVisionInputConversionError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "document input conversion failed"
	}
	return message
}

func NewDocumentVisionInputConversionError(code, message string, retryable bool) error {
	code = strings.TrimSpace(code)
	if code == "" {
		code = "DOCUMENT_VISION_CONVERSION_FAILED"
	}
	return DocumentVisionInputConversionError{Code: code, Message: message, Retryable: retryable}
}

func AsDocumentVisionInputConversionError(err error) (DocumentVisionInputConversionError, bool) {
	if err == nil {
		return DocumentVisionInputConversionError{}, false
	}
	var value DocumentVisionInputConversionError
	if errors.As(err, &value) {
		return value, true
	}
	return DocumentVisionInputConversionError{}, false
}

func documentVisionNeedsConversion(mimeType string) bool {
	mimeType = strings.TrimSpace(mimeType)
	return strings.EqualFold(mimeType, PowerPointOpenXMLMIME) || strings.EqualFold(mimeType, PowerPointLegacyMIME)
}
