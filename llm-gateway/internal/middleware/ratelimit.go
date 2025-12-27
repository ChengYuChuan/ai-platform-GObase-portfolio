package middleware

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"

	"github.com/username/llm-gateway/internal/config"
)

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	mu              sync.RWMutex
	buckets         map[string]*tokenBucket
	requestsPerMin  int
	burstSize       int
	cleanupInterval time.Duration
	stopCleanup     chan struct{}
}

// tokenBucket represents a single client's rate limit bucket
type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter from config
func NewRateLimiter(cfg config.RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		buckets:         make(map[string]*tokenBucket),
		requestsPerMin:  cfg.RequestsPerMin,
		burstSize:       cfg.BurstSize,
		cleanupInterval: cfg.CleanupInterval,
		stopCleanup:     make(chan struct{}),
	}

	// Start cleanup goroutine to prevent memory leaks
	go rl.cleanup()

	return rl
}

// RateLimit returns a middleware that rate limits requests
func (rl *RateLimiter) RateLimit() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get client identifier (API key > IP address)
			clientID := rl.getClientID(r)

			// Check rate limit
			if !rl.allow(clientID) {
				rl.writeRateLimitError(w, clientID)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// getClientID extracts client identifier from request
func (rl *RateLimiter) getClientID(r *http.Request) string {
	// Priority: API Key > X-Forwarded-For > Remote Address
	if apiKey := r.Context().Value(APIKeyContextKey); apiKey != nil {
		if key, ok := apiKey.(string); ok && key != "" {
			return "key:" + key[:min(8, len(key))] + "***" // Partially mask for logging
		}
	}

	// Use request ID for tracking
	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		// Fall back to IP-based limiting
	}

	// Use IP address
	return "ip:" + r.RemoteAddr
}

// allow checks if a request should be allowed based on token bucket
func (rl *RateLimiter) allow(clientID string) bool {
	bucket := rl.getBucket(clientID)

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Refill tokens based on time passed
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	tokensPerSecond := float64(rl.requestsPerMin) / 60.0

	// Add new tokens (capped at burst size)
	bucket.tokens += elapsed * tokensPerSecond
	if bucket.tokens > float64(rl.burstSize) {
		bucket.tokens = float64(rl.burstSize)
	}
	bucket.lastRefill = now

	// Check if we have enough tokens
	if bucket.tokens >= 1.0 {
		bucket.tokens -= 1.0
		return true
	}

	return false
}

// getBucket gets or creates a token bucket for the client
func (rl *RateLimiter) getBucket(clientID string) *tokenBucket {
	rl.mu.RLock()
	bucket, exists := rl.buckets[clientID]
	rl.mu.RUnlock()

	if exists {
		return bucket
	}

	// Create new bucket
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if bucket, exists = rl.buckets[clientID]; exists {
		return bucket
	}

	bucket = &tokenBucket{
		tokens:     float64(rl.burstSize), // Start with full bucket
		lastRefill: time.Now(),
	}
	rl.buckets[clientID] = bucket

	return bucket
}

// cleanup periodically removes stale buckets to prevent memory leaks
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.removeStale()
		case <-rl.stopCleanup:
			return
		}
	}
}

// removeStale removes buckets that haven't been used recently
func (rl *RateLimiter) removeStale() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	staleThreshold := time.Now().Add(-5 * time.Minute)
	staleCount := 0

	for clientID, bucket := range rl.buckets {
		bucket.mu.Lock()
		if bucket.lastRefill.Before(staleThreshold) {
			delete(rl.buckets, clientID)
			staleCount++
		}
		bucket.mu.Unlock()
	}

	if staleCount > 0 {
		log.Debug().
			Int("removed", staleCount).
			Int("remaining", len(rl.buckets)).
			Msg("Rate limiter cleanup completed")
	}
}

// Stop stops the cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
}

// writeRateLimitError writes a rate limit exceeded error response
func (rl *RateLimiter) writeRateLimitError(w http.ResponseWriter, clientID string) {
	log.Warn().
		Str("client_id", clientID).
		Int("requests_per_min", rl.requestsPerMin).
		Msg("Rate limit exceeded")

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", "60")
	w.Header().Set("X-RateLimit-Limit", string(rune(rl.requestsPerMin)))
	w.Header().Set("X-RateLimit-Remaining", "0")
	w.WriteHeader(http.StatusTooManyRequests)

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"message": "Rate limit exceeded. Please retry after some time.",
			"type":    "rate_limit_error",
			"code":    "rate_limit_exceeded",
		},
	}

	json.NewEncoder(w).Encode(response)
}

// GetStats returns current rate limiter statistics
func (rl *RateLimiter) GetStats() map[string]interface{} {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	return map[string]interface{}{
		"active_clients":    len(rl.buckets),
		"requests_per_min":  rl.requestsPerMin,
		"burst_size":        rl.burstSize,
		"cleanup_interval":  rl.cleanupInterval.String(),
	}
}
