package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	ErrBrioAPIKeyMissing = errors.New("BRIO_API_KEY is not set")
)

type openAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openAIToolCallFunction `json:"function"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    any              `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIFunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAIToolDefinition struct {
	Type     string                   `json:"type"`
	Function openAIFunctionDefinition `json:"function"`
}

type openAIRequest struct {
	Model    string                 `json:"model"`
	Messages []openAIMessage        `json:"messages"`
	Tools    []openAIToolDefinition `json:"tools,omitempty"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIChoiceMessage struct {
	Role      string           `json:"role"`
	Content   *string          `json:"content"`
	ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
}

type openAIChoice struct {
	Index        int                 `json:"index"`
	Message      openAIChoiceMessage `json:"message"`
	FinishReason string              `json:"finish_reason"`
}

type openAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

type OpenAIClient struct {
	apiKey         string
	model          string
	baseURL        string
	endpoint       string
	httpClient     *http.Client
	hasPricing     bool
	priceIn        float64
	priceOut       float64
	RetryBaseDelay time.Duration
}

func NewOpenAIClient(baseURL, model string) (*OpenAIClient, error) {
	return NewOpenAIClientWithHTTP(baseURL, model, nil)
}

func NewOpenAIClientWithHTTP(baseURL, model string, client *http.Client) (*OpenAIClient, error) {
	key := os.Getenv("BRIO_API_KEY")
	if strings.TrimSpace(key) == "" {
		return nil, ErrBrioAPIKeyMissing
	}

	if baseURL == "" {
		baseURL = os.Getenv("BRIO_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	if model == "" {
		model = os.Getenv("BRIO_MODEL")
	}
	if model == "" {
		model = "gpt-4o-mini"
	}

	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}

	endpoint := strings.TrimRight(baseURL, "/")
	if !strings.HasSuffix(endpoint, "/chat/completions") {
		endpoint += "/chat/completions"
	}

	var priceIn, priceOut float64
	hasPricing := false
	priceInStr := strings.TrimSpace(os.Getenv("BRIO_PRICE_IN"))
	priceOutStr := strings.TrimSpace(os.Getenv("BRIO_PRICE_OUT"))
	if priceInStr != "" && priceOutStr != "" {
		pIn, err1 := strconv.ParseFloat(priceInStr, 64)
		pOut, err2 := strconv.ParseFloat(priceOutStr, 64)
		if err1 == nil && err2 == nil {
			priceIn = pIn
			priceOut = pOut
			hasPricing = true
		}
	}

	return &OpenAIClient{
		apiKey:     key,
		model:      model,
		baseURL:    baseURL,
		endpoint:   endpoint,
		httpClient: client,
		hasPricing: hasPricing,
		priceIn:    priceIn,
		priceOut:   priceOut,
	}, nil
}

func parseRetryAfter(header string) time.Duration {
	header = strings.TrimSpace(header)
	if header == "" {
		return 0
	}
	if sec, err := strconv.Atoi(header); err == nil && sec >= 0 {
		return time.Duration(sec) * time.Second
	}
	if secFloat, err := strconv.ParseFloat(header, 64); err == nil && secFloat >= 0 {
		return time.Duration(secFloat * float64(time.Second))
	}
	if t, err := http.ParseTime(header); err == nil {
		diff := time.Until(t)
		if diff > 0 {
			return diff
		}
		return 0
	}
	return 0
}

func redactKey(text, key string) string {
	if key == "" {
		return text
	}
	return strings.ReplaceAll(text, key, "[REDACTED]")
}

func (c *OpenAIClient) SendMessage(ctx context.Context, system string, messages []Message, tools []Tool) (*ModelResponse, error) {
	var apiMsgs []openAIMessage

	if system != "" {
		apiMsgs = append(apiMsgs, openAIMessage{
			Role:    "system",
			Content: system,
		})
	}

	for _, msg := range messages {
		switch content := msg.Content.(type) {
		case string:
			apiMsgs = append(apiMsgs, openAIMessage{
				Role:    msg.Role,
				Content: content,
			})
		case []ContentBlock:
			if msg.Role == "assistant" {
				var textParts []string
				var toolCalls []openAIToolCall
				for _, b := range content {
					if b.Type == "text" && b.Text != "" {
						textParts = append(textParts, b.Text)
					} else if b.Type == "tool_use" {
						argsBytes, _ := json.Marshal(b.Input)
						toolCalls = append(toolCalls, openAIToolCall{
							ID:   b.ID,
							Type: "function",
							Function: openAIToolCallFunction{
								Name:      b.Name,
								Arguments: string(argsBytes),
							},
						})
					}
				}
				asstMsg := openAIMessage{
					Role:      "assistant",
					ToolCalls: toolCalls,
				}
				if len(textParts) > 0 {
					asstMsg.Content = strings.Join(textParts, "\n")
				}
				apiMsgs = append(apiMsgs, asstMsg)
			} else {
				// User message carrying tool results
				for _, b := range content {
					if b.Type == "tool_result" {
						apiMsgs = append(apiMsgs, openAIMessage{
							Role:       "tool",
							ToolCallID: b.ToolUseID,
							Content:    b.Content,
						})
					}
				}
			}
		}
	}

	var apiTools []openAIToolDefinition
	for _, t := range tools {
		apiTools = append(apiTools, openAIToolDefinition{
			Type: "function",
			Function: openAIFunctionDefinition{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	reqBody := openAIRequest{
		Model:    c.model,
		Messages: apiMsgs,
		Tools:    apiTools,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	maxRetries := 3
	baseDelay := c.RetryBaseDelay
	if baseDelay <= 0 {
		baseDelay = 500 * time.Millisecond
	}

	var resp *http.Response
	var respBytes []byte
	var latency int64

	for attempt := 0; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		start := time.Now()
		resp, err = c.httpClient.Do(req)
		latency = time.Since(start).Milliseconds()
		if err != nil {
			return nil, fmt.Errorf("http request: %s", redactKey(err.Error(), c.apiKey))
		}

		respBytes, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read response body: %w", err)
		}

		// Retry on 429 and 5xx if retries remaining
		if (resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode >= 500 && resp.StatusCode <= 599)) && attempt < maxRetries {
			delay := parseRetryAfter(resp.Header.Get("Retry-After"))
			if delay <= 0 {
				delay = baseDelay * time.Duration(1<<attempt)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}

		break
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet := string(respBytes)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		snippet = redactKey(snippet, c.apiKey)
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, snippet)
	}

	var openAIResp openAIResponse
	if err := json.Unmarshal(respBytes, &openAIResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(openAIResp.Choices) == 0 {
		return nil, fmt.Errorf("empty choices in model response")
	}

	choice := openAIResp.Choices[0]
	var contentBlocks []ContentBlock

	if choice.Message.Content != nil && *choice.Message.Content != "" {
		contentBlocks = append(contentBlocks, ContentBlock{
			Type: "text",
			Text: *choice.Message.Content,
		})
	}

	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		var argErr string
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
			argErr = fmt.Sprintf("malformed JSON in tool call arguments: %v", err)
		}
		contentBlocks = append(contentBlocks, ContentBlock{
			Type:     "tool_use",
			ID:       tc.ID,
			Name:     tc.Function.Name,
			Input:    args,
			ArgError: argErr,
		})
	}

	cost := 0.0
	if c.hasPricing {
		cost = (float64(openAIResp.Usage.PromptTokens)*c.priceIn + float64(openAIResp.Usage.CompletionTokens)*c.priceOut) / 1_000_000.0
	}

	return &ModelResponse{
		ID:         openAIResp.ID,
		Type:       "message",
		Role:       "assistant",
		Content:    contentBlocks,
		StopReason: choice.FinishReason,
		Usage: ModelUsage{
			InputTokens:  openAIResp.Usage.PromptTokens,
			OutputTokens: openAIResp.Usage.CompletionTokens,
		},
		CostUSD:   &cost,
		LatencyMS: latency,
	}, nil
}
