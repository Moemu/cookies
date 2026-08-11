package provider

import (
	"encoding/json"
	"testing"
)

func TestApplyDocumentVisionRouteConstraints(t *testing.T) {
	snapshot := GatewayRouteSnapshot{
		RouteID: "route_1", RouteRevisionID: "route_revision_1",
		ConnectionID: "connection_1", ConnectionRevisionID: "connection_revision_1",
		ConnectionType: "las_operator", BaseURL: "https://operator.las.cn-beijing.volces.com/api/v1",
		UpstreamModel: "las_pdf_parse_doubao", CredentialID: "credential_1", CredentialVersion: 1,
		TimeoutSeconds: 900, MaxResponseBytes: 8 << 20,
	}
	raw := json.RawMessage(`{
		"endpoint":"/submit","poll_endpoint":"/poll","operator_version":"v1",
		"parse_mode":"detail","full_result":true,"aspect_ratio_threshold":0.334,
		"poll_interval_ms":1500
	}`)
	if err := applyDocumentVisionRouteConstraints(&snapshot, raw); err != nil {
		t.Fatalf("apply constraints: %v", err)
	}
	if err := snapshot.ValidateDocumentVisionWithPolicy(false); err != nil {
		t.Fatalf("validate snapshot: %v", err)
	}
	if snapshot.DocumentSubmitPath != "/submit" || snapshot.DocumentPollPath != "/poll" ||
		snapshot.DocumentOperatorVersion != "v1" || snapshot.DocumentParseMode != "detail" ||
		!snapshot.DocumentFullResult || snapshot.DocumentPollIntervalMS != 1500 {
		t.Fatalf("unexpected document route snapshot: %#v", snapshot)
	}
}

func TestDocumentVisionRouteRejectsUnsafeOrIncompleteConstraints(t *testing.T) {
	base := GatewayRouteSnapshot{
		RouteID: "route_1", RouteRevisionID: "route_revision_1",
		ConnectionID: "connection_1", ConnectionRevisionID: "connection_revision_1",
		ConnectionType: "las_operator", BaseURL: "https://operator.las.cn-beijing.volces.com/api/v1",
		UpstreamModel: "las_pdf_parse_doubao", CredentialID: "credential_1", CredentialVersion: 1,
		TimeoutSeconds: 900, MaxResponseBytes: 8 << 20,
	}
	tests := []json.RawMessage{
		json.RawMessage(`{"endpoint":"https://evil.example/submit","poll_endpoint":"/poll","operator_version":"v1","parse_mode":"detail"}`),
		json.RawMessage(`{"endpoint":"/submit","poll_endpoint":"/poll?token=x","operator_version":"v1","parse_mode":"detail"}`),
		json.RawMessage(`{"endpoint":"/submit","poll_endpoint":"/poll","operator_version":"v1","parse_mode":"unknown"}`),
		json.RawMessage(`{"endpoint":"/submit","poll_endpoint":"/poll","operator_version":"v1","parse_mode":"detail","full_result":false}`),
	}
	for index, raw := range tests {
		snapshot := base
		if err := applyDocumentVisionRouteConstraints(&snapshot, raw); err != nil {
			continue
		}
		if err := snapshot.ValidateDocumentVisionWithPolicy(false); err == nil {
			t.Fatalf("case %d unexpectedly accepted", index)
		}
	}
}
