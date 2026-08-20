package accountcatalog

import "time"

const (
	PlatformOceanEngine = "ocean_engine"
	ConnectionWebAPI    = "web_api"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusVerified Status = "verified"
	StatusRevoked  Status = "revoked"
	StatusBlocked  Status = "blocked"
)

type Account struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Platform       string    `json:"platform"`
	ExternalID     string    `json:"external_id"`
	DisplayLabel   string    `json:"display_label"`
	Status         Status    `json:"status"`
	VerifiedAt     time.Time `json:"verified_at,omitempty"`
	LastCheckedAt  time.Time `json:"last_checked_at,omitempty"`
}

type Connection struct {
	ID             string    `json:"id"`
	AccountID      string    `json:"account_id"`
	OrganizationID string    `json:"organization_id"`
	ProjectID      string    `json:"project_id"`
	ConnectionType string    `json:"connection_type"`
	CredentialRef  string    `json:"credential_ref"`
	Status         Status    `json:"status"`
	LastVerifiedAt time.Time `json:"last_verified_at,omitempty"`
}

func (a Account) Valid() bool {
	return a.ID != "" && a.OrganizationID != "" && a.Platform == PlatformOceanEngine && a.ExternalID != "" && validStatus(a.Status)
}

func (c Connection) Valid() bool {
	return c.ID != "" && c.AccountID != "" && c.OrganizationID != "" && c.ProjectID != "" && c.ConnectionType == ConnectionWebAPI && c.CredentialRef != "" && validStatus(c.Status)
}

func validStatus(s Status) bool {
	switch s {
	case StatusPending, StatusVerified, StatusRevoked, StatusBlocked:
		return true
	default:
		return false
	}
}
