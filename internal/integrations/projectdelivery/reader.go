// Package projectdelivery resolves Delivery plan lineage against Project's
// authoritative task and workbench read models.
package projectdelivery

import (
	"context"
	"fmt"
	"net/url"

	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/project"
	"github.com/shikanon/cookies/internal/systems/delivery"
)

type Reader struct {
	Service *project.Service
}

func (r Reader) ResolvePlanReferences(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, strategy delivery.StrategyReference, creatives []delivery.CreativeReference) (delivery.StrategyReference, []delivery.CreativeReference, error) {
	if r.Service == nil || strategy.TaskID == "" || strategy.Version < 1 || len(creatives) == 0 {
		return delivery.StrategyReference{}, nil, delivery.ErrInvalidRequest
	}
	task, err := r.Service.GetBusinessTask(ctx, actor, projectID, strategy.TaskID)
	if err != nil {
		return delivery.StrategyReference{}, nil, delivery.ErrNotFound
	}
	if task.Type != project.BusinessTaskStrategy || task.Version != strategy.Version ||
		(task.Status != project.BusinessTaskReady && task.Status != project.BusinessTaskCompleted) {
		return delivery.StrategyReference{}, nil, delivery.ErrInvalidState
	}
	strategyHash, err := contract.CanonicalJSONHash(task)
	if err != nil {
		return delivery.StrategyReference{}, nil, err
	}
	strategy = delivery.StrategyReference{
		TaskID: task.ID, Version: task.Version, ContentHash: strategyHash,
		Route: fmt.Sprintf("/projects/%s/strategy/workspaces/%s", url.PathEscape(string(projectID)), url.PathEscape(task.ID)),
	}

	workbench, err := r.Service.GetWorkbench(ctx, actor, projectID)
	if err != nil {
		return delivery.StrategyReference{}, nil, err
	}
	resolved := make([]delivery.CreativeReference, 0, len(creatives))
	for _, requested := range creatives {
		var pointer *project.WorkbenchAssetVersionPointer
		for index := range workbench.AssetVersionPointers {
			if workbench.AssetVersionPointers[index].AssetID == requested.AssetID {
				pointer = &workbench.AssetVersionPointers[index]
				break
			}
		}
		if pointer == nil {
			return delivery.StrategyReference{}, nil, delivery.ErrNotFound
		}
		var immutableVersion *project.WorkbenchAssetVersion
		for index := range pointer.Versions {
			if pointer.Versions[index].Version == requested.Version {
				immutableVersion = &pointer.Versions[index]
				break
			}
		}
		if immutableVersion == nil || pointer.HumanConfirmedVersion == nil || *pointer.HumanConfirmedVersion != requested.Version {
			return delivery.StrategyReference{}, nil, delivery.ErrInvalidState
		}
		contentHash, hashErr := contract.CanonicalJSONHash(struct {
			AssetID       string                              `json:"asset_id"`
			Version       project.WorkbenchAssetVersion       `json:"version"`
			Authorization project.WorkbenchAssetAuthorization `json:"authorization"`
		}{pointer.AssetID, *immutableVersion, pointer.Authorization})
		if hashErr != nil {
			return delivery.StrategyReference{}, nil, hashErr
		}
		resolved = append(resolved, delivery.CreativeReference{
			AssetID: pointer.AssetID, Version: requested.Version, ContentHash: contentHash, Confirmed: true,
			Route: fmt.Sprintf("/projects/%s/creative/reviews/%s@v%d", url.PathEscape(string(projectID)), url.PathEscape(pointer.AssetID), requested.Version),
		})
	}
	return strategy, resolved, nil
}
