package insights

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// MiyunMaterialPreview is an authorized, short-lived media stream. The caller
// only receives its bytes through the Insights HTTP handler; it never receives
// the upstream resource locator.
type MiyunMaterialPreview struct {
	Content   io.ReadCloser
	SizeBytes int64
	MIMEType  string
}

type MiyunAuthorizedPreviewRequest struct {
	Actor           contract.ActorContext
	ProjectID       contract.ProjectID
	MaterialID      string
	MiyunMaterialID string
	ResourceURL     string
	ExpectedSize    int64
}

type MiyunAuthorizedPreviewer interface {
	OpenMiyunMaterialPreview(context.Context, MiyunAuthorizedPreviewRequest) (MiyunMaterialPreview, error)
}

// OpenMiyunMaterialPreview authorizes a Project-scoped candidate preview. It
// decrypts the crawler locator only long enough to pass it to the server-side
// preview adapter, then clears it before returning a stream to HTTP.
func (s Service) OpenMiyunMaterialPreview(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, materialID string) (MiyunMaterialPreview, error) {
	if err := s.miyunReady(actor, projectID, ScopeRead); err != nil {
		return MiyunMaterialPreview{}, err
	}
	if s.MiyunCrawl == nil || s.MiyunSecrets == nil || s.MiyunPreviews == nil {
		return MiyunMaterialPreview{}, fmt.Errorf("Miyun candidate preview is unavailable")
	}
	material, err := s.MiyunCrawl.GetMiyunMaterial(ctx, actor.OrganizationID, projectID, strings.TrimSpace(materialID))
	if err != nil {
		return MiyunMaterialPreview{}, err
	}
	if material.ImportMethod != MiyunImportCrawler || len(material.ResourceURLCiphertext) == 0 || strings.TrimSpace(material.ResourceURLKeyVersion) == "" {
		return MiyunMaterialPreview{}, fmt.Errorf("%w: candidate preview is unavailable for this material", ErrInvalidState)
	}
	plaintext, err := s.MiyunSecrets.Decrypt(material.ResourceURLCiphertext, material.ResourceURLKeyVersion)
	if err != nil {
		return MiyunMaterialPreview{}, fmt.Errorf("Miyun candidate preview is unavailable")
	}
	defer clearMiyunPreviewBytes(plaintext)
	preview, err := s.MiyunPreviews.OpenMiyunMaterialPreview(ctx, MiyunAuthorizedPreviewRequest{
		Actor: actor, ProjectID: projectID, MaterialID: material.ID, MiyunMaterialID: material.MiyunMaterialID,
		ResourceURL: string(plaintext), ExpectedSize: material.ResourceExpectedSize,
	})
	if err != nil {
		return MiyunMaterialPreview{}, err
	}
	if preview.Content == nil || preview.SizeBytes < 1 || preview.MIMEType != "video/mp4" {
		if preview.Content != nil {
			_ = preview.Content.Close()
		}
		return MiyunMaterialPreview{}, fmt.Errorf("Miyun candidate preview is unavailable")
	}
	return preview, nil
}

func clearMiyunPreviewBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
