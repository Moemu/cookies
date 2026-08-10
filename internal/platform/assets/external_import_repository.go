package assets

import (
	"context"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// ExternalImportRepository is deliberately separate from Repository so that an
// import is always anchored by its own durable idempotency ledger.
type ExternalImportRepository interface {
	CreateExternalImport(context.Context, ExternalImport) (ExternalImport, bool, error)
	GetExternalImport(context.Context, contract.OrganizationID, contract.ProjectID, string) (ExternalImport, error)
	GetExternalImportBySource(context.Context, contract.OrganizationID, contract.ProjectID, string, string) (ExternalImport, error)
	MarkExternalImportRunning(context.Context, contract.OrganizationID, contract.ProjectID, string, time.Time) error
	CompleteExternalImport(context.Context, string, AssetCommit, time.Time) (contract.ProjectAssetRef, error)
	FailExternalImport(context.Context, contract.OrganizationID, contract.ProjectID, string, string, string, time.Time) error
	MarkExternalImportResultUnknown(context.Context, contract.OrganizationID, contract.ProjectID, string, string, time.Time) error
	FindProjectAssetBySHA256(context.Context, contract.OrganizationID, contract.ProjectID, string) (contract.ProjectAssetRef, error)
}
