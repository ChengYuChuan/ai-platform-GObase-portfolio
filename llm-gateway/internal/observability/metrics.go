package observability

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// MetricsConfig holds configuration for metrics collection
type MetricsConfig struct {
	Enabled      bool
	Path         string
	Namespace    string
	Subsystem    string
	HistogramBuckets []float64
}

// DefaultMetricsConfig returns sensible defaults
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		Enabled:   true,
		Path:      "/metrics",
		Namespace: "llm_gateway",
		Subsystem: "http",
		HistogramBuckets: []float64{
			0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60,
		},
	}
}

// Counter represents a monotonically increasing counter
type Counter struct {
	mu    sync.RWMutex
	value int64
}

func (c *Counter) Inc() {
	c.mu.Lock()
	c.value++
	c.mu.Unlock()
}

func (c *Counter) Add(n int64) {
	c.mu.Lock()
	c.value += n
	c.mu.Unlock()
}

func (c *Counter) Value() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.value
}

// Gauge represents a value that can go up or down
type Gauge struct {
	mu    sync.RWMutex
	value float64
}

func (g *Gauge) Set(v float64) {
	g.mu.Lock()
	g.value = v
	g.mu.Unlock()
}

func (g *Gauge) Inc() {
	g.mu.Lock()
	g.value++
	g.mu.Unlock()
}

func (g *Gauge) Dec() {
	g.mu.Lock()
	g.value--
	g.mu.Unlock()
}

func (g *Gauge) Add(v float64) {
	g.mu.Lock()
	g.value += v
	g.mu.Unlock()
}

func (g *Gauge) Value() float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.value
}

// Histogram tracks distribution of values
type Histogram struct {
	mu      sync.RWMutex
	buckets []float64
	counts  []int64
	sum     float64
	count   int64
}

func NewHistogram(buckets []float64) *Histogram {
	return &Histogram{
		buckets: buckets,
		counts:  make([]int64, len(buckets)+1), // +1 for +Inf bucket
	}
}

func (h *Histogram) Observe(v float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sum += v
	h.count++

	for i, bucket := range h.buckets {
		if v <= bucket {
			h.counts[i]++
			return
		}
	}
	// +Inf bucket
	h.counts[len(h.buckets)]++
}

func (h *Histogram) Values() (buckets []float64, counts []int64, sum float64, count int64) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	bucketsCopy := make([]float64, len(h.buckets))
	countsCopy := make([]int64, len(h.counts))
	copy(bucketsCopy, h.buckets)
	copy(countsCopy, h.counts)

	return bucketsCopy, countsCopy, h.sum, h.count
}

// LabeledCounter is a counter with labels
type LabeledCounter struct {
	mu       sync.RWMutex
	counters map[string]*Counter
}

func NewLabeledCounter() *LabeledCounter {
	return &LabeledCounter{
		counters: make(map[string]*Counter),
	}
}

func (lc *LabeledCounter) WithLabels(labels map[string]string) *Counter {
	key := labelsToKey(labels)

	lc.mu.Lock()
	defer lc.mu.Unlock()

	if c, ok := lc.counters[key]; ok {
		return c
	}

	c := &Counter{}
	lc.counters[key] = c
	return c
}

func (lc *LabeledCounter) All() map[string]*Counter {
	lc.mu.RLock()
	defer lc.mu.RUnlock()

	result := make(map[string]*Counter, len(lc.counters))
	for k, v := range lc.counters {
		result[k] = v
	}
	return result
}

// LabeledHistogram is a histogram with labels
type LabeledHistogram struct {
	mu         sync.RWMutex
	histograms map[string]*Histogram
	buckets    []float64
}

func NewLabeledHistogram(buckets []float64) *LabeledHistogram {
	return &LabeledHistogram{
		histograms: make(map[string]*Histogram),
		buckets:    buckets,
	}
}

func (lh *LabeledHistogram) WithLabels(labels map[string]string) *Histogram {
	key := labelsToKey(labels)

	lh.mu.Lock()
	defer lh.mu.Unlock()

	if h, ok := lh.histograms[key]; ok {
		return h
	}

	h := NewHistogram(lh.buckets)
	lh.histograms[key] = h
	return h
}

func (lh *LabeledHistogram) All() map[string]*Histogram {
	lh.mu.RLock()
	defer lh.mu.RUnlock()

	result := make(map[string]*Histogram, len(lh.histograms))
	for k, v := range lh.histograms {
		result[k] = v
	}
	return result
}

