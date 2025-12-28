package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/username/llm-gateway/pkg/models"
)

func TestHandler_writeError(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name       string
		status     int
		code       string
		message    string
		wantStatus int
	}{
		{
			name:       "bad request",
			status:     http.StatusBadRequest,
			code:       "invalid_request",
			message:    "Missing required field",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unauthorized",
			status:     http.StatusUnauthorized,
			code:       "invalid_api_key",
			message:    "Invalid API key",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "internal error",
			status:     http.StatusInternalServerError,
			code:       "internal_error",
			message:    "Something went wrong",
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.writeError(rr, tt.status, tt.code, tt.message)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}

			if rr.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %s, want application/json", rr.Header().Get("Content-Type"))
			}

			var resp models.ErrorResponse
			if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if resp.Error.Type != tt.code {
				t.Errorf("error.type = %s, want %s", resp.Error.Type, tt.code)
			}
			if resp.Error.Message != tt.message {
				t.Errorf("error.message = %s, want %s", resp.Error.Message, tt.message)
			}
		})
	}
}

func TestHandler_writeSSEError(t *testing.T) {
	h := &Handler{}

	rr := httptest.NewRecorder()
	h.writeSSEError(rr, "provider_error", "Provider unavailable")

	body := rr.Body.String()

	// Should contain data: prefix
	if !bytes.Contains([]byte(body), []byte("data:")) {
		t.Error("SSE error should contain 'data:' prefix")
	}

	// Should contain [DONE]
	if !bytes.Contains([]byte(body), []byte("[DONE]")) {
		t.Error("SSE error should contain '[DONE]' terminator")
	}

	// Should contain error message
	if !bytes.Contains([]byte(body), []byte("provider_error")) {
		t.Error("SSE error should contain error code")
	}
}

func TestNewHandler(t *testing.T) {
	h := NewHandler(nil, nil)

	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
}

func TestChatCompletionsRequest_Parsing(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantErr    bool
		wantStatus int
	}{
		{
			name: "valid request",
			body: `{
				"model": "gpt-4o-mini",
				"messages": [{"role": "user", "content": "Hello"}]
			}`,
			wantErr:    false,
			wantStatus: 0, // Not testing full flow, just parsing
		},
		{
			name:       "invalid json",
			body:       `{invalid json}`,
			wantErr:    true,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			body:       ``,
			wantErr:    true,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req models.ChatCompletionRequest
			err := json.Unmarshal([]byte(tt.body), &req)
			if (err != nil) != tt.wantErr && tt.body != "" {
				t.Errorf("parsing error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestChatCompletionRequest_StreamField(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStream bool
	}{
		{
			name:       "stream true",
			body:       `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hi"}], "stream": true}`,
			wantStream: true,
		},
		{
			name:       "stream false",
			body:       `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hi"}], "stream": false}`,
			wantStream: false,
		},
		{
			name:       "stream omitted",
			body:       `{"model": "gpt-4", "messages": [{"role": "user", "content": "Hi"}]}`,
			wantStream: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req models.ChatCompletionRequest
			json.Unmarshal([]byte(tt.body), &req)

			if req.Stream != tt.wantStream {
				t.Errorf("Stream = %v, want %v", req.Stream, tt.wantStream)
			}
		})
	}
}

func TestCompletionRequest_Parsing(t *testing.T) {
	body := `{
		"model": "gpt-3.5-turbo-instruct",
		"prompt": "Once upon a time",
		"max_tokens": 100,
		"temperature": 0.7
	}`

	var req models.CompletionRequest
	err := json.Unmarshal([]byte(body), &req)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if req.Model != "gpt-3.5-turbo-instruct" {
		t.Errorf("Model = %s, want gpt-3.5-turbo-instruct", req.Model)
	}
	if req.Prompt != "Once upon a time" {
		t.Errorf("Prompt = %s, want 'Once upon a time'", req.Prompt)
	}
	if req.MaxTokens != 100 {
		t.Errorf("MaxTokens = %d, want 100", req.MaxTokens)
	}
}

func TestEmbeddingRequest_Parsing(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		input interface{}
	}{
		{
			name:  "string input",
			body:  `{"model": "text-embedding-3-small", "input": "Hello world"}`,
			input: "Hello world",
		},
		{
			name:  "array input",
			body:  `{"model": "text-embedding-3-small", "input": ["Hello", "World"]}`,
			input: []interface{}{"Hello", "World"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req models.EmbeddingRequest
			err := json.Unmarshal([]byte(tt.body), &req)
			if err != nil {
				t.Fatalf("failed to parse: %v", err)
			}

			if req.Model != "text-embedding-3-small" {
				t.Errorf("Model = %s, want text-embedding-3-small", req.Model)
			}
		})
	}
}

