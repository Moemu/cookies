package identity

import (
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type Organization struct {
	ID        contract.OrganizationID `json:"id"`
	Name      string                  `json:"name"`
	Status    string                  `json:"status"`
	CreatedAt time.Time               `json:"created_at"`
	UpdatedAt time.Time               `json:"updated_at"`
}

type User struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type OrganizationMembership struct {
	OrganizationID contract.OrganizationID `json:"organization_id"`
	UserID         string                  `json:"user_id"`
	Role           string                  `json:"role"`
	Status         string                  `json:"status"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}

type CurrentIdentity struct {
	Actor        contract.ActorContext   `json:"actor"`
	Organization Organization            `json:"organization"`
	User         *User                   `json:"user"`
	Membership   *OrganizationMembership `json:"membership"`
}

func (o Organization) Validate() error {
	if strings.TrimSpace(string(o.ID)) == "" || strings.TrimSpace(o.Name) == "" {
		return fmt.Errorf("organization ID and name are required")
	}
	if o.Status != "active" && o.Status != "suspended" && o.Status != "archived" {
		return fmt.Errorf("organization status is invalid")
	}
	return nil
}
