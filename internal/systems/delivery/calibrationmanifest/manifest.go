// Package calibrationmanifest owns the single parser and projection for the
// frozen OceanEngine calibration Manifest. It has no dependency on Delivery so
// both Delivery and Platform Skills can consume the same typed facts.
package calibrationmanifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const SchemaVersion = "oceanengine-calibration-manifest/v1"
const Fixture = "docs/delivery/fixtures/oceanengine-calibration-manifest-v1.json"

var ErrInvalid = errors.New("invalid OceanEngine calibration manifest")

type Consumer string

const (
	DeliveryIntent            Consumer = "DeliveryIntent"
	OceanEngineConfiguration  Consumer = "OceanEngineConfiguration"
	DeliveryDecisionCandidate Consumer = "DeliveryDecisionCandidate"
	CompiledDeliveryWorkflow  Consumer = "CompiledDeliveryWorkflow"
	PlatformSkill             Consumer = "PlatformSkill"
)

type Treatment string

const (
	Modelled         Treatment = "modelled"
	DynamicReference Treatment = "dynamic_reference"
	EvidenceOnly     Treatment = "evidence_only"
	Blocked          Treatment = "blocked"
)

type Locator struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}
type PlaywrightRPAControl struct {
	Operation           string         `json:"operation"`
	Scope               Locator        `json:"scope"`
	Target              Locator        `json:"target"`
	ExpectedTargetCount int            `json:"expected_target_count"`
	Readback            Locator        `json:"readback"`
	ObservedOptions     []string       `json:"observed_options,omitempty"`
	InputConstraints    map[string]any `json:"input_constraints,omitempty"`
	BlockedState        string         `json:"blocked_state,omitempty"`
	Reason              string         `json:"reason,omitempty"`
}
type ConditionPredicate struct {
	Dimension string   `json:"dimension"`
	Operator  string   `json:"operator"`
	Values    []string `json:"values,omitempty"`
}
type ConditionRule struct {
	All []ConditionPredicate `json:"all"`
}
type Field struct {
	Key                      string               `json:"key"`
	SemanticLabel            string               `json:"semantic_label,omitempty"`
	ConfigurationRequirement string               `json:"configuration_requirement,omitempty"`
	PageFamily               string               `json:"page_family"`
	ValueType                string               `json:"value_type"`
	Unit                     string               `json:"unit,omitempty"`
	Condition                string               `json:"condition,omitempty"`
	ConditionDimensions      []string             `json:"condition_dimensions,omitempty"`
	ConditionState           string               `json:"condition_state,omitempty"`
	ConditionRule            *ConditionRule       `json:"condition_rule,omitempty"`
	EvidenceState            string               `json:"evidence_state"`
	Consumers                []Consumer           `json:"consumers"`
	PlaywrightRPA            PlaywrightRPAControl `json:"playwright_rpa"`
}
type ConsumerMapping struct {
	FieldKey     string    `json:"field_key"`
	Destination  Consumer  `json:"destination"`
	Treatment    Treatment `json:"treatment"`
	ContractPath string    `json:"contract_path"`
}
type ConditionVocabularyEntry struct {
	Key                   string   `json:"key"`
	SourceKind            string   `json:"source_kind"`
	Source                string   `json:"source"`
	ValueKind             string   `json:"value_kind"`
	KnownValues           []string `json:"known_values,omitempty"`
	UnknownValueTreatment string   `json:"unknown_value_treatment"`
}
type CoverageCase struct {
	ID        string   `json:"id"`
	FieldKeys []string `json:"field_keys"`
}
type Manifest struct {
	SchemaVersion       string `json:"schema_version"`
	ManifestID          string `json:"manifest_id"`
	Platform            string `json:"platform"`
	ObservationBoundary struct {
		RemoteWriteAuthorized bool `json:"remote_write_authorized"`
	} `json:"observation_boundary"`
	PageFamilies []struct {
		ID string `json:"id"`
	} `json:"page_families"`
	ConditionVocabulary []ConditionVocabularyEntry `json:"condition_vocabulary"`
	Fields              []Field                    `json:"fields"`
	CoverageCases       []CoverageCase             `json:"coverage_cases"`
	ConsumerMappings    []ConsumerMapping          `json:"consumer_mappings"`
}
type FieldProjection struct {
	Field      Field
	Mapping    ConsumerMapping
	Executable bool
	Blocked    bool
	Reason     string
}

