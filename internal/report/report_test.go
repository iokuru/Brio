package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iokuru/brio/internal/rules"
)

func samplePassingReport() TraceReport {
	return TraceReport{
		Path:   "run_pass.jsonl",
		Pass:   true,
		Failed: 0,
		Total:  6,
		Results: []RuleResult{
			{Name: "required_calls", Pass: true},
			{Name: "forbidden_calls", Pass: true},
			{Name: "order", Pass: true},
			{Name: "subsequence", Pass: true},
			{Name: "arg_match", Pass: true},
			{Name: "limits", Pass: true},
		},
	}
}

func sampleFailingReport() TraceReport {
	return TraceReport{
		Path:   "run_fail.jsonl",
		Pass:   false,
		Failed: 2,
		Total:  6,
		Results: []RuleResult{
			{Name: "required_calls", Pass: true},
			{Name: "forbidden_calls", Pass: true},
			{
				Name:    "order",
				Pass:    false,
				Step:    4,
				Message: "issue_refund called before confirm_user",
			},
			{Name: "subsequence", Pass: true},
			{Name: "arg_match", Pass: true},
			{
				Name:    "limits",
				Pass:    false,
				Step:    11,
				Message: "lookup_user called 4 times (max_repeat_calls: 3)",
			},
		},
	}
}

func normalizeLineEndings(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func TestTextReporterGolden(t *testing.T) {
	goldenDir := filepath.Join("..", "..", "testdata", "golden")

	t.Run("passing report", func(t *testing.T) {
		got := normalizeLineEndings(FormatText(samplePassingReport()))
		goldenPath := filepath.Join(goldenDir, "report_pass.txt")
		expectedBytes, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden file %s: %v", goldenPath, err)
		}
		expected := normalizeLineEndings(string(expectedBytes))
		if got != expected {
			t.Errorf("mismatch with golden file %s:\nGOT:\n%s\nWANT:\n%s", goldenPath, got, expected)
		}
	})

	t.Run("failing report", func(t *testing.T) {
		got := normalizeLineEndings(FormatText(sampleFailingReport()))
		goldenPath := filepath.Join(goldenDir, "report_fail.txt")
		expectedBytes, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden file %s: %v", goldenPath, err)
		}
		expected := normalizeLineEndings(string(expectedBytes))
		if got != expected {
			t.Errorf("mismatch with golden file %s:\nGOT:\n%s\nWANT:\n%s", goldenPath, got, expected)
		}
	})
}

func TestJSONReporterGolden(t *testing.T) {
	goldenDir := filepath.Join("..", "..", "testdata", "golden")

	t.Run("passing report json", func(t *testing.T) {
		gotJSON, err := FormatJSON(samplePassingReport())
		if err != nil {
			t.Fatalf("format json: %v", err)
		}
		got := normalizeLineEndings(gotJSON)
		goldenPath := filepath.Join(goldenDir, "report_pass.json")
		expectedBytes, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden file %s: %v", goldenPath, err)
		}
		expected := normalizeLineEndings(string(expectedBytes))
		if got != expected {
			t.Errorf("mismatch with golden file %s:\nGOT:\n%s\nWANT:\n%s", goldenPath, got, expected)
		}
	})

	t.Run("failing report json", func(t *testing.T) {
		gotJSON, err := FormatJSON(sampleFailingReport())
		if err != nil {
			t.Fatalf("format json: %v", err)
		}
		got := normalizeLineEndings(gotJSON)
		goldenPath := filepath.Join(goldenDir, "report_fail.json")
		expectedBytes, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden file %s: %v", goldenPath, err)
		}
		expected := normalizeLineEndings(string(expectedBytes))
		if got != expected {
			t.Errorf("mismatch with golden file %s:\nGOT:\n%s\nWANT:\n%s", goldenPath, got, expected)
		}
	})
}

