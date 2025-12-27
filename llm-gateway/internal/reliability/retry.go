package reliability

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// RetryConfig holds configuration for retry behavior
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int
	// InitialBackoff is the initial backoff duration
	InitialBackoff time.Duration
	// MaxBackoff is the maximum backoff duration
	MaxBackoff time.Duration
	// BackoffMultiplier is the multiplier for exponential backoff
	BackoffMultiplier float64
	// JitterFactor adds randomness to prevent thundering herd (0-1)
	JitterFactor float64
	// RetryableStatusCodes are HTTP status codes that should trigger a retry
	RetryableStatusCodes []int
}

// DefaultRetryConfig returns sensible defaults for LLM API calls
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        3,
		InitialBackoff:    500 * time.Millisecond,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		JitterFactor:      0.2,
		RetryableStatusCodes: []int{
			http.StatusTooManyRequests,     // 429
			http.StatusInternalServerError, // 500
			http.StatusBadGateway,          // 502
			http.StatusServiceUnavailable,  // 503
			http.StatusGatewayTimeout,      // 504
		},
	}
}

// Retryer handles retry logic with exponential backoff
type Retryer struct {
	config RetryConfig
}

// NewRetryer creates a new retryer with the given config
func NewRetryer(config RetryConfig) *Retryer {
	return &Retryer{config: config}
}

// RetryableError is an error that can be retried
type RetryableError struct {
	Err        error
	StatusCode int
	Retryable  bool
}

func (e *RetryableError) Error() string {
	return e.Err.Error()
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NewRetryableError creates a new retryable error
func NewRetryableError(err error, statusCode int, retryable bool) *RetryableError {
	return &RetryableError{
		Err:        err,
		StatusCode: statusCode,
		Retryable:  retryable,
	}
}

// RetryResult contains the result of a retry operation
type RetryResult struct {
	Attempts   int
	TotalTime  time.Duration
	LastError  error
	Successful bool
}

// Execute runs a function with retry logic
func (r *Retryer) Execute(ctx context.Context, operation string, fn func() error) RetryResult {
	result := RetryResult{}
	startTime := time.Now()

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		result.Attempts = attempt + 1

		// Check context before attempt
		if ctx.Err() != nil {
			result.LastError = ctx.Err()
			result.TotalTime = time.Since(startTime)
			return result
		}

		// Execute the operation
		err := fn()
		if err == nil {
			result.Successful = true
			result.TotalTime = time.Since(startTime)

			if attempt > 0 {
				log.Info().
					Str("operation", operation).
					Int("attempts", result.Attempts).
					Dur("total_time", result.TotalTime).
					Msg("Operation succeeded after retry")
			}
			return result
		}

		result.LastError = err

		// Check if error is retryable
		if !r.isRetryable(err) {
			result.TotalTime = time.Since(startTime)
			log.Debug().
				Str("operation", operation).
				Err(err).
				Msg("Error is not retryable, giving up")
			return result
		}

		// Don't wait after the last attempt
		if attempt >= r.config.MaxRetries {
			break
		}

		// Calculate backoff with jitter
		backoff := r.calculateBackoff(attempt)

		log.Warn().
			Str("operation", operation).
			Int("attempt", attempt+1).
			Int("max_retries", r.config.MaxRetries).
			Dur("backoff", backoff).
			Err(err).
			Msg("Operation failed, retrying")

		// Wait with context cancellation support
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			result.LastError = ctx.Err()
			result.TotalTime = time.Since(startTime)
			return result
		case <-timer.C:
			// Continue to next attempt
		}
	}

	result.TotalTime = time.Since(startTime)
	log.Error().
		Str("operation", operation).
		Int("attempts", result.Attempts).
		Dur("total_time", result.TotalTime).
		Err(result.LastError).
		Msg("Operation failed after all retries")

	return result
}

// ExecuteFunc runs a function that returns an interface{} result with retry logic
func (r *Retryer) ExecuteFunc(ctx context.Context, operation string, fn func() (interface{}, error)) (interface{}, RetryResult) {
	var result interface{}
	retryResult := RetryResult{}
	startTime := time.Now()

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		retryResult.Attempts = attempt + 1

		// Check context before attempt
		if ctx.Err() != nil {
			retryResult.LastError = ctx.Err()
			retryResult.TotalTime = time.Since(startTime)
			return result, retryResult
		}

		// Execute the operation
		res, err := fn()
		if err == nil {
			retryResult.Successful = true
			retryResult.TotalTime = time.Since(startTime)

			if attempt > 0 {
				log.Info().
					Str("operation", operation).
					Int("attempts", retryResult.Attempts).
					Dur("total_time", retryResult.TotalTime).
					Msg("Operation succeeded after retry")
			}
			return res, retryResult
		}

		retryResult.LastError = err

		// Check if error is retryable
		if !r.isRetryable(err) {
			retryResult.TotalTime = time.Since(startTime)
			return result, retryResult
		}

		// Don't wait after the last attempt
		if attempt >= r.config.MaxRetries {
			break
		}

		// Calculate backoff with jitter
		backoff := r.calculateBackoff(attempt)

		log.Warn().
			Str("operation", operation).
			Int("attempt", attempt+1).
			Dur("backoff", backoff).
			Err(err).
			Msg("Operation failed, retrying")

		// Wait with context cancellation support
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			retryResult.LastError = ctx.Err()
			retryResult.TotalTime = time.Since(startTime)
			return result, retryResult
		case <-timer.C:
		}
	}

	retryResult.TotalTime = time.Since(startTime)
	return result, retryResult
}

// isRetryable checks if an error should trigger a retry
func (r *Retryer) isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for context cancellation - never retry these
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Check for RetryableError
	var retryableErr *RetryableError
	if errors.As(err, &retryableErr) {
		if !retryableErr.Retryable {
			return false
		}
		// Check status code
		for _, code := range r.config.RetryableStatusCodes {
			if retryableErr.StatusCode == code {
				return true
			}
		}
		return false
	}

	// Check for circuit breaker errors
	if errors.Is(err, ErrCircuitOpen) {
		return true // Can retry when circuit may recover
	}

	// Network errors are generally retryable
	// This is a simplified check - in production you might want more specific checks
	return true
}

// calculateBackoff calculates the backoff duration for a given attempt
func (r *Retryer) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: initialBackoff * (multiplier ^ attempt)
	backoff := float64(r.config.InitialBackoff) * math.Pow(r.config.BackoffMultiplier, float64(attempt))

	// Apply max backoff cap
	if backoff > float64(r.config.MaxBackoff) {
		backoff = float64(r.config.MaxBackoff)
	}

	// Apply jitter
	if r.config.JitterFactor > 0 {
		jitter := backoff * r.config.JitterFactor * (rand.Float64()*2 - 1)
		backoff += jitter
	}

	// Ensure non-negative
	if backoff < 0 {
		backoff = float64(r.config.InitialBackoff)
	}

	return time.Duration(backoff)
}

// IsRetryableStatusCode checks if a status code should trigger a retry
func (r *Retryer) IsRetryableStatusCode(statusCode int) bool {
	for _, code := range r.config.RetryableStatusCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}
