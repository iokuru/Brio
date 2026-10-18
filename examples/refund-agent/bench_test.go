package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestBenchmarkMock(t *testing.T) {
	tmpDir := t.TempDir()
	outDir := filepath.Join(tmpDir, "results")

	var stdout, stderr bytes.Buffer
	code := run([]string{"bench", "--mock", "--runs", "1", "--out", outDir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0, got %d (stderr: %s)", code, stderr.String())
	}

	out := stdout.String()
	if !strings.Contains(out, "BENCHMARK RESULTS") {
		t.Errorf("expected header 'BENCHMARK RESULTS' in stdout:\n%s", out)
	}
	if !strings.Contains(out, "CAREFUL") || !strings.Contains(out, "NAIVE") {
		t.Errorf("expected CAREFUL and NAIVE in stdout:\n%s", out)
	}
	if !strings.Contains(out, "Overall Pass Rate") {
		t.Errorf("expected 'Overall Pass Rate' in stdout:\n%s", out)
	}
	if !strings.Contains(out, "Runaway Loops") {
		t.Errorf("expected 'Runaway Loops' in stdout:\n%s", out)
	}
}

func TestRunCommandMock(t *testing.T) {
	tmpDir := t.TempDir()
	tracePath := filepath.Join(tmpDir, "trace.jsonl")

	var stdout, stderr bytes.Buffer
	code := run([]string{"run", "--prompt", "careful", "--scenario", "1", "--out", tracePath, "--mock"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected code 0, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Trace written to") {
		t.Errorf("expected confirmation on stderr: %s", stderr.String())
	}
}

func TestRefundAgentCLIUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected code 2 for no args, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("expected usage on stderr: %s", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"unknown_cmd"}, &stdout, &stderr)
	if code != 2 {
		t.Errorf("expected code 2 for unknown cmd, got %d", code)
	}
}
