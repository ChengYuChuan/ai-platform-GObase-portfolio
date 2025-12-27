package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// TracingConfig holds configuration for distributed tracing
type TracingConfig struct {
	Enabled      bool
	ServiceName  string
	SamplingRate float64 // 0.0 to 1.0
	// Exporter configuration (placeholder for OTLP/Jaeger/Zipkin)
	ExporterType    string // "console", "otlp", "jaeger", "zipkin"
	ExporterAddress string
}

// DefaultTracingConfig returns sensible defaults
func DefaultTracingConfig() TracingConfig {
	return TracingConfig{
		Enabled:      true,
		ServiceName:  "llm-gateway",
		SamplingRate: 1.0, // Sample everything in dev
		ExporterType: "console",
	}
}

// SpanContext holds the context for a span
type SpanContext struct {
	TraceID  string
	SpanID   string
	ParentID string
	Sampled  bool
}

// Span represents a unit of work
type Span struct {
	mu         sync.Mutex
	Name       string
	Context    SpanContext
	StartTime  time.Time
	EndTime    time.Time
	Status     SpanStatus
	Attributes map[string]interface{}
	Events     []SpanEvent
	tracer     *Tracer
	ended      bool
}

// SpanStatus represents the status of a span
type SpanStatus struct {
	Code    StatusCode
	Message string
}

// StatusCode represents span status
type StatusCode int

const (
	StatusUnset StatusCode = iota
	StatusOK
	StatusError
)

// SpanEvent represents an event within a span
type SpanEvent struct {
	Name       string
	Timestamp  time.Time
	Attributes map[string]interface{}
}

// SetAttribute sets a span attribute
func (s *Span) SetAttribute(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Attributes == nil {
		s.Attributes = make(map[string]interface{})
	}
	s.Attributes[key] = value
}

// SetStatus sets the span status
func (s *Span) SetStatus(code StatusCode, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Status = SpanStatus{Code: code, Message: message}
}

// AddEvent adds an event to the span
func (s *Span) AddEvent(name string, attrs map[string]interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Events = append(s.Events, SpanEvent{
		Name:       name,
		Timestamp:  time.Now(),
		Attributes: attrs,
	})
}

// End ends the span and exports it
func (s *Span) End() {
	s.mu.Lock()
	if s.ended {
		s.mu.Unlock()
		return
	}
	s.ended = true
	s.EndTime = time.Now()
	s.mu.Unlock()

	if s.tracer != nil && s.Context.Sampled {
		s.tracer.export(s)
	}
}

// Duration returns the span duration
func (s *Span) Duration() time.Duration {
	if s.EndTime.IsZero() {
		return time.Since(s.StartTime)
	}
	return s.EndTime.Sub(s.StartTime)
}

// Tracer creates and manages spans
type Tracer struct {
	config   TracingConfig
	exporter SpanExporter
	mu       sync.RWMutex
	spans    []*Span // Buffer for batch export
}

// SpanExporter exports spans to a backend
type SpanExporter interface {
	Export(spans []*Span) error
	Shutdown(ctx context.Context) error
}

// ConsoleExporter exports spans to console/log
type ConsoleExporter struct{}

func (e *ConsoleExporter) Export(spans []*Span) error {
	for _, span := range spans {
		log.Debug().
			Str("trace_id", span.Context.TraceID).
			Str("span_id", span.Context.SpanID).
			Str("parent_id", span.Context.ParentID).
			Str("name", span.Name).
			Dur("duration", span.Duration()).
			Int("status", int(span.Status.Code)).
			Interface("attributes", span.Attributes).
			Msg("Span exported")
	}
	return nil
}

func (e *ConsoleExporter) Shutdown(ctx context.Context) error {
	return nil
}

var (
	globalTracer *Tracer
	tracerOnce   sync.Once
)

// NewTracer creates a new tracer
func NewTracer(config TracingConfig) *Tracer {
	var exporter SpanExporter

	switch config.ExporterType {
	case "otlp":
		// Placeholder for OTLP exporter
		log.Warn().Msg("OTLP exporter not yet implemented, falling back to console")
		exporter = &ConsoleExporter{}
	case "jaeger":
		// Placeholder for Jaeger exporter
		log.Warn().Msg("Jaeger exporter not yet implemented, falling back to console")
		exporter = &ConsoleExporter{}
	case "console":
		fallthrough
	default:
		exporter = &ConsoleExporter{}
	}

	tracer := &Tracer{
		config:   config,
		exporter: exporter,
		spans:    make([]*Span, 0, 100),
	}

	log.Info().
		Str("service_name", config.ServiceName).
		Float64("sampling_rate", config.SamplingRate).
		Str("exporter", config.ExporterType).
		Msg("Tracer initialized")

	return tracer
}

// InitGlobalTracer initializes the global tracer
func InitGlobalTracer(config TracingConfig) *Tracer {
	tracerOnce.Do(func() {
		globalTracer = NewTracer(config)
	})
	return globalTracer
}

// GetTracer returns the global tracer
func GetTracer() *Tracer {
	if globalTracer == nil {
		globalTracer = NewTracer(DefaultTracingConfig())
	}
	return globalTracer
}

