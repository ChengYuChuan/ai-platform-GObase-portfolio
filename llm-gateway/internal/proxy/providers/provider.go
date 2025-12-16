package providers

import (
	"context"
	"io"
	"sync"

	"github.com/username/llm-gateway/pkg/models"
)

// Provider defines the interface that all LLM providers must implement
type Provider interface {
	// Name returns the provider name (e.g., "openai", "anthropic")
	Name() string

	// ChatCompletion performs a non-streaming chat completion
	ChatCompletion(ctx context.Context, req *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error)

	// ChatCompletionStream performs a streaming chat completion
	// Returns a ReadCloser that streams SSE-formatted data
	ChatCompletionStream(ctx context.Context, req *models.ChatCompletionRequest) (io.ReadCloser, error)

	// Completion performs a legacy completion (text-davinci style)
	Completion(ctx context.Context, req *models.CompletionRequest) (*models.CompletionResponse, error)

	// Embedding generates embeddings for the input
	Embedding(ctx context.Context, req *models.EmbeddingRequest) (*models.EmbeddingResponse, error)

	// ListModels returns the list of models supported by this provider
	ListModels() []models.Model

	// SupportsModel checks if this provider supports the given model
	SupportsModel(model string) bool

	// HealthCheck verifies the provider is accessible
	HealthCheck(ctx context.Context) error
}

// Registry manages provider registration and lookup
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(name string, provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = provider
}

// Get retrieves a provider by name
func (r *Registry) Get(name string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[name]
	return provider, ok
}

// GetForModel finds a provider that supports the given model
func (r *Registry) GetForModel(model string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, provider := range r.providers {
		if provider.SupportsModel(model) {
			return provider, true
		}
	}
	return nil, false
}

// List returns all registered provider names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// ListAllModels returns models from all registered providers
func (r *Registry) ListAllModels() []models.Model {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var allModels []models.Model
	for _, provider := range r.providers {
		allModels = append(allModels, provider.ListModels()...)
	}
	return allModels
}

// HealthCheckAll checks all providers and returns their status
func (r *Registry) HealthCheckAll(ctx context.Context) map[string]error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make(map[string]error)
	var wg sync.WaitGroup

	for name, provider := range r.providers {
		wg.Add(1)
		go func(n string, p Provider) {
			defer wg.Done()
			results[n] = p.HealthCheck(ctx)
		}(name, provider)
	}

	wg.Wait()
	return results
}
