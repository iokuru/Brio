package spec

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidSpec(t *testing.T) {
	input := `name: refund-flow
rules:
  - required_calls: [lookup_user, issue_refund]
  - forbidden_calls: [delete_user]
  - order:
      - before: confirm_user
        after: issue_refund
        immediately: false
  - subsequence: [lookup_user, confirm_user, issue_refund]
  - arg_match:
      call: issue_refund
      arg: amount
      regex: '^\d{1,4}(\.\d{2})?$'
  - limits:
      max_steps: 20
      max_cost_usd: 0.25
      max_latency_ms: 15000
      max_repeat_calls: 3
`
	sp, err := Load(strings.NewReader(input), "spec.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sp.Name != "refund-flow" {
		t.Errorf("expected name refund-flow, got %q", sp.Name)
	}
	if len(sp.Rules) != 6 {
		t.Fatalf("expected 6 rules, got %d", len(sp.Rules))
	}

	// Verify required_calls
	r0 := sp.Rules[0]
	if r0.Kind != "required_calls" || len(r0.RequiredCalls) != 2 {
		t.Errorf("unexpected rule 0: %+v", r0)
	}

	// Verify forbidden_calls
	r1 := sp.Rules[1]
	if r1.Kind != "forbidden_calls" || len(r1.ForbiddenCalls) != 1 {
		t.Errorf("unexpected rule 1: %+v", r1)
	}

	// Verify order
	r2 := sp.Rules[2]
	if r2.Kind != "order" || len(r2.Order) != 1 || r2.Order[0].Before != "confirm_user" || r2.Order[0].After != "issue_refund" || r2.Order[0].Immediately {
		t.Errorf("unexpected rule 2: %+v", r2)
	}

	// Verify subsequence
	r3 := sp.Rules[3]
	if r3.Kind != "subsequence" || len(r3.Subsequence) != 3 {
		t.Errorf("unexpected rule 3: %+v", r3)
	}

	// Verify arg_match
	r4 := sp.Rules[4]
	if r4.Kind != "arg_match" || r4.ArgMatch.Call != "issue_refund" || r4.ArgMatch.Arg != "amount" || r4.ArgMatch.Predicate != "regex" {
		t.Errorf("unexpected rule 4: %+v", r4)
	}

	// Verify limits
	r5 := sp.Rules[5]
	if r5.Kind != "limits" || *r5.Limits.MaxSteps != 20 || *r5.Limits.MaxCostUSD != 0.25 || *r5.Limits.MaxLatencyMS != 15000 || *r5.Limits.MaxRepeatCalls != 3 {
		t.Errorf("unexpected rule 5: %+v", r5)
	}

	builtRules := sp.BuildRules()
	if len(builtRules) != 6 {
		t.Fatalf("expected 6 built rules, got %d", len(builtRules))
	}
	expectedNames := []string{"required_calls", "forbidden_calls", "order", "subsequence", "arg_match", "limits"}
	for i, r := range builtRules {
		if r.Name() != expectedNames[i] {
			t.Errorf("rule %d name expected %q, got %q", i, expectedNames[i], r.Name())
		}
	}
}

func TestLoadValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		errSubstr string
	}{
		{
			name:      "empty file",
			input:     "",
			errSubstr: "spec file is empty",
		},
		{
			name:      "root not mapping",
			input:     "- a\n- b\n",
			errSubstr: "spec root must be a mapping",
		},
		{
			name:      "unknown top-level key",
			input:     "name: foo\nfoo: bar\nrules:\n  - required_calls: [a]\n",
			errSubstr: `unknown spec key "foo"`,
		},
		{
			name:      "missing name",
			input:     "rules:\n  - required_calls: [a]\n",
			errSubstr: "spec name is required",
		},
		{
			name:      "missing rules list",
			input:     "name: foo\n",
			errSubstr: "spec rules list is required",
		},
		{
			name:      "empty rules list",
			input:     "name: foo\nrules: []\n",
			errSubstr: "rules must be a non-empty list",
		},
		{
			name:      "rule not mapping",
			input:     "name: foo\nrules:\n  - a\n",
			errSubstr: "rule 0: rule must be a mapping",
		},
		{
			name:      "empty rule definition",
			input:     "name: foo\nrules:\n  - {}\n",
			errSubstr: "rule 0: empty rule definition",
		},
		{
			name:      "rule with multiple keys",
			input:     "name: foo\nrules:\n  - required_calls: [a]\n    forbidden_calls: [b]\n",
			errSubstr: "rule 0: rule must contain exactly one rule key",
		},
		{
			name:      "unknown rule key",
			input:     "name: foo\nrules:\n  - unknown_rule: [a]\n",
			errSubstr: `rule 0: unknown rule key "unknown_rule"`,
		},
		{
			name:      "required_calls empty list",
			input:     "name: foo\nrules:\n  - required_calls: []\n",
			errSubstr: "rule 0: required_calls list cannot be empty",
		},
		{
			name:      "forbidden_calls empty item",
			input:     "name: foo\nrules:\n  - forbidden_calls: ['']\n",
			errSubstr: "rule 0: forbidden_calls items must be non-empty strings",
		},
		{
			name:      "order empty list",
			input:     "name: foo\nrules:\n  - order: []\n",
			errSubstr: "rule 0: order list cannot be empty",
		},
		{
			name:      "order item missing before",
			input:     "name: foo\nrules:\n  - order:\n      - after: b\n",
			errSubstr: "rule 0: order before is required",
		},
		{
			name:      "order item missing after",
			input:     "name: foo\nrules:\n  - order:\n      - before: a\n",
			errSubstr: "rule 0: order after is required",
		},
		{
			name:      "order item unknown key",
			input:     "name: foo\nrules:\n  - order:\n      - before: a\n        after: b\n        extra: true\n",
			errSubstr: `rule 0: order unknown key "extra"`,
		},
		{
			name:      "subsequence empty list",
			input:     "name: foo\nrules:\n  - subsequence: []\n",
			errSubstr: "rule 0: subsequence list cannot be empty",
		},
		{
			name:      "arg_match missing call",
			input:     "name: foo\nrules:\n  - arg_match:\n      arg: x\n      present: true\n",
			errSubstr: "rule 0: arg_match call is required",
		},
		{
			name:      "arg_match missing arg",
			input:     "name: foo\nrules:\n  - arg_match:\n      call: x\n      present: true\n",
			errSubstr: "rule 0: arg_match arg is required",
		},
		{
			name:      "arg_match no predicate",
			input:     "name: foo\nrules:\n  - arg_match:\n      call: x\n      arg: y\n",
			errSubstr: "rule 0: arg_match must specify exactly one predicate",
		},
		{
			name:      "arg_match multiple predicates",
			input:     "name: foo\nrules:\n  - arg_match:\n      call: x\n      arg: y\n      equals: 'a'\n      present: true\n",
			errSubstr: "rule 0: arg_match must specify exactly one predicate",
		},
		{
			name:      "arg_match invalid regex",
			input:     "name: foo\nrules:\n  - arg_match:\n      call: x\n      arg: y\n      regex: '[unclosed'\n",
			errSubstr: "rule 0: arg_match invalid regex",
		},
		{
			name:      "limits empty",
			input:     "name: foo\nrules:\n  - limits: {}\n",
			errSubstr: "rule 0: limits must specify at least one limit",
		},
		{
			name:      "limits unknown key",
			input:     "name: foo\nrules:\n  - limits:\n      max_steps: 10\n      extra_limit: 5\n",
			errSubstr: `rule 0: limits unknown key "extra_limit"`,
		},
		{
			name:      "limits negative max_steps",
			input:     "name: foo\nrules:\n  - limits:\n      max_steps: -1\n",
			errSubstr: "rule 0: limits max_steps cannot be negative: -1",
		},
		{
			name:      "limits negative max_cost_usd",
			input:     "name: foo\nrules:\n  - limits:\n      max_cost_usd: -0.5\n",
			errSubstr: "rule 0: limits max_cost_usd cannot be negative: -0.5",
		},
		{
			name:      "limits negative max_latency_ms",
			input:     "name: foo\nrules:\n  - limits:\n      max_latency_ms: -100\n",
			errSubstr: "rule 0: limits max_latency_ms cannot be negative: -100",
		},
		{
			name:      "limits negative max_repeat_calls",
			input:     "name: foo\nrules:\n  - limits:\n      max_repeat_calls: -3\n",
			errSubstr: "rule 0: limits max_repeat_calls cannot be negative: -3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(strings.NewReader(tc.input), "test.yaml")
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			var valErr *ValidationError
			if !errors.As(err, &valErr) && !strings.Contains(err.Error(), "test.yaml") {
				t.Errorf("expected ValidationError or file prefix, got %v", err)
			}
			if !strings.Contains(err.Error(), tc.errSubstr) {
				t.Errorf("expected error containing %q, got %q", tc.errSubstr, err.Error())
			}
		})
	}
}

func TestLoadFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "spec.yaml")
	content := "name: test\nrules:\n  - required_calls: [a]\n"
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	sp, err := LoadFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sp.Name != "test" {
		t.Errorf("expected name test, got %q", sp.Name)
	}

	_, err = LoadFile(filepath.Join(tmpDir, "nonexistent.yaml"))
	if err == nil {
		t.Errorf("expected error for nonexistent file, got nil")
	}
}
