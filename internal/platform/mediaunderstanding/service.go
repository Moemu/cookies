package mediaunderstanding

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/assets"
	"github.com/shikanon/cookies/internal/platform/contract"
	"github.com/shikanon/cookies/internal/platform/ids"
	"github.com/shikanon/cookies/internal/platform/media"
	"github.com/shikanon/cookies/internal/platform/provider"
)

var ErrNotFound = errors.New("media understanding artifact not found")
var ErrUnsupportedProfile = errors.New("asset is unsupported by the media understanding profile")
var ErrNotReady = errors.New("media understanding artifact is not ready")

type ProjectReader interface {
	GetContext(context.Context, contract.ActorContext, contract.ProjectID) (contract.ProjectContext, error)
}

type AssetReader interface {
	Get(context.Context, contract.ActorContext, contract.ProjectID, contract.AssetVersionRef) (assets.ProjectAsset, error)
}

type DerivedImageWriter interface {
	IngestDerivedImage(context.Context, contract.RequestContext, contract.ProjectID, string, contract.AssetVersionRef, io.Reader, int64, string) (contract.ProjectAssetRef, error)
}

type VisionReader interface {
	UnderstandVision(context.Context, provider.VisionUnderstandRequest) (provider.SynchronousResponse, error)
}

type Scheduler interface {
	Schedule(context.Context, Artifact) (string, error)
}

type Service struct {
	Store         MySQLStore
	Projects      ProjectReader
	Assets        AssetReader
	DerivedImages DerivedImageWriter
	Frames        media.FrameExtractor
	Vision        VisionReader
	Scheduler     Scheduler
	ModelAlias    string
	NewID         ids.Generator
	Now           func() time.Time
}

type CreateRequest struct {
	AssetID string `json:"asset_id"`
	Version int64  `json:"version"`
	Profile string `json:"profile,omitempty"`
}

func (s Service) Request(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, request CreateRequest) (Artifact, bool, error) {
	if s.Store.DB == nil || s.Projects == nil || s.Assets == nil || s.Scheduler == nil {
		return Artifact{}, false, fmt.Errorf("media understanding service is unavailable")
	}
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return Artifact{}, false, err
	}
	profile := strings.TrimSpace(request.Profile)
	if profile == "" {
		profile = DefaultProfile
	}
	if profile != DefaultProfile {
		return Artifact{}, false, ErrUnsupportedProfile
	}
	ref := contract.AssetVersionRef{AssetID: contract.AssetID(strings.TrimSpace(request.AssetID)), Version: request.Version}
	if err := ref.Validate(); err != nil {
		return Artifact{}, false, err
	}
	asset, err := s.Assets.Get(ctx, actor, projectID, ref)
	if err != nil {
		return Artifact{}, false, err
	}
	if asset.Asset.Status != assets.AssetReady || asset.Version.Status != assets.AssetReady {
		return Artifact{}, false, assets.ErrAssetNotReady
	}
	if err := validateProfileAsset(asset); err != nil {
		return Artifact{}, false, err
	}
	identity, err := contract.CanonicalJSONHash(struct {
		AssetSHA256    string `json:"asset_sha256"`
		Profile        string `json:"profile"`
		ProfileVersion string `json:"profile_version"`
		PromptVersion  string `json:"prompt_version"`
		SchemaVersion  string `json:"schema_version"`
		ModelAlias     string `json:"model_alias"`
	}{asset.Version.SHA256, profile, DefaultProfileVersion, PromptVersion, SchemaVersion, s.modelAlias()})
	if err != nil {
		return Artifact{}, false, err
	}
	newID := s.NewID
	if newID == nil {
		newID = ids.New
	}
	id, err := newID("mediaunderstanding")
	if err != nil {
		return Artifact{}, false, err
	}
	now := s.now()
	value := Artifact{
		ContractVersion: ContractVersion, ID: id, OrganizationID: actor.OrganizationID, ProjectID: projectID,
		AssetRef: contract.ProjectAssetRef{ProjectID: projectID, AssetVersion: ref}, AssetKind: asset.Asset.Kind,
		AssetSHA256: asset.Version.SHA256, Profile: profile, ProfileVersion: DefaultProfileVersion,
		InputIdentityHash: identity, Status: StatusRunning,
		VisibleText: emptyEvidence(), Observations: emptyEvidence(), Inferences: emptyEvidence(),
		Risks: emptyEvidence(), Unknowns: emptyEvidence(), Keyframes: emptyKeyframes(),
		Transcript: emptyEvidence(), Warnings: emptyWarnings(),
		Lineage:   ModelLineage{ModelAlias: s.modelAlias(), PromptVersion: PromptVersion, SchemaVersion: SchemaVersion},
		CreatedBy: actor.Principal, CreatedAt: now, UpdatedAt: now,
	}
	value, duplicate, err := s.Store.Create(ctx, value)
	if err != nil || duplicate {
		return value, duplicate, err
	}
	jobID, err := s.Scheduler.Schedule(ctx, value)
	if err != nil {
		value.Status, value.ErrorCode, value.ErrorMessage = StatusFailed, "MEDIA_JOB_SCHEDULE_FAILED", "媒体理解任务无法调度"
		value, _ = s.Store.Complete(ctx, value, s.now())
		return value, false, err
	}
	value, err = s.Store.BindJob(ctx, value, jobID, s.now())
	return value, false, err
}

