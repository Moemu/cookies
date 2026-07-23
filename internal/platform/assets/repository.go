package assets

import (
	"context"
	"errors"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var ErrNotFound = errors.New("asset resource not found")
var ErrIdempotencyConflict = errors.New("idempotency key conflicts with an existing request")
var ErrInvalidState = errors.New("asset resource is in an invalid state")
var ErrInvalidAssetContent = errors.New("asset content is invalid")
var ErrOutputMetadataMismatch = errors.New("output metadata mismatch")
var ErrAssetChecksumMismatch = errors.New("asset checksum mismatch")
var ErrAssetNotReady = errors.New("asset is not ready")
var ErrProviderOutputExpired = errors.New("provider output has expired")
var ErrUnsupportedAsset = errors.New("asset type or size is unsupported")
var ErrProjectContextStale = errors.New("project context version is stale")

type Repository interface {
	CreateUpload(context.Context, UploadSession) (UploadSession, bool, error)
	GetUpload(context.Context, contract.OrganizationID, contract.ProjectID, string) (UploadSession, error)
	MarkUploadUploaded(context.Context, contract.OrganizationID, contract.ProjectID, string, time.Time) error
	MarkUploadProcessing(context.Context, contract.OrganizationID, contract.ProjectID, string, time.Time) error
	CompleteUpload(context.Context, string, AssetCommit, time.Time) (contract.ProjectAssetRef, error)
	FailUpload(context.Context, contract.OrganizationID, contract.ProjectID, string, string, time.Time) error

	CreateIntake(context.Context, GeneratedIntake) (GeneratedIntake, bool, error)
	GetIntake(context.Context, contract.OrganizationID, contract.ProjectID, string) (GeneratedIntake, error)
	ClaimIntake(context.Context, contract.ActorContext, string, time.Time) (GeneratedIntake, bool, error)
	CompleteIntake(context.Context, GeneratedIntake, AssetCommit, time.Time) (contract.ProjectAssetRef, error)
	RetryIntake(context.Context, GeneratedIntake, contract.JobError, time.Time) error
	FailIntake(context.Context, GeneratedIntake, contract.JobError, time.Time) error

	GetProjectAsset(context.Context, contract.OrganizationID, contract.ProjectID, contract.AssetVersionRef) (ProjectAsset, error)
	ListProjectAssets(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]ProjectAsset, error)
	RemoveProjectAsset(context.Context, contract.OrganizationID, contract.ProjectID, contract.AssetVersionRef) error
}
