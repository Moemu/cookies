package assets

import (
	"context"
	"fmt"
	"strings"
)

type MediaProbeSource struct {
	Location  ObjectLocation
	MIMEType  string
	SizeBytes int64
	SHA256    string
}

type MediaProbe interface {
	Probe(context.Context, MediaProbeSource) (MediaMetadata, error)
}

type UnconfiguredMediaProbe struct{}

func (UnconfiguredMediaProbe) Probe(context.Context, MediaProbeSource) (MediaMetadata, error) {
	return MediaMetadata{}, fmt.Errorf("media probe is not configured")
}

type StaticMediaProbe struct {
	Metadata MediaMetadata
	Err      error
}

func (p StaticMediaProbe) Probe(context.Context, MediaProbeSource) (MediaMetadata, error) {
	if p.Err != nil {
		return MediaMetadata{}, p.Err
	}
	metadata := p.Metadata
	metadata.ProbeStatus = MediaProbeSucceeded
	return metadata, nil
}

func normalizeProbeFailure(err error) MediaMetadata {
	message := "media probe failed"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = err.Error()
	}
	return MediaMetadata{ProbeStatus: MediaProbeFailed, ProbeError: message}
}
