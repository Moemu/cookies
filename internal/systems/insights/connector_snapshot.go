package insights

import (
	"context"

	"github.com/shikanon/cookies/internal/platform/connector"
)

// ConnectorSnapshotReader is the versioned compatibility boundary for new
// Ocean Engine facts. The existing session route remains available during
// migration, but new Insights code must not use it for data reads.
type ConnectorSnapshotReader interface {
	Snapshot(context.Context, connector.Query) (connector.CanonicalSnapshot, error)
}
