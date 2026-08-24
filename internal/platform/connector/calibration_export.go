package connector

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const CalibrationExportPolicyVersion = "connector-calibration-export/v1"

// CalibrationExportRef creates one dataset-safe stable reference. The caller
// owns the export key and must not persist it with the exported cases.
func CalibrationExportRef(exportKey []byte, objectKind, connectorRef string) (string, error) {
	objectKind = strings.TrimSpace(objectKind)
	connectorRef = strings.TrimSpace(connectorRef)
	if len(exportKey) != 32 || objectKind == "" || connectorRef == "" || !strings.HasPrefix(connectorRef, "ref_") {
		return "", fmt.Errorf("%w: calibration export reference input is invalid", ErrInvalidFact)
	}
	mac := hmac.New(sha256.New, exportKey)
	_, _ = mac.Write([]byte(SourceSystem + "\x00" + objectKind + "\x00" + connectorRef))
	return "anon_v1_" + hex.EncodeToString(mac.Sum(nil)), nil
}
