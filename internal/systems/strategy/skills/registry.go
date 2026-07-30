package skills

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed platform/*.json objective/*.json creative/*.json
var skillFiles embed.FS

type Skill struct {
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	Kind          string   `json:"kind"`
	Match         []string `json:"match"`
	Instructions  []string `json:"instructions"`
	QualityChecks []string `json:"quality_checks"`
}

type Snapshot struct {
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	ContentHash   string   `json:"content_hash"`
	Instructions  []string `json:"instructions"`
	QualityChecks []string `json:"quality_checks"`
}

type Descriptor struct {
	Name          string   `json:"name"`
	Version       string   `json:"version"`
	Kind          string   `json:"kind"`
	Match         []string `json:"match"`
	Instructions  []string `json:"instructions,omitempty"`
	QualityChecks []string `json:"quality_checks"`
	ContentHash   string   `json:"content_hash"`
}

type Registry struct {
	skills []Skill
}

func DefaultRegistry() (Registry, error) {
	var values []Skill
	err := fs.WalkDir(skillFiles, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		content, err := skillFiles.ReadFile(path)
		if err != nil {
			return err
		}
		var skill Skill
		if err := json.Unmarshal(content, &skill); err != nil {
			return fmt.Errorf("decode Strategy skill %s: %w", path, err)
		}
		if err := skill.Validate(); err != nil {
			return fmt.Errorf("validate Strategy skill %s: %w", path, err)
		}
		values = append(values, skill)
		return nil
	})
	if err != nil {
		return Registry{}, err
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Kind != values[j].Kind {
			return values[i].Kind == "platform"
		}
		return values[i].Name < values[j].Name
	})
	return Registry{skills: values}, nil
}

func (s Skill) Validate() error {
	if strings.TrimSpace(s.Name) == "" || strings.TrimSpace(s.Version) == "" {
		return fmt.Errorf("name and version are required")
	}
	if s.Kind != "platform" && s.Kind != "objective" && s.Kind != "creative_task" {
		return fmt.Errorf("kind must be platform, objective, or creative_task")
	}
	if len(s.Match) == 0 || len(s.Instructions) == 0 || len(s.QualityChecks) == 0 {
		return fmt.Errorf("match, instructions, and quality_checks are required")
	}
	return nil
}

func (r Registry) Select(channels []string, objective string) []Snapshot {
	keys := make(map[string]struct{}, len(channels)+4)
	for _, channel := range channels {
		keys[normalize(channel)] = struct{}{}
	}
	for _, key := range objectiveKeys(objective) {
		keys[key] = struct{}{}
	}
	var snapshots []Snapshot
	for _, skill := range r.skills {
		if skill.Kind != "platform" && skill.Kind != "objective" {
			continue
		}
		matched := false
		for _, candidate := range skill.Match {
			if _, ok := keys[normalize(candidate)]; ok {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		encoded, _ := json.Marshal(skill)
		sum := sha256.Sum256(encoded)
		snapshots = append(snapshots, Snapshot{
			Name: skill.Name, Version: skill.Version, ContentHash: hex.EncodeToString(sum[:]),
			Instructions:  append([]string(nil), skill.Instructions...),
			QualityChecks: append([]string(nil), skill.QualityChecks...),
		})
	}
	return snapshots
}

func (r Registry) SelectCreativeTask(businessCode string) (Snapshot, error) {
	key := normalize(businessCode)
	var matched []Skill
	for _, skill := range r.skills {
		if skill.Kind != "creative_task" {
			continue
		}
		for _, candidate := range skill.Match {
			if normalize(candidate) == key {
				matched = append(matched, skill)
				break
			}
		}
	}
	if len(matched) != 1 {
		return Snapshot{}, fmt.Errorf("creative task skill for %q: found %d, want 1", businessCode, len(matched))
	}
	skill := matched[0]
	encoded, _ := json.Marshal(skill)
	sum := sha256.Sum256(encoded)
	return Snapshot{
		Name: skill.Name, Version: skill.Version, ContentHash: hex.EncodeToString(sum[:]),
		Instructions:  append([]string(nil), skill.Instructions...),
		QualityChecks: append([]string(nil), skill.QualityChecks...),
	}, nil
}

func (r Registry) List(includeInstructions bool) []Descriptor {
	values := make([]Descriptor, 0, len(r.skills))
	for _, skill := range r.skills {
		encoded, _ := json.Marshal(skill)
		sum := sha256.Sum256(encoded)
		value := Descriptor{
			Name:          skill.Name,
			Version:       skill.Version,
			Kind:          skill.Kind,
			Match:         append([]string(nil), skill.Match...),
			QualityChecks: append([]string(nil), skill.QualityChecks...),
			ContentHash:   hex.EncodeToString(sum[:]),
		}
		if includeInstructions {
			value.Instructions = append([]string(nil), skill.Instructions...)
		}
		values = append(values, value)
	}
	return values
}

func objectiveKeys(objective string) []string {
	value := normalize(objective)
	switch {
	case containsAny(value, "线索", "获客", "留资", "lead"):
		return []string{"lead_generation"}
	case containsAny(value, "转化", "成交", "销售", "下单", "conversion"):
		return []string{"conversion"}
	default:
		return []string{"awareness"}
	}
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}
