package delivery

import (
	"context"

	"github.com/shikanon/cookies/internal/platform/connector"
)

// ConnectorSnapshotReader is Delivery's read-only Connector boundary.
// It does not expose session or raw-evidence repositories.
type ConnectorSnapshotReader interface {
	Snapshot(context.Context, connector.Query) (connector.CanonicalSnapshot, error)
}

// ConnectorLaunchBatchCalibrationReader exposes only safe, compact model priors.
// It does not expose report rows, platform IDs, or session data.
type ConnectorLaunchBatchCalibrationReader interface {
	LatestLaunchBatchCalibration(context.Context, string, string) (connector.LaunchBatchCalibrationSnapshot, error)
}
