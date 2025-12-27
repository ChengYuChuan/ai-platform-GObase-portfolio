package providers

import (
	"bufio"
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

	"github.com/username/llm-gateway/pkg/models"
)

// OllamaProviderConfig holds configuration for the Ollama provider
type OllamaProviderConfig struct {
	BaseURL string
	Timeout time.Duration
}

// OllamaProvider implements the Provider interface for Ollama
type OllamaProvider struct {
	config     OllamaProviderConfig
	httpClient *http.Client
	models     []models.Model
}

// Ollama model prefixes for routing
var ollamaModelPrefixes = []string{
	"llama",
	"mistral",
	"mixtral",
	"codellama",
	"phi",
	"qwen",
	"gemma",
	"deepseek",
	"starcoder",
	"wizard",
	"neural-chat",
	"orca",
	"vicuna",
	"nous",
	"dolphin",
	"yi",
}

// Default Ollama models (commonly available)
var defaultOllamaModels = []models.Model{
	{ID: "llama3.2", Object: "model", OwnedBy: "ollama", Provider: "ollama"},
	{ID: "llama3.1", Object: "model", OwnedBy: "ollama", Provider: "ollama"},
	{ID: "llama3", Object: "model", OwnedBy: "ollama", Provider: "ollama"},
	{ID: "mistral", Object: "model", OwnedBy: "ollama", Provider: "ollama"},
	{ID: "mixtral", Object: "model", OwnedBy: "ollama", Provider: "ollama"},
	{ID: "codellama", Object: "model", OwnedBy: "ollama", Provider: "ollama"},
	{ID: "phi3", Object: "model", OwnedBy: "ollama", Provider: "ollama"},
	{ID: "qwen2", Object: "model", OwnedBy: "ollama", Provider: "ollama"},
	{ID: "gemma2", Object: "model", OwnedBy: "ollama", Provider: "ollama"},
}

// Ollama API request/response types
type ollamaChatRequest struct {
	Model    string                `json:"model"`
	Messages []ollamaChatMessage   `json:"messages"`
	Stream   bool                  `json:"stream"`
	Options  *ollamaOptions        `json:"options,omitempty"`
}

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"top_p,omitempty"`
	TopK        *int     `json:"top_k,omitempty"`
	NumPredict  int      `json:"num_predict,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

type ollamaChatResponse struct {
	Model              string            `json:"model"`
	CreatedAt          string            `json:"created_at"`
	Message            ollamaChatMessage `json:"message"`
	Done               bool              `json:"done"`
	TotalDuration      int64             `json:"total_duration,omitempty"`
	LoadDuration       int64             `json:"load_duration,omitempty"`
	PromptEvalCount    int               `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64             `json:"prompt_eval_duration,omitempty"`
	EvalCount          int               `json:"eval_count,omitempty"`
	EvalDuration       int64             `json:"eval_duration,omitempty"`
}

type ollamaGenerateRequest struct {
	Model   string         `json:"model"`
	Prompt  string         `json:"prompt"`
	Stream  bool           `json:"stream"`
	Options *ollamaOptions `json:"options,omitempty"`
}

type ollamaGenerateResponse struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"`
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	TotalDuration      int64  `json:"total_duration,omitempty"`
	PromptEvalCount    int    `json:"prompt_eval_count,omitempty"`
	EvalCount          int    `json:"eval_count,omitempty"`
}

type ollamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

type ollamaTagsResponse struct {
	Models []ollamaModelInfo `json:"models"`
}

type ollamaModelInfo struct {
	Name       string `json:"name"`
	ModifiedAt string `json:"modified_at"`
	Size       int64  `json:"size"`
}

// NewOllamaProvider creates a new Ollama provider instance
func NewOllamaProvider(config OllamaProviderConfig) *OllamaProvider {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:11434"
	}
	if config.Timeout == 0 {
		config.Timeout = 120 * time.Second // Longer timeout for local inference
	}

	return &OllamaProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		models: defaultOllamaModels,
	}
}

// Name returns the provider name
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// ChatCompletion performs a non-streaming chat completion
func (p *OllamaProvider) ChatCompletion(ctx context.Context, req *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	// Convert to Ollama format
	ollamaReq := p.convertToOllamaRequest(req)
	ollamaReq.Stream = false

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.handleErrorResponse(resp)
	}

	var ollamaResp ollamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to OpenAI format
	return p.convertToOpenAIResponse(&ollamaResp, req.Model), nil
}

