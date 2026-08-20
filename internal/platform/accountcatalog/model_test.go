package accountcatalog

import "testing"

func TestOceanEngineWebAPIAccountAndConnection(t *testing.T) {
	account := Account{ID: "acct-1", OrganizationID: "org-1", Platform: PlatformOceanEngine, ExternalID: "900719925474099312345", Status: StatusPending}
	if !account.Valid() {
		t.Fatal("expected account to be valid")
	}
	connection := Connection{ID: "conn-1", AccountID: account.ID, OrganizationID: account.OrganizationID, ProjectID: "project-1", ConnectionType: ConnectionWebAPI, CredentialRef: "secret-ref", Status: StatusPending}
	if !connection.Valid() {
		t.Fatal("expected connection to be valid")
	}
}

func TestAccountCatalogRejectsRPAConnection(t *testing.T) {
	connection := Connection{ID: "conn-1", AccountID: "acct-1", OrganizationID: "org-1", ProjectID: "project-1", ConnectionType: "computer_use", CredentialRef: "secret-ref", Status: StatusPending}
	if connection.Valid() {
		t.Fatal("computer_use must not be an Ocean Engine connector connection")
	}
}
