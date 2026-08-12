package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const CreativeHandoffContractVersion = "strategy-creative-handoff/v1"

type CreativeHandoff struct {
	ContractVersion    string                  `json:"contract_version"`
	OrganizationID     contract.OrganizationID `json:"organization_id,omitempty"`
	ProjectID          contract.ProjectID      `json:"project_id"`
	PackageRef         CreativePackageRef      `json:"package_ref"`
	HandoffContentHash contract.ContentHash    `json:"handoff_content_hash"`
	CreativeView       CreativeView            `json:"creative_view"`
	Routes             []CreativeHandoffRoute  `json:"routes"`
	UpstreamReadiness  HandoffReadiness        `json:"upstream_readiness"`
	PublishedAt        time.Time               `json:"published_at"`
}

type CreativePackageRef struct {
	PackageID          string               `json:"package_id"`
	PackageVersion     int64                `json:"package_version"`
	PackageContentHash contract.ContentHash `json:"package_content_hash"`
	ApprovedAt         time.Time            `json:"approved_at"`
}

type CreativeView struct {
	Market             string                `json:"market"`
	Language           string                `json:"language"`
	Objective          CreativeObjective     `json:"objective"`
	AudienceSegments   []CreativeAudience    `json:"audience_segments"`
	ProductAndOffer    CreativeProductOffer  `json:"product_and_offer"`
	Communication      CreativeCommunication `json:"communication"`
	Guardrails         []CreativeGuardrail   `json:"guardrails"`
	Claims             []CreativeClaim       `json:"claims"`
	Assets             []CreativeAsset       `json:"assets"`
	CreativeHypotheses []CreativeHypothesis  `json:"creative_hypotheses"`
	OpenQuestions      []HandoffIssue        `json:"open_questions"`
	SourceRefs         []CreativeSourceRef   `json:"source_refs"`
}

type CreativeObjective struct {
	ObjectiveType  string   `json:"objective_type"`
	Statement      string   `json:"statement"`
	SuccessSignals []string `json:"success_signals"`
}

type CreativeAudience struct {
	SegmentID      string   `json:"segment_id"`
	Label          string   `json:"label"`
	Priority       int      `json:"priority"`
	Insight        string   `json:"insight"`
	Tension        string   `json:"tension"`
	EvidenceRefIDs []string `json:"evidence_ref_ids"`
}

type CreativeProductOffer struct {
	ProductRefIDs      []string `json:"product_ref_ids"`
	CampaignMechanism  string   `json:"campaign_mechanism"`
	OfferText          string   `json:"offer_text"`
	LandingDestination string   `json:"landing_destination"`
}

type CreativeCommunication struct {
	SingleMindedProposition string            `json:"single_minded_proposition"`
	MessageHierarchy        []CreativeMessage `json:"message_hierarchy"`
	CTAIntent               string            `json:"cta_intent"`
	ApprovedCTAs            []string          `json:"approved_ctas"`
	ToneConstraints         []string          `json:"tone_constraints"`
}

type CreativeMessage struct {
	Priority       int      `json:"priority"`
	Message        string   `json:"message"`
	EvidenceRefIDs []string `json:"evidence_ref_ids"`
}

type CreativeGuardrail struct {
	GuardrailID  string   `json:"guardrail_id"`
	Kind         string   `json:"kind"`
	Severity     string   `json:"severity"`
	Scope        string   `json:"scope"`
	Text         string   `json:"text"`
	SourceRefIDs []string `json:"source_ref_ids"`
}

type CreativeClaim struct {
	ClaimID            string                `json:"claim_id"`
	ApprovedText       string                `json:"approved_text"`
	ParaphrasePolicy   string                `json:"paraphrase_policy"`
	EvidenceRefIDs     []string              `json:"evidence_ref_ids"`
	RequiredDisclaimer string                `json:"required_disclaimer"`
	Validity           CreativeClaimValidity `json:"validity"`
}

type CreativeClaimValidity struct {
	Markets   []string `json:"markets"`
	Channels  []string `json:"channels"`
	ValidFrom string   `json:"valid_from,omitempty"`
	ValidTo   string   `json:"valid_to,omitempty"`
}

type CreativeAsset struct {
	AssetRef CreativeAssetRef    `json:"asset_ref"`
	Role     string              `json:"role"`
	Rights   CreativeAssetRights `json:"rights"`
}

type CreativeAssetRef struct {
	AssetID string `json:"asset_id"`
	Version int64  `json:"version"`
}

type CreativeAssetRights struct {
	Status                string     `json:"status"`
	GenerativeAIAllowed   bool       `json:"generative_ai_allowed"`
	DerivativeWorkAllowed bool       `json:"derivative_work_allowed"`
	AllowedChannels       []string   `json:"allowed_channels"`
	Territories           []string   `json:"territories"`
	ValidUntil            *time.Time `json:"valid_until"`
}

type CreativeHypothesis struct {
	HypothesisID string   `json:"hypothesis_id"`
	Statement    string   `json:"statement"`
	Variable     string   `json:"variable"`
	Variants     []string `json:"variants"`
	MetricRefIDs []string `json:"metric_ref_ids"`
}

type CreativeSourceRef struct {
	RefID       string               `json:"ref_id"`
	RefType     string               `json:"ref_type"`
	Producer    string               `json:"producer"`
	ResourceURI string               `json:"resource_uri"`
	Version     string               `json:"version"`
	ContentHash contract.ContentHash `json:"content_hash"`
	ObservedAt  time.Time            `json:"observed_at"`
}

type CreativeHandoffRoute struct {
	RouteID           string                     `json:"route_id"`
	DeliverableType   string                     `json:"deliverable_type"`
	Purpose           string                     `json:"purpose"`
	PerformanceMode   string                     `json:"performance_mode,omitempty"`
	Channels          []string                   `json:"channels"`
	Reason            string                     `json:"reason"`
	Spec              CreativeRouteSpec          `json:"spec"`
	CTAPolicy         CreativeCTAPolicy          `json:"cta_policy"`
	ClaimRefs         []string                   `json:"claim_refs"`
	AssetRequirements []CreativeAssetRequirement `json:"asset_requirements"`
	AssetRefs         []string                   `json:"asset_refs"`
	RouteReadiness    HandoffReadiness           `json:"route_readiness"`
}

