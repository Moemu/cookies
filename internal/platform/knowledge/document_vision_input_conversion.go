package knowledge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
)

const maxDocumentVisionDerivedPDFBytes = int64(64 * 1024 * 1024)

type documentVisionInputConversion struct {
	AttemptID        string
	Status           string
	AttemptCount     int
	SourceMIMEType   string
	SourceSHA256     string
	Source           assets.ObjectLocation
	ConverterCode    string
	ConverterVersion string
	Derived          assets.ObjectLocation
	DerivedSHA256    string
	DerivedSize      *int64
	ErrorCode        string
	ErrorMessage     string
}

type documentVisionResolvedInput struct {
	Filename string
	MIMEType string
	Location assets.ObjectLocation
}

func insertDocumentVisionInputConversion(
	ctx context.Context,
	tx *sql.Tx,
	document Document,
	attemptID string,
	capability DocumentVisionInputConversionCapability,
	bucket string,
	now time.Time,
) error {
	if tx == nil || !knowledgeDocumentBlobInScope(document, bucket) ||
		strings.TrimSpace(capability.ConverterCode) == "" || strings.TrimSpace(capability.Version) == "" {
		return fmt.Errorf("document vision input conversion scope is invalid")
	}
	key := documentVisionDerivedPDFKey(document, attemptID)
	_, err := tx.ExecContext(ctx, `INSERT INTO platform_knowledge_document_vision_input_conversions
		(organization_id, project_id, document_id, attempt_id, status,
		 source_mime_type, source_sha256, source_provider, source_bucket, source_object_key,
		 source_version_id, source_etag, converter_code, converter_version,
		 derived_bucket, derived_object_key, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'prepared', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		document.OrganizationID, document.ProjectID, document.ID, attemptID,
		document.MIMEType, document.ContentSHA256, document.Blob.Provider, document.Blob.Bucket,
		document.Blob.Key, document.Blob.VersionID, document.Blob.ETag,
		capability.ConverterCode, capability.Version, bucket, key, now, now,
	)
	return err
}

func documentVisionDerivedPDFKey(document Document, attemptID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(document.ContentSHA256) + "|" + strings.TrimSpace(attemptID)))
	encoded := hex.EncodeToString(digest[:])
	return knowledgeDocumentObjectPrefix(document.OrganizationID, document.ProjectID, document.ID) +
		fmt.Sprintf("derived/document-vision/%s/%s.pdf", encoded[:2], encoded)
}

func (s Service) resolveDocumentVisionInput(
	ctx context.Context,
	document Document,
	attemptID string,
	traceID string,
) (documentVisionResolvedInput, error) {
	if !documentVisionNeedsConversion(document.MIMEType) {
		return documentVisionResolvedInput{Filename: document.Filename, MIMEType: document.MIMEType, Location: document.Blob}, nil
	}
	if s.DocumentConverter == nil || s.Blobs == nil {
		return documentVisionResolvedInput{}, NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERTER_DISABLED", "document converter is unavailable", false,
		)
	}
	conversion, err := s.loadDocumentVisionInputConversion(ctx, document, attemptID)
	if err != nil {
		return documentVisionResolvedInput{}, err
	}
	if err := validateDocumentVisionInputConversion(document, conversion, s.AssetsBucket); err != nil {
		return documentVisionResolvedInput{}, NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_LINEAGE_INVALID", "document conversion lineage is invalid", false,
		)
	}
	switch conversion.Status {
	case "ready":
		if err := s.validateReadyDocumentVisionInput(ctx, conversion); err != nil {
			return documentVisionResolvedInput{}, err
		}
		return resolvedConvertedInput(conversion), nil
	case "failed":
		return documentVisionResolvedInput{}, NewDocumentVisionInputConversionError(
			conversion.ErrorCode, conversion.ErrorMessage, false,
		)
	case "prepared":
		claimed, err := s.claimDocumentVisionInputConversion(ctx, document, conversion)
		if err != nil {
			return documentVisionResolvedInput{}, err
		}
		if !claimed {
			return documentVisionResolvedInput{}, NewDocumentVisionInputConversionError(
				"DOCUMENT_VISION_CONVERSION_BUSY", "document conversion is already being prepared", true,
			)
		}
		conversion.Status = "converting"
	case "converting":
	default:
		return documentVisionResolvedInput{}, NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_STATE_INVALID", "document conversion state is invalid", false,
		)
	}
	conversionAttempt, err := s.beginDocumentVisionInputConversionAttempt(ctx, document, conversion)
	if err != nil {
		return documentVisionResolvedInput{}, err
	}

	stream, info, err := s.Blobs.Open(ctx, conversion.Source)
	if err != nil {
		return documentVisionResolvedInput{}, NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_SOURCE_UNAVAILABLE", "document conversion source is unavailable", true,
		)
	}
	if info.SizeBytes != document.SizeBytes || !documentVisionObjectIdentityMatches(info.ObjectLocation, conversion.Source) {
		_ = stream.Close()
		return documentVisionResolvedInput{}, NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_LINEAGE_INVALID", "document conversion source no longer matches its checkpoint", false,
		)
	}
	result, convertErr := s.DocumentConverter.Convert(ctx, DocumentVisionInputConversionRequest{
		OrganizationID: document.OrganizationID, ProjectID: document.ProjectID,
		DocumentID: document.ID, AttemptID: attemptID, Filename: document.Filename,
		MIMEType: conversion.SourceMIMEType, SizeBytes: info.SizeBytes, Source: stream, TraceID: traceID,
	})
	_ = stream.Close()
	if convertErr != nil {
		if conversionError, ok := AsDocumentVisionInputConversionError(convertErr); ok && conversionError.Retryable && conversionAttempt >= 3 {
			return documentVisionResolvedInput{}, NewDocumentVisionInputConversionError(
				"DOCUMENT_VISION_CONVERSION_RETRY_EXHAUSTED", "document conversion did not recover after three attempts", false,
			)
		}
		return documentVisionResolvedInput{}, convertErr
	}
	if result.ConverterCode != conversion.ConverterCode || result.Version != conversion.ConverterVersion || !validConvertedPDF(result.PDF) {
		return documentVisionResolvedInput{}, NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_OUTPUT_INVALID", "document converter returned an invalid or unexpected result", false,
		)
	}
	stored, err := s.Blobs.Put(
		ctx, conversion.Derived.Bucket, conversion.Derived.Key, bytes.NewReader(result.PDF), int64(len(result.PDF)), "application/pdf",
	)
	if err != nil {
		return documentVisionResolvedInput{}, NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_STORE_UNAVAILABLE", "converted PDF could not be stored", true,
		)
	}
	actual, actualInfo, err := readAndValidateConvertedPDF(ctx, s.Blobs, stored.ObjectLocation)
	if err != nil {
		return documentVisionResolvedInput{}, err
	}
	digest := sha256.Sum256(actual)
	conversion.Derived = actualInfo.ObjectLocation
	conversion.DerivedSHA256 = hex.EncodeToString(digest[:])
	size := int64(len(actual))
	conversion.DerivedSize = &size
	if err := s.persistReadyDocumentVisionInputConversion(ctx, document, conversion); err != nil {
		return documentVisionResolvedInput{}, err
	}
	conversion.Status = "ready"
	return resolvedConvertedInput(conversion), nil
}

func (s Service) loadDocumentVisionInputConversion(ctx context.Context, document Document, attemptID string) (documentVisionInputConversion, error) {
	var value documentVisionInputConversion
	var size sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `SELECT attempt_id, status, attempt_count, source_mime_type, source_sha256,
		source_provider, source_bucket, source_object_key, source_version_id, source_etag,
		converter_code, converter_version, derived_provider, derived_bucket, derived_object_key,
		derived_version_id, derived_etag, derived_sha256, derived_size_bytes, error_code, error_message
		FROM platform_knowledge_document_vision_input_conversions
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ?`,
		document.OrganizationID, document.ProjectID, document.ID, attemptID,
	).Scan(
		&value.AttemptID, &value.Status, &value.AttemptCount, &value.SourceMIMEType, &value.SourceSHA256,
		&value.Source.Provider, &value.Source.Bucket, &value.Source.Key, &value.Source.VersionID, &value.Source.ETag,
		&value.ConverterCode, &value.ConverterVersion, &value.Derived.Provider, &value.Derived.Bucket,
		&value.Derived.Key, &value.Derived.VersionID, &value.Derived.ETag, &value.DerivedSHA256,
		&size, &value.ErrorCode, &value.ErrorMessage,
	)
	if size.Valid {
		valueSize := size.Int64
		value.DerivedSize = &valueSize
	}
	return value, err
}

func (s Service) beginDocumentVisionInputConversionAttempt(
	ctx context.Context,
	document Document,
	conversion documentVisionInputConversion,
) (int, error) {
	now := s.now()
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_knowledge_document_vision_input_conversions
		SET attempt_count = attempt_count + 1, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ?
		  AND status = 'converting' AND attempt_count < 3`,
		now, document.OrganizationID, document.ProjectID, document.ID, conversion.AttemptID,
	)
	if err != nil {
		return 0, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if changed != 1 {
		return 0, NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_RETRY_EXHAUSTED", "document conversion retry budget is exhausted", false,
		)
	}
	return conversion.AttemptCount + 1, nil
}

