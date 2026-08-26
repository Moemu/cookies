package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/integrations/oceanengine"
)

type platformObjectReader interface {
	ImageMaterialsPage(context.Context, oceanengine.AssetPageRequest) (map[string]any, error)
	VideoMaterialsPage(context.Context, oceanengine.AssetPageRequest) (map[string]any, error)
	OrangeLandingPagesPage(context.Context, oceanengine.AssetPageRequest) (map[string]any, error)
}

type platformObjectPage struct {
	Items      []map[string]any
	TotalPages int
}

func (s Synchronizer) syncPlatformObjectCatalog(ctx context.Context, request SyncRequest, runID string, reader oceanengine.Reader, limit, maxPages int) (map[PlatformObjectKind]PlatformObjectSyncStats, error) {
	if request.ProjectID == "" {
		return nil, nil
	}
	objectReader, ok := reader.(platformObjectReader)
	if !ok {
		return nil, fmt.Errorf("%w: Ocean Engine reader has no platform object catalog", ErrInvalidFact)
	}
	catalog, ok := s.Writer.(PlatformObjectCatalog)
	if !ok {
		return nil, fmt.Errorf("%w: Connector writer has no platform object catalog", ErrInvalidFact)
	}
	type source struct {
		kind     PlatformObjectKind
		endpoint string
		fetch    func(context.Context, oceanengine.AssetPageRequest) (map[string]any, error)
		parse    func(map[string]any) platformObjectPage
		convert  func(map[string]any) (PlatformObjectCandidate, bool)
	}
	sources := []source{
		{PlatformObjectImageMaterial, "image_material_list", objectReader.ImageMaterialsPage, imageMaterialPage, imageMaterialCandidate},
		{PlatformObjectVideoMaterial, "video_material_list", objectReader.VideoMaterialsPage, videoMaterialPage, videoMaterialCandidate},
		{PlatformObjectOrangeLandingPage, "orange_landing_page_list", objectReader.OrangeLandingPagesPage, orangeLandingPage, orangeLandingCandidate},
	}
	result := make(map[PlatformObjectKind]PlatformObjectSyncStats, len(sources))
	for _, current := range sources {
		candidates := []PlatformObjectCandidate{}
		observedAt := time.Time{}
		for page := 1; page <= maxPages; page++ {
			cursor := fmt.Sprintf("platform_objects:%s:page:%d", current.kind, page)
			if err := s.Writer.UpdateSyncCursor(ctx, runID, cursor); err != nil {
				return result, err
			}
			payload, err := current.fetch(ctx, oceanengine.AssetPageRequest{Page: page, Limit: limit})
			if err != nil {
				return result, err
			}
			_, collectedAt, err := s.storeRaw(ctx, request, runID, current.endpoint, map[string]any{"page": page, "limit": limit}, payload)
			if err != nil {
				return result, err
			}
			observedAt = collectedAt
			parsed := current.parse(payload)
			for _, item := range parsed.Items {
				if candidate, valid := current.convert(item); valid {
					candidates = append(candidates, candidate)
				}
			}
			complete := parsed.TotalPages > 0 && page >= parsed.TotalPages
			if parsed.TotalPages == 0 && len(parsed.Items) < limit {
				complete = true
			}
			if complete {
				break
			}
			if page == maxPages {
				return result, fmt.Errorf("%w: %s exceeds page limit", ErrInvalidFact, current.kind)
			}
		}
		if observedAt.IsZero() {
			return result, fmt.Errorf("%w: %s produced no page", ErrInvalidFact, current.kind)
		}
		stats, err := catalog.ReconcilePlatformObjects(ctx, request.OrganizationID, request.ProjectID, request.AccountRef, runID, current.kind, observedAt, candidates)
		if err != nil {
			return result, err
		}
		result[current.kind] = stats
	}
	return result, nil
}

func imageMaterialPage(payload map[string]any) platformObjectPage {
	data, _ := payload["data"].(map[string]any)
	return platformObjectPage{Items: mapItems(data["images"]), TotalPages: totalPages(data, 0)}
}

func videoMaterialPage(payload map[string]any) platformObjectPage {
	data, _ := payload["data"].(map[string]any)
	return platformObjectPage{Items: mapItems(data["videos"]), TotalPages: totalPages(data, 0)}
}

func orangeLandingPage(payload map[string]any) platformObjectPage {
	data, _ := payload["data"].(map[string]any)
	return platformObjectPage{Items: mapItems(data["data"]), TotalPages: totalPages(data, 30)}
}

func totalPages(data map[string]any, fallbackPageSize int) int {
	pagination, _ := data["pagination"].(map[string]any)
	if pages := int(numberValue(pagination["total_page"])); pages > 0 {
		return pages
	}
	total := int(numberValue(pagination["total"]))
	pageSize := int(numberValue(pagination["size"]))
	if pageSize < 1 {
		pageSize = fallbackPageSize
	}
	if total > 0 && pageSize > 0 {
		return int(math.Ceil(float64(total) / float64(pageSize)))
	}
	return 0
}

func mapItems(value any) []map[string]any {
	items, _ := value.([]any)
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if mapped, ok := item.(map[string]any); ok {
			result = append(result, mapped)
		}
	}
	return result
}

func imageMaterialCandidate(item map[string]any) (PlatformObjectCandidate, bool) {
	id := firstString(item, "material_id")
	if !numericPlatformObjectID(id) {
		return PlatformObjectCandidate{}, false
	}
	return PlatformObjectCandidate{
		Kind: PlatformObjectImageMaterial, PlatformObjectID: id,
		DisplayName: firstString(item, "file_name"),
		Metadata:    scalarMetadata(item, "width", "height", "size", "image_mode", "ratio", "create_time"),
	}, true
}

func videoMaterialCandidate(item map[string]any) (PlatformObjectCandidate, bool) {
	id := firstString(item, "material_id", "video_id")
	if !numericPlatformObjectID(id) {
		return PlatformObjectCandidate{}, false
	}
	return PlatformObjectCandidate{
		Kind: PlatformObjectVideoMaterial, PlatformObjectID: id,
		DisplayName: firstString(item, "video_name"),
		Metadata:    scalarMetadata(item, "video_filmLength", "image_mode", "is_low_quality", "similar_material_status", "related_creative_count", "create_time"),
	}, true
}

func orangeLandingCandidate(item map[string]any) (PlatformObjectCandidate, bool) {
	id := firstString(item, "site_id")
	if !numericPlatformObjectID(id) {
		return PlatformObjectCandidate{}, false
	}
	return PlatformObjectCandidate{
		Kind: PlatformObjectOrangeLandingPage, PlatformObjectID: id,
		DisplayName: firstString(item, "name"),
		Metadata:    scalarMetadata(item, "audit_status", "status", "share_mode", "create_time"),
	}, true
}

func scalarMetadata(item map[string]any, keys ...string) map[string]any {
	result := map[string]any{}
	for _, key := range keys {
		switch value := item[key].(type) {
		case string:
			value = strings.TrimSpace(value)
			if value != "" && !strings.HasPrefix(strings.ToLower(value), "http://") && !strings.HasPrefix(strings.ToLower(value), "https://") {
				result[key] = value
			}
		case bool, float64, json.Number:
			result[key] = value
		}
	}
	return result
}