type CreativeRouteSpec struct {
	TargetDurationSeconds int     `json:"target_duration_seconds,omitempty"`
	AspectRatio           string  `json:"aspect_ratio"`
	Resolution            string  `json:"resolution"`
	HookDeadlineSeconds   float64 `json:"hook_deadline_seconds,omitempty"`
	CompositionRequired   bool    `json:"composition_required"`
}

type CreativeCTAPolicy struct {
	RequiredForGeneration bool   `json:"required_for_generation"`
	RequiredForDelivery   bool   `json:"required_for_delivery"`
	CTAIntent             string `json:"cta_intent"`
}

type CreativeAssetRequirement struct {
	Role          string `json:"role"`
	RequiredStage string `json:"required_stage"`
}

type HandoffReadiness struct {
	Status   string         `json:"status"`
	Blockers []HandoffIssue `json:"blockers"`
	Warnings []HandoffIssue `json:"warnings"`
}

type HandoffIssue struct {
	Code         string   `json:"code"`
	Stage        string   `json:"stage"`
	Path         string   `json:"path"`
	Message      string   `json:"message"`
	Source       string   `json:"source"`
	SourceRefIDs []string `json:"source_ref_ids,omitempty"`
}

// BuildCreativeHandoff creates the immutable Creative-facing projection at
// approval time. It deliberately leaves information absent when the approved
// Strategy package cannot prove it, and records blockers instead of inventing
// routes, asset rights, claims, CTA copy, or experiment variants.
func BuildCreativeHandoff(value PackageVersion, productIDs []contract.ProductID) (CreativeHandoff, error) {
	snapshot := value.Snapshot
	briefRefID := "client_brief_" + snapshot.Brief.BriefID + "_v" + strconv.FormatInt(snapshot.Brief.Version, 10)
	sourceRefIDs := []string{briefRefID}
	blockers := make([]HandoffIssue, 0)
	warnings := make([]HandoffIssue, 0)
	openQuestions := make([]HandoffIssue, 0, len(snapshot.Strategy.AssumptionsAndGaps))

	objectiveType, objectiveTypeKnown := creativeObjectiveType(snapshot)
	if !objectiveTypeKnown {
		blockers = append(blockers, HandoffIssue{
			Code: "objective_type_missing", Stage: "planning",
			Path:    "creative_view.objective.objective_type",
			Message: "现有 StrategyPackage 未明确区分品牌或效果目标。",
			Source:  "strategy", SourceRefIDs: sourceRefIDs,
		})
	} else {
		warnings = append(warnings, HandoffIssue{
			Code: "objective_type_derived", Stage: "planning",
			Path:    "creative_view.objective.objective_type",
			Message: "objective type 由已批准的渠道角色派生；后续 Strategy 版本应改为显式结构化字段。",
			Source:  "strategy", SourceRefIDs: sourceRefIDs,
		})
	}

	audiences := make([]CreativeAudience, 0, 1)
	if audience := strings.TrimSpace(snapshot.Strategy.Audience.Primary); audience != "" {
		audiences = append(audiences, CreativeAudience{
			SegmentID: "audience_primary", Label: audience, Priority: 1,
			Insight:        strings.Join(nonEmptyStrings(snapshot.Strategy.Audience.Insights), "；"),
			Tension:        strings.Join(nonEmptyStrings(snapshot.Brief.Snapshot.Audience.PainPoints), "；"),
			EvidenceRefIDs: []string{},
		})
	} else {
		issue := HandoffIssue{
			Code: "audience_missing", Stage: "planning", Path: "creative_view.audience_segments",
			Message: "至少需要一个有明确优先级的受众分群。",
			Source:  "strategy", SourceRefIDs: sourceRefIDs,
		}
		blockers = append(blockers, issue)
		openQuestions = append(openQuestions, issue)
	}

	for index, question := range nonEmptyStrings(snapshot.Strategy.AssumptionsAndGaps) {
		openQuestions = append(openQuestions, HandoffIssue{
			Code: "strategy_open_question_" + strconv.Itoa(index+1), Stage: "planning",
			Path: "creative_view.open_questions", Message: question, Source: "strategy",
			SourceRefIDs: sourceRefIDs,
		})
	}

	messages := make([]CreativeMessage, 0)
	if len(nonEmptyStrings(snapshot.Strategy.CreativeDirections())) > 0 {
		warnings = append(warnings, HandoffIssue{
			Code: "message_hierarchy_missing", Stage: "planning",
			Path:    "creative_view.communication.message_hierarchy",
			Message: "创意建议未被当作传播信息层级；需要在 Strategy 中显式确认 message hierarchy。",
			Source:  "strategy", SourceRefIDs: sourceRefIDs,
		})
	}

	productRefs := productIDStrings(productIDs)
	guardrails := packageGuardrails(snapshot, briefRefID)
	routes, routeBlockers, routeWarnings := creativeHandoffRoutes(snapshot, objectiveType, objectiveTypeKnown, productRefs)
	blockers = append(blockers, routeBlockers...)
	warnings = append(warnings, routeWarnings...)
	if len(routes) == 0 {
		blockers = append(blockers, HandoffIssue{
			Code: "creative_route_missing", Stage: "planning", Path: "routes",
			Message: "至少需要一条具有稳定 ID 和完整规格的 Creative Route。",
			Source:  "strategy", SourceRefIDs: []string{},
		})
	}
	if strings.TrimSpace(snapshot.Brief.Snapshot.Region) == "" {
		blockers = append(blockers, HandoffIssue{
			Code: "market_missing", Stage: "planning", Path: "creative_view.market",
			Message: "Creative 规划需要明确市场。", Source: "strategy", SourceRefIDs: sourceRefIDs,
		})
	}
	if strings.TrimSpace(snapshot.Brief.Snapshot.Language) == "" {
		blockers = append(blockers, HandoffIssue{
			Code: "language_missing", Stage: "planning", Path: "creative_view.language",
			Message: "Creative 规划需要明确语言。", Source: "strategy", SourceRefIDs: sourceRefIDs,
		})
	}
	if strings.TrimSpace(snapshot.Strategy.Objective) == "" {
		blockers = append(blockers, HandoffIssue{
			Code: "objective_missing", Stage: "planning", Path: "creative_view.objective.statement",
			Message: "Creative 规划需要明确目标。", Source: "strategy", SourceRefIDs: sourceRefIDs,
		})
	}
	if strings.TrimSpace(snapshot.Strategy.Proposition) == "" {
		blockers = append(blockers, HandoffIssue{
			Code: "proposition_missing", Stage: "planning",
			Path:    "creative_view.communication.single_minded_proposition",
			Message: "Creative 规划需要明确单一核心主张。", Source: "strategy", SourceRefIDs: sourceRefIDs,
		})
	}
	if !snapshot.Readiness.CreativeReady {
		blockers = append(blockers, HandoffIssue{
			Code: "strategy_creative_not_ready", Stage: "planning",
			Path: "upstream_readiness", Message: "StrategyPackage 尚未达到创意交接就绪条件。",
			Source: "strategy", SourceRefIDs: sourceRefIDs,
		})
	}
	for _, problem := range snapshot.Readiness.PublishBlockers {
		blockers = append(blockers, HandoffIssue{
			Code: "strategy_publish_blocker", Stage: "planning",
			Path: nonEmptyOr(problem.Field, "strategy"), Message: nonEmptyOr(problem.Reason, "StrategyPackage 存在发布阻断项。"),
			Source: "strategy", SourceRefIDs: sourceRefIDs,
		})
	}

	readinessStatus := "ready"
	if len(blockers) > 0 {
		readinessStatus = "blocked"
	}
	handoff := CreativeHandoff{
		ContractVersion: CreativeHandoffContractVersion,
		OrganizationID:  value.OrganizationID,
		ProjectID:       value.ProjectID,
		PackageRef: CreativePackageRef{
			PackageID: value.PackageID, PackageVersion: value.Version,
			PackageContentHash: value.ContentHash, ApprovedAt: snapshot.Approval.ApprovedAt,
		},
		CreativeView: CreativeView{
			Market: snapshot.Brief.Snapshot.Region, Language: snapshot.Brief.Snapshot.Language,
			Objective: CreativeObjective{
				ObjectiveType: objectiveType, Statement: snapshot.Strategy.Objective,
				SuccessSignals: nonEmptyStrings(snapshot.Strategy.Measurement),
			},
			AudienceSegments: audiences,
			ProductAndOffer: CreativeProductOffer{
				ProductRefIDs: productRefs, CampaignMechanism: snapshot.Brief.Snapshot.Campaign.Objective,
				OfferText: "", LandingDestination: "",
			},
			Communication: CreativeCommunication{
				SingleMindedProposition: snapshot.Strategy.Proposition,
				MessageHierarchy:        messages, CTAIntent: "", ApprovedCTAs: []string{},
				ToneConstraints: nonEmptyStrings(snapshot.Brief.Snapshot.Creative.Tone),
			},
			Guardrails: guardrails, Claims: []CreativeClaim{}, Assets: []CreativeAsset{},
			CreativeHypotheses: []CreativeHypothesis{}, OpenQuestions: openQuestions,
			SourceRefs: []CreativeSourceRef{{
				RefID: briefRefID, RefType: "client_brief", Producer: "strategy",
				ResourceURI: fmt.Sprintf("/api/strategy/v1/briefs/%s/versions/%d", snapshot.Brief.BriefID, snapshot.Brief.Version),
				Version:     strconv.FormatInt(snapshot.Brief.Version, 10), ContentHash: snapshot.Brief.ContentHash,
				ObservedAt: snapshot.Brief.ConfirmedAt,
			}},
		},
		Routes: routes,
		UpstreamReadiness: HandoffReadiness{
			Status: readinessStatus, Blockers: blockers, Warnings: warnings,
		},
		// Approval and publication are one transaction. The approval timestamp
		// lives inside the immutable Package JSON with its original precision,
		// while the relational DATETIME(6) column may round it to microseconds.
		// Using the snapshot value keeps approval-time and backfill hashes equal.
		PublishedAt: snapshot.Approval.ApprovedAt,
	}
	hash, err := creativeHandoffContentHash(handoff)
	if err != nil {
		return CreativeHandoff{}, err
	}
	handoff.HandoffContentHash = hash
	if err := handoff.Validate(); err != nil {
		return CreativeHandoff{}, fmt.Errorf("%w: invalid creative handoff: %v", ErrInvalidRequest, err)
	}
	return handoff, nil
}

