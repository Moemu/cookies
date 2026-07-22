package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var ErrOutputHandleNotFound = errors.New("provider output handle not found")

// OutputHandleStore retains Provider-owned, short-lived result bytes. It is
// intentionally separate from Assets' blob storage: the only cross-module
// handoff remains Assets.GeneratedOutputFetcher.Open.
type OutputHandleStore interface {
	Put(context.Context, contract.ProjectRef, contract.ProviderOutputRef, []byte) error
	Open(context.Context, contract.ProjectRef, contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error)
	Delete(context.Context, contract.OrganizationID, contract.ProjectID, string, string) error
}

// MySQLOutputHandleStore persists inline bytes only until Generated Intake has
// made a durable asset. It stores no vendor URL, object key, or public handle.
type MySQLOutputHandleStore struct{ DB *sql.DB }

func (s MySQLOutputHandleStore) Put(ctx context.Context, project contract.ProjectRef, ref contract.ProviderOutputRef, contents []byte) error {
	if s.DB == nil {
		return fmt.Errorf("MySQL database is required")
	}
	if err := project.Validate(); err != nil {
		return err
	}
	if err := ref.Validate(); err != nil {
		return err
	}
	if len(contents) == 0 || int64(len(contents)) != ref.DeclaredSizeBytes {
		return fmt.Errorf("provider output contents do not match declared size")
	}
	digest := sha256.Sum256(contents)
	actualSHA := hex.EncodeToString(digest[:])
	if ref.DeclaredSHA256 == nil || *ref.DeclaredSHA256 != actualSHA {
		return fmt.Errorf("provider output contents do not match declared SHA-256")
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO provider_job_output_handles (
		provider_job_id, output_id, organization_id, project_id, provider_code,
		retrieval_expires_at, mime_type, size_bytes, sha256, contents
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON DUPLICATE KEY UPDATE organization_id = VALUES(organization_id), project_id = VALUES(project_id),
		provider_code = VALUES(provider_code), retrieval_expires_at = VALUES(retrieval_expires_at),
		mime_type = VALUES(mime_type), size_bytes = VALUES(size_bytes), sha256 = VALUES(sha256), contents = VALUES(contents)`,
		ref.ProviderJobID, ref.OutputID, project.OrganizationID, project.ProjectID, ref.ProviderCode,
		ref.RetrievalExpiresAt, ref.DeclaredMIMEType, ref.DeclaredSizeBytes, actualSHA, contents)
	return err
}

// Open implements Assets' GeneratedOutputFetcher seam. Every lookup is scoped
// by organization, project, provider job, provider code and output ID.
func (s MySQLOutputHandleStore) Open(ctx context.Context, project contract.ProjectRef, ref contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error) {
	if s.DB == nil {
		return nil, contract.OutputMetadata{}, fmt.Errorf("MySQL database is required")
	}
	if err := project.Validate(); err != nil {
		return nil, contract.OutputMetadata{}, err
	}
	if err := ref.Validate(); err != nil {
		return nil, contract.OutputMetadata{}, err
	}
	var contents []byte
	var mimeType, sha string
	var size int64
	err := s.DB.QueryRowContext(ctx, `SELECT contents, mime_type, size_bytes, sha256
		FROM provider_job_output_handles
		WHERE provider_job_id = ? AND output_id = ? AND organization_id = ? AND project_id = ?
			AND provider_code = ? AND retrieval_expires_at > UTC_TIMESTAMP(6)`,
		ref.ProviderJobID, ref.OutputID, project.OrganizationID, project.ProjectID, ref.ProviderCode).Scan(&contents, &mimeType, &size, &sha)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, contract.OutputMetadata{}, ErrOutputHandleNotFound
	}
	if err != nil {
		return nil, contract.OutputMetadata{}, err
	}
	if mimeType != ref.DeclaredMIMEType || size != ref.DeclaredSizeBytes || (ref.DeclaredSHA256 != nil && sha != *ref.DeclaredSHA256) || int64(len(contents)) != size {
		return nil, contract.OutputMetadata{}, fmt.Errorf("provider output handle metadata mismatch")
	}
	return io.NopCloser(bytes.NewReader(contents)), contract.OutputMetadata{MIMEType: mimeType, SizeBytes: size, SHA256: sha}, nil
}

func (s MySQLOutputHandleStore) Delete(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, providerJobID, outputID string) error {
	if s.DB == nil {
		return fmt.Errorf("MySQL database is required")
	}
	if strings.TrimSpace(string(organizationID)) == "" || strings.TrimSpace(string(projectID)) == "" || strings.TrimSpace(providerJobID) == "" || strings.TrimSpace(outputID) == "" {
		return fmt.Errorf("organization ID, project ID, provider job ID, and output ID are required")
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM provider_job_output_handles WHERE provider_job_id = ? AND output_id = ? AND organization_id = ? AND project_id = ?`, providerJobID, outputID, organizationID, projectID)
	return err
}

// NewOutputRef creates the public, opaque reference after the complete bytes
// and their metadata have been verified inside Provider.
func NewOutputRef(providerCode, providerJobID, outputID, mimeType string, contents []byte, expiresAt time.Time) (contract.ProviderOutputRef, error) {
	if !strings.HasPrefix(mimeType, "image/") || len(contents) == 0 {
		return contract.ProviderOutputRef{}, fmt.Errorf("image contents and MIME type are required")
	}
	digest := sha256.Sum256(contents)
	sha := hex.EncodeToString(digest[:])
	ref := contract.ProviderOutputRef{ProviderCode: providerCode, ProviderJobID: providerJobID, OutputID: outputID, RetrievalExpiresAt: expiresAt.UTC(), DeclaredMIMEType: mimeType, DeclaredSizeBytes: int64(len(contents)), DeclaredSHA256: &sha}
	if err := ref.Validate(); err != nil {
		return contract.ProviderOutputRef{}, err
	}
	return ref, nil
}
