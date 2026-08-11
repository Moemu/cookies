package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/systems/insights"
)

// miyunReturnImportAdapter is the only bridge from the Insights handoff state
// to Assets. It never exposes a signed object-store upload URL: the MP4 bytes
// enter the existing Assets external-import quarantine, scanner and probe path.
type miyunReturnImportAdapter struct {
	imports assets.ExternalImportService
	uploads assets.UploadService
}

func (a miyunReturnImportAdapter) ImportMiyunReturnMP4(ctx context.Context, rc contract.RequestContext, projectID contract.ProjectID, key contract.IdempotencyKey, request insights.MiyunReturnAssetImportRequest) (insights.MiyunReturnAssetImportResult, error) {
	if request.Content == nil || strings.TrimSpace(request.ReturnID) == "" || request.DeclaredMIMEType != "video/mp4" || request.DeclaredSizeBytes < 1 || request.DeclaredSizeBytes > assets.MaxVideoBytes {
		return insights.MiyunReturnAssetImportResult{}, fmt.Errorf("Miyun return requires one MP4 within the upload limit")
	}
	// A retry is keyed by both the return record and this upload request. This
	// lets a failed downstream Insights index retry the already-authorized
	// Assets intake without treating a new request key as a conflicting source.
	ref, err := a.imports.Import(ctx, rc, projectID, key, assets.ExternalMediaImportRequest{SourceProvider: "miyun_manual_return", SourceObjectID: request.ReturnID + ":" + string(key), MIMEType: "video/mp4", SizeBytes: request.DeclaredSizeBytes, ReturnSources: request.SourceResources}, func(context.Context) (io.ReadCloser, error) { return io.NopCloser(request.Content), nil })
	if err != nil {
		return insights.MiyunReturnAssetImportResult{}, err
	}
	stored, err := a.uploads.Get(ctx, rc.Actor, projectID, ref.AssetVersion)
	if err != nil {
		return insights.MiyunReturnAssetImportResult{}, err
	}
	if stored.Version.Status != assets.AssetReady || stored.Version.SourceType != contract.AssetSourceImported || stored.Version.MIMEType != "video/mp4" || stored.Version.SizeBytes != request.DeclaredSizeBytes || stored.Version.Media.ProbeStatus != assets.MediaProbeSucceeded || (request.DeclaredSHA256 != nil && stored.Version.SHA256 != *request.DeclaredSHA256) {
		return insights.MiyunReturnAssetImportResult{}, assets.ErrInvalidAssetContent
	}
	return insights.MiyunReturnAssetImportResult{AssetVersion: ref.AssetVersion, MIMEType: stored.Version.MIMEType, SHA256: stored.Version.SHA256, SizeBytes: stored.Version.SizeBytes}, nil
}
