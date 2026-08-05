package contract

import (
	"crypto/subtle"
	"fmt"
	"strings"
)

const contentHashPrefix = "sha256:"

// ContentHash is a cross-module SHA-256 digest formatted as
// "sha256:<64 lowercase hexadecimal characters>".
type ContentHash string

func NewContentHash(value any) (ContentHash, error) {
	digest, err := CanonicalJSONHash(value)
	if err != nil {
		return "", err
	}
	return ContentHash(contentHashPrefix + digest), nil
}

func ParseContentHash(value string) (ContentHash, error) {
	hash := ContentHash(strings.TrimSpace(value))
	if err := hash.Validate(); err != nil {
		return "", err
	}
	return hash, nil
}

func (h ContentHash) Validate() error {
	value := string(h)
	if len(value) != len(contentHashPrefix)+64 || !strings.HasPrefix(value, contentHashPrefix) {
		return fmt.Errorf("content hash must use sha256:<64 lowercase hex> format")
	}
	for _, character := range value[len(contentHashPrefix):] {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return fmt.Errorf("content hash digest must be lowercase hexadecimal")
		}
	}
	return nil
}

func (h ContentHash) Equal(other ContentHash) bool {
	if h.Validate() != nil || other.Validate() != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(h), []byte(other)) == 1
}