func (s Service) Get(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, id string) (Artifact, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return Artifact{}, err
	}
	value, err := s.Store.Get(ctx, actor.OrganizationID, projectID, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Artifact{}, ErrNotFound
	}
	return value, err
}

func (s Service) GetLatestForAsset(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, ref contract.AssetVersionRef) (Artifact, error) {
	if _, err := s.Projects.GetContext(ctx, actor, projectID); err != nil {
		return Artifact{}, err
	}
	value, err := s.Store.GetLatestForAsset(ctx, actor.OrganizationID, projectID, ref)
	if errors.Is(err, sql.ErrNoRows) {
		return Artifact{}, ErrNotFound
	}
	return value, err
}

func validateProfileAsset(value assets.ProjectAsset) error {
	switch value.Asset.Kind {
	case contract.AssetImage:
		if value.Version.MIMEType != "image/png" && value.Version.MIMEType != "image/jpeg" {
			return ErrUnsupportedProfile
		}
	case contract.AssetVideo:
		durationMS := value.Version.DurationMS
		if durationMS == 0 && value.Version.Media.DurationSeconds > 0 {
			durationMS = int64(value.Version.Media.DurationSeconds * 1000)
		}
		if value.Version.MIMEType != "video/mp4" || value.Version.SizeBytes > assets.MaxVideoBytes || durationMS < 15_000 || durationMS > 90_000 {
			return ErrUnsupportedProfile
		}
	default:
		return ErrUnsupportedProfile
	}
	return nil
}

type visionCandidate struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	FrameIndex int     `json:"frame_index"`
}

type visionOutput struct {
	Summary      string            `json:"summary"`
	VisibleText  []visionCandidate `json:"visible_text"`
	Observations []visionCandidate `json:"observations"`
	Inferences   []visionCandidate `json:"inferences"`
	Risks        []visionCandidate `json:"risks"`
	Unknowns     []visionCandidate `json:"unknowns"`
}

func visionOutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false,"required":["summary","visible_text","observations","inferences","risks","unknowns"],"properties":{"summary":{"type":"string","maxLength":800},"visible_text":{"$ref":"#/$defs/items"},"observations":{"$ref":"#/$defs/items"},"inferences":{"$ref":"#/$defs/items"},"risks":{"$ref":"#/$defs/items"},"unknowns":{"$ref":"#/$defs/items"}},"$defs":{"items":{"type":"array","maxItems":24,"items":{"type":"object","additionalProperties":false,"required":["text","confidence","frame_index"],"properties":{"text":{"type":"string","maxLength":500},"confidence":{"type":"number","minimum":0,"maximum":1},"frame_index":{"type":"integer","minimum":0,"maximum":7}}}}}}`)
}

func (s Service) modelAlias() string {
	if value := strings.TrimSpace(s.ModelAlias); value != "" {
		return value
	}
	return "cookies.vision.standard"
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
