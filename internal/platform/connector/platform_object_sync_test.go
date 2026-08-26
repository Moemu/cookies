package connector

import "testing"

func TestPlatformObjectCandidatesKeepSafeMetadata(t *testing.T) {
	tests := []struct {
		name      string
		candidate PlatformObjectCandidate
		valid     bool
	}{
		{"image", candidate(imageMaterialCandidate(map[string]any{"material_id": "101", "file_name": "image-a", "width": float64(1080), "preview_url": "https://example.invalid/image"})), true},
		{"video", candidate(videoMaterialCandidate(map[string]any{"material_id": "202", "video_name": "video-a", "video_filmLength": float64(15), "url": "https://example.invalid/video"})), true},
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
	}
}

func candidate(value PlatformObjectCandidate, valid bool) PlatformObjectCandidate {
	if !valid {
		return PlatformObjectCandidate{}
	}
	return value
}

func TestPlatformObjectPaginationShapes(t *testing.T) {
	image := imageMaterialPage(map[string]any{"data": map[string]any{"images": []any{map[string]any{"material_id": "1"}}, "pagination": map[string]any{"total_page": float64(4)}}})
	landing := orangeLandingPage(map[string]any{"data": map[string]any{"data": []any{map[string]any{"site_id": "2"}}, "pagination": map[string]any{"total": float64(61), "size": float64(30)}}})
	if len(image.Items) != 1 || image.TotalPages != 4 {
		t.Fatalf("image=%#v", image)
	}
	if len(landing.Items) != 1 || landing.TotalPages != 3 {
		t.Fatalf("landing=%#v", landing)
	}
}
