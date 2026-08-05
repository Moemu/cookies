package strategy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/shikanon/cookies/internal/platform/contract"
)

const BriefContractVersionV3 = "strategy-brief-version/v3"

type BriefCoreV3 struct {
	Objective         string `json:"objective"`
	DeliverableIntent string `json:"deliverable_intent"`
	ProductOrSubject  string `json:"product_or_subject"`
	Audience          string `json:"audience"`
}

type BriefFactV3 struct {
	ID         string          `json:"id"`
	Kind       string          `json:"kind"`
	Label      string          `json:"label,omitempty"`
	Value      json.RawMessage `json:"value"`
	SourceRefs []FieldSource   `json:"source_refs"`
	Confidence string          `json:"confidence"`
}

type BriefAssumptionV3 struct {
	ID         string        `json:"id"`
	Statement  string        `json:"statement"`
	Reason     string        `json:"reason,omitempty"`
	SourceRefs []FieldSource `json:"source_refs"`
}

type BriefUnknownV3 struct {
	ID          string `json:"id"`
	Question    string `json:"question"`
	Impact      string `json:"impact,omitempty"`
	RequiredFor string `json:"required_for"`
}

type BriefConflictCandidateV3 struct {
	Value      json.RawMessage `json:"value"`
	SourceRefs []FieldSource   `json:"source_refs"`
}

type BriefConflictV3 struct {
	ID         string                     `json:"id"`
	Field      string                     `json:"field"`
	Candidates []BriefConflictCandidateV3 `json:"candidates"`
	Status     string                     `json:"status"`
}

type storedBriefDocumentV3 struct {
	ContractVersion string                     `json:"contract_version"`
	Core            BriefCoreV3                `json:"core"`
	Facts           []BriefFactV3              `json:"facts"`
	Constraints     []string                   `json:"constraints"`
	Assumptions     []BriefAssumptionV3        `json:"assumptions"`
	Unknowns        []BriefUnknownV3           `json:"unknowns"`
	Conflicts       []BriefConflictV3          `json:"conflicts"`
	AssetRefs       []contract.AssetVersionRef `json:"asset_refs"`
	ReferenceIDs    []string                   `json:"reference_ids"`
	Extensions      map[string]json.RawMessage `json:"extensions"`
}

func marshalBriefDocumentV3(document BriefDocument) ([]byte, error) {
	stored := storedBriefDocumentV3{
		ContractVersion: BriefContractVersionV3,
		Core:            document.Core,
		Facts:           canonicalBriefFacts(document.Facts),
		Constraints:     nonNilStrings(document.Constraints),
		Assumptions:     canonicalBriefAssumptions(document.Assumptions),
		Unknowns:        nonNilBriefUnknowns(document.Unknowns),
		Conflicts:       canonicalBriefConflicts(document.Conflicts),
		AssetRefs:       nonNilAssetRefs(document.AssetRefs),
		ReferenceIDs:    nonNilStrings(document.ReferenceIDs),
		Extensions:      document.Extensions,
	}
	if stored.Extensions == nil {
		stored.Extensions = map[string]json.RawMessage{}
	}
	if err := validateBriefDocumentV3(stored); err != nil {
		return nil, err
	}
	return json.Marshal(stored)
}

func unmarshalBriefDocumentV3(data []byte, document *BriefDocument) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var stored storedBriefDocumentV3
	if err := decoder.Decode(&stored); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrInvalidRequest
	}
	if err := validateBriefDocumentV3(stored); err != nil {
		return err
	}
	*document = BriefDocument{
		ContractVersion: stored.ContractVersion,
		Core:            stored.Core,
		Facts:           stored.Facts,
		Constraints:     stored.Constraints,
		Assumptions:     stored.Assumptions,
		Unknowns:        stored.Unknowns,
		Conflicts:       stored.Conflicts,
		AssetRefs:       stored.AssetRefs,
		ReferenceIDs:    stored.ReferenceIDs,
		Extensions:      stored.Extensions,
	}
	document.SyncV3LegacyProjection()
	return nil
}

