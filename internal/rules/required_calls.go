package rules

import (
	"fmt"
	"strings"

	"github.com/owner/brio/internal/trace"
)

type RequiredCallsRule struct {
	Calls []string
}

func NewRequiredCalls(calls []string) *RequiredCallsRule {
	return &RequiredCallsRule{Calls: calls}
}

func (r *RequiredCallsRule) Name() string {
	return "required_calls"
}

func (r *RequiredCallsRule) Check(t trace.Trace) Result {
	seen := make(map[string]bool)
	for _, call := range t.ToolCalls() {
		seen[call.Name] = true
	}

	var missing []string
	for _, required := range r.Calls {
		if !seen[required] {
			missing = append(missing, required)
		}
	}

	if len(missing) > 0 {
		return Result{
			Pass:    false,
			Step:    0,
			Message: fmt.Sprintf("missing required call: %s", strings.Join(missing, ", ")),
		}
	}

	return Result{Pass: true}
}
