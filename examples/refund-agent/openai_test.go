package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestOpenAIClientEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		handler     func(w http.ResponseWriter, r *http.Request)
		setupEnv    func(t *testing.T)
		setupClient func(c *OpenAIClient)
		messages    []Message
		tools       []Tool
		wantErr     bool
		errSubstr   string
		checkResp   func(t *testing.T, resp *ModelResponse)
	}{
		{
			name: "normal tool call",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-key-123" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(openAIResponse{
					ID: "call_abc",
					Choices: []openAIChoice{
						{
							Index: 0,
							Message: openAIChoiceMessage{
								Role: "assistant",
								ToolCalls: []openAIToolCall{
									{
										ID:   "call_1",
										Type: "function",
										Function: openAIToolCallFunction{
											Name:      "lookup_user",
											Arguments: `{"id":"u_42"}`,
										},
									},
								},
							},
							FinishReason: "tool_calls",
						},
					},
					Usage: openAIUsage{
						PromptTokens:     150,
						CompletionTokens: 30,
					},
				})
			},
			setupEnv: func(t *testing.T) {
				t.Setenv("BRIO_API_KEY", "test-key-123")
			},
			checkResp: func(t *testing.T, resp *ModelResponse) {
				if len(resp.Content) != 1 {
					t.Fatalf("expected 1 content block, got %d", len(resp.Content))
				}
				block := resp.Content[0]
				if block.Type != "tool_use" {
					t.Errorf("expected type tool_use, got %s", block.Type)
				}
				if block.Name != "lookup_user" {
					t.Errorf("expected tool lookup_user, got %s", block.Name)
				}
				if block.Input["id"] != "u_42" {
					t.Errorf("expected id u_42, got %v", block.Input["id"])
				}
				if block.ArgError != "" {
					t.Errorf("expected no arg error, got %s", block.ArgError)
				}
				if resp.Usage.InputTokens != 150 || resp.Usage.OutputTokens != 30 {
					t.Errorf("unexpected usage: %+v", resp.Usage)
				}
				if resp.CostUSD == nil || *resp.CostUSD != 0.0 {
					t.Errorf("expected 0.0 cost without pricing env, got %v", resp.CostUSD)
				}
			},
		},
		{
			name: "no tool call",
			handler: func(w http.ResponseWriter, r *http.Request) {
				content := "Your refund has been processed."
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(openAIResponse{
					ID: "resp_no_tools",
					Choices: []openAIChoice{
						{
							Index: 0,
							Message: openAIChoiceMessage{
								Role:    "assistant",
								Content: &content,
							},
							FinishReason: "stop",
						},
					},
					Usage: openAIUsage{
						PromptTokens:     200,
						CompletionTokens: 15,
					},
				})
			},
			setupEnv: func(t *testing.T) {
				t.Setenv("BRIO_API_KEY", "test-key-123")
			},
			checkResp: func(t *testing.T, resp *ModelResponse) {
				if len(resp.Content) != 1 {
					t.Fatalf("expected 1 content block, got %d", len(resp.Content))
				}
				block := resp.Content[0]
				if block.Type != "text" {
					t.Errorf("expected type text, got %s", block.Type)
				}
				if !strings.Contains(block.Text, "refund") {
					t.Errorf("expected text to mention refund, got %s", block.Text)
				}
				if resp.StopReason != "stop" {
					t.Errorf("expected stop reason 'stop', got %s", resp.StopReason)
				}
			},
		},
		{
			name: "malformed arguments",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(openAIResponse{
					ID: "resp_bad_args",
					Choices: []openAIChoice{
						{
							Index: 0,
							Message: openAIChoiceMessage{
								Role: "assistant",
								ToolCalls: []openAIToolCall{
									{
										ID:   "call_bad",
										Type: "function",
										Function: openAIToolCallFunction{
											Name:      "lookup_user",
											Arguments: `{"id": unquoted}`,
										},
									},
								},
							},
							FinishReason: "tool_calls",
						},
					},
				})
			},
			setupEnv: func(t *testing.T) {
				t.Setenv("BRIO_API_KEY", "test-key-123")
			},
			checkResp: func(t *testing.T, resp *ModelResponse) {
				if len(resp.Content) != 1 {
					t.Fatalf("expected 1 content block, got %d", len(resp.Content))
				}
				block := resp.Content[0]
				if block.Type != "tool_use" {
					t.Errorf("expected type tool_use, got %s", block.Type)
				}
				if block.ArgError == "" {
					t.Errorf("expected ArgError for malformed JSON arguments, got none")
				}
			},
		},
		{
			name: "429 then success",
			handler: func() func(w http.ResponseWriter, r *http.Request) {
				var calls int32
				return func(w http.ResponseWriter, r *http.Request) {
					c := atomic.AddInt32(&calls, 1)
					if c == 1 {
						w.Header().Set("Retry-After", "0.001")
						w.WriteHeader(http.StatusTooManyRequests)
						_, _ = w.Write([]byte(`{"error":"rate limited"}`))
						return
					}
					content := "Success after rate limit"
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(openAIResponse{
						ID: "resp_after_retry",
						Choices: []openAIChoice{
							{
								Index: 0,
								Message: openAIChoiceMessage{
									Role:    "assistant",
									Content: &content,
								},
								FinishReason: "stop",
							},
						},
					})
				}
			}(),
			setupEnv: func(t *testing.T) {
				t.Setenv("BRIO_API_KEY", "test-key-123")
			},
			setupClient: func(c *OpenAIClient) {
				c.RetryBaseDelay = time.Millisecond
			},
			checkResp: func(t *testing.T, resp *ModelResponse) {
				if len(resp.Content) != 1 || resp.Content[0].Text != "Success after rate limit" {
					t.Errorf("expected successful response after retry, got %+v", resp)
				}
			},
		},
		{
			name: "persistent 500",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`internal server error secret-key-contained-here`))
			},
			setupEnv: func(t *testing.T) {
				t.Setenv("BRIO_API_KEY", "secret-key-contained-here")
			},
			setupClient: func(c *OpenAIClient) {
				c.RetryBaseDelay = time.Millisecond
			},
			wantErr:   true,
			errSubstr: "api error (status 500)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.handler))
			defer server.Close()

			if tt.setupEnv != nil {
				tt.setupEnv(t)
			}

			client, err := NewOpenAIClient(server.URL, "test-model")
			if err != nil {
				t.Fatalf("unexpected error creating client: %v", err)
			}
			if tt.setupClient != nil {
				tt.setupClient(client)
			}

			resp, err := client.SendMessage(context.Background(), "test system", tt.messages, tt.tools)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain expected substring %q", err.Error(), tt.errSubstr)
				}
				apiKey := os.Getenv("BRIO_API_KEY")
				if apiKey != "" && strings.Contains(err.Error(), apiKey) {
					t.Errorf("error message leaked API key: %q", err.Error())
				}
				return
			}
			if tt.checkResp != nil {
				tt.checkResp(t, resp)
			}
		})
	}
}

