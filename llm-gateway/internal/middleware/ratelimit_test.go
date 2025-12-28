package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/username/llm-gateway/internal/config"
)

func TestNewRateLimiter(t *testing.T) {
	cfg := config.RateLimitConfig{
		Enabled:         true,
		RequestsPerMin:  60,
		BurstSize:       10,
		CleanupInterval: 1 * time.Minute,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	if rl.requestsPerMin != 60 {
		t.Errorf("requestsPerMin = %d, want 60", rl.requestsPerMin)
	}
	if rl.burstSize != 10 {
		t.Errorf("burstSize = %d, want 10", rl.burstSize)
	}
}

func TestRateLimiter_Allow(t *testing.T) {
	cfg := config.RateLimitConfig{
		Enabled:         true,
		RequestsPerMin:  60,
		BurstSize:       5,
		CleanupInterval: 1 * time.Minute,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	clientID := "test-client"

	// First burst should be allowed
	for i := 0; i < cfg.BurstSize; i++ {
		if !rl.allow(clientID) {
			t.Errorf("request %d should be allowed within burst", i+1)
		}
	}

	// Next request should be denied (exceeded burst)
	if rl.allow(clientID) {
		t.Error("request after burst should be denied")
	}
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	cfg := config.RateLimitConfig{
		Enabled:         true,
		RequestsPerMin:  600, // 10 per second
		BurstSize:       1,
		CleanupInterval: 1 * time.Minute,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	clientID := "test-refill"

	// Use the token
	if !rl.allow(clientID) {
		t.Error("first request should be allowed")
	}

	// Should be denied immediately
	if rl.allow(clientID) {
		t.Error("second request should be denied")
	}

	// Wait for refill (100ms should add ~1 token at 10/sec)
	time.Sleep(150 * time.Millisecond)

	// Should be allowed after refill
	if !rl.allow(clientID) {
		t.Error("request after refill should be allowed")
	}
}

func TestRateLimiter_MultipleClients(t *testing.T) {
	cfg := config.RateLimitConfig{
		Enabled:         true,
		RequestsPerMin:  60,
		BurstSize:       2,
		CleanupInterval: 1 * time.Minute,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	// Each client should have their own bucket
	clients := []string{"client-a", "client-b", "client-c"}

	for _, clientID := range clients {
		// Each should get their full burst allowance
		for i := 0; i < cfg.BurstSize; i++ {
			if !rl.allow(clientID) {
				t.Errorf("request %d for %s should be allowed", i+1, clientID)
			}
		}
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	cfg := config.RateLimitConfig{
		Enabled:         true,
		RequestsPerMin:  60,
		BurstSize:       2,
		CleanupInterval: 1 * time.Minute,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := rl.RateLimit()
	wrappedHandler := middleware(handler)

	// Make requests
	for i := 0; i < cfg.BurstSize+2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rr := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(rr, req)

		if i < cfg.BurstSize {
			if rr.Code != http.StatusOK {
				t.Errorf("request %d: got status %d, want 200", i+1, rr.Code)
			}
		} else {
			if rr.Code != http.StatusTooManyRequests {
				t.Errorf("request %d: got status %d, want 429", i+1, rr.Code)
			}
		}
	}
}

func TestRateLimiter_GetClientID_WithAPIKey(t *testing.T) {
	cfg := config.RateLimitConfig{
		Enabled:         true,
		RequestsPerMin:  60,
		BurstSize:       10,
		CleanupInterval: 1 * time.Minute,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	// Add API key to context
	ctx := context.WithValue(req.Context(), APIKeyContextKey, "sk-test-api-key-12345")
	req = req.WithContext(ctx)

	clientID := rl.getClientID(req)

	// Should use partial API key
	if clientID != "key:sk-test-***" {
		t.Errorf("getClientID = %s, want key:sk-test-***", clientID)
	}
}

func TestRateLimiter_GetClientID_WithIP(t *testing.T) {
	cfg := config.RateLimitConfig{
		Enabled:         true,
		RequestsPerMin:  60,
		BurstSize:       10,
		CleanupInterval: 1 * time.Minute,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:54321"

	clientID := rl.getClientID(req)

	if clientID != "ip:192.168.1.100:54321" {
		t.Errorf("getClientID = %s, want ip:192.168.1.100:54321", clientID)
	}
}

func TestRateLimiter_GetStats(t *testing.T) {
	cfg := config.RateLimitConfig{
		Enabled:         true,
		RequestsPerMin:  60,
		BurstSize:       10,
		CleanupInterval: 1 * time.Minute,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	// Make some requests
	rl.allow("client-1")
	rl.allow("client-2")

	stats := rl.GetStats()

	if stats["active_clients"].(int) != 2 {
		t.Errorf("active_clients = %v, want 2", stats["active_clients"])
	}
	if stats["requests_per_min"].(int) != 60 {
		t.Errorf("requests_per_min = %v, want 60", stats["requests_per_min"])
	}
	if stats["burst_size"].(int) != 10 {
		t.Errorf("burst_size = %v, want 10", stats["burst_size"])
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	cfg := config.RateLimitConfig{
		Enabled:         true,
		RequestsPerMin:  600,
		BurstSize:       100,
		CleanupInterval: 1 * time.Minute,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			clientID := "client-" + string(rune('a'+id%5))
			for j := 0; j < 10; j++ {
				rl.allow(clientID)
			}
		}(i)
	}

	wg.Wait()

	stats := rl.GetStats()
	if stats["active_clients"].(int) != 5 {
		t.Errorf("active_clients = %v, want 5", stats["active_clients"])
	}
}

func TestRateLimiter_RemoveStale(t *testing.T) {
	cfg := config.RateLimitConfig{
		Enabled:         true,
		RequestsPerMin:  60,
		BurstSize:       10,
		CleanupInterval: 100 * time.Millisecond,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	// Create some buckets
	rl.allow("client-1")
	rl.allow("client-2")

	stats := rl.GetStats()
	if stats["active_clients"].(int) != 2 {
		t.Fatalf("initial active_clients = %v, want 2", stats["active_clients"])
	}

	// Manually set last refill time to past
	rl.mu.Lock()
	for _, bucket := range rl.buckets {
		bucket.lastRefill = time.Now().Add(-10 * time.Minute)
	}
	rl.mu.Unlock()

	// Trigger cleanup
	rl.removeStale()

	stats = rl.GetStats()
	if stats["active_clients"].(int) != 0 {
		t.Errorf("active_clients after cleanup = %v, want 0", stats["active_clients"])
	}
}

func TestRateLimiter_WriteRateLimitError(t *testing.T) {
	cfg := config.RateLimitConfig{
		Enabled:         true,
		RequestsPerMin:  60,
		BurstSize:       10,
		CleanupInterval: 1 * time.Minute,
	}

	rl := NewRateLimiter(cfg)
	defer rl.Stop()

	rr := httptest.NewRecorder()
	rl.writeRateLimitError(rr, "test-client")

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("status = %d, want 429", rr.Code)
	}

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", rr.Header().Get("Content-Type"))
	}

	if rr.Header().Get("Retry-After") != "60" {
		t.Errorf("Retry-After = %s, want 60", rr.Header().Get("Retry-After"))
	}
}
