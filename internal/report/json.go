package report

import (
	"encoding/json"
	"fmt"
)

type MultiTraceReport struct {
	Pass   bool          `json:"pass"`
	Passed int           `json:"passed"`
	Failed int           `json:"failed"`
	Errors int           `json:"errors"`
	Total  int           `json:"total"`
	Traces []TraceReport `json:"traces"`
}

func FormatJSON(tr TraceReport) (string, error) {
	data, err := json.MarshalIndent(tr, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal json report: %w", err)
	}
	return string(data) + "\n", nil
}

func FormatMultiJSON(reports []TraceReport) (string, error) {
	passed := 0
	failed := 0
	errorsCount := 0
	for _, r := range reports {
		if r.Error != "" {
			errorsCount++
		} else if !r.Pass {
			failed++
		} else {
			passed++
		}
	}
	mtr := MultiTraceReport{
		Pass:   failed == 0 && errorsCount == 0,
		Passed: passed,
		Failed: failed,
		Errors: errorsCount,
		Total:  len(reports),
		Traces: reports,
	}
	data, err := json.MarshalIndent(mtr, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal multi json report: %w", err)
	}
	return string(data) + "\n", nil
}
