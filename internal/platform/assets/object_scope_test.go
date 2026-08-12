package assets

import (
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestQuarantineKeyIncludesOrganizationAndProjectScope(t *testing.T) {
	key := quarantineKey(contract.OrganizationID("org_1"), contract.ProjectID("project_1"), "upload_1")
	if key != "quarantine/org_1/project_1/upload_1" {
		t.Fatalf("quarantineKey() = %q", key)
	}
	service := UploadService{QuarantineBucket: "cookies"}
	if err := service.validateQuarantineScope(UploadSession{
		ID: "upload_1", OrganizationID: "org_1", ProjectID: "project_1",
		Quarantine: ObjectLocation{Bucket: "cookies", Key: key},
	}); err != nil {
		t.Fatalf("validateQuarantineScope() error = %v", err)
	}
	if err := service.validateQuarantineScope(UploadSession{
		ID: "upload_1", OrganizationID: "org_1", ProjectID: "project_2",
		Quarantine: ObjectLocation{Bucket: "cookies", Key: key},
	}); err == nil {
		t.Fatal("cross-project quarantine key must be rejected")
	}
}
