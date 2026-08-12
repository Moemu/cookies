// Package eval provides a provider-neutral offline quality gate for Strategy
// outputs. It never invokes a model or uploads business data.
package eval

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shikanon/cookies/internal/systems/strategy"
)

//go:embed cases/*.json
var caseFiles embed.FS

type Case struct {
	ID              string   `json:"id"`
	Scenario        string   `json:"scenario,omitempty"`
	Objective       string   `json:"objective"`
	Audience        string   `json:"audience"`
	Proposition     string   `json:"proposition"`
	Channels        []string `json:"channels"`
	Budget          string   `json:"budget,omitempty"`
	Schedule        string   `json:"schedule,omitempty"`
	PrimaryKPI      string   `json:"primary_kpi,omitempty"`
	Constraints     []string `json:"constraints,omitempty"`
	MissingFields   []string `json:"missing_fields,omitempty"`
	UntrustedInput  string   `json:"untrusted_input,omitempty"`
	ExpectedSignals []string `json:"expected_signals"`
}

type Score struct {
	CaseID             string   `json:"case_id"`
	Total              int      `json:"total"`
	BriefAlignment     int      `json:"brief_alignment"`
	Specificity        int      `json:"specificity"`
	Executability      int      `json:"executability"`
	Measurement        int      `json:"measurement"`
	PlatformAdaptation int      `json:"platform_adaptation"`
	ExpectedSignalHits []string `json:"expected_signal_hits"`
	Failures           []string `json:"failures"`
	QualityScore       int      `json:"quality_score"`
	QualityGatePassed  bool     `json:"quality_gate_passed"`
	QualityRubric      Rubric   `json:"quality_rubric"`
	QualityFailures    []string `json:"quality_failures"`
}

// Rubric is an additive, decision-quality scorecard. The legacy ten-point
// score above remains stable for existing eval consumers while this rubric
// raises the bar for V4 strategy outputs.
type Rubric struct {
	DecisionClarity      int `json:"decision_clarity"`
	AudienceTension      int `json:"audience_tension"`
	CreativeDistinctness int `json:"creative_distinctness"`
	Executability        int `json:"executability"`
	PlatformFit          int `json:"platform_fit"`
	EvidenceDiscipline   int `json:"evidence_discipline"`
	SignalCoverage       int `json:"signal_coverage"`
}

func LoadCases() ([]Case, error) {
	entries, err := caseFiles.ReadDir("cases")
	if err != nil {
		return nil, err
	}
	values := make([]Case, 0, len(entries))
	for _, entry := range entries {
		content, err := caseFiles.ReadFile("cases/" + entry.Name())
		if err != nil {
			return nil, err
		}
		var value Case
		if err := json.Unmarshal(content, &value); err != nil {
			return nil, fmt.Errorf("decode eval case %s: %w", entry.Name(), err)
		}
		if strings.TrimSpace(value.ID) == "" || strings.TrimSpace(value.Objective) == "" ||
			strings.TrimSpace(value.Audience) == "" || strings.TrimSpace(value.Proposition) == "" ||
			len(value.Channels) == 0 {
			return nil, fmt.Errorf("eval case %s is incomplete", entry.Name())
		}
		values = append(values, value)
	}
	return values, nil
}

