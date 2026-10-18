package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

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
	case "run":
		return runAgentCmd(cmdArgs, stdout, stderr)
	case "bench":
		return runBenchCmd(cmdArgs, stdout, stderr)
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
  refund-agent run --prompt careful|naive --scenario 3 [--out trace.jsonl] [--provider anthropic|openai-compat] [--base-url url] [--model model] [--mock [script]]
  refund-agent bench [--runs 5] [--out results] [--provider anthropic|openai-compat] [--base-url url] [--model model] [--mock]
`)
}

type mockFlag struct {
	enabled bool
	script  string
}

func (m *mockFlag) String() string {
	return m.script
}

func (m *mockFlag) Set(s string) error {
	m.enabled = true
	if s == "true" {
		m.script = ""
	} else {
		m.script = s
	}
	return nil
}

func (m *mockFlag) IsBoolFlag() bool {
	return true
}

func runAgentCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)

	promptVariant := fs.String("prompt", "careful", "prompt variant: careful or naive")
	scenarioID := fs.String("scenario", "1", "scenario ID (1-8)")
	outPath := fs.String("out", "", "output trace JSONL path (default stdout)")
	provider := fs.String("provider", "", "model provider: anthropic or openai-compat (env: BRIO_PROVIDER)")
	baseURL := fs.String("base-url", "", "custom API base URL (env: BRIO_BASE_URL)")
	modelName := fs.String("model", "", "model name (env: BRIO_MODEL)")

	var mf mockFlag
	fs.Var(&mf, "mock", "use scripted mock (optional script name: clean, refund_before_confirm, delete_user, repeat_lookup)")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	prompt, err := GetPrompt(*promptVariant)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	scenariosDir := "scenarios"
	if _, err := os.Stat(scenariosDir); err != nil {
		if rootDir, err := findRootDir(); err == nil {
			candidate := filepath.Join(rootDir, "examples", "refund-agent", "scenarios")
			if _, err := os.Stat(candidate); err == nil {
				scenariosDir = candidate
			}
		}
	}
	scenarios, err := LoadAllScenarios(scenariosDir)
	if err != nil {
		fmt.Fprintf(stderr, "error loading scenarios from %s: %v\n", scenariosDir, err)
		return 2
	}

	sc, err := FindScenario(scenarios, *scenarioID)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 2
	}

	var traceOut io.Writer = stdout
	if *outPath != "" {
		if err := os.MkdirAll(filepath.Dir(*outPath), 0755); err != nil && filepath.Dir(*outPath) != "." {
			fmt.Fprintf(stderr, "error creating dir for %s: %v\n", *outPath, err)
			return 2
		}
		f, err := os.Create(*outPath)
		if err != nil {
			fmt.Fprintf(stderr, "error creating trace file %s: %v\n", *outPath, err)
			return 2
		}
		defer f.Close()
		traceOut = f
	}

	tw := NewTraceWriter(traceOut)
	store := NewStore(sc.Users)

	var model ModelClient
	if mf.enabled {
		if mf.script == "" {
			model = NewScenarioMockModel(*promptVariant, sc.ID)
		} else {
			model = NewScriptedMockModel(mf.script)
		}
	} else {
		pCfg := resolveProviderConfig(*provider, *baseURL, *modelName)
		client, err := CreateModelClient(pCfg)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v (use --mock for offline mode)\n", err)
			return 2
		}
		model = client
	}

	cfg := AgentConfig{
		SystemPrompt: prompt,
		Store:        store,
		Model:        model,
		TraceWriter:  tw,
	}

	if err := RunAgent(context.Background(), cfg, sc.UserMessage); err != nil && !errors.Is(err, ErrTurnCapReached) {
		fmt.Fprintf(stderr, "agent run error: %v\n", err)
		return 1
	}

	if *outPath != "" {
		fmt.Fprintf(stderr, "scenario %s (%s) finished. Trace written to %s\n", sc.ID, *promptVariant, *outPath)
	}

	return 0
}

func runBenchCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("bench", flag.ContinueOnError)
	fs.SetOutput(stderr)

	runs := fs.Int("runs", 5, "number of runs per scenario per variant")
	outDir := fs.String("out", "results", "output directory for traces")
	mock := fs.Bool("mock", false, "use scripted mock model for deterministic offline benchmark")
	brioBin := fs.String("brio-bin", "", "path to brio binary (built automatically if empty)")
	provider := fs.String("provider", "", "model provider: anthropic or openai-compat (env: BRIO_PROVIDER)")
	baseURL := fs.String("base-url", "", "custom API base URL (env: BRIO_BASE_URL)")
	modelName := fs.String("model", "", "model name (env: BRIO_MODEL)")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	benchCfg := BenchConfig{
		Runs:         *runs,
		OutDir:       *outDir,
		UseMock:      *mock,
		ScenariosDir: "scenarios",
		BrioBin:      *brioBin,
		Provider:     *provider,
		BaseURL:      *baseURL,
		Model:        *modelName,
	}

	if err := RunBenchmark(context.Background(), benchCfg, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "benchmark error: %v\n", err)
		return 1
	}

	return 0
}
