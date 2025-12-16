package models

import (
	"errors"
)

// ChatCompletionRequest represents an OpenAI-compatible chat completion request
type ChatCompletionRequest struct {
	Model            string         `json:"model"`
	Messages         []ChatMessage  `json:"messages"`
	Temperature      *float64       `json:"temperature,omitempty"`
	TopP             *float64       `json:"top_p,omitempty"`
	N                int            `json:"n,omitempty"`
	Stream           bool           `json:"stream,omitempty"`
	Stop             []string       `json:"stop,omitempty"`
	MaxTokens        int            `json:"max_tokens,omitempty"`
	PresencePenalty  float64        `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64        `json:"frequency_penalty,omitempty"`
	LogitBias        map[string]int `json:"logit_bias,omitempty"`
	User             string         `json:"user,omitempty"`
	// Function calling (OpenAI)
	Functions    []Function `json:"functions,omitempty"`
	FunctionCall interface{} `json:"function_call,omitempty"`
	// Tool use (newer API)
	Tools      []Tool      `json:"tools,omitempty"`
	ToolChoice interface{} `json:"tool_choice,omitempty"`
	// Response format
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	// Seed for reproducibility
	Seed *int `json:"seed,omitempty"`
}

// ChatMessage represents a message in a chat completion request
type ChatMessage struct {
	Role       string      `json:"role"`
	Content    string      `json:"content"`
	Name       string      `json:"name,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// Function represents a function definition for function calling
type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

// Tool represents a tool definition
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// ToolCall represents a tool call in a message
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function call
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ResponseFormat specifies the format of the response
type ResponseFormat struct {
	Type string `json:"type"` // "text" or "json_object"
}

// Validate validates the chat completion request
func (r *ChatCompletionRequest) Validate() error {
	if r.Model == "" {
		return errors.New("model is required")
	}
	if len(r.Messages) == 0 {
		return errors.New("messages array is required and must not be empty")
	}
	for i, msg := range r.Messages {
		if msg.Role == "" {
			return errors.New("message role is required at index " + string(rune(i)))
		}
		if msg.Role != "system" && msg.Role != "user" && msg.Role != "assistant" && msg.Role != "tool" {
			return errors.New("invalid message role: " + msg.Role)
		}
	}
	if r.Temperature != nil && (*r.Temperature < 0 || *r.Temperature > 2) {
		return errors.New("temperature must be between 0 and 2")
	}
	if r.TopP != nil && (*r.TopP < 0 || *r.TopP > 1) {
		return errors.New("top_p must be between 0 and 1")
	}
	return nil
}

// CompletionRequest represents a legacy completion request
type CompletionRequest struct {
	Model            string   `json:"model"`
	Prompt           string   `json:"prompt"`
	Suffix           string   `json:"suffix,omitempty"`
	MaxTokens        int      `json:"max_tokens,omitempty"`
	Temperature      *float64 `json:"temperature,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	N                int      `json:"n,omitempty"`
	Stream           bool     `json:"stream,omitempty"`
	LogProbs         *int     `json:"logprobs,omitempty"`
	Echo             bool     `json:"echo,omitempty"`
	Stop             []string `json:"stop,omitempty"`
	PresencePenalty  float64  `json:"presence_penalty,omitempty"`
	FrequencyPenalty float64  `json:"frequency_penalty,omitempty"`
	BestOf           int      `json:"best_of,omitempty"`
	User             string   `json:"user,omitempty"`
}

// Validate validates the completion request
func (r *CompletionRequest) Validate() error {
	if r.Model == "" {
		return errors.New("model is required")
	}
	if r.Prompt == "" {
		return errors.New("prompt is required")
	}
	return nil
}

// EmbeddingRequest represents an embedding request
type EmbeddingRequest struct {
	Model          string      `json:"model"`
	Input          interface{} `json:"input"` // string or []string
	User           string      `json:"user,omitempty"`
	EncodingFormat string      `json:"encoding_format,omitempty"` // "float" or "base64"
	Dimensions     int         `json:"dimensions,omitempty"`
}

// Validate validates the embedding request
func (r *EmbeddingRequest) Validate() error {
	if r.Model == "" {
		return errors.New("model is required")
	}
	if r.Input == nil {
		return errors.New("input is required")
	}
	return nil
}

// AnthropicMessageRequest represents an Anthropic-style message request
type AnthropicMessageRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	System      string        `json:"system,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
	TopP        *float64      `json:"top_p,omitempty"`
	TopK        *int          `json:"top_k,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	StopSeq     []string      `json:"stop_sequences,omitempty"`
	Metadata    interface{}   `json:"metadata,omitempty"`
}

// ToChatCompletionRequest converts Anthropic request to OpenAI format
func (r *AnthropicMessageRequest) ToChatCompletionRequest() *ChatCompletionRequest {
	messages := r.Messages
	
	// Add system message if present
	if r.System != "" {
		messages = append([]ChatMessage{
			{Role: "system", Content: r.System},
		}, messages...)
	}

	return &ChatCompletionRequest{
		Model:       r.Model,
		Messages:    messages,
		MaxTokens:   r.MaxTokens,
		Temperature: r.Temperature,
		TopP:        r.TopP,
		Stream:      r.Stream,
		Stop:        r.StopSeq,
	}
}
