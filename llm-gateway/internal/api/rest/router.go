package rest

import (
	"compress/gzip"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"

	"github.com/username/llm-gateway/internal/config"
	"github.com/username/llm-gateway/internal/middleware"
	"github.com/username/llm-gateway/internal/observability"
	"github.com/username/llm-gateway/internal/performance"
	"github.com/username/llm-gateway/internal/proxy"
)

// rateLimiter holds the global rate limiter instance
var rateLimiter *middleware.RateLimiter

// NewRouter creates and configures a new Chi router with all routes and middleware
func NewRouter(cfg *config.Config, proxyRouter *proxy.Router) http.Handler {
	r := chi.NewRouter()

	// ============================================
	// Global Middleware Stack
	// ============================================

	// Request ID for tracing
	r.Use(chimiddleware.RequestID)

	// Real IP extraction (for reverse proxy setups)
	r.Use(chimiddleware.RealIP)

	// Custom structured logging with zerolog
	r.Use(middleware.Logger())

	// Panic recovery
	r.Use(chimiddleware.Recoverer)

	// Request timeout (configurable)
	r.Use(chimiddleware.Timeout(cfg.Server.WriteTimeout))

	// Rate limiting (if enabled)
	if cfg.RateLimit.Enabled {
		rateLimiter = middleware.NewRateLimiter(cfg.RateLimit)
		r.Use(rateLimiter.RateLimit())
		log.Info().
			Int("requests_per_min", cfg.RateLimit.RequestsPerMin).
			Int("burst_size", cfg.RateLimit.BurstSize).
			Msg("Rate limiting enabled")
	}

	// CORS (configure as needed for your frontend)
	r.Use(corsMiddleware)

	// Observability middleware (metrics and tracing)
	if cfg.Observability.Metrics.Enabled || cfg.Observability.Tracing.Enabled {
		// Initialize metrics if enabled
		var metrics *observability.Metrics
		if cfg.Observability.Metrics.Enabled {
			metricsConfig := observability.MetricsConfig{
				Enabled:   true,
				Path:      cfg.Observability.Metrics.Path,
				Namespace: cfg.Observability.Metrics.Namespace,
				Subsystem: "http",
			}
			metrics = observability.InitGlobalMetrics(metricsConfig)
		}

		// Initialize tracer if enabled
		var tracer *observability.Tracer
		if cfg.Observability.Tracing.Enabled {
			tracingConfig := observability.TracingConfig{
				Enabled:      true,
				ServiceName:  cfg.Observability.Tracing.ServiceName,
				SamplingRate: cfg.Observability.Tracing.SamplingRate,
				ExporterType: cfg.Observability.Tracing.ExporterType,
			}
			tracer = observability.InitGlobalTracer(tracingConfig)
		}

		// Add combined observability middleware
		r.Use(observability.ObservabilityMiddleware(tracer, metrics))
		log.Info().
			Bool("metrics", cfg.Observability.Metrics.Enabled).
			Bool("tracing", cfg.Observability.Tracing.Enabled).
			Msg("Observability middleware enabled")
	}

	// Response compression (if enabled)
	if cfg.Performance.Compression.Enabled {
		compressionLevel := cfg.Performance.Compression.Level
		if compressionLevel == 0 {
			compressionLevel = gzip.DefaultCompression
		}
		compressionConfig := performance.CompressionConfig{
			Enabled: true,
			Level:   compressionLevel,
			MinSize: cfg.Performance.Compression.MinSize,
			ContentTypes: []string{
				"application/json",
				"text/plain",
				"text/html",
			},
		}
		r.Use(performance.CompressionMiddleware(compressionConfig))
		log.Info().
			Int("level", compressionLevel).
			Int("min_size", cfg.Performance.Compression.MinSize).
			Msg("Response compression enabled")
	}

	// ============================================
	// Health & Metrics Endpoints (no auth required)
	// ============================================
	r.Group(func(r chi.Router) {
		r.Get("/health", healthHandler)
		r.Get("/ready", readyHandler(proxyRouter))
		// Use real metrics handler if available
		if cfg.Observability.Metrics.Enabled {
			r.Get(cfg.Observability.Metrics.Path, observability.GetMetrics().Handler())
		} else {
			r.Get("/metrics", metricsHandler)
		}
	})

	// ============================================
	// API v1 Routes
	// ============================================
	r.Route("/v1", func(r chi.Router) {
		// Create handler with dependencies
		h := NewHandler(cfg, proxyRouter)

		// Chat completions (OpenAI-compatible)
		r.Post("/chat/completions", h.ChatCompletions)

		// Legacy completions endpoint
		r.Post("/completions", h.Completions)

		// Embeddings
		r.Post("/embeddings", h.Embeddings)

		// Models listing
		r.Get("/models", h.ListModels)
	})

	// ============================================
	// Anthropic-style Routes (optional compatibility)
	// ============================================
	r.Route("/v1/messages", func(r chi.Router) {
		h := NewHandler(cfg, proxyRouter)
		r.Post("/", h.AnthropicMessages)
	})

	return r
}

// corsMiddleware handles CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-API-Key")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// healthHandler returns basic health status
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"healthy","service":"llm-gateway"}`))
}

// readyHandler checks if the service is ready to accept traffic
func readyHandler(proxyRouter *proxy.Router) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if at least one provider is available
		providers := proxyRouter.AvailableProviders()
		
		w.Header().Set("Content-Type", "application/json")
		
		if len(providers) == 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"not_ready","reason":"no providers available"}`))
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready","providers":` + formatProviders(providers) + `}`))
	}
}

// metricsHandler placeholder for Prometheus metrics
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement Prometheus metrics exposition
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("# Metrics endpoint - Prometheus integration pending\n"))
}

// formatProviders converts provider list to JSON array string
func formatProviders(providers []string) string {
	if len(providers) == 0 {
		return "[]"
	}
	result := `["`
	for i, p := range providers {
		if i > 0 {
			result += `","`
		}
		result += p
	}
	result += `"]`
	return result
}
