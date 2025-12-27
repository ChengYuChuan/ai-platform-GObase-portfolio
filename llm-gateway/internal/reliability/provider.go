package reliability

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/username/llm-gateway/pkg/models"
	"github.com/username/llm-gateway/internal/proxy/providers"
)

// ResilientProviderConfig holds configuration for resilient provider wrapper
type ResilientProviderConfig struct {
	// Circuit breaker settings
	CircuitBreaker CircuitBreakerConfig
	// Retry settings
	Retry RetryConfig
	// Request timeout (overrides provider default if set)
	RequestTimeout time.Duration
}

// DefaultResilientProviderConfig returns sensible defaults
func DefaultResilientProviderConfig(providerName string) ResilientProviderConfig {
	return ResilientProviderConfig{
		CircuitBreaker: DefaultCircuitBreakerConfig(providerName),
		Retry:          DefaultRetryConfig(),
		RequestTimeout: 60 * time.Second,
	}
}

// ResilientProvider wraps a provider with circuit breaker and retry logic
type ResilientProvider struct {
	provider       providers.Provider
	circuitBreaker *CircuitBreaker
	retryer        *Retryer
	config         ResilientProviderConfig
}

// NewResilientProvider creates a new resilient provider wrapper
func NewResilientProvider(provider providers.Provider, config ResilientProviderConfig) *ResilientProvider {
	return &ResilientProvider{
		provider:       provider,
		circuitBreaker: NewCircuitBreaker(config.CircuitBreaker),
		retryer:        NewRetryer(config.Retry),
		config:         config,
	}
}

// Name returns the provider name
func (rp *ResilientProvider) Name() string {
	return rp.provider.Name()
}

// ChatCompletion performs a resilient chat completion with circuit breaker and retry
func (rp *ResilientProvider) ChatCompletion(ctx context.Context, req *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	operation := fmt.Sprintf("%s:chat_completion", rp.provider.Name())

	var result *models.ChatCompletionResponse

	err := rp.circuitBreaker.Execute(func() error {
		res, retryResult := rp.retryer.ExecuteFunc(ctx, operation, func() (interface{}, error) {
			resp, err := rp.provider.ChatCompletion(ctx, req)
			if err != nil {
				return nil, rp.wrapError(err)
			}
			return resp, nil
		})

		if !retryResult.Successful {
			return retryResult.LastError
		}

		if res != nil {
			result = res.(*models.ChatCompletionResponse)
		}
		return nil
	})

	if err != nil {
		return nil, rp.unwrapError(err)
	}

	return result, nil
}

// ChatCompletionStream performs streaming chat completion
// Note: Streaming has limited retry capability - we can only retry before the stream starts
func (rp *ResilientProvider) ChatCompletionStream(ctx context.Context, req *models.ChatCompletionRequest) (io.ReadCloser, error) {
	operation := fmt.Sprintf("%s:chat_completion_stream", rp.provider.Name())

	var result io.ReadCloser

	err := rp.circuitBreaker.Execute(func() error {
		res, retryResult := rp.retryer.ExecuteFunc(ctx, operation, func() (interface{}, error) {
			stream, err := rp.provider.ChatCompletionStream(ctx, req)
			if err != nil {
				return nil, rp.wrapError(err)
			}
			return stream, nil
		})

		if !retryResult.Successful {
			return retryResult.LastError
		}

		if res != nil {
			result = res.(io.ReadCloser)
		}
		return nil
	})

	if err != nil {
		return nil, rp.unwrapError(err)
	}

	return result, nil
}

// Completion performs a resilient legacy completion
func (rp *ResilientProvider) Completion(ctx context.Context, req *models.CompletionRequest) (*models.CompletionResponse, error) {
	operation := fmt.Sprintf("%s:completion", rp.provider.Name())

	var result *models.CompletionResponse

	err := rp.circuitBreaker.Execute(func() error {
		res, retryResult := rp.retryer.ExecuteFunc(ctx, operation, func() (interface{}, error) {
			resp, err := rp.provider.Completion(ctx, req)
			if err != nil {
				return nil, rp.wrapError(err)
			}
			return resp, nil
		})

		if !retryResult.Successful {
			return retryResult.LastError
		}

		if res != nil {
			result = res.(*models.CompletionResponse)
		}
		return nil
	})

	if err != nil {
		return nil, rp.unwrapError(err)
	}

	return result, nil
}