func labelsToKey(labels map[string]string) string {
	// Simple label encoding for map key
	key := ""
	for k, v := range labels {
		key += k + "=" + v + ","
	}
	return key
}

// Metrics holds all application metrics
type Metrics struct {
	config MetricsConfig

	// HTTP metrics
	RequestsTotal      *LabeledCounter
	RequestDuration    *LabeledHistogram
	RequestsInFlight   *Gauge
	ResponseSizeBytes  *LabeledHistogram

	// Provider metrics
	ProviderRequestsTotal   *LabeledCounter
	ProviderRequestDuration *LabeledHistogram
	ProviderErrors          *LabeledCounter

	// Circuit breaker metrics
	CircuitBreakerState   *LabeledCounter // state changes
	CircuitBreakerOpen    *LabeledCounter

	// Rate limiter metrics
	RateLimitedRequests *LabeledCounter

	// Cache metrics
	CacheHits   *LabeledCounter
	CacheMisses *LabeledCounter

	// Token usage metrics
	TokensPrompt     *LabeledCounter
	TokensCompletion *LabeledCounter
	TokensTotal      *LabeledCounter
}

var (
	globalMetrics *Metrics
	metricsOnce   sync.Once
)

// NewMetrics creates a new metrics instance
func NewMetrics(config MetricsConfig) *Metrics {
	buckets := config.HistogramBuckets
	if len(buckets) == 0 {
		buckets = DefaultMetricsConfig().HistogramBuckets
	}

	m := &Metrics{
		config: config,

		// HTTP metrics
		RequestsTotal:     NewLabeledCounter(),
		RequestDuration:   NewLabeledHistogram(buckets),
		RequestsInFlight:  &Gauge{},
		ResponseSizeBytes: NewLabeledHistogram([]float64{100, 1000, 10000, 100000, 1000000}),

		// Provider metrics
		ProviderRequestsTotal:   NewLabeledCounter(),
		ProviderRequestDuration: NewLabeledHistogram(buckets),
		ProviderErrors:          NewLabeledCounter(),

		// Circuit breaker metrics
		CircuitBreakerState: NewLabeledCounter(),
		CircuitBreakerOpen:  NewLabeledCounter(),

		// Rate limiter metrics
		RateLimitedRequests: NewLabeledCounter(),

		// Cache metrics
		CacheHits:   NewLabeledCounter(),
		CacheMisses: NewLabeledCounter(),

		// Token metrics
		TokensPrompt:     NewLabeledCounter(),
		TokensCompletion: NewLabeledCounter(),
		TokensTotal:      NewLabeledCounter(),
	}

	log.Info().
		Str("namespace", config.Namespace).
		Str("path", config.Path).
		Msg("Metrics collector initialized")

	return m
}

// InitGlobalMetrics initializes the global metrics instance
func InitGlobalMetrics(config MetricsConfig) *Metrics {
	metricsOnce.Do(func() {
		globalMetrics = NewMetrics(config)
	})
	return globalMetrics
}

// GetMetrics returns the global metrics instance
func GetMetrics() *Metrics {
	if globalMetrics == nil {
		globalMetrics = NewMetrics(DefaultMetricsConfig())
	}
	return globalMetrics
}

// RecordRequest records an HTTP request
func (m *Metrics) RecordRequest(method, path string, statusCode int, duration time.Duration, responseSize int64) {
	labels := map[string]string{
		"method": method,
		"path":   path,
		"status": strconv.Itoa(statusCode),
	}

	m.RequestsTotal.WithLabels(labels).Inc()
	m.RequestDuration.WithLabels(labels).Observe(duration.Seconds())
	m.ResponseSizeBytes.WithLabels(labels).Observe(float64(responseSize))
}

// RecordProviderRequest records a provider API call
func (m *Metrics) RecordProviderRequest(provider, operation string, success bool, duration time.Duration) {
	labels := map[string]string{
		"provider":  provider,
		"operation": operation,
		"success":   strconv.FormatBool(success),
	}

	m.ProviderRequestsTotal.WithLabels(labels).Inc()
	m.ProviderRequestDuration.WithLabels(labels).Observe(duration.Seconds())

	if !success {
		m.ProviderErrors.WithLabels(map[string]string{
			"provider":  provider,
			"operation": operation,
		}).Inc()
	}
}

