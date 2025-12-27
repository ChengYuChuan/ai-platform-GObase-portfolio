package performance

import (
	"container/heap"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	ErrQueueFull     = errors.New("request queue is full")
	ErrQueueClosed   = errors.New("request queue is closed")
	ErrRequestExpired = errors.New("request expired while in queue")
)

// Priority levels for request queuing
type Priority int

const (
	PriorityLow    Priority = 0
	PriorityNormal Priority = 1
	PriorityHigh   Priority = 2
	PriorityCritical Priority = 3
)

// QueueConfig holds configuration for the request queue
type QueueConfig struct {
	// Enabled controls whether queuing is active
	Enabled bool
	// MaxQueueSize is the maximum number of requests that can be queued
	MaxQueueSize int
	// MaxWaitTime is the maximum time a request can wait in queue
	MaxWaitTime time.Duration
	// WorkerCount is the number of concurrent workers processing requests
	WorkerCount int
	// PriorityEnabled enables priority-based ordering
	PriorityEnabled bool
}

// DefaultQueueConfig returns sensible defaults
func DefaultQueueConfig() QueueConfig {
	return QueueConfig{
		Enabled:         false,
		MaxQueueSize:    1000,
		MaxWaitTime:     30 * time.Second,
		WorkerCount:     10,
		PriorityEnabled: true,
	}
}

// QueuedRequest represents a request waiting to be processed
type QueuedRequest struct {
	ID        string
	Priority  Priority
	Payload   interface{}
	ResultCh  chan QueueResult
	CreatedAt time.Time
	Deadline  time.Time
	index     int // Internal index for heap
}

// QueueResult contains the result of processing a queued request
type QueueResult struct {
	Result interface{}
	Error  error
}

// RequestProcessor is a function that processes a queued request
type RequestProcessor func(ctx context.Context, payload interface{}) (interface{}, error)

// RequestQueue implements a priority queue for rate-limited request handling
type RequestQueue struct {
	config    QueueConfig
	processor RequestProcessor
	pq        priorityQueue
	mu        sync.Mutex
	cond      *sync.Cond
	closed    bool
	wg        sync.WaitGroup

	// Statistics
	totalEnqueued  int64
	totalProcessed int64
	totalDropped   int64
	totalExpired   int64
}

// NewRequestQueue creates a new request queue
func NewRequestQueue(config QueueConfig, processor RequestProcessor) *RequestQueue {
	q := &RequestQueue{
		config:    config,
		processor: processor,
		pq:        make(priorityQueue, 0, config.MaxQueueSize),
	}
	q.cond = sync.NewCond(&q.mu)

	heap.Init(&q.pq)

	// Start worker goroutines
	for i := 0; i < config.WorkerCount; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}

	log.Info().
		Int("max_queue_size", config.MaxQueueSize).
		Int("worker_count", config.WorkerCount).
		Dur("max_wait_time", config.MaxWaitTime).
		Bool("priority_enabled", config.PriorityEnabled).
		Msg("Request queue initialized")

	return q
}

// Enqueue adds a request to the queue
func (q *RequestQueue) Enqueue(ctx context.Context, id string, priority Priority, payload interface{}) (interface{}, error) {
	q.mu.Lock()

	if q.closed {
		q.mu.Unlock()
		return nil, ErrQueueClosed
	}

	// Check queue capacity
	if len(q.pq) >= q.config.MaxQueueSize {
		q.mu.Unlock()
		atomic.AddInt64(&q.totalDropped, 1)
		return nil, ErrQueueFull
	}

	// Create queued request
	req := &QueuedRequest{
		ID:        id,
		Priority:  priority,
		Payload:   payload,
		ResultCh:  make(chan QueueResult, 1),
		CreatedAt: time.Now(),
		Deadline:  time.Now().Add(q.config.MaxWaitTime),
	}

	// Add to priority queue
	heap.Push(&q.pq, req)
	atomic.AddInt64(&q.totalEnqueued, 1)

	// Signal a waiting worker
	q.cond.Signal()
	q.mu.Unlock()

	// Wait for result or context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-req.ResultCh:
		return result.Result, result.Error
	}
}

// EnqueueAsync adds a request without waiting for result
func (q *RequestQueue) EnqueueAsync(id string, priority Priority, payload interface{}) (<-chan QueueResult, error) {
	q.mu.Lock()

	if q.closed {
		q.mu.Unlock()
		return nil, ErrQueueClosed
	}

	if len(q.pq) >= q.config.MaxQueueSize {
		q.mu.Unlock()
		atomic.AddInt64(&q.totalDropped, 1)
		return nil, ErrQueueFull
	}

	req := &QueuedRequest{
		ID:        id,
		Priority:  priority,
		Payload:   payload,
		ResultCh:  make(chan QueueResult, 1),
		CreatedAt: time.Now(),
		Deadline:  time.Now().Add(q.config.MaxWaitTime),
	}

	heap.Push(&q.pq, req)
	atomic.AddInt64(&q.totalEnqueued, 1)
	q.cond.Signal()
	q.mu.Unlock()

	return req.ResultCh, nil
}

