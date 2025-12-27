package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the gateway
type Config struct {
	Version      string             `mapstructure:"version"`
	Server       ServerConfig       `mapstructure:"server"`
	Log          LogConfig          `mapstructure:"log"`
	Providers    ProvidersConfig    `mapstructure:"providers"`
	RateLimit    RateLimitConfig    `mapstructure:"rate_limit"`
	Reliability  ReliabilityConfig  `mapstructure:"reliability"`
	Cache        CacheConfig        `mapstructure:"cache"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// ProvidersConfig holds all LLM provider configurations
type ProvidersConfig struct {
	Default   string          `mapstructure:"default"`
	OpenAI    OpenAIConfig    `mapstructure:"openai"`
	Anthropic AnthropicConfig `mapstructure:"anthropic"`
	Ollama    OllamaConfig    `mapstructure:"ollama"`
}

// OpenAIConfig holds OpenAI-specific configuration
type OpenAIConfig struct {
	APIKey  string        `mapstructure:"api_key"`
	BaseURL string        `mapstructure:"base_url"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// AnthropicConfig holds Anthropic-specific configuration
type AnthropicConfig struct {
	APIKey  string        `mapstructure:"api_key"`
	BaseURL string        `mapstructure:"base_url"`
	Timeout time.Duration `mapstructure:"timeout"`
	Version string        `mapstructure:"version"`
}

// OllamaConfig holds Ollama-specific configuration
type OllamaConfig struct {
	BaseURL string        `mapstructure:"base_url"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	RequestsPerMin  int           `mapstructure:"requests_per_min"`
	BurstSize       int           `mapstructure:"burst_size"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

// ReliabilityConfig holds reliability feature configuration
type ReliabilityConfig struct {
	CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker"`
	Retry          RetryConfig          `mapstructure:"retry"`
}

// CircuitBreakerConfig holds circuit breaker settings
type CircuitBreakerConfig struct {
	Enabled             bool          `mapstructure:"enabled"`
	FailureThreshold    int           `mapstructure:"failure_threshold"`
	SuccessThreshold    int           `mapstructure:"success_threshold"`
	Timeout             time.Duration `mapstructure:"timeout"`
	MaxHalfOpenRequests int           `mapstructure:"max_half_open_requests"`
}

// RetryConfig holds retry settings
type RetryConfig struct {
	Enabled           bool          `mapstructure:"enabled"`
	MaxRetries        int           `mapstructure:"max_retries"`
	InitialBackoff    time.Duration `mapstructure:"initial_backoff"`
	MaxBackoff        time.Duration `mapstructure:"max_backoff"`
	BackoffMultiplier float64       `mapstructure:"backoff_multiplier"`
}

// CacheConfig holds caching configuration
type CacheConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	TTL     time.Duration `mapstructure:"ttl"`
	Redis   RedisConfig   `mapstructure:"redis"`
}

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Address  string `mapstructure:"address"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// Load reads configuration from file and environment variables
func Load() (*Config, error) {
	v := viper.New()

	// Set config name and paths
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	v.AddConfigPath("/etc/llm-gateway")

	// Set defaults
	setDefaults(v)

	// Enable environment variable override
	v.SetEnvPrefix("LLM_GATEWAY")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file (optional - env vars can override everything)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found is OK - we use defaults and env vars
	}

	// Unmarshal config
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate required fields
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// setDefaults sets default values for all configuration options
func setDefaults(v *viper.Viper) {
	// Version
	v.SetDefault("version", "0.1.0")

	// Server defaults
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "120s") // Longer for streaming
	v.SetDefault("server.idle_timeout", "120s")

	// Log defaults
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")

	// Provider defaults
	v.SetDefault("providers.default", "openai")
	v.SetDefault("providers.openai.base_url", "https://api.openai.com/v1")
	v.SetDefault("providers.openai.timeout", "60s")
	v.SetDefault("providers.anthropic.base_url", "https://api.anthropic.com")
	v.SetDefault("providers.anthropic.timeout", "60s")
	v.SetDefault("providers.anthropic.version", "2023-06-01")
	v.SetDefault("providers.ollama.base_url", "http://localhost:11434")
	v.SetDefault("providers.ollama.timeout", "120s")

	// Rate limit defaults
	v.SetDefault("rate_limit.enabled", false)
	v.SetDefault("rate_limit.requests_per_min", 60)
	v.SetDefault("rate_limit.burst_size", 10)
	v.SetDefault("rate_limit.cleanup_interval", "1m")

	// Reliability defaults - Circuit Breaker
	v.SetDefault("reliability.circuit_breaker.enabled", true)
	v.SetDefault("reliability.circuit_breaker.failure_threshold", 5)
	v.SetDefault("reliability.circuit_breaker.success_threshold", 3)
	v.SetDefault("reliability.circuit_breaker.timeout", "30s")
	v.SetDefault("reliability.circuit_breaker.max_half_open_requests", 1)

	// Reliability defaults - Retry
	v.SetDefault("reliability.retry.enabled", true)
	v.SetDefault("reliability.retry.max_retries", 3)
	v.SetDefault("reliability.retry.initial_backoff", "500ms")
	v.SetDefault("reliability.retry.max_backoff", "30s")
	v.SetDefault("reliability.retry.backoff_multiplier", 2.0)

	// Cache defaults
	v.SetDefault("cache.enabled", false)
	v.SetDefault("cache.ttl", "1h")
	v.SetDefault("cache.redis.address", "localhost:6379")
	v.SetDefault("cache.redis.db", 0)
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Check if at least one provider is configured
	hasProvider := c.Providers.OpenAI.APIKey != "" ||
		c.Providers.Anthropic.APIKey != "" ||
		c.Providers.Ollama.BaseURL != ""

	if !hasProvider {
		// Allow running without providers for health check testing
		// but log a warning
	}

	// Validate server port
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	return nil
}

// GetProviderConfig returns the configuration for a specific provider
func (c *Config) GetProviderConfig(name string) interface{} {
	switch name {
	case "openai":
		return c.Providers.OpenAI
	case "anthropic":
		return c.Providers.Anthropic
	case "ollama":
		return c.Providers.Ollama
	default:
		return nil
	}
}
