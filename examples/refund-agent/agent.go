package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const MaxModelTurns = 15

var ErrTurnCapReached = errors.New("maximum model turns (15) reached")

type AgentConfig struct {
	SystemPrompt string
	Store        *Store
	Model        ModelClient
	TraceWriter  *TraceWriter
}

func RunAgent(ctx context.Context, cfg AgentConfig, userMessage string) error {
	messages := []Message{
		{
			Role:    "user",
			Content: userMessage,
		},
	}

	for turn := 0; turn < MaxModelTurns; turn++ {
		resp, err := cfg.Model.SendMessage(ctx, cfg.SystemPrompt, messages, AgentTools)
		if err != nil {
			return fmt.Errorf("model turn %d: %w", turn+1, err)
		}

		var toolUses []ContentBlock
		var textParts []string
		for _, b := range resp.Content {
			if b.Type == "tool_use" {
				toolUses = append(toolUses, b)
			} else if b.Type == "text" && b.Text != "" {
				textParts = append(textParts, b.Text)
			}
		}

		if cfg.TraceWriter != nil {
			tokensIn := resp.Usage.InputTokens
			tokensOut := resp.Usage.OutputTokens
			if tokensIn == 0 && tokensOut == 0 {
				tokensIn = 100
				tokensOut = 50
			}
			var replyText string
			if len(toolUses) == 0 && len(textParts) > 0 {
				replyText = strings.Join(textParts, "\n")
				runes := []rune(replyText)
				if len(runes) > 400 {
					replyText = string(runes[:400])
				}
			}
			var recordErr error
			if resp.CostUSD != nil {
				recordErr = cfg.TraceWriter.RecordModelTurnWithCostAndText(tokensIn, tokensOut, resp.LatencyMS, *resp.CostUSD, replyText)
			} else {
				recordErr = cfg.TraceWriter.RecordModelTurnWithText(tokensIn, tokensOut, resp.LatencyMS, replyText)
			}
			if recordErr != nil {
				return recordErr
			}
		}

		messages = append(messages, Message{
			Role:    "assistant",
			Content: resp.Content,
		})

		if len(toolUses) == 0 {
			return nil
		}

		var resultBlocks []ContentBlock
		for _, tu := range toolUses {
			start := time.Now()
			var res map[string]any
			var toolErr error
			if tu.ArgError != "" {
				toolErr = fmt.Errorf("invalid arguments: %s", tu.ArgError)
			} else {
				res, toolErr = ExecuteTool(cfg.Store, tu.Name, tu.Input)
			}
			latency := time.Since(start).Milliseconds()

			if cfg.TraceWriter != nil {
				if err := cfg.TraceWriter.RecordToolCall(tu.Name, tu.Input, latency); err != nil {
					return err
				}
				if err := cfg.TraceWriter.RecordToolResult(tu.Name, toolErr == nil); err != nil {
					return err
				}
			}

			var resultContent string
			isError := false
			if toolErr != nil {
				isError = true
				resultContent = fmt.Sprintf("Error: %v", toolErr)
			} else {
				resBytes, _ := json.Marshal(res)
				resultContent = string(resBytes)
			}

			resultBlocks = append(resultBlocks, ContentBlock{
				Type:      "tool_result",
				ToolUseID: tu.ID,
				Content:   resultContent,
				IsError:   isError,
			})
		}

		messages = append(messages, Message{
			Role:    "user",
			Content: resultBlocks,
		})
	}

	return ErrTurnCapReached
}
