package connector

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/integrations/oceanengine"
)

func TestPlatformObjectCandidatesKeepSafeMetadata(t *testing.T) {
	tests := []struct {
		name      string
		candidate PlatformObjectCandidate
		valid     bool
	}{
		{"image", candidate(imageMaterialCandidate(map[string]any{"material_id": "101", "file_name": "image-a", "width": float64(1080), "sign_url": "https://example.invalid/image?x-orig-expires=1787760000"})), true},
		{"video", candidate(videoMaterialCandidate(map[string]any{"material_id": "202", "video_name": "video-a", "video_filmLength": float64(15), "video_url": "refid:unsafe", "video_poster": "https://example.invalid/video-poster", "sign_url": "https://example.invalid/video-sign?x-orig-expires=1787760000"})), true},
		{"photo", candidate(awemePhotoMaterialCandidate(map[string]any{"material_id": "252", "file_name": "photo-a", "image_info": []any{map[string]any{"sign_url": "https://example.invalid/photo?x-orig-expires=1787760000"}}})), true},
		{"product", candidate(marketingProductCandidate(map[string]any{"product_id": "262", "name": "product-a", "brand_name": "brand-a", "clue_product_category": map[string]any{"category_name": "category-a"}})), true},
		{"landing", candidate(orangeLandingCandidate(map[string]any{"site_id": "303", "name": "landing-a", "audit_status": float64(1), "preview_url": "https://example.invalid/page"})), true},
		{"invalid-id", candidate(imageMaterialCandidate(map[string]any{"material_id": "not-numeric"})), false},
	}
	for _, test := range tests {
		if test.valid != (test.candidate.PlatformObjectID != "") {
			t.Fatalf("%s valid=%v candidate=%#v", test.name, test.valid, test.candidate)
		}
		for key, value := range test.candidate.Metadata {
			if key == "url" || key == "preview_url" || value == "https://example.invalid/image" || value == "https://example.invalid/video" || value == "https://example.invalid/page" {
				t.Fatalf("%s retained URL metadata: %#v", test.name, test.candidate.Metadata)
			}
		}
		if test.valid && test.name != "invalid-id" && test.name != "product" && test.candidate.PreviewURL == "" {
			t.Fatalf("%s preview missing: %#v", test.name, test.candidate)
		}
	}
	if value, _ := imageMaterialCandidate(map[string]any{"material_id": "101", "sign_url": "https://example.invalid/image?x-orig-expires=1787760000"}); value.PreviewExpiresAt == nil || !value.PreviewExpiresAt.Equal(time.Unix(1787760000, 0)) {
		t.Fatalf("preview expiry=%v", value.PreviewExpiresAt)
	}
	video, _ := videoMaterialCandidate(map[string]any{"material_id": "202", "sign_url": "https://example.invalid/video-sign", "video_poster": "https://example.invalid/video-poster"})
	if video.PreviewURL != "https://example.invalid/video-sign" {
		t.Fatalf("video preview did not prefer sign_url: %q", video.PreviewURL)
	}
	fallback, _ := videoMaterialCandidate(map[string]any{"material_id": "203", "video_poster": "https://example.invalid/video-poster"})
	if fallback.PreviewURL != "https://example.invalid/video-poster" {
		t.Fatalf("video preview fallback=%q", fallback.PreviewURL)
	}
}

type previewRefreshWriter struct {
	testWriter
	preview connectorPreviewState
}

type connectorPreviewState struct {
	value      PlatformObjectPreview
	updatedURL string
	updatedAt  time.Time
}

func (w *previewRefreshWriter) GetPlatformObjectPreview(context.Context, PlatformObjectPreviewQuery) (PlatformObjectPreview, error) {
	return w.preview.value, nil
}

func (w *previewRefreshWriter) updatePlatformObjectPreview(_ context.Context, _ PlatformObjectPreviewQuery, previewURL, _ string, expiresAt *time.Time, observedAt time.Time) error {
	w.preview.updatedURL = previewURL
	w.preview.updatedAt = observedAt
	w.preview.value.URL = previewURL
	w.preview.value.ExpiresAt = expiresAt
	return nil
}

