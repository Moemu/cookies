package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/shikanon/cookies/internal/systems/strategy"
	strategyeval "github.com/shikanon/cookies/internal/systems/strategy/eval"
)

func main() {
	strategyFile := flag.String("strategy", "", "path to a StrategyDocument JSON file; omitted to list the offline cases")
	caseID := flag.String("case", "", "Golden Case ID to evaluate")
	flag.Parse()
	cases, err := strategyeval.LoadCases()
	if err != nil {
		log.Fatal(err)
	}
	if *strategyFile != "" {
		runEvaluation(*strategyFile, *caseID, cases)
		return
	}
	result := struct {
		Mode  string              `json:"mode"`
		Cases []strategyeval.Case `json:"cases"`
	}{
		Mode: "offline-no-provider-calls", Cases: cases,
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		log.Fatal(err)
	}
}

func runEvaluation(strategyFile, caseID string, cases []strategyeval.Case) {
	if caseID == "" {
		log.Fatal("-case is required with -strategy")
	}
	content, err := os.ReadFile(strategyFile)
	if err != nil {
		log.Fatal(err)
	}
	var document strategy.StrategyDocument
	if err := json.Unmarshal(content, &document); err != nil {
		log.Fatal(err)
	}
	for _, testCase := range cases {
		if testCase.ID != caseID {
			continue
		}
		result := struct {
			Mode  string             `json:"mode"`
			Score strategyeval.Score `json:"score"`
		}{
			Mode:  "offline-no-provider-calls",
			Score: strategyeval.Evaluate(testCase, document),
		}
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(result); err != nil {
			log.Fatal(err)
		}
		return
	}
	log.Fatalf("Golden Case %q was not found", caseID)
}
