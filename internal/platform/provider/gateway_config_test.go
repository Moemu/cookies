package provider

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestAESGCMCredentialCipherRoundTripAndTamperDetection(t *testing.T) {
	t.Parallel()
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x2a}, 32))
	cipher, err := NewAESGCMCredentialCipher(key, "kms-v1")
	if err != nil {
		t.Fatalf("NewAESGCMCredentialCipher() error = %v", err)
	}
	ciphertext, nonce, keyVersion, err := cipher.Encrypt([]byte("adapter-service-token"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}
	plaintext, err := cipher.Decrypt(ciphertext, nonce, keyVersion)
	if err != nil || string(plaintext) != "adapter-service-token" {
		t.Fatalf("Decrypt() = %q, %v", plaintext, err)
	}
	tampered := append([]byte(nil), ciphertext...)
	tampered[0] ^= 0xff
	if _, err := cipher.Decrypt(tampered, nonce, keyVersion); err == nil {
		t.Fatal("Decrypt() accepted tampered ciphertext")
	}
	if _, err := cipher.Decrypt(ciphertext, nonce, "kms-v2"); err == nil {
		t.Fatal("Decrypt() accepted an unavailable key version")
	}
}

func TestVideoRouteAllowsLongPollingAndVideoSizedResponses(t *testing.T) {
	t.Parallel()
	route := GatewayRouteSnapshot{
		RouteID:              "route_video_1",
		RouteRevisionID:      "route_video_r1",
		ConnectionID:         "connection_video_1",
		ConnectionRevisionID: "connection_video_r1",
		BaseURL:              "https://ark.cn-beijing.volces.com/api/v3",
		UpstreamModel:        "doubao-seedance-2-0-fast-260128",
		CredentialID:         "credential_video_1",
		CredentialVersion:    1,
		TimeoutSeconds:       900,
		MaxResponseBytes:     200 << 20,
	}

	if err := route.ValidateVideoWithPolicy(false); err != nil {
		t.Fatalf("ValidateVideoWithPolicy() rejected a supported Seedance route: %v", err)
	}
	if err := route.ValidateWithPolicy(false); err == nil {
		t.Fatal("image/text route policy unexpectedly accepted video-sized limits")
	}
}