// ChatCompletionStream performs a streaming chat completion
func (p *OllamaProvider) ChatCompletionStream(ctx context.Context, req *models.ChatCompletionRequest) (io.ReadCloser, error) {
	// Convert to Ollama format
	ollamaReq := p.convertToOllamaRequest(req)
	ollamaReq.Stream = true

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Use client without timeout for streaming
	streamClient := &http.Client{}

	resp, err := streamClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, p.handleErrorResponse(resp)
	}

	// Create a pipe to convert NDJSON to SSE format
	pr, pw := io.Pipe()

	go p.convertStreamToSSE(resp.Body, pw, req.Model)

	return pr, nil
}

// convertStreamToSSE converts Ollama NDJSON stream to OpenAI SSE format
func (p *OllamaProvider) convertStreamToSSE(src io.ReadCloser, dst *io.PipeWriter, model string) {
	defer src.Close()
	defer dst.Close()

	scanner := bufio.NewScanner(src)
	requestID := "chatcmpl-" + uuid.New().String()[:8]
	created := time.Now().Unix()

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var ollamaResp ollamaChatResponse
		if err := json.Unmarshal([]byte(line), &ollamaResp); err != nil {
			log.Error().Err(err).Str("line", line).Msg("Failed to parse Ollama stream response")
			continue
		}

		// Convert to OpenAI stream format
		streamResp := models.ChatCompletionStreamResponse{
			ID:      requestID,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   model,
			Choices: []models.ChatCompletionStreamChoice{
				{
					Index: 0,
					Delta: models.ChatMessageDelta{
						Content: ollamaResp.Message.Content,
					},
				},
			},
		}

		// Set role on first chunk
		if ollamaResp.Message.Role != "" && ollamaResp.Message.Content == "" {
			streamResp.Choices[0].Delta.Role = ollamaResp.Message.Role
		}

		// Set finish reason on last chunk
		if ollamaResp.Done {
			finishReason := "stop"
			streamResp.Choices[0].FinishReason = &finishReason
		}

		// Write SSE format
		jsonData, err := json.Marshal(streamResp)
		if err != nil {
			log.Error().Err(err).Msg("Failed to marshal stream response")
			continue
		}

		if _, err := fmt.Fprintf(dst, "data: %s\n\n", jsonData); err != nil {
			log.Error().Err(err).Msg("Failed to write to stream")
			return
		}

		// Send [DONE] after final message
		if ollamaResp.Done {
			if _, err := fmt.Fprintf(dst, "data: [DONE]\n\n"); err != nil {
				log.Error().Err(err).Msg("Failed to write DONE to stream")
			}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Error().Err(err).Msg("Scanner error in stream conversion")
	}
}

// Completion performs a legacy completion
func (p *OllamaProvider) Completion(ctx context.Context, req *models.CompletionRequest) (*models.CompletionResponse, error) {
	ollamaReq := ollamaGenerateRequest{
		Model:  req.Model,
		Prompt: req.Prompt,
		Stream: false,
		Options: &ollamaOptions{
			Temperature: req.Temperature,
			TopP:        req.TopP,
			NumPredict:  req.MaxTokens,
			Stop:        req.Stop,
		},
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, p.handleErrorResponse(resp)
	}

	var ollamaResp ollamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to OpenAI format
	return &models.CompletionResponse{
		ID:      "cmpl-" + uuid.New().String()[:8],
		Object:  "text_completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []models.CompletionChoice{
			{
				Text:         ollamaResp.Response,
				Index:        0,
				FinishReason: "stop",
			},
		},
		Usage: models.Usage{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
	}, nil
}

