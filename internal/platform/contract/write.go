package contract

import (
	"fmt"
	"strings"
)

// IdempotencyKey identifies one client-intended write. Durable storage of the
// key, request hash, result, and expiry belongs to the later workflow/storage
// module; this type keeps the HTTP contract consistent from the first write.
type IdempotencyKey string

func (k IdempotencyKey) Validate() error {
	value := strings.TrimSpace(string(k))
	if value == "" || len(value) > 255 {
		return fmt.Errorf("idempotency key must be between 1 and 255 characters")
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '_' || character == '-') {
			return fmt.Errorf("idempotency key contains an unsupported character")
		}
	}
	return nil
}

// ExpectedVersion is supplied by a caller when updating a mutable resource.
// Immutable versions never accept updates.
type ExpectedVersion int64

func (v ExpectedVersion) Validate() error {
	if v < 1 {
		return fmt.Errorf("expected version must be positive")
	}
	return nil
}
