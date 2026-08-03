package creative

import (
	"fmt"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const (
	PerformanceModeBrandFilm = "brand_video"
	ManualBrandFilmRouteID   = "route_fixture_brand_video_guerlain_v1"
	GuerlainBrandFixtureID   = "brand-video-guerlain/v1"
)

type BrandFilmStage string

const (
	BrandFilmWaitingBrief     BrandFilmStage = "waiting_for_input"
	BrandFilmBriefDraft       BrandFilmStage = "brief_analysis_draft"
	BrandFilmBriefConfirmed   BrandFilmStage = "brief_confirmed"
	BrandFilmConceptSelection BrandFilmStage = "concept_selection"
	BrandFilmConceptConfirmed BrandFilmStage = "concept_confirmed"
	BrandFilmPlanDraft        BrandFilmStage = "production_plan_draft"
	BrandFilmPlanConfirmed    BrandFilmStage = "production_plan_confirmed"
)

type ManualBrandFilmInput struct {
	FixtureID      string `json:"fixture_id"`
	FixtureVersion int64  `json:"fixture_version"`
	FixtureHash    string `json:"fixture_hash"`
	BriefName      string `json:"brief_name"`
	BriefText      string `json:"brief_text"`
	ProductName    string `json:"product_name"`
}

func (i ManualBrandFilmInput) Validate() error {
	if i.FixtureID != GuerlainBrandFixtureID || i.FixtureVersion != 1 ||
		!validSHA256Ref(i.FixtureHash) || strings.TrimSpace(i.BriefName) == "" ||
		strings.TrimSpace(i.BriefText) == "" || strings.TrimSpace(i.ProductName) == "" {
		return fmt.Errorf("manual brand film fixture input is incomplete")
	}
	return nil
}

type BrandFilmSourceSnapshot struct {
	FixtureID      string   `json:"fixture_id"`
	FixtureVersion int64    `json:"fixture_version"`
	FixtureHash    string   `json:"fixture_hash"`
	BriefName      string   `json:"brief_name"`
	BriefText      string   `json:"brief_text"`
	ProductName    string   `json:"product_name"`
	Channel        string   `json:"channel"`
	Duration       int      `json:"duration_seconds"`
	AspectRatio    string   `json:"aspect_ratio"`
	EvidenceRefs   []string `json:"evidence_refs"`
}

type BrandBriefFact struct {
	Text       string  `json:"text"`
	Locator    string  `json:"locator"`
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status"`
}

type BrandBriefAssetCandidate struct {
	ID              string                    `json:"id"`
	Role            string                    `json:"role"`
	Label           string                    `json:"label"`
	SourceLocator   string                    `json:"source_locator"`
	FixtureURI      string                    `json:"fixture_uri,omitempty"`
	AssetRef        *contract.AssetVersionRef `json:"asset_ref,omitempty"`
	RightsStatus    string                    `json:"rights_status"`
	UserConfirmed   bool                      `json:"user_confirmed"`
	ReplacementNote string                    `json:"replacement_note,omitempty"`
}

type BrandBriefAnalysisVersion struct {
	Revision          int64                      `json:"revision"`
	Summary           string                     `json:"summary"`
	Audience          string                     `json:"audience"`
	CoreMessage       string                     `json:"core_message"`
	SellingPoints     []BrandBriefFact           `json:"selling_points"`
	Mandatory         []string                   `json:"mandatory_elements"`
	Prohibited        []string                   `json:"prohibited_claims"`
	ImageRequirements []string                   `json:"image_requirements"`
	VideoRequirements []string                   `json:"video_requirements"`
	VoiceDirection    string                     `json:"voice_direction"`
	AssetCandidates   []BrandBriefAssetCandidate `json:"asset_candidates"`
	Uncertainties     []string                   `json:"uncertainties"`
	Confirmed         bool                       `json:"confirmed"`
	ConfirmedBy       string                     `json:"confirmed_by,omitempty"`
	ConfirmedAt       *time.Time                 `json:"confirmed_at,omitempty"`
	ModelAlias        string                     `json:"model_alias"`
	ModelVersion      string                     `json:"model_version"`
	RouteRevisionID   string                     `json:"route_revision_id,omitempty"`
	PromptVersion     string                     `json:"prompt_version"`
	CreatedAt         time.Time                  `json:"created_at"`
}

func (v BrandBriefAnalysisVersion) Validate() error {
	if v.Revision < 1 || strings.TrimSpace(v.Summary) == "" || strings.TrimSpace(v.Audience) == "" ||
		strings.TrimSpace(v.CoreMessage) == "" || len(v.SellingPoints) == 0 || len(v.Mandatory) == 0 ||
		len(v.Prohibited) == 0 || strings.TrimSpace(v.VoiceDirection) == "" || v.CreatedAt.IsZero() {
		return fmt.Errorf("brand brief analysis is incomplete")
	}
	for _, fact := range v.SellingPoints {
		if strings.TrimSpace(fact.Text) == "" || strings.TrimSpace(fact.Locator) == "" ||
			fact.Confidence < 0 || fact.Confidence > 1 || (fact.Status != "brief_fact" && fact.Status != "needs_confirmation") {
			return fmt.Errorf("brand brief fact is invalid")
		}
	}
	return nil
}

type BrandCreativeConcept struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	OneLiner       string   `json:"one_liner"`
	StoryMechanism string   `json:"story_mechanism"`
	BrandEntrance  string   `json:"brand_entrance"`
	VisualLanguage []string `json:"visual_language"`
	SoundIdea      string   `json:"sound_idea"`
	BriefRationale string   `json:"brief_rationale"`
	Risk           string   `json:"risk"`
	Selected       bool     `json:"selected"`
	Confirmed      bool     `json:"confirmed"`
}

