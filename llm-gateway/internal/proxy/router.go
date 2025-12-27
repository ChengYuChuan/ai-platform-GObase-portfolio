package proxy

import (
	"fmt"

	"github.com/username/llm-gateway/internal/config"
	"github.com/username/llm-gateway/internal/proxy/providers"
	"github.com/username/llm-gateway/pkg/models"
)

// Provider is an alias to the providers.Provider interface for external access
type Provider = providers.Provider

// ProviderError is an alias to providers.ProviderError for external access
type ProviderError = providers.ProviderError

// Router handles routing requests to the appropriate provider
type Router struct {
	registry        *providers.Registry
	config          *config.Config
	defaultProvider string
}

// NewRouter creates a new proxy router
func NewRouter(registry *providers.Registry, cfg *config.Config) *Router {
	return &Router{
		registry:        registry,
		config:          cfg,
		defaultProvider: cfg.Providers.Default,
	}
}

// GetProviderForModel returns the appropriate provider for a given model
func (r *Router) GetProviderForModel(model string) (Provider, error) {
	// First, try to find a provider that explicitly supports this model
	provider, found := r.registry.GetForModel(model)
	if found {
		return provider, nil
	}

	// If no specific provider found, use the default
	if r.defaultProvider != "" {
		provider, found := r.registry.Get(r.defaultProvider)
		if found {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("no provider found for model: %s", model)
}

// GetProvider returns a specific provider by name
func (r *Router) GetProvider(name string) (Provider, error) {
	provider, found := r.registry.Get(name)
	if !found {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return provider, nil
}

// AvailableProviders returns a list of available provider names
func (r *Router) AvailableProviders() []string {
	return r.registry.List()
}

// ListModels returns all available models from all providers
func (r *Router) ListModels() []models.Model {
	return r.registry.ListAllModels()
}
