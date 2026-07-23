package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
)

// CanonicalJSONHash returns SHA-256 over RFC 8785 JSON Canonicalization Scheme
// bytes. It is the only request-hash algorithm used by cross-module writes.
func CanonicalJSONHash(value any) (string, error) {
	payload, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal request for canonical hash: %w", err)
	}
	canonical, err := jsoncanonicalizer.Transform(payload)
	if err != nil {
		return "", fmt.Errorf("canonicalize request: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}