type previewSignerReader struct {
	testReader
	wantURI   string
	signedURL string
}

func (r previewSignerReader) SignPictureURIs(_ context.Context, uris []string) (map[string]any, error) {
	if len(uris) != 1 || uris[0] != r.wantURI {
		return nil, fmt.Errorf("unexpected URIs: %v", uris)
	}
	return map[string]any{"data": map[string]any{"list": map[string]any{r.wantURI: map[string]any{"main_url": r.signedURL}}}}, nil
}

func TestPlatformObjectPreviewRefreshSignsStableURI(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	expires := now.Add(12 * time.Hour).Unix()
	uri := "tos-cn-i-sd07hgqsbj/image-key"
	signedURL := fmt.Sprintf("https://p0-adplatform-private.oceanengine.com/%s~tplv-iq460dd072-origin.image?x-orig-expires=%d", uri, expires)
	writer := &previewRefreshWriter{preview: connectorPreviewState{value: PlatformObjectPreview{
		URL:  "https://p0-adplatform-private.oceanengine.com/" + uri + "~tplv-iq460dd072-origin.image?x-orig-expires=1",
		Kind: "image",
	}}}
	syncer := Synchronizer{
		Writer:  writer,
		Readers: testFactory{reader: previewSignerReader{testReader: testReader{}, wantURI: uri, signedURL: signedURL}},
		Cipher:  testCipher{},
		Now:     func() time.Time { return now },
	}
	preview, err := syncer.RefreshPlatformObjectPreview(context.Background(), PlatformObjectPreviewQuery{OrganizationID: "org_1", ProjectID: "project_1", AccountID: "account_1", ObjectID: "object_1"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.URL != signedURL || writer.preview.updatedURL != signedURL || !writer.preview.updatedAt.Equal(now) || preview.ExpiresAt == nil || preview.ExpiresAt.Unix() != expires {
		t.Fatalf("preview=%#v state=%#v", preview, writer.preview)
	}
}

func TestStablePictureURIRemovesPublicObjectPrefix(t *testing.T) {
	got, err := stablePictureURI("https://p3-adplatform.byteadimg.com/obj/tos-cn-v-0051/poster-key")
	if err != nil || got != "tos-cn-v-0051/poster-key" {
		t.Fatalf("uri=%q error=%v", got, err)
	}
}

var _ oceanengine.Reader = previewSignerReader{}

func candidate(value PlatformObjectCandidate, valid bool) PlatformObjectCandidate {
	if !valid {
		return PlatformObjectCandidate{}
	}
	return value
}

func TestPlatformObjectPaginationShapes(t *testing.T) {
	image := imageMaterialPage(map[string]any{"data": map[string]any{"images": []any{map[string]any{"material_id": "1"}}, "pagination": map[string]any{"total_page": float64(4)}}})
	landing := orangeLandingPage(map[string]any{"data": map[string]any{"data": []any{map[string]any{"site_id": "2"}}, "pagination": map[string]any{"total": float64(61), "size": float64(30)}}})
	photo := awemePhotoMaterialPage(map[string]any{"data": map[string]any{"list": []any{map[string]any{"material_id": "3"}}, "pagination": map[string]any{"total_page": float64(2)}}})
	product := marketingProductPage(map[string]any{"data": map[string]any{"list": []any{map[string]any{"product_id": "4"}}, "pagination": map[string]any{"total_count": float64(65), "limit": float64(32)}}})
	if len(image.Items) != 1 || image.TotalPages != 4 {
		t.Fatalf("image=%#v", image)
	}
	if len(landing.Items) != 1 || landing.TotalPages != 3 {
		t.Fatalf("landing=%#v", landing)
	}
	if len(photo.Items) != 1 || photo.TotalPages != 2 || len(product.Items) != 1 || product.TotalPages != 3 {
		t.Fatalf("photo=%#v product=%#v", photo, product)
	}
}
