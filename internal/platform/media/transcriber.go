package media

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

type TranscriptionRequest struct {
	Actor       contract.ActorContext
	ProjectID   contract.ProjectID
	SourceVideo contract.AssetVersionRef
}

func (r TranscriptionRequest) Validate() error {
	if err := r.Actor.Validate(); err != nil || r.ProjectID == "" || r.SourceVideo.Validate() != nil {
		return fmt.Errorf("valid actor, project, and source video are required")
	}
	return nil
}

type Transcription struct {
	Text         string
	ProviderCode string
	ModelVersion string
}

func (t Transcription) Validate() error {
	if strings.TrimSpace(t.ProviderCode) == "" || strings.TrimSpace(t.ModelVersion) == "" {
		return fmt.Errorf("transcription provider lineage is required")
	}
	return nil
}

type Transcriber interface {
	Transcribe(context.Context, TranscriptionRequest) (Transcription, error)
}
