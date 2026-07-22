package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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
	organizationID contract.OrganizationID
	projectID      contract.ProjectID
	providerJobID  string
	polls          int
	output         []byte
	expiresAt      time.Time
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
		a.tasks[externalTaskID] = &fakeImageTask{
			organizationID: request.OrganizationID, projectID: request.ProjectID, providerJobID: request.ProviderJobID,
			output: fakeImagePNG, expiresAt: a.now().UTC().Add(15 * time.Minute),
		}
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
		RetrievalExpiresAt: task.expiresAt, DeclaredMIMEType: "image/png", DeclaredSizeBytes: int64(len(task.output)), DeclaredSHA256: fakeImageSHA256(task.output),
	}}}, nil
}

// Open implements the Assets-owned GeneratedOutputFetcher seam. ProjectRef
// is required so the output handle cannot be replayed into another project.
func (a *FakeImageAdapter) Open(_ context.Context, project contract.ProjectRef, ref contract.ProviderOutputRef) (io.ReadCloser, contract.OutputMetadata, error) {
	if err := project.Validate(); err != nil {
		return nil, contract.OutputMetadata{}, err
	}
	if err := ref.Validate(); err != nil {
		return nil, contract.OutputMetadata{}, err
	}
	if ref.ProviderCode != fakeProviderCode || ref.OutputID != "output_1" {
		return nil, contract.OutputMetadata{}, fmt.Errorf("fake output was not found")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	var task *fakeImageTask
	for _, candidate := range a.tasks {
		if candidate.providerJobID == ref.ProviderJobID {
			task = candidate
			break
		}
	}
	if task == nil || task.organizationID != project.OrganizationID || task.projectID != project.ProjectID {
		return nil, contract.OutputMetadata{}, fmt.Errorf("fake output is not authorized for this project")
	}
	if !a.now().UTC().Before(task.expiresAt) {
		return nil, contract.OutputMetadata{}, fmt.Errorf("fake output has expired")
	}
	sha := sha256.Sum256(task.output)
	return io.NopCloser(bytes.NewReader(task.output)), contract.OutputMetadata{MIMEType: "image/png", SizeBytes: int64(len(task.output)), SHA256: hex.EncodeToString(sha[:])}, nil
}

func fakeImageSHA256(contents []byte) *string {
	digest := sha256.Sum256(contents)
	value := hex.EncodeToString(digest[:])
	return &value
}

var fakeImagePNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x04, 0x00, 0x00, 0x00, 0xb5, 0x1c, 0x0c,
	0x02, 0x00, 0x00, 0x00, 0x0b, 0x49, 0x44, 0x41, 0x54, 0x78, 0xda, 0x63, 0xfc, 0xff, 0x1f, 0x00,
	0x02, 0xeb, 0x01, 0xf5, 0x69, 0x47, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42,
	0x60, 0x82,
}
