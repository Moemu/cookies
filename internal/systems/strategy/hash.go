package strategy

import (
	"encoding/json"
	"fmt"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func PackageContentHash(snapshot PackageSnapshot) (contract.ContentHash, error) {
	snapshot.Approval.ContentHash = ""
	// Readiness is derived when a BriefVersion is read. It is operational UI
	// metadata, not part of the immutable Brief or Strategy package identity.
	snapshot.Brief.FullStrategyReadiness = nil
	return contract.NewContentHash(snapshot)
}

func VerifyPackageContentHash(snapshot PackageSnapshot) error {
	stored := snapshot.Approval.ContentHash
	if err := stored.Validate(); err != nil {
		return err
	}
	calculated, err := PackageContentHash(snapshot)
	if err != nil {
		return err
	}
	if !stored.Equal(calculated) {
		return fmt.Errorf("strategy package content hash does not match its snapshot")
	}
	return nil
}

func snapshotJSON(value any) (json.RawMessage, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(encoded) > 1<<20 {
		return nil, fmt.Errorf("%w: snapshot exceeds 1 MiB", ErrInvalidRequest)
	}
	return encoded, nil
}
