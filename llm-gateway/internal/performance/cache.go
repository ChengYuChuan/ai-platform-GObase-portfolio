package performance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/username/llm-gateway/pkg/models"
)

var (
	ErrCacheMiss   = errors.New("cache miss")
	ErrCacheError  = errors.New("cache error")
	ErrNotCachable = errors.New("request not cacheable")
)

// CacheConfig holds cache configuration
type CacheConfig struct {
	Enabled bool
	TTL     time.Duration
	// MaxEntries limits memory cache size (0 = unlimited)
	MaxEntries int
	// Backend specifies cache backend: "memory" or "redis"
	Backend string
	// Redis configuration
	RedisAddress  string
	RedisPassword string
	RedisDB       int
}

// DefaultCacheConfig returns sensible defaults
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:    false,
		TTL:        1 * time.Hour,
		MaxEntries: 1000,
		Backend:    "memory",
	}
}

// CacheBackend defines the interface for cache storage
type CacheBackend interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Stats() CacheStats
	Close() error
}

// CacheStats holds cache statistics
type CacheStats struct {
	Hits       int64
	Misses     int64
	Sets       int64
	Deletes    int64
	Evictions  int64
	EntryCount int
	SizeBytes  int64
}

// SemanticCache provides semantic caching for LLM responses
type SemanticCache struct {
	backend CacheBackend
	config  CacheConfig
	mu      sync.RWMutex
	stats   CacheStats
}

// NewSemanticCache creates a new semantic cache with the specified backend
func NewSemanticCache(config CacheConfig) (*SemanticCache, error) {
	if !config.Enabled {
		return nil, nil
	}

	var backend CacheBackend
	var err error

	switch config.Backend {
	case "redis":
		backend, err = NewRedisBackend(config.RedisAddress, config.RedisPassword, config.RedisDB)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to connect to Redis, falling back to memory cache")
			backend = NewMemoryBackend(config.MaxEntries)
		}
	case "memory":
		fallthrough
	default:
		backend = NewMemoryBackend(config.MaxEntries)
	}

	cache := &SemanticCache{
		backend: backend,
		config:  config,
	}

	log.Info().
		Str("backend", config.Backend).
		Dur("ttl", config.TTL).
		Msg("Semantic cache initialized")

	return cache, nil
}

// GenerateCacheKey creates a deterministic cache key from a chat request
func (c *SemanticCache) GenerateCacheKey(req *models.ChatCompletionRequest) (string, error) {
	// Don't cache streaming requests
	if req.Stream {
		return "", ErrNotCachable
	}

	// Create a normalized representation of the request
	keyData := struct {
		Model       string               `json:"model"`
		Messages    []models.ChatMessage `json:"messages"`
		Temperature *float64             `json:"temperature,omitempty"`
		MaxTokens   int                  `json:"max_tokens,omitempty"`
		TopP        *float64             `json:"top_p,omitempty"`
		Stop        []string             `json:"stop,omitempty"`
	}{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		TopP:        req.TopP,
		Stop:        req.Stop,
	}

	// Sort stop tokens for consistency
	if len(keyData.Stop) > 0 {
		sort.Strings(keyData.Stop)
	}

	// Serialize to JSON
	data, err := json.Marshal(keyData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal cache key data: %w", err)
	}

	// Generate SHA-256 hash
	hash := sha256.Sum256(data)
	key := "llm:chat:" + hex.EncodeToString(hash[:])

	return key, nil
}

// Get retrieves a cached response
func (c *SemanticCache) Get(ctx context.Context, req *models.ChatCompletionRequest) (*models.ChatCompletionResponse, error) {
	key, err := c.GenerateCacheKey(req)
	if err != nil {
		return nil, err
	}

	data, err := c.backend.Get(ctx, key)
	if err != nil {
		c.mu.Lock()
		c.stats.Misses++
		c.mu.Unlock()
		return nil, err
	}

	var resp models.ChatCompletionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached response: %w", err)
	}

	c.mu.Lock()
	c.stats.Hits++
	c.mu.Unlock()

	log.Debug().
		Str("key", key).
		Str("model", req.Model).
		Msg("Cache hit")

	return &resp, nil
}

