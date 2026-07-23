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