func Evaluate(testCase Case, document strategy.StrategyDocument) Score {
	score := Score{CaseID: testCase.ID, Failures: []string{}}
	if containsFold(document.Objective, testCase.Objective) ||
		containsFold(testCase.Objective, document.Objective) {
		score.BriefAlignment++
	} else {
		score.Failures = append(score.Failures, "objective drift")
	}
	if containsFold(document.Audience.Primary, testCase.Audience) ||
		containsFold(testCase.Audience, document.Audience.Primary) {
		score.BriefAlignment++
	} else {
		score.Failures = append(score.Failures, "audience drift")
	}
	if containsFold(document.Proposition, testCase.Proposition) ||
		containsFold(testCase.Proposition, document.Proposition) {
		score.BriefAlignment++
	} else {
		score.Failures = append(score.Failures, "proposition drift")
	}
	if len(document.Audience.Insights) >= 1 && len(document.CreativeDirections()) >= 1 {
		score.Specificity = 2
	} else {
		score.Failures = append(score.Failures, "insufficient insights or creative recommendations")
	}
	if len(document.ChannelStrategy) >= 1 && len(document.ExperimentMatrix) >= 1 {
		score.Executability = 2
	} else {
		score.Failures = append(score.Failures, "missing channel execution or experiment")
	}
	if len(document.Measurement) >= 1 &&
		len(document.ExperimentMatrix) >= 1 &&
		strings.TrimSpace(document.ExperimentMatrix[0].Metric) != "" {
		score.Measurement = 2
	} else {
		score.Failures = append(score.Failures, "measurement is incomplete")
	}
	if coversChannels(testCase.Channels, document.ChannelStrategy) && platformPlansAreDistinct(document.PlatformPlans) {
		score.PlatformAdaptation = 1
	} else {
		score.Failures = append(score.Failures, "channel mismatch or duplicated platform plans")
	}
	encoded, _ := json.Marshal(document)
	for _, signal := range testCase.ExpectedSignals {
		if containsFold(string(encoded), signal) {
			score.ExpectedSignalHits = append(score.ExpectedSignalHits, signal)
		}
	}
	score.Total = score.BriefAlignment + score.Specificity + score.Executability + score.Measurement + score.PlatformAdaptation
	score.QualityRubric, score.QualityFailures = evaluateDecisionQuality(testCase, document, score.ExpectedSignalHits)
	score.QualityScore = score.QualityRubric.total()
	score.QualityGatePassed = score.QualityScore >= 80
	return score
}

func (r Rubric) total() int {
	return r.DecisionClarity + r.AudienceTension + r.CreativeDistinctness +
		r.Executability + r.PlatformFit + r.EvidenceDiscipline + r.SignalCoverage
}

func evaluateDecisionQuality(testCase Case, document strategy.StrategyDocument, signalHits []string) (Rubric, []string) {
	rubric := Rubric{}
	failures := []string{}

	propositionLength := len([]rune(strings.TrimSpace(document.Proposition)))
	if propositionLength > 0 && propositionLength <= 60 {
		rubric.DecisionClarity += 8
	} else if propositionLength > 0 {
		rubric.DecisionClarity += 4
		failures = append(failures, "core proposition is not concise")
	} else {
		failures = append(failures, "core proposition is missing")
	}
	summaryLength := len([]rune(strings.TrimSpace(document.ExecutiveSummary)))
	if summaryLength >= 30 && summaryLength <= 220 {
		rubric.DecisionClarity += 12
	} else if summaryLength > 0 && summaryLength <= 220 {
		rubric.DecisionClarity += 8
		failures = append(failures, "executive summary is too thin")
	} else {
		failures = append(failures, "executive summary is missing or too long")
	}

	if strings.TrimSpace(document.Audience.Primary) != "" {
		rubric.AudienceTension += 5
	}
	if len(document.Audience.Insights) > 0 {
		rubric.AudienceTension += 5
		if len([]rune(strings.TrimSpace(document.Audience.Insights[0]))) >= 12 {
			rubric.AudienceTension += 5
		} else {
			failures = append(failures, "audience insight does not expose a decision tension")
		}
	} else {
		failures = append(failures, "audience insight is missing")
	}

	directions := document.CreativeDirections()
	if len(directions) >= 1 {
		rubric.CreativeDistinctness += 5
	}
	structured := 0
	for _, direction := range directions {
		if directionHasAnatomy(direction) {
			structured++
		}
	}
	if structured >= 3 {
		rubric.CreativeDistinctness += 10
	} else {
		failures = append(failures, "creative directions are not decision-ready")
	}
	if creativeDirectionsAreDistinct(directions) {
		rubric.CreativeDistinctness += 10
	} else {
		failures = append(failures, "creative directions are semantically duplicated")
	}

	if len(document.ChannelStrategy) > 0 {
		rubric.Executability += 5
	}
	if hasExecutableExperiment(document.ExperimentMatrix) {
		rubric.Executability += 10
	} else {
		failures = append(failures, "experiment is missing a hypothesis, single variable, or metric")
	}

	if coversChannels(testCase.Channels, document.ChannelStrategy) {
		rubric.PlatformFit += 5
	}
	if platformPlansAreDistinct(document.PlatformPlans) {
		rubric.PlatformFit += 5
	} else {
		failures = append(failures, "platform plans repeat the same content plan")
	}

	if len(document.EvidenceRefs) > 0 {
		rubric.EvidenceDiscipline = 10
	} else if len(document.AssumptionsAndGaps) > 0 {
		rubric.EvidenceDiscipline = 6
		failures = append(failures, "strategy relies on declared gaps instead of evidence")
	} else {
		failures = append(failures, "strategy has neither evidence nor declared gaps")
	}

	if len(testCase.ExpectedSignals) == 0 {
		rubric.SignalCoverage = 5
	} else {
		rubric.SignalCoverage = len(signalHits) * 5 / len(testCase.ExpectedSignals)
		if rubric.SignalCoverage < 5 {
			failures = append(failures, "expected scenario signals are not fully covered")
		}
	}

	return rubric, failures
}

