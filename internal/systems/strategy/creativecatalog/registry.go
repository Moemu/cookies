package creativecatalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/shikanon/cookies/internal/platform/contract"
	strategyskills "github.com/shikanon/cookies/internal/systems/strategy/skills"
)

//go:embed profiles/*.json
var profileFiles embed.FS

type RecommendationRule struct {
	ID       string   `json:"id"`
	Field    string   `json:"field"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
	Number   int      `json:"number,omitempty"`
	Weight   int      `json:"weight"`
	Reason   string   `json:"reason"`
	Required bool     `json:"required,omitempty"`
}

type QuestionOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type QuestionCondition struct {
	QuestionID string `json:"question_id"`
	Equals     any    `json:"equals"`
}

type QuestionValidation struct {
	MaxLength int `json:"max_length,omitempty"`
	MaxItems  int `json:"max_items,omitempty"`
}

type QuestionDefinition struct {
	ID              string              `json:"id"`
	Label           string              `json:"label"`
	Type            string              `json:"type"`
	RequiredFor     string              `json:"required_for"`
	BriefSourcePath string              `json:"brief_source_path,omitempty"`
	Help            string              `json:"help,omitempty"`
	Options         []QuestionOption    `json:"options,omitempty"`
	DependsOn       *QuestionCondition  `json:"depends_on,omitempty"`
	Validation      *QuestionValidation `json:"validation,omitempty"`
}

type BusinessRequirements struct {
	Strategy   []string `json:"strategy"`
	Production []string `json:"production"`
}

type OutputFieldDefinition struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	MaxItems    int    `json:"max_items,omitempty"`
	MaxLength   int    `json:"max_length,omitempty"`
	Description string `json:"description"`
}

type ReferenceUsePolicy struct {
	AllowsUnknownForStrategy bool     `json:"allows_unknown_for_strategy"`
	AllowedStrategyUses      []string `json:"allowed_strategy_uses"`
	ProductionConfirmations  []string `json:"production_confirmations"`
}

type Profile struct {
	BusinessCode     string                  `json:"business_code"`
	Generation       int64                   `json:"generation"`
	Version          string                  `json:"version"`
	DisplayName      string                  `json:"display_name"`
	Summary          string                  `json:"summary"`
	Lifecycle        string                  `json:"lifecycle"`
	Selectable       bool                    `json:"selectable"`
	DisplayOrder     int                     `json:"display_order"`
	MatchRules       []RecommendationRule    `json:"match_rules"`
	Questions        []QuestionDefinition    `json:"questions"`
	Requirements     BusinessRequirements    `json:"requirements"`
	OutputFields     []OutputFieldDefinition `json:"output_fields"`
	ReferencePolicy  ReferenceUsePolicy      `json:"reference_policy"`
	SkillName        string                  `json:"skill_name"`
	SkillVersion     string                  `json:"skill_version"`
	Owner            string                  `json:"owner"`
	ReviewedBy       string                  `json:"reviewed_by"`
	ReviewedAt       *time.Time              `json:"reviewed_at,omitempty"`
	PublishedAt      time.Time               `json:"published_at"`
	SkillContentHash string                  `json:"skill_content_hash"`
	ContentHash      string                  `json:"content_hash,omitempty"`
}

type Ref struct {
	BusinessCode string `json:"business_code"`
	Generation   int64  `json:"generation"`
	Version      string `json:"version"`
	ContentHash  string `json:"content_hash"`
}

type Registry struct {
	profiles []Profile
}

func DefaultRegistry() (Registry, error) {
	skillRegistry, err := strategyskills.DefaultRegistry()
	if err != nil {
		return Registry{}, err
	}
	var profiles []Profile
	err = fs.WalkDir(profileFiles, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		content, err := profileFiles.ReadFile(path)
		if err != nil {
			return err
		}
		var profile Profile
		if err := json.Unmarshal(content, &profile); err != nil {
			return fmt.Errorf("decode creative business profile %s: %w", path, err)
		}
		skill, err := skillRegistry.SelectCreativeTask(profile.BusinessCode)
		if err != nil {
			return fmt.Errorf("resolve creative business profile %s skill: %w", path, err)
		}
		if profile.SkillName != skill.Name || profile.SkillVersion != skill.Version {
			return fmt.Errorf("creative business profile %s skill reference does not match registry", path)
		}
		profile.SkillContentHash = skill.ContentHash
		if err := profile.Validate(); err != nil {
			return fmt.Errorf("validate creative business profile %s: %w", path, err)
		}
		hash, err := contract.NewContentHash(profile)
		if err != nil {
			return err
		}
		profile.ContentHash = string(hash)
		profiles = append(profiles, profile)
		return nil
	})
	if err != nil {
		return Registry{}, err
	}
	sort.Slice(profiles, func(i, j int) bool {
		if profiles[i].BusinessCode != profiles[j].BusinessCode {
			return profiles[i].BusinessCode < profiles[j].BusinessCode
		}
		return profiles[i].Generation < profiles[j].Generation
	})
	return Registry{profiles: profiles}, nil
}

func (r Registry) All() []Profile {
	return append([]Profile(nil), r.profiles...)
}

func (r Registry) Current() []Profile {
	current := map[string]Profile{}
	for _, profile := range r.profiles {
		if profile.Lifecycle == "draft" {
			continue
		}
		prior, found := current[profile.BusinessCode]
		if !found || profile.Generation > prior.Generation {
			current[profile.BusinessCode] = profile
		}
	}
	result := make([]Profile, 0, len(current))
	for _, profile := range current {
		result = append(result, profile)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].DisplayOrder != result[j].DisplayOrder {
			return result[i].DisplayOrder < result[j].DisplayOrder
		}
		return result[i].BusinessCode < result[j].BusinessCode
	})
	return result
}

func (r Registry) FindCurrent(businessCode string) (Profile, bool) {
	for _, profile := range r.Current() {
		if profile.BusinessCode == strings.TrimSpace(businessCode) {
			return profile, true
		}
	}
	return Profile{}, false
}

func (r Registry) CatalogHash() (string, error) {
	current := r.Current()
	refs := make([]Ref, 0, len(current))
	for _, profile := range current {
		refs = append(refs, profile.Ref())
	}
	hash, err := contract.NewContentHash(refs)
	return string(hash), err
}

func (p Profile) Ref() Ref {
	return Ref{
		BusinessCode: p.BusinessCode,
		Generation:   p.Generation,
		Version:      p.Version,
		ContentHash:  p.ContentHash,
	}
}
