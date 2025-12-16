package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

// responseWriter wraps http.ResponseWriter to capture status code and bytes written
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	bytes       int
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

// Unwrap returns the underlying ResponseWriter for http.Flusher support
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// Flush implements http.Flusher for streaming support
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Logger returns a middleware that logs HTTP requests using zerolog
func Logger() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := wrapResponseWriter(w)

			// Get request ID from chi middleware
			requestID := middleware.GetReqID(r.Context())

			// Process request
			next.ServeHTTP(wrapped, r)

			// Calculate duration
			duration := time.Since(start)

			// Build log event
			event := log.Info()
			
			// Adjust log level based on status code
			if wrapped.status >= 500 {
				event = log.Error()
			} else if wrapped.status >= 400 {
				event = log.Warn()
			}

			// Log the request
			event.
				Str("request_id", requestID).
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("query", r.URL.RawQuery).
				Int("status", wrapped.status).
				Int("bytes", wrapped.bytes).
				Dur("duration", duration).
				Str("ip", r.RemoteAddr).
				Str("user_agent", r.UserAgent()).
				Msg("HTTP request")
		})
	}
}

// RequestLogger creates a logger for a specific request context
// Useful for adding request-scoped fields to logs
func RequestLogger(r *http.Request) *log.Logger {
	requestID := middleware.GetReqID(r.Context())
	logger := log.With().
		Str("request_id", requestID).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()
	return &logger
}