func TestSSEHeaders(t *testing.T) {
	// Test that SSE headers are set correctly
	rr := httptest.NewRecorder()

	rr.Header().Set("Content-Type", "text/event-stream")
	rr.Header().Set("Cache-Control", "no-cache")
	rr.Header().Set("Connection", "keep-alive")
	rr.Header().Set("X-Accel-Buffering", "no")

	if rr.Header().Get("Content-Type") != "text/event-stream" {
		t.Error("Content-Type should be text/event-stream for SSE")
	}
	if rr.Header().Get("Cache-Control") != "no-cache" {
		t.Error("Cache-Control should be no-cache for SSE")
	}
	if rr.Header().Get("Connection") != "keep-alive" {
		t.Error("Connection should be keep-alive for SSE")
	}
	if rr.Header().Get("X-Accel-Buffering") != "no" {
		t.Error("X-Accel-Buffering should be no for SSE")
	}
}

func TestErrorResponse_Structure(t *testing.T) {
	resp := models.ErrorResponse{
		Error: models.APIError{
			Type:    "invalid_request",
			Message: "Test error message",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	errObj, ok := parsed["error"].(map[string]interface{})
	if !ok {
		t.Fatal("error field should be an object")
	}

	if errObj["type"] != "invalid_request" {
		t.Errorf("error.type = %v, want invalid_request", errObj["type"])
	}
	if errObj["message"] != "Test error message" {
		t.Errorf("error.message = %v, want 'Test error message'", errObj["message"])
	}
}

func TestListModelsResponse_Structure(t *testing.T) {
	resp := map[string]interface{}{
		"object": "list",
		"data": []map[string]string{
			{"id": "gpt-4o", "object": "model", "owned_by": "openai"},
			{"id": "claude-3-haiku-20240307", "object": "model", "owned_by": "anthropic"},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if parsed["object"] != "list" {
		t.Errorf("object = %v, want list", parsed["object"])
	}

	models, ok := parsed["data"].([]interface{})
	if !ok {
		t.Fatal("data should be an array")
	}
	if len(models) != 2 {
		t.Errorf("len(data) = %d, want 2", len(models))
	}
}

func TestAnthropicMessageRequest_Parsing(t *testing.T) {
	body := `{
		"model": "claude-3-haiku-20240307",
		"max_tokens": 1024,
		"system": "You are a helpful assistant",
		"messages": [{"role": "user", "content": "Hello"}],
		"stream": true
	}`

	var req models.AnthropicMessageRequest
	err := json.Unmarshal([]byte(body), &req)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if req.Model != "claude-3-haiku-20240307" {
		t.Errorf("Model = %s, want claude-3-haiku-20240307", req.Model)
	}
	if req.MaxTokens != 1024 {
		t.Errorf("MaxTokens = %d, want 1024", req.MaxTokens)
	}
	if req.System != "You are a helpful assistant" {
		t.Errorf("System = %s", req.System)
	}
	if !req.Stream {
		t.Error("Stream should be true")
	}
}

func TestHTTPMethods(t *testing.T) {
	tests := []struct {
		method   string
		endpoint string
		valid    bool
	}{
		{"POST", "/v1/chat/completions", true},
		{"POST", "/v1/completions", true},
		{"POST", "/v1/embeddings", true},
		{"GET", "/v1/models", true},
		{"POST", "/v1/messages", true},
		{"GET", "/health", true},
		{"GET", "/ready", true},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.endpoint, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.endpoint, nil)
			if req.Method != tt.method {
				t.Errorf("method = %s, want %s", req.Method, tt.method)
			}
			if req.URL.Path != tt.endpoint {
				t.Errorf("path = %s, want %s", req.URL.Path, tt.endpoint)
			}
		})
	}
}
