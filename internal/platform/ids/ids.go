// Package ids creates opaque identifiers for platform-owned resources.
package ids

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

type Generator func(prefix string) (string, error)

func New(prefix string) (string, error) {
	if !validPrefix(prefix) {
		return "", fmt.Errorf("ID prefix must contain only lowercase ASCII letters and digits")
	}
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate random ID: %w", err)
	}
	return prefix + "_" + hex.EncodeToString(bytes), nil
}

func validPrefix(prefix string) bool {
	if strings.TrimSpace(prefix) == "" || len(prefix) > 24 {
		return false
	}
	for _, character := range prefix {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}
