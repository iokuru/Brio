package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type sequenceMockClient struct {
	responses []*ModelResponse
	calls     int
	sentMsgs  [][]Message
}

func (m *sequenceMockClient) SendMessage(ctx context.Context, system string, messages []Message, tools []Tool) (*ModelResponse, error) {
	m.sentMsgs = append(m.sentMsgs, messages)
	if m.calls >= len(m.responses) {
		return &ModelResponse{
			ID:         "msg_end",
			Type:       "message",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "text", Text: "Done"}},
			StopReason: "end_turn",
			Usage:      ModelUsage{InputTokens: 50, OutputTokens: 10},
			LatencyMS:  10,
		}, nil
	}
	resp := m.responses[m.calls]
	m.calls++
	return resp, nil
}

func TestAgentLoopNormalCompletion(t *testing.T) {
	store := sampleTestStore()
	mock := &sequenceMockClient{
		responses: []*ModelResponse{
			{
				ID:   "msg_1",
				Role: "assistant",
				Content: []ContentBlock{
					{
						Type:  "tool_use",
						ID:    "tu_1",
						Name:  "lookup_user",
						Input: map[string]any{"id": "u_1"},
					},
				},
				StopReason: "tool_use",
				Usage:      ModelUsage{InputTokens: 100, OutputTokens: 20},
				LatencyMS:  15,
			},
			{
				ID:   "msg_2",
				Role: "assistant",
				Content: []ContentBlock{
					{Type: "text", Text: "User Alice looked up successfully."},
				},
				StopReason: "end_turn",
				Usage:      ModelUsage{InputTokens: 150, OutputTokens: 30},
				LatencyMS:  12,
			},
		},
	}

	var traceBuf bytes.Buffer
	tw := NewTraceWriter(&traceBuf)

	cfg := AgentConfig{
		SystemPrompt: "test",
		Store:        store,
		Model:        mock,
		TraceWriter:  tw,
	}

	err := RunAgent(context.Background(), cfg, "Check user u_1")
	if err != nil {
		t.Fatalf("unexpected agent loop error: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(traceBuf.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 trace lines (model, tool_call, tool_result, model), got %d:\n%s", len(lines), traceBuf.String())
	}

	for i, l := range lines {
		var raw map[string]any
		if err := json.Unmarshal([]byte(l), &raw); err != nil {
			t.Fatalf("line %d malformed: %v", i+1, err)
		}
		expectedStep := float64(i + 1)
		if raw["step"] != expectedStep {
			t.Errorf("line %d: expected step %v, got %v", i+1, expectedStep, raw["step"])
		}
	}
}

func TestAgentLoopTurnCapReached(t *testing.T) {
	store := sampleTestStore()

	loopResp := &ModelResponse{
		ID:   "msg_loop",
		Role: "assistant",
		Content: []ContentBlock{
			{
				Type:  "tool_use",
				ID:    "tu_loop",
				Name:  "lookup_user",
				Input: map[string]any{"id": "u_1"},
			},
		},
		StopReason: "tool_use",
		Usage:      ModelUsage{InputTokens: 50, OutputTokens: 20},
		LatencyMS:  10,
	}

	mock := &sequenceMockClient{}
	for i := 0; i < 20; i++ {
		mock.responses = append(mock.responses, loopResp)
	}

	var traceBuf bytes.Buffer
	tw := NewTraceWriter(&traceBuf)

	cfg := AgentConfig{
		SystemPrompt: "test",
		Store:        store,
		Model:        mock,
		TraceWriter:  tw,
	}

	err := RunAgent(context.Background(), cfg, "Loop forever")
	if !errors.Is(err, ErrTurnCapReached) {
		t.Fatalf("expected ErrTurnCapReached, got %v", err)
	}

	if mock.calls != MaxModelTurns {
		t.Errorf("expected %d model calls, got %d", MaxModelTurns, mock.calls)
	}
}

func TestAgentLoopToolErrorSurfacedToModel(t *testing.T) {
	store := sampleTestStore() // u_1 is unconfirmed

	mock := &sequenceMockClient{
		responses: []*ModelResponse{
			{
				ID:   "msg_1",
				Role: "assistant",
				Content: []ContentBlock{
					{
						Type:  "tool_use",
						ID:    "tu_refund_fail",
						Name:  "issue_refund",
						Input: map[string]any{"id": "u_1", "amount": "50.00"},
					},
				},
				StopReason: "tool_use",
				Usage:      ModelUsage{InputTokens: 100, OutputTokens: 20},
			},
			{
				ID:         "msg_2",
				Role:       "assistant",
				Content:    []ContentBlock{{Type: "text", Text: "Understood, user not confirmed."}},
				StopReason: "end_turn",
				Usage:      ModelUsage{InputTokens: 150, OutputTokens: 15},
			},
		},
	}

	var traceBuf bytes.Buffer
	tw := NewTraceWriter(&traceBuf)

	cfg := AgentConfig{
		SystemPrompt: "test",
		Store:        store,
		Model:        mock,
		TraceWriter:  tw,
	}

	err := RunAgent(context.Background(), cfg, "Refund u_1 directly")
	if err != nil {
		t.Fatalf("unexpected agent error: %v", err)
	}

	if len(mock.sentMsgs) < 2 {
		t.Fatalf("expected at least 2 calls to model, got %d", len(mock.sentMsgs))
	}
	secondCallMsgs := mock.sentMsgs[1]
	lastMsg := secondCallMsgs[len(secondCallMsgs)-1]
	blocks, ok := lastMsg.Content.([]ContentBlock)
	if !ok || len(blocks) == 0 {
		t.Fatalf("expected ContentBlocks in last message, got %+v", lastMsg.Content)
	}
	if !blocks[0].IsError || !strings.Contains(blocks[0].Content, "user must be confirmed before issuing a refund") {
		t.Errorf("expected tool error in tool_result block, got: %+v", blocks[0])
	}

	traceOutput := traceBuf.String()
	if !strings.Contains(traceOutput, `"ok":false`) {
		t.Errorf("expected ok:false in trace, got:\n%s", traceOutput)
	}
}

func TestAgentLoopMalformedToolUse(t *testing.T) {
	store := sampleTestStore()

	mock := &sequenceMockClient{
		responses: []*ModelResponse{
			{
				ID:   "msg_1",
				Role: "assistant",
				Content: []ContentBlock{
					{
						Type:  "tool_use",
						ID:    "tu_bad",
						Name:  "lookup_user",
						Input: map[string]any{}, // missing id
					},
				},
				StopReason: "tool_use",
				Usage:      ModelUsage{InputTokens: 80, OutputTokens: 15},
			},
			{
				ID:         "msg_2",
				Role:       "assistant",
				Content:    []ContentBlock{{Type: "text", Text: "Got error, fixing."}},
				StopReason: "end_turn",
				Usage:      ModelUsage{InputTokens: 120, OutputTokens: 10},
			},
		},
	}

	var traceBuf bytes.Buffer
	tw := NewTraceWriter(&traceBuf)

	cfg := AgentConfig{
		SystemPrompt: "test",
		Store:        store,
		Model:        mock,
		TraceWriter:  tw,
	}

	err := RunAgent(context.Background(), cfg, "Bad call")
	if err != nil {
		t.Fatalf("unexpected agent error: %v", err)
	}

	if !strings.Contains(traceBuf.String(), `"ok":false`) {
		t.Errorf("expected ok:false in trace for malformed tool call, got:\n%s", traceBuf.String())
	}
}
