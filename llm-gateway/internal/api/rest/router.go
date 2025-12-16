package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/username/llm-gateway/internal/config"
	"github.com/username/llm-gateway/internal/middleware"
	"github.com/username/llm-gateway/internal/proxy"
)

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
	
	// Request timeout
	r.Use(chimiddleware.Timeout(60 * time.Second))

	// CORS (configure as needed for your frontend)
	r.Use(corsMiddleware)

	// ============================================
	// Health & Metrics Endpoints (no auth required)
	// ============================================
	r.Group(func(r chi.Router) {
		r.Get("/health", healthHandler)
		r.Get("/ready", readyHandler(proxyRouter))
		r.Get("/metrics", metricsHandler) // Placeholder for Prometheus
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