// RecordCircuitBreakerStateChange records circuit breaker state changes
func (m *Metrics) RecordCircuitBreakerStateChange(provider, fromState, toState string) {
	m.CircuitBreakerState.WithLabels(map[string]string{
		"provider":   provider,
		"from_state": fromState,
		"to_state":   toState,
	}).Inc()

	if toState == "open" {
		m.CircuitBreakerOpen.WithLabels(map[string]string{
			"provider": provider,
		}).Inc()
	}
}

// RecordRateLimited records a rate-limited request
func (m *Metrics) RecordRateLimited(clientID string) {
	m.RateLimitedRequests.WithLabels(map[string]string{
		"client_id": clientID,
	}).Inc()
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit(model string) {
	m.CacheHits.WithLabels(map[string]string{
		"model": model,
	}).Inc()
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss(model string) {
	m.CacheMisses.WithLabels(map[string]string{
		"model": model,
	}).Inc()
}

// RecordTokenUsage records token usage
func (m *Metrics) RecordTokenUsage(provider, model string, promptTokens, completionTokens int) {
	labels := map[string]string{
		"provider": provider,
		"model":    model,
	}

	m.TokensPrompt.WithLabels(labels).Add(int64(promptTokens))
	m.TokensCompletion.WithLabels(labels).Add(int64(completionTokens))
	m.TokensTotal.WithLabels(labels).Add(int64(promptTokens + completionTokens))
}

// Handler returns an HTTP handler for metrics endpoint
func (m *Metrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)

		// Write metrics in Prometheus exposition format
		m.writePrometheusMetrics(w)
	}
}

