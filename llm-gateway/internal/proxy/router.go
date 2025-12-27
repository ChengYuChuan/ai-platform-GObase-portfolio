package proxy

import (
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/username/llm-gateway/internal/config"
	"github.com/username/llm-gateway/internal/proxy/providers"
	"github.com/username/llm-gateway/internal/reliability"
	"github.com/username/llm-gateway/pkg/models"
)

// Provider is an alias to the providers.Provider interface for external access
type Provider = providers.Provider

// ProviderError is an alias to providers.ProviderError for external access
type ProviderError = providers.ProviderError

// Router handles routing requests to the appropriate provider
type Router struct {
	registry          *providers.Registry
	resilientRegistry map[string]*reliability.ResilientProvider
	config            *config.Config
	defaultProvider   string
	reliabilityEnabled bool
}

// NewRouter creates a new proxy router
func NewRouter(registry *providers.Registry, cfg *config.Config) *Router {
	r := &Router{
		registry:          registry,
		resilientRegistry: make(map[string]*reliability.ResilientProvider),
		config:            cfg,
		defaultProvider:   cfg.Providers.Default,
		reliabilityEnabled: cfg.Reliability.CircuitBreaker.Enabled || cfg.Reliability.Retry.Enabled,
	}

	// Wrap providers with resilience features if enabled
	if r.reliabilityEnabled {
		r.initResilientProviders()
	}

	return r
}

// initResilientProviders wraps all providers with resilience features
func (r *Router) initResilientProviders() {
	for _, name := range r.registry.List() {
		provider, _ := r.registry.Get(name)

		// Build config from settings
		resConfig := reliability.ResilientProviderConfig{
			CircuitBreaker: reliability.CircuitBreakerConfig{
				Name:                name,
				FailureThreshold:    r.config.Reliability.CircuitBreaker.FailureThreshold,
				SuccessThreshold:    r.config.Reliability.CircuitBreaker.SuccessThreshold,
				Timeout:             r.config.Reliability.CircuitBreaker.Timeout,
				MaxHalfOpenRequests: r.config.Reliability.CircuitBreaker.MaxHalfOpenRequests,
			},
			Retry: reliability.RetryConfig{
				MaxRetries:        r.config.Reliability.Retry.MaxRetries,
				InitialBackoff:    r.config.Reliability.Retry.InitialBackoff,
				MaxBackoff:        r.config.Reliability.Retry.MaxBackoff,
				BackoffMultiplier: r.config.Reliability.Retry.BackoffMultiplier,
				JitterFactor:      0.2, // Default jitter
				RetryableStatusCodes: []int{429, 500, 502, 503, 504},
			},
			RequestTimeout: 60 * time.Second,
		}

		r.resilientRegistry[name] = reliability.NewResilientProvider(provider, resConfig)

		log.Info().
			Str("provider", name).
			Bool("circuit_breaker", r.config.Reliability.CircuitBreaker.Enabled).
			Bool("retry", r.config.Reliability.Retry.Enabled).
			Msg("Provider wrapped with resilience features")
	}
}

// GetProviderForModel returns the appropriate provider for a given model
func (r *Router) GetProviderForModel(model string) (Provider, error) {
	// First, try to find a provider that explicitly supports this model
	provider, found := r.registry.GetForModel(model)
	if found {
		// Return resilient wrapper if available
		if r.reliabilityEnabled {
			if resilient, ok := r.resilientRegistry[provider.Name()]; ok {
				return resilient, nil
			}
		}
		return provider, nil
	}

	// If no specific provider found, use the default
	if r.defaultProvider != "" {
		provider, found := r.registry.Get(r.defaultProvider)
		if found {
			// Return resilient wrapper if available
			if r.reliabilityEnabled {
				if resilient, ok := r.resilientRegistry[provider.Name()]; ok {
					return resilient, nil
				}
			}
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

	// Return resilient wrapper if available
	if r.reliabilityEnabled {
		if resilient, ok := r.resilientRegistry[name]; ok {
			return resilient, nil
		}
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

// GetReliabilityStats returns stats for all resilient providers
func (r *Router) GetReliabilityStats() map[string]interface{} {
	stats := make(map[string]interface{})
	for name, provider := range r.resilientRegistry {
		stats[name] = provider.Stats()
	}
	return stats
}

// IsReliabilityEnabled returns whether reliability features are enabled
func (r *Router) IsReliabilityEnabled() bool {
	return r.reliabilityEnabled
}
