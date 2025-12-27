package performance

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

// PoolConfig holds configuration for HTTP connection pooling
type PoolConfig struct {
	// MaxIdleConns is the maximum number of idle connections across all hosts
	MaxIdleConns int
	// MaxIdleConnsPerHost is the maximum number of idle connections per host
	MaxIdleConnsPerHost int
	// MaxConnsPerHost limits the total number of connections per host
	MaxConnsPerHost int
	// IdleConnTimeout is how long idle connections remain in the pool
	IdleConnTimeout time.Duration
	// TLSHandshakeTimeout limits TLS handshake time
	TLSHandshakeTimeout time.Duration
	// ResponseHeaderTimeout limits response header wait time
	ResponseHeaderTimeout time.Duration
	// ExpectContinueTimeout limits 100-continue wait time
	ExpectContinueTimeout time.Duration
	// DialTimeout limits TCP connection establishment time
	DialTimeout time.Duration
	// KeepAlive sets the keep-alive period for active connections
	KeepAlive time.Duration
	// DisableCompression disables transport compression
	DisableCompression bool
	// ForceAttemptHTTP2 enables HTTP/2 support
	ForceAttemptHTTP2 bool
}

// DefaultPoolConfig returns production-ready defaults
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		MaxConnsPerHost:       0, // No limit
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialTimeout:           10 * time.Second,
		KeepAlive:             30 * time.Second,
		DisableCompression:    false,
		ForceAttemptHTTP2:     true,
	}
}

// HighThroughputPoolConfig returns config optimized for high throughput
func HighThroughputPoolConfig() PoolConfig {
	return PoolConfig{
		MaxIdleConns:          500,
		MaxIdleConnsPerHost:   50,
		MaxConnsPerHost:       100,
		IdleConnTimeout:       120 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialTimeout:           15 * time.Second,
		KeepAlive:             60 * time.Second,
		DisableCompression:    false,
		ForceAttemptHTTP2:     true,
	}
}

// HTTPClientPool manages pooled HTTP clients for different purposes
type HTTPClientPool struct {
	defaultClient   *http.Client
	streamingClient *http.Client
	config          PoolConfig
}

// NewHTTPClientPool creates a new HTTP client pool with the given configuration
func NewHTTPClientPool(config PoolConfig) *HTTPClientPool {
	pool := &HTTPClientPool{
		config: config,
	}

	// Create the shared transport
	transport := pool.createTransport()

	// Default client with timeout
	pool.defaultClient = &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}

	// Streaming client without timeout (streams can be long-running)
	pool.streamingClient = &http.Client{
		Transport: transport,
		// No timeout for streaming
	}

	log.Info().
		Int("max_idle_conns", config.MaxIdleConns).
		Int("max_idle_conns_per_host", config.MaxIdleConnsPerHost).
		Dur("idle_conn_timeout", config.IdleConnTimeout).
		Bool("http2_enabled", config.ForceAttemptHTTP2).
		Msg("HTTP connection pool initialized")

	return pool
}

// createTransport creates the optimized HTTP transport with connection pooling
func (p *HTTPClientPool) createTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   p.config.DialTimeout,
			KeepAlive: p.config.KeepAlive,
		}).DialContext,
		MaxIdleConns:          p.config.MaxIdleConns,
		MaxIdleConnsPerHost:   p.config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       p.config.MaxConnsPerHost,
		IdleConnTimeout:       p.config.IdleConnTimeout,
		TLSHandshakeTimeout:   p.config.TLSHandshakeTimeout,
		ResponseHeaderTimeout: p.config.ResponseHeaderTimeout,
		ExpectContinueTimeout: p.config.ExpectContinueTimeout,
		DisableCompression:    p.config.DisableCompression,
		ForceAttemptHTTP2:     p.config.ForceAttemptHTTP2,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}
}

// GetDefaultClient returns the default HTTP client with timeout
func (p *HTTPClientPool) GetDefaultClient() *http.Client {
	return p.defaultClient
}

// GetStreamingClient returns the streaming HTTP client without timeout
func (p *HTTPClientPool) GetStreamingClient() *http.Client {
	return p.streamingClient
}

// GetClientWithTimeout returns a client configured with a specific timeout
func (p *HTTPClientPool) GetClientWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: p.defaultClient.Transport,
		Timeout:   timeout,
	}
}

// Stats returns current pool statistics
func (p *HTTPClientPool) Stats() map[string]interface{} {
	return map[string]interface{}{
		"max_idle_conns":          p.config.MaxIdleConns,
		"max_idle_conns_per_host": p.config.MaxIdleConnsPerHost,
		"max_conns_per_host":      p.config.MaxConnsPerHost,
		"idle_conn_timeout":       p.config.IdleConnTimeout.String(),
		"http2_enabled":           p.config.ForceAttemptHTTP2,
	}
}

// Close closes idle connections in the pool
func (p *HTTPClientPool) Close() {
	if transport, ok := p.defaultClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	log.Info().Msg("HTTP connection pool closed")
}

// Global pool instance for convenience
var globalPool *HTTPClientPool

// InitGlobalPool initializes the global HTTP client pool
func InitGlobalPool(config PoolConfig) {
	globalPool = NewHTTPClientPool(config)
}

// GetGlobalPool returns the global HTTP client pool
func GetGlobalPool() *HTTPClientPool {
	if globalPool == nil {
		// Initialize with defaults if not configured
		globalPool = NewHTTPClientPool(DefaultPoolConfig())
	}
	return globalPool
}

// CloseGlobalPool closes the global pool
func CloseGlobalPool() {
	if globalPool != nil {
		globalPool.Close()
	}
}
