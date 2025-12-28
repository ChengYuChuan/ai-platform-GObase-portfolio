package performance

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/username/llm-gateway/pkg/models"
)

func TestDefaultCacheConfig(t *testing.T) {
	cfg := DefaultCacheConfig()

	if cfg.Enabled {
		t.Error("default config should have Enabled = false")
	}
	if cfg.TTL != 1*time.Hour {
		t.Errorf("TTL = %v, want 1h", cfg.TTL)
	}
	if cfg.MaxEntries != 1000 {
		t.Errorf("MaxEntries = %d, want 1000", cfg.MaxEntries)
	}
	if cfg.Backend != "memory" {
		t.Errorf("Backend = %s, want memory", cfg.Backend)
	}
}

func TestNewSemanticCache_Disabled(t *testing.T) {
	cfg := CacheConfig{Enabled: false}

	cache, err := NewSemanticCache(cfg)

	if err != nil {
		t.Errorf("NewSemanticCache() error = %v", err)
	}
	if cache != nil {
		t.Error("cache should be nil when disabled")
	}
}

func TestNewSemanticCache_Memory(t *testing.T) {
	cfg := CacheConfig{
		Enabled:    true,
		TTL:        1 * time.Hour,
		MaxEntries: 100,
		Backend:    "memory",
	}

	cache, err := NewSemanticCache(cfg)
	if err != nil {
		t.Fatalf("NewSemanticCache() error = %v", err)
	}
	if cache == nil {
		t.Fatal("cache should not be nil")
	}
	defer cache.Close()
}

func TestSemanticCache_GenerateCacheKey(t *testing.T) {
	cfg := CacheConfig{
		Enabled:    true,
		TTL:        1 * time.Hour,
		MaxEntries: 100,
		Backend:    "memory",
	}

	cache, _ := NewSemanticCache(cfg)
	defer cache.Close()

	tests := []struct {
		name    string
		req     *models.ChatCompletionRequest
		wantErr error
	}{
		{
			name: "valid non-streaming request",
			req: &models.ChatCompletionRequest{
				Model:    "gpt-4o-mini",
				Stream:   false,
				Messages: []models.ChatMessage{{Role: "user", Content: "Hello"}},
			},
			wantErr: nil,
		},
		{
			name: "streaming request not cacheable",
			req: &models.ChatCompletionRequest{
				Model:    "gpt-4o-mini",
				Stream:   true,
				Messages: []models.ChatMessage{{Role: "user", Content: "Hello"}},
			},
			wantErr: ErrNotCachable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := cache.GenerateCacheKey(tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("GenerateCacheKey() error = %v, want %v", err, tt.wantErr)
			}
			if err == nil && key == "" {
				t.Error("key should not be empty for valid request")
			}
		})
	}
}

func TestSemanticCache_GenerateCacheKey_Deterministic(t *testing.T) {
	cfg := CacheConfig{
		Enabled:    true,
		TTL:        1 * time.Hour,
		MaxEntries: 100,
		Backend:    "memory",
	}

	cache, _ := NewSemanticCache(cfg)
	defer cache.Close()

	req := &models.ChatCompletionRequest{
		Model:    "gpt-4o-mini",
		Stream:   false,
		Messages: []models.ChatMessage{{Role: "user", Content: "Hello"}},
	}

	key1, _ := cache.GenerateCacheKey(req)
	key2, _ := cache.GenerateCacheKey(req)

	if key1 != key2 {
		t.Error("same request should generate same key")
	}
}

func TestSemanticCache_GetSet(t *testing.T) {
	cfg := CacheConfig{
		Enabled:    true,
		TTL:        1 * time.Hour,
		MaxEntries: 100,
		Backend:    "memory",
	}

	cache, _ := NewSemanticCache(cfg)
	defer cache.Close()

	ctx := context.Background()
	req := &models.ChatCompletionRequest{
		Model:    "gpt-4o-mini",
		Stream:   false,
		Messages: []models.ChatMessage{{Role: "user", Content: "Hello"}},
	}
	resp := &models.ChatCompletionResponse{
		ID:    "test-response-id",
		Model: "gpt-4o-mini",
	}

	// Initially should miss
	_, err := cache.Get(ctx, req)
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Get() on empty cache error = %v, want ErrCacheMiss", err)
	}

	// Set value
	err = cache.Set(ctx, req, resp)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Now should hit
	got, err := cache.Get(ctx, req)
	if err != nil {
		t.Fatalf("Get() after Set() error = %v", err)
	}
	if got.ID != resp.ID {
		t.Errorf("Get().ID = %s, want %s", got.ID, resp.ID)
	}
}