func validateDocumentVisionInputConversion(document Document, conversion documentVisionInputConversion, bucket string) error {
	if conversion.AttemptID != document.VisionAttemptID || conversion.SourceMIMEType != document.MIMEType ||
		conversion.SourceSHA256 != document.ContentSHA256 || conversion.Source != document.Blob ||
		!knowledgeDocumentBlobInScope(document, bucket) || conversion.Derived.Bucket != bucket ||
		conversion.Derived.Key != documentVisionDerivedPDFKey(document, conversion.AttemptID) ||
		conversion.ConverterCode == "" || conversion.ConverterVersion == "" {
		return fmt.Errorf("document conversion identity does not match the source")
	}
	return nil
}

func (s Service) claimDocumentVisionInputConversion(ctx context.Context, document Document, conversion documentVisionInputConversion) (bool, error) {
	now := s.now()
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_knowledge_document_vision_input_conversions
		SET status = 'converting', started_at = COALESCE(started_at, ?), error_code = '', error_message = '', updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ? AND status = 'prepared'`,
		now, now, document.OrganizationID, document.ProjectID, document.ID, conversion.AttemptID,
	)
	if err != nil {
		return false, err
	}
	changed, err := result.RowsAffected()
	return changed == 1, err
}

func (s Service) persistReadyDocumentVisionInputConversion(ctx context.Context, document Document, conversion documentVisionInputConversion) error {
	if conversion.DerivedSize == nil {
		return fmt.Errorf("converted PDF size is missing")
	}
	now := s.now()
	result, err := s.DB.ExecContext(ctx, `UPDATE platform_knowledge_document_vision_input_conversions
		SET status = 'ready', derived_provider = ?, derived_version_id = ?, derived_etag = ?,
			derived_sha256 = ?, derived_size_bytes = ?, error_code = '', error_message = '',
			completed_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ? AND status = 'converting'`,
		conversion.Derived.Provider, conversion.Derived.VersionID, conversion.Derived.ETag,
		conversion.DerivedSHA256, *conversion.DerivedSize, now, now,
		document.OrganizationID, document.ProjectID, document.ID, conversion.AttemptID,
	)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("document conversion checkpoint lost its active state")
	}
	return nil
}

func (s Service) markDocumentVisionInputConversionFailed(ctx context.Context, document Document, attemptID, code, message string) {
	_, _ = s.DB.ExecContext(ctx, `UPDATE platform_knowledge_document_vision_input_conversions
		SET status = 'failed', error_code = ?, error_message = ?, completed_at = ?, updated_at = ?
		WHERE organization_id = ? AND project_id = ? AND document_id = ? AND attempt_id = ? AND status IN ('prepared', 'converting')`,
		strings.TrimSpace(code), boundedVisionMessage(message), s.now(), s.now(),
		document.OrganizationID, document.ProjectID, document.ID, attemptID,
	)
}

func (s Service) validateReadyDocumentVisionInput(ctx context.Context, conversion documentVisionInputConversion) error {
	if conversion.Derived.Provider == "" || conversion.DerivedSHA256 == "" || conversion.DerivedSize == nil {
		return NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_LINEAGE_INVALID", "converted PDF checkpoint is incomplete", false,
		)
	}
	info, err := s.Blobs.Head(ctx, conversion.Derived)
	if err != nil {
		return NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_STORE_UNAVAILABLE", "converted PDF is unavailable", true,
		)
	}
	if info.SizeBytes != *conversion.DerivedSize || !strings.EqualFold(info.MIMEType, "application/pdf") ||
		!documentVisionObjectIdentityMatches(info.ObjectLocation, conversion.Derived) {
		return NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_LINEAGE_INVALID", "converted PDF no longer matches its checkpoint", false,
		)
	}
	return nil
}

func documentVisionObjectIdentityMatches(actual, expected assets.ObjectLocation) bool {
	return actual.Provider == expected.Provider && actual.Bucket == expected.Bucket && actual.Key == expected.Key &&
		(expected.VersionID == "" || actual.VersionID == expected.VersionID) &&
		(expected.ETag == "" || actual.ETag == expected.ETag)
}

func readAndValidateConvertedPDF(ctx context.Context, blobs assets.BlobStore, location assets.ObjectLocation) ([]byte, assets.ObjectInfo, error) {
	stream, info, err := blobs.Open(ctx, location)
	if err != nil {
		return nil, assets.ObjectInfo{}, NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_STORE_UNAVAILABLE", "converted PDF could not be verified", true,
		)
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, maxDocumentVisionDerivedPDFBytes+1))
	if err != nil {
		return nil, assets.ObjectInfo{}, NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_STORE_UNAVAILABLE", "converted PDF could not be verified", true,
		)
	}
	if int64(len(data)) > maxDocumentVisionDerivedPDFBytes || int64(len(data)) != info.SizeBytes || !validConvertedPDF(data) {
		return nil, assets.ObjectInfo{}, NewDocumentVisionInputConversionError(
			"DOCUMENT_VISION_CONVERSION_OUTPUT_INVALID", "stored conversion result is not a valid PDF", false,
		)
	}
	return data, info, nil
}

func validConvertedPDF(data []byte) bool {
	return len(data) >= 8 && bytes.HasPrefix(data, []byte("%PDF-")) &&
		bytes.Contains(data[len(data)-min(len(data), 2048):], []byte("%%EOF"))
}

func resolvedConvertedInput(conversion documentVisionInputConversion) documentVisionResolvedInput {
	return documentVisionResolvedInput{Filename: "document-vision-input.pdf", MIMEType: "application/pdf", Location: conversion.Derived}
}
