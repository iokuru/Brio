package rules

import (
	"testing"

	"github.com/iokuru/brio/internal/trace"
)

func TestOrderRule(t *testing.T) {
	tests := []struct {
		name     string
		pairs    []OrderPair
		trace    trace.Trace
		wantPass bool
		wantStep int
		wantSub  string
	}{
		{
			name: "before precedes after",
			pairs: []OrderPair{
				{Before: "confirm_user", After: "issue_refund", Immediately: false},
			},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "confirm_user"},
					{Step: 2, Type: "tool_call", Name: "issue_refund"},
				},
			},
			wantPass: true,
		},
		{
			name: "before precedes after with interleaved other calls",
			pairs: []OrderPair{
				{Before: "confirm_user", After: "issue_refund", Immediately: false},
			},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "confirm_user"},
					{Step: 2, Type: "tool_call", Name: "audit_log"},
					{Step: 3, Type: "tool_call", Name: "issue_refund"},
				},
			},
			wantPass: true,
		},
		{
			name: "multiple before calls all precede after",
			pairs: []OrderPair{
				{Before: "confirm_user", After: "issue_refund", Immediately: false},
			},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "confirm_user"},
					{Step: 2, Type: "tool_call", Name: "confirm_user"},
					{Step: 3, Type: "tool_call", Name: "issue_refund"},
				},
			},
			wantPass: true,
		},
		{
			name: "after called before before",
			pairs: []OrderPair{
				{Before: "confirm_user", After: "issue_refund", Immediately: false},
			},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "lookup_user"},
					{Step: 4, Type: "tool_call", Name: "issue_refund"},
					{Step: 5, Type: "tool_call", Name: "confirm_user"},
				},
			},
			wantPass: false,
			wantStep: 4,
			wantSub:  "issue_refund called before confirm_user",
		},
		{
			name: "after appears and before never does",
			pairs: []OrderPair{
				{Before: "confirm_user", After: "issue_refund", Immediately: false},
			},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "lookup_user"},
					{Step: 4, Type: "tool_call", Name: "issue_refund"},
				},
			},
			wantPass: false,
			wantStep: 4,
			wantSub:  "issue_refund called before confirm_user",
		},
		{
			name: "before called again after after",
			pairs: []OrderPair{
				{Before: "confirm_user", After: "issue_refund", Immediately: false},
			},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 2, Type: "tool_call", Name: "confirm_user"},
					{Step: 4, Type: "tool_call", Name: "issue_refund"},
					{Step: 6, Type: "tool_call", Name: "confirm_user"},
				},
			},
			wantPass: false,
			wantStep: 6,
			wantSub:  "confirm_user called after issue_refund",
		},
		{
			name: "neither tool appears",
			pairs: []OrderPair{
				{Before: "confirm_user", After: "issue_refund", Immediately: false},
			},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "lookup_user"},
				},
			},
			wantPass: true,
		},
		{
			name: "before appears but after never does with immediately false",
			pairs: []OrderPair{
				{Before: "confirm_user", After: "issue_refund", Immediately: false},
			},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "confirm_user"},
				},
			},
			wantPass: true,
		},
		{
			name: "immediately true satisfied directly",
			pairs: []OrderPair{
				{Before: "confirm_user", After: "issue_refund", Immediately: true},
			},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "confirm_user"},
					{Step: 2, Type: "tool_call", Name: "issue_refund"},
				},
			},
			wantPass: true,
		},
		{
			name: "immediately true satisfied ignoring non tool_call steps",
			pairs: []OrderPair{
				{Before: "confirm_user", After: "issue_refund", Immediately: true},
			},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "confirm_user"},
					{Step: 2, Type: "tool_result", Name: "confirm_user"},
					{Step: 3, Type: "model"},
					{Step: 4, Type: "tool_call", Name: "issue_refund"},
				},
			},
			wantPass: true,
		},
		{
			name: "immediately true fails when interleaved with another tool_call",
			pairs: []OrderPair{
				{Before: "confirm_user", After: "issue_refund", Immediately: true},
			},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "confirm_user"},
					{Step: 2, Type: "tool_call", Name: "audit_log"},
					{Step: 3, Type: "tool_call", Name: "issue_refund"},
				},
			},
			wantPass: false,
			wantStep: 2,
			wantSub:  "audit_log called after confirm_user, expected issue_refund",
		},
		{
			name: "immediately true fails when before is the last tool call",
			pairs: []OrderPair{
				{Before: "confirm_user", After: "issue_refund", Immediately: true},
			},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "confirm_user"},
					{Step: 2, Type: "tool_result", Name: "confirm_user"},
				},
			},
			wantPass: false,
			wantStep: 1,
			wantSub:  "confirm_user not immediately followed by issue_refund",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rule := NewOrder(tc.pairs)
			if rule.Name() != "order" {
				t.Errorf("expected name order, got %q", rule.Name())
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
