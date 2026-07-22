package provider

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const fakeProviderCode = "fake"
const fakeImageModelVersion = "fake-image-v1"

// FakeImageAdapter is an in-process deterministic Provider implementation for
// local development and consumer-contract tests. It never contacts a vendor
// and only returns opaque ProviderOutputRef values; a future Assets-facing
// fetcher will remain a separately authorized seam.
type FakeImageAdapter struct {
	mu    sync.Mutex
	now   func() time.Time
	tasks map[string]*fakeImageTask
}

type fakeImageTask struct {
	providerJobID string
	polls         int
}

func NewFakeImageAdapter(now func() time.Time) *FakeImageAdapter {
	if now == nil {
		now = time.Now
	}
	return &FakeImageAdapter{now: now, tasks: make(map[string]*fakeImageTask)}
}

func (a *FakeImageAdapter) Submit(_ context.Context, request ImageGenerationRequest) (ImageSubmission, error) {
	if err := request.Validate(); err != nil {
		return ImageSubmission{}, err
	}
	externalTaskID := "fake-task-" + request.ProviderJobID
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.tasks[externalTaskID]; !exists {
		a.tasks[externalTaskID] = &fakeImageTask{providerJobID: request.ProviderJobID}
	}
	return ImageSubmission{ProviderCode: fakeProviderCode, ModelVersion: fakeImageModelVersion, ExternalTaskID: externalTaskID}, nil
}

func (a *FakeImageAdapter) Poll(_ context.Context, reference ImageTaskReference) (ImageTaskResult, error) {
	if err := reference.Validate(); err != nil {
		return ImageTaskResult{}, err
	}
	if reference.ProviderCode != fakeProviderCode || reference.ModelVersion != fakeImageModelVersion {
		return ImageTaskResult{}, fmt.Errorf("fake task reference targets another provider or model")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	task, exists := a.tasks[reference.ExternalTaskID]
	if !exists {
		return ImageTaskResult{}, fmt.Errorf("fake external task was not found")
	}
	task.polls++
	if task.polls == 1 {
		return ImageTaskResult{Status: ImageTaskRunning, Progress: 50}, nil
	}
	return ImageTaskResult{Status: ImageTaskSucceeded, Outputs: []contract.ProviderOutputRef{{
		ProviderCode: fakeProviderCode, ProviderJobID: task.providerJobID, OutputID: "output_1",
		RetrievalExpiresAt: a.now().UTC().Add(15 * time.Minute), DeclaredMIMEType: "image/png", DeclaredSizeBytes: 1024,
	}}}, nil
}
