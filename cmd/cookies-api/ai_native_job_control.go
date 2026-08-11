package main

import (
	"context"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/jobruntime"
)

type creativeAINativeOperationCanceller struct {
	store jobruntime.Canceller
	now   func() time.Time
}

func (c creativeAINativeOperationCanceller) CancelAINativeOperation(ctx context.Context, organizationID contract.OrganizationID, projectID contract.ProjectID, operationID string, version int64) error {
	now := time.Now().UTC()
	if c.now != nil {
		now = c.now()
	}
	_, err := c.store.RequestCancel(ctx, organizationID, projectID, operationID, version, now)
	return err
}
