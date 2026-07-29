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
