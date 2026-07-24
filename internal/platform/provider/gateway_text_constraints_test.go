package provider

import "testing"

func TestApplyTextRouteConstraintsDefaultsToPromptJSON(t *testing.T) {
	t.Parallel()
	var snapshot GatewayRouteSnapshot
	if err := applyTextRouteConstraints(&snapshot, []byte(`{"source_provider":"openai"}`)); err != nil {
		t.Fatal(err)
	}
	if snapshot.TextResponseMode != TextResponsePromptJSON {
		t.Fatalf("response mode = %q", snapshot.TextResponseMode)
	}
}

func TestApplyTextRouteConstraintsReadsSamplingPolicy(t *testing.T) {
	t.Parallel()
	var snapshot GatewayRouteSnapshot
	err := applyTextRouteConstraints(&snapshot, []byte(`{
		"text_response_mode":"json_object",
		"max_output_tokens":4096,
		"temperature":0.4,
		"thinking_mode":"disabled"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.TextResponseMode != TextResponseJSONObject ||
		snapshot.MaxOutputTokens != 4096 || snapshot.Temperature != 0.4 || !snapshot.TemperatureSet ||
		snapshot.ThinkingMode != "disabled" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestGatewayRouteRejectsUnknownThinkingMode(t *testing.T) {
	t.Parallel()
	snapshot := GatewayRouteSnapshot{
		RouteID: "route", RouteRevisionID: "revision", ConnectionID: "connection",
		ConnectionRevisionID: "connection_revision", BaseURL: "https://gateway.example",
		UpstreamModel: "model", CredentialID: "credential", CredentialVersion: 1,
		TimeoutSeconds: 30, MaxResponseBytes: 1024, TextResponseMode: TextResponsePromptJSON,
		ThinkingMode: "sometimes",
	}
	if err := snapshot.ValidateTextWithPolicy(false); err == nil {
		t.Fatal("expected invalid thinking mode")
	}
}

func TestApplyTextRouteConstraintsPreservesExplicitZeroTemperature(t *testing.T) {
	t.Parallel()
	var snapshot GatewayRouteSnapshot
	if err := applyTextRouteConstraints(&snapshot, []byte(`{"temperature":0}`)); err != nil {
		t.Fatal(err)
	}
	if !snapshot.TemperatureSet || snapshot.Temperature != 0 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}