func creativeHandoffContentHash(value CreativeHandoff) (contract.ContentHash, error) {
	value.HandoffContentHash = ""
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	var hashInput map[string]any
	if err := json.Unmarshal(payload, &hashInput); err != nil {
		return "", err
	}
	delete(hashInput, "handoff_content_hash")
	return contract.NewContentHash(hashInput)
}

func (s Service) GetCreativeHandoff(ctx context.Context, actor contract.ActorContext, projectID contract.ProjectID, packageID string, version int64) (CreativeHandoff, error) {
	if err := requireScope(actor, ScopePackageRead); err != nil {
		return CreativeHandoff{}, err
	}
	if packageID == "" || version < 1 {
		return CreativeHandoff{}, ErrInvalidRequest
	}
	if _, err := s.project(ctx, actor, projectID); err != nil {
		return CreativeHandoff{}, err
	}
	var payload json.RawMessage
	var storedHash contract.ContentHash
	err := s.DB.QueryRowContext(ctx, `SELECT snapshot, content_hash
		FROM strategy_creative_handoffs WHERE organization_id = ? AND project_id = ?
		AND package_id = ? AND package_version = ?`,
		actor.OrganizationID, projectID, packageID, version).Scan(&payload, &storedHash)
	if err != nil {
		return CreativeHandoff{}, mapNotFound(err)
	}
	var value CreativeHandoff
	if err := json.Unmarshal(payload, &value); err != nil {
		return CreativeHandoff{}, err
	}
	if err := value.Validate(); err != nil {
		return CreativeHandoff{}, fmt.Errorf("%w: invalid creative handoff snapshot: %v", ErrInvalidState, err)
	}
	if !storedHash.Equal(value.HandoffContentHash) {
		return CreativeHandoff{}, fmt.Errorf("%w: stored creative handoff hash mismatch", ErrInvalidState)
	}
	calculated, err := creativeHandoffContentHash(value)
	if err != nil {
		return CreativeHandoff{}, err
	}
	if !calculated.Equal(value.HandoffContentHash) {
		return CreativeHandoff{}, fmt.Errorf("%w: creative handoff snapshot hash mismatch", ErrInvalidState)
	}
	return value, nil
}

