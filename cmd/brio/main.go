package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/iokuru/brio/internal/report"
	"github.com/iokuru/brio/internal/rules"
	"github.com/iokuru/brio/internal/spec"
	"github.com/iokuru/brio/internal/trace"
)

var version = "dev"

const starterSpec = `# Brio specification for grading agent trajectories
name: starter-flow

rules:
  # Ensure required tools are called at least once
  - required_calls:
      - lookup_user
      - issue_refund

  # Prevent forbidden tools from being called
  - forbidden_calls:
      - delete_user

  # Enforce ordering between tool calls
  - order:
      - before: confirm_user
        after: issue_refund
        immediately: false

  # Match a subsequence of tool calls allowing interleaved calls
  - subsequence:
      - lookup_user
      - confirm_user
      - issue_refund

  # Validate tool arguments using regex, equals, or present
  - arg_match:
      call: issue_refund
      arg: amount
      regex: '^\d{1,4}(\.\d{2})?$'

  # Set execution limits to guard against runaway tool loops
  - limits:
      max_steps: 20
      max_cost_usd: 0.25
      max_latency_ms: 15000
      max_repeat_calls: 3
`

func main() {
	code := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(code)
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "version":
		return runVersion(cmdArgs, stdout, stderr)
	case "init":
		return runInit(cmdArgs, stdout, stderr)
	case "check":
		return runCheck(cmdArgs, stdout, stderr)
	case "-h", "--help", "help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", cmd)
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `Usage:
  brio check --spec spec.yaml --trace run.jsonl [--format text|json|github]
  brio check --spec spec.yaml --traces ./traces/ [--format text|json|github]
  brio init [--out brio.yaml] [--force]
  brio version
`)
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fmt.Fprintf(stdout, "brio version %s\n", version)
	return 0
}

func runInit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)

	outPath := fs.String("out", "brio.yaml", "output path for starter spec")
	force := fs.Bool("force", false, "overwrite existing file")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if _, err := os.Stat(*outPath); err == nil && !*force {
		fmt.Fprintf(stderr, "file already exists: %s (use --force to overwrite)\n", *outPath)
		return 2
	}

	if err := os.WriteFile(*outPath, []byte(starterSpec), 0644); err != nil {
		fmt.Fprintf(stderr, "failed to write spec file: %v\n", err)
		return 2
	}

	fmt.Fprintf(stdout, "wrote starter spec to %s\n", *outPath)
	return 0
}

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)

	specPath := fs.String("spec", "", "path to YAML spec file")
	tracePath := fs.String("trace", "", "path to JSONL trace file")
	tracesDir := fs.String("traces", "", "path to directory containing JSONL trace files")
	format := fs.String("format", "text", "output format: text, json, or github")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *specPath == "" {
		fmt.Fprintln(stderr, "error: --spec is required")
		printUsage(stderr)
		return 2
	}

	if (*tracePath == "" && *tracesDir == "") || (*tracePath != "" && *tracesDir != "") {
		fmt.Fprintln(stderr, "error: specify exactly one of --trace or --traces")
		printUsage(stderr)
		return 2
	}

	fmtLower := strings.ToLower(*format)
	if fmtLower != "text" && fmtLower != "json" && fmtLower != "github" {
		fmt.Fprintf(stderr, "error: invalid format %q, must be text, json, or github\n", *format)
		return 2
	}

	sp, err := spec.LoadFile(*specPath)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	ruleList := sp.BuildRules()

	if *tracePath != "" {
		tr, err := trace.ParseFile(*tracePath)
		if err != nil {
			fmt.Fprintf(stderr, "%v\n", err)
			return 2
		}

		var results []rules.Result
		for _, r := range ruleList {
			results = append(results, r.Check(tr))
		}

		rep := report.NewTraceReport(*tracePath, ruleList, results)
		out, err := renderReport(rep, fmtLower)
		if err != nil {
			fmt.Fprintf(stderr, "error rendering report: %v\n", err)
			return 2
		}
		fmt.Fprint(stdout, out)

		if rep.Pass {
			return 0
		}
		return 1
	}

	// Multi-trace mode
	files, err := os.ReadDir(*tracesDir)
	if err != nil {
		fmt.Fprintf(stderr, "error reading traces directory: %v\n", err)
		return 2
	}

	var traceFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(strings.ToLower(f.Name()), ".jsonl") {
			traceFiles = append(traceFiles, filepath.Join(*tracesDir, f.Name()))
		}
	}
	sort.Strings(traceFiles)

	if len(traceFiles) == 0 {
		fmt.Fprintf(stderr, "error: no .jsonl files found in %s\n", *tracesDir)
		return 2
	}

	var reports []report.TraceReport
	hasError := false
	hasFail := false

	for _, tf := range traceFiles {
		tr, err := trace.ParseFile(tf)
		if err != nil {
			hasError = true
			reports = append(reports, report.TraceReport{
				Path:   tf,
				Status: "error",
				Error:  err.Error(),
			})
			continue
		}

		var results []rules.Result
		for _, r := range ruleList {
			results = append(results, r.Check(tr))
		}

		rep := report.NewTraceReport(tf, ruleList, results)
		if !rep.Pass {
			hasFail = true
		}
		reports = append(reports, rep)
	}

	out, err := renderMultiReport(reports, fmtLower)
	if err != nil {
		fmt.Fprintf(stderr, "error rendering report: %v\n", err)
		return 2
	}
	fmt.Fprint(stdout, out)

	if hasError {
		return 2
	}
	if hasFail {
		return 1
	}
	return 0
}

func renderReport(tr report.TraceReport, format string) (string, error) {
	switch format {
	case "text":
		return report.FormatText(tr), nil
	case "json":
		return report.FormatJSON(tr)
	case "github":
		return report.FormatGitHub(tr), nil
	default:
		return "", errors.New("unsupported format")
	}
}

func renderMultiReport(reports []report.TraceReport, format string) (string, error) {
	switch format {
	case "text":
		return report.FormatMultiText(reports), nil
	case "json":
		return report.FormatMultiJSON(reports)
	case "github":
		return report.FormatMultiGitHub(reports), nil
	default:
		return "", errors.New("unsupported format")
	}
}