func TestSemanticCache_Invalidate(t *testing.T) {
	cfg := CacheConfig{
		Enabled:    true,
		TTL:        1 * time.Hour,
		MaxEntries: 100,
		Backend:    "memory",
	}

	cache, _ := NewSemanticCache(cfg)
	defer cache.Close()

	ctx := context.Background()
	req := &models.ChatCompletionRequest{
		Model:    "gpt-4o-mini",
		Stream:   false,
		Messages: []models.ChatMessage{{Role: "user", Content: "Hello"}},
	}
	resp := &models.ChatCompletionResponse{ID: "test-id"}

	cache.Set(ctx, req, resp)

	// Invalidate
	err := cache.Invalidate(ctx, req)
	if err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}

	// Should miss after invalidation
	_, err = cache.Get(ctx, req)
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Get() after Invalidate() error = %v, want ErrCacheMiss", err)
	}
}

func TestSemanticCache_Clear(t *testing.T) {
	cfg := CacheConfig{
		Enabled:    true,
		TTL:        1 * time.Hour,
		MaxEntries: 100,
		Backend:    "memory",
	}

	cache, _ := NewSemanticCache(cfg)
	defer cache.Close()

	ctx := context.Background()

	// Add multiple entries
	for i := 0; i < 5; i++ {
		req := &models.ChatCompletionRequest{
			Model:    "gpt-4o-mini",
			Stream:   false,
			Messages: []models.ChatMessage{{Role: "user", Content: string(rune('a' + i))}},
		}
		cache.Set(ctx, req, &models.ChatCompletionResponse{ID: string(rune('0' + i))})
	}

	// Clear
	err := cache.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	// All should miss
	req := &models.ChatCompletionRequest{
		Model:    "gpt-4o-mini",
		Stream:   false,
		Messages: []models.ChatMessage{{Role: "user", Content: "a"}},
	}
	_, err = cache.Get(ctx, req)
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Get() after Clear() error = %v, want ErrCacheMiss", err)
	}
}

func TestSemanticCache_Stats(t *testing.T) {
	cfg := CacheConfig{
		Enabled:    true,
		TTL:        1 * time.Hour,
		MaxEntries: 100,
		Backend:    "memory",
	}

	cache, _ := NewSemanticCache(cfg)
	defer cache.Close()

	ctx := context.Background()
	req := &models.ChatCompletionRequest{
		Model:    "gpt-4o-mini",
		Stream:   false,
		Messages: []models.ChatMessage{{Role: "user", Content: "Hello"}},
	}
	resp := &models.ChatCompletionResponse{ID: "test-id"}

	// Miss
	cache.Get(ctx, req)

	// Set and hit
	cache.Set(ctx, req, resp)
	cache.Get(ctx, req)

	stats := cache.Stats()

	if stats["hits"].(int64) != 1 {
		t.Errorf("stats[hits] = %v, want 1", stats["hits"])
	}
	if stats["misses"].(int64) != 1 {
		t.Errorf("stats[misses] = %v, want 1", stats["misses"])
	}
	if stats["sets"].(int64) != 1 {
		t.Errorf("stats[sets] = %v, want 1", stats["sets"])
	}
}

func TestMemoryBackend_Get_Miss(t *testing.T) {
	backend := NewMemoryBackend(100)

	_, err := backend.Get(context.Background(), "nonexistent")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Get() error = %v, want ErrCacheMiss", err)
	}
}

func TestMemoryBackend_SetGet(t *testing.T) {
	backend := NewMemoryBackend(100)
	ctx := context.Background()

	key := "test-key"
	value := []byte("test-value")
	ttl := 1 * time.Hour

	err := backend.Set(ctx, key, value, ttl)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got, err := backend.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if string(got) != string(value) {
		t.Errorf("Get() = %s, want %s", string(got), string(value))
	}
}

func TestMemoryBackend_Expiration(t *testing.T) {
	backend := NewMemoryBackend(100)
	ctx := context.Background()

	key := "expiring-key"
	value := []byte("value")
	ttl := 50 * time.Millisecond

	backend.Set(ctx, key, value, ttl)

	// Should exist immediately
	_, err := backend.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get() immediately after Set() error = %v", err)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, err = backend.Get(ctx, key)
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Get() after expiration error = %v, want ErrCacheMiss", err)
	}
}

func TestMemoryBackend_Eviction(t *testing.T) {
	maxEntries := 5
	backend := NewMemoryBackend(maxEntries)
	ctx := context.Background()

	// Fill cache
	for i := 0; i < maxEntries; i++ {
		key := string(rune('a' + i))
		backend.Set(ctx, key, []byte(key), 1*time.Hour)
	}

	// Add one more (should evict oldest)
	backend.Set(ctx, "z", []byte("z"), 1*time.Hour)

	// First entry should be evicted
	_, err := backend.Get(ctx, "a")
	if !errors.Is(err, ErrCacheMiss) {
		t.Error("oldest entry should have been evicted")
	}

	// Newest entry should exist
	_, err = backend.Get(ctx, "z")
	if err != nil {
		t.Errorf("newest entry should exist, error = %v", err)
	}

	stats := backend.Stats()
	if stats.EntryCount != maxEntries {
		t.Errorf("entry count = %d, want %d", stats.EntryCount, maxEntries)
	}
}

