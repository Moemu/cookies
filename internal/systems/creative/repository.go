package creative

import (
	"context"
	"errors"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

var (
	ErrNotFound            = errors.New("creative resource not found")
	ErrIdempotencyConflict = errors.New("creative idempotency key conflicts with an earlier request")
	ErrIntakeNotReady      = errors.New("creative intake needs clarification before task creation")
	ErrProviderJobConflict = errors.New("production job already registered with a different provider job")
	ErrVersionConflict     = errors.New("creative resource version conflict")
	ErrInvalidState        = errors.New("creative resource is not in a state that allows this action")
)

type Repository interface {
	CreateIntake(context.Context, CreativeIntake) (CreativeIntake, bool, error)
	ListIntakes(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]CreativeIntake, error)
	GetIntake(context.Context, contract.OrganizationID, contract.ProjectID, string) (CreativeIntake, error)
	CreateTask(context.Context, CreativeTask, ImageTextDraft) (CreativeTask, error)
	CreateVideoTask(context.Context, CreativeTask, VideoDraft) (CreativeTask, error)
	ListTasks(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]CreativeTask, error)
	GetTaskDetail(context.Context, contract.OrganizationID, contract.ProjectID, string) (TaskDetail, error)
	ArchiveTask(context.Context, contract.OrganizationID, contract.ProjectID, string, time.Time) error
	ReviseDraft(context.Context, contract.OrganizationID, contract.ProjectID, string, int64, ImageTextDraft) (ImageTextDraft, error)
	RegisterProductionJob(context.Context, contract.OrganizationID, contract.ProjectID, string, ProductionJob) error
	CreateRenderJob(context.Context, RenderJob) (RenderJob, bool, error)
	GetRenderJob(context.Context, contract.OrganizationID, contract.ProjectID, string) (RenderJob, error)
	MarkRenderRunning(context.Context, contract.OrganizationID, contract.ProjectID, string, time.Time) (RenderJob, error)
	CompleteRenderJob(context.Context, contract.OrganizationID, contract.ProjectID, string, contract.ProjectAssetRef, time.Time) error
	FailRenderJob(context.Context, contract.OrganizationID, contract.ProjectID, string, string, string, time.Time) error
	CreateVersion(context.Context, CreativeVersion) (CreativeVersion, bool, error)
	GetVersion(context.Context, contract.OrganizationID, contract.ProjectID, string) (CreativeVersion, error)
	ListVersions(context.Context, contract.OrganizationID, contract.ProjectID, string, int) ([]CreativeVersion, error)
	RecordVersionCheck(context.Context, contract.OrganizationID, contract.ProjectID, string, CreativeCheck) (CreativeVersion, error)
	ApproveVersion(context.Context, contract.OrganizationID, contract.ProjectID, string, CreativeApproval) (CreativeVersion, error)
	CreatePackage(context.Context, CreativePackage) (CreativePackage, error)
	ListPackages(context.Context, contract.OrganizationID, contract.ProjectID, int) ([]CreativePackage, error)
}
