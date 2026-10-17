package rules

import (
	"testing"

	"github.com/iokuru/brio/internal/trace"
)

func TestForbiddenCallsRule(t *testing.T) {
	tests := []struct {
		name      string
		forbidden []string
		trace     trace.Trace
		wantPass  bool
		wantStep  int
		wantSub   string
	}{
		{
			name:      "none of forbidden calls appear",
			forbidden: []string{"delete_user", "drop_db"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "model"},
					{Step: 2, Type: "tool_call", Name: "lookup_user"},
					{Step: 3, Type: "tool_call", Name: "issue_refund"},
				},
			},
			wantPass: true,
		},
		{
			name:      "single forbidden call appears",
			forbidden: []string{"delete_user"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "model"},
					{Step: 2, Type: "tool_call", Name: "lookup_user"},
					{Step: 3, Type: "tool_call", Name: "delete_user"},
				},
			},
			wantPass: false,
			wantStep: 3,
			wantSub:  "forbidden tool called: delete_user",
		},
		{
			name:      "earliest forbidden call reported when multiple appear",
			forbidden: []string{"delete_user", "drop_db"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "lookup_user"},
					{Step: 2, Type: "tool_call", Name: "drop_db"},
					{Step: 3, Type: "tool_call", Name: "delete_user"},
				},
			},
			wantPass: false,
			wantStep: 2,
			wantSub:  "forbidden tool called: drop_db",
		},
		{
			name:      "empty trace has no forbidden calls",
			forbidden: []string{"delete_user"},
			trace:     trace.Trace{},
			wantPass:  true,
		},
		{
			name:      "tool_result with forbidden name does not count",
			forbidden: []string{"delete_user"},
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_result", Name: "delete_user"},
				},
			},
			wantPass: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rule := NewForbiddenCalls(tc.forbidden)
			if rule.Name() != "forbidden_calls" {
				t.Errorf("expected name forbidden_calls, got %q", rule.Name())
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