func (value CreativeHandoff) Validate() error {
	if value.ContractVersion != CreativeHandoffContractVersion {
		return fmt.Errorf("contract_version must be %q", CreativeHandoffContractVersion)
	}
	if strings.TrimSpace(string(value.ProjectID)) == "" || strings.TrimSpace(value.PackageRef.PackageID) == "" ||
		value.PackageRef.PackageVersion < 1 || value.PackageRef.ApprovedAt.IsZero() || value.PublishedAt.IsZero() {
		return fmt.Errorf("project, package reference, approval time, and published time are required")
	}
	if err := value.PackageRef.PackageContentHash.Validate(); err != nil {
		return fmt.Errorf("package content hash: %w", err)
	}
	if err := value.HandoffContentHash.Validate(); err != nil {
		return fmt.Errorf("handoff content hash: %w", err)
	}
	sourceRefs, err := validateCreativeView(value.CreativeView)
	if err != nil {
		return err
	}
	if value.Routes == nil {
		return fmt.Errorf("routes must be an array")
	}
	routeIDs := map[string]bool{}
	for index, route := range value.Routes {
		if err := validateCreativeRoute(route, sourceRefs); err != nil {
			return fmt.Errorf("routes[%d]: %w", index, err)
		}
		if routeIDs[route.RouteID] {
			return fmt.Errorf("routes[%d].route_id is duplicated", index)
		}
		routeIDs[route.RouteID] = true
	}
	if err := validateHandoffReadiness("upstream_readiness", value.UpstreamReadiness, sourceRefs); err != nil {
		return err
	}
	return nil
}

func validateCreativeView(value CreativeView) (map[string]bool, error) {
	if err := validateMaxLength("creative_view.market", value.Market, 100); err != nil {
		return nil, err
	}
	if err := validateMaxLength("creative_view.language", value.Language, 100); err != nil {
		return nil, err
	}
	if !oneOf(value.Objective.ObjectiveType, "brand", "performance", "mixed") {
		return nil, fmt.Errorf("creative_view.objective.objective_type is invalid")
	}
	if err := validateMaxLength("creative_view.objective.statement", value.Objective.Statement, 1000); err != nil {
		return nil, err
	}
	if err := validateStringList("creative_view.objective.success_signals", value.Objective.SuccessSignals, false); err != nil {
		return nil, err
	}
	if value.AudienceSegments == nil {
		return nil, fmt.Errorf("creative_view.audience_segments must be an array")
	}
	audienceIDs := map[string]bool{}
	for index, audience := range value.AudienceSegments {
		if strings.TrimSpace(audience.SegmentID) == "" || strings.TrimSpace(audience.Label) == "" || audience.Priority < 1 {
			return nil, fmt.Errorf("creative_view.audience_segments[%d] has an invalid identity or priority", index)
		}
		if audienceIDs[audience.SegmentID] {
			return nil, fmt.Errorf("creative_view.audience_segments[%d].segment_id is duplicated", index)
		}
		audienceIDs[audience.SegmentID] = true
		if err := validateMaxLength("audience insight", audience.Insight, 1000); err != nil {
			return nil, err
		}
		if err := validateMaxLength("audience tension", audience.Tension, 1000); err != nil {
			return nil, err
		}
		if err := validateStringList("audience evidence_ref_ids", audience.EvidenceRefIDs, false); err != nil {
			return nil, err
		}
	}
	if err := validateStringList("creative_view.product_and_offer.product_ref_ids", value.ProductAndOffer.ProductRefIDs, false); err != nil {
		return nil, err
	}
	for field, candidate := range map[string]string{
		"creative_view.product_and_offer.campaign_mechanism":    value.ProductAndOffer.CampaignMechanism,
		"creative_view.product_and_offer.offer_text":            value.ProductAndOffer.OfferText,
		"creative_view.communication.single_minded_proposition": value.Communication.SingleMindedProposition,
	} {
		if err := validateMaxLength(field, candidate, 1000); err != nil {
			return nil, err
		}
	}
	if err := validateMaxLength("creative_view.product_and_offer.landing_destination", value.ProductAndOffer.LandingDestination, 2000); err != nil {
		return nil, err
	}
	if err := validateMaxLength("creative_view.communication.cta_intent", value.Communication.CTAIntent, 500); err != nil {
		return nil, err
	}
	if value.Communication.MessageHierarchy == nil {
		return nil, fmt.Errorf("creative_view.communication.message_hierarchy must be an array")
	}
	for index, message := range value.Communication.MessageHierarchy {
		if message.Priority < 1 || strings.TrimSpace(message.Message) == "" {
			return nil, fmt.Errorf("creative_view.communication.message_hierarchy[%d] is invalid", index)
		}
		if err := validateStringList("message evidence_ref_ids", message.EvidenceRefIDs, false); err != nil {
			return nil, err
		}
	}
	if err := validateStringList("creative_view.communication.approved_ctas", value.Communication.ApprovedCTAs, false); err != nil {
		return nil, err
	}
	if err := validateStringList("creative_view.communication.tone_constraints", value.Communication.ToneConstraints, false); err != nil {
		return nil, err
	}
	if value.Guardrails == nil || value.Claims == nil || value.Assets == nil || value.CreativeHypotheses == nil ||
		value.OpenQuestions == nil || value.SourceRefs == nil {
		return nil, fmt.Errorf("creative_view collection fields must be arrays")
	}
	sourceRefs := map[string]bool{}
	for index, ref := range value.SourceRefs {
		if strings.TrimSpace(ref.RefID) == "" || strings.TrimSpace(ref.Producer) == "" ||
			strings.TrimSpace(ref.ResourceURI) == "" || strings.TrimSpace(ref.Version) == "" ||
			!oneOf(ref.RefType, "client_material", "client_brief", "web_research", "prelaunch_insight", "brand_knowledge", "historical_experience") ||
			ref.ObservedAt.IsZero() {
			return nil, fmt.Errorf("creative_view.source_refs[%d] is invalid", index)
		}
		if sourceRefs[ref.RefID] {
			return nil, fmt.Errorf("creative_view.source_refs[%d].ref_id is duplicated", index)
		}
		if err := ref.ContentHash.Validate(); err != nil {
			return nil, fmt.Errorf("creative_view.source_refs[%d].content_hash: %w", index, err)
		}
		sourceRefs[ref.RefID] = true
	}
	for index, audience := range value.AudienceSegments {
		if err := validateReferences(fmt.Sprintf("creative_view.audience_segments[%d].evidence_ref_ids", index), audience.EvidenceRefIDs, sourceRefs); err != nil {
			return nil, err
		}
	}
	for index, message := range value.Communication.MessageHierarchy {
		if err := validateReferences(fmt.Sprintf("creative_view.communication.message_hierarchy[%d].evidence_ref_ids", index), message.EvidenceRefIDs, sourceRefs); err != nil {
			return nil, err
		}
	}
	for index, guardrail := range value.Guardrails {
		if strings.TrimSpace(guardrail.GuardrailID) == "" || strings.TrimSpace(guardrail.Text) == "" ||
			!oneOf(guardrail.Kind, "mandatory", "prohibited", "disclosure") ||
			!oneOf(guardrail.Severity, "blocker", "warning") ||
			!oneOf(guardrail.Scope, "global", "copy", "visual", "claim", "channel", "route") {
			return nil, fmt.Errorf("creative_view.guardrails[%d] is invalid", index)
		}
		if err := validateReferences(fmt.Sprintf("creative_view.guardrails[%d].source_ref_ids", index), guardrail.SourceRefIDs, sourceRefs); err != nil {
			return nil, err
		}
	}
	for index, claim := range value.Claims {
		if strings.TrimSpace(claim.ClaimID) == "" || strings.TrimSpace(claim.ApprovedText) == "" ||
			!oneOf(claim.ParaphrasePolicy, "exact_only", "limited", "free") {
			return nil, fmt.Errorf("creative_view.claims[%d] is invalid", index)
		}
		if err := validateMaxLength("claim required_disclaimer", claim.RequiredDisclaimer, 2000); err != nil {
			return nil, err
		}
		if err := validateReferences(fmt.Sprintf("creative_view.claims[%d].evidence_ref_ids", index), claim.EvidenceRefIDs, sourceRefs); err != nil {
			return nil, err
		}
		if err := validateStringList("claim validity markets", claim.Validity.Markets, false); err != nil {
			return nil, err
		}
		if err := validateStringList("claim validity channels", claim.Validity.Channels, false); err != nil {
			return nil, err
		}
		for _, date := range []string{claim.Validity.ValidFrom, claim.Validity.ValidTo} {
			if date != "" {
				if _, err := time.Parse("2006-01-02", date); err != nil {
					return nil, fmt.Errorf("creative_view.claims[%d] has an invalid validity date", index)
				}
			}
		}
	}
	for index, asset := range value.Assets {
		if strings.TrimSpace(asset.AssetRef.AssetID) == "" || asset.AssetRef.Version < 1 ||
			strings.TrimSpace(asset.Role) == "" ||
			!oneOf(asset.Rights.Status, "verified", "pending", "restricted", "expired") {
			return nil, fmt.Errorf("creative_view.assets[%d] is invalid", index)
		}
		if err := validateStringList("asset rights allowed_channels", asset.Rights.AllowedChannels, false); err != nil {
			return nil, err
		}
		if err := validateStringList("asset rights territories", asset.Rights.Territories, false); err != nil {
			return nil, err
		}
	}
	for index, hypothesis := range value.CreativeHypotheses {
		if strings.TrimSpace(hypothesis.HypothesisID) == "" || strings.TrimSpace(hypothesis.Statement) == "" ||
			strings.TrimSpace(hypothesis.Variable) == "" || len(hypothesis.Variants) < 2 {
			return nil, fmt.Errorf("creative_view.creative_hypotheses[%d] is invalid", index)
		}
		if err := validateStringList("hypothesis variants", hypothesis.Variants, false); err != nil {
			return nil, err
		}
		if err := validateStringList("hypothesis metric_ref_ids", hypothesis.MetricRefIDs, false); err != nil {
			return nil, err
		}
	}
	for index, issue := range value.OpenQuestions {
		if err := validateHandoffIssue(fmt.Sprintf("creative_view.open_questions[%d]", index), issue, sourceRefs); err != nil {
			return nil, err
		}
	}
	return sourceRefs, nil
}

