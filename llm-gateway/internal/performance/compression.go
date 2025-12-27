package performance

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
)

// CompressionConfig holds configuration for response compression
type CompressionConfig struct {
	// Enabled controls whether compression is active
	Enabled bool
	// Level is the gzip compression level (1-9, where 9 is best compression)
	Level int
	// MinSize is the minimum response size in bytes to trigger compression
	MinSize int
	// ContentTypes are the content types that should be compressed
	ContentTypes []string
}

// DefaultCompressionConfig returns sensible defaults
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Enabled: true,
		Level:   gzip.DefaultCompression,
		MinSize: 1024, // 1KB minimum
		ContentTypes: []string{
			"application/json",
			"text/plain",
			"text/html",
			"text/css",
			"text/javascript",
			"application/javascript",
			"text/event-stream", // SSE
		},
	}
}

// gzipResponseWriter wraps http.ResponseWriter with gzip compression
type gzipResponseWriter struct {
	http.ResponseWriter
	writer      *gzip.Writer
	wroteHeader bool
	config      CompressionConfig
	shouldGzip  bool
	buffered    []byte
}

// gzipWriterPool reduces allocations by reusing gzip writers
var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// getGzipWriter gets a gzip writer from the pool
func getGzipWriter(w io.Writer, level int) *gzip.Writer {
	gz := gzipWriterPool.Get().(*gzip.Writer)
	gz.Reset(w)
	return gz
}

// putGzipWriter returns a gzip writer to the pool
func putGzipWriter(gz *gzip.Writer) {
	gz.Reset(io.Discard)
	gzipWriterPool.Put(gz)
}

func newGzipResponseWriter(w http.ResponseWriter, config CompressionConfig) *gzipResponseWriter {
	return &gzipResponseWriter{
		ResponseWriter: w,
		config:         config,
	}
}

func (g *gzipResponseWriter) WriteHeader(code int) {
	if g.wroteHeader {
		return
	}
	g.wroteHeader = true

	// Check if we should compress based on content type
	contentType := g.Header().Get("Content-Type")
	g.shouldGzip = g.shouldCompress(contentType)

	if g.shouldGzip {
		g.Header().Set("Content-Encoding", "gzip")
		g.Header().Del("Content-Length") // Length will change
		g.Header().Add("Vary", "Accept-Encoding")
	}

	g.ResponseWriter.WriteHeader(code)
}

func (g *gzipResponseWriter) Write(b []byte) (int, error) {
	if !g.wroteHeader {
		// Buffer until we know content type
		g.buffered = append(g.buffered, b...)

		// Check if we have enough to make a decision
		if len(g.buffered) >= g.config.MinSize {
			g.WriteHeader(http.StatusOK)
			return g.writeBuffered()
		}
		return len(b), nil
	}

	if g.shouldGzip && g.writer == nil {
		g.writer = getGzipWriter(g.ResponseWriter, g.config.Level)
	}

	if g.writer != nil {
		return g.writer.Write(b)
	}
	return g.ResponseWriter.Write(b)
}

func (g *gzipResponseWriter) writeBuffered() (int, error) {
	if len(g.buffered) == 0 {
		return 0, nil
	}

	// Check min size requirement
	if len(g.buffered) < g.config.MinSize {
		g.shouldGzip = false
	}

	if g.shouldGzip && g.writer == nil {
		g.writer = getGzipWriter(g.ResponseWriter, g.config.Level)
	}

	data := g.buffered
	g.buffered = nil

	if g.writer != nil {
		return g.writer.Write(data)
	}
	return g.ResponseWriter.Write(data)
}

func (g *gzipResponseWriter) shouldCompress(contentType string) bool {
	if contentType == "" {
		return false
	}

	// Extract base content type (remove charset, etc.)
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = strings.TrimSpace(contentType[:idx])
	}

	for _, ct := range g.config.ContentTypes {
		if strings.EqualFold(contentType, ct) {
			return true
		}
	}
	return false
}

