package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type brioJSONResult struct {
	Name    string `json:"name"`
	Pass    bool   `json:"pass"`
	Step    int    `json:"step"`
	Message string `json:"message"`
}

type brioJSONReport struct {
	Path    string           `json:"path"`
	Pass    bool             `json:"pass"`
	Failed  int              `json:"failed"`
	Total   int              `json:"total"`
	Results []brioJSONResult `json:"results"`
}

type BenchConfig struct {
	Runs         int
	OutDir       string
	UseMock      bool
	ScenariosDir string
	BrioBin      string
}

type VariantStats struct {
	Variant      string
	TotalRuns    int
	PassedRuns   int
	TotalSteps   int
	TotalCostUSD float64
	RunawayLoops int
	RuleTotal    map[string]int
	RulePassed   map[string]int
}

type TraceMetrics struct {
	Steps   int
	CostUSD float64
}

func readTraceMetrics(path string) (TraceMetrics, error) {
	f, err := os.Open(path)
	if err != nil {
		return TraceMetrics{}, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	steps := 0
	cost := 0.0

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(line, &raw); err != nil {
			continue
		}
		steps++
		if c, ok := raw["cost_usd"].(float64); ok {
			cost += c
		}
	}
	return TraceMetrics{Steps: steps, CostUSD: cost}, scanner.Err()
}

