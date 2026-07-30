package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const fakeVideoProviderCode = "fake-video"
const fakeVideoModelVersion = "fake-video-v1"

type FakeVideoAdapter struct {
	mu    sync.Mutex
	now   func() time.Time
	tasks map[string]*fakeVideoTask
}

type fakeVideoTask struct {
	organizationID contract.OrganizationID
	projectID      contract.ProjectID
	providerJobID  string
	polls          int
	output         []byte
	expiresAt      time.Time
}

func NewFakeVideoAdapter(now func() time.Time) *FakeVideoAdapter {
	if now == nil {
		now = time.Now
	}
	return &FakeVideoAdapter{now: now, tasks: make(map[string]*fakeVideoTask)}
}

func (*FakeVideoAdapter) ProviderCode() string { return fakeVideoProviderCode }

func (a *FakeVideoAdapter) Submit(_ context.Context, request VideoGenerationRequest) (VideoSubmission, error) {
	if err := request.Validate(); err != nil {
		return VideoSubmission{}, err
	}
	externalTaskID := "fake-video-task-" + request.ProviderJobID
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.tasks[externalTaskID]; !exists {
		a.tasks[externalTaskID] = &fakeVideoTask{
			organizationID: request.OrganizationID,
			projectID:      request.ProjectID,
			providerJobID:  request.ProviderJobID,
			output:         fakeVideoMP4,
			expiresAt:      a.now().UTC().Add(15 * time.Minute),
		}
	}
	return VideoSubmission{
		Status:         VideoSubmissionAccepted,
		ProviderCode:   fakeVideoProviderCode,
		ModelVersion:   fakeVideoModelVersion,
		ExternalTaskID: externalTaskID,
	}, nil
}

