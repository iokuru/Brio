package rules

import (
	"testing"

	"github.com/owner/brio/internal/trace"
)

func TestSubsequenceRule(t *testing.T) {
	tests := []struct {
		name     string
		seq      []string
		trace    trace.Trace
		wantPass bool
		wantStep int
		wantSub  string
	}{
		{
			name: "exact consecutive subsequence match",
			seq:  []string{"lookup_user", "confirm_user", "issue_refund"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "lookup_user"},
					{Step: 2, Type: "tool_call", Name: "confirm_user"},
					{Step: 3, Type: "tool_call", Name: "issue_refund"},
				},
			},
			wantPass: true,
		},
		{
			name: "interleaved calls allowed",
			seq:  []string{"lookup_user", "confirm_user", "issue_refund"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "lookup_user"},
					{Step: 2, Type: "tool_call", Name: "check_balance"},
					{Step: 3, Type: "tool_call", Name: "confirm_user"},
					{Step: 4, Type: "tool_call", Name: "log_event"},
					{Step: 5, Type: "tool_call", Name: "issue_refund"},
				},
			},
			wantPass: true,
		},
		{
			name: "greedy matching with repeated items",
			seq:  []string{"lookup_user", "confirm_user", "lookup_user"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "lookup_user"},
					{Step: 2, Type: "tool_call", Name: "lookup_user"},
					{Step: 3, Type: "tool_call", Name: "confirm_user"},
					{Step: 4, Type: "tool_call", Name: "lookup_user"},
				},
			},
			wantPass: true,
		},
		{
			name: "third element missing",
			seq:  []string{"lookup_user", "confirm_user", "issue_refund"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "lookup_user"},
					{Step: 2, Type: "tool_call", Name: "confirm_user"},
				},
			},
			wantPass: false,
			wantStep: 0,
			wantSub:  `subsequence element "issue_refund" (index 2) not matched`,
		},
		{
			name: "out of order match fails at remaining element",
			seq:  []string{"lookup_user", "confirm_user", "issue_refund"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "confirm_user"},
					{Step: 2, Type: "tool_call", Name: "lookup_user"},
					{Step: 3, Type: "tool_call", Name: "issue_refund"},
				},
			},
			wantPass: false,
			wantStep: 0,
			wantSub:  `subsequence element "confirm_user" (index 1) not matched`,
		},
		{
			name: "first element never appears",
			seq:  []string{"lookup_user", "confirm_user"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "confirm_user"},
				},
			},
			wantPass: false,
			wantStep: 0,
			wantSub:  `subsequence element "lookup_user" (index 0) not matched`,
		},
		{
			name:     "empty trace fails at first element",
			seq:      []string{"lookup_user"},
			trace:    trace.Trace{},
			wantPass: false,
			wantStep: 0,
			wantSub:  `subsequence element "lookup_user" (index 0) not matched`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rule := NewSubsequence(tc.seq)
			if rule.Name() != "subsequence" {
				t.Errorf("expected name subsequence, got %q", rule.Name())
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
