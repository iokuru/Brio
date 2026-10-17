package rules

import (
	"testing"

	"github.com/iokuru/brio/internal/trace"
)

func TestRequiredCallsRule(t *testing.T) {
	tests := []struct {
		name     string
		required []string
		trace    trace.Trace
		wantPass bool
		wantStep int
		wantSub  string
	}{
		{
			name:     "all required calls present",
			required: []string{"lookup_user", "issue_refund"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "model"},
					{Step: 2, Type: "tool_call", Name: "lookup_user"},
					{Step: 3, Type: "tool_result", Name: "lookup_user"},
					{Step: 4, Type: "tool_call", Name: "issue_refund"},
				},
			},
			wantPass: true,
		},
		{
			name:     "one required call missing",
			required: []string{"lookup_user", "issue_refund"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "model"},
					{Step: 2, Type: "tool_call", Name: "lookup_user"},
				},
			},
			wantPass: false,
			wantStep: 0,
			wantSub:  "missing required call: issue_refund",
		},
		{
			name:     "multiple required calls missing",
			required: []string{"a", "b", "c"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "a"},
				},
			},
			wantPass: false,
			wantStep: 0,
			wantSub:  "missing required call: b, c",
		},
		{
			name:     "empty trace with no calls",
			required: []string{"lookup_user"},
			trace:    trace.Trace{},
			wantPass: false,
			wantStep: 0,
			wantSub:  "missing required call: lookup_user",
		},
		{
			name:     "tool_result does not count as call",
			required: []string{"lookup_user"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_result", Name: "lookup_user"},
				},
			},
			wantPass: false,
			wantStep: 0,
			wantSub:  "missing required call: lookup_user",
		},
		{
			name:     "repeated calls satisfy requirement",
			required: []string{"lookup_user"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "lookup_user"},
					{Step: 2, Type: "tool_call", Name: "lookup_user"},
				},
			},
			wantPass: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rule := NewRequiredCalls(tc.required)
			if rule.Name() != "required_calls" {
				t.Errorf("expected name required_calls, got %q", rule.Name())
			}

			res := rule.Check(tc.trace)
			if res.Pass != tc.wantPass {
				t.Fatalf("expected Pass=%v, got %v (msg: %s)", tc.wantPass, res.Pass, res.Message)
			}
			if !tc.wantPass {
				if res.Step != tc.wantStep {
					t.Errorf("expected Step=%d, got %d", tc.wantStep, res.Step)
				}
				if tc.wantSub != "" && res.Message != tc.wantSub {
					t.Errorf("expected Message=%q, got %q", tc.wantSub, res.Message)
				}
			}
		})
	}
}