// StartSpan starts a new span
func (t *Tracer) StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	if !t.config.Enabled {
		return ctx, &Span{Name: name, tracer: nil}
	}

	// Get parent span context from context
	parentCtx := SpanFromContext(ctx)

	spanCtx := SpanContext{
		TraceID: generateTraceID(),
		SpanID:  generateSpanID(),
		Sampled: t.shouldSample(),
	}

	// Inherit trace ID from parent
	if parentCtx != nil {
		spanCtx.TraceID = parentCtx.Context.TraceID
		spanCtx.ParentID = parentCtx.Context.SpanID
		spanCtx.Sampled = parentCtx.Context.Sampled
	}

	span := &Span{
		Name:       name,
		Context:    spanCtx,
		StartTime:  time.Now(),
		Attributes: make(map[string]interface{}),
		tracer:     t,
	}

	// Add service name attribute
	span.SetAttribute("service.name", t.config.ServiceName)

	return ContextWithSpan(ctx, span), span
}

// StartSpanFromHTTP extracts trace context from HTTP headers
func (t *Tracer) StartSpanFromHTTP(r *http.Request, name string) (context.Context, *Span) {
	ctx := r.Context()

	// Try to extract W3C Trace Context headers
	traceParent := r.Header.Get("traceparent")

	span := &Span{
		Name:       name,
		StartTime:  time.Now(),
		Attributes: make(map[string]interface{}),
		tracer:     t,
	}

	if traceParent != "" {
		// Parse W3C traceparent header: version-traceid-parentid-flags
		// Format: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
		if parsed := parseTraceParent(traceParent); parsed != nil {
			span.Context = *parsed
		}
	}

	// Generate new IDs if not from parent
	if span.Context.TraceID == "" {
		span.Context.TraceID = generateTraceID()
		span.Context.SpanID = generateSpanID()
		span.Context.Sampled = t.shouldSample()
	} else {
		// New span ID, keep trace ID
		span.Context.ParentID = span.Context.SpanID
		span.Context.SpanID = generateSpanID()
	}

	// Add HTTP attributes
	span.SetAttribute("http.method", r.Method)
	span.SetAttribute("http.url", r.URL.String())
	span.SetAttribute("http.user_agent", r.UserAgent())
	span.SetAttribute("service.name", t.config.ServiceName)

	return ContextWithSpan(ctx, span), span
}

// InjectHTTP injects trace context into HTTP headers
func (t *Tracer) InjectHTTP(ctx context.Context, req *http.Request) {
	span := SpanFromContext(ctx)
	if span == nil || !span.Context.Sampled {
		return
	}

	// W3C Trace Context format
	traceParent := "00-" + span.Context.TraceID + "-" + span.Context.SpanID + "-01"
	req.Header.Set("traceparent", traceParent)
}

func (t *Tracer) shouldSample() bool {
	if t.config.SamplingRate >= 1.0 {
		return true
	}
	if t.config.SamplingRate <= 0.0 {
		return false
	}

	b := make([]byte, 1)
	rand.Read(b)
	return float64(b[0])/255.0 < t.config.SamplingRate
}

func (t *Tracer) export(span *Span) {
	t.mu.Lock()
	t.spans = append(t.spans, span)

	// Batch export when buffer is full
	if len(t.spans) >= 100 {
		spans := t.spans
		t.spans = make([]*Span, 0, 100)
		t.mu.Unlock()

		go t.exporter.Export(spans)
		return
	}
	t.mu.Unlock()
}

// Flush exports all buffered spans
func (t *Tracer) Flush() error {
	t.mu.Lock()
	spans := t.spans
	t.spans = make([]*Span, 0, 100)
	t.mu.Unlock()

	if len(spans) > 0 {
		return t.exporter.Export(spans)
	}
	return nil
}

// Shutdown flushes and shuts down the tracer
func (t *Tracer) Shutdown(ctx context.Context) error {
	if err := t.Flush(); err != nil {
		return err
	}
	return t.exporter.Shutdown(ctx)
}

// Context helpers

type spanContextKey struct{}

// ContextWithSpan returns a new context with the span attached
func ContextWithSpan(ctx context.Context, span *Span) context.Context {
	return context.WithValue(ctx, spanContextKey{}, span)
}

// SpanFromContext returns the span from context, or nil if not present
func SpanFromContext(ctx context.Context) *Span {
	if span, ok := ctx.Value(spanContextKey{}).(*Span); ok {
		return span
	}
	return nil
}

// Helper functions

func generateTraceID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateSpanID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func parseTraceParent(header string) *SpanContext {
	// Simple parser for W3C traceparent header
	// Format: version-traceid-parentid-flags
	// Example: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01

	if len(header) != 55 {
		return nil
	}

	if header[2] != '-' || header[35] != '-' || header[52] != '-' {
		return nil
	}

	version := header[0:2]
	if version != "00" {
		return nil // Only support version 00
	}

	return &SpanContext{
		TraceID:  header[3:35],
		SpanID:   header[36:52],
		Sampled:  header[53:55] == "01",
	}
}

// TraceID returns the trace ID from context for logging
func TraceID(ctx context.Context) string {
	if span := SpanFromContext(ctx); span != nil {
		return span.Context.TraceID
	}
	return ""
}

// SpanID returns the span ID from context for logging
func SpanID(ctx context.Context) string {
	if span := SpanFromContext(ctx); span != nil {
		return span.Context.SpanID
	}
	return ""
}