// Flush implements http.Flusher for streaming support
func (g *gzipResponseWriter) Flush() {
	// Flush buffered data first
	if len(g.buffered) > 0 {
		g.WriteHeader(http.StatusOK)
		g.writeBuffered()
	}

	if g.writer != nil {
		g.writer.Flush()
	}

	if flusher, ok := g.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Close flushes and closes the gzip writer
func (g *gzipResponseWriter) Close() error {
	// Write any remaining buffered data
	if len(g.buffered) > 0 {
		g.WriteHeader(http.StatusOK)
		g.writeBuffered()
	}

	if g.writer != nil {
		err := g.writer.Close()
		putGzipWriter(g.writer)
		g.writer = nil
		return err
	}
	return nil
}

// Hijack implements http.Hijacker for WebSocket support
func (g *gzipResponseWriter) Hijack() (interface{}, interface{}, error) {
	if hijacker, ok := g.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// Push implements http.Pusher for HTTP/2 push support
func (g *gzipResponseWriter) Push(target string, opts *http.PushOptions) error {
	if pusher, ok := g.ResponseWriter.(http.Pusher); ok {
		return pusher.Push(target, opts)
	}
	return http.ErrNotSupported
}

// CompressionMiddleware returns a middleware that compresses responses
func CompressionMiddleware(config CompressionConfig) func(http.Handler) http.Handler {
	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	log.Info().
		Int("level", config.Level).
		Int("min_size", config.MinSize).
		Msg("Response compression middleware enabled")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if client accepts gzip encoding
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			// Skip compression for SSE (streaming) - it needs special handling
			if r.Header.Get("Accept") == "text/event-stream" {
				// For SSE, we skip gzip as it interferes with real-time streaming
				next.ServeHTTP(w, r)
				return
			}

			// Create gzip response writer
			gzw := newGzipResponseWriter(w, config)
			defer gzw.Close()

			next.ServeHTTP(gzw, r)
		})
	}
}

// StreamingCompressionMiddleware handles compression for streaming responses
// This is separate because SSE needs different handling
func StreamingCompressionMiddleware(config CompressionConfig) func(http.Handler) http.Handler {
	if !config.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this is a streaming request that wants compression
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			isStreaming := r.Header.Get("Accept") == "text/event-stream" ||
				strings.Contains(r.URL.Path, "/stream")

			if !isStreaming {
				// Use regular compression
				gzw := newGzipResponseWriter(w, config)
				defer gzw.Close()
				next.ServeHTTP(gzw, r)
				return
			}

			// For streaming, use a flushing gzip writer
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Transfer-Encoding", "chunked")
			w.Header().Add("Vary", "Accept-Encoding")

			gz := getGzipWriter(w, config.Level)
			defer func() {
				gz.Close()
				putGzipWriter(gz)
			}()

			// Create a streaming writer that flushes after each write
			sw := &streamingGzipWriter{
				ResponseWriter: w,
				gzip:           gz,
			}

			next.ServeHTTP(sw, r)
		})
	}
}

// streamingGzipWriter handles gzip compression for SSE/streaming
type streamingGzipWriter struct {
	http.ResponseWriter
	gzip        *gzip.Writer
	wroteHeader bool
}

func (s *streamingGzipWriter) WriteHeader(code int) {
	if s.wroteHeader {
		return
	}
	s.wroteHeader = true
	s.ResponseWriter.WriteHeader(code)
}

func (s *streamingGzipWriter) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.WriteHeader(http.StatusOK)
	}

	n, err := s.gzip.Write(b)
	if err != nil {
		return n, err
	}

	// Flush immediately for streaming
	if err := s.gzip.Flush(); err != nil {
		return n, err
	}

	if flusher, ok := s.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}

	return n, nil
}

func (s *streamingGzipWriter) Flush() {
	s.gzip.Flush()
	if flusher, ok := s.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
