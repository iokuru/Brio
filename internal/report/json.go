package report

import (
	"encoding/json"
	"fmt"
)

type MultiTraceReport struct {
	Pass   bool          `json:"pass"`
	Failed int           `json:"failed"`
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
	failed := 0
	for _, r := range reports {
		if !r.Pass {
			failed++
		}
	}
	mtr := MultiTraceReport{
		Pass:   failed == 0,
		Failed: failed,
		Total:  len(reports),
		Traces: reports,
	}
	data, err := json.MarshalIndent(mtr, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal multi json report: %w", err)
	}
	return string(data) + "\n", nil
}
