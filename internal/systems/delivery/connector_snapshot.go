package delivery

import "github.com/shikanon/cookies/internal/platform/connector"

// ConnectorSnapshotReader is Delivery's read-only Connector boundary.
// It does not expose session or raw-evidence repositories.
type ConnectorSnapshotReader interface {
	Snapshot(connector.Query) (connector.CanonicalSnapshot, error)
}
