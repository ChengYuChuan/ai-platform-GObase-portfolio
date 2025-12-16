package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// APIKeyContextKey is the context key for the API key
	APIKeyContextKey contextKey = "api_key"
	// UserIDContextKey is the context key for the user ID
	UserIDContextKey contextKey = "user_id"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	// Enabled determines if authentication is required
	Enabled bool
	// ValidKeys is a map of valid API keys to user IDs
	ValidKeys map[string]string
	// HeaderName is the header to look for the API key (default: Authorization)
	HeaderName string
	// Prefix is the expected prefix (default: Bearer)
	Prefix string
}

// DefaultAuthConfig returns default authentication configuration
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		Enabled:    false,
		ValidKeys:  make(map[string]string),
		HeaderName: "Authorization",
		Prefix:     "Bearer",
	}
}

// Auth returns a middleware that validates API keys
func Auth(config AuthConfig) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth if disabled
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			// Extract API key from header
			apiKey := extractAPIKey(r, config)
			if apiKey == "" {
				writeAuthError(w, "missing_api_key", "API key is required")
				return
			}

			// Validate API key
			userID, valid := config.ValidKeys[apiKey]
			if !valid {
				log.Warn().
					Str("ip", r.RemoteAddr).
					Msg("Invalid API key attempted")
				writeAuthError(w, "invalid_api_key", "Invalid API key")
				return
			}

			// Add API key and user ID to context
			ctx := context.WithValue(r.Context(), APIKeyContextKey, apiKey)
			ctx = context.WithValue(ctx, UserIDContextKey, userID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractAPIKey extracts the API key from the request
func extractAPIKey(r *http.Request, config AuthConfig) string {
	// Try Authorization header first
	authHeader := r.Header.Get(config.HeaderName)
	if authHeader != "" {
		// Check for Bearer prefix
		if config.Prefix != "" {
			prefix := config.Prefix + " "
			if strings.HasPrefix(authHeader, prefix) {
				return strings.TrimPrefix(authHeader, prefix)
			}
		}
		return authHeader
	}

	// Try X-API-Key header as fallback
	apiKey := r.Header.Get("X-API-Key")
	if apiKey != "" {
		return apiKey
	}

	// Try query parameter as last resort (not recommended for production)
	return r.URL.Query().Get("api_key")
}

// writeAuthError writes an authentication error response
func writeAuthError(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte(`{"error":{"type":"` + code + `","message":"` + message + `"}}`))
}

// GetAPIKey retrieves the API key from the request context
func GetAPIKey(ctx context.Context) string {
	if key, ok := ctx.Value(APIKeyContextKey).(string); ok {
		return key
	}
	return ""
}

// GetUserID retrieves the user ID from the request context
func GetUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDContextKey).(string); ok {
		return userID
	}
	return ""
}

// APIKeyValidator is a function type for validating API keys externally
type APIKeyValidator func(apiKey string) (userID string, valid bool)

// AuthWithValidator returns a middleware that uses an external validator
func AuthWithValidator(validator APIKeyValidator) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			config := DefaultAuthConfig()
			config.Enabled = true

			apiKey := extractAPIKey(r, config)
			if apiKey == "" {
				writeAuthError(w, "missing_api_key", "API key is required")
				return
			}

			userID, valid := validator(apiKey)
			if !valid {
				log.Warn().
					Str("ip", r.RemoteAddr).
					Msg("Invalid API key attempted")
				writeAuthError(w, "invalid_api_key", "Invalid API key")
				return
			}

			ctx := context.WithValue(r.Context(), APIKeyContextKey, apiKey)
			ctx = context.WithValue(ctx, UserIDContextKey, userID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
