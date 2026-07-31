package provider

import (
	"reflect"
	"testing"
)

func TestApplyVideoRouteConstraintsCapturesSeedanceCapabilities(t *testing.T) {
	var snapshot GatewayRouteSnapshot
	err := applyVideoRouteConstraints(&snapshot, []byte(`{
		"video_input_modes": ["text_only", "reference_image", "first_last_frame"],
		"video_audio_policies": ["silent", "generated_audio"]
	}`))
	if err != nil {
		t.Fatalf("applyVideoRouteConstraints() error = %v", err)
	}
	if !reflect.DeepEqual(snapshot.VideoInputModes, []VideoInputMode{
		VideoInputTextOnly, VideoInputReferenceImage, VideoInputFirstLastFrame,
	}) {
		t.Fatalf("video input modes = %#v", snapshot.VideoInputModes)
	}
	if !reflect.DeepEqual(snapshot.VideoAudioPolicies, []VideoAudioPolicy{
		VideoAudioSilent, VideoAudioGenerated,
	}) {
		t.Fatalf("video audio policies = %#v", snapshot.VideoAudioPolicies)
	}
}