// SyncV3LegacyProjection keeps existing Strategy and Creative readers useful
// during the migration window. It is an adapter only: v3 JSON never serializes
// the legacy form fields.
func (document *BriefDocument) SyncV3LegacyProjection() {
	if document == nil || document.ContractVersion != BriefContractVersionV3 {
		return
	}
	document.Campaign.Objective = document.Core.Objective
	document.Product.Name = document.Core.ProductOrSubject
	document.Audience.Primary = document.Core.Audience
	for _, fact := range document.Facts {
		values := briefFactStrings(fact.Value)
		if len(values) == 0 {
			continue
		}
		switch fact.Kind {
		case "brand":
			document.Brand.Name = values[0]
		case "proposition":
			document.Proposition = values[0]
		case "industry":
			document.Industry = values[0]
		case "region":
			document.Region = values[0]
		case "language":
			document.Language = values[0]
		case "channel":
			for _, value := range values {
				document.Channels = appendUnique(document.Channels, value)
			}
		case "selling_point":
			for _, value := range values {
				document.Product.SellingPoints = appendUnique(document.Product.SellingPoints, value)
			}
		case "budget":
			document.Budget.Total = values[0]
		case "schedule":
			document.Schedule.Window = values[0]
		case "primary_kpi":
			document.Measurement.PrimaryKPI = values[0]
		}
	}
}

func validateBriefDocumentV3(document storedBriefDocumentV3) error {
	if document.ContractVersion != BriefContractVersionV3 {
		return ErrInvalidRequest
	}
	seen := map[string]struct{}{}
	for _, fact := range document.Facts {
		if !validRequirementItemID(fact.ID) || !allowedBriefFactKind(fact.Kind) || !validBriefFactValue(fact.Value) {
			return ErrInvalidRequest
		}
		if fact.Kind == "custom" && strings.TrimSpace(fact.Label) == "" {
			return ErrInvalidRequest
		}
		if fact.Confidence != "low" && fact.Confidence != "medium" && fact.Confidence != "high" {
			return ErrInvalidRequest
		}
		if !validUniqueRequirementID(seen, fact.ID) || !validFieldSources(fact.SourceRefs) {
			return ErrInvalidRequest
		}
	}
	for _, assumption := range document.Assumptions {
		if !validRequirementItemID(assumption.ID) || strings.TrimSpace(assumption.Statement) == "" || !validUniqueRequirementID(seen, assumption.ID) || !validFieldSources(assumption.SourceRefs) {
			return ErrInvalidRequest
		}
	}
	for _, unknown := range document.Unknowns {
		if !validRequirementItemID(unknown.ID) || strings.TrimSpace(unknown.Question) == "" || !validUniqueRequirementID(seen, unknown.ID) {
			return ErrInvalidRequest
		}
		if unknown.RequiredFor != "creative_intake" && unknown.RequiredFor != "production" && unknown.RequiredFor != "optional" {
			return ErrInvalidRequest
		}
	}
	for _, conflict := range document.Conflicts {
		if !validRequirementItemID(conflict.ID) || strings.TrimSpace(conflict.Field) == "" || len(conflict.Candidates) < 2 || !validUniqueRequirementID(seen, conflict.ID) {
			return ErrInvalidRequest
		}
		if conflict.Status != "open" && conflict.Status != "resolved" {
			return ErrInvalidRequest
		}
		for _, candidate := range conflict.Candidates {
			if !json.Valid(candidate.Value) || !validFieldSources(candidate.SourceRefs) {
				return ErrInvalidRequest
			}
		}
	}
	assetSeen := map[string]struct{}{}
	for _, ref := range document.AssetRefs {
		if ref.Validate() != nil {
			return ErrInvalidRequest
		}
		key := string(ref.AssetID) + ":" + formatInt64(ref.Version)
		if _, duplicate := assetSeen[key]; duplicate {
			return ErrInvalidRequest
		}
		assetSeen[key] = struct{}{}
	}
	referenceSeen := map[string]struct{}{}
	for _, id := range document.ReferenceIDs {
		if !validRequirementItemID(id) || !validUniqueRequirementID(referenceSeen, id) {
			return ErrInvalidRequest
		}
	}
	for _, constraint := range document.Constraints {
		if strings.TrimSpace(constraint) == "" {
			return ErrInvalidRequest
		}
	}
	for key, value := range document.Extensions {
		if strings.TrimSpace(key) == "" || len(key) > 96 || !json.Valid(value) {
			return ErrInvalidRequest
		}
	}
	return nil
}