// Embedding performs resilient embedding generation
func (rp *ResilientProvider) Embedding(ctx context.Context, req *models.EmbeddingRequest) (*models.EmbeddingResponse, error) {
	operation := fmt.Sprintf("%s:embedding", rp.provider.Name())

	var result *models.EmbeddingResponse

	err := rp.circuitBreaker.Execute(func() error {
		res, retryResult := rp.retryer.ExecuteFunc(ctx, operation, func() (interface{}, error) {
			resp, err := rp.provider.Embedding(ctx, req)
			if err != nil {
				return nil, rp.wrapError(err)
			}
			return resp, nil
		})

		if !retryResult.Successful {
			return retryResult.LastError
		}

		if res != nil {
			result = res.(*models.EmbeddingResponse)
		}
		return nil
	})

	if err != nil {
		return nil, rp.unwrapError(err)
	}

	return result, nil
}

// ListModels returns supported models (no retry needed - cached locally)
func (rp *ResilientProvider) ListModels() []models.Model {
	return rp.provider.ListModels()
}

// SupportsModel checks if this provider supports the given model
func (rp *ResilientProvider) SupportsModel(model string) bool {
	return rp.provider.SupportsModel(model)
}

// HealthCheck performs a health check with circuit breaker awareness
func (rp *ResilientProvider) HealthCheck(ctx context.Context) error {
	// Don't use circuit breaker for health checks - they're used to determine circuit state
	return rp.provider.HealthCheck(ctx)
}

// CircuitState returns the current circuit breaker state
func (rp *ResilientProvider) CircuitState() CircuitState {
	return rp.circuitBreaker.State()
}

// Stats returns reliability statistics for this provider
func (rp *ResilientProvider) Stats() map[string]interface{} {
	return map[string]interface{}{
		"provider":        rp.provider.Name(),
		"circuit_breaker": rp.circuitBreaker.Stats(),
	}
}

// ResetCircuitBreaker resets the circuit breaker to closed state
func (rp *ResilientProvider) ResetCircuitBreaker() {
	rp.circuitBreaker.Reset()
}

// wrapError wraps provider errors for retry logic
func (rp *ResilientProvider) wrapError(err error) error {
	if err == nil {
		return nil
	}

	// Check if it's already a provider error
	if providerErr, ok := err.(*providers.ProviderError); ok {
		retryable := rp.isRetryableStatusCode(providerErr.StatusCode)
		return NewRetryableError(err, providerErr.StatusCode, retryable)
	}

	// For other errors, assume retryable (network issues, etc.)
	return NewRetryableError(err, 0, true)
}

// unwrapError converts internal errors back to provider errors
func (rp *ResilientProvider) unwrapError(err error) error {
	if err == nil {
		return nil
	}

	// Handle circuit breaker errors
	if err == ErrCircuitOpen {
		return &providers.ProviderError{
			Provider:   rp.provider.Name(),
			StatusCode: http.StatusServiceUnavailable,
			Code:       "circuit_open",
			Message:    fmt.Sprintf("Provider %s is temporarily unavailable (circuit breaker open)", rp.provider.Name()),
		}
	}

	if err == ErrTooManyRequests {
		return &providers.ProviderError{
			Provider:   rp.provider.Name(),
			StatusCode: http.StatusServiceUnavailable,
			Code:       "circuit_half_open",
			Message:    fmt.Sprintf("Provider %s is recovering, please retry shortly", rp.provider.Name()),
		}
	}

	// Unwrap retryable errors
	if retryableErr, ok := err.(*RetryableError); ok {
		return retryableErr.Unwrap()
	}

	return err
}

// isRetryableStatusCode checks if a status code should trigger a retry
func (rp *ResilientProvider) isRetryableStatusCode(statusCode int) bool {
	retryableCodes := []int{
		http.StatusTooManyRequests,     // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout,      // 504
	}

	for _, code := range retryableCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// ResilientRegistry wraps all providers with resilience features
type ResilientRegistry struct {
	providers map[string]*ResilientProvider
}

// NewResilientRegistry creates resilient wrappers for all providers in a registry
func NewResilientRegistry(registry *providers.Registry) *ResilientRegistry {
	rr := &ResilientRegistry{
		providers: make(map[string]*ResilientProvider),
	}

	for _, name := range registry.List() {
		provider, _ := registry.Get(name)
		config := DefaultResilientProviderConfig(name)
		rr.providers[name] = NewResilientProvider(provider, config)

		log.Info().
			Str("provider", name).
			Msg("Wrapped provider with resilience features")
	}

	return rr
}

// Get returns a resilient provider by name
func (rr *ResilientRegistry) Get(name string) (*ResilientProvider, bool) {
	provider, ok := rr.providers[name]
	return provider, ok
}

// AllStats returns stats for all providers
func (rr *ResilientRegistry) AllStats() map[string]interface{} {
	stats := make(map[string]interface{})
	for name, provider := range rr.providers {
		stats[name] = provider.Stats()
	}
	return stats
}
