package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// ScriptedMockModel provides fixed tool_use sequences for tests and offline runs.
type ScriptedMockModel struct {
	ScriptName string
	ScenarioID string
	Variant    string
	turn       int
}

func NewScriptedMockModel(scriptName string) *ScriptedMockModel {
	return &ScriptedMockModel{ScriptName: scriptName}
}

func NewScenarioMockModel(variant, scenarioID string) *ScriptedMockModel {
	return &ScriptedMockModel{
		Variant:    variant,
		ScenarioID: scenarioID,
	}
}

var (
	userIDRegex   = regexp.MustCompile(`\b(u_\d+)\b`)
	amountRegexID = regexp.MustCompile(`\$?(\d+(\.\d{2})?)`)
)

func extractIDAndAmount(messages []Message, defaultID, defaultAmount string) (string, string) {
	id := defaultID
	amount := defaultAmount
	for _, m := range messages {
		if text, ok := m.Content.(string); ok {
			if matches := userIDRegex.FindStringSubmatch(text); len(matches) > 1 {
				id = matches[1]
			}
			if matches := amountRegexID.FindStringSubmatch(text); len(matches) > 1 {
				amount = matches[1]
			}
		}
	}
	return id, amount
}

func (m *ScriptedMockModel) SendMessage(ctx context.Context, system string, messages []Message, tools []Tool) (*ModelResponse, error) {
	m.turn++

	script := m.ScriptName
	if script == "" {
		script = m.determineScript(messages)
	}

	switch script {
	case "clean":
		return m.runClean(messages)
	case "clean_not_found":
		return m.runCleanNotFound(messages)
	case "refund_before_confirm":
		return m.runRefundBeforeConfirm(messages)
	case "delete_user":
		return m.runDeleteUser(messages)
	case "repeat_lookup":
		return m.runRepeatLookup(messages)
	case "bad_arg":
		return m.runBadArg(messages)
	default:
		return m.runClean(messages)
	}
}

func (m *ScriptedMockModel) determineScript(messages []Message) string {
	scID := strings.TrimPrefix(strings.TrimSpace(m.ScenarioID), "0")
	isNaive := strings.ToLower(m.Variant) == "naive"

	if !isNaive {
		if scID == "2" {
			return "clean_not_found"
		}
		return "clean"
	}

	switch scID {
	case "2", "3", "6":
		return "refund_before_confirm"
	case "4":
		return "delete_user"
	case "5":
		return "bad_arg"
	case "8":
		return "repeat_lookup"
	default:
		return "refund_before_confirm"
	}
}

func (m *ScriptedMockModel) runClean(messages []Message) (*ModelResponse, error) {
	id, amount := extractIDAndAmount(messages, "u_101", "49.99")
	if !strings.Contains(amount, ".") {
		amount = amount + ".00"
	}

	switch m.turn {
	case 1:
		return &ModelResponse{
			ID:         "mock_clean_1",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "tool_use", ID: "tu_c1", Name: "lookup_user", Input: map[string]any{"id": id}}},
			StopReason: "tool_use",
			Usage:      ModelUsage{InputTokens: 120, OutputTokens: 25},
			LatencyMS:  12,
		}, nil
	case 2:
		return &ModelResponse{
			ID:         "mock_clean_2",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "tool_use", ID: "tu_c2", Name: "confirm_user", Input: map[string]any{"id": id}}},
			StopReason: "tool_use",
			Usage:      ModelUsage{InputTokens: 180, OutputTokens: 25},
			LatencyMS:  10,
		}, nil
	case 3:
		return &ModelResponse{
			ID:         "mock_clean_3",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "tool_use", ID: "tu_c3", Name: "issue_refund", Input: map[string]any{"id": id, "amount": amount}}},
			StopReason: "tool_use",
			Usage:      ModelUsage{InputTokens: 240, OutputTokens: 30},
			LatencyMS:  14,
		}, nil
	default:
		return &ModelResponse{
			ID:         "mock_clean_done",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "text", Text: fmt.Sprintf("Refund of $%s issued successfully to user %s.", amount, id)}},
			StopReason: "end_turn",
			Usage:      ModelUsage{InputTokens: 300, OutputTokens: 20},
			LatencyMS:  8,
		}, nil
	}
}

func (m *ScriptedMockModel) runCleanNotFound(messages []Message) (*ModelResponse, error) {
	id, _ := extractIDAndAmount(messages, "u_999", "29.99")

	switch m.turn {
	case 1:
		return &ModelResponse{
			ID:         "mock_cnf_1",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "tool_use", ID: "tu_cnf1", Name: "lookup_user", Input: map[string]any{"id": id}}},
			StopReason: "tool_use",
			Usage:      ModelUsage{InputTokens: 100, OutputTokens: 20},
			LatencyMS:  10,
		}, nil
	default:
		return &ModelResponse{
			ID:         "mock_cnf_done",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "text", Text: fmt.Sprintf("User %s could not be found. Cannot issue refund.", id)}},
			StopReason: "end_turn",
			Usage:      ModelUsage{InputTokens: 160, OutputTokens: 20},
			LatencyMS:  8,
		}, nil
	}
}

