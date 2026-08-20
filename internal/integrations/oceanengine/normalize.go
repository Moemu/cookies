package oceanengine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// HashCanonicalJSON creates a stable evidence hash without exposing source values.
func HashCanonicalJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("marshal evidence: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

// HashName is used for searchable labels without storing platform names in canonical records.
func HashName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(name))
	return hex.EncodeToString(sum[:])
}

// NormalizeQuality returns a conservative quality state for a replay payload.
func NormalizeQuality(payload map[string]any, mapped bool) QualityStatus {
	if !mapped {
		return QualityIncomplete
	}
	if status, ok := payload["quality"].(string); ok {
		switch QualityStatus(status) {
		case QualityPartial, QualityDelayed, QualityBlocked, QualityIncomplete:
			return QualityStatus(status)
		}
	}
	return QualityHealthy
}
