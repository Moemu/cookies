package oceanengine

import "testing"

func TestContractVocabulary(t *testing.T) {
	if PlatformCode != "ocean_engine" || WebAPISessionMode != "web_api" || SourceConnector != "connector" {
		t.Fatalf("unexpected connector vocabulary")
	}
	for _, kind := range []ObjectKind{ObjectAccount, ObjectProject, ObjectPromotion, ObjectMaterial} {
		if !kind.Valid() {
			t.Fatalf("object kind %q is invalid", kind)
		}
	}
}
