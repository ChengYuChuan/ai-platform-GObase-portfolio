package observability

import (
	"context"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// LoggingConfig holds configuration for structured logging
type LoggingConfig struct {
	Level      string // debug, info, warn, error
	Format     string // json, pretty
	Output     string // stdout, stderr, file
	FilePath   string // Path when output is file
	TimeFormat string // Time format for logs
	// Include fields
	IncludeTimestamp bool
	IncludeCaller    bool
	IncludeHostname  bool
	// Sampling
	SamplingEnabled bool
	SamplingRate    int // Log every Nth message at debug level
}

// DefaultLoggingConfig returns sensible defaults
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		Level:            "info",
		Format:           "json",
		Output:           "stdout",
		TimeFormat:       time.RFC3339Nano,
		IncludeTimestamp: true,
		IncludeCaller:    false,
		IncludeHostname:  false,
		SamplingEnabled:  false,
		SamplingRate:     10,
	}
}

// Logger wraps zerolog with additional functionality
type Logger struct {
	config LoggingConfig
	logger zerolog.Logger
}

// NewLogger creates a new logger with the given configuration
func NewLogger(config LoggingConfig) *Logger {
	// Set global log level
	level, err := zerolog.ParseLevel(config.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Set time format
	zerolog.TimeFieldFormat = config.TimeFormat

	// Create output writer
	var output io.Writer
	switch config.Output {
	case "stderr":
		output = os.Stderr
	case "file":
		if config.FilePath != "" {
			file, err := os.OpenFile(config.FilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				output = os.Stdout
			} else {
				output = file
			}
		} else {
			output = os.Stdout
		}
	default:
		output = os.Stdout
	}

	// Format output
	if config.Format == "pretty" {
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: config.TimeFormat,
		}
	}

	// Build logger context
	logCtx := zerolog.New(output).With().Timestamp()

	if config.IncludeCaller {
		logCtx = logCtx.Caller()
	}

	if config.IncludeHostname {
		hostname, _ := os.Hostname()
		logCtx = logCtx.Str("hostname", hostname)
	}

	logger := logCtx.Logger()

	return &Logger{
		config: config,
		logger: logger,
	}
}

// WithContext returns a logger with trace context fields
func (l *Logger) WithContext(ctx context.Context) zerolog.Logger {
	span := SpanFromContext(ctx)
	if span == nil {
		return l.logger
	}

	return l.logger.With().
		Str("trace_id", span.Context.TraceID).
		Str("span_id", span.Context.SpanID).
		Logger()
}

// WithRequest returns a logger with HTTP request context
func (l *Logger) WithRequest(r *http.Request) zerolog.Logger {
	ctx := r.Context()
	logger := l.WithContext(ctx)

	// Add request-specific fields
	return logger.With().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Str("user_agent", r.UserAgent()).
		Logger()
}

// WithFields returns a logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) zerolog.Logger {
	logCtx := l.logger.With()
	for k, v := range fields {
		logCtx = logCtx.Interface(k, v)
	}
	return logCtx.Logger()
}

// Log returns the underlying logger
func (l *Logger) Log() zerolog.Logger {
	return l.logger
}

// RequestLogger provides request-scoped logging
type RequestLogger struct {
	logger  zerolog.Logger
	startTime time.Time
	fields  map[string]interface{}
}

// NewRequestLogger creates a new request-scoped logger
func NewRequestLogger(ctx context.Context, r *http.Request) *RequestLogger {
	span := SpanFromContext(ctx)

	logCtx := log.With().
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr)

	if span != nil {
		logCtx = logCtx.
			Str("trace_id", span.Context.TraceID).
			Str("span_id", span.Context.SpanID)
	}

	return &RequestLogger{
		logger:    logCtx.Logger(),
		startTime: time.Now(),
		fields:    make(map[string]interface{}),
	}
}

// SetField sets a field to be included in the final log
func (rl *RequestLogger) SetField(key string, value interface{}) {
	rl.fields[key] = value
}

// Debug logs a debug message
func (rl *RequestLogger) Debug(msg string) {
	rl.logger.Debug().Msg(msg)
}

// Info logs an info message
func (rl *RequestLogger) Info(msg string) {
	rl.logger.Info().Msg(msg)
}

// Warn logs a warning message
func (rl *RequestLogger) Warn(msg string) {
	rl.logger.Warn().Msg(msg)
}

// Error logs an error message
func (rl *RequestLogger) Error(err error, msg string) {
	rl.logger.Error().Err(err).Msg(msg)
}

