package creative

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"

	"github.com/shikanon/cookies/internal/platform/contract"
)

func TestFrameConditionerDerivesFrostedStartFromClearTail(t *testing.T) {
	sourceRef := contract.AssetVersionRef{AssetID: "asset_product", Version: 1}
	store := newMemoryFrameAssetStore(sourceRef, solidPNG(t, 120, 300, color.RGBA{R: 194, G: 143, B: 42, A: 255}))
	conditioner := FrameConditioner{Assets: store}

	result, err := conditioner.Prepare(context.Background(), FrameConditioningRequest{
		OrganizationID: "org_1",
		ProjectID:      "project_1",
		FramePlan: CreativeFramePlan{
			ContractVersion: "creative-frame-plan/v1",
			TaskID:          "creativetask_1",
			Template:        TemplateReference{ID: CommerceWindowRevealTemplateID, Version: 1},
			ProductAsset:    sourceRef,
			WidthPixels:     720,
			HeightPixels:    1280,
			StartFrameKind:  "frosted_overlay",
			TailFrameKind:   "clear_product_reveal",
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if result.StartFrame == result.TailFrame || result.StartFrame.Validate() != nil || result.TailFrame.Validate() != nil {
		t.Fatalf("conditioned frame refs are invalid or equal: %+v", result)
	}

	start := decodePNG(t, store.bytesFor(result.StartFrame))
	tail := decodePNG(t, store.bytesFor(result.TailFrame))
	wantBounds := image.Rect(0, 0, 720, 1280)
	if start.Bounds() != wantBounds || tail.Bounds() != wantBounds {
		t.Fatalf("frame bounds start=%v tail=%v, want %v", start.Bounds(), tail.Bounds(), wantBounds)
	}
	if bytes.Equal(store.bytesFor(result.StartFrame), store.bytesFor(result.TailFrame)) {
		t.Fatal("frosted start frame equals clear tail frame")
	}
	tailCenter := color.RGBAModel.Convert(tail.At(360, 640)).(color.RGBA)
	startCenter := color.RGBAModel.Convert(start.At(360, 640)).(color.RGBA)
	if tailCenter == startCenter {
		t.Fatalf("frost overlay did not affect product center: %+v", tailCenter)
	}
	if len(store.savedRoles) != 2 || store.savedRoles[0] != FrameRoleTail || store.savedRoles[1] != FrameRoleStart {
		t.Fatalf("saved frame roles = %v, want tail then start", store.savedRoles)
	}
}

type memoryFrameAssetStore struct {
	images     map[contract.AssetVersionRef][]byte
	savedRoles []ConditionedFrameRole
	next       int
}

func newMemoryFrameAssetStore(ref contract.AssetVersionRef, contents []byte) *memoryFrameAssetStore {
	return &memoryFrameAssetStore{images: map[contract.AssetVersionRef][]byte{ref: contents}}
}

func (s *memoryFrameAssetStore) OpenImage(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, ref contract.AssetVersionRef) (io.ReadCloser, string, error) {
	contents, ok := s.images[ref]
	if !ok {
		return nil, "", fmt.Errorf("asset not found")
	}
	return io.NopCloser(bytes.NewReader(contents)), "image/png", nil
}

func (s *memoryFrameAssetStore) SavePNG(_ context.Context, _ contract.OrganizationID, _ contract.ProjectID, _ string, role ConditionedFrameRole, contents []byte) (contract.AssetVersionRef, error) {
	s.next++
	ref := contract.AssetVersionRef{AssetID: contract.AssetID(fmt.Sprintf("asset_frame_%d", s.next)), Version: 1}
	s.images[ref] = append([]byte{}, contents...)
	s.savedRoles = append(s.savedRoles, role)
	return ref, nil
}

func (s *memoryFrameAssetStore) bytesFor(ref contract.AssetVersionRef) []byte {
	return s.images[ref]
}

func solidPNG(t *testing.T, width, height int, fill color.Color) []byte {
	t.Helper()
	value := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			value.Set(x, y, fill)
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, value); err != nil {
		t.Fatalf("encode source PNG: %v", err)
	}
	return output.Bytes()
}

func decodePNG(t *testing.T, contents []byte) image.Image {
	t.Helper()
	value, err := png.Decode(bytes.NewReader(contents))
	if err != nil {
		t.Fatalf("decode PNG: %v", err)
	}
	return value
}