// Set stores a response in the cache
func (c *SemanticCache) Set(ctx context.Context, req *models.ChatCompletionRequest, resp *models.ChatCompletionResponse) error {
	key, err := c.GenerateCacheKey(req)
	if err != nil {
		return err
	}

	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response for caching: %w", err)
	}

	if err := c.backend.Set(ctx, key, data, c.config.TTL); err != nil {
		return err
	}

	c.mu.Lock()
	c.stats.Sets++
	c.mu.Unlock()

	log.Debug().
		Str("key", key).
		Str("model", req.Model).
		Int("size_bytes", len(data)).
		Msg("Response cached")

	return nil
}

// Invalidate removes a specific entry from the cache
func (c *SemanticCache) Invalidate(ctx context.Context, req *models.ChatCompletionRequest) error {
	key, err := c.GenerateCacheKey(req)
	if err != nil {
		return err
	}

	if err := c.backend.Delete(ctx, key); err != nil {
		return err
	}

	c.mu.Lock()
	c.stats.Deletes++
	c.mu.Unlock()

	return nil
}

// Clear removes all entries from the cache
func (c *SemanticCache) Clear(ctx context.Context) error {
	return c.backend.Clear(ctx)
}

// Stats returns cache statistics
func (c *SemanticCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	backendStats := c.backend.Stats()

	hitRate := float64(0)
	total := c.stats.Hits + c.stats.Misses
	if total > 0 {
		hitRate = float64(c.stats.Hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"enabled":     c.config.Enabled,
		"backend":     c.config.Backend,
		"ttl":         c.config.TTL.String(),
		"hits":        c.stats.Hits,
		"misses":      c.stats.Misses,
		"sets":        c.stats.Sets,
		"deletes":     c.stats.Deletes,
		"hit_rate":    fmt.Sprintf("%.2f%%", hitRate),
		"entry_count": backendStats.EntryCount,
		"size_bytes":  backendStats.SizeBytes,
	}
}

// Close closes the cache backend
func (c *SemanticCache) Close() error {
	return c.backend.Close()
}

// MemoryBackend implements an in-memory cache with LRU eviction
type MemoryBackend struct {
	mu         sync.RWMutex
	entries    map[string]*cacheEntry
	order      []string
	maxEntries int
	stats      CacheStats
}

type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

// NewMemoryBackend creates a new in-memory cache backend
func NewMemoryBackend(maxEntries int) *MemoryBackend {
	if maxEntries <= 0 {
		maxEntries = 1000
	}

	backend := &MemoryBackend{
		entries:    make(map[string]*cacheEntry),
		order:      make([]string, 0, maxEntries),
		maxEntries: maxEntries,
	}

	// Start cleanup goroutine
	go backend.cleanupLoop()

	return backend
}

func (b *MemoryBackend) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		b.cleanup()
	}
}

func (b *MemoryBackend) cleanup() {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	for key, entry := range b.entries {
		if now.After(entry.expiresAt) {
			delete(b.entries, key)
			b.removeFromOrder(key)
		}
	}
}

func (b *MemoryBackend) removeFromOrder(key string) {
	for i, k := range b.order {
		if k == key {
			b.order = append(b.order[:i], b.order[i+1:]...)
			return
		}
	}
}

func (b *MemoryBackend) Get(ctx context.Context, key string) ([]byte, error) {
	b.mu.RLock()
	entry, ok := b.entries[key]
	b.mu.RUnlock()

	if !ok || time.Now().After(entry.expiresAt) {
		return nil, ErrCacheMiss
	}

	return entry.data, nil
}

func (b *MemoryBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Evict oldest if at capacity
	if len(b.entries) >= b.maxEntries {
		if len(b.order) > 0 {
			oldest := b.order[0]
			delete(b.entries, oldest)
			b.order = b.order[1:]
			b.stats.Evictions++
		}
	}

	b.entries[key] = &cacheEntry{
		data:      value,
		expiresAt: time.Now().Add(ttl),
	}
	b.order = append(b.order, key)

	return nil
}