func findRootDir() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		modFile := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(modFile); err == nil {
			if strings.Contains(string(data), "module github.com/iokuru/brio\n") || strings.Contains(string(data), "module github.com/iokuru/brio\r\n") {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not find root module directory containing go.mod")
}

func ensureBrioBinary(customBin string) (string, error) {
	if customBin != "" {
		if _, err := os.Stat(customBin); err == nil {
			return customBin, nil
		}
		return "", fmt.Errorf("specified brio binary not found: %s", customBin)
	}

	rootDir, err := findRootDir()
	if err != nil {
		return "", fmt.Errorf("resolve root dir: %w", err)
	}

	candidates := []string{
		filepath.Join(rootDir, "brio.exe"),
		filepath.Join(rootDir, "bin", "brio.exe"),
		filepath.Join(rootDir, "brio"),
		filepath.Join(rootDir, "bin", "brio"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}

	tmpBin := filepath.Join(os.TempDir(), "brio_bench.exe")
	buildCmd := exec.Command("go", "build", "-o", tmpBin, "./cmd/brio")
	buildCmd.Dir = rootDir
	if out, err := buildCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to build brio binary: %w (output: %s)", err, string(out))
	}
	return tmpBin, nil
}

func RunBenchmark(ctx context.Context, cfg BenchConfig, stdout, stderr io.Writer) error {
	brioBin, err := ensureBrioBinary(cfg.BrioBin)
	if err != nil {
		return err
	}

	rootDir, _ := findRootDir()

	scenariosDir := cfg.ScenariosDir
	if scenariosDir == "" {
		scenariosDir = "scenarios"
	}
	if _, err := os.Stat(scenariosDir); err != nil && rootDir != "" {
		candidate := filepath.Join(rootDir, "examples", "refund-agent", "scenarios")
		if _, err := os.Stat(candidate); err == nil {
			scenariosDir = candidate
		}
	}

	scenarios, err := LoadAllScenarios(scenariosDir)
	if err != nil {
		return fmt.Errorf("load scenarios: %w", err)
	}
	if len(scenarios) == 0 {
		return fmt.Errorf("no scenarios found in %s", scenariosDir)
	}

	variants := []string{"careful", "naive"}
	stats := make(map[string]*VariantStats)
	for _, v := range variants {
		stats[v] = &VariantStats{
			Variant:    v,
			RuleTotal:  make(map[string]int),
			RulePassed: make(map[string]int),
		}
	}

	var realClient *AnthropicClient
	if !cfg.UseMock {
		realClient, err = NewAnthropicClient()
		if err != nil {
			return fmt.Errorf("anthropic client: %w (use --mock for offline run)", err)
		}
	}

	outDir := cfg.OutDir
	if outDir == "" {
		outDir = "results"
	}

	for _, variant := range variants {
		variantDir := filepath.Join(outDir, variant)
		if err := os.MkdirAll(variantDir, 0755); err != nil {
			return fmt.Errorf("create variant dir %s: %w", variantDir, err)
		}

		prompt, err := GetPrompt(variant)
		if err != nil {
			return err
		}

		vStats := stats[variant]

		for _, sc := range scenarios {
			for run := 1; run <= cfg.Runs; run++ {
				traceFileName := fmt.Sprintf("scenario-%s-%d.jsonl", sc.ID, run)
				tracePath := filepath.Join(variantDir, traceFileName)

				traceFile, err := os.Create(tracePath)
				if err != nil {
					return fmt.Errorf("create trace file %s: %w", tracePath, err)
				}

				tw := NewTraceWriter(traceFile)
				store := NewStore(sc.Users)

				var model ModelClient
				if cfg.UseMock {
					model = NewScenarioMockModel(variant, sc.ID)
				} else {
					model = realClient
				}

				agentCfg := AgentConfig{
					SystemPrompt: prompt,
					Store:        store,
					Model:        model,
					TraceWriter:  tw,
				}

				_ = RunAgent(ctx, agentCfg, sc.UserMessage)
				traceFile.Close()

				metrics, err := readTraceMetrics(tracePath)
				if err == nil {
					vStats.TotalSteps += metrics.Steps
					vStats.TotalCostUSD += metrics.CostUSD
				}
				vStats.TotalRuns++

				specPath := sc.Spec
				if !filepath.IsAbs(specPath) {
					if _, err := os.Stat(specPath); err != nil && rootDir != "" {
						candidate := filepath.Join(rootDir, "examples", "refund-agent", sc.Spec)
						if _, err := os.Stat(candidate); err == nil {
							specPath = candidate
						}
					}
				}

				cmd := exec.Command(brioBin, "check", "--spec", specPath, "--trace", tracePath, "--format", "json")
				var outBuf, errBuf bytes.Buffer
				cmd.Stdout = &outBuf
				cmd.Stderr = &errBuf
				_ = cmd.Run()

				var rep brioJSONReport
				if err := json.Unmarshal(outBuf.Bytes(), &rep); err == nil {
					if rep.Pass {
						vStats.PassedRuns++
					}
					for _, r := range rep.Results {
						vStats.RuleTotal[r.Name]++
						if r.Pass {
							vStats.RulePassed[r.Name]++
						} else if r.Name == "limits" && strings.Contains(r.Message, "max_repeat_calls") {
							vStats.RunawayLoops++
						}
					}
				}
			}
		}
	}

	printSummaryTable(stats, variants, stdout)
	return nil
}

func printSummaryTable(stats map[string]*VariantStats, variants []string, w io.Writer) {
	fmt.Fprintln(w, "\n=======================================================")
	fmt.Fprintln(w, "                    BENCHMARK RESULTS                  ")
	fmt.Fprintln(w, "=======================================================")

	for _, v := range variants {
		s := stats[v]
		overallPassRate := 0.0
		meanSteps := 0.0
		meanCost := 0.0
		if s.TotalRuns > 0 {
			overallPassRate = float64(s.PassedRuns) / float64(s.TotalRuns) * 100.0
			meanSteps = float64(s.TotalSteps) / float64(s.TotalRuns)
			meanCost = s.TotalCostUSD / float64(s.TotalRuns)
		}

		fmt.Fprintf(w, "\nVariant: %s (%d runs)\n", strings.ToUpper(v), s.TotalRuns)
		fmt.Fprintf(w, "  Overall Pass Rate:  %6.1f%% (%d/%d)\n", overallPassRate, s.PassedRuns, s.TotalRuns)
		fmt.Fprintf(w, "  Mean Steps:         %6.1f\n", meanSteps)
		fmt.Fprintf(w, "  Mean Cost (USD):   $%6.4f\n", meanCost)
		fmt.Fprintf(w, "  Runaway Loops:      %d\n", s.RunawayLoops)
		fmt.Fprintln(w, "  Rule Pass Rates:")

		var ruleNames []string
		for r := range s.RuleTotal {
			ruleNames = append(ruleNames, r)
		}
		sort.Strings(ruleNames)

		for _, r := range ruleNames {
			tot := s.RuleTotal[r]
			pass := s.RulePassed[r]
			rate := 0.0
			if tot > 0 {
				rate = float64(pass) / float64(tot) * 100.0
			}
			fmt.Fprintf(w, "    - %-16s %5.1f%% (%d/%d)\n", r+":", rate, pass, tot)
		}
	}

	fmt.Fprintln(w, "\n-------------------------------------------------------")
	fmt.Fprintln(w, "| Metric                      | Careful    | Naive     |")
	fmt.Fprintln(w, "|-----------------------------|------------|-----------|")

	careful := stats["careful"]
	naive := stats["naive"]

	cPassRate := 0.0
	nPassRate := 0.0
	cSteps := 0.0
	nSteps := 0.0
	cCost := 0.0
	nCost := 0.0

	if careful.TotalRuns > 0 {
		cPassRate = float64(careful.PassedRuns) / float64(careful.TotalRuns) * 100.0
		cSteps = float64(careful.TotalSteps) / float64(careful.TotalRuns)
		cCost = careful.TotalCostUSD / float64(careful.TotalRuns)
	}
	if naive.TotalRuns > 0 {
		nPassRate = float64(naive.PassedRuns) / float64(naive.TotalRuns) * 100.0
		nSteps = float64(naive.TotalSteps) / float64(naive.TotalRuns)
		nCost = naive.TotalCostUSD / float64(naive.TotalRuns)
	}

	fmt.Fprintf(w, "| Total Runs                  | %-10d | %-9d |\n", careful.TotalRuns, naive.TotalRuns)
	fmt.Fprintf(w, "| Overall Pass Rate           | %9.1f%% | %8.1f%% |\n", cPassRate, nPassRate)
	fmt.Fprintf(w, "| Mean Steps                  | %10.1f | %9.1f |\n", cSteps, nSteps)
	fmt.Fprintf(w, "| Mean Cost (USD)             | $%9.4f | $%8.4f |\n", cCost, nCost)
	fmt.Fprintf(w, "| Runaway Loops               | %-10d | %-9d |\n", careful.RunawayLoops, naive.RunawayLoops)

	ruleSet := make(map[string]bool)
	for r := range careful.RuleTotal {
		ruleSet[r] = true
	}
	for r := range naive.RuleTotal {
		ruleSet[r] = true
	}
	var allRules []string
	for r := range ruleSet {
		allRules = append(allRules, r)
	}
	sort.Strings(allRules)

	for _, r := range allRules {
		cRate := 0.0
		if careful.RuleTotal[r] > 0 {
			cRate = float64(careful.RulePassed[r]) / float64(careful.RuleTotal[r]) * 100.0
		}
		nRate := 0.0
		if naive.RuleTotal[r] > 0 {
			nRate = float64(naive.RulePassed[r]) / float64(naive.RuleTotal[r]) * 100.0
		}
		label := fmt.Sprintf("%s pass", r)
		fmt.Fprintf(w, "| %-27s | %9.1f%% | %8.1f%% |\n", label, cRate, nRate)
	}
	fmt.Fprintln(w, "-------------------------------------------------------")
}
