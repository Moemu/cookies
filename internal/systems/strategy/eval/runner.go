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
	if len(document.Audience.Insights) >= 1 && len(document.CreativeRecommendations) >= 3 {
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
	if coversChannels(testCase.Channels, document.ChannelStrategy) {
		score.PlatformAdaptation = 1
	} else {
		score.Failures = append(score.Failures, "channel mismatch")
	}
	encoded, _ := json.Marshal(document)
	for _, signal := range testCase.ExpectedSignals {
		if containsFold(string(encoded), signal) {
			score.ExpectedSignalHits = append(score.ExpectedSignalHits, signal)
		}
	}
	score.Total = score.BriefAlignment + score.Specificity + score.Executability + score.Measurement + score.PlatformAdaptation
	return score
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
