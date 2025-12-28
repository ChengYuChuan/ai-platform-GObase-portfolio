package models

import (
	"testing"
)

func TestChatCompletionRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     ChatCompletionRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request",
			req: ChatCompletionRequest{
				Model:    "gpt-4o-mini",
				Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
			},
			wantErr: false,
		},
		{
			name: "missing model",
			req: ChatCompletionRequest{
				Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
			},
			wantErr: true,
			errMsg:  "model is required",
		},
		{
			name: "empty messages",
			req: ChatCompletionRequest{
				Model:    "gpt-4o-mini",
				Messages: []ChatMessage{},
			},
			wantErr: true,
			errMsg:  "messages array is required",
		},
		{
			name: "nil messages",
			req: ChatCompletionRequest{
				Model: "gpt-4o-mini",
			},
			wantErr: true,
			errMsg:  "messages array is required",
		},
		{
			name: "message missing role",
			req: ChatCompletionRequest{
				Model:    "gpt-4o-mini",
				Messages: []ChatMessage{{Content: "Hello"}},
			},
			wantErr: true,
			errMsg:  "message role is required",
		},
		{
			name: "invalid message role",
			req: ChatCompletionRequest{
				Model:    "gpt-4o-mini",
				Messages: []ChatMessage{{Role: "invalid", Content: "Hello"}},
			},
			wantErr: true,
			errMsg:  "invalid message role",
		},
		{
			name: "valid system role",
			req: ChatCompletionRequest{
				Model: "gpt-4o-mini",
				Messages: []ChatMessage{
					{Role: "system", Content: "You are helpful"},
					{Role: "user", Content: "Hello"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid assistant role",
			req: ChatCompletionRequest{
				Model: "gpt-4o-mini",
				Messages: []ChatMessage{
					{Role: "user", Content: "Hello"},
					{Role: "assistant", Content: "Hi there!"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid tool role",
			req: ChatCompletionRequest{
				Model: "gpt-4o-mini",
				Messages: []ChatMessage{
					{Role: "user", Content: "Hello"},
					{Role: "tool", Content: "Tool output", ToolCallID: "call_123"},
				},
			},
			wantErr: false,
		},
		{
			name: "temperature too low",
			req: ChatCompletionRequest{
				Model:       "gpt-4o-mini",
				Messages:    []ChatMessage{{Role: "user", Content: "Hello"}},
				Temperature: floatPtr(-0.5),
			},
			wantErr: true,
			errMsg:  "temperature must be between 0 and 2",
		},
		{
			name: "temperature too high",
			req: ChatCompletionRequest{
				Model:       "gpt-4o-mini",
				Messages:    []ChatMessage{{Role: "user", Content: "Hello"}},
				Temperature: floatPtr(2.5),
			},
			wantErr: true,
			errMsg:  "temperature must be between 0 and 2",
		},
		{
			name: "valid temperature at boundaries",
			req: ChatCompletionRequest{
				Model:       "gpt-4o-mini",
				Messages:    []ChatMessage{{Role: "user", Content: "Hello"}},
				Temperature: floatPtr(2.0),
			},
			wantErr: false,
		},
		{
			name: "top_p too low",
			req: ChatCompletionRequest{
				Model:    "gpt-4o-mini",
				Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
				TopP:     floatPtr(-0.1),
			},
			wantErr: true,
			errMsg:  "top_p must be between 0 and 1",
		},
		{
			name: "top_p too high",
			req: ChatCompletionRequest{
				Model:    "gpt-4o-mini",
				Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
				TopP:     floatPtr(1.5),
			},
			wantErr: true,
			errMsg:  "top_p must be between 0 and 1",
		},
		{
			name: "valid top_p",
			req: ChatCompletionRequest{
				Model:    "gpt-4o-mini",
				Messages: []ChatMessage{{Role: "user", Content: "Hello"}},
				TopP:     floatPtr(0.9),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if err.Error() != tt.errMsg && !contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestCompletionRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     CompletionRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request",
			req: CompletionRequest{
				Model:  "gpt-3.5-turbo-instruct",
				Prompt: "Once upon a time",
			},
			wantErr: false,
		},
		{
			name: "missing model",
			req: CompletionRequest{
				Prompt: "Once upon a time",
			},
			wantErr: true,
			errMsg:  "model is required",
		},
		{
			name: "missing prompt",
			req: CompletionRequest{
				Model: "gpt-3.5-turbo-instruct",
			},
			wantErr: true,
			errMsg:  "prompt is required",
		},
		{
			name: "empty prompt",
			req: CompletionRequest{
				Model:  "gpt-3.5-turbo-instruct",
				Prompt: "",
			},
			wantErr: true,
			errMsg:  "prompt is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEmbeddingRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     EmbeddingRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid string input",
			req: EmbeddingRequest{
				Model: "text-embedding-3-small",
				Input: "Hello world",
			},
			wantErr: false,
		},
		{
			name: "valid array input",
			req: EmbeddingRequest{
				Model: "text-embedding-3-small",
				Input: []string{"Hello", "World"},
			},
			wantErr: false,
		},
		{
			name: "missing model",
			req: EmbeddingRequest{
				Input: "Hello world",
			},
			wantErr: true,
			errMsg:  "model is required",
		},
		{
			name: "missing input",
			req: EmbeddingRequest{
				Model: "text-embedding-3-small",
			},
			wantErr: true,
			errMsg:  "input is required",
		},
		{
			name: "nil input",
			req: EmbeddingRequest{
				Model: "text-embedding-3-small",
				Input: nil,
			},
			wantErr: true,
			errMsg:  "input is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAnthropicMessageRequest_ToChatCompletionRequest(t *testing.T) {
	temp := 0.7

	tests := []struct {
		name     string
		anthReq  AnthropicMessageRequest
		expected ChatCompletionRequest
	}{
		{
			name: "basic conversion",
			anthReq: AnthropicMessageRequest{
				Model:     "claude-3-haiku-20240307",
				MaxTokens: 1000,
				Messages:  []ChatMessage{{Role: "user", Content: "Hello"}},
			},
			expected: ChatCompletionRequest{
				Model:     "claude-3-haiku-20240307",
				MaxTokens: 1000,
				Messages:  []ChatMessage{{Role: "user", Content: "Hello"}},
			},
		},
		{
			name: "with system message",
			anthReq: AnthropicMessageRequest{
				Model:     "claude-3-haiku-20240307",
				MaxTokens: 1000,
				System:    "You are a helpful assistant",
				Messages:  []ChatMessage{{Role: "user", Content: "Hello"}},
			},
			expected: ChatCompletionRequest{
				Model:     "claude-3-haiku-20240307",
				MaxTokens: 1000,
				Messages: []ChatMessage{
					{Role: "system", Content: "You are a helpful assistant"},
					{Role: "user", Content: "Hello"},
				},
			},
		},
		{
			name: "with all fields",
			anthReq: AnthropicMessageRequest{
				Model:       "claude-3-5-sonnet-20241022",
				MaxTokens:   2000,
				System:      "Be concise",
				Temperature: &temp,
				Stream:      true,
				StopSeq:     []string{"END", "STOP"},
				Messages:    []ChatMessage{{Role: "user", Content: "Test"}},
			},
			expected: ChatCompletionRequest{
				Model:       "claude-3-5-sonnet-20241022",
				MaxTokens:   2000,
				Temperature: &temp,
				Stream:      true,
				Stop:        []string{"END", "STOP"},
				Messages: []ChatMessage{
					{Role: "system", Content: "Be concise"},
					{Role: "user", Content: "Test"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.anthReq.ToChatCompletionRequest()

			if got.Model != tt.expected.Model {
				t.Errorf("Model = %v, want %v", got.Model, tt.expected.Model)
			}
			if got.MaxTokens != tt.expected.MaxTokens {
				t.Errorf("MaxTokens = %v, want %v", got.MaxTokens, tt.expected.MaxTokens)
			}
			if got.Stream != tt.expected.Stream {
				t.Errorf("Stream = %v, want %v", got.Stream, tt.expected.Stream)
			}
			if len(got.Messages) != len(tt.expected.Messages) {
				t.Errorf("len(Messages) = %v, want %v", len(got.Messages), len(tt.expected.Messages))
			}
			for i := range got.Messages {
				if got.Messages[i].Role != tt.expected.Messages[i].Role {
					t.Errorf("Messages[%d].Role = %v, want %v", i, got.Messages[i].Role, tt.expected.Messages[i].Role)
				}
				if got.Messages[i].Content != tt.expected.Messages[i].Content {
					t.Errorf("Messages[%d].Content = %v, want %v", i, got.Messages[i].Content, tt.expected.Messages[i].Content)
				}
			}
		})
	}
}

func TestChatMessage_Roles(t *testing.T) {
	validRoles := []string{"system", "user", "assistant", "tool"}

	for _, role := range validRoles {
		msg := ChatMessage{Role: role, Content: "test"}
		req := ChatCompletionRequest{
			Model:    "gpt-4",
			Messages: []ChatMessage{msg},
		}
		if err := req.Validate(); err != nil {
			t.Errorf("role %q should be valid, got error: %v", role, err)
		}
	}
}

func TestChatMessage_ToolCall(t *testing.T) {
	msg := ChatMessage{
		Role: "assistant",
		ToolCalls: []ToolCall{
			{
				ID:   "call_123",
				Type: "function",
				Function: FunctionCall{
					Name:      "get_weather",
					Arguments: `{"location": "San Francisco"}`,
				},
			},
		},
	}

	if len(msg.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Name != "get_weather" {
		t.Errorf("function name = %s, want get_weather", msg.ToolCalls[0].Function.Name)
	}
}

func TestFunction_Fields(t *testing.T) {
	fn := Function{
		Name:        "get_weather",
		Description: "Get the current weather",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]string{"type": "string"},
			},
		},
	}

	if fn.Name != "get_weather" {
		t.Errorf("Name = %s, want get_weather", fn.Name)
	}
	if fn.Description != "Get the current weather" {
		t.Errorf("Description mismatch")
	}
}

func TestTool_Fields(t *testing.T) {
	tool := Tool{
		Type: "function",
		Function: Function{
			Name:        "search",
			Description: "Search the web",
		},
	}

	if tool.Type != "function" {
		t.Errorf("Type = %s, want function", tool.Type)
	}
	if tool.Function.Name != "search" {
		t.Errorf("Function.Name = %s, want search", tool.Function.Name)
	}
}

func TestResponseFormat_Types(t *testing.T) {
	tests := []struct {
		format string
	}{
		{"text"},
		{"json_object"},
	}

	for _, tt := range tests {
		rf := ResponseFormat{Type: tt.format}
		if rf.Type != tt.format {
			t.Errorf("Type = %s, want %s", rf.Type, tt.format)
		}
	}
}

// Helper functions
func floatPtr(f float64) *float64 {
	return &f
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