type BrandCreativeConceptSet struct {
	Revision         int64                  `json:"revision"`
	AnalysisRevision int64                  `json:"analysis_revision"`
	Candidates       []BrandCreativeConcept `json:"candidates"`
	ModelAlias       string                 `json:"model_alias"`
	ModelVersion     string                 `json:"model_version"`
	RouteRevisionID  string                 `json:"route_revision_id,omitempty"`
	PromptVersion    string                 `json:"prompt_version"`
	CreatedAt        time.Time              `json:"created_at"`
}

func (s BrandCreativeConceptSet) Validate() error {
	if s.Revision < 1 || s.AnalysisRevision < 1 || len(s.Candidates) < 2 || len(s.Candidates) > 3 || s.CreatedAt.IsZero() {
		return fmt.Errorf("brand concept set is incomplete")
	}
	seen := map[string]bool{}
	for _, candidate := range s.Candidates {
		if strings.TrimSpace(candidate.ID) == "" || strings.TrimSpace(candidate.Title) == "" ||
			strings.TrimSpace(candidate.StoryMechanism) == "" || strings.TrimSpace(candidate.BriefRationale) == "" || seen[candidate.ID] {
			return fmt.Errorf("brand concept candidate is invalid")
		}
		seen[candidate.ID] = true
	}
	return nil
}

type BrandFilmShot struct {
	ID              string `json:"id"`
	Order           int    `json:"order"`
	StartSecond     int    `json:"start_second"`
	EndSecond       int    `json:"end_second"`
	Purpose         string `json:"purpose"`
	Visual          string `json:"visual"`
	Action          string `json:"action"`
	Camera          string `json:"camera"`
	Lighting        string `json:"lighting"`
	Voiceover       string `json:"voiceover"`
	OnScreenText    string `json:"on_screen_text"`
	ReferenceRole   string `json:"reference_role"`
	ContinuityNotes string `json:"continuity_notes"`
}

type BrandFilmPlanVersion struct {
	Revision        int64           `json:"revision"`
	ConceptID       string          `json:"concept_id"`
	Title           string          `json:"title"`
	StorySummary    string          `json:"story_summary"`
	VoiceDirection  string          `json:"voice_direction"`
	MusicDirection  string          `json:"music_direction"`
	Shots           []BrandFilmShot `json:"shots"`
	Confirmed       bool            `json:"confirmed"`
	ConfirmedBy     string          `json:"confirmed_by,omitempty"`
	ConfirmedAt     *time.Time      `json:"confirmed_at,omitempty"`
	ModelAlias      string          `json:"model_alias"`
	ModelVersion    string          `json:"model_version"`
	RouteRevisionID string          `json:"route_revision_id,omitempty"`
	PromptVersion   string          `json:"prompt_version"`
	CreatedAt       time.Time       `json:"created_at"`
}

