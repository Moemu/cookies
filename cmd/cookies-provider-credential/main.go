// cookies-provider-credential encrypts one Adapter service token for insertion
// into provider_credentials. It reads the token from stdin so it does not
// appear in shell history or process arguments.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/shikanon/cookies/internal/platform/provider"
)

func main() {
	key := os.Getenv("COOKIES_PROVIDER_MASTER_KEY")
	keyVersion := os.Getenv("COOKIES_PROVIDER_MASTER_KEY_VERSION")
	cipher, err := provider.NewAESGCMCredentialCipher(key, keyVersion)
	if err != nil {
		exitError(err)
	}
	token, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(token) == 0 {
		exitError(fmt.Errorf("read Adapter token from stdin: %w", err))
	}
	token = strings.TrimSpace(token)
	if token == "" {
		exitError(fmt.Errorf("Adapter token from stdin is empty"))
	}
	ciphertext, nonce, version, err := cipher.Encrypt([]byte(token))
	if err != nil {
		exitError(err)
	}
	output := struct {
		CiphertextBase64 string `json:"ciphertext_base64"`
		NonceBase64      string `json:"nonce_base64"`
		KeyVersion       string `json:"key_version"`
	}{
		CiphertextBase64: base64.StdEncoding.EncodeToString(ciphertext),
		NonceBase64:      base64.StdEncoding.EncodeToString(nonce),
		KeyVersion:       version,
	}
	if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
		exitError(err)
	}
}

func exitError(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