func (a *FakeVideoAdapter) Poll(_ context.Context, reference VideoTaskReference) (VideoTaskResult, error) {
	if err := reference.Validate(); err != nil {
		return VideoTaskResult{}, err
	}
	if reference.ProviderCode != fakeVideoProviderCode || reference.ModelVersion != fakeVideoModelVersion {
		return VideoTaskResult{}, fmt.Errorf("fake video task reference targets another provider or model")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	task, exists := a.tasks[reference.ExternalTaskID]
	if !exists {
		return VideoTaskResult{}, fmt.Errorf("fake video task was not found")
	}
	task.polls++
	if task.polls == 1 {
		return VideoTaskResult{Status: VideoTaskRunning, Progress: 50}, nil
	}
	digest := sha256.Sum256(task.output)
	sha := hex.EncodeToString(digest[:])
	return VideoTaskResult{Status: VideoTaskSucceeded, Outputs: []contract.ProviderOutputRef{{
		ProviderCode:       fakeVideoProviderCode,
		ProviderJobID:      task.providerJobID,
		OutputID:           "output_1",
		RetrievalExpiresAt: task.expiresAt,
		DeclaredMIMEType:   "video/mp4",
		DeclaredSizeBytes:  int64(len(task.output)),
		DeclaredSHA256:     &sha,
	}}}, nil
}

func (a *FakeVideoAdapter) Open(_ context.Context, project contract.ProjectRef, ref contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error) {
	if err := project.Validate(); err != nil {
		return nil, contract.OutputMetadata{}, err
	}
	if err := ref.Validate(); err != nil {
		return nil, contract.OutputMetadata{}, err
	}
	if ref.ProviderCode != fakeVideoProviderCode || ref.OutputID != "output_1" {
		return nil, contract.OutputMetadata{}, ErrOutputHandleNotFound
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var task *fakeVideoTask
	for _, candidate := range a.tasks {
		if candidate.providerJobID == ref.ProviderJobID {
			task = candidate
			break
		}
	}
	if task == nil || task.organizationID != project.OrganizationID || task.projectID != project.ProjectID {
		return nil, contract.OutputMetadata{}, fmt.Errorf("fake video output is not authorized for this project")
	}
	if !a.now().UTC().Before(task.expiresAt) {
		return nil, contract.OutputMetadata{}, fmt.Errorf("fake video output has expired")
	}
	digest := sha256.Sum256(task.output)
	return io.NopCloser(bytes.NewReader(task.output)), contract.OutputMetadata{
		MIMEType:  "video/mp4",
		SizeBytes: int64(len(task.output)),
		SHA256:    hex.EncodeToString(digest[:]),
	}, nil
}

// This deterministic two-frame H.264 MP4 is deliberately tiny, but it is a
// complete media file rather than only an ISO BMFF header. Local development
// can therefore exercise Provider output intake and the real FFmpeg render
// path without calling a paid model.
var fakeVideoMP4 = mustDecodeFakeVideo("AAAAIGZ0eXBpc29tAAACAGlzb21pc28yYXZjMW1wNDEAAAMrbW9vdgAAAGxtdmhkAAAAAAAAAAAAAAAAAAAD6AAAAFAAAQAAAQAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAgAAAlZ0cmFrAAAAXHRraGQAAAADAAAAAAAAAAAAAAABAAAAAAAAAFAAAAAAAAAAAAAAAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAABAAAAAAAAAAAAAAAAAABAAAAAAFoAAACgAAAAAAAkZWR0cwAAABxlbHN0AAAAAAAAAAEAAABQAAAAAAABAAAAAAHObWRpYQAAACBtZGhkAAAAAAAAAAAAAAAAAAAyAAAABABVxAAAAAAALWhkbHIAAAAAAAAAAHZpZGUAAAAAAAAAAAAAAABWaWRlb0hhbmRsZXIAAAABeW1pbmYAAAAUdm1oZAAAAAEAAAAAAAAAAAAAACRkaW5mAAAAHGRyZWYAAAAAAAAAAQAAAAx1cmwgAAAAAQAAATlzdGJsAAAAuXN0c2QAAAAAAAAAAQAAAKlhdmMxAAAAAAAAAAEAAAAAAAAAAAAAAAAAAAAAAFoAoABIAAAASAAAAAAAAAABFUxhdmM2MS4yMi4xMDAgbGlieDI2NAAAAAAAAAAAAAAAGP//AAAAL2F2Y0MBQsAL/+EAGGdCwAvaGFeTwEQAAAMABAAAAwDIPFCqgAEABGjOD8gAAAAQcGFzcAAAAAEAAAABAAAAFGJ0cnQAAAAAAAEI2AABCNgAAAAYc3R0cwAAAAAAAAABAAAAAgAAAgAAAAAUc3RzcwAAAAAAAAABAAAAAQAAABxzdHNjAAAAAAAAAAEAAAABAAAAAgAAAAEAAAAcc3RzegAAAAAAAAAAAAAAAgAAApwAAAAKAAAAFHN0Y28AAAAAAAAAAQAAA1sAAABhdWR0YQAAAFltZXRhAAAAAAAAACFoZGxyAAAAAAAAAABtZGlyYXBwbAAAAAAAAAAAAAAAACxpbHN0AAAAJKl0b28AAAAcZGF0YQAAAAEAAAAATGF2ZjYxLjkuMTAwAAAACGZyZWUAAAKubWRhdAAAAlQGBf//UNxF6b3m2Ui3lizYINkj7u94MjY0IC0gY29yZSAxNjQgcjMxOTggZGExNGRmNSAtIEguMjY0L01QRUctNCBBVkMgY29kZWMgLSBDb3B5bGVmdCAyMDAzLTIwMjQgLSBodHRwOi8vd3d3LnZpZGVvbGFuLm9yZy94MjY0Lmh0bWwgLSBvcHRpb25zOiBjYWJhYz0wIHJlZj0xIGRlYmxvY2s9MDowOjAgYW5hbHlzZT0wOjAgbWU9ZGlhIHN1Ym1lPTAgcHN5PTEgcHN5X3JkPTEuMDA6MC4wMCBtaXhlZF9yZWY9MCBtZV9yYW5nZT0xNiBjaHJvbWFfbWU9MSB0cmVsbGlzPTAgOHg4ZGN0PTAgY3FtPTAgZGVhZHpvbmU9MjEsMTEgZmFzdF9wc2tpcD0xIGNocm9tYV9xcF9vZmZzZXQ9MCB0aHJlYWRzPTUgbG9va2FoZWFkX3RocmVhZHM9MSBzbGljZWRfdGhyZWFkcz0wIG5yPTAgZGVjaW1hdGU9MSBpbnRlcmxhY2VkPTAgYmx1cmF5X2NvbXBhdD0wIGNvbnN0cmFpbmVkX2ludHJhPTAgYmZyYW1lcz0wIHdlaWdodHA9MCBrZXlpbnQ9MjUwIGtleWludF9taW49MjUgc2NlbmVjdXQ9MCBpbnRyYV9yZWZyZXNoPTAgcmM9Y3JmIG1idHJlZT0wIGNyZj0yMy4wIHFjb21wPTAuNjAgcXBtaW49MCBxcG1heD02OSBxcHN0ZXA9NCBpcF9yYXRpbz0xLjQwIGFxPTAAgAAAAEBliIQ6EYoAAg5xwABCIjgACBzJycnJyddddddddddddddddddddddddddddddddddddddddddddddddddddddeAAAABkGaIC6B7A==")

func mustDecodeFakeVideo(value string) []byte {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		panic("decode deterministic fake video fixture: " + err.Error())
	}
	return decoded
}
