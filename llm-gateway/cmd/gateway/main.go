package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/username/llm-gateway/internal/api/rest"
	"github.com/username/llm-gateway/internal/config"
	"github.com/username/llm-gateway/internal/performance"
	"github.com/username/llm-gateway/internal/proxy"
	"github.com/username/llm-gateway/internal/proxy/providers"
)

func main() {
	// Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	initLogger(cfg)
	log.Info().Str("version", cfg.Version).Msg("Starting LLM Gateway")

	// Initialize HTTP connection pool for providers
	poolConfig := performance.PoolConfig{
		MaxIdleConns:        cfg.Performance.ConnectionPool.MaxIdleConns,
		MaxIdleConnsPerHost: cfg.Performance.ConnectionPool.MaxIdleConnsPerHost,
		MaxConnsPerHost:     cfg.Performance.ConnectionPool.MaxConnsPerHost,
		IdleConnTimeout:     cfg.Performance.ConnectionPool.IdleConnTimeout,
		TLSHandshakeTimeout: 10 * time.Second,
		DialTimeout:         10 * time.Second,
		KeepAlive:           30 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	performance.InitGlobalPool(poolConfig)
	defer performance.CloseGlobalPool()

	// Initialize providers
	providerRegistry := initProviders(cfg)

	// Initialize proxy router
	proxyRouter := proxy.NewRouter(providerRegistry, cfg)

	// Initialize HTTP server
	router := rest.NewRouter(cfg, proxyRouter)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Info().
			Int("port", cfg.Server.Port).
			Msg("HTTP server starting")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("HTTP server failed")
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server stopped")
}

// initLogger configures the global zerolog logger
func initLogger(cfg *config.Config) {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure output format
	if cfg.Log.Format == "pretty" {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		})
	} else {
		// JSON format (default for production)
		zerolog.TimeFieldFormat = time.RFC3339Nano
	}

	log.Logger = log.With().
		Str("service", "llm-gateway").
		Str("version", cfg.Version).
		Logger()
}

// initProviders initializes all configured LLM providers
func initProviders(cfg *config.Config) *providers.Registry {
	registry := providers.NewRegistry()

	// Register OpenAI provider if configured
	if cfg.Providers.OpenAI.APIKey != "" {
		openai := providers.NewOpenAIProvider(providers.OpenAIConfig{
			APIKey:  cfg.Providers.OpenAI.APIKey,
			BaseURL: cfg.Providers.OpenAI.BaseURL,
			Timeout: cfg.Providers.OpenAI.Timeout,
		})
		registry.Register("openai", openai)
		log.Info().Msg("OpenAI provider registered")
	}

	// Register Anthropic provider if configured
	if cfg.Providers.Anthropic.APIKey != "" {
		anthropic := providers.NewAnthropicProvider(providers.AnthropicConfig{
			APIKey:  cfg.Providers.Anthropic.APIKey,
			BaseURL: cfg.Providers.Anthropic.BaseURL,
			Timeout: cfg.Providers.Anthropic.Timeout,
			Version: cfg.Providers.Anthropic.Version,
		})
		registry.Register("anthropic", anthropic)
		log.Info().Msg("Anthropic provider registered")
	}

	// Register Ollama provider if configured
	if cfg.Providers.Ollama.BaseURL != "" {
		ollama := providers.NewOllamaProvider(providers.OllamaProviderConfig{
			BaseURL: cfg.Providers.Ollama.BaseURL,
			Timeout: cfg.Providers.Ollama.Timeout,
		})
		registry.Register("ollama", ollama)
		log.Info().Str("base_url", cfg.Providers.Ollama.BaseURL).Msg("Ollama provider registered")
	}

	return registry
}
