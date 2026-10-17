package report

import (
	"fmt"
	"strings"
)

func FormatText(tr TraceReport) string {
	var b strings.Builder
	maxNameLen := 0
	for _, r := range tr.Results {
		if len(r.Name) > maxNameLen {
			maxNameLen = len(r.Name)
		}
	}

	for _, r := range tr.Results {
		if r.Pass {
			b.WriteString(fmt.Sprintf("PASS %s\n", r.Name))
		} else {
			if r.Step > 0 {
				b.WriteString(fmt.Sprintf("FAIL %-*s step %d: %s\n", maxNameLen, r.Name, r.Step, r.Message))
			} else {
				b.WriteString(fmt.Sprintf("FAIL %-*s %s\n", maxNameLen, r.Name, r.Message))
			}
		}
	}

	b.WriteString(fmt.Sprintf("%d of %d rules failed\n", tr.Failed, tr.Total))
	return b.String()
}

func FormatMultiText(reports []TraceReport) string {
	var b strings.Builder
	failedTraces := 0

	for i, tr := range reports {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("=== %s ===\n", tr.Path))
		b.WriteString(FormatText(tr))
		if !tr.Pass {
			failedTraces++
		}
	}

	b.WriteString(fmt.Sprintf("\nSummary: %d of %d traces failed\n", failedTraces, len(reports)))
	return b.String()
}