func Load(path string) (Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read calibration manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return Manifest{}, fmt.Errorf("decode calibration manifest: %w", err)
	}
	return m, m.Validate()
}
func Current() (Manifest, error) {
	dir, err := os.Getwd()
	if err != nil {
		return Manifest{}, err
	}
	for {
		candidate := filepath.Join(dir, filepath.FromSlash(Fixture))
		if _, err := os.Stat(candidate); err == nil {
			return Load(candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return Manifest{}, fmt.Errorf("%w: frozen fixture %q was not found", ErrInvalid, Fixture)
}
func (m Manifest) ValidateBinding(schemaVersion, manifestID string) error {
	if schemaVersion != m.SchemaVersion || manifestID != m.ManifestID {
		return fmt.Errorf("%w: binding does not identify frozen source", ErrInvalid)
	}
	return nil
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != SchemaVersion || m.ManifestID == "" || m.Platform != "ocean_engine" || m.ObservationBoundary.RemoteWriteAuthorized {
		return fmt.Errorf("%w: invalid identity or remote-write boundary", ErrInvalid)
	}
	pages := map[string]struct{}{}
	for _, p := range m.PageFamilies {
		if p.ID == "" || has(pages, p.ID) {
			return fmt.Errorf("%w: duplicate page family", ErrInvalid)
		}
		pages[p.ID] = struct{}{}
	}
	vocabulary := map[string]ConditionVocabularyEntry{}
	for _, v := range m.ConditionVocabulary {
		if v.Key == "" || has(vocabulary, v.Key) || v.UnknownValueTreatment != "platform_pending" || (v.ValueKind != "enum" && v.ValueKind != "reference") {
			return fmt.Errorf("%w: invalid condition vocabulary", ErrInvalid)
		}
		vocabulary[v.Key] = v
	}
	fields := map[string]Field{}
	for _, f := range m.Fields {
		if f.Key == "" || has(fields, f.Key) || !has(pages, f.PageFamily) || len(f.Consumers) == 0 {
			return fmt.Errorf("%w: invalid field %q", ErrInvalid, f.Key)
		}
		if err := validateField(f, vocabulary); err != nil {
			return err
		}
		fields[f.Key] = f
	}
	mappings := map[string]struct{}{}
	for _, x := range m.ConsumerMappings {
		f, ok := fields[x.FieldKey]
		key := x.FieldKey + ":" + string(x.Destination)
		if !ok || x.ContractPath == "" || !containsConsumer(f.Consumers, x.Destination) || !validConsumer(x.Destination) || !validTreatment(x.Treatment) || has(mappings, key) {
			return fmt.Errorf("%w: invalid consumer mapping %q", ErrInvalid, key)
		}
		if x.Treatment == EvidenceOnly && !strings.Contains(strings.ToLower(x.ContractPath), "calibrationmanifest") && !strings.Contains(x.ContractPath, "FieldEvidence") && !strings.Contains(x.ContractPath, "calibration_manifest") {
			return fmt.Errorf("%w: evidence-only mapping has executable path", ErrInvalid)
		}
		mappings[key] = struct{}{}
	}
	for _, f := range m.Fields {
		for _, c := range f.Consumers {
			if !has(mappings, f.Key+":"+string(c)) {
				return fmt.Errorf("%w: missing consumer mapping for %s", ErrInvalid, f.Key)
			}
		}
	}
	covered := map[string]struct{}{}
	for _, c := range m.CoverageCases {
		for _, k := range c.FieldKeys {
			if !has(fields, k) {
				return fmt.Errorf("%w: unknown coverage field", ErrInvalid)
			}
			covered[k] = struct{}{}
		}
	}
	for k := range fields {
		if !has(covered, k) {
			return fmt.Errorf("%w: uncovered field %s", ErrInvalid, k)
		}
	}
	return nil
}
func (m Manifest) Project(consumer Consumer, facts map[string]string) []FieldProjection {
	out := []FieldProjection{}
	for _, mapping := range m.ConsumerMappings {
		if mapping.Destination != consumer {
			continue
		}
		field := m.field(mapping.FieldKey)
		p := FieldProjection{Field: field, Mapping: mapping}
		active, reason := field.matches(facts)
		if !active {
			p.Blocked = true
			p.Reason = reason
		} else if mapping.Treatment == Blocked {
			p.Blocked = true
			p.Reason = "blocked by frozen Manifest treatment"
		} else if mapping.Treatment == EvidenceOnly {
			p.Reason = "evidence-only field"
		} else if field.PlaywrightRPA.Operation == "no_action" {
			p.Blocked = true
			p.Reason = field.PlaywrightRPA.BlockedState
		} else {
			p.Executable = true
		}
		out = append(out, p)
	}
	return out
}
func (m Manifest) field(key string) Field {
	for _, f := range m.Fields {
		if f.Key == key {
			return f
		}
	}
	return Field{}
}
func (f Field) matches(facts map[string]string) (bool, string) {
	if f.Condition == "" {
		return true, ""
	}
	if f.ConditionState != "evaluable" || f.ConditionRule == nil {
		return false, "platform_pending"
	}
	for _, p := range f.ConditionRule.All {
		v, ok := facts[p.Dimension]
		if !ok || v == "" {
			return false, "platform_pending"
		}
		in := containsString(p.Values, v)
		switch p.Operator {
		case "equals", "in":
			if !in {
				return false, "condition_not_met"
			}
		case "not_in":
			if in {
				return false, "condition_not_met"
			}
		case "present":
		case "absent":
			return false, "condition_not_met"
		default:
			return false, "platform_pending"
		}
	}
	return true, ""
}
func validateField(f Field, vocabulary map[string]ConditionVocabularyEntry) error {
	if f.PlaywrightRPA.Operation == "" {
		return fmt.Errorf("%w: field %q lacks Computer Use control", ErrInvalid, f.Key)
	}
	if f.Condition != "" {
		if len(f.ConditionDimensions) == 0 || (f.ConditionState != "evaluable" && f.ConditionState != "dependency_only") {
			return fmt.Errorf("%w: invalid condition metadata", ErrInvalid)
		}
		dims := map[string]struct{}{}
		for _, d := range f.ConditionDimensions {
			if has(dims, d) || !has(vocabulary, d) {
				return fmt.Errorf("%w: unknown condition dimension", ErrInvalid)
			}
			dims[d] = struct{}{}
		}
		if f.ConditionState == "dependency_only" {
			if f.ConditionRule != nil || f.EvidenceState != "platform_pending" || f.PlaywrightRPA.Operation != "no_action" {
				return fmt.Errorf("%w: dependency-only field can act", ErrInvalid)
			}
			return nil
		}
		if f.ConditionRule == nil || len(f.ConditionRule.All) == 0 {
			return fmt.Errorf("%w: evaluable field lacks rule", ErrInvalid)
		}
		for _, p := range f.ConditionRule.All {
			if !has(dims, p.Dimension) || !validPredicate(p) {
				return fmt.Errorf("%w: invalid condition rule", ErrInvalid)
			}
		}
	}
	if f.PlaywrightRPA.Operation == "no_action" {
		return nil
	}
	if f.PlaywrightRPA.ExpectedTargetCount != 1 || !validLocator(f.PlaywrightRPA.Scope) || !validLocator(f.PlaywrightRPA.Target) || !validLocator(f.PlaywrightRPA.Readback) {
		return fmt.Errorf("%w: unsafe Computer Use control", ErrInvalid)
	}
	return nil
}
func validPredicate(p ConditionPredicate) bool {
	if p.Dimension == "" {
		return false
	}
	switch p.Operator {
	case "equals":
		return len(p.Values) == 1
	case "in", "not_in":
		return len(p.Values) > 0
	case "present", "absent":
		return len(p.Values) == 0
	default:
		return false
	}
}
func validLocator(l Locator) bool {
	if l.Value == "" || strings.Contains(l.Value, "nth-child") || strings.Contains(l.Value, "[") {
		return false
	}
	switch l.Kind {
	case "role_name", "label", "visible_text", "placeholder", "attribute", "domain_key":
		return true
	default:
		return false
	}
}
func validConsumer(v Consumer) bool {
	switch v {
	case DeliveryIntent, OceanEngineConfiguration, DeliveryDecisionCandidate, CompiledDeliveryWorkflow, PlatformSkill:
		return true
	default:
		return false
	}
}
func validTreatment(v Treatment) bool {
	switch v {
	case Modelled, DynamicReference, EvidenceOnly, Blocked:
		return true
	default:
		return false
	}
}
func containsConsumer(v []Consumer, wanted Consumer) bool {
	for _, x := range v {
		if x == wanted {
			return true
		}
	}
	return false
}
func containsString(v []string, w string) bool {
	for _, x := range v {
		if x == w {
			return true
		}
	}
	return false
}
func has[V any](m map[string]V, k string) bool { _, ok := m[k]; return ok }
