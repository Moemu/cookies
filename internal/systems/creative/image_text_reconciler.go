package creative

import (
	"context"
	"fmt"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const defaultImageAttemptRecoveryAge = 2 * time.Minute

type ImageProviderJobReader interface {
	GetJob(context.Context, contract.OrganizationID, contract.ProjectID, string) (contract.ProviderJob, error)
}

// ImageTextReconciler advances image generation attempts independently of
// workspace reads. It also releases queued attempts left behind when the API
// process stops after persisting the attempt but before attaching a provider job.
type ImageTextReconciler struct {
	Service    *Service
	Repository ImageTextV2Repository
	Provider   ImageProviderJobReader
	Now        func() time.Time
	Limit      int
}

func (r ImageTextReconciler) ProcessOnce(ctx context.Context) (bool, error) {
	if r.Repository == nil || r.Provider == nil {
		return false, fmt.Errorf("image-text reconciler dependencies are required")
	}
	now := time.Now().UTC()
	if r.Now != nil {
		now = r.Now().UTC()
	}
	attempts, err := r.Repository.ListActiveImageGenerationAttempts(ctx, r.Limit)
	if err != nil {
		return false, err
	}
	processed := false
	var firstErr error
	for _, attempt := range attempts {
		if err := ctx.Err(); err != nil {
			return processed, err
		}
		if attempt.ProviderJobID == "" {
			if attempt.Status == ImageAttemptQueued && now.Sub(attempt.UpdatedAt) >= defaultImageAttemptRecoveryAge {
				_, markErr := r.Repository.MarkImageAttemptFailed(
					ctx, attempt.OrganizationID, attempt.ProjectID, attempt.ID,
					"PROVIDER_JOB_NOT_ATTACHED",
					"provider job was not attached; retry this image slot",
					now,
				)
				if markErr != nil {
					if firstErr == nil {
						firstErr = markErr
					}
					continue
				}
				processed = true
			}
			continue
		}
		job, getErr := r.Provider.GetJob(
			ctx, attempt.OrganizationID, attempt.ProjectID, attempt.ProviderJobID,
		)
		if getErr != nil {
			if firstErr == nil {
				firstErr = getErr
			}
			continue
		}
		requestContext := contract.RequestContext{
			RequestID: "image-reconcile-" + attempt.ID,
			TraceID:   "image-reconcile-" + attempt.ID,
			Actor: contract.ActorContext{
				OrganizationID: attempt.OrganizationID,
				Principal: contract.Principal{
					Kind: attempt.CreatedByKind,
					ID:   attempt.CreatedBy,
				},
				Scopes: []contract.Scope{
					"project.read", "assets.read", "assets.write", ScopeRead, ScopeWrite,
				},
			},
		}
		if _, reconcileErr := r.Service.ReconcileImageGenerationAttempt(
			ctx, requestContext, attempt.ProjectID, attempt.ID, job,
		); reconcileErr != nil {
			if firstErr == nil {
				firstErr = reconcileErr
			}
			continue
		}
		processed = true
	}
	return processed, firstErr
}