func validateCreativeRoute(value CreativeHandoffRoute, sourceRefs map[string]bool) error {
	if strings.TrimSpace(value.RouteID) == "" ||
		!oneOf(value.DeliverableType, "image_text", "video") ||
		!oneOf(value.Purpose, "performance", "brand") ||
		len(value.Channels) == 0 {
		return fmt.Errorf("identity, deliverable_type, purpose, and channels are required")
	}
	if err := validateMaxLength("route reason", value.Reason, 1000); err != nil {
		return err
	}
	if err := validateEnumList("route channels", value.Channels, "xiaohongshu", "wechat_official_account", "douyin", "kuaishou"); err != nil {
		return err
	}
	if value.DeliverableType == "video" {
		if value.Spec.TargetDurationSeconds < 1 {
			return fmt.Errorf("video spec target_duration_seconds is required")
		}
		if value.Purpose == "performance" && !oneOf(value.PerformanceMode, "short_drama_preroll", "game_preroll", "commerce_preroll", "viral_remake") {
			return fmt.Errorf("performance video mode is invalid")
		}
		if value.Purpose == "brand" && value.PerformanceMode != "brand_video" {
			return fmt.Errorf("brand video mode must be brand_video")
		}
	} else if value.PerformanceMode != "" {
		return fmt.Errorf("image_text route must not carry performance_mode")
	}
	if strings.TrimSpace(value.Spec.AspectRatio) == "" || strings.TrimSpace(value.Spec.Resolution) == "" ||
		value.Spec.HookDeadlineSeconds < 0 {
		return fmt.Errorf("route spec is invalid")
	}
	if err := validateMaxLength("route cta intent", value.CTAPolicy.CTAIntent, 500); err != nil {
		return err
	}
	if err := validateStringList("route claim_refs", value.ClaimRefs, false); err != nil {
		return err
	}
	if err := validateStringList("route asset_refs", value.AssetRefs, false); err != nil {
		return err
	}
	if value.AssetRequirements == nil {
		return fmt.Errorf("route asset_requirements must be an array")
	}
	for index, requirement := range value.AssetRequirements {
		if strings.TrimSpace(requirement.Role) == "" || !oneOf(requirement.RequiredStage, "generation", "production") {
			return fmt.Errorf("route asset_requirements[%d] is invalid", index)
		}
	}
	return validateHandoffReadiness("route_readiness", value.RouteReadiness, sourceRefs)
}

