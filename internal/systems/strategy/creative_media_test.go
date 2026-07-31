package strategy

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
)

type stubCreativeAssetReader struct {
	assets             map[string]assets.ProjectAsset
	features           []assets.AssetFeature
	unavailableContent map[string]bool
}

func (s stubCreativeAssetReader) OpenPreview(
	_ context.Context,
	_ contract.ActorContext,
	_ contract.ProjectID,
	ref contract.AssetVersionRef,
) (io.ReadCloser, assets.ObjectInfo, error) {
	if s.unavailableContent[assetRefKey(ref)] {
		return nil, assets.ObjectInfo{}, errors.New("content unavailable")
	}
	return io.NopCloser(strings.NewReader("asset")), assets.ObjectInfo{
		SizeBytes: 5, MIMEType: "application/octet-stream",
	}, nil
}

func (s stubCreativeAssetReader) Get(
	_ context.Context,
	_ contract.ActorContext,
	_ contract.ProjectID,
	ref contract.AssetVersionRef,
) (assets.ProjectAsset, error) {
	value, found := s.assets[assetRefKey(ref)]
	if !found {
		return assets.ProjectAsset{}, errors.New("missing")
	}
	return value, nil
}

func TestCreativeMediaAssessmentRejectsMetadataWithoutReadableContent(t *testing.T) {
	t.Parallel()
	ref := contract.AssetVersionRef{AssetID: "image_without_content", Version: 1}
	service := Service{CreativeAssets: stubCreativeAssetReader{
		assets: map[string]assets.ProjectAsset{
			assetRefKey(ref): {
				Asset: assets.Asset{ID: ref.AssetID, Kind: contract.AssetImage},
				Version: assets.AssetVersion{
					AssetID: ref.AssetID, Version: ref.Version,
					Status: assets.AssetReady, MIMEType: "image/png",
				},
			},
		},
		unavailableContent: map[string]bool{assetRefKey(ref): true},
	}}
	result := service.assessCreativeMedia(
		context.Background(), contract.ActorContext{}, "project_1",
		[]creativeMediaCandidate{{Ref: ref, Role: "product_image", Origin: "brief"}},
	)
	if result.UnavailableCount != 1 || result.ProductionOnlyCount != 0 ||
		result.Items[0].Status != "unavailable" {
		t.Fatalf("metadata-only phantom asset must be unavailable: %#v", result)
	}
}

func (s stubCreativeAssetReader) ListFeatures(
	context.Context,
	contract.ActorContext,
	contract.ProjectID,
	int,
) ([]assets.AssetFeature, error) {
	return s.features, nil
}

func TestCreativeMediaAssessmentDistinguishesSemanticAndMetadataOnlyInputs(t *testing.T) {
	t.Parallel()
	imageRef := contract.AssetVersionRef{AssetID: "image_1", Version: 1}
	videoRef := contract.AssetVersionRef{AssetID: "video_1", Version: 2}
	reader := stubCreativeAssetReader{
		assets: map[string]assets.ProjectAsset{
			assetRefKey(imageRef): {
				Asset: assets.Asset{ID: imageRef.AssetID, Kind: contract.AssetImage},
				Version: assets.AssetVersion{
					AssetID: imageRef.AssetID, Version: imageRef.Version,
					Status: assets.AssetReady, MIMEType: "image/png",
					WidthPixels: 1200, HeightPixels: 1600,
				},
			},
			assetRefKey(videoRef): {
				Asset: assets.Asset{ID: videoRef.AssetID, Kind: contract.AssetVideo},
				Version: assets.AssetVersion{
					AssetID: videoRef.AssetID, Version: videoRef.Version,
					Status: assets.AssetReady, MIMEType: "video/mp4",
					Media: assets.MediaMetadata{DurationSeconds: 15},
				},
			},
		},
		features: []assets.AssetFeature{{
			AssetID: imageRef.AssetID, AssetVersion: imageRef.Version,
			UpdatedAt: time.Now(), ProductVisibility: 0.9,
			SceneTags: []string{"通勤"}, SellingPoints: []string{"0 糖"},
			SimilarityRisk: assets.AssetFeatureRiskLow,
		}},
	}
	service := Service{CreativeAssets: reader}
	result := service.assessCreativeMedia(
		context.Background(),
		contract.ActorContext{},
		"project_1",
		[]creativeMediaCandidate{
			{Ref: imageRef, Role: "product_image", Origin: "brief"},
			{Ref: videoRef, Role: "main_video", Origin: "plan"},
		},
	)
	if result.SemanticCount != 1 || result.ProductionOnlyCount != 1 ||
		result.Items[0].Usefulness != CreativeMediaSemantic ||
		result.Items[1].Usefulness != CreativeMediaProductionOnly {
		t.Fatalf("unexpected media assessment: %#v", result)
	}
	if len(result.Items[1].Limitations) == 0 {
		t.Fatal("metadata-only video must explicitly say that content was not inspected")
	}
}
