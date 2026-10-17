package trace

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

var (
	ErrEmptyTrace = errors.New("trace is empty: no steps found")
)

type Step struct {
	Step      int            `json:"step"`
	Type      string         `json:"type"`
	Name      string         `json:"name,omitempty"`
	Args      map[string]any `json:"args,omitempty"`
	CostUSD   float64        `json:"cost_usd,omitempty"`
	LatencyMS int64          `json:"latency_ms,omitempty"`
}

type Trace struct {
	Path  string
	Steps []Step
}

func (t Trace) ToolCalls() []Step {
	var calls []Step
	for _, s := range t.Steps {
		if s.Type == "tool_call" {
			calls = append(calls, s)
		}
	}
	return calls
}

func Parse(r io.Reader, path string) (Trace, error) {
	reader := bufio.NewReader(r)
	var steps []Step
	lineNum := 0
	prevStep := 0

	for {
		line, err := reader.ReadBytes('\n')
		if len(line) == 0 && err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return Trace{}, fmt.Errorf("reading trace: %w", err)
		}

		lineNum++
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			if errors.Is(err, io.EOF) {
				break
			}
			continue
		}

		var raw map[string]any
		if jsonErr := json.Unmarshal(trimmed, &raw); jsonErr != nil {
			return Trace{}, fmt.Errorf("line %d: malformed JSON: %w", lineNum, jsonErr)
		}

		var s Step
		if jsonErr := json.Unmarshal(trimmed, &s); jsonErr != nil {
			return Trace{}, fmt.Errorf("line %d: malformed step: %w", lineNum, jsonErr)
		}

		if _, hasStep := raw["step"]; !hasStep {
			return Trace{}, fmt.Errorf("line %d: missing step number", lineNum)
		}

		if len(steps) == 0 {
			if s.Step != 1 {
				return Trace{}, fmt.Errorf("line %d: steps must start at 1, got %d", lineNum, s.Step)
			}
		} else {
			if s.Step <= prevStep {
				return Trace{}, fmt.Errorf("line %d: steps must be strictly increasing, got %d after %d", lineNum, s.Step, prevStep)
			}
		}

		if strings.TrimSpace(s.Type) == "" {
			return Trace{}, fmt.Errorf("line %d: step type cannot be empty", lineNum)
		}

		if s.Type == "tool_call" && strings.TrimSpace(s.Name) == "" {
			return Trace{}, fmt.Errorf("line %d: tool_call must specify name", lineNum)
		}

		prevStep = s.Step
		steps = append(steps, s)

		if errors.Is(err, io.EOF) {
			break
		}
	}

	if len(steps) == 0 {
		return Trace{}, ErrEmptyTrace
	}

	return Trace{
		Path:  path,
		Steps: steps,
	}, nil
}

func ParseFile(path string) (Trace, error) {
	f, err := os.Open(path)
	if err != nil {
		return Trace{}, fmt.Errorf("open trace file: %w", err)
	}
	defer f.Close()

	return Parse(f, path)
}
