package rest

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"

	"github.com/username/llm-gateway/internal/config"
	"github.com/username/llm-gateway/internal/proxy"
	"github.com/username/llm-gateway/pkg/models"
)

// Handler handles HTTP requests for LLM endpoints
type Handler struct {
	config      *config.Config
	proxyRouter *proxy.Router
}

// NewHandler creates a new Handler with dependencies
func NewHandler(cfg *config.Config, proxyRouter *proxy.Router) *Handler {
	return &Handler{
		config:      cfg,
		proxyRouter: proxyRouter,
	}
}

// ChatCompletions handles POST /v1/chat/completions (OpenAI-compatible)
func (h *Handler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetReqID(ctx)

	// Parse request body
	var req models.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Failed to parse request body: "+err.Error())
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	log.Debug().
		Str("request_id", requestID).
		Str("model", req.Model).
		Bool("stream", req.Stream).
		Int("messages", len(req.Messages)).
		Msg("Processing chat completion request")

	// Determine provider from model name
	provider, err := h.proxyRouter.GetProviderForModel(req.Model)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_model", err.Error())
		return
	}

	// Handle streaming vs non-streaming
	if req.Stream {
		h.handleStreamingResponse(w, r, provider, &req)
	} else {
		h.handleSyncResponse(w, r, provider, &req)
	}
}

// handleSyncResponse handles non-streaming chat completion
func (h *Handler) handleSyncResponse(w http.ResponseWriter, r *http.Request, provider proxy.Provider, req *models.ChatCompletionRequest) {
	ctx := r.Context()

	resp, err := provider.ChatCompletion(ctx, req)
	if err != nil {
		var providerErr *proxy.ProviderError
		if errors.As(err, &providerErr) {
			h.writeError(w, providerErr.StatusCode, providerErr.Code, providerErr.Message)
			return
		}
		h.writeError(w, http.StatusInternalServerError, "provider_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// handleStreamingResponse handles SSE streaming chat completion
func (h *Handler) handleStreamingResponse(w http.ResponseWriter, r *http.Request, provider proxy.Provider, req *models.ChatCompletionRequest) {
	ctx := r.Context()

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Get streaming response from provider
	stream, err := provider.ChatCompletionStream(ctx, req)
	if err != nil {
		var providerErr *proxy.ProviderError
		if errors.As(err, &providerErr) {
			// For streaming, we need to send error as SSE event
			h.writeSSEError(w, providerErr.Code, providerErr.Message)
			return
		}
		h.writeSSEError(w, "provider_error", err.Error())
		return
	}
	defer stream.Close()

	// Flush writer for SSE
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.writeSSEError(w, "streaming_not_supported", "Response writer does not support flushing")
		return
	}

	// Read and forward stream
	reader := bufio.NewReader(stream)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err == io.EOF {
					// Send final [DONE] message if not already sent
					w.Write([]byte("data: [DONE]\n\n"))
					flusher.Flush()
					return
				}
				log.Error().Err(err).Msg("Error reading stream")
				return
			}

			// Forward the line as-is (provider returns SSE-formatted data)
			w.Write(line)
			flusher.Flush()
		}
	}
}

// Completions handles POST /v1/completions (legacy endpoint)
func (h *Handler) Completions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetReqID(ctx)

	var req models.CompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Failed to parse request body: "+err.Error())
		return
	}

	if err := req.Validate(); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	log.Debug().
		Str("request_id", requestID).
		Str("model", req.Model).
		Bool("stream", req.Stream).
		Msg("Processing legacy completion request")

	provider, err := h.proxyRouter.GetProviderForModel(req.Model)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_model", err.Error())
		return
	}

	resp, err := provider.Completion(ctx, &req)
	if err != nil {
		var providerErr *proxy.ProviderError
		if errors.As(err, &providerErr) {
			h.writeError(w, providerErr.StatusCode, providerErr.Code, providerErr.Message)
			return
		}
		h.writeError(w, http.StatusInternalServerError, "provider_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// Embeddings handles POST /v1/embeddings
func (h *Handler) Embeddings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req models.EmbeddingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Failed to parse request body: "+err.Error())
		return
	}

	if err := req.Validate(); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	provider, err := h.proxyRouter.GetProviderForModel(req.Model)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_model", err.Error())
		return
	}

	resp, err := provider.Embedding(ctx, &req)
	if err != nil {
		var providerErr *proxy.ProviderError
		if errors.As(err, &providerErr) {
			h.writeError(w, providerErr.StatusCode, providerErr.Code, providerErr.Message)
			return
		}
		h.writeError(w, http.StatusInternalServerError, "provider_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// ListModels handles GET /v1/models
func (h *Handler) ListModels(w http.ResponseWriter, r *http.Request) {
	models := h.proxyRouter.ListModels()

	resp := map[string]interface{}{
		"object": "list",
		"data":   models,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

// AnthropicMessages handles POST /v1/messages (Anthropic-compatible)
func (h *Handler) AnthropicMessages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetReqID(ctx)

	var req models.AnthropicMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_request", "Failed to parse request body: "+err.Error())
		return
	}

	log.Debug().
		Str("request_id", requestID).
		Str("model", req.Model).
		Bool("stream", req.Stream).
		Msg("Processing Anthropic-style message request")

	// Route to Anthropic provider
	provider, err := h.proxyRouter.GetProvider("anthropic")
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "provider_unavailable", "Anthropic provider not configured")
		return
	}

	// Convert to internal format and process
	chatReq := req.ToChatCompletionRequest()
	
	if req.Stream {
		h.handleStreamingResponse(w, r, provider, chatReq)
	} else {
		h.handleSyncResponse(w, r, provider, chatReq)
	}
}

// writeError writes a JSON error response
func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	
	resp := models.ErrorResponse{
		Error: models.APIError{
			Type:    code,
			Message: message,
		},
	}
	json.NewEncoder(w).Encode(resp)
}

// writeSSEError writes an error as SSE event
func (h *Handler) writeSSEError(w http.ResponseWriter, code, message string) {
	errData, _ := json.Marshal(map[string]interface{}{
		"error": map[string]string{
			"type":    code,
			"message": message,
		},
	})
	w.Write([]byte("data: " + string(errData) + "\n\n"))
	w.Write([]byte("data: [DONE]\n\n"))
	
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