func (m *Metrics) writePrometheusMetrics(w http.ResponseWriter) {
	ns := m.config.Namespace
	ss := m.config.Subsystem

	// HTTP Request metrics
	w.Write([]byte("# HELP " + ns + "_" + ss + "_requests_total Total number of HTTP requests\n"))
	w.Write([]byte("# TYPE " + ns + "_" + ss + "_requests_total counter\n"))
	for key, counter := range m.RequestsTotal.All() {
		w.Write([]byte(ns + "_" + ss + "_requests_total{" + key + "} " + strconv.FormatInt(counter.Value(), 10) + "\n"))
	}

	w.Write([]byte("\n# HELP " + ns + "_" + ss + "_request_duration_seconds HTTP request duration in seconds\n"))
	w.Write([]byte("# TYPE " + ns + "_" + ss + "_request_duration_seconds histogram\n"))
	for key, hist := range m.RequestDuration.All() {
		buckets, counts, sum, count := hist.Values()
		cumulative := int64(0)
		for i, bucket := range buckets {
			cumulative += counts[i]
			w.Write([]byte(ns + "_" + ss + "_request_duration_seconds_bucket{" + key + "le=\"" + strconv.FormatFloat(bucket, 'f', 3, 64) + "\"} " + strconv.FormatInt(cumulative, 10) + "\n"))
		}
		cumulative += counts[len(buckets)]
		w.Write([]byte(ns + "_" + ss + "_request_duration_seconds_bucket{" + key + "le=\"+Inf\"} " + strconv.FormatInt(cumulative, 10) + "\n"))
		w.Write([]byte(ns + "_" + ss + "_request_duration_seconds_sum{" + key + "} " + strconv.FormatFloat(sum, 'f', 6, 64) + "\n"))
		w.Write([]byte(ns + "_" + ss + "_request_duration_seconds_count{" + key + "} " + strconv.FormatInt(count, 10) + "\n"))
	}

	w.Write([]byte("\n# HELP " + ns + "_" + ss + "_requests_in_flight Current number of requests in flight\n"))
	w.Write([]byte("# TYPE " + ns + "_" + ss + "_requests_in_flight gauge\n"))
	w.Write([]byte(ns + "_" + ss + "_requests_in_flight " + strconv.FormatFloat(m.RequestsInFlight.Value(), 'f', 0, 64) + "\n"))

	// Provider metrics
	w.Write([]byte("\n# HELP " + ns + "_provider_requests_total Total number of provider API requests\n"))
	w.Write([]byte("# TYPE " + ns + "_provider_requests_total counter\n"))
	for key, counter := range m.ProviderRequestsTotal.All() {
		w.Write([]byte(ns + "_provider_requests_total{" + key + "} " + strconv.FormatInt(counter.Value(), 10) + "\n"))
	}

	w.Write([]byte("\n# HELP " + ns + "_provider_errors_total Total number of provider errors\n"))
	w.Write([]byte("# TYPE " + ns + "_provider_errors_total counter\n"))
	for key, counter := range m.ProviderErrors.All() {
		w.Write([]byte(ns + "_provider_errors_total{" + key + "} " + strconv.FormatInt(counter.Value(), 10) + "\n"))
	}

	// Circuit breaker metrics
	w.Write([]byte("\n# HELP " + ns + "_circuit_breaker_state_changes_total Circuit breaker state changes\n"))
	w.Write([]byte("# TYPE " + ns + "_circuit_breaker_state_changes_total counter\n"))
	for key, counter := range m.CircuitBreakerState.All() {
		w.Write([]byte(ns + "_circuit_breaker_state_changes_total{" + key + "} " + strconv.FormatInt(counter.Value(), 10) + "\n"))
	}

	// Rate limiter metrics
	w.Write([]byte("\n# HELP " + ns + "_rate_limited_requests_total Total number of rate-limited requests\n"))
	w.Write([]byte("# TYPE " + ns + "_rate_limited_requests_total counter\n"))
	for key, counter := range m.RateLimitedRequests.All() {
		w.Write([]byte(ns + "_rate_limited_requests_total{" + key + "} " + strconv.FormatInt(counter.Value(), 10) + "\n"))
	}

	// Cache metrics
	w.Write([]byte("\n# HELP " + ns + "_cache_hits_total Cache hits\n"))
	w.Write([]byte("# TYPE " + ns + "_cache_hits_total counter\n"))
	for key, counter := range m.CacheHits.All() {
		w.Write([]byte(ns + "_cache_hits_total{" + key + "} " + strconv.FormatInt(counter.Value(), 10) + "\n"))
	}

	w.Write([]byte("\n# HELP " + ns + "_cache_misses_total Cache misses\n"))
	w.Write([]byte("# TYPE " + ns + "_cache_misses_total counter\n"))
	for key, counter := range m.CacheMisses.All() {
		w.Write([]byte(ns + "_cache_misses_total{" + key + "} " + strconv.FormatInt(counter.Value(), 10) + "\n"))
	}

	// Token usage metrics
	w.Write([]byte("\n# HELP " + ns + "_tokens_prompt_total Total prompt tokens used\n"))
	w.Write([]byte("# TYPE " + ns + "_tokens_prompt_total counter\n"))
	for key, counter := range m.TokensPrompt.All() {
		w.Write([]byte(ns + "_tokens_prompt_total{" + key + "} " + strconv.FormatInt(counter.Value(), 10) + "\n"))
	}

	w.Write([]byte("\n# HELP " + ns + "_tokens_completion_total Total completion tokens used\n"))
	w.Write([]byte("# TYPE " + ns + "_tokens_completion_total counter\n"))
	for key, counter := range m.TokensCompletion.All() {
		w.Write([]byte(ns + "_tokens_completion_total{" + key + "} " + strconv.FormatInt(counter.Value(), 10) + "\n"))
	}

	w.Write([]byte("\n# HELP " + ns + "_tokens_total Total tokens used\n"))
	w.Write([]byte("# TYPE " + ns + "_tokens_total counter\n"))
	for key, counter := range m.TokensTotal.All() {
		w.Write([]byte(ns + "_tokens_total{" + key + "} " + strconv.FormatInt(counter.Value(), 10) + "\n"))
	}
}

// GetStats returns metrics as a map for JSON endpoints
func (m *Metrics) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"requests_in_flight": m.RequestsInFlight.Value(),
	}

	// Aggregate request counts
	totalRequests := int64(0)
	for _, c := range m.RequestsTotal.All() {
		totalRequests += c.Value()
	}
	stats["total_requests"] = totalRequests

	// Aggregate provider errors
	totalErrors := int64(0)
	for _, c := range m.ProviderErrors.All() {
		totalErrors += c.Value()
	}
	stats["total_provider_errors"] = totalErrors

	// Cache stats
	cacheHits := int64(0)
	for _, c := range m.CacheHits.All() {
		cacheHits += c.Value()
	}
	cacheMisses := int64(0)
	for _, c := range m.CacheMisses.All() {
		cacheMisses += c.Value()
	}
	stats["cache_hits"] = cacheHits
	stats["cache_misses"] = cacheMisses
	if cacheHits+cacheMisses > 0 {
		stats["cache_hit_rate"] = float64(cacheHits) / float64(cacheHits+cacheMisses)
	}

	// Token usage
	totalTokens := int64(0)
	for _, c := range m.TokensTotal.All() {
		totalTokens += c.Value()
	}
	stats["total_tokens"] = totalTokens

	return stats
}