func TestGitHubReporterGolden(t *testing.T) {
	goldenDir := filepath.Join("..", "..", "testdata", "golden")

	t.Run("passing report github", func(t *testing.T) {
		got := normalizeLineEndings(FormatGitHub(samplePassingReport()))
		goldenPath := filepath.Join(goldenDir, "report_pass.github")
		expectedBytes, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden file %s: %v", goldenPath, err)
		}
		expected := normalizeLineEndings(string(expectedBytes))
		if got != expected {
			t.Errorf("mismatch with golden file %s:\nGOT:\n%s\nWANT:\n%s", goldenPath, got, expected)
		}
	})

	t.Run("failing report github", func(t *testing.T) {
		got := normalizeLineEndings(FormatGitHub(sampleFailingReport()))
		goldenPath := filepath.Join(goldenDir, "report_fail.github")
		expectedBytes, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden file %s: %v", goldenPath, err)
		}
		expected := normalizeLineEndings(string(expectedBytes))
		if got != expected {
			t.Errorf("mismatch with golden file %s:\nGOT:\n%s\nWANT:\n%s", goldenPath, got, expected)
		}
	})

	t.Run("result without step", func(t *testing.T) {
		rep := TraceReport{
			Path:   "run_no_step.jsonl",
			Pass:   false,
			Failed: 1,
			Total:  1,
			Results: []RuleResult{
				{
					Name:    "required_calls",
					Pass:    false,
					Step:    0,
					Message: "missing required call: lookup_user",
				},
			},
		}
		got := normalizeLineEndings(FormatGitHub(rep))
		goldenPath := filepath.Join(goldenDir, "report_no_step.github")
		expectedBytes, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden file %s: %v", goldenPath, err)
		}
		expected := normalizeLineEndings(string(expectedBytes))
		if got != expected {
			t.Errorf("mismatch with golden file %s:\nGOT:\n%s\nWANT:\n%s", goldenPath, got, expected)
		}
	})

	t.Run("message needing escaping", func(t *testing.T) {
		rep := TraceReport{
			Path:   "run_escaped.jsonl",
			Pass:   false,
			Failed: 1,
			Total:  1,
			Results: []RuleResult{
				{
					Name:    "arg_match",
					Pass:    false,
					Step:    3,
					Message: "rate 100% exceeded\r\nthreshold",
				},
			},
		}
		got := normalizeLineEndings(FormatGitHub(rep))
		goldenPath := filepath.Join(goldenDir, "report_escaped.github")
		expectedBytes, err := os.ReadFile(goldenPath)
		if err != nil {
			t.Fatalf("read golden file %s: %v", goldenPath, err)
		}
		expected := normalizeLineEndings(string(expectedBytes))
		if got != expected {
			t.Errorf("mismatch with golden file %s:\nGOT:\n%s\nWANT:\n%s", goldenPath, got, expected)
		}
	})
}

func TestFormatMultiGitHub(t *testing.T) {
	rep1 := samplePassingReport()
	rep2 := sampleFailingReport()
	reports := []TraceReport{rep1, rep2}

	out := FormatMultiGitHub(reports)
	if !strings.Contains(out, "::error file=run_fail.jsonl,line=4,title=Brio order::issue_refund called before confirm_user") {
		t.Errorf("missing expected error annotation in multi-github: %s", out)
	}
	if !strings.Contains(out, "Summary: 1 of 2 traces failed") {
		t.Errorf("missing summary in multi-github: %s", out)
	}
}

func TestFormatMulti(t *testing.T) {
	reports := []TraceReport{samplePassingReport(), sampleFailingReport()}

	text := FormatMultiText(reports)
	if !strings.Contains(text, "Summary: 1 of 2 traces failed") {
		t.Errorf("expected multi-text summary, got: %s", text)
	}

	jsonStr, err := FormatMultiJSON(reports)
	if err != nil {
		t.Fatalf("format multi json error: %v", err)
	}
	if !strings.Contains(jsonStr, `"total": 2`) || !strings.Contains(jsonStr, `"failed": 1`) {
		t.Errorf("unexpected multi-json output: %s", jsonStr)
	}
}

func TestNewTraceReport(t *testing.T) {
	rule1 := rules.NewRequiredCalls([]string{"a"})
	rule2 := rules.NewForbiddenCalls([]string{"b"})
	ruleList := []rules.Rule{rule1, rule2}
	results := []rules.Result{
		{Pass: true},
		{Pass: false, Step: 0, Message: "missing a"},
	}

	rep := NewTraceReport("test.jsonl", ruleList, results)
	if rep.Pass {
		t.Errorf("expected report to fail")
	}
	if rep.Failed != 1 || rep.Total != 2 {
		t.Errorf("expected 1 failed out of 2, got failed=%d total=%d", rep.Failed, rep.Total)
	}
	if len(rep.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(rep.Results))
	}
	if rep.Results[0].Name != "required_calls" || !rep.Results[0].Pass {
		t.Errorf("unexpected result 0: %+v", rep.Results[0])
	}
	if rep.Results[1].Name != "forbidden_calls" || rep.Results[1].Pass {
		t.Errorf("unexpected result 1: %+v", rep.Results[1])
	}

	text := FormatText(rep)
	if !strings.Contains(text, "FAIL forbidden_calls missing a\n") {
		t.Errorf("expected step-0 formatting without 'step 0:', got:\n%s", text)
	}
}
