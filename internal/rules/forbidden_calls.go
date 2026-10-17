package rules

import (
	"fmt"

	"github.com/iokuru/brio/internal/trace"
)

type ForbiddenCallsRule struct {
	Calls []string
}

func NewForbiddenCalls(calls []string) *ForbiddenCallsRule {
	return &ForbiddenCallsRule{Calls: calls}
}

func (r *ForbiddenCallsRule) Name() string {
	return "forbidden_calls"
}

func (r *ForbiddenCallsRule) Check(t trace.Trace) Result {
	forbidden := make(map[string]bool, len(r.Calls))
	for _, call := range r.Calls {
		forbidden[call] = true
	}

	for _, call := range t.ToolCalls() {
		if forbidden[call.Name] {
			return Result{
				Pass:    false,
				Step:    call.Step,
				Message: fmt.Sprintf("forbidden tool called: %s", call.Name),
			}
		}
	}

	return Result{Pass: true}
}
