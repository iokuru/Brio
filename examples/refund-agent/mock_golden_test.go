package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/iokuru/brio/internal/trace"
)

func TestMockScriptsGolden(t *testing.T) {
	tmpDir := t.TempDir()

	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve root dir: %v", err)
	}
	brioBin := filepath.Join(tmpDir, "brio_test_bin.exe")
	buildCmd := exec.Command("go", "build", "-o", brioBin, "./cmd/brio")
	buildCmd.Dir = rootDir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build brio binary: %v\nOutput: %s", err, string(out))
	}

	tests := []struct {
		name         string
		script       string
		specFile     string
		wantPass     bool
		wantFailRule string
	}{
		{
			name:     "clean run passes all rules",
			script:   "clean",
			specFile: "brio.yaml",
			wantPass: true,
		},
		{
			name:         "refund before confirm fails order rule",
			script:       "refund_before_confirm",
			specFile:     "brio.yaml",
			wantPass:     false,
			wantFailRule: "order",
		},
		{
			name:         "delete user fails forbidden rule",
			script:       "delete_user",
			specFile:     "brio.yaml",
			wantPass:     false,
			wantFailRule: "forbidden_calls",
		},
		{
			name:         "repeat lookup loop fails limits rule",
			script:       "repeat_lookup",
			specFile:     "brio.yaml",
			wantPass:     false,
			wantFailRule: "limits",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := sampleTestStore()
			var traceBuf bytes.Buffer
			tw := NewTraceWriter(&traceBuf)

			mock := NewScriptedMockModel(tc.script)
			cfg := AgentConfig{
				SystemPrompt: "test",
				Store:        store,
				Model:        mock,
				TraceWriter:  tw,
			}

			_ = RunAgent(context.Background(), cfg, "Process refund for u_1 for 49.99")
			traceBytes := traceBuf.Bytes()

			parsedTrace, err := trace.Parse(bytes.NewReader(traceBytes), tc.script+".jsonl")
			if err != nil {
				t.Fatalf("failed to parse trace with internal/trace: %v\nTrace:\n%s", err, string(traceBytes))
			}
			if len(parsedTrace.Steps) == 0 {
				t.Fatalf("parsed trace has 0 steps")
			}

			tracePath := filepath.Join(tmpDir, tc.script+".jsonl")
			if err := os.WriteFile(tracePath, traceBytes, 0644); err != nil {
				t.Fatalf("failed to write trace file: %v", err)
			}

			cmd := exec.Command(brioBin, "check", "--spec", tc.specFile, "--trace", tracePath, "--format", "json")
			var outBuf, errBuf bytes.Buffer
			cmd.Stdout = &outBuf
			cmd.Stderr = &errBuf
			runErr := cmd.Run()

			if tc.wantPass && runErr != nil {
				t.Fatalf("expected brio exit code 0 for passing run, got error: %v (stderr: %s)", runErr, errBuf.String())
			}
			if !tc.wantPass && runErr == nil {
				t.Fatalf("expected brio non-zero exit code for failing run, got success")
			}

			var report brioJSONReport
			if err := json.Unmarshal(outBuf.Bytes(), &report); err != nil {
				t.Fatalf("failed to parse brio JSON report: %v (output: %s)", err, outBuf.String())
			}

			if report.Pass != tc.wantPass {
				t.Errorf("expected pass=%v, got pass=%v", tc.wantPass, report.Pass)
			}

			if tc.wantFailRule != "" {
				foundFailedRule := false
				for _, r := range report.Results {
					if r.Name == tc.wantFailRule && !r.Pass {
						foundFailedRule = true
						break
					}
				}
				if !foundFailedRule {
					t.Errorf("expected rule %q to fail in report: %+v", tc.wantFailRule, report.Results)
				}
			}
		})
	}
}

func TestClarifySpecGolden(t *testing.T) {
	tmpDir := t.TempDir()

	rootDir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve root dir: %v", err)
	}
	brioBin := filepath.Join(tmpDir, "brio_test_bin.exe")
	buildCmd := exec.Command("go", "build", "-o", brioBin, "./cmd/brio")
	buildCmd.Dir = rootDir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build brio binary: %v\nOutput: %s", err, string(out))
	}

	passTrace := filepath.Join(tmpDir, "pass_clarify.jsonl")
	passData := `{"step":1,"type":"model","tokens_in":100,"tokens_out":20,"latency_ms":10}
{"step":2,"type":"tool_call","name":"lookup_user","args":{"id":"u_104"},"latency_ms":5}
{"step":3,"type":"tool_result","name":"lookup_user","ok":true}
{"step":4,"type":"model","tokens_in":150,"tokens_out":20,"latency_ms":10}
{"step":5,"type":"tool_call","name":"confirm_user","args":{"id":"u_104"},"latency_ms":5}
{"step":6,"type":"tool_result","name":"confirm_user","ok":true}
{"step":7,"type":"model","tokens_in":200,"tokens_out":20,"latency_ms":10}
`
	if err := os.WriteFile(passTrace, []byte(passData), 0644); err != nil {
		t.Fatalf("write pass trace: %v", err)
	}

	cmd := exec.Command(brioBin, "check", "--spec", "brio.clarify.yaml", "--trace", passTrace, "--format", "json")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected brio to pass clarify trace: %v (stderr: %s)", err, errBuf.String())
	}

	var report brioJSONReport
	if err := json.Unmarshal(outBuf.Bytes(), &report); err != nil {
		t.Fatalf("unmarshal brio json: %v", err)
	}
	if !report.Pass {
		t.Errorf("expected clarify pass trace to pass, got: %+v", report)
	}

	failTrace := filepath.Join(tmpDir, "fail_clarify.jsonl")
	failData := `{"step":1,"type":"model","tokens_in":100,"tokens_out":20,"latency_ms":10}
{"step":2,"type":"tool_call","name":"lookup_user","args":{"id":"u_104"},"latency_ms":5}
{"step":3,"type":"tool_result","name":"lookup_user","ok":true}
{"step":4,"type":"model","tokens_in":150,"tokens_out":20,"latency_ms":10}
{"step":5,"type":"tool_call","name":"delete_user","args":{"id":"u_104"},"latency_ms":5}
{"step":6,"type":"tool_result","name":"delete_user","ok":true}
{"step":7,"type":"model","tokens_in":200,"tokens_out":20,"latency_ms":10}
`
	if err := os.WriteFile(failTrace, []byte(failData), 0644); err != nil {
		t.Fatalf("write fail trace: %v", err)
	}

	outBuf.Reset()
	errBuf.Reset()
	cmdFail := exec.Command(brioBin, "check", "--spec", "brio.clarify.yaml", "--trace", failTrace, "--format", "json")
	cmdFail.Stdout = &outBuf
	cmdFail.Stderr = &errBuf
	if err := cmdFail.Run(); err == nil {
		t.Fatalf("expected brio to fail trace with delete_user")
	}

	var failReport brioJSONReport
	if err := json.Unmarshal(outBuf.Bytes(), &failReport); err != nil {
		t.Fatalf("unmarshal brio fail json: %v", err)
	}
	if failReport.Pass {
		t.Errorf("expected failReport.Pass to be false")
	}
	foundForbiddenFail := false
	for _, r := range failReport.Results {
		if r.Name == "forbidden_calls" && !r.Pass {
			foundForbiddenFail = true
		}
	}
	if !foundForbiddenFail {
		t.Errorf("expected forbidden_calls to fail in report: %+v", failReport.Results)
	}
}
