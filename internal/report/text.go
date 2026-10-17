package report

import (
	"fmt"
	"strings"
)

func FormatText(tr TraceReport) string {
	if tr.Error != "" {
		return fmt.Sprintf("ERROR %s: %s\n", tr.Path, tr.Error)
	}

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
	passed := 0
	failed := 0
	errorsCount := 0

	for _, tr := range reports {
		if tr.Error != "" {
			errorsCount++
			b.WriteString(fmt.Sprintf("ERROR %s: %s\n", tr.Path, tr.Error))
		} else if !tr.Pass {
			failed++
			b.WriteString(fmt.Sprintf("FAIL  %s (%d of %d rules failed)\n", tr.Path, tr.Failed, tr.Total))
		} else {
			passed++
			b.WriteString(fmt.Sprintf("PASS  %s\n", tr.Path))
		}
	}

	b.WriteString(fmt.Sprintf("%d passed, %d failed, %d errors\n", passed, failed, errorsCount))
	return b.String()
}