// Embedding generates embeddings
func (p *OllamaProvider) Embedding(ctx context.Context, req *models.EmbeddingRequest) (*models.EmbeddingResponse, error) {
	// Handle input as string or []string
	var inputs []string
	switch v := req.Input.(type) {
	case string:
		inputs = []string{v}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				inputs = append(inputs, s)
			}
		}
	case []string:
		inputs = v
	default:
		return nil, fmt.Errorf("invalid input type")
	}

	var embeddings []models.EmbeddingData
	var totalTokens int

	for i, input := range inputs {
		ollamaReq := ollamaEmbeddingRequest{
			Model:  req.Model,
			Prompt: input,
		}

		body, err := json.Marshal(ollamaReq)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/api/embeddings", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := p.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, p.handleErrorResponse(resp)
		}

		var ollamaResp ollamaEmbeddingResponse
		if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		embeddings = append(embeddings, models.EmbeddingData{
			Object:    "embedding",
			Embedding: ollamaResp.Embedding,
			Index:     i,
		})

		// Estimate tokens (rough approximation)
		totalTokens += len(strings.Fields(input))
	}

	return &models.EmbeddingResponse{
		Object: "list",
		Data:   embeddings,
		Model:  req.Model,
		Usage: models.EmbeddingUsage{
			PromptTokens: totalTokens,
			TotalTokens:  totalTokens,
		},
	}, nil
}

// ListModels returns supported models
func (p *OllamaProvider) ListModels() []models.Model {
	// Try to fetch actual models from Ollama
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "GET", p.config.BaseURL+"/api/tags", nil)
	if err != nil {
		return p.models
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return p.models
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p.models
	}

	var tagsResp ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return p.models
	}

	// Convert to our model format
	ollamaModels := make([]models.Model, len(tagsResp.Models))
	for i, m := range tagsResp.Models {
		ollamaModels[i] = models.Model{
			ID:       m.Name,
			Object:   "model",
			OwnedBy:  "ollama",
			Provider: "ollama",
		}
	}

	if len(ollamaModels) > 0 {
		return ollamaModels
	}

	return p.models
}

// SupportsModel checks if this provider supports the given model
func (p *OllamaProvider) SupportsModel(model string) bool {
	modelLower := strings.ToLower(model)
	for _, prefix := range ollamaModelPrefixes {
		if strings.HasPrefix(modelLower, prefix) {
			return true
		}
	}
	// Also check available models
	for _, m := range p.ListModels() {
		if strings.EqualFold(m.ID, model) {
			return true
		}
	}
	return false
}

// HealthCheck verifies the provider is accessible
func (p *OllamaProvider) HealthCheck(ctx context.Context) error {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", p.config.BaseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

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

// convertToOllamaRequest converts OpenAI request to Ollama format
func (p *OllamaProvider) convertToOllamaRequest(req *models.ChatCompletionRequest) *ollamaChatRequest {
	messages := make([]ollamaChatMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = ollamaChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	ollamaReq := &ollamaChatRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   req.Stream,
	}

	// Set options if any are specified
	if req.Temperature != nil || req.TopP != nil || req.MaxTokens > 0 || len(req.Stop) > 0 {
		ollamaReq.Options = &ollamaOptions{
			Temperature: req.Temperature,
			TopP:        req.TopP,
			NumPredict:  req.MaxTokens,
			Stop:        req.Stop,
		}
	}

	return ollamaReq
}

// convertToOpenAIResponse converts Ollama response to OpenAI format
func (p *OllamaProvider) convertToOpenAIResponse(resp *ollamaChatResponse, model string) *models.ChatCompletionResponse {
	return &models.ChatCompletionResponse{
		ID:      "chatcmpl-" + uuid.New().String()[:8],
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []models.ChatCompletionChoice{
			{
				Index: 0,
				Message: models.ChatMessage{
					Role:    resp.Message.Role,
					Content: resp.Message.Content,
				},
				FinishReason: "stop",
			},
		},
		Usage: models.Usage{
			PromptTokens:     resp.PromptEvalCount,
			CompletionTokens: resp.EvalCount,
			TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
		},
	}
}

// handleErrorResponse parses an error response from Ollama
func (p *OllamaProvider) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	log.Error().
		Int("status", resp.StatusCode).
		Str("body", string(body)).
		Msg("Ollama API error")

	// Try to parse Ollama error format
	var errResp struct {
		Error string `json:"error"`
	}

	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
		return &ProviderError{
			Provider:   "ollama",
			StatusCode: resp.StatusCode,
			Code:       "ollama_error",
			Message:    errResp.Error,
		}
	}

	return &ProviderError{
		Provider:   "ollama",
		StatusCode: resp.StatusCode,
		Code:       "api_error",
		Message:    fmt.Sprintf("Ollama API returned status %d", resp.StatusCode),
	}
}
