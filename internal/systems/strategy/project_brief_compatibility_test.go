package strategy

import (
	"context"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type compatibilityProjectReader struct {
	business contract.ProjectBusinessContext
}

func (reader compatibilityProjectReader) GetContext(
	context.Context,
	contract.ActorContext,
	contract.ProjectID,
) (contract.ProjectContext, error) {
	brandID := contract.BrandID("brand_guerlain")
	return contract.ProjectContext{
		OrganizationID: "org_1", ProjectID: "project_guerlain", BrandID: &brandID,
		ProductIDs: []contract.ProductID{"product_abeille_royale"}, ProjectContextVersion: 1,
	}, nil
}

func (reader compatibilityProjectReader) GetBusinessContext(
	context.Context,
	contract.ActorContext,
	contract.ProjectID,
) (contract.ProjectBusinessContext, error) {
	return reader.business, nil
}

func TestProjectBriefCompatibilityRejectsCrossBusinessContext(t *testing.T) {
	t.Parallel()
	brandID := contract.BrandID("brand_baiyu_precision")
	service := Service{Projects: compatibilityProjectReader{business: contract.ProjectBusinessContext{
		ProjectID: "project_investor_precision_evidence", ProjectName: "投资人路演：精度证据增长",
		BrandID: &brandID, BrandName: "白羽精工",
		Products: []contract.ProjectBusinessProduct{{ID: "product_precision_cnc_parts", Name: "精密 CNC 零部件"}},
	}}}
	document := EmptyBriefDocumentV2()
	document.Brand.Name = "娇兰"
	document.Product.Name = "第三代黄金复原蜜"

	problems := service.projectBriefCompatibilityProblems(
		context.Background(), contract.ActorContext{}, "project_investor_precision_evidence", document,
	)
	if len(problems) != 2 || problems[0].Field != "project.brand" || problems[1].Field != "project.product" {
		t.Fatalf("cross-business Brief was not blocked: %#v", problems)
	}
}

func TestProjectBriefCompatibilityAcceptsQualifiedProductName(t *testing.T) {
	t.Parallel()
	brandID := contract.BrandID("brand_guerlain")
	service := Service{Projects: compatibilityProjectReader{business: contract.ProjectBusinessContext{
		ProjectID: "project_guerlain", ProjectName: "娇兰黄金复原蜜品牌广告",
		BrandID: &brandID, BrandName: "法国娇兰",
		Products: []contract.ProjectBusinessProduct{{ID: "product_abeille_royale", Name: "娇兰第三代黄金复原蜜"}},
	}}}
	document := EmptyBriefDocumentV2()
	document.Brand.Name = "娇兰"
	document.Product.Name = "第三代黄金复原蜜"

	if problems := service.projectBriefCompatibilityProblems(
		context.Background(), contract.ActorContext{}, "project_guerlain", document,
	); len(problems) != 0 {
		t.Fatalf("compatible qualified names were blocked: %#v", problems)
	}
}
