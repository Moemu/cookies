package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateCommonAcceptsOnlyConnectorLocalAccount(t *testing.T) {
	if _, err := validateCommon(commonFlags{organization: "org", project: "project", account: "platform-account-id"}); err == nil {
		t.Fatal("raw platform account input was accepted")
	}
	value, err := validateCommon(commonFlags{organization: "org", account: "oeacct_local"})
	if err != nil || value == "oeacct_local" {
		t.Fatalf("local account was not converted to an opaque source ref: value=%q error=%v", value, err)
	}
}

func TestReadExportKeyRequiresExactly32Base64Bytes(t *testing.T) {
	t.Setenv(exportKeyEnvironment, base64.StdEncoding.EncodeToString(make([]byte, 31)))
	if _, err := readExportKey(); err == nil {
		t.Fatal("weak key was accepted")
	}
	t.Setenv(exportKeyEnvironment, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	value, err := readExportKey()
	if err != nil || len(value) != 32 {
		t.Fatalf("valid key rejected: length=%d error=%v", len(value), err)
	}
}

func TestWriteJSONNeverOverwritesOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.json")
	if err := writeJSON(path, map[string]string{"status": "first"}); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(path, map[string]string{"status": "second"}); err == nil {
		t.Fatal("existing export was overwritten")
	}
	value, err := os.ReadFile(path)
	if err != nil || string(value) != "{\n  \"status\": \"first\"\n}\n" {
		t.Fatalf("unexpected output %q error=%v", value, err)
	}
}
