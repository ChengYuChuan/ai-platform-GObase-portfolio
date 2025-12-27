package observability

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

// responseWriter wraps http.ResponseWriter to capture status code and size
type responseWriter struct {
	http.ResponseWriter
	status      int
	size        int64
	wroteHeader bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
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
	rw.size += int64(n)
	return n, err
}

// Flush implements http.Flusher
func (rw *responseWriter) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// TracingMiddleware adds distributed tracing to requests
func TracingMiddleware(tracer *Tracer) func(http.Handler) http.Handler {
	if tracer == nil || !tracer.config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Start span from incoming request
			ctx, span := tracer.StartSpanFromHTTP(r, r.Method+" "+r.URL.Path)
			defer span.End()

			// Add request ID if present
			if reqID := middleware.GetReqID(ctx); reqID != "" {
				span.SetAttribute("request.id", reqID)
			}

			// Wrap the request with the new context
			r = r.WithContext(ctx)

			// Wrap response writer to capture status
			rw := newResponseWriter(w)

			// Call the next handler
			next.ServeHTTP(rw, r)

			// Set span status based on HTTP status code
			span.SetAttribute("http.status_code", rw.status)
			span.SetAttribute("http.response_size", rw.size)

			if rw.status >= 400 {
				span.SetStatus(StatusError, http.StatusText(rw.status))
			} else {
				span.SetStatus(StatusOK, "")
			}
		})
	}
}

// MetricsMiddleware collects HTTP metrics
func MetricsMiddleware(metrics *Metrics) func(http.Handler) http.Handler {
	if metrics == nil {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Track in-flight requests
			metrics.RequestsInFlight.Inc()
			defer metrics.RequestsInFlight.Dec()

			// Wrap response writer
			rw := newResponseWriter(w)

			// Call the next handler
			next.ServeHTTP(rw, r)

			// Record metrics
			duration := time.Since(start)
			metrics.RecordRequest(r.Method, r.URL.Path, rw.status, duration, rw.size)
		})
	}
}

// LoggingMiddleware provides structured request logging with trace context
func LoggingMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx := r.Context()

			// Get trace context for logging
			traceID := TraceID(ctx)
			spanID := SpanID(ctx)

			// Create request logger
			event := log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Str("remote_addr", r.RemoteAddr)

			if traceID != "" {
				event = event.Str("trace_id", traceID)
			}
			if spanID != "" {
				event = event.Str("span_id", spanID)
			}

			// Log request start
			event.Msg("Request started")

			// Wrap response writer
			rw := newResponseWriter(w)

			// Call the next handler
			next.ServeHTTP(rw, r)

			// Log request completion
			duration := time.Since(start)
			completionEvent := log.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", rw.status).
				Int64("size", rw.size).
				Dur("duration", duration)

			if traceID != "" {
				completionEvent = completionEvent.Str("trace_id", traceID)
			}

			completionEvent.Msg("Request completed")
		})
	}
}

// ObservabilityMiddleware combines tracing, metrics, and logging
func ObservabilityMiddleware(tracer *Tracer, metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Chain: Tracing -> Metrics -> Logging -> Handler
		handler := next

		// Add logging (innermost)
		handler = LoggingMiddleware()(handler)

		// Add metrics
		if metrics != nil {
			handler = MetricsMiddleware(metrics)(handler)
		}

		// Add tracing (outermost)
		if tracer != nil {
			handler = TracingMiddleware(tracer)(handler)
		}

		return handler
	}
}

// ProviderMetricsWrapper wraps provider calls with metrics
type ProviderMetricsWrapper struct {
	metrics *Metrics
}

// NewProviderMetricsWrapper creates a new provider metrics wrapper
func NewProviderMetricsWrapper(metrics *Metrics) *ProviderMetricsWrapper {
	return &ProviderMetricsWrapper{metrics: metrics}
}

// RecordCall records a provider call
func (w *ProviderMetricsWrapper) RecordCall(provider, operation string, start time.Time, err error) {
	if w.metrics == nil {
		return
	}

	duration := time.Since(start)
	success := err == nil
	w.metrics.RecordProviderRequest(provider, operation, success, duration)
}

// RecordTokens records token usage
func (w *ProviderMetricsWrapper) RecordTokens(provider, model string, promptTokens, completionTokens int) {
	if w.metrics == nil {
		return
	}
	w.metrics.RecordTokenUsage(provider, model, promptTokens, completionTokens)
}

// ProviderTracingWrapper wraps provider calls with tracing
type ProviderTracingWrapper struct {
	tracer *Tracer
}

// NewProviderTracingWrapper creates a new provider tracing wrapper
func NewProviderTracingWrapper(tracer *Tracer) *ProviderTracingWrapper {
	return &ProviderTracingWrapper{tracer: tracer}
}

// StartSpan starts a span for a provider call
func (w *ProviderTracingWrapper) StartSpan(ctx context.Context, provider, operation string) (context.Context, *Span) {
	if w.tracer == nil {
		return ctx, nil
	}

	ctx, span := w.tracer.StartSpan(ctx, provider+"."+operation)
	span.SetAttribute("provider", provider)
	span.SetAttribute("operation", operation)

	return ctx, span
}

// RecordError records an error on a span
func (w *ProviderTracingWrapper) RecordError(span *Span, err error) {
	if span == nil {
		return
	}

	span.SetStatus(StatusError, err.Error())
	span.AddEvent("error", map[string]interface{}{
		"message": err.Error(),
	})
}

// ObservabilityConfig holds all observability configuration
type ObservabilityConfig struct {
	Metrics MetricsConfig
	Tracing TracingConfig
	Logging LoggingConfig
}

// DefaultObservabilityConfig returns sensible defaults
func DefaultObservabilityConfig() ObservabilityConfig {
	return ObservabilityConfig{
		Metrics: DefaultMetricsConfig(),
		Tracing: DefaultTracingConfig(),
		Logging: DefaultLoggingConfig(),
	}
}

// Observability holds all observability components
type Observability struct {
	Metrics *Metrics
	Tracer  *Tracer
	Logger  *Logger
	Config  ObservabilityConfig
}

// NewObservability creates a new observability instance
func NewObservability(config ObservabilityConfig) *Observability {
	obs := &Observability{
		Config: config,
	}

	if config.Metrics.Enabled {
		obs.Metrics = NewMetrics(config.Metrics)
	}

	if config.Tracing.Enabled {
		obs.Tracer = NewTracer(config.Tracing)
	}

	obs.Logger = NewLogger(config.Logging)

	log.Info().Msg("Observability stack initialized")

	return obs
}

// Middleware returns the combined observability middleware
func (o *Observability) Middleware() func(http.Handler) http.Handler {
	return ObservabilityMiddleware(o.Tracer, o.Metrics)
}

// MetricsHandler returns the metrics HTTP handler
func (o *Observability) MetricsHandler() http.HandlerFunc {
	if o.Metrics != nil {
		return o.Metrics.Handler()
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# Metrics not enabled\n"))
	}
}

// Shutdown gracefully shuts down observability components
func (o *Observability) Shutdown(ctx context.Context) error {
	if o.Tracer != nil {
		return o.Tracer.Shutdown(ctx)
	}
	return nil
}