func (b *MemoryBackend) Delete(ctx context.Context, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.entries, key)
	b.removeFromOrder(key)

	return nil
}

func (b *MemoryBackend) Clear(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.entries = make(map[string]*cacheEntry)
	b.order = make([]string, 0, b.maxEntries)

	return nil
}

func (b *MemoryBackend) Stats() CacheStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var sizeBytes int64
	for _, entry := range b.entries {
		sizeBytes += int64(len(entry.data))
	}

	return CacheStats{
		EntryCount: len(b.entries),
		SizeBytes:  sizeBytes,
		Evictions:  b.stats.Evictions,
	}
}

func (b *MemoryBackend) Close() error {
	return nil
}

// RedisBackend implements a Redis-based cache backend
type RedisBackend struct {
	// Note: In production, you would use github.com/redis/go-redis/v9
	// For now, we provide a placeholder that can be easily integrated
	address  string
	password string
	db       int
	// client *redis.Client  // Uncomment when adding redis dependency
}

// NewRedisBackend creates a new Redis cache backend
func NewRedisBackend(address, password string, db int) (*RedisBackend, error) {
	// Placeholder implementation - would use go-redis in production
	// For now, return error to fall back to memory cache

	if address == "" {
		address = "localhost:6379"
	}

	backend := &RedisBackend{
		address:  address,
		password: password,
		db:       db,
	}

	// In production:
	// client := redis.NewClient(&redis.Options{
	//     Addr:     address,
	//     Password: password,
	//     DB:       db,
	// })
	// if err := client.Ping(context.Background()).Err(); err != nil {
	//     return nil, err
	// }
	// backend.client = client

	log.Info().
		Str("address", address).
		Int("db", db).
		Msg("Redis backend initialized (placeholder mode)")

	return backend, nil
}

func (b *RedisBackend) Get(ctx context.Context, key string) ([]byte, error) {
	// Placeholder - would use Redis GET command
	// val, err := b.client.Get(ctx, key).Bytes()
	// if err == redis.Nil {
	//     return nil, ErrCacheMiss
	// }
	// return val, err
	return nil, ErrCacheMiss
}

func (b *RedisBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	// Placeholder - would use Redis SET with EX
	// return b.client.Set(ctx, key, value, ttl).Err()
	return nil
}

func (b *RedisBackend) Delete(ctx context.Context, key string) error {
	// Placeholder - would use Redis DEL
	// return b.client.Del(ctx, key).Err()
	return nil
}

func (b *RedisBackend) Clear(ctx context.Context) error {
	// Placeholder - would use Redis FLUSHDB or pattern delete
	// return b.client.FlushDB(ctx).Err()
	return nil
}

func (b *RedisBackend) Stats() CacheStats {
	// Placeholder - would query Redis INFO
	return CacheStats{}
}

func (b *RedisBackend) Close() error {
	// Placeholder - would close Redis connection
	// return b.client.Close()
	return nil
}

// CacheMiddleware provides caching at the handler level
type CacheMiddleware struct {
	cache *SemanticCache
}

// NewCacheMiddleware creates a new cache middleware
func NewCacheMiddleware(cache *SemanticCache) *CacheMiddleware {
	return &CacheMiddleware{cache: cache}
}

// IsCacheable checks if a request can be cached
func IsCacheable(req *models.ChatCompletionRequest) bool {
	// Don't cache streaming requests
	if req.Stream {
		return false
	}

	// Don't cache if temperature is high (more randomness = less cacheable)
	if req.Temperature != nil && *req.Temperature > 0.5 {
		return false
	}

	// Check for cache-control headers or flags
	// This could be extended to check request metadata

	return true
}

// BuildCacheKeyFromMessages creates a semantic hash from message content
func BuildCacheKeyFromMessages(messages []models.ChatMessage) string {
	var parts []string
	for _, msg := range messages {
		parts = append(parts, fmt.Sprintf("%s:%s", msg.Role, msg.Content))
	}
	content := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes
}
