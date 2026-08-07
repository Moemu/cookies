package creative

import (
	"strings"
	"testing"
)

func TestBrandFilmSourceInvocationTokenSupportsStrategyHandoff(t *testing.T) {
	source := BrandFilmSourceSnapshot{
		SourceType:           strategyBrandFilmSourceType,
		DirectionContentHash: "sha256:9e96b18466abeb0842f745b72f40c54095bb897ba2284f05d646e6688fa780fc",
	}
	token := brandFilmSourceInvocationToken(source)
	if token != "9e96b18466ab" {
		t.Fatalf("unexpected Strategy source token: %s", token)
	}
}

func TestBrandFilmPlanOutputSchemaUsesRequestedDuration(t *testing.T) {
	schema := string(brandFilmPlanOutputSchema(30))
	if !strings.Contains(schema, `"end_second":{"type":"integer","minimum":1,"maximum":30}`) ||
		strings.Contains(schema, `"end_second":{"type":"integer","minimum":1,"maximum":15}`) {
		t.Fatalf("30-second plan schema still carries a 15-second ceiling: %s", schema)
	}
}
