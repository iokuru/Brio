package trace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseValidTrace(t *testing.T) {
	input := `{"step":1,"type":"model","tokens_in":812,"tokens_out":64,"latency_ms":900,"unknown_field":"ignored"}

{"step":2,"type":"tool_call","name":"lookup_user","args":{"id":"u_42"},"cost_usd":0.002,"latency_ms":340}
{"step":3,"type":"tool_result","name":"lookup_user","ok":true}
`
	tr, err := Parse(strings.NewReader(input), "test.jsonl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tr.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(tr.Steps))
	}
	if tr.Path != "test.jsonl" {
		t.Errorf("expected path test.jsonl, got %q", tr.Path)
	}

	calls := tr.ToolCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}
	if calls[0].Name != "lookup_user" {
		t.Errorf("expected tool call name lookup_user, got %q", calls[0].Name)
	}
	if calls[0].Args["id"] != "u_42" {
		t.Errorf("expected arg id=u_42, got %v", calls[0].Args["id"])
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		errSubstr string
		targetErr error
	}{
		{
			name:      "empty input",
			input:     "",
			targetErr: ErrEmptyTrace,
		},
		{
			name:      "only blank lines",
			input:     "\n  \n\t\n",
			targetErr: ErrEmptyTrace,
		},
		{
			name:      "malformed JSON",
			input:     `{"step":1,"type":"model"}` + "\n" + `{"step":2, bad json}` + "\n",
			errSubstr: "line 2: malformed JSON",
		},
		{
			name:      "first step not 1",
			input:     `{"step":2,"type":"model"}` + "\n",
			errSubstr: "line 1: steps must start at 1, got 2",
		},
		{
			name:      "steps not strictly increasing",
			input:     `{"step":1,"type":"model"}` + "\n" + `{"step":1,"type":"tool_call","name":"a"}` + "\n",
			errSubstr: "line 2: steps must be strictly increasing, got 1 after 1",
		},
		{
			name:      "step decreased",
			input:     `{"step":1,"type":"model"}` + "\n" + `{"step":3,"type":"tool_call","name":"a"}` + "\n" + `{"step":2,"type":"tool_result"}` + "\n",
			errSubstr: "line 3: steps must be strictly increasing, got 2 after 3",
		},
		{
			name:      "missing step field",
			input:     `{"type":"model"}` + "\n",
			errSubstr: "line 1: missing step number",
		},
		{
			name:      "empty type",
			input:     `{"step":1,"type":""}` + "\n",
			errSubstr: "line 1: step type cannot be empty",
		},
		{
			name:      "tool call missing name",
			input:     `{"step":1,"type":"tool_call","name":""}` + "\n",
			errSubstr: "line 1: tool_call must specify name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader(tc.input), "test.jsonl")
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if tc.targetErr != nil && !errors.Is(err, tc.targetErr) {
				t.Errorf("expected error %v, got %v", tc.targetErr, err)
			}
			if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
				t.Errorf("expected error containing %q, got %q", tc.errSubstr, err.Error())
			}
		})
	}
}

func TestParseFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "valid.jsonl")
	content := `{"step":1,"type":"model"}` + "\n"

	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	tr, err := ParseFile(filePath)
	if err != nil {
		t.Fatalf("unexpected error parsing file: %v", err)
	}
	if len(tr.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(tr.Steps))
	}

	_, err = ParseFile(filepath.Join(tmpDir, "nonexistent.jsonl"))
	if err == nil {
		t.Errorf("expected error for nonexistent file, got nil")
	}
}