func directionHasAnatomy(value string) bool {
	return strings.Count(value, "｜")+strings.Count(value, "|") >= 4
}

func hasExecutableExperiment(values []strategy.Experiment) bool {
	for _, value := range values {
		if strings.TrimSpace(value.Hypothesis) != "" && strings.TrimSpace(value.Variable) != "" && strings.TrimSpace(value.Metric) != "" {
			return true
		}
	}
	return false
}

func creativeDirectionsAreDistinct(values []string) bool {
	if len(values) < 3 {
		return false
	}
	for left := 0; left < len(values); left++ {
		for right := left + 1; right < len(values); right++ {
			if runeBigramSimilarity(values[left], values[right]) >= 0.70 {
				return false
			}
		}
	}
	return true
}

func runeBigramSimilarity(left, right string) float64 {
	leftSet := runeBigrams(left)
	rightSet := runeBigrams(right)
	if len(leftSet) == 0 || len(rightSet) == 0 {
		return 0
	}
	intersection := 0
	for value := range leftSet {
		if _, ok := rightSet[value]; ok {
			intersection++
		}
	}
	union := len(leftSet) + len(rightSet) - intersection
	return float64(intersection) / float64(union)
}

func runeBigrams(value string) map[string]struct{} {
	runes := []rune(strings.ToLower(strings.TrimSpace(value)))
	result := map[string]struct{}{}
	for index := 0; index+1 < len(runes); index++ {
		result[string(runes[index:index+2])] = struct{}{}
	}
	return result
}

func platformPlansAreDistinct(values []strategy.PlatformPlan) bool {
	if len(values) < 2 {
		return true
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		signature := strings.ToLower(strings.TrimSpace(value.Role)) + "\x00" +
			strings.ToLower(strings.TrimSpace(value.AudienceAngle)) + "\x00" +
			strings.Join(value.ContentPillars, "\x00") + "\x00" +
			strings.Join(value.Formats, "\x00") + "\x00" +
			strings.ToLower(strings.TrimSpace(value.ConversionPath)) + "\x00" +
			strings.Join(value.CreativeIdeas, "\x00")
		if _, ok := seen[signature]; ok {
			return false
		}
		seen[signature] = struct{}{}
	}
	return true
}

func containsFold(value, candidate string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(value)), strings.ToLower(strings.TrimSpace(candidate)))
}

func coversChannels(expected []string, actual []strategy.ChannelStrategy) bool {
	for _, channel := range expected {
		found := false
		for _, value := range actual {
			if strings.EqualFold(strings.TrimSpace(channel), strings.TrimSpace(value.Platform)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return len(expected) > 0
}
