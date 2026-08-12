package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/shikanon/cookies/internal/platform/knowledge"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(arguments []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("cookies-eval-document-vision", flag.ContinueOnError)
	flags.SetOutput(stderr)
	inputPath := flags.String("input", "", "path to a versioned document-vision evaluation dataset")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if *inputPath == "" {
		fmt.Fprintln(stderr, "-input is required")
		return 2
	}
	content, err := os.ReadFile(*inputPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var dataset knowledge.DocumentVisionEvaluationDataset
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dataset); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := requireJSONEOF(decoder); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	report, err := knowledge.EvaluateDocumentVision(dataset)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	fmt.Fprintln(stdout, string(encoded))
	return 0
}

func requireJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err == io.EOF {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("evaluation dataset contains trailing JSON values")
}
