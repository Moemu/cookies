package provider

import "testing"

func TestApplyTextRouteConstraintsDefaultsToPromptJSON(t *testing.T) {
	t.Parallel()
	var snapshot GatewayRouteSnapshot
	if err := applyTextRouteConstraints(&snapshot, []byte(`{"source_provider":"openai"}`)); err != nil {
		t.Fatal(err)
	}
	if snapshot.TextResponseMode != TextResponsePromptJSON || snapshot.TextAPIMode != TextAPIChatCompletions {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestApplyTextRouteConstraintsReadsResponsesLifecycle(t *testing.T) {
	t.Parallel()
	var snapshot GatewayRouteSnapshot
	err := applyTextRouteConstraints(&snapshot, []byte(`{
		"api_mode":"responses",
		"text_response_mode":"json_schema",
		"max_output_tokens":8192,
		"output_token_parameter":"max_output_tokens",
		"reasoning_effort":"xhigh",
		"background":true,
		"poll_interval_ms":750
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.TextAPIMode != TextAPIResponses || !snapshot.Background ||
		snapshot.ReasoningEffort != "xhigh" || snapshot.PollIntervalMS != 750 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestApplyTextRouteConstraintsReadsSamplingPolicy(t *testing.T) {
	t.Parallel()
	var snapshot GatewayRouteSnapshot
	err := applyTextRouteConstraints(&snapshot, []byte(`{
		"text_response_mode":"json_object",
		"max_output_tokens":4096,
		"output_token_parameter":"max_completion_tokens",
		"temperature":0.4,
		"thinking_mode":"disabled",
		"reasoning_split":true
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.TextResponseMode != TextResponseJSONObject ||
		snapshot.MaxOutputTokens != 4096 || snapshot.Temperature != 0.4 || !snapshot.TemperatureSet ||
		snapshot.OutputTokenParameter != TextOutputTokenParameterMaxCompletionTokens ||
		snapshot.ThinkingMode != "disabled" || !snapshot.ReasoningSplit {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestGatewayRouteRejectsUnknownOutputTokenParameter(t *testing.T) {
	t.Parallel()
	snapshot := GatewayRouteSnapshot{
		RouteID: "route", RouteRevisionID: "revision", ConnectionID: "connection",
		ConnectionRevisionID: "connection_revision", BaseURL: "https://gateway.example",
		UpstreamModel: "model", CredentialID: "credential", CredentialVersion: 1,
		TimeoutSeconds: 30, MaxResponseBytes: 1024, TextResponseMode: TextResponsePromptJSON,
		MaxOutputTokens: 1, OutputTokenParameter: "output_length",
	}
	if err := snapshot.ValidateTextWithPolicy(false); err == nil {
		t.Fatal("expected invalid output token parameter")
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
