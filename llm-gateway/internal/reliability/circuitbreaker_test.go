package reliability

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestCircuitState_String(t *testing.T) {
	tests := []struct {
		state    CircuitState
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{CircuitState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.state.String(); got != tt.expected {
				t.Errorf("CircuitState.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewCircuitBreaker(t *testing.T) {
	config := CircuitBreakerConfig{
		Name:                "test",
		FailureThreshold:    5,
		SuccessThreshold:    3,
		Timeout:             30 * time.Second,
		MaxHalfOpenRequests: 1,
	}

	cb := NewCircuitBreaker(config)

	if cb == nil {
		t.Fatal("NewCircuitBreaker returned nil")
	}
	if cb.State() != StateClosed {
		t.Errorf("initial state = %v, want closed", cb.State())
	}
}

func TestDefaultCircuitBreakerConfig(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test-circuit")

	if config.Name != "test-circuit" {
		t.Errorf("Name = %v, want test-circuit", config.Name)
	}
	if config.FailureThreshold != 5 {
		t.Errorf("FailureThreshold = %v, want 5", config.FailureThreshold)
	}
	if config.SuccessThreshold != 3 {
		t.Errorf("SuccessThreshold = %v, want 3", config.SuccessThreshold)
	}
	if config.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", config.Timeout)
	}
}

func TestCircuitBreaker_Execute_Success(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig("test"))

	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}
	if cb.State() != StateClosed {
		t.Errorf("state = %v, want closed", cb.State())
	}
}

func TestCircuitBreaker_Execute_Failure(t *testing.T) {
	cb := NewCircuitBreaker(DefaultCircuitBreakerConfig("test"))
	testErr := errors.New("test error")

	err := cb.Execute(func() error {
		return testErr
	})

	if !errors.Is(err, testErr) {
		t.Errorf("Execute() error = %v, want %v", err, testErr)
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	config := CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 3,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
	}
	cb := NewCircuitBreaker(config)
	testErr := errors.New("test error")

	// Cause failures to open circuit
	for i := 0; i < config.FailureThreshold; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("state = %v, want open after %d failures", cb.State(), config.FailureThreshold)
	}

	// Next request should fail fast
	err := cb.Execute(func() error {
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("Execute() error = %v, want ErrCircuitOpen", err)
	}
}

func TestCircuitBreaker_TransitionsToHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		Name:                "test",
		FailureThreshold:    2,
		SuccessThreshold:    1,
		Timeout:             50 * time.Millisecond, // Short timeout for testing
		MaxHalfOpenRequests: 1,
	}
	cb := NewCircuitBreaker(config)
	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < config.FailureThreshold; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("state = %v, want open", cb.State())
	}

	// Wait for timeout
	time.Sleep(config.Timeout + 10*time.Millisecond)

	// Next request should be allowed (half-open)
	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("Execute() in half-open state error = %v, want nil", err)
	}
}

func TestCircuitBreaker_ClosesAfterSuccessInHalfOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		Name:                "test",
		FailureThreshold:    2,
		SuccessThreshold:    2,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 3,
	}
	cb := NewCircuitBreaker(config)
	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < config.FailureThreshold; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	// Wait for timeout
	time.Sleep(config.Timeout + 10*time.Millisecond)

	// Successful requests in half-open should close circuit
	for i := 0; i < config.SuccessThreshold; i++ {
		cb.Execute(func() error {
			return nil
		})
	}

	if cb.State() != StateClosed {
		t.Errorf("state = %v, want closed after successful requests in half-open", cb.State())
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	config := CircuitBreakerConfig{
		Name:             "test",
		FailureThreshold: 2,
		SuccessThreshold: 2,
		Timeout:          1 * time.Second,
	}
	cb := NewCircuitBreaker(config)
	testErr := errors.New("test error")

	// Open the circuit
	for i := 0; i < config.FailureThreshold; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("state = %v, want open", cb.State())
	}

	// Reset
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("state after Reset() = %v, want closed", cb.State())
	}

	// Should work normally after reset
	err := cb.Execute(func() error {
		return nil
	})
	if err != nil {
		t.Errorf("Execute() after Reset() error = %v, want nil", err)
	}
}

func TestCircuitBreaker_Stats(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test-stats")
	cb := NewCircuitBreaker(config)

	stats := cb.Stats()

	if stats["name"] != "test-stats" {
		t.Errorf("stats[name] = %v, want test-stats", stats["name"])
	}
	if stats["state"] != "closed" {
		t.Errorf("stats[state] = %v, want closed", stats["state"])
	}
	if stats["failure_threshold"] != config.FailureThreshold {
		t.Errorf("stats[failure_threshold] = %v, want %d", stats["failure_threshold"], config.FailureThreshold)
	}
}

func TestCircuitBreaker_Concurrent(t *testing.T) {
	config := CircuitBreakerConfig{
		Name:                "concurrent-test",
		FailureThreshold:    10,
		SuccessThreshold:    5,
		Timeout:             1 * time.Second,
		MaxHalfOpenRequests: 5,
	}
	cb := NewCircuitBreaker(config)

	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			cb.Execute(func() error {
				if id%3 == 0 {
					return errors.New("simulated error")
				}
				return nil
			})
		}(i)
	}

	wg.Wait()
	// Just ensure no race conditions - the state depends on execution order
	_ = cb.State()
}

func TestCircuitBreakerRegistry_Get(t *testing.T) {
	registry := NewCircuitBreakerRegistry()

	cb1 := registry.Get("service-a")
	cb2 := registry.Get("service-a")
	cb3 := registry.Get("service-b")

	if cb1 != cb2 {
		t.Error("Get() should return same instance for same name")
	}
	if cb1 == cb3 {
		t.Error("Get() should return different instances for different names")
	}
}

func TestCircuitBreakerRegistry_GetWithConfig(t *testing.T) {
	registry := NewCircuitBreakerRegistry()

	config := CircuitBreakerConfig{
		Name:             "custom-service",
		FailureThreshold: 10,
		SuccessThreshold: 5,
		Timeout:          1 * time.Minute,
	}

	cb := registry.GetWithConfig(config)
	stats := cb.Stats()

	if stats["failure_threshold"] != 10 {
		t.Errorf("failure_threshold = %v, want 10", stats["failure_threshold"])
	}
}

func TestCircuitBreakerRegistry_AllStats(t *testing.T) {
	registry := NewCircuitBreakerRegistry()

	registry.Get("service-1")
	registry.Get("service-2")
	registry.Get("service-3")

	stats := registry.AllStats()

	if len(stats) != 3 {
		t.Errorf("AllStats() returned %d items, want 3", len(stats))
	}
}

func TestCircuitBreakerRegistry_Concurrent(t *testing.T) {
	registry := NewCircuitBreakerRegistry()

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			name := "service-" + string(rune('a'+id%5))
			cb := registry.Get(name)
			cb.Execute(func() error {
				return nil
			})
		}(i)
	}

	wg.Wait()

	stats := registry.AllStats()
	if len(stats) != 5 {
		t.Errorf("AllStats() returned %d items, want 5", len(stats))
	}
}