func validateHandoffReadiness(path string, value HandoffReadiness, sourceRefs map[string]bool) error {
	if !oneOf(value.Status, "ready", "blocked") || value.Blockers == nil || value.Warnings == nil {
		return fmt.Errorf("%s is invalid", path)
	}
	if value.Status == "ready" && len(value.Blockers) > 0 {
		return fmt.Errorf("%s ready status cannot contain blockers", path)
	}
	if value.Status == "blocked" && len(value.Blockers) == 0 {
		return fmt.Errorf("%s blocked status requires blockers", path)
	}
	for index, issue := range value.Blockers {
		if err := validateHandoffIssue(fmt.Sprintf("%s.blockers[%d]", path, index), issue, sourceRefs); err != nil {
			return err
		}
	}
	for index, issue := range value.Warnings {
		if err := validateHandoffIssue(fmt.Sprintf("%s.warnings[%d]", path, index), issue, sourceRefs); err != nil {
			return err
		}
	}
	return nil
}

func validateHandoffIssue(path string, value HandoffIssue, sourceRefs map[string]bool) error {
	if strings.TrimSpace(value.Code) == "" || strings.TrimSpace(value.Path) == "" ||
		strings.TrimSpace(value.Message) == "" ||
		!oneOf(value.Stage, "planning", "generation", "production") ||
		!oneOf(value.Source, "strategy", "creative_validation") {
		return fmt.Errorf("%s is invalid", path)
	}
	return validateReferences(path+".source_ref_ids", value.SourceRefIDs, sourceRefs)
}

func validateReferences(path string, values []string, known map[string]bool) error {
	if err := validateStringList(path, values, true); err != nil {
		return err
	}
	for _, value := range values {
		if !known[value] {
			return fmt.Errorf("%s contains unknown source reference %q", path, value)
		}
	}
	return nil
}

func validateStringList(path string, values []string, optional bool) error {
	if values == nil {
		if optional {
			return nil
		}
		return fmt.Errorf("%s must be an array", path)
	}
	seen := map[string]bool{}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s entries must be non-empty", path)
		}
		if seen[value] {
			return fmt.Errorf("%s entries must be unique", path)
		}
		seen[value] = true
	}
	return nil
}

func validateEnumList(path string, values []string, allowed ...string) error {
	if err := validateStringList(path, values, false); err != nil {
		return err
	}
	for _, value := range values {
		if !oneOf(value, allowed...) {
			return fmt.Errorf("%s contains unsupported value %q", path, value)
		}
	}
	return nil
}