func TestMemoryBackend_Delete(t *testing.T) {
	backend := NewMemoryBackend(100)
	ctx := context.Background()

	key := "delete-me"
	backend.Set(ctx, key, []byte("value"), 1*time.Hour)

	err := backend.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = backend.Get(ctx, key)
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Get() after Delete() error = %v, want ErrCacheMiss", err)
	}
}

func TestMemoryBackend_Clear(t *testing.T) {
	backend := NewMemoryBackend(100)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		backend.Set(ctx, string(rune('a'+i)), []byte("value"), 1*time.Hour)
	}

	err := backend.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	stats := backend.Stats()
	if stats.EntryCount != 0 {
		t.Errorf("entry count after Clear() = %d, want 0", stats.EntryCount)
	}
}

func TestMemoryBackend_Stats(t *testing.T) {
	backend := NewMemoryBackend(100)
	ctx := context.Background()

	backend.Set(ctx, "key1", []byte("value1"), 1*time.Hour)
	backend.Set(ctx, "key2", []byte("longer-value"), 1*time.Hour)

	stats := backend.Stats()

	if stats.EntryCount != 2 {
		t.Errorf("entry count = %d, want 2", stats.EntryCount)
	}
	expectedSize := int64(len("value1") + len("longer-value"))
	if stats.SizeBytes != expectedSize {
		t.Errorf("size = %d, want %d", stats.SizeBytes, expectedSize)
	}
}

func TestMemoryBackend_Concurrent(t *testing.T) {
	backend := NewMemoryBackend(1000)
	ctx := context.Background()

	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := string(rune('a' + id%26))
			for j := 0; j < 10; j++ {
				backend.Set(ctx, key, []byte("value"), 1*time.Hour)
				backend.Get(ctx, key)
			}
		}(i)
	}

	wg.Wait()

	// Just ensure no panics - exact count depends on execution order
	stats := backend.Stats()
	if stats.EntryCount > 26 {
		t.Errorf("entry count = %d, should be <= 26 (a-z)", stats.EntryCount)
	}
}

func TestIsCacheable(t *testing.T) {
	lowTemp := 0.3
	highTemp := 0.8

	tests := []struct {
		name     string
		req      *models.ChatCompletionRequest
		expected bool
	}{
		{
			name: "non-streaming low temp",
			req: &models.ChatCompletionRequest{
				Stream:      false,
				Temperature: &lowTemp,
			},
			expected: true,
		},
		{
			name: "streaming",
			req: &models.ChatCompletionRequest{
				Stream: true,
			},
			expected: false,
		},
		{
			name: "high temperature",
			req: &models.ChatCompletionRequest{
				Stream:      false,
				Temperature: &highTemp,
			},
			expected: false,
		},
		{
			name: "no temperature",
			req: &models.ChatCompletionRequest{
				Stream: false,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCacheable(tt.req); got != tt.expected {
				t.Errorf("IsCacheable() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestBuildCacheKeyFromMessages(t *testing.T) {
	messages1 := []models.ChatMessage{
		{Role: "user", Content: "Hello"},
	}
	messages2 := []models.ChatMessage{
		{Role: "user", Content: "Hello"},
	}
	messages3 := []models.ChatMessage{
		{Role: "user", Content: "World"},
	}

	key1 := BuildCacheKeyFromMessages(messages1)
	key2 := BuildCacheKeyFromMessages(messages2)
	key3 := BuildCacheKeyFromMessages(messages3)

	if key1 != key2 {
		t.Error("same messages should produce same key")
	}
	if key1 == key3 {
		t.Error("different messages should produce different keys")
	}
	if len(key1) != 32 {
		t.Errorf("key length = %d, want 32 (16 bytes hex)", len(key1))
	}
}

func TestRedisBackend_PlaceholderImplementation(t *testing.T) {
	backend, err := NewRedisBackend("localhost:6379", "", 0)
	if err != nil {
		t.Fatalf("NewRedisBackend() error = %v", err)
	}

	ctx := context.Background()

	// Placeholder implementations should return appropriate defaults
	_, err = backend.Get(ctx, "key")
	if !errors.Is(err, ErrCacheMiss) {
		t.Errorf("Get() error = %v, want ErrCacheMiss", err)
	}

	if err := backend.Set(ctx, "key", []byte("value"), time.Hour); err != nil {
		t.Errorf("Set() error = %v", err)
	}

	if err := backend.Delete(ctx, "key"); err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	if err := backend.Clear(ctx); err != nil {
		t.Errorf("Clear() error = %v", err)
	}

	stats := backend.Stats()
	if stats.EntryCount != 0 {
		t.Errorf("Stats().EntryCount = %d, want 0", stats.EntryCount)
	}

	if err := backend.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
