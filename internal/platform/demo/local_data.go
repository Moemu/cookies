package demo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/project"
)

// ImportLocalDemoData copies a reviewed local demo-data directory into the
// configured object store and makes every file visible in the investor demo
// project. Object keys and asset IDs are content-addressed, so repeated local
// deployments are safe and do not create duplicate project-library rows.
//
// The directory is intentionally supplied by the deployment environment. Demo
// media is not checked into the repository and no source filename is used as an
// object-store key.
func ImportLocalDemoData(ctx context.Context, actor contract.ActorContext, projects InvestorDemoProjectStore, assetStore InvestorDemoAssetStore, blobs assets.BlobStore, assetsBucket, directory string, contextVersion int64) (ImportedDemoDataResult, error) {
	if projects == nil || assetStore == nil || blobs == nil {
		return ImportedDemoDataResult{}, fmt.Errorf("demo-data import dependencies are required")
	}
	if err := actor.Validate(); err != nil {
		return ImportedDemoDataResult{}, err
	}
	if strings.TrimSpace(assetsBucket) == "" || strings.TrimSpace(directory) == "" || contextVersion < 1 {
		return ImportedDemoDataResult{}, fmt.Errorf("demo-data import configuration is invalid")
	}

	entries, err := os.ReadDir(directory)
	if err != nil {
		return ImportedDemoDataResult{}, fmt.Errorf("read demo-data directory: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	result := ImportedDemoDataResult{ProjectID: InvestorDemoProjectID}
	refs := make([]contract.ProjectAssetRef, 0, len(entries))
	var briefRef *contract.ProjectAssetRef
	var firstVideoRef *contract.ProjectAssetRef
	for _, entry := range entries {
		if entry.IsDir() || !entry.Type().IsRegular() {
			continue
		}
		path := filepath.Join(directory, entry.Name())
		seed, media, err := importDemoFile(ctx, actor.OrganizationID, path, blobs, assetsBucket, contextVersion)
		if err != nil {
			return ImportedDemoDataResult{}, err
		}
		ref, err := assetStore.EnsureSeedAsset(ctx, seed, time.Now().UTC())
		if err != nil {
			return ImportedDemoDataResult{}, fmt.Errorf("record demo-data asset %q: %w", entry.Name(), err)
		}
		refs = append(refs, ref)
		result.AssetRefs = append(result.AssetRefs, ref)
		result.TotalBytes += seed.SizeBytes
		if seed.Kind == contract.AssetDocument {
			result.DocumentCount++
			if briefRef == nil {
				copy := ref
				briefRef = &copy
			}
		}
		if seed.Kind == contract.AssetVideo {
			result.VideoCount++
			result.TotalVideoSeconds += media.DurationSeconds
			if firstVideoRef == nil {
				copy := ref
				firstVideoRef = &copy
			}
		}
	}
	if len(refs) == 0 || briefRef == nil || firstVideoRef == nil {
		return ImportedDemoDataResult{}, fmt.Errorf("demo-data directory must contain at least one PDF brief and one MP4 video")
	}
	if err := ensureImportedDataWalkthrough(ctx, actor.OrganizationID, projects, *briefRef, *firstVideoRef, result, time.Now().UTC()); err != nil {
		return ImportedDemoDataResult{}, err
	}
	return result, nil
}

type ImportedDemoDataResult struct {
	ProjectID         contract.ProjectID
	AssetRefs         []contract.ProjectAssetRef
	DocumentCount     int
	VideoCount        int
	TotalBytes        int64
	TotalVideoSeconds float64
}

func importDemoFile(ctx context.Context, organizationID contract.OrganizationID, path string, blobs assets.BlobStore, bucket string, contextVersion int64) (assets.SeedAsset, assets.MediaMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return assets.SeedAsset{}, assets.MediaMetadata{}, fmt.Errorf("open demo-data file %q: %w", path, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 {
		return assets.SeedAsset{}, assets.MediaMetadata{}, fmt.Errorf("invalid demo-data file %q", path)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return assets.SeedAsset{}, assets.MediaMetadata{}, fmt.Errorf("hash demo-data file %q: %w", path, err)
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	nameHashBytes := sha256.Sum256([]byte(filepath.Base(path)))
	nameHash := hex.EncodeToString(nameHashBytes[:])[:12]
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return assets.SeedAsset{}, assets.MediaMetadata{}, fmt.Errorf("rewind demo-data file %q: %w", path, err)
	}
	extension := strings.ToLower(filepath.Ext(path))
	kind, mimeType, err := demoDataType(extension)
	if err != nil {
		return assets.SeedAsset{}, assets.MediaMetadata{}, fmt.Errorf("unsupported demo-data file %q: %w", path, err)
	}
	// Keep every source file distinct even when two reviewed files contain the
	// same bytes, while still avoiding untrusted filenames in object keys.
	key := "demo/investor/local-data/" + digest + "-" + nameHash + extension
	stored, err := blobs.Put(ctx, bucket, key, file, info.Size(), mimeType)
	if err != nil {
		return assets.SeedAsset{}, assets.MediaMetadata{}, fmt.Errorf("store demo-data file %q: %w", path, err)
	}
	probe := demoVideoProbe{MediaMetadata: assets.MediaMetadata{ProbeStatus: assets.MediaProbeNotRequired}}
	if kind == contract.AssetVideo {
		probe = probeDemoVideo(ctx, path)
	}
	return assets.SeedAsset{
		OrganizationID: organizationID, ProjectID: InvestorDemoProjectID,
		AssetID: contract.AssetID("asset_demo_data_" + digest[:20] + "_" + nameHash), BlobID: "blob_demo_data_" + digest[:20] + "_" + nameHash,
		Kind: kind, SourceType: contract.AssetSourceImported, MIMEType: mimeType, SizeBytes: info.Size(), SHA256: digest,
		WidthPixels: probe.Width, HeightPixels: probe.Height, Media: probe.MediaMetadata,
		ProjectContextVersion: contextVersion, Location: stored.ObjectLocation,
	}, probe.MediaMetadata, nil
}

func demoDataType(extension string) (contract.AssetKind, string, error) {
	switch extension {
	case ".mp4":
		return contract.AssetVideo, "video/mp4", nil
	case ".pdf":
		return contract.AssetDocument, "application/pdf", nil
	default:
		return "", "", fmt.Errorf("only .pdf and .mp4 files are supported")
	}
}

// MediaMetadata does not retain dimensions. Demo seed dimensions are recorded
// in the database only for video files and are carried via this small probe
// result wrapper.
type demoVideoProbe struct {
	assets.MediaMetadata
	Width  int
	Height int
}

func probeDemoVideo(ctx context.Context, path string) demoVideoProbe {
	probe := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format=duration,bit_rate:stream=codec_type,codec_name,width,height,avg_frame_rate,channels,sample_rate", "-of", "json", path)
	output, err := probe.Output()
	if err != nil {
		return demoVideoProbe{MediaMetadata: assets.MediaMetadata{ProbeStatus: assets.MediaProbeFailed, ProbeError: "ffprobe metadata extraction failed"}}
	}
	var decoded struct {
		Format struct {
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			CodecType  string `json:"codec_type"`
			CodecName  string `json:"codec_name"`
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			FPS        string `json:"avg_frame_rate"`
			Channels   int    `json:"channels"`
			SampleRate string `json:"sample_rate"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(output, &decoded); err != nil {
		return demoVideoProbe{MediaMetadata: assets.MediaMetadata{ProbeStatus: assets.MediaProbeFailed, ProbeError: "ffprobe returned invalid metadata"}}
	}
	result := demoVideoProbe{MediaMetadata: assets.MediaMetadata{DurationSeconds: parseFloat(decoded.Format.Duration), BitrateBPS: int64(parseFloat(decoded.Format.BitRate)), ProbeStatus: assets.MediaProbeSucceeded}}
	for _, stream := range decoded.Streams {
		switch stream.CodecType {
		case "video":
			result.Codec = stream.CodecName
			result.FPS = parseFPS(stream.FPS)
			result.Width = stream.Width
			result.Height = stream.Height
		case "audio":
			result.AudioCodec = stream.CodecName
			result.AudioChannels = stream.Channels
			result.AudioSampleRate = int(parseFloat(stream.SampleRate))
		}
	}
	if result.DurationSeconds <= 0 || result.Codec == "" || result.Width < 1 || result.Height < 1 {
		return demoVideoProbe{MediaMetadata: assets.MediaMetadata{ProbeStatus: assets.MediaProbeFailed, ProbeError: "ffprobe did not return a video stream"}}
	}
	return result
}

func parseFloat(value string) float64 { parsed, _ := strconv.ParseFloat(value, 64); return parsed }
func parseFPS(value string) float64 {
	numerator, denominator, found := strings.Cut(value, "/")
	if !found {
		return parseFloat(value)
	}
	divisor := parseFloat(denominator)
	if divisor == 0 {
		return 0
	}
	return parseFloat(numerator) / divisor
}

func ensureImportedDataWalkthrough(ctx context.Context, organizationID contract.OrganizationID, store InvestorDemoProjectStore, brief, video contract.ProjectAssetRef, imported ImportedDemoDataResult, now time.Time) error {
	existing, err := store.ListBusinessTasks(ctx, organizationID, InvestorDemoProjectID)
	if err != nil {
		return err
	}
	desired := project.BusinessTask{ID: "task_demo_imported_brief_to_video", OrganizationID: organizationID, ProjectID: InvestorDemoProjectID, Type: project.BusinessTaskVideo, Name: "Guerlain KOL Brief 解析与视频生成演示", Objective: "解析导入的 PDF brief，并以已入库视频素材验证 brief 到视频创意的完整演示链路。", Status: project.BusinessTaskReady, SourceTaskIDs: []string{"task_demo_precision_strategy"}, SourceArtifactIDs: []string{string(brief.AssetVersion.AssetID)}, OutputArtifactIDs: []string{string(video.AssetVersion.AssetID)}, Version: 1, CreatedAt: now, UpdatedAt: now}
	foundTask := false
	for _, current := range existing {
		if current.ID == desired.ID {
			foundTask = true
			desired.Version = current.Version
			desired.CreatedAt = current.CreatedAt
			if taskNeedsUpdate(current, desired) {
				return store.UpdateBusinessTask(ctx, desired)
			}
			break
		}
	}
	if !foundTask {
		if err := store.CreateBusinessTask(ctx, desired); err != nil {
			return err
		}
	}

	operations, err := store.ListOperationalRecords(ctx, organizationID, InvestorDemoProjectID)
	if err != nil {
		return err
	}
	record := project.OperationalRecord{ID: "INSIGHT-DEMO-DATA-01", OrganizationID: organizationID, ProjectID: InvestorDemoProjectID, Kind: project.OperationalRecordMetric, Title: "导入素材洞察基线", Status: "ready", OccurredAt: now, Fields: map[string]any{"video_count": imported.VideoCount, "document_count": imported.DocumentCount, "total_video_seconds": int(imported.TotalVideoSeconds), "total_bytes": imported.TotalBytes, "insight": "已注入真实视频素材池；可在素材库、混剪和 brief 到视频任务中复演。"}, CreatedAt: now, UpdatedAt: now}
	for _, current := range operations {
		if current.ID == record.ID {
			record.CreatedAt = current.CreatedAt
			return store.UpdateOperationalRecord(ctx, record)
		}
	}
	return store.CreateOperationalRecord(ctx, record)
}
