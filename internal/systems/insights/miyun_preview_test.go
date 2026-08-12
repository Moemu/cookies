package insights

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestMiyunCandidatePreviewUsesProjectScopedAuthorizedAdapter(t *testing.T) {
	service, repository, _, _ := newMiyunCrawlTestService(t)
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	material := MiyunMaterial{
		ID: "preview_material", OrganizationID: "org_1", ProjectID: "project_1", MiyunMaterialID: "remote_preview",
		ImportMethod: MiyunImportCrawler, ResourceURLCiphertext: []byte("https://cdn.example.test/private.mp4"), ResourceURLKeyVersion: "key-v1",
		SelectionStatus: MiyunMaterialDiscovered, ImportStatus: MiyunMaterialImportPending, SourceRefStatus: "unknown",
		Version: 1, CreatedBy: "operator_1", CreatedAt: now, UpdatedAt: now,
	}
	repository.materials[material.MiyunMaterialID] = material
	previewer := &miyunPreviewTestAdapter{}
	service.MiyunPreviews = previewer

	preview, err := service.OpenMiyunMaterialPreview(context.Background(), miyunTestActor(), "project_1", material.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer preview.Content.Close()
	body, err := io.ReadAll(preview.Content)
	if err != nil || string(body) != "mp4-preview" || preview.MIMEType != "video/mp4" || preview.SizeBytes != int64(len(body)) {
		t.Fatalf("preview=%#v body=%q err=%v", preview, body, err)
	}
	if previewer.request.ResourceURL != "https://cdn.example.test/private.mp4" || previewer.request.ProjectID != "project_1" || previewer.request.MaterialID != material.ID {
		t.Fatalf("unexpected adapter request=%#v", previewer.request)
	}
}

func TestMiyunCandidatePreviewRequiresReadScope(t *testing.T) {
	service, _, _, _ := newMiyunCrawlTestService(t)
	actor := miyunTestActor()
	actor.Scopes = nil
	if _, err := service.OpenMiyunMaterialPreview(context.Background(), actor, "project_1", "anything"); err == nil {
		t.Fatal("preview without insights.read scope must fail")
	}
}

type miyunPreviewTestAdapter struct {
	request MiyunAuthorizedPreviewRequest
}

func (a *miyunPreviewTestAdapter) OpenMiyunMaterialPreview(_ context.Context, request MiyunAuthorizedPreviewRequest) (MiyunMaterialPreview, error) {
	a.request = request
	content := "mp4-preview"
	return MiyunMaterialPreview{Content: io.NopCloser(strings.NewReader(content)), SizeBytes: int64(len(content)), MIMEType: "video/mp4"}, nil
}
