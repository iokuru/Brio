package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"version"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "brio version dev") {
		t.Errorf("unexpected output: %s", stdout.String())
	}
}

func TestInitCommand(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "brio.yaml")

	var stdout, stderr bytes.Buffer
	code := run([]string{"init", "--out", outPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wrote starter spec to") {
		t.Errorf("unexpected output: %s", stdout.String())
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if !strings.Contains(string(data), "name: starter-flow") {
		t.Errorf("unexpected starter spec content: %s", string(data))
	}

	// Refuse overwrite without --force
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"init", "--out", outPath}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected code 2 when file exists, got %d", code)
	}
	if !strings.Contains(stderr.String(), "file already exists") {
		t.Errorf("expected file already exists error, got: %s", stderr.String())
	}

	// Overwrite with --force
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"init", "--out", outPath, "--force"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0 with --force, got %d (stderr: %s)", code, stderr.String())
	}
}

func TestCheckCommandValidation(t *testing.T) {
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.yaml")
	if err := os.WriteFile(specFile, []byte("name: test\nrules:\n  - required_calls: [a]\n"), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}
	traceFile := filepath.Join(tmpDir, "trace.jsonl")
	if err := os.WriteFile(traceFile, []byte(`{"step":1,"type":"tool_call","name":"a"}`+"\n"), 0644); err != nil {
		t.Fatalf("failed to write trace: %v", err)
	}

	tests := []struct {
		name     string
		args     []string
		wantCode int
		errSub   string
	}{
		{
			name:     "missing spec",
			args:     []string{"check", "--trace", traceFile},
			wantCode: 2,
			errSub:   "--spec is required",
		},
		{
			name:     "missing both trace and traces",
			args:     []string{"check", "--spec", specFile},
			wantCode: 2,
			errSub:   "specify exactly one of --trace or --traces",
		},
		{
			name:     "both trace and traces specified",
			args:     []string{"check", "--spec", specFile, "--trace", traceFile, "--traces", tmpDir},
			wantCode: 2,
			errSub:   "specify exactly one of --trace or --traces",
		},
		{
			name:     "invalid format",
			args:     []string{"check", "--spec", specFile, "--trace", traceFile, "--format", "xml"},
			wantCode: 2,
			errSub:   "invalid format",
		},
		{
			name:     "invalid spec file path",
			args:     []string{"check", "--spec", filepath.Join(tmpDir, "none.yaml"), "--trace", traceFile},
			wantCode: 2,
			errSub:   "open spec file",
		},
		{
			name:     "invalid trace file path",
			args:     []string{"check", "--spec", specFile, "--trace", filepath.Join(tmpDir, "none.jsonl")},
			wantCode: 2,
			errSub:   "open trace file",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tc.args, &stdout, &stderr)
			if code != tc.wantCode {
				t.Fatalf("expected code %d, got %d (stdout: %s, stderr: %s)", tc.wantCode, code, stdout.String(), stderr.String())
			}
			if tc.errSub != "" && !strings.Contains(stderr.String(), tc.errSub) {
				t.Errorf("expected error containing %q, got %q", tc.errSub, stderr.String())
			}
		})
	}
}

