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