func (v BrandFilmPlanVersion) Validate() error {
	if v.Revision < 1 || strings.TrimSpace(v.ConceptID) == "" || strings.TrimSpace(v.Title) == "" ||
		strings.TrimSpace(v.StorySummary) == "" || strings.TrimSpace(v.VoiceDirection) == "" ||
		len(v.Shots) < 3 || v.CreatedAt.IsZero() {
		return fmt.Errorf("brand film plan is incomplete")
	}
	end := 0
	for index, shot := range v.Shots {
		if shot.Order != index+1 || shot.StartSecond != end || shot.EndSecond <= shot.StartSecond ||
			strings.TrimSpace(shot.ID) == "" || strings.TrimSpace(shot.Visual) == "" || strings.TrimSpace(shot.Purpose) == "" {
			return fmt.Errorf("brand film shot %d is invalid", index+1)
		}
		end = shot.EndSecond
	}
	if end != 15 {
		return fmt.Errorf("brand film plan must total 15 seconds")
	}
	return nil
}

type BrandFilmDraft struct {
	ContractVersion   string                          `json:"contract_version"`
	TaskID            string                          `json:"task_id"`
	Revision          int64                           `json:"revision"`
	Stage             BrandFilmStage                  `json:"stage"`
	SourceSnapshot    BrandFilmSourceSnapshot         `json:"source_snapshot"`
	SourceHash        string                          `json:"source_hash"`
	BriefAnalyses     []BrandBriefAnalysisVersion     `json:"brief_analysis_versions"`
	ConceptSets       []BrandCreativeConceptSet       `json:"concept_sets"`
	SelectedConceptID string                          `json:"selected_concept_id,omitempty"`
	FilmPlans         []BrandFilmPlanVersion          `json:"film_plan_versions"`
	Readiness         CreativeReadiness               `json:"readiness"`
	PromptSeam        BrandFilmReservedGenerationSeam `json:"generation_seam"`
	CreatedAt         time.Time                       `json:"created_at"`
	UpdatedAt         time.Time                       `json:"updated_at"`
}

type BrandFilmReservedGenerationSeam struct {
	ContractVersion string `json:"contract_version"`
	UnitPolicy      string `json:"unit_policy"`
	PromptContract  string `json:"prompt_contract"`
	AttemptPolicy   string `json:"attempt_policy"`
}

func (d BrandFilmDraft) Validate() error {
	if d.ContractVersion != "creative-brand-film-draft/v1" || strings.TrimSpace(d.TaskID) == "" || d.Revision < 1 ||
		!validSHA256Ref(d.SourceHash) || d.SourceSnapshot.Duration != 15 || d.SourceSnapshot.AspectRatio != "9:16" ||
		d.PromptSeam.ContractVersion != "creative-brand-generation-seam/v1" || d.CreatedAt.IsZero() || d.UpdatedAt.IsZero() {
		return fmt.Errorf("brand film draft is incomplete")
	}
	for _, analysis := range d.BriefAnalyses {
		if err := analysis.Validate(); err != nil {
			return err
		}
	}
	for _, concepts := range d.ConceptSets {
		if err := concepts.Validate(); err != nil {
			return err
		}
	}
	for _, plan := range d.FilmPlans {
		if err := plan.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (d BrandFilmDraft) CurrentAnalysis() *BrandBriefAnalysisVersion {
	if len(d.BriefAnalyses) == 0 {
		return nil
	}
	return &d.BriefAnalyses[len(d.BriefAnalyses)-1]
}

func (d BrandFilmDraft) CurrentConceptSet() *BrandCreativeConceptSet {
	if len(d.ConceptSets) == 0 {
		return nil
	}
	return &d.ConceptSets[len(d.ConceptSets)-1]
}

func (d BrandFilmDraft) CurrentPlan() *BrandFilmPlanVersion {
	if len(d.FilmPlans) == 0 {
		return nil
	}
	return &d.FilmPlans[len(d.FilmPlans)-1]
}
