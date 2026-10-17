package rules

import (
	"fmt"

	"github.com/owner/brio/internal/trace"
)

type SubsequenceRule struct {
	Sequence []string
}

func NewSubsequence(seq []string) *SubsequenceRule {
	return &SubsequenceRule{Sequence: seq}
}

func (r *SubsequenceRule) Name() string {
	return "subsequence"
}

func (r *SubsequenceRule) Check(t trace.Trace) Result {
	calls := t.ToolCalls()
	subIdx := 0

	for _, call := range calls {
		if subIdx < len(r.Sequence) && call.Name == r.Sequence[subIdx] {
			subIdx++
			if subIdx == len(r.Sequence) {
				break
			}
		}
	}

	if subIdx < len(r.Sequence) {
		return Result{
			Pass:    false,
			Step:    0,
			Message: fmt.Sprintf("subsequence element %q (index %d) not matched", r.Sequence[subIdx], subIdx),
		}
	}

	return Result{Pass: true}
}