func validRequirementItemID(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= 96 && !strings.ContainsAny(value, " \t\r\n")
}

func validUniqueRequirementID(seen map[string]struct{}, value string) bool {
	if _, duplicate := seen[value]; duplicate {
		return false
	}
	seen[value] = struct{}{}
	return true
}

func validFieldSources(sources []FieldSource) bool {
	for _, source := range sources {
		if strings.TrimSpace(source.Type) == "" || strings.TrimSpace(source.ID) == "" {
			return false
		}
	}
	return true
}

func allowedBriefFactKind(value string) bool {
	switch value {
	case "brand", "proposition", "industry", "region", "language", "channel", "selling_point", "budget", "schedule", "primary_kpi", "claim", "custom":
		return true
	default:
		return false
	}
}

func briefFactStrings(raw json.RawMessage) []string {
	var scalar string
	if json.Unmarshal(raw, &scalar) == nil && strings.TrimSpace(scalar) != "" {
		return []string{strings.TrimSpace(scalar)}
	}
	var values []string
	if json.Unmarshal(raw, &values) != nil {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func validBriefFactValue(raw json.RawMessage) bool {
	if !json.Valid(raw) || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return false
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return true
	}
	var number float64
	if json.Unmarshal(raw, &number) == nil {
		return true
	}
	var boolean bool
	if json.Unmarshal(raw, &boolean) == nil {
		return true
	}
	var values []string
	return json.Unmarshal(raw, &values) == nil && values != nil
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func canonicalBriefFacts(values []BriefFactV3) []BriefFactV3 {
	if values == nil {
		return []BriefFactV3{}
	}
	result := append([]BriefFactV3(nil), values...)
	for index := range result {
		result[index].SourceRefs = nonNilFieldSources(result[index].SourceRefs)
	}
	return result
}

func canonicalBriefAssumptions(values []BriefAssumptionV3) []BriefAssumptionV3 {
	if values == nil {
		return []BriefAssumptionV3{}
	}
	result := append([]BriefAssumptionV3(nil), values...)
	for index := range result {
		result[index].SourceRefs = nonNilFieldSources(result[index].SourceRefs)
	}
	return result
}

func nonNilBriefUnknowns(values []BriefUnknownV3) []BriefUnknownV3 {
	if values == nil {
		return []BriefUnknownV3{}
	}
	return values
}

func canonicalBriefConflicts(values []BriefConflictV3) []BriefConflictV3 {
	if values == nil {
		return []BriefConflictV3{}
	}
	result := append([]BriefConflictV3(nil), values...)
	for index := range result {
		result[index].Candidates = append([]BriefConflictCandidateV3(nil), result[index].Candidates...)
		for candidateIndex := range result[index].Candidates {
			result[index].Candidates[candidateIndex].SourceRefs = nonNilFieldSources(result[index].Candidates[candidateIndex].SourceRefs)
		}
	}
	return result
}

func nonNilAssetRefs(values []contract.AssetVersionRef) []contract.AssetVersionRef {
	if values == nil {
		return []contract.AssetVersionRef{}
	}
	return values
}

func nonNilFieldSources(values []FieldSource) []FieldSource {
	if values == nil {
		return []FieldSource{}
	}
	return values
}
