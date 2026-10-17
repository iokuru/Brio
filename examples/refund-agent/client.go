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
	"strings"
	"time"
)

var (
	ErrAPIKeyMissing = errors.New("ANTHROPIC_API_KEY is not set")
)

type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type ContentBlock struct {
	Type      string         `json:"type"`
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   string         `json:"content,omitempty"`
	IsError   bool           `json:"is_error,omitempty"`
}

type ModelUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ModelResponse struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Role       string         `json:"role"`
	Content    []ContentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
	Usage      ModelUsage     `json:"usage"`
	LatencyMS  int64          `json:"latency_ms"`
}

type ModelClient interface {
	SendMessage(ctx context.Context, system string, messages []Message, tools []Tool) (*ModelResponse, error)
}

type AnthropicClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
	baseURL    string
}

func NewAnthropicClient() (*AnthropicClient, error) {
	key := os.Getenv("ANTHROPIC_API_KEY")
	if strings.TrimSpace(key) == "" {
		return nil, ErrAPIKeyMissing
	}
	model := os.Getenv("BRIO_MODEL")
	if strings.TrimSpace(model) == "" {
		model = "claude-haiku-5-5"
	}
	return &AnthropicClient{
		apiKey:     key,
		model:      model,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		baseURL:    "https://api.anthropic.com/v1/messages",
	}, nil
}

type apiRequestBody struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
	Tools     []Tool    `json:"tools,omitempty"`
}

func (c *AnthropicClient) SendMessage(ctx context.Context, system string, messages []Message, tools []Tool) (*ModelResponse, error) {
	reqBody := apiRequestBody{
		Model:     c.model,
		MaxTokens: 1024,
		System:    system,
		Messages:  messages,
		Tools:     tools,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("api error (status %d): %s", resp.StatusCode, string(respBytes))
	}

	var modelResp ModelResponse
	if err := json.Unmarshal(respBytes, &modelResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	modelResp.LatencyMS = latency

	return &modelResp, nil
}
