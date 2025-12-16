package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/username/llm-gateway/internal/proxy"
	"github.com/username/llm-gateway/pkg/models"
)

// AnthropicConfig holds configuration for the Anthropic provider
type AnthropicConfig struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
	Version string // API version (e.g., "2023-06-01")
}

// AnthropicProvider implements the Provider interface for Anthropic
type AnthropicProvider struct {
	config     AnthropicConfig
	httpClient *http.Client
	models     []models.Model
}

// Anthropic model prefixes for routing
var anthropicModelPrefixes = []string{
	"claude-",
}

// Supported Anthropic models
var anthropicModels = []models.Model{
	{ID: "claude-sonnet-4-20250514", Object: "model", OwnedBy: "anthropic", Provider: "anthropic"},
	{ID: "claude-3-5-sonnet-20241022", Object: "model", OwnedBy: "anthropic", Provider: "anthropic"},
	{ID: "claude-3-5-haiku-20241022", Object: "model", OwnedBy: "anthropic", Provider: "anthropic"},
	{ID: "claude-3-opus-20240229", Object: "model", OwnedBy: "anthropic", Provider: "anthropic"},
	{ID: "claude-3-sonnet-20240229", Object: "model", OwnedBy: "anthropic", Provider: "anthropic"},
	{ID: "claude-3-haiku-20240307", Object: "model", OwnedBy: "anthropic", Provider: "anthropic"},
}

// NewAnthropicProvider creates a new Anthropic provider instance
func NewAnthropicProvider(config AnthropicConfig) *AnthropicProvider {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.anthropic.com"
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}
	if config.Version == "" {
		config.Version = "2023-06-01"
	}

	return &AnthropicProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		models: anthropicModels,
	}
}

