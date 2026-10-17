package report

import (
	"github.com/owner/brio/internal/rules"
)

type RuleResult struct {
	Name    string `json:"name"`
	Pass    bool   `json:"pass"`
	Step    int    `json:"step,omitempty"`
	Message string `json:"message,omitempty"`
}

type TraceReport struct {
	Path    string       `json:"path,omitempty"`
	Pass    bool         `json:"pass"`
	Failed  int          `json:"failed"`
	Total   int          `json:"total"`
	Results []RuleResult `json:"results"`
}

func NewTraceReport(path string, ruleList []rules.Rule, results []rules.Result) TraceReport {
	failed := 0
	var rResults []RuleResult

	for i, r := range results {
		name := ""
		if i < len(ruleList) {
			name = ruleList[i].Name()
		}
		if !r.Pass {
			failed++
		}
		rResults = append(rResults, RuleResult{
			Name:    name,
			Pass:    r.Pass,
			Step:    r.Step,
			Message: r.Message,
		})
	}

	return TraceReport{
		Path:    path,
		Pass:    failed == 0,
		Failed:  failed,
		Total:   len(results),
		Results: rResults,
	}
}