// worker processes requests from the queue
func (q *RequestQueue) worker(id int) {
	defer q.wg.Done()

	log.Debug().Int("worker_id", id).Msg("Queue worker started")

	for {
		q.mu.Lock()

		// Wait for work or shutdown
		for len(q.pq) == 0 && !q.closed {
			q.cond.Wait()
		}

		if q.closed && len(q.pq) == 0 {
			q.mu.Unlock()
			log.Debug().Int("worker_id", id).Msg("Queue worker shutting down")
			return
		}

		// Get highest priority request
		req := heap.Pop(&q.pq).(*QueuedRequest)
		q.mu.Unlock()

		// Check if request has expired
		if time.Now().After(req.Deadline) {
			atomic.AddInt64(&q.totalExpired, 1)
			req.ResultCh <- QueueResult{Error: ErrRequestExpired}
			close(req.ResultCh)
			continue
		}

		// Process the request
		ctx, cancel := context.WithDeadline(context.Background(), req.Deadline)
		result, err := q.processor(ctx, req.Payload)
		cancel()

		atomic.AddInt64(&q.totalProcessed, 1)

		// Send result
		req.ResultCh <- QueueResult{Result: result, Error: err}
		close(req.ResultCh)
	}
}

// Close shuts down the queue gracefully
func (q *RequestQueue) Close() {
	q.mu.Lock()
	q.closed = true
	q.cond.Broadcast() // Wake up all workers
	q.mu.Unlock()

	// Wait for workers to finish
	q.wg.Wait()
	log.Info().Msg("Request queue closed")
}

// Len returns the current queue length
func (q *RequestQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.pq)
}

// Stats returns queue statistics
func (q *RequestQueue) Stats() map[string]interface{} {
	q.mu.Lock()
	queueLen := len(q.pq)
	q.mu.Unlock()

	return map[string]interface{}{
		"enabled":         q.config.Enabled,
		"queue_length":    queueLen,
		"max_queue_size":  q.config.MaxQueueSize,
		"worker_count":    q.config.WorkerCount,
		"total_enqueued":  atomic.LoadInt64(&q.totalEnqueued),
		"total_processed": atomic.LoadInt64(&q.totalProcessed),
		"total_dropped":   atomic.LoadInt64(&q.totalDropped),
		"total_expired":   atomic.LoadInt64(&q.totalExpired),
	}
}

// priorityQueue implements heap.Interface for QueuedRequest
type priorityQueue []*QueuedRequest

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// Higher priority first
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	// Earlier deadline first (FIFO for same priority)
	return pq[i].CreatedAt.Before(pq[j].CreatedAt)
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*QueuedRequest)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // Avoid memory leak
	item.index = -1 // Mark as removed
	*pq = old[0 : n-1]
	return item
}

// AdaptiveRateLimiter combines rate limiting with queue management
type AdaptiveRateLimiter struct {
	queue       *RequestQueue
	rateLimit   int // Requests per second
	burstSize   int
	tokens      chan struct{}
	refillStop  chan struct{}
}

// NewAdaptiveRateLimiter creates a new adaptive rate limiter
func NewAdaptiveRateLimiter(rateLimit, burstSize int, queueConfig QueueConfig, processor RequestProcessor) *AdaptiveRateLimiter {
	arl := &AdaptiveRateLimiter{
		queue:      NewRequestQueue(queueConfig, processor),
		rateLimit:  rateLimit,
		burstSize:  burstSize,
		tokens:     make(chan struct{}, burstSize),
		refillStop: make(chan struct{}),
	}

	// Fill initial tokens
	for i := 0; i < burstSize; i++ {
		arl.tokens <- struct{}{}
	}

	// Start token refill goroutine
	go arl.refillTokens()

	return arl
}

func (arl *AdaptiveRateLimiter) refillTokens() {
	interval := time.Second / time.Duration(arl.rateLimit)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-arl.refillStop:
			return
		case <-ticker.C:
			select {
			case arl.tokens <- struct{}{}:
				// Token added
			default:
				// Bucket full, skip
			}
		}
	}
}

// Execute processes a request with rate limiting and queuing
func (arl *AdaptiveRateLimiter) Execute(ctx context.Context, id string, priority Priority, payload interface{}) (interface{}, error) {
	// Try to get a token
	select {
	case <-arl.tokens:
		// Got token, process immediately
		return arl.queue.processor(ctx, payload)
	default:
		// No token available, queue the request
		return arl.queue.Enqueue(ctx, id, priority, payload)
	}
}

// Close shuts down the rate limiter
func (arl *AdaptiveRateLimiter) Close() {
	close(arl.refillStop)
	arl.queue.Close()
}

// Stats returns combined statistics
func (arl *AdaptiveRateLimiter) Stats() map[string]interface{} {
	stats := arl.queue.Stats()
	stats["rate_limit"] = arl.rateLimit
	stats["burst_size"] = arl.burstSize
	stats["available_tokens"] = len(arl.tokens)
	return stats
}
