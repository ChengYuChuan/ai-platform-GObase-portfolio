package config

import (
	"os"
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config with OpenAI",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Providers: ProvidersConfig{
					OpenAI: OpenAIConfig{APIKey: "sk-test-key"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with Anthropic",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Providers: ProvidersConfig{
					Anthropic: AnthropicConfig{APIKey: "sk-ant-test"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with Ollama",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Providers: ProvidersConfig{
					Ollama: OllamaConfig{BaseURL: "http://localhost:11434"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid port - zero",
			config: Config{
				Server: ServerConfig{Port: 0},
			},
			wantErr: true,
		},
		{
			name: "invalid port - negative",
			config: Config{
				Server: ServerConfig{Port: -1},
			},
			wantErr: true,
		},
		{
			name: "invalid port - too high",
			config: Config{
				Server: ServerConfig{Port: 70000},
			},
			wantErr: true,
		},
		{
			name: "valid port at boundary",
			config: Config{
				Server: ServerConfig{Port: 65535},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_GetProviderConfig(t *testing.T) {
	cfg := &Config{
		Providers: ProvidersConfig{
			OpenAI: OpenAIConfig{
				APIKey:  "openai-key",
				BaseURL: "https://api.openai.com/v1",
			},
			Anthropic: AnthropicConfig{
				APIKey:  "anthropic-key",
				BaseURL: "https://api.anthropic.com",
			},
			Ollama: OllamaConfig{
				BaseURL: "http://localhost:11434",
			},
		},
	}

	tests := []struct {
		name     string
		provider string
		wantNil  bool
	}{
		{"openai", "openai", false},
		{"anthropic", "anthropic", false},
		{"ollama", "ollama", false},
		{"unknown", "unknown", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cfg.GetProviderConfig(tt.provider)
			if (got == nil) != tt.wantNil {
				t.Errorf("GetProviderConfig(%q) = %v, wantNil %v", tt.provider, got, tt.wantNil)
			}
		})
	}
}

func TestServerConfig_Defaults(t *testing.T) {
	// Test that defaults are properly set
	cfg := ServerConfig{
		Port:         8080,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	if cfg.Port != 8080 {
		t.Errorf("expected port 8080, got %d", cfg.Port)
	}
	if cfg.ReadTimeout != 30*time.Second {
		t.Errorf("expected read timeout 30s, got %v", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 120*time.Second {
		t.Errorf("expected write timeout 120s, got %v", cfg.WriteTimeout)
	}
}

func TestLoad_WithEnvVars(t *testing.T) {
	// Save and restore env vars
	origPort := os.Getenv("LLM_GATEWAY_SERVER_PORT")
	origLogLevel := os.Getenv("LLM_GATEWAY_LOG_LEVEL")
	defer func() {
		os.Setenv("LLM_GATEWAY_SERVER_PORT", origPort)
		os.Setenv("LLM_GATEWAY_LOG_LEVEL", origLogLevel)
	}()

	// Set test env vars
	os.Setenv("LLM_GATEWAY_SERVER_PORT", "9090")
	os.Setenv("LLM_GATEWAY_LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("expected port 9090 from env, got %d", cfg.Server.Port)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("expected log level 'debug' from env, got %s", cfg.Log.Level)
	}
}

func TestRateLimitConfig_Defaults(t *testing.T) {
	cfg := RateLimitConfig{
		Enabled:        true,
		RequestsPerMin: 60,
		BurstSize:      10,
	}

	if cfg.RequestsPerMin != 60 {
		t.Errorf("expected 60 requests per min, got %d", cfg.RequestsPerMin)
	}
	if cfg.BurstSize != 10 {
		t.Errorf("expected burst size 10, got %d", cfg.BurstSize)
	}
}

func TestCacheConfig_Backends(t *testing.T) {
	tests := []struct {
		name    string
		backend string
		valid   bool
	}{
		{"memory backend", "memory", true},
		{"redis backend", "redis", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := CacheConfig{
				Enabled: true,
				Backend: tt.backend,
			}
			if cfg.Backend != tt.backend {
				t.Errorf("expected backend %s, got %s", tt.backend, cfg.Backend)
			}
		})
	}
}

func TestReliabilityConfig_CircuitBreaker(t *testing.T) {
	cfg := ReliabilityConfig{
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:             true,
			FailureThreshold:    5,
			SuccessThreshold:    3,
			Timeout:             30 * time.Second,
			MaxHalfOpenRequests: 1,
		},
	}

	if !cfg.CircuitBreaker.Enabled {
		t.Error("circuit breaker should be enabled")
	}
	if cfg.CircuitBreaker.FailureThreshold != 5 {
		t.Errorf("expected failure threshold 5, got %d", cfg.CircuitBreaker.FailureThreshold)
	}
}

func TestRetryConfig_Settings(t *testing.T) {
	cfg := RetryConfig{
		Enabled:           true,
		MaxRetries:        3,
		InitialBackoff:    500 * time.Millisecond,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
	}

	if !cfg.Enabled {
		t.Error("retry should be enabled")
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", cfg.MaxRetries)
	}
	if cfg.BackoffMultiplier != 2.0 {
		t.Errorf("expected backoff multiplier 2.0, got %f", cfg.BackoffMultiplier)
	}
}