// Name returns the provider name
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// anthropicRequest represents the Anthropic API request format
type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
	TopP        *float64           `json:"top_p,omitempty"`
	TopK        *int               `json:"top_k,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
	StopSeq     []string           `json:"stop_sequences,omitempty"`
}

// anthropicMessage represents a message in Anthropic format
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse represents the Anthropic API response format
type anthropicResponse struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Role         string `json:"role"`
	Content      []anthropicContent `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence,omitempty"`
	Usage        anthropicUsage `json:"usage"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ChatCompletion performs a non-streaming chat completion
func (p *AnthropicProvider) ChatCompletion(ctx context.Context, req *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	anthropicReq := p.convertToAnthropicRequest(req)
	anthropicReq.Stream = false

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.handleErrorResponse(resp)
	}

	var anthropicResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return p.convertToOpenAIResponse(&anthropicResp, req.Model), nil
}

// ChatCompletionStream performs a streaming chat completion
func (p *AnthropicProvider) ChatCompletionStream(ctx context.Context, req *models.ChatCompletionRequest) (io.ReadCloser, error) {
	anthropicReq := p.convertToAnthropicRequest(req)
	anthropicReq.Stream = true

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(httpReq)

	streamClient := &http.Client{}

	resp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, p.handleErrorResponse(resp)
	}

	// Return a wrapper that converts Anthropic SSE format to OpenAI format
	return &anthropicStreamConverter{
		reader: resp.Body,
		model:  req.Model,
	}, nil
}

// Completion performs a legacy completion (converted to chat format)
func (p *AnthropicProvider) Completion(ctx context.Context, req *models.CompletionRequest) (*models.CompletionResponse, error) {
	// Convert completion request to chat format
	chatReq := &models.ChatCompletionRequest{
		Model: req.Model,
		Messages: []models.ChatMessage{
			{Role: "user", Content: req.Prompt},
		},
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.Stop,
	}

	chatResp, err := p.ChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, err
	}

	// Convert chat response to completion format
	text := ""
	if len(chatResp.Choices) > 0 {
		text = chatResp.Choices[0].Message.Content
	}

	return &models.CompletionResponse{
		ID:      chatResp.ID,
		Object:  "text_completion",
		Created: chatResp.Created,
		Model:   chatResp.Model,
		Choices: []models.CompletionChoice{
			{
				Text:         text,
				Index:        0,
				FinishReason: chatResp.Choices[0].FinishReason,
			},
		},
		Usage: chatResp.Usage,
	}, nil
}

// Embedding returns an error as Anthropic doesn't support embeddings
func (p *AnthropicProvider) Embedding(ctx context.Context, req *models.EmbeddingRequest) (*models.EmbeddingResponse, error) {
	return nil, &proxy.ProviderError{
		Provider:   "anthropic",
		StatusCode: http.StatusNotImplemented,
		Code:       "not_supported",
		Message:    "Anthropic does not support embeddings API",
	}
}

// ListModels returns supported models
func (p *AnthropicProvider) ListModels() []models.Model {
	return p.models
}

// SupportsModel checks if this provider supports the given model
func (p *AnthropicProvider) SupportsModel(model string) bool {
	modelLower := strings.ToLower(model)
	for _, prefix := range anthropicModelPrefixes {
		if strings.HasPrefix(modelLower, prefix) {
			return true
		}
	}
	for _, m := range p.models {
		if strings.EqualFold(m.ID, model) {
			return true
		}
	}
	return false
}

// HealthCheck verifies the provider is accessible
func (p *AnthropicProvider) HealthCheck(ctx context.Context) error {
	// Anthropic doesn't have a dedicated health endpoint, so we make a minimal request
	req := &models.ChatCompletionRequest{
		Model: "claude-3-haiku-20240307",
		Messages: []models.ChatMessage{
			{Role: "user", Content: "hi"},
		},
		MaxTokens: 1,
	}

	_, err := p.ChatCompletion(ctx, req)
	return err
}

// setHeaders sets common headers for Anthropic API requests
func (p *AnthropicProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.config.APIKey)
	req.Header.Set("anthropic-version", p.config.Version)
}

// convertToAnthropicRequest converts OpenAI-style request to Anthropic format
func (p *AnthropicProvider) convertToAnthropicRequest(req *models.ChatCompletionRequest) *anthropicRequest {
	var messages []anthropicMessage
	var systemPrompt string

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			systemPrompt = msg.Content
			continue
		}
		messages = append(messages, anthropicMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096 // Default for Anthropic
	}

	return &anthropicRequest{
		Model:       req.Model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		System:      systemPrompt,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		StopSeq:     req.Stop,
	}
}

// convertToOpenAIResponse converts Anthropic response to OpenAI format
func (p *AnthropicProvider) convertToOpenAIResponse(resp *anthropicResponse, model string) *models.ChatCompletionResponse {
	content := ""
	for _, c := range resp.Content {
		if c.Type == "text" {
			content += c.Text
		}
	}

	finishReason := "stop"
	switch resp.StopReason {
	case "end_turn":
		finishReason = "stop"
	case "max_tokens":
		finishReason = "length"
	case "stop_sequence":
		finishReason = "stop"
	}

	return &models.ChatCompletionResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []models.ChatCompletionChoice{
			{
				Index: 0,
				Message: models.ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
		Usage: models.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}

// handleErrorResponse parses an error response from Anthropic
func (p *AnthropicProvider) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	log.Error().
		Int("status", resp.StatusCode).
		Str("body", string(body)).
		Msg("Anthropic API error")

	var errResp struct {
		Type  string `json:"type"`
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		return &proxy.ProviderError{
			Provider:   "anthropic",
			StatusCode: resp.StatusCode,
			Code:       errResp.Error.Type,
			Message:    errResp.Error.Message,
		}
	}

	return &proxy.ProviderError{
		Provider:   "anthropic",
		StatusCode: resp.StatusCode,
		Code:       "api_error",
		Message:    fmt.Sprintf("Anthropic API returned status %d", resp.StatusCode),
	}
}

// anthropicStreamConverter converts Anthropic SSE stream to OpenAI format
type anthropicStreamConverter struct {
	reader io.ReadCloser
	model  string
	buffer []byte
}

func (c *anthropicStreamConverter) Read(p []byte) (n int, err error) {
	// For simplicity, we pass through the Anthropic stream
	// In a production implementation, you'd convert each event to OpenAI format
	return c.reader.Read(p)
}

func (c *anthropicStreamConverter) Close() error {
	return c.reader.Close()
}

// generateID creates a unique ID for responses
func generateID() string {
	return "chatcmpl-" + uuid.New().String()[:8]
}
