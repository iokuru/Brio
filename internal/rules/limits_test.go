package rules

import (
	"testing"

	"github.com/iokuru/brio/internal/trace"
)

func TestLimitsRule(t *testing.T) {
	intPtr := func(v int) *int { return &v }
	floatPtr := func(v float64) *float64 { return &v }
	int64Ptr := func(v int64) *int64 { return &v }

	tests := []struct {
		name     string
		rule     *LimitsRule
		trace    trace.Trace
		wantPass bool
		wantStep int
		wantSub  string
	}{
		{
			name: "within all limits",
			rule: NewLimits(intPtr(5), floatPtr(1.0), int64Ptr(1000), intPtr(2)),
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "model", CostUSD: 0.1, LatencyMS: 200},
					{Step: 2, Type: "tool_call", Name: "lookup", CostUSD: 0.1, LatencyMS: 200},
					{Step: 3, Type: "tool_call", Name: "lookup", CostUSD: 0.1, LatencyMS: 200},
				},
			},
			wantPass: true,
		},
		{
			name: "exact boundaries pass",
			rule: NewLimits(intPtr(2), floatPtr(0.2), int64Ptr(400), intPtr(1)),
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "lookup", CostUSD: 0.1, LatencyMS: 200},
					{Step: 2, Type: "tool_call", Name: "refund", CostUSD: 0.1, LatencyMS: 200},
				},
			},
			wantPass: true,
		},
		{
			name: "exceeds max_repeat_calls",
			rule: NewLimits(nil, nil, nil, intPtr(3)),
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "lookup_user"},
					{Step: 2, Type: "tool_call", Name: "lookup_user"},
					{Step: 5, Type: "tool_call", Name: "lookup_user"},
					{Step: 11, Type: "tool_call", Name: "lookup_user"},
				},
			},
			wantPass: false,
			wantStep: 11,
			wantSub:  "lookup_user called 4 times (max_repeat_calls: 3)",
		},
		{
			name: "exceeds max_steps",
			rule: NewLimits(intPtr(2), nil, nil, nil),
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "model"},
					{Step: 2, Type: "tool_call", Name: "lookup"},
					{Step: 3, Type: "tool_result", Name: "lookup"},
				},
			},
			wantPass: false,
			wantStep: 3,
			wantSub:  "exceeded max_steps: 3 (max_steps: 2)",
		},
		{
			name: "exceeds max_cost_usd",
			rule: NewLimits(nil, floatPtr(0.25), nil, nil),
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "model", CostUSD: 0.10},
					{Step: 2, Type: "tool_call", Name: "lookup", CostUSD: 0.10},
					{Step: 3, Type: "tool_result", Name: "lookup", CostUSD: 0.10},
				},
			},
			wantPass: false,
			wantStep: 3,
			wantSub:  "exceeded max_cost_usd: 0.3000 (max_cost_usd: 0.2500)",
		},
		{
			name: "exceeds max_latency_ms",
			rule: NewLimits(nil, nil, int64Ptr(1000), nil),
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "model", LatencyMS: 600},
					{Step: 2, Type: "tool_call", Name: "lookup", LatencyMS: 500},
				},
			},
			wantPass: false,
			wantStep: 2,
			wantSub:  "exceeded max_latency_ms: 1100 (max_latency_ms: 1000)",
		},
		{
			name: "tool_result does not count toward repeat calls",
			rule: NewLimits(nil, nil, nil, intPtr(1)),
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "lookup"},
					{Step: 2, Type: "tool_result", Name: "lookup"},
					{Step: 3, Type: "tool_result", Name: "lookup"},
				},
			},
			wantPass: true,
		},
		{
			name:     "empty trace passes limits",
			rule:     NewLimits(intPtr(5), floatPtr(1.0), int64Ptr(1000), intPtr(1)),
			trace:    trace.Trace{},
			wantPass: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.rule.Name() != "limits" {
				t.Errorf("expected name limits, got %q", tc.rule.Name())
			}

			res := tc.rule.Check(tc.trace)
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
