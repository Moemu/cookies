package provider

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

// VideoProviderAdapter isolates asynchronous vendor submission and polling
// from Provider's durable job state machine.
type VideoProviderAdapter interface {
	Submit(context.Context, VideoGenerationRequest) (VideoSubmission, error)
	Poll(context.Context, VideoTaskReference) (VideoTaskResult, error)
}

type VideoGenerationRequest struct {
	OrganizationID contract.OrganizationID
	ProjectID      contract.ProjectID
	ProviderJobID  string
	ModelAlias     string
	IdempotencyKey contract.IdempotencyKey
	Input          VideoGenerationInput
	Sources        []VideoSource
	Route          *VideoRouteSnapshot
}

func (r VideoGenerationRequest) Validate() error {
	if strings.TrimSpace(string(r.OrganizationID)) == "" || strings.TrimSpace(string(r.ProjectID)) == "" || strings.TrimSpace(r.ProviderJobID) == "" || strings.TrimSpace(r.ModelAlias) == "" {
		return fmt.Errorf("organization ID, project ID, provider job ID, and model alias are required")
	}
	if err := r.IdempotencyKey.Validate(); err != nil {
		return err
	}
	if err := r.Input.Validate(); err != nil {
		return err
	}
	if len(r.Sources) != len(r.Input.ConditioningAssets) {
		return fmt.Errorf("video request source count does not match conditioning assets")
	}
	for index, source := range r.Sources {
		expected := r.Input.ConditioningAssets[index]
		if source.Role != expected.Role || source.Reference != expected.Reference ||
			!strings.HasPrefix(strings.ToLower(strings.TrimSpace(source.MIMEType)), "image/") || source.Content == nil {
			return fmt.Errorf("video request source at index %d is invalid", index)
		}
	}
	return nil
}

// VideoSource is an execution-scoped, authorized stream. Adapters may encode
// it into a vendor request, but must not persist or log its contents.
type VideoSource struct {
	Role      VideoConditioningRole
	Reference contract.ProjectAssetRef
	MIMEType  string
	Content   io.ReadCloser
}

type VideoSubmissionStatus string

const (
	VideoSubmissionAccepted VideoSubmissionStatus = "accepted"
)

type VideoSubmission struct {
	Status         VideoSubmissionStatus
	ProviderCode   string
	ModelVersion   string
	ExternalTaskID string
}

func (s VideoSubmission) Validate() error {
	if s.Status != VideoSubmissionAccepted || strings.TrimSpace(s.ProviderCode) == "" || strings.TrimSpace(s.ModelVersion) == "" || strings.TrimSpace(s.ExternalTaskID) == "" {
		return fmt.Errorf("accepted video submission requires provider, model, and external task ID")
	}
	return nil
}

type VideoTaskReference struct {
	OrganizationID contract.OrganizationID
	ProjectID      contract.ProjectID
	ProviderJobID  string
	ProviderCode   string
	ModelAlias     string
	ModelVersion   string
	ExternalTaskID string
	Route          *VideoRouteSnapshot
}

func (r VideoTaskReference) Validate() error {
	if strings.TrimSpace(string(r.OrganizationID)) == "" || strings.TrimSpace(string(r.ProjectID)) == "" || strings.TrimSpace(r.ProviderJobID) == "" ||
		strings.TrimSpace(r.ProviderCode) == "" || strings.TrimSpace(r.ModelAlias) == "" || strings.TrimSpace(r.ModelVersion) == "" || strings.TrimSpace(r.ExternalTaskID) == "" {
		return fmt.Errorf("project scope, provider job, provider code, model alias, model version, and external task ID are required")
	}
	return nil
}

type VideoTaskStatus string

const (
	VideoTaskRunning   VideoTaskStatus = "running"
	VideoTaskSucceeded VideoTaskStatus = "succeeded"
	VideoTaskFailed    VideoTaskStatus = "failed"
)

type VideoTaskResult struct {
	Status   VideoTaskStatus
	Progress int
	Outputs  []contract.ProviderOutputRef
	Error    *contract.JobError
}

func (r VideoTaskResult) Validate() error {
	switch r.Status {
	case VideoTaskRunning:
		if r.Progress < 0 || r.Progress > 70 || len(r.Outputs) != 0 || r.Error != nil {
			return fmt.Errorf("running video task requires progress from 0 to 70 and no output or error")
		}
	case VideoTaskSucceeded:
		if len(r.Outputs) == 0 || r.Error != nil {
			return fmt.Errorf("succeeded video task requires one or more outputs and no error")
		}
		for index, output := range r.Outputs {
			if err := output.Validate(); err != nil {
				return fmt.Errorf("invalid video output at index %d: %w", index, err)
			}
			if output.DeclaredMIMEType != "video/mp4" {
				return fmt.Errorf("video output at index %d is not MP4", index)
			}
		}
	case VideoTaskFailed:
		if r.Error == nil || strings.TrimSpace(r.Error.Code) == "" || len(r.Outputs) != 0 {
			return fmt.Errorf("failed video task requires one error and no output")
		}
	default:
		return fmt.Errorf("video task status is invalid")
	}
	return nil
}

// VideoRouteSnapshot is the immutable route selected when the job is created.
type VideoRouteSnapshot = GatewayRouteSnapshot
