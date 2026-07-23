package contract

import (
	"fmt"
	"strings"
)

// ResourceRef is an immutable reference to a resource owned by another module.
// It is deliberately small: callers must fetch complete resources through an
// authorized module API instead of duplicating mutable fields across systems.
type ResourceRef struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Version *int64 `json:"version,omitempty"`
}

func (r ResourceRef) Validate() error {
	if strings.TrimSpace(r.Type) == "" || strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("resource type and ID are required")
	}
	if r.Version != nil && *r.Version < 1 {
		return fmt.Errorf("resource version must be positive when supplied")
	}
	return nil
}
