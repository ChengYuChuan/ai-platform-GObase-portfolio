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

	"github.com/rs/zerolog/log"

	
	"github.com/username/llm-gateway/pkg/models"
)

// OpenAIConfig holds configuration for the OpenAI provider
type OpenAIConfig struct {
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

// OpenAIProvider implements the Provider interface for OpenAI
type OpenAIProvider struct {
	config     OpenAIConfig
	httpClient *http.Client
	models     []models.Model
}

// OpenAI model prefixes for routing
var openAIModelPrefixes = []string{
	"gpt-4",
	"gpt-3.5",
	"text-davinci",
	"text-embedding",
	"text-ada",
	"embedding",
}

// Supported OpenAI models
var openAIModels = []models.Model{
	{ID: "gpt-4o", Object: "model", OwnedBy: "openai", Provider: "openai"},
	{ID: "gpt-4o-mini", Object: "model", OwnedBy: "openai", Provider: "openai"},
	{ID: "gpt-4-turbo", Object: "model", OwnedBy: "openai", Provider: "openai"},
	{ID: "gpt-4", Object: "model", OwnedBy: "openai", Provider: "openai"},
	{ID: "gpt-3.5-turbo", Object: "model", OwnedBy: "openai", Provider: "openai"},
	{ID: "text-embedding-3-small", Object: "model", OwnedBy: "openai", Provider: "openai"},
	{ID: "text-embedding-3-large", Object: "model", OwnedBy: "openai", Provider: "openai"},
	{ID: "text-embedding-ada-002", Object: "model", OwnedBy: "openai", Provider: "openai"},
}

// NewOpenAIProvider creates a new OpenAI provider instance
func NewOpenAIProvider(config OpenAIConfig) *OpenAIProvider {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com/v1"
	}
	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}

	return &OpenAIProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		models: openAIModels,
	}
}

// Name returns the provider name
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// ChatCompletion performs a non-streaming chat completion
func (p *OpenAIProvider) ChatCompletion(ctx context.Context, req *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	// Ensure stream is false for sync request
	reqCopy := *req
	reqCopy.Stream = false

	body, err := json.Marshal(reqCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/chat/completions", bytes.NewReader(body))
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

	var result models.ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ChatCompletionStream performs a streaming chat completion
func (p *OpenAIProvider) ChatCompletionStream(ctx context.Context, req *models.ChatCompletionRequest) (io.ReadCloser, error) {
	// Ensure stream is true
	reqCopy := *req
	reqCopy.Stream = true

	body, err := json.Marshal(reqCopy)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(httpReq)

	// Use a client without timeout for streaming
	streamClient := &http.Client{
		// No timeout - streaming can be long
	}

	resp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, p.handleErrorResponse(resp)
	}

	return resp.Body, nil
}

// Completion performs a legacy completion
func (p *OpenAIProvider) Completion(ctx context.Context, req *models.CompletionRequest) (*models.CompletionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/completions", bytes.NewReader(body))
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

	var result models.CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// Embedding generates embeddings
func (p *OpenAIProvider) Embedding(ctx context.Context, req *models.EmbeddingRequest) (*models.EmbeddingResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/embeddings", bytes.NewReader(body))
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

	var result models.EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// ListModels returns supported models
func (p *OpenAIProvider) ListModels() []models.Model {
	return p.models
}

// SupportsModel checks if this provider supports the given model
func (p *OpenAIProvider) SupportsModel(model string) bool {
	modelLower := strings.ToLower(model)
	for _, prefix := range openAIModelPrefixes {
		if strings.HasPrefix(modelLower, prefix) {
			return true
		}
	}
	// Also check exact matches
	for _, m := range p.models {
		if strings.EqualFold(m.ID, model) {
			return true
		}
	}
	return false
}

// HealthCheck verifies the provider is accessible
func (p *OpenAIProvider) HealthCheck(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", p.config.BaseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// setHeaders sets common headers for OpenAI API requests
func (p *OpenAIProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
}

// handleErrorResponse parses an error response from OpenAI
func (p *OpenAIProvider) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	log.Error().
		Int("status", resp.StatusCode).
		Str("body", string(body)).
		Msg("OpenAI API error")

	// Try to parse OpenAI error format
	var errResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		return &ProviderError{
			Provider:   "openai",
			StatusCode: resp.StatusCode,
			Code:       errResp.Error.Code,
			Message:    errResp.Error.Message,
		}
	}

	return &ProviderError{
		Provider:   "openai",
		StatusCode: resp.StatusCode,
		Code:       "api_error",
		Message:    fmt.Sprintf("OpenAI API returned status %d", resp.StatusCode),
	}
}
