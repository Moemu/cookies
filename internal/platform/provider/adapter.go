package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// ImageProviderAdapter isolates vendor-specific request, polling, and
// response formats. Adapters must use IdempotencyKey when the upstream
// provider supports it: a worker retry may repeat Submit after its response
// was lost before Provider persisted ExternalTaskID.
type ImageProviderAdapter interface {
	Submit(context.Context, ImageGenerationRequest) (ImageSubmission, error)
	Poll(context.Context, ImageTaskReference) (ImageTaskResult, error)
}

// ImageGenerationRequest contains only the Provider-owned fields needed to
// invoke an image model. It deliberately contains neither a database handle
// nor an Asset storage location.
type ImageGenerationRequest struct {
	OrganizationID contract.OrganizationID
	ProjectID      contract.ProjectID
	ProviderJobID  string
	ModelAlias     string
	IdempotencyKey contract.IdempotencyKey
	Input          ImageGenerationInput
}

func (r ImageGenerationRequest) Validate() error {
	if strings.TrimSpace(string(r.OrganizationID)) == "" || strings.TrimSpace(string(r.ProjectID)) == "" || strings.TrimSpace(r.ProviderJobID) == "" || strings.TrimSpace(r.ModelAlias) == "" {
		return fmt.Errorf("organization ID, project ID, provider job ID, and model alias are required")
	}
	if err := r.IdempotencyKey.Validate(); err != nil {
		return err
	}
	return r.Input.Validate()
}

// ImageSubmission is the durable acknowledgment from a Provider after a
// request was accepted. ProviderCode and ModelVersion are system-resolved,
// never accepted from the public API request.
type ImageSubmission struct {
	ProviderCode   string
	ModelVersion   string
	ExternalTaskID string
}

func (s ImageSubmission) Validate() error {
	if strings.TrimSpace(s.ProviderCode) == "" || strings.TrimSpace(s.ModelVersion) == "" || strings.TrimSpace(s.ExternalTaskID) == "" {
		return fmt.Errorf("provider code, model version, and external task ID are required")
	}
	return nil
}

type ImageTaskReference struct {
	ProviderCode   string
	ModelAlias     string
	ModelVersion   string
	ExternalTaskID string
}

func (r ImageTaskReference) Validate() error {
	if strings.TrimSpace(r.ProviderCode) == "" || strings.TrimSpace(r.ModelAlias) == "" || strings.TrimSpace(r.ModelVersion) == "" || strings.TrimSpace(r.ExternalTaskID) == "" {
		return fmt.Errorf("provider code, model alias, model version, and external task ID are required")
	}
	return nil
}

type ImageTaskStatus string

const (
	ImageTaskRunning   ImageTaskStatus = "running"
	ImageTaskSucceeded ImageTaskStatus = "succeeded"
	ImageTaskFailed    ImageTaskStatus = "failed"
)

// ImageTaskResult is a vendor-normalized polling response. Outputs remain
// opaque handles; any vendor URL stays inside the adapter implementation.
type ImageTaskResult struct {
	Status   ImageTaskStatus
	Progress int
	Outputs  []contract.ProviderOutputRef
	Error    *contract.JobError
}

func (r ImageTaskResult) Validate() error {
	switch r.Status {
	case ImageTaskRunning:
		if r.Progress < 0 || r.Progress > 70 || len(r.Outputs) != 0 || r.Error != nil {
			return fmt.Errorf("running image task requires progress from 0 to 70 and no output or error")
		}
	case ImageTaskSucceeded:
		if len(r.Outputs) == 0 || r.Error != nil {
			return fmt.Errorf("succeeded image task requires one or more outputs and no error")
		}
		for index, output := range r.Outputs {
			if err := output.Validate(); err != nil {
				return fmt.Errorf("invalid image output at index %d: %w", index, err)
			}
		}
	case ImageTaskFailed:
		if r.Error == nil || strings.TrimSpace(r.Error.Code) == "" || len(r.Outputs) != 0 {
			return fmt.Errorf("failed image task requires one error and no output")
		}
	default:
		return fmt.Errorf("image task status is invalid")
	}
	return nil
}
