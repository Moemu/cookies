package provider

import (
	"strings"
	"testing"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestProviderOutputObjectKeyIncludesOrganizationAndProjectScope(t *testing.T) {
	key := providerOutputObjectKey(contract.ProjectRef{
		OrganizationID: "org_1", ProjectID: "project_1",
	}, contract.ProviderOutputRef{ProviderJobID: "job_1", OutputID: "output_1"})
	if !strings.HasPrefix(key, "provider-output/org_1/project_1/") {
		t.Fatalf("providerOutputObjectKey() = %q", key)
	}
	other := providerOutputObjectKey(contract.ProjectRef{
		OrganizationID: "org_1", ProjectID: "project_2",
	}, contract.ProviderOutputRef{ProviderJobID: "job_1", OutputID: "output_1"})
	if key == other {
		t.Fatal("different projects must not share provider-output object keys")
	}
}

func TestProviderOutputLocationScopeRequiresExactBucketProjectAndDigest(t *testing.T) {
	project := contract.ProjectRef{OrganizationID: "org_1", ProjectID: "project_1"}
	key := providerOutputObjectKey(project, contract.ProviderOutputRef{ProviderJobID: "job_1", OutputID: "output_1"})
	valid := assets.ObjectLocation{Bucket: "cookies", Key: key}
	if !providerOutputLocationInScope(valid, "cookies", project, "job_1", "output_1") {
		t.Fatal("valid Provider output location rejected")
	}
	for _, location := range []assets.ObjectLocation{
		{Bucket: "other", Key: key},
		{Bucket: "cookies", Key: "provider-output/org_1/project_2/forged"},
		{Bucket: "cookies", Key: key + "-other"},
	} {
		if providerOutputLocationInScope(location, "cookies", project, "job_1", "output_1") {
			t.Fatalf("out-of-scope Provider output accepted: %#v", location)
		}
	}
}