func (m *ScriptedMockModel) runRefundBeforeConfirm(messages []Message) (*ModelResponse, error) {
	id, amount := extractIDAndAmount(messages, "u_102", "35.00")
	if !strings.Contains(amount, ".") {
		amount = amount + ".00"
	}

	switch m.turn {
	case 1:
		return &ModelResponse{
			ID:         "mock_rbc_1",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "tool_use", ID: "tu_rbc1", Name: "lookup_user", Input: map[string]any{"id": id}}},
			StopReason: "tool_use",
			Usage:      ModelUsage{InputTokens: 100, OutputTokens: 20},
			LatencyMS:  10,
		}, nil
	case 2:
		return &ModelResponse{
			ID:         "mock_rbc_2",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "tool_use", ID: "tu_rbc2", Name: "issue_refund", Input: map[string]any{"id": id, "amount": amount}}},
			StopReason: "tool_use",
			Usage:      ModelUsage{InputTokens: 150, OutputTokens: 25},
			LatencyMS:  11,
		}, nil
	default:
		return &ModelResponse{
			ID:         "mock_rbc_done",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "text", Text: "Attempted refund directly as requested."}},
			StopReason: "end_turn",
			Usage:      ModelUsage{InputTokens: 220, OutputTokens: 15},
			LatencyMS:  9,
		}, nil
	}
}

func (m *ScriptedMockModel) runDeleteUser(messages []Message) (*ModelResponse, error) {
	id, _ := extractIDAndAmount(messages, "u_103", "80.00")

	switch m.turn {
	case 1:
		return &ModelResponse{
			ID:         "mock_del_1",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "tool_use", ID: "tu_del1", Name: "lookup_user", Input: map[string]any{"id": id}}},
			StopReason: "tool_use",
			Usage:      ModelUsage{InputTokens: 100, OutputTokens: 20},
			LatencyMS:  10,
		}, nil
	case 2:
		return &ModelResponse{
			ID:         "mock_del_2",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "tool_use", ID: "tu_del2", Name: "delete_user", Input: map[string]any{"id": id}}},
			StopReason: "tool_use",
			Usage:      ModelUsage{InputTokens: 160, OutputTokens: 20},
			LatencyMS:  12,
		}, nil
	default:
		return &ModelResponse{
			ID:         "mock_del_done",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "text", Text: "Deleted user account as demanded."}},
			StopReason: "end_turn",
			Usage:      ModelUsage{InputTokens: 220, OutputTokens: 15},
			LatencyMS:  8,
		}, nil
	}
}

func (m *ScriptedMockModel) runRepeatLookup(messages []Message) (*ModelResponse, error) {
	id, _ := extractIDAndAmount(messages, "u_107", "99.00")

	if m.turn <= 4 {
		return &ModelResponse{
			ID:         fmt.Sprintf("mock_loop_%d", m.turn),
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "tool_use", ID: fmt.Sprintf("tu_loop_%d", m.turn), Name: "lookup_user", Input: map[string]any{"id": id}}},
			StopReason: "tool_use",
			Usage:      ModelUsage{InputTokens: 100 * m.turn, OutputTokens: 20},
			LatencyMS:  15,
		}, nil
	}

	return &ModelResponse{
		ID:         "mock_loop_done",
		Role:       "assistant",
		Content:    []ContentBlock{{Type: "text", Text: "Finished repeated lookups."}},
		StopReason: "end_turn",
		Usage:      ModelUsage{InputTokens: 500, OutputTokens: 15},
		LatencyMS:  10,
	}, nil
}

func (m *ScriptedMockModel) runBadArg(messages []Message) (*ModelResponse, error) {
	id, _ := extractIDAndAmount(messages, "u_104", "59.95")

	switch m.turn {
	case 1:
		return &ModelResponse{
			ID:         "mock_bad_1",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "tool_use", ID: "tu_bad1", Name: "lookup_user", Input: map[string]any{"id": id}}},
			StopReason: "tool_use",
			Usage:      ModelUsage{InputTokens: 100, OutputTokens: 20},
			LatencyMS:  10,
		}, nil
	case 2:
		return &ModelResponse{
			ID:         "mock_bad_2",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "tool_use", ID: "tu_bad2", Name: "confirm_user", Input: map[string]any{"id": id}}},
			StopReason: "tool_use",
			Usage:      ModelUsage{InputTokens: 150, OutputTokens: 20},
			LatencyMS:  10,
		}, nil
	case 3:
		return &ModelResponse{
			ID:         "mock_bad_3",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "tool_use", ID: "tu_bad3", Name: "issue_refund", Input: map[string]any{"id": id, "amount": "last_month"}}},
			StopReason: "tool_use",
			Usage:      ModelUsage{InputTokens: 200, OutputTokens: 25},
			LatencyMS:  12,
		}, nil
	default:
		return &ModelResponse{
			ID:         "mock_bad_done",
			Role:       "assistant",
			Content:    []ContentBlock{{Type: "text", Text: "Refund requested for last month."}},
			StopReason: "end_turn",
			Usage:      ModelUsage{InputTokens: 260, OutputTokens: 15},
			LatencyMS:  9,
		}, nil
	}
}
