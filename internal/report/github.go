package report

import (
	"fmt"
	"strings"
)

func FormatGitHub(tr TraceReport) string {
	var b strings.Builder
	filePath := tr.Path
	if filePath == "" {
		filePath = "trace.jsonl"
	}

	for _, r := range tr.Results {
		if !r.Pass {
			if r.Step > 0 {
				b.WriteString(fmt.Sprintf("::error file=%s,title=Brio::%s step %d: %s\n", filePath, r.Name, r.Step, r.Message))
			} else {
				b.WriteString(fmt.Sprintf("::error file=%s,title=Brio::%s: %s\n", filePath, r.Name, r.Message))
			}
		}
	}

	b.WriteString(fmt.Sprintf("%d of %d rules failed\n", tr.Failed, tr.Total))
	return b.String()
}

func FormatMultiGitHub(reports []TraceReport) string {
	var b strings.Builder
	failedTraces := 0

	for _, tr := range reports {
		filePath := tr.Path
		if filePath == "" {
			filePath = "trace.jsonl"
		}
		if !tr.Pass {
			failedTraces++
		}
		for _, r := range tr.Results {
			if !r.Pass {
				if r.Step > 0 {
					b.WriteString(fmt.Sprintf("::error file=%s,title=Brio::%s step %d: %s\n", filePath, r.Name, r.Step, r.Message))
				} else {
					b.WriteString(fmt.Sprintf("::error file=%s,title=Brio::%s: %s\n", filePath, r.Name, r.Message))
				}
			}
		}
	}

	b.WriteString(fmt.Sprintf("Summary: %d of %d traces failed\n", failedTraces, len(reports)))
	return b.String()
}
