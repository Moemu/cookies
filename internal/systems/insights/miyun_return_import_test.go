package insights

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func validMiyunReturnImportInput() MiyunReturnImportInput {
	return MiyunReturnImportInput{HandoffID: "handoff_1", ManifestInputHash: strings.Repeat("a", 64), Filename: "final.mp4",
		AssetVersion: contract.AssetVersionRef{AssetID: "asset_1", Version: 2}, MIMEType: MiyunReturnImportMIMEType,
		SHA256: strings.Repeat("b", 64), SizeBytes: 42, ScanPassed: true, ProbePassed: true}
}

func TestMiyunReturnImportInputRejectsEmptyAndNonMP4(t *testing.T) {
	for name, mutate := range map[string]func(*MiyunReturnImportInput){
		"empty handoff": func(v *MiyunReturnImportInput) { v.HandoffID = " " },
		"zip":           func(v *MiyunReturnImportInput) { v.Filename, v.MIMEType = "handoff.zip", "application/zip" },
		"manifest":      func(v *MiyunReturnImportInput) { v.Filename, v.MIMEType = "manifest.csv", "text/csv" },
		"not scanned":   func(v *MiyunReturnImportInput) { v.ScanPassed = false },
		"not probed":    func(v *MiyunReturnImportInput) { v.ProbePassed = false },
	} {
		t.Run(name, func(t *testing.T) {
			input := validMiyunReturnImportInput()
			mutate(&input)
			if err := input.Validate(); !errors.Is(err, ErrInvalidRequest) && !errors.Is(err, ErrInvalidState) {
				t.Fatalf("Validate() error = %v, want invalid input/state", err)
			}
		})
	}
}

func TestMiyunReturnImportInputRejectsBadFrozenHashOrSize(t *testing.T) {
	for name, mutate := range map[string]func(*MiyunReturnImportInput){
		"bad manifest hash": func(v *MiyunReturnImportInput) { v.ManifestInputHash = strings.Repeat("z", 64) },
		"bad file hash":     func(v *MiyunReturnImportInput) { v.SHA256 = "short" },
		"empty file":        func(v *MiyunReturnImportInput) { v.SizeBytes = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			input := validMiyunReturnImportInput()
			mutate(&input)
			if err := input.Validate(); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("Validate() error = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestMiyunReturnImportRecordFreezesSuccessMetadata(t *testing.T) {
	now := time.Date(2026, 8, 11, 12, 0, 0, 0, time.UTC)
	input := validMiyunReturnImportInput()
	record := MiyunReturnImportRecord{Status: MiyunReturnImportReturned, HandoffID: input.HandoffID, ManifestInputHash: input.ManifestInputHash,
		Filename: input.Filename, AssetVersion: input.AssetVersion, MIMEType: input.MIMEType, SHA256: input.SHA256, SizeBytes: input.SizeBytes,
		ReturnedBy: "user_1", ReturnedAt: &now}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestMiyunReturnImportFailurePreservesHandoffForRecovery(t *testing.T) {
	record := MiyunReturnImportRecord{Status: MiyunReturnImportFailed, HandoffID: "handoff_1", ManifestInputHash: strings.Repeat("a", 64), ReturnedBy: "user_1", FailureCode: "SCAN_FAILED"}
	if err := record.Validate(); err != nil {
		t.Fatalf("failed record Validate() error = %v", err)
	}
	if next, err := MiyunHandoffStatusAfterReturn(MiyunHandoffExported, MiyunReturnImportFailed); err != nil || next != MiyunHandoffExported {
		t.Fatalf("failed result state = %q, %v; want exported unchanged", next, err)
	}
	if _, err := MiyunHandoffStatusAfterReturn(MiyunHandoffFailed, MiyunReturnImportReturned); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("failed handoff returned error = %v, want ErrInvalidState", err)
	}
}
