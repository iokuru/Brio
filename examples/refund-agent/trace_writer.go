package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"
)

const (
	PricePerMillionInputTokens  = 1.00
	PricePerMillionOutputTokens = 5.00
)

type flusher interface {
	Flush() error
}

type TraceWriter struct {
	w       io.Writer
	flusher flusher
	mu      sync.Mutex
	step    int
}

func NewTraceWriter(w io.Writer) *TraceWriter {
	tw := &TraceWriter{w: w}
	if f, ok := w.(flusher); ok {
		tw.flusher = f
	} else {
		bw := bufio.NewWriter(w)
		tw.w = bw
		tw.flusher = bw
	}
	return tw
}

func (tw *TraceWriter) writeLine(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal trace step: %w", err)
	}
	if _, err := fmt.Fprintf(tw.w, "%s\n", data); err != nil {
		return fmt.Errorf("write trace line: %w", err)
	}
	if tw.flusher != nil {
		if err := tw.flusher.Flush(); err != nil {
			return fmt.Errorf("flush trace line: %w", err)
		}
	}
	return nil
}

type modelTraceStep struct {
	Step      int     `json:"step"`
	Type      string  `json:"type"`
	TokensIn  int     `json:"tokens_in"`
	TokensOut int     `json:"tokens_out"`
	LatencyMS int64   `json:"latency_ms"`
	CostUSD   float64 `json:"cost_usd"`
}

func (tw *TraceWriter) RecordModelTurnWithCost(tokensIn, tokensOut int, latencyMS int64, cost float64) error {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	tw.step++

	return tw.writeLine(modelTraceStep{
		Step:      tw.step,
		Type:      "model",
		TokensIn:  tokensIn,
		TokensOut: tokensOut,
		LatencyMS: latencyMS,
		CostUSD:   cost,
	})
}

func (tw *TraceWriter) RecordModelTurn(tokensIn, tokensOut int, latencyMS int64) error {
	cost := (float64(tokensIn)*PricePerMillionInputTokens + float64(tokensOut)*PricePerMillionOutputTokens) / 1_000_000.0
	return tw.RecordModelTurnWithCost(tokensIn, tokensOut, latencyMS, cost)
}

type toolCallTraceStep struct {
	Step      int            `json:"step"`
	Type      string         `json:"type"`
	Name      string         `json:"name"`
	Args      map[string]any `json:"args"`
	LatencyMS int64          `json:"latency_ms"`
}

func (tw *TraceWriter) RecordToolCall(name string, args map[string]any, latencyMS int64) error {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	tw.step++
	if args == nil {
		args = map[string]any{}
	}

	return tw.writeLine(toolCallTraceStep{
		Step:      tw.step,
		Type:      "tool_call",
		Name:      name,
		Args:      args,
		LatencyMS: latencyMS,
	})
}

type toolResultTraceStep struct {
	Step int    `json:"step"`
	Type string `json:"type"`
	Name string `json:"name"`
	OK   bool   `json:"ok"`
}

func (tw *TraceWriter) RecordToolResult(name string, ok bool) error {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	tw.step++

	return tw.writeLine(toolResultTraceStep{
		Step: tw.step,
		Type: "tool_result",
		Name: name,
		OK:   ok,
	})
}
