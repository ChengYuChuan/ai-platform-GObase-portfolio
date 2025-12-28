package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractAPIKey_WithConfig(t *testing.T) {
	config := DefaultAuthConfig()

	tests := []struct {
		name       string
		authHeader string
		wantKey    string
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer sk-test-api-key-12345",
			wantKey:    "sk-test-api-key-12345",
		},
		{
			name:       "empty header",
			authHeader: "",
			wantKey:    "",
		},
		{
			name:       "bearer without space",
			authHeader: "Bearersk-no-space",
			wantKey:    "Bearersk-no-space", // Returns as-is if no proper prefix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			key := extractAPIKey(req, config)
			if key != tt.wantKey {
				t.Errorf("extractAPIKey() = %s, want %s", key, tt.wantKey)
			}
		})
	}
}

func TestExtractAPIKey_XAPIKeyHeader(t *testing.T) {
	config := DefaultAuthConfig()

	tests := []struct {
		name    string
		header  string
		wantKey string
	}{
		{
			name:    "valid x-api-key",
			header:  "sk-x-api-key-value",
			wantKey: "sk-x-api-key-value",
		},
		{
			name:    "empty header",
			header:  "",
			wantKey: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.header != "" {
				req.Header.Set("X-API-Key", tt.header)
			}

			key := extractAPIKey(req, config)
			if key != tt.wantKey {
				t.Errorf("extractAPIKey() = %s, want %s", key, tt.wantKey)
			}
		})
	}
}

func TestExtractAPIKey_QueryParam(t *testing.T) {
	config := DefaultAuthConfig()

	req := httptest.NewRequest("GET", "/test?api_key=sk-query-param-key", nil)
	key := extractAPIKey(req, config)

	if key != "sk-query-param-key" {
		t.Errorf("extractAPIKey() = %s, want sk-query-param-key", key)
	}
}

func TestExtractAPIKey_Priority(t *testing.T) {
	config := DefaultAuthConfig()

	// Authorization header should take priority
	req := httptest.NewRequest("GET", "/test?api_key=query-key", nil)
	req.Header.Set("Authorization", "Bearer header-key")
	req.Header.Set("X-API-Key", "x-api-key")

	key := extractAPIKey(req, config)
	if key != "header-key" {
		t.Errorf("extractAPIKey() = %s, want header-key (Authorization takes priority)", key)
	}
}

func TestGetAPIKey(t *testing.T) {
	apiKey := "sk-test-context-key"
	ctx := context.WithValue(context.Background(), APIKeyContextKey, apiKey)

	got := GetAPIKey(ctx)
	if got != apiKey {
		t.Errorf("GetAPIKey() = %s, want %s", got, apiKey)
	}
}

func TestGetAPIKey_NotSet(t *testing.T) {
	ctx := context.Background()

	got := GetAPIKey(ctx)
	if got != "" {
		t.Errorf("GetAPIKey() = %s, want empty string", got)
	}
}

func TestGetUserID(t *testing.T) {
	userID := "user-123"
	ctx := context.WithValue(context.Background(), UserIDContextKey, userID)

	got := GetUserID(ctx)
	if got != userID {
		t.Errorf("GetUserID() = %s, want %s", got, userID)
	}
}

func TestGetUserID_NotSet(t *testing.T) {
	ctx := context.Background()

	got := GetUserID(ctx)
	if got != "" {
		t.Errorf("GetUserID() = %s, want empty string", got)
	}
}

func TestDefaultAuthConfig(t *testing.T) {
	config := DefaultAuthConfig()

	if config.Enabled {
		t.Error("Enabled should be false by default")
	}
	if config.HeaderName != "Authorization" {
		t.Errorf("HeaderName = %s, want Authorization", config.HeaderName)
	}
	if config.Prefix != "Bearer" {
		t.Errorf("Prefix = %s, want Bearer", config.Prefix)
	}
	if config.ValidKeys == nil {
		t.Error("ValidKeys should not be nil")
	}
}

func TestAuthMiddleware_Disabled(t *testing.T) {
	config := DefaultAuthConfig()
	config.Enabled = false

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	middleware := Auth(config)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	// No API key provided
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 when auth disabled", rr.Code)
	}
}

func TestAuthMiddleware_MissingAPIKey(t *testing.T) {
	config := DefaultAuthConfig()
	config.Enabled = true
	config.ValidKeys = map[string]string{"valid-key": "user-1"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := Auth(config)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	// No API key
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for missing API key", rr.Code)
	}
}

func TestAuthMiddleware_InvalidAPIKey(t *testing.T) {
	config := DefaultAuthConfig()
	config.Enabled = true
	config.ValidKeys = map[string]string{"valid-key": "user-1"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := Auth(config)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-key")
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for invalid API key", rr.Code)
	}
}

func TestAuthMiddleware_ValidAPIKey(t *testing.T) {
	config := DefaultAuthConfig()
	config.Enabled = true
	config.ValidKeys = map[string]string{"valid-key": "user-1"}

	var capturedAPIKey string
	var capturedUserID string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAPIKey = GetAPIKey(r.Context())
		capturedUserID = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	middleware := Auth(config)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-key")
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for valid API key", rr.Code)
	}
	if capturedAPIKey != "valid-key" {
		t.Errorf("captured API key = %s, want valid-key", capturedAPIKey)
	}
	if capturedUserID != "user-1" {
		t.Errorf("captured user ID = %s, want user-1", capturedUserID)
	}
}

func TestAuthWithValidator(t *testing.T) {
	validator := func(apiKey string) (string, bool) {
		if apiKey == "custom-valid-key" {
			return "custom-user", true
		}
		return "", false
	}

	var capturedUserID string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = GetUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	middleware := AuthWithValidator(validator)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer custom-valid-key")
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rr.Code)
	}
	if capturedUserID != "custom-user" {
		t.Errorf("user ID = %s, want custom-user", capturedUserID)
	}
}

func TestAuthWithValidator_Invalid(t *testing.T) {
	validator := func(apiKey string) (string, bool) {
		return "", false
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := AuthWithValidator(validator)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer any-key")
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}
}

func TestWriteAuthError(t *testing.T) {
	rr := httptest.NewRecorder()
	writeAuthError(rr, "test_error", "Test error message")

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rr.Code)
	}

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %s, want application/json", rr.Header().Get("Content-Type"))
	}

	body := rr.Body.String()
	if body == "" {
		t.Error("body should not be empty")
	}
}

func TestAuthConfig_CustomHeaderName(t *testing.T) {
	config := AuthConfig{
		Enabled:    true,
		ValidKeys:  map[string]string{"my-key": "user-1"},
		HeaderName: "X-Custom-Auth",
		Prefix:     "",
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Custom-Auth", "my-key")

	key := extractAPIKey(req, config)
	if key != "my-key" {
		t.Errorf("extractAPIKey() = %s, want my-key", key)
	}
}

func TestAuthConfig_NoPrefix(t *testing.T) {
	config := AuthConfig{
		Enabled:    true,
		ValidKeys:  map[string]string{"raw-key": "user-1"},
		HeaderName: "Authorization",
		Prefix:     "",
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "raw-key")

	key := extractAPIKey(req, config)
	if key != "raw-key" {
		t.Errorf("extractAPIKey() = %s, want raw-key", key)
	}
}
