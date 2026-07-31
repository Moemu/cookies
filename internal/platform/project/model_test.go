package project

import (
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestCreateProjectRequestValidatesIndustry(t *testing.T) {
	valid := CreateProjectRequest{Name: "Demo", Industry: IndustryGame, ProductIDs: []contract.ProductID{}}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid industry rejected: %v", err)
	}
	invalid := CreateProjectRequest{Name: "Demo", Industry: "finance", ProductIDs: []contract.ProductID{}}
	if err := invalid.Validate(); err == nil {
		t.Fatal("invalid industry accepted")
	}
}

func TestProjectRolesEnforceActions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind    contract.PrincipalKind
		role    string
		action  string
		allowed bool
	}{
		{contract.PrincipalUser, "owner", "manage", true},
		{contract.PrincipalUser, "editor", "write", true},
		{contract.PrincipalUser, "editor", "approve", false},
		{contract.PrincipalUser, "viewer", "read", true},
		{contract.PrincipalUser, "viewer", "write", false},
		{contract.PrincipalService, "worker", "write", true},
		{contract.PrincipalService, "worker", "manage", false},
	}
	for _, test := range tests {
		if got := projectRoleAllowsAction(test.kind, test.role, test.action); got != test.allowed {
			t.Fatalf("%s/%s/%s allowed=%v, want %v", test.kind, test.role, test.action, got, test.allowed)
		}
	}
}

func TestProjectRolesStayCompatibleWithPrincipalKind(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind  contract.PrincipalKind
		role  string
		valid bool
	}{
		{contract.PrincipalUser, "owner", true},
		{contract.PrincipalUser, "editor", true},
		{contract.PrincipalUser, "viewer", true},
		{contract.PrincipalUser, "worker", false},
		{contract.PrincipalService, "worker", true},
		{contract.PrincipalService, "owner", false},
		{contract.PrincipalKind("unknown"), "viewer", false},
	}
	for _, test := range tests {
		if got := ValidProjectRoleForPrincipal(test.kind, test.role); got != test.valid {
			t.Fatalf("%s/%s valid=%v, want %v", test.kind, test.role, got, test.valid)
		}
	}
}