func validateMaxLength(path, value string, limit int) error {
	if utf8.RuneCountInString(value) > limit {
		return fmt.Errorf("%s exceeds %d characters", path, limit)
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func creativeObjectiveType(snapshot PackageSnapshot) (string, bool) {
	objectiveText := strings.ToLower(strings.Join(append(
		[]string{snapshot.Strategy.Objective, snapshot.Brief.Snapshot.Campaign.Objective},
		snapshot.Strategy.Measurement...,
	), " "))
	brand := containsAny(objectiveText,
		"brand", "awareness", "品牌", "种草", "心智", "产品认知", "品牌资产")
	performance := containsAny(objectiveText,
		"performance", "conversion", "转化", "获客", "留资", "线索", "成交", "销售", "购买", "安装", "注册")
	if brand || performance {
		switch {
		case brand && performance:
			return "mixed", true
		case performance:
			return "performance", true
		default:
			return "brand", true
		}
	}
	for _, channel := range snapshot.Strategy.ChannelStrategy {
		role := strings.ToLower(strings.TrimSpace(channel.Role))
		if containsAny(role, "brand", "awareness", "品牌", "认知", "种草", "心智") {
			brand = true
		}
		if containsAny(role, "performance", "conversion", "转化", "引流", "成交", "销售", "效果") {
			performance = true
		}
	}
	switch {
	case brand && performance:
		return "mixed", true
	case performance:
		return "performance", true
	case brand:
		return "brand", true
	default:
		return "mixed", false
	}
}

func packageGuardrails(snapshot PackageSnapshot, briefRefID string) []CreativeGuardrail {
	result := make([]CreativeGuardrail, 0,
		len(snapshot.Brief.Snapshot.Creative.MandatoryElements)+
			len(snapshot.Brief.Snapshot.Creative.ProhibitedClaims)+len(snapshot.Strategy.Constraints))
	appendValues := func(values []string, prefix, kind, severity, scope string, sourceRefIDs []string) {
		for _, value := range nonEmptyStrings(values) {
			result = append(result, CreativeGuardrail{
				GuardrailID: prefix + "_" + strconv.Itoa(len(result)+1), Kind: kind,
				Severity: severity, Scope: scope, Text: value, SourceRefIDs: sourceRefIDs,
			})
		}
	}
	appendValues(snapshot.Brief.Snapshot.Creative.MandatoryElements, "mandatory", "mandatory", "blocker", "global", []string{briefRefID})
	appendValues(snapshot.Brief.Snapshot.Creative.ProhibitedClaims, "prohibited_claim", "prohibited", "blocker", "claim", []string{briefRefID})
	appendValues(snapshot.Strategy.Constraints, "strategy_constraint", "mandatory", "warning", "global", []string{})
	return result
}

func creativeHandoffRoutes(snapshot PackageSnapshot, objectiveType string, objectiveTypeKnown bool, productRefs []string) ([]CreativeHandoffRoute, []HandoffIssue, []HandoffIssue) {
	routes := make([]CreativeHandoffRoute, 0)
	blockers := make([]HandoffIssue, 0)
	warnings := make([]HandoffIssue, 0)
	seen := map[string]bool{}
	brandVideoChannels := make([]string, 0)
	ambiguousImageRoutes := make([]HandoffIssue, 0)
	legacyPerformanceChannels := map[string]bool{}
	for _, route := range snapshot.CreativeRoutes {
		if route.RouteType != "pre_roll" || route.VideoPurpose != "performance" {
			continue
		}
		for _, channel := range route.Channels {
			legacyPerformanceChannels[strings.ToLower(strings.TrimSpace(channel))] = true
		}
	}
	for _, channel := range snapshot.Strategy.ChannelStrategy {
		platform := strings.ToLower(strings.TrimSpace(channel.Platform))
		if legacyPerformanceChannels[platform] && oneOf(platform, "douyin", "kuaishou") && containsVideoFormat(channel.Formats) {
			purpose, known := creativeRoutePurpose(channel.Role)
			if !known && objectiveTypeKnown && objectiveType == "performance" {
				purpose, known = objectiveType, true
			}
			if known && purpose == "performance" && creativeCommerceObjective(snapshot) {
				routeBlockers := make([]HandoffIssue, 0)
				routeWarnings := []HandoffIssue{{
					Code: "cta_missing", Stage: "production",
					Path:    "routes.route_" + platform + "_commerce_preroll.cta_policy.cta_intent",
					Message: "效果 Route 需要在任务计划中显式确认 CTA intent。", Source: "strategy", SourceRefIDs: []string{},
				}}
				if len(productRefs) == 0 {
					routeBlockers = append(routeBlockers, HandoffIssue{
						Code: "product_missing", Stage: "generation",
						Path:    "creative_view.product_and_offer.product_ref_ids",
						Message: "电商前贴 Route 需要明确产品引用。", Source: "strategy", SourceRefIDs: []string{},
					})
				}
				routeStatus := "ready"
				if len(routeBlockers) > 0 {
					routeStatus = "blocked"
					blockers = append(blockers, routeBlockers...)
				}
				warnings = append(warnings, routeWarnings...)
				routes = append(routes, CreativeHandoffRoute{
					RouteID: "route_" + platform + "_commerce_preroll", DeliverableType: "video",
					Purpose: "performance", PerformanceMode: "commerce_preroll", Channels: []string{platform},
					Reason: creativeRouteReason(snapshot.Strategy, platform),
					Spec: CreativeRouteSpec{
						TargetDurationSeconds: 6, AspectRatio: "9:16", Resolution: "720p",
						HookDeadlineSeconds: 1, CompositionRequired: true,
					},
					CTAPolicy: CreativeCTAPolicy{RequiredForGeneration: false, RequiredForDelivery: true},
					ClaimRefs: []string{}, AssetRequirements: []CreativeAssetRequirement{
						{Role: "product_image", RequiredStage: "generation"},
						{Role: "main_video", RequiredStage: "production"},
					},
					AssetRefs: []string{}, RouteReadiness: HandoffReadiness{
						Status: routeStatus, Blockers: routeBlockers, Warnings: routeWarnings,
					},
				})
			}
		}
		if supportsBrandVideoPlatform(platform) && containsVideoFormat(channel.Formats) {
			purpose, known := creativeRoutePurpose(channel.Role)
			if !known && objectiveTypeKnown && objectiveType != "mixed" {
				purpose, known = objectiveType, true
			}
			roleHasBrand, _ := creativeRouteSignals(channel.Role)
			if roleHasBrand || (known && purpose == "brand") || creativeHandoffHasBrandGoal(snapshot) {
				brandVideoChannels = appendUnique(brandVideoChannels, platform)
			}
		}
		if platform != "xiaohongshu" || seen[platform] || !containsImageTextFormat(channel.Formats) {
			continue
		}
		purpose, known := creativeRoutePurpose(channel.Role)
		if !known {
			if objectiveTypeKnown && objectiveType != "mixed" {
				purpose, known = objectiveType, true
			}
		}
		if !known {
			ambiguousImageRoutes = append(ambiguousImageRoutes, HandoffIssue{
				Code: "route_purpose_missing", Stage: "planning", Path: "routes",
				Message: "小红书图文 Route 缺少明确的品牌或效果目的。",
				Source:  "strategy", SourceRefIDs: []string{},
			})
			continue
		}
		seen[platform] = true
		routeBlockers := make([]HandoffIssue, 0)
		routeWarnings := make([]HandoffIssue, 0)
		ctaPolicy := CreativeCTAPolicy{
			RequiredForGeneration: false, RequiredForDelivery: false, CTAIntent: "",
		}
		if purpose == "performance" {
			if len(productRefs) == 0 {
				routeBlockers = append(routeBlockers, HandoffIssue{
					Code: "product_missing", Stage: "generation",
					Path:    "creative_view.product_and_offer.product_ref_ids",
					Message: "效果 Route 需要明确产品引用。", Source: "strategy", SourceRefIDs: []string{},
				})
			}
			routeWarnings = append(routeWarnings, HandoffIssue{
				Code: "cta_missing", Stage: "generation",
				Path:    "routes.route_xiaohongshu_image_text.cta_policy.cta_intent",
				Message: "效果 Route 需要在任务计划中显式确认 CTA intent。", Source: "strategy", SourceRefIDs: []string{},
			})
			ctaPolicy.RequiredForGeneration = true
			ctaPolicy.RequiredForDelivery = true
		}
		routeStatus := "ready"
		if len(routeBlockers) > 0 {
			routeStatus = "blocked"
			blockers = append(blockers, routeBlockers...)
		}
		warnings = append(warnings, routeWarnings...)
		routes = append(routes, CreativeHandoffRoute{
			RouteID: "route_xiaohongshu_image_text", DeliverableType: "image_text",
			Purpose: purpose, Channels: []string{"xiaohongshu"},
			Reason: creativeRouteReason(snapshot.Strategy, platform),
			Spec: CreativeRouteSpec{
				AspectRatio: "3:4", Resolution: "1080x1440", CompositionRequired: false,
			},
			CTAPolicy: ctaPolicy,
			ClaimRefs: []string{}, AssetRequirements: []CreativeAssetRequirement{},
			AssetRefs: []string{}, RouteReadiness: HandoffReadiness{
				Status: routeStatus, Blockers: routeBlockers, Warnings: routeWarnings,
			},
		})
	}
	if len(brandVideoChannels) > 0 {
		routes = append(routes, CreativeHandoffRoute{
			RouteID: "route_brand_video", DeliverableType: "video", Purpose: "brand",
			PerformanceMode: "brand_video", Channels: brandVideoChannels,
			Reason: creativeBrandVideoReason(snapshot.Strategy, brandVideoChannels),
			Spec: CreativeRouteSpec{
				TargetDurationSeconds: 30, AspectRatio: "9:16", Resolution: "1080x1920",
				HookDeadlineSeconds: 3, CompositionRequired: true,
			},
			CTAPolicy: CreativeCTAPolicy{}, ClaimRefs: []string{},
			AssetRequirements: []CreativeAssetRequirement{
				{Role: "brand_identity", RequiredStage: "production"},
				{Role: "product_visuals", RequiredStage: "production"},
				{Role: "music_and_voice_rights", RequiredStage: "production"},
			},
			AssetRefs: []string{}, RouteReadiness: HandoffReadiness{
				Status: "ready", Blockers: []HandoffIssue{}, Warnings: []HandoffIssue{},
			},
		})
	}
	// An ambiguous optional image route must not make an independently valid
	// brand-video route unusable. Keep the diagnosis visible as a warning; it
	// remains a blocker when no executable route can be frozen at all.
	if len(ambiguousImageRoutes) > 0 {
		if len(routes) > 0 {
			warnings = append(warnings, ambiguousImageRoutes...)
		} else {
			blockers = append(blockers, ambiguousImageRoutes...)
		}
	}
	if len(snapshot.CreativeRoutes) > 0 {
		issue := HandoffIssue{
			Code: "creative_route_mode_missing", Stage: "planning", Path: "routes",
			Message: "旧版 pre_roll Route 没有冻结契约要求的稳定 Route ID 和 performance mode。",
			Source:  "strategy", SourceRefIDs: []string{},
		}
		if len(routes) == 0 {
			blockers = append(blockers, issue)
		} else {
			issue.Code = "creative_route_legacy_ignored"
			issue.Message = "旧版 pre_roll Route 未迁移；已冻结的新 Route 仍可独立交接。"
			warnings = append(warnings, issue)
		}
	}
	return routes, blockers, warnings
}

func creativeRoutePurpose(role string) (string, bool) {
	brand, performance := creativeRouteSignals(role)
	switch {
	case brand && !performance:
		return "brand", true
	case performance && !brand:
		return "performance", true
	default:
		return "", false
	}
}

func creativeRouteSignals(role string) (brand bool, performance bool) {
	role = strings.ToLower(strings.TrimSpace(role))
	return containsAny(role, "brand", "awareness", "品牌", "认知", "种草", "心智", "产品认知"),
		containsAny(role, "performance", "conversion", "转化", "引流", "获客", "留资", "成交", "销售", "效果")
}

func creativeRouteReason(document StrategyDocument, platform string) string {
	for _, plan := range document.PlatformPlans {
		if strings.EqualFold(strings.TrimSpace(plan.Platform), platform) &&
			strings.TrimSpace(plan.Role) != "" {
			return strings.TrimSpace(plan.Role)
		}
	}
	for _, channel := range document.ChannelStrategy {
		if strings.EqualFold(strings.TrimSpace(channel.Platform), platform) &&
			strings.TrimSpace(channel.Role) != "" {
			return strings.TrimSpace(channel.Role)
		}
	}
	return strings.TrimSpace(document.Proposition)
}

func containsImageTextFormat(formats []string) bool {
	for _, format := range formats {
		normalized := strings.ToLower(strings.TrimSpace(format))
		if strings.Contains(normalized, "图文") || strings.Contains(normalized, "image_text") ||
			strings.Contains(normalized, "image-text") || strings.Contains(normalized, "image text") ||
			strings.Contains(normalized, "图片") || strings.Contains(normalized, "配图") ||
			strings.Contains(normalized, "长图") || strings.Contains(normalized, "海报") ||
			strings.Contains(normalized, "三联图") || strings.Contains(normalized, "对比图") ||
			strings.Contains(normalized, "步骤图") || strings.Contains(normalized, "实拍图") ||
			strings.Contains(normalized, "carousel") || strings.Contains(normalized, "poster") ||
			strings.Contains(normalized, "static image") || strings.Contains(normalized, "photo post") {
			return true
		}
	}
	return false
}

func containsVideoFormat(formats []string) bool {
	for _, format := range formats {
		normalized := strings.ToLower(strings.TrimSpace(format))
		if strings.Contains(normalized, "视频") || strings.Contains(normalized, "video") ||
			strings.Contains(normalized, "短片") || strings.Contains(normalized, "tvc") {
			return true
		}
	}
	return false
}

func supportsBrandVideoPlatform(platform string) bool {
	return oneOf(platform, "xiaohongshu", "wechat_official_account", "douyin", "kuaishou")
}

func creativeHandoffHasBrandGoal(snapshot PackageSnapshot) bool {
	objective := strings.Join([]string{
		snapshot.Brief.Snapshot.Campaign.Objective,
		snapshot.Strategy.Objective,
		snapshot.Strategy.CrossPlatformRole,
	}, " ")
	return containsAny(strings.ToLower(objective),
		"品牌", "品牌广告", "品牌认知", "品牌心智", "品牌资产", "产品认知", "种草", "brand", "awareness")
}

func creativeCommerceObjective(snapshot PackageSnapshot) bool {
	objective := strings.Join(append([]string{
		snapshot.Brief.Snapshot.Campaign.Objective,
		snapshot.Brief.Snapshot.Product.Name,
		snapshot.Brief.Snapshot.Product.Category,
		snapshot.Strategy.Objective,
		snapshot.Strategy.Proposition,
	}, snapshot.Strategy.Measurement...), " ")
	normalized := strings.ToLower(objective)
	if containsAny(normalized,
		"game", "gaming", "游戏", "安装", "预约", "召回",
		"short drama", "短剧", "正片",
		"viral remake", "remake", "爆款复刻", "内容复刻") {
		return false
	}
	return containsAny(normalized,
		"commerce", "conversion", "purchase", "sales", "电商", "购买", "成交", "销售", "转化")
}

func creativeBrandVideoReason(document StrategyDocument, channels []string) string {
	reasons := make([]string, 0, len(channels))
	for _, channel := range channels {
		if reason := creativeRouteReason(document, channel); reason != "" {
			reasons = append(reasons, reason)
		}
	}
	if len(reasons) == 0 {
		return strings.TrimSpace(document.Proposition)
	}
	return strings.Join(reasons, "；")
}

func productIDStrings(values []contract.ProductID) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if normalized := strings.TrimSpace(string(value)); normalized != "" {
			result = append(result, normalized)
		}
	}
	return nonEmptyStrings(result)
}

func nonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func nonEmptyOr(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
