package rules

import (
	"regexp"
	"strings"
	"testing"

	"github.com/iokuru/brio/internal/trace"
)

func TestArgMatchRule(t *testing.T) {
	re := regexp.MustCompile(`^\d{1,4}(\.\d{2})?$`)

	tests := []struct {
		name     string
		rule     *ArgMatchRule
		trace    trace.Trace
		wantPass bool
		wantStep int
		wantSub  string
	}{
		{
			name: "regex matches successfully",
			rule: NewArgMatch("issue_refund", "amount", "regex", nil, re, re.String(), false),
			trace: trace.Trace{
				Steps: []trace.Step{
					{
						Step: 2,
						Type: "tool_call",
						Name: "issue_refund",
						Args: map[string]any{"amount": "49.99"},
					},
				},
			},
			wantPass: true,
		},
		{
			name: "regex fails format",
			rule: NewArgMatch("issue_refund", "amount", "regex", nil, re, re.String(), false),
			trace: trace.Trace{
				Steps: []trace.Step{
					{
						Step: 4,
						Type: "tool_call",
						Name: "issue_refund",
						Args: map[string]any{"amount": "not_a_number"},
					},
				},
			},
			wantPass: false,
			wantStep: 4,
			wantSub:  `issue_refund argument "amount" with value "not_a_number" did not match regex`,
		},
		{
			name: "regex fails on missing argument",
			rule: NewArgMatch("issue_refund", "amount", "regex", nil, re, re.String(), false),
			trace: trace.Trace{
				Steps: []trace.Step{
					{
						Step: 5,
						Type: "tool_call",
						Name: "issue_refund",
						Args: map[string]any{"other": 123},
					},
				},
			},
			wantPass: false,
			wantStep: 5,
			wantSub:  `issue_refund missing argument "amount"`,
		},
		{
			name: "equals matches string",
			rule: NewArgMatch("lookup_user", "id", "equals", "u_42", nil, "", false),
			trace: trace.Trace{
				Steps: []trace.Step{
					{
						Step: 2,
						Type: "tool_call",
						Name: "lookup_user",
						Args: map[string]any{"id": "u_42"},
					},
				},
			},
			wantPass: true,
		},
		{
			name: "equals matches numeric values with different representation",
			rule: NewArgMatch("set_limit", "count", "equals", 42, nil, "", false),
			trace: trace.Trace{
				Steps: []trace.Step{
					{
						Step: 2,
						Type: "tool_call",
						Name: "set_limit",
						Args: map[string]any{"count": float64(42)},
					},
				},
			},
			wantPass: true,
		},
		{
			name: "equals fails on mismatched value",
			rule: NewArgMatch("lookup_user", "id", "equals", "u_42", nil, "", false),
			trace: trace.Trace{
				Steps: []trace.Step{
					{
						Step: 3,
						Type: "tool_call",
						Name: "lookup_user",
						Args: map[string]any{"id": "u_99"},
					},
				},
			},
			wantPass: false,
			wantStep: 3,
			wantSub:  `issue_refund` + ``, // will check lookup_user below
		},
		{
			name: "present true succeeds when present",
			rule: NewArgMatch("confirm_user", "token", "present", nil, nil, "", true),
			trace: trace.Trace{
				Steps: []trace.Step{
					{
						Step: 1,
						Type: "tool_call",
						Name: "confirm_user",
						Args: map[string]any{"token": "xyz"},
					},
				},
			},
			wantPass: true,
		},
		{
			name: "present true fails when missing",
			rule: NewArgMatch("confirm_user", "token", "present", nil, nil, "", true),
			trace: trace.Trace{
				Steps: []trace.Step{
					{
						Step: 1,
						Type: "tool_call",
						Name: "confirm_user",
						Args: map[string]any{},
					},
				},
			},
			wantPass: false,
			wantStep: 1,
			wantSub:  `confirm_user missing required argument "token"`,
		},
		{
			name: "present false succeeds when missing",
			rule: NewArgMatch("delete_user", "force", "present", nil, nil, "", false),
			trace: trace.Trace{
				Steps: []trace.Step{
					{
						Step: 1,
						Type: "tool_call",
						Name: "delete_user",
						Args: map[string]any{"id": "123"},
					},
				},
			},
			wantPass: true,
		},
		{
			name: "present false fails when present",
			rule: NewArgMatch("delete_user", "force", "present", nil, nil, "", false),
			trace: trace.Trace{
				Steps: []trace.Step{
					{
						Step: 2,
						Type: "tool_call",
						Name: "delete_user",
						Args: map[string]any{"force": true},
					},
				},
			},
			wantPass: false,
			wantStep: 2,
			wantSub:  `delete_user unexpected argument "force" present`,
		},
		{
			name: "tool not called vacuously passes",
			rule: NewArgMatch("issue_refund", "amount", "present", nil, nil, "", true),
			trace: trace.Trace{
				Steps: []trace.Step{
					{Step: 1, Type: "tool_call", Name: "lookup_user"},
				},
			},
			wantPass: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.rule.Name() != "arg_match" {
				t.Errorf("expected name arg_match, got %q", tc.rule.Name())
			}

			res := tc.rule.Check(tc.trace)
			if res.Pass != tc.wantPass {
				t.Fatalf("expected Pass=%v, got %v (msg: %s)", tc.wantPass, res.Pass, res.Message)
			}
			if !tc.wantPass {
				if res.Step != tc.wantStep {
					t.Errorf("expected Step=%d, got %d", tc.wantStep, res.Step)
				}
				if tc.name == "equals fails on mismatched value" {
					if res.Message != `lookup_user argument "id" value u_99 does not equal u_42` {
						t.Errorf("unexpected message: %q", res.Message)
					}
				} else if tc.wantSub != "" && !strings.Contains(res.Message, tc.wantSub) {
					t.Errorf("expected Message containing %q, got %q", tc.wantSub, res.Message)
				}
			}
		})
	}
}
