package main

import (
	"errors"
	"regexp"
	"testing"
)

func TestNewUUIDv4(t *testing.T) {
	value, err := newUUIDv4()
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(value) {
		t.Fatalf("invalid UUID v4 shape")
	}
}

func TestClassifyExecutionResult(t *testing.T) {
	tests := []struct {
		name         string
		businessCode int
		writeErr     error
		queryErr     error
		matches      []map[string]any
		responseID   string
		queryID      string
		want         string
	}{
		{
			name:         "business rejection with completed reconciliation",
			businessCode: 50100,
			matches:      []map[string]any{},
			want:         "deterministic_rejection",
		},
		{
			name:     "transport failure remains unknown",
			writeErr: errors.New("connection reset"),
			matches:  []map[string]any{},
			want:     "result_unknown",
		},
		{
			name:       "one reconciled object is confirmed",
			matches:    []map[string]any{{"id": "123"}},
			responseID: "123",
			queryID:    "123",
			want:       "confirmed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := classifyExecutionResult(test.businessCode, test.writeErr, test.queryErr, test.matches, test.responseID, test.queryID)
			if got != test.want {
				t.Fatalf("result=%q want=%q", got, test.want)
			}
		})
	}
}