func TestCheckCommandExecution(t *testing.T) {
	tmpDir := t.TempDir()
	specFile := filepath.Join(tmpDir, "spec.yaml")
	specContent := `name: test
rules:
  - required_calls: [a]
  - forbidden_calls: [b]
`
	if err := os.WriteFile(specFile, []byte(specContent), 0644); err != nil {
		t.Fatalf("failed to write spec: %v", err)
	}

	passTrace := filepath.Join(tmpDir, "pass.jsonl")
	passContent := `{"step":1,"type":"tool_call","name":"a"}` + "\n"
	if err := os.WriteFile(passTrace, []byte(passContent), 0644); err != nil {
		t.Fatalf("failed to write pass trace: %v", err)
	}

	failTrace := filepath.Join(tmpDir, "fail.jsonl")
	failContent := `{"step":1,"type":"tool_call","name":"b"}` + "\n"
	if err := os.WriteFile(failTrace, []byte(failContent), 0644); err != nil {
		t.Fatalf("failed to write fail trace: %v", err)
	}

	t.Run("single passing trace text format", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"check", "--spec", specFile, "--trace", passTrace}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected code 0, got %d (stderr: %s)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "0 of 2 rules failed") {
			t.Errorf("unexpected output: %s", stdout.String())
		}
	})

	t.Run("single failing trace text format", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"check", "--spec", specFile, "--trace", failTrace}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("expected code 1, got %d (stderr: %s)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "2 of 2 rules failed") {
			t.Errorf("unexpected output: %s", stdout.String())
		}
	})

	t.Run("single failing trace json format", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"check", "--spec", specFile, "--trace", failTrace, "--format", "json"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("expected code 1, got %d (stderr: %s)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), `"pass": false`) {
			t.Errorf("unexpected json output: %s", stdout.String())
		}
	})

	t.Run("single failing trace github format", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"check", "--spec", specFile, "--trace", failTrace, "--format", "github"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("expected code 1, got %d (stderr: %s)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "::error file=") {
			t.Errorf("unexpected github output: %s", stdout.String())
		}
	})

	t.Run("multi traces directory", func(t *testing.T) {
		tracesDir := filepath.Join(tmpDir, "traces")
		if err := os.Mkdir(tracesDir, 0755); err != nil {
			t.Fatalf("failed to create traces dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tracesDir, "01_pass.jsonl"), []byte(passContent), 0644); err != nil {
			t.Fatalf("failed to copy pass trace: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tracesDir, "02_fail.jsonl"), []byte(failContent), 0644); err != nil {
			t.Fatalf("failed to copy fail trace: %v", err)
		}

		var stdout, stderr bytes.Buffer
		code := run([]string{"check", "--spec", specFile, "--traces", tracesDir}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("expected code 1, got %d (stderr: %s)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "Summary: 1 of 2 traces failed") {
			t.Errorf("unexpected output: %s", stdout.String())
		}
	})

	t.Run("multi traces directory all pass", func(t *testing.T) {
		tracesDir := filepath.Join(tmpDir, "traces_pass")
		if err := os.Mkdir(tracesDir, 0755); err != nil {
			t.Fatalf("failed to create traces dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(tracesDir, "01_pass.jsonl"), []byte(passContent), 0644); err != nil {
			t.Fatalf("failed to copy pass trace: %v", err)
		}

		var stdout, stderr bytes.Buffer
		code := run([]string{"check", "--spec", specFile, "--traces", tracesDir}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("expected code 0, got %d (stderr: %s)", code, stderr.String())
		}
	})

	t.Run("multi traces json format", func(t *testing.T) {
		tracesDir := filepath.Join(tmpDir, "traces")
		var stdout, stderr bytes.Buffer
		code := run([]string{"check", "--spec", specFile, "--traces", tracesDir, "--format", "json"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("expected code 1, got %d", code)
		}
		if !strings.Contains(stdout.String(), `"traces": [`) {
			t.Errorf("unexpected multi-trace json: %s", stdout.String())
		}
	})

	t.Run("multi traces github format", func(t *testing.T) {
		tracesDir := filepath.Join(tmpDir, "traces")
		var stdout, stderr bytes.Buffer
		code := run([]string{"check", "--spec", specFile, "--traces", tracesDir, "--format", "github"}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("expected code 1, got %d", code)
		}
		if !strings.Contains(stdout.String(), "::error file=") {
			t.Errorf("unexpected multi-trace github: %s", stdout.String())
		}
	})

	t.Run("multi traces empty directory", func(t *testing.T) {
		emptyDir := filepath.Join(tmpDir, "empty_traces")
		if err := os.Mkdir(emptyDir, 0755); err != nil {
			t.Fatalf("failed to create empty dir: %v", err)
		}
		var stdout, stderr bytes.Buffer
		code := run([]string{"check", "--spec", specFile, "--traces", emptyDir}, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("expected code 2 for empty traces dir, got %d", code)
		}
	})
}

func TestMiscCommands(t *testing.T) {
	t.Run("no args", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{}, &stdout, &stderr)
		if code != 2 {
			t.Errorf("expected code 2, got %d", code)
		}
	})

	t.Run("unknown command", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"unknown_cmd"}, &stdout, &stderr)
		if code != 2 {
			t.Errorf("expected code 2, got %d", code)
		}
	})

	t.Run("help flag", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"--help"}, &stdout, &stderr)
		if code != 0 {
			t.Errorf("expected code 0, got %d", code)
		}
	})
}
