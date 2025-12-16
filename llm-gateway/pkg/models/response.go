package models

// ChatCompletionResponse represents an OpenAI-compatible chat completion response
type ChatCompletionResponse struct {
	ID                string                 `json:"id"`
	Object            string                 `json:"object"`
	Created           int64                  `json:"created"`
	Model             string                 `json:"model"`
	Choices           []ChatCompletionChoice `json:"choices"`
	Usage             Usage                  `json:"usage"`
	SystemFingerprint string                 `json:"system_fingerprint,omitempty"`
}

// ChatCompletionChoice represents a choice in a chat completion response
type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
	LogProbs     *LogProbs   `json:"logprobs,omitempty"`
}

// ChatCompletionStreamResponse represents a streaming chat completion chunk
type ChatCompletionStreamResponse struct {
	ID                string                       `json:"id"`
	Object            string                       `json:"object"`
	Created           int64                        `json:"created"`
	Model             string                       `json:"model"`
	Choices           []ChatCompletionStreamChoice `json:"choices"`
	SystemFingerprint string                       `json:"system_fingerprint,omitempty"`
}

// ChatCompletionStreamChoice represents a choice in a streaming response
type ChatCompletionStreamChoice struct {
	Index        int              `json:"index"`
	Delta        ChatMessageDelta `json:"delta"`
	FinishReason *string          `json:"finish_reason"`
	LogProbs     *LogProbs        `json:"logprobs,omitempty"`
}

// ChatMessageDelta represents the delta content in streaming
type ChatMessageDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// CompletionResponse represents a legacy completion response
type CompletionResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []CompletionChoice `json:"choices"`
	Usage   Usage              `json:"usage"`
}

// CompletionChoice represents a choice in a completion response
type CompletionChoice struct {
	Text         string    `json:"text"`
	Index        int       `json:"index"`
	LogProbs     *LogProbs `json:"logprobs,omitempty"`
	FinishReason string    `json:"finish_reason"`
}

// EmbeddingResponse represents an embedding response
type EmbeddingResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage"`
}

// EmbeddingData represents a single embedding
type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbeddingUsage represents token usage for embeddings
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// Usage represents token usage information
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// LogProbs represents log probability information
type LogProbs struct {
	Content []LogProbContent `json:"content,omitempty"`
}

// LogProbContent represents log probability for a token
type LogProbContent struct {
	Token       string             `json:"token"`
	LogProb     float64            `json:"logprob"`
	Bytes       []int              `json:"bytes,omitempty"`
	TopLogProbs []TopLogProbEntry  `json:"top_logprobs,omitempty"`
}

// TopLogProbEntry represents a top log probability entry
type TopLogProbEntry struct {
	Token   string  `json:"token"`
	LogProb float64 `json:"logprob"`
	Bytes   []int   `json:"bytes,omitempty"`
}

// Model represents a model in the models list response
type Model struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	Created  int64  `json:"created,omitempty"`
	OwnedBy  string `json:"owned_by"`
	Provider string `json:"provider,omitempty"` // Custom field for routing
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error APIError `json:"error"`
}

// APIError represents the error details
type APIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
}