// Finish logs the final request log with duration and status
func (rl *RequestLogger) Finish(statusCode int, responseSize int64) {
	duration := time.Since(rl.startTime)

	event := rl.logger.Info().
		Int("status", statusCode).
		Int64("response_size", responseSize).
		Dur("duration", duration)

	for k, v := range rl.fields {
		event = event.Interface(k, v)
	}

	event.Msg("Request completed")
}

// LogEvent represents a structured log event
type LogEvent struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Timestamp time.Time              `json:"timestamp"`
	TraceID   string                 `json:"trace_id,omitempty"`
	SpanID    string                 `json:"span_id,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// ContextLogger provides context-aware logging functions
type ContextLogger struct{}

// Debug logs a debug message with context
func (cl *ContextLogger) Debug(ctx context.Context, msg string, fields ...map[string]interface{}) {
	event := log.Debug()
	cl.addTraceContext(ctx, event)
	cl.addFields(event, fields...)
	event.Msg(msg)
}

// Info logs an info message with context
func (cl *ContextLogger) Info(ctx context.Context, msg string, fields ...map[string]interface{}) {
	event := log.Info()
	cl.addTraceContext(ctx, event)
	cl.addFields(event, fields...)
	event.Msg(msg)
}

// Warn logs a warning message with context
func (cl *ContextLogger) Warn(ctx context.Context, msg string, fields ...map[string]interface{}) {
	event := log.Warn()
	cl.addTraceContext(ctx, event)
	cl.addFields(event, fields...)
	event.Msg(msg)
}

// Error logs an error message with context
func (cl *ContextLogger) Error(ctx context.Context, err error, msg string, fields ...map[string]interface{}) {
	event := log.Error().Err(err)
	cl.addTraceContext(ctx, event)
	cl.addFields(event, fields...)
	event.Msg(msg)
}

func (cl *ContextLogger) addTraceContext(ctx context.Context, event *zerolog.Event) {
	if span := SpanFromContext(ctx); span != nil {
		event.Str("trace_id", span.Context.TraceID)
		event.Str("span_id", span.Context.SpanID)
	}
}

func (cl *ContextLogger) addFields(event *zerolog.Event, fields ...map[string]interface{}) {
	for _, f := range fields {
		for k, v := range f {
			event.Interface(k, v)
		}
	}
}

// LogWithTrace is a helper to log with trace context
func LogWithTrace(ctx context.Context) zerolog.Logger {
	if span := SpanFromContext(ctx); span != nil {
		return log.With().
			Str("trace_id", span.Context.TraceID).
			Str("span_id", span.Context.SpanID).
			Logger()
	}
	return log.Logger
}

// Global context logger instance
var CtxLog = &ContextLogger{}

// AuditLog represents an audit log entry
type AuditLog struct {
	Timestamp   time.Time              `json:"timestamp"`
	Action      string                 `json:"action"`
	Actor       string                 `json:"actor,omitempty"`
	Resource    string                 `json:"resource"`
	ResourceID  string                 `json:"resource_id,omitempty"`
	TraceID     string                 `json:"trace_id,omitempty"`
	Status      string                 `json:"status"` // success, failure
	Details     map[string]interface{} `json:"details,omitempty"`
}

// LogAudit writes an audit log entry
func LogAudit(ctx context.Context, action, resource string, details map[string]interface{}) {
	event := log.Info().
		Str("log_type", "audit").
		Str("action", action).
		Str("resource", resource)

	if span := SpanFromContext(ctx); span != nil {
		event.Str("trace_id", span.Context.TraceID)
	}

	if details != nil {
		for k, v := range details {
			event.Interface(k, v)
		}
	}

	event.Msg("Audit log")
}

// LogProviderRequest logs a provider API request
func LogProviderRequest(ctx context.Context, provider, operation, model string, duration time.Duration, err error) {
	event := log.Info().
		Str("log_type", "provider_request").
		Str("provider", provider).
		Str("operation", operation).
		Str("model", model).
		Dur("duration", duration)

	if span := SpanFromContext(ctx); span != nil {
		event.Str("trace_id", span.Context.TraceID)
		event.Str("span_id", span.Context.SpanID)
	}

	if err != nil {
		event.Err(err).Str("status", "error")
	} else {
		event.Str("status", "success")
	}

	event.Msg("Provider request")
}

// LogTokenUsage logs token usage for billing/monitoring
func LogTokenUsage(ctx context.Context, provider, model string, promptTokens, completionTokens int) {
	event := log.Info().
		Str("log_type", "token_usage").
		Str("provider", provider).
		Str("model", model).
		Int("prompt_tokens", promptTokens).
		Int("completion_tokens", completionTokens).
		Int("total_tokens", promptTokens+completionTokens)

	if span := SpanFromContext(ctx); span != nil {
		event.Str("trace_id", span.Context.TraceID)
	}

	event.Msg("Token usage")
}