func TestOpenAIMissingKey(t *testing.T) {
	t.Setenv("BRIO_API_KEY", "")
	_, err := NewOpenAIClient("http://localhost", "model")
	if err == nil {
		t.Fatalf("expected error when BRIO_API_KEY is missing, got nil")
	}
	if !strings.Contains(err.Error(), "BRIO_API_KEY is not set") {
		t.Errorf("expected error mentioning BRIO_API_KEY, got: %v", err)
	}
}

func TestOpenAIUnreachableHost(t *testing.T) {
	t.Setenv("BRIO_API_KEY", "secret-key-456")
	client, err := NewOpenAIClient("http://127.0.0.1:1", "model")
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}
	_, err = client.SendMessage(context.Background(), "sys", nil, nil)
	if err == nil {
		t.Fatalf("expected error for unreachable host, got nil")
	}
	if strings.Contains(err.Error(), "secret-key-456") {
		t.Errorf("error leaked secret key: %v", err)
	}
}

func TestOpenAICustomPricing(t *testing.T) {
	t.Setenv("BRIO_API_KEY", "test-key")
	t.Setenv("BRIO_PRICE_IN", "2.50")
	t.Setenv("BRIO_PRICE_OUT", "10.00")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openAIResponse{
			ID: "pricing_test",
			Choices: []openAIChoice{
				{
					Index: 0,
					Message: openAIChoiceMessage{
						Role: "assistant",
					},
					FinishReason: "stop",
				},
			},
			Usage: openAIUsage{
				PromptTokens:     1000,
				CompletionTokens: 500,
			},
		})
	}))
	defer server.Close()

	client, err := NewOpenAIClient(server.URL, "pricing-model")
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	resp, err := client.SendMessage(context.Background(), "sys", nil, nil)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if resp.CostUSD == nil {
		t.Fatalf("expected CostUSD to be set")
	}
	expectedCost := 0.0075
	diff := *resp.CostUSD - expectedCost
	if diff < -1e-6 || diff > 1e-6 {
		t.Errorf("expected cost %f, got %f", expectedCost, *resp.CostUSD)
	}
}

func TestOpenAIRunAgentMalformedArgsReturnsToolError(t *testing.T) {
	t.Setenv("BRIO_API_KEY", "test-key")

	turn := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		turn++
		w.Header().Set("Content-Type", "application/json")

		if turn == 1 {
			_ = json.NewEncoder(w).Encode(openAIResponse{
				ID: "turn_1",
				Choices: []openAIChoice{
					{
						Index: 0,
						Message: openAIChoiceMessage{
							Role: "assistant",
							ToolCalls: []openAIToolCall{
								{
									ID:   "call_malformed",
									Type: "function",
									Function: openAIToolCallFunction{
										Name:      "lookup_user",
										Arguments: `{"id": not_json}`,
									},
								},
							},
						},
						FinishReason: "tool_calls",
					},
				},
			})
			return
		}

		var req openAIRequest
		_ = json.NewDecoder(r.Body).Decode(&req)

		foundToolError := false
		for _, m := range req.Messages {
			if m.Role == "tool" && m.ToolCallID == "call_malformed" {
				if contentStr, ok := m.Content.(string); ok && strings.Contains(contentStr, "invalid arguments") {
					foundToolError = true
				}
			}
		}
		if !foundToolError {
			t.Errorf("expected role: tool message with error content in turn 2, got messages: %+v", req.Messages)
		}

		content := "Handled tool error gracefully"
		_ = json.NewEncoder(w).Encode(openAIResponse{
			ID: "turn_2",
			Choices: []openAIChoice{
				{
					Index: 0,
					Message: openAIChoiceMessage{
						Role:    "assistant",
						Content: &content,
					},
					FinishReason: "stop",
				},
			},
		})
	}))
	defer server.Close()

	client, err := NewOpenAIClient(server.URL, "test-model")
	if err != nil {
		t.Fatalf("create client: %v", err)
	}

	store := NewStore([]User{{ID: "u_42", Name: "Alice", Status: "active", Balance: "$100"}})
	cfg := AgentConfig{
		SystemPrompt: "test prompt",
		Store:        store,
		Model:        client,
	}

	err = RunAgent(context.Background(), cfg, "Hello")
	if err != nil {
		t.Fatalf("RunAgent failed: %v", err)
	}
}
