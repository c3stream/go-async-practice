package solutions

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Solution17EventBus demonstrates three approaches to fixing event bus issues:
// 1. Reliable delivery with deduplication
// 2. Memory-efficient with bounded resources
// 3. Circuit breaker pattern for fault tolerance

// Solution 1: Reliable Event Bus with Deduplication
type ReliableEventBus struct {
	mu           sync.RWMutex
	subscribers  map[string][]EventHandler
	processedIDs sync.Map // Track processed events
	metrics      *EventMetrics
	maxHistory   int
	historyTTL   time.Duration
}

type Event struct {
	ID            string
	Type          string
	Payload       interface{}
	Timestamp     time.Time
	RetryCount    int
	MaxRetries    int
}

type EventHandler func(ctx context.Context, event Event) error

type EventMetrics struct {
	processed  int64
	failed     int64
	duplicates int64
	mu         sync.RWMutex
}

func NewReliableEventBus() *ReliableEventBus {
	eb := &ReliableEventBus{
		subscribers: make(map[string][]EventHandler),
		metrics:     &EventMetrics{},
		maxHistory:  10000,
		historyTTL:  1 * time.Hour,
	}

	// Start cleanup goroutine
	go eb.cleanupHistory()

	return eb
}

func (eb *ReliableEventBus) Subscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Check for duplicate handlers
	for _, existing := range eb.subscribers[eventType] {
		if &existing == &handler {
			return // Already subscribed
		}
	}

	eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)
}

func (eb *ReliableEventBus) Publish(ctx context.Context, event Event) error {
	// Check for duplicate
	if _, exists := eb.processedIDs.LoadOrStore(event.ID, time.Now()); exists {
		atomic.AddInt64(&eb.metrics.duplicates, 1)
		return nil // Already processed
	}

	eb.mu.RLock()
	handlers := eb.subscribers[event.Type]
	eb.mu.RUnlock()

	if len(handlers) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errChan := make(chan error, len(handlers))

	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()

			// Create timeout context
			handlerCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			// Execute with panic recovery
			err := eb.safeExecute(handlerCtx, h, event)
			if err != nil {
				errChan <- err
				atomic.AddInt64(&eb.metrics.failed, 1)
			} else {
				atomic.AddInt64(&eb.metrics.processed, 1)
			}
		}(handler)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to process event for %d handlers", len(errors))
	}

	return nil
}

func (eb *ReliableEventBus) safeExecute(ctx context.Context, handler EventHandler, event Event) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v", r)
		}
	}()

	return handler(ctx, event)
}

func (eb *ReliableEventBus) cleanupHistory() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-eb.historyTTL)
		count := 0

		eb.processedIDs.Range(func(key, value interface{}) bool {
			if timestamp, ok := value.(time.Time); ok {
				if timestamp.Before(cutoff) {
					eb.processedIDs.Delete(key)
					count++
				}
			}
			return true
		})

		if count > 0 {
			log.Printf("Cleaned up %d old event IDs", count)
		}
	}
}

// Solution 2: Memory-Efficient Event Bus with Bounded Resources
type BoundedEventBus struct {
	subscribers   map[string][]EventSubscriber
	buffer        chan Event
	workers       int
	mu            sync.RWMutex
	wg            sync.WaitGroup
	shutdown      chan struct{}
	eventHistory  *RingBuffer
}

type EventSubscriber struct {
	ID       string
	Handler  EventHandler
	Priority int
	Filter   func(Event) bool
}

type RingBuffer struct {
	buffer []Event
	head   int64
	tail   int64
	size   int64
	mu     sync.Mutex
}

func NewBoundedEventBus(workers int, bufferSize int) *BoundedEventBus {
	eb := &BoundedEventBus{
		subscribers:  make(map[string][]EventSubscriber),
		buffer:      make(chan Event, bufferSize),
		workers:     workers,
		shutdown:    make(chan struct{}),
		eventHistory: NewRingBuffer(1000), // Fixed-size history
	}

	// Start worker pool
	for i := 0; i < workers; i++ {
		eb.wg.Add(1)
		go eb.worker()
	}

	return eb
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buffer: make([]Event, size),
		size:   int64(size),
	}
}

func (rb *RingBuffer) Add(event Event) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	rb.buffer[rb.tail%rb.size] = event
	rb.tail++

	if rb.tail-rb.head > rb.size {
		rb.head = rb.tail - rb.size
	}
}

func (rb *RingBuffer) GetRecent(n int) []Event {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	count := rb.tail - rb.head
	if int64(n) > count {
		n = int(count)
	}

	result := make([]Event, n)
	for i := 0; i < n; i++ {
		idx := (rb.tail - int64(n) + int64(i)) % rb.size
		result[i] = rb.buffer[idx]
	}

	return result
}

func (eb *BoundedEventBus) Subscribe(eventType string, subscriber EventSubscriber) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Sort by priority
	subscribers := eb.subscribers[eventType]
	inserted := false

	for i, existing := range subscribers {
		if subscriber.Priority > existing.Priority {
			subscribers = append(subscribers[:i], append([]EventSubscriber{subscriber}, subscribers[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		subscribers = append(subscribers, subscriber)
	}

	eb.subscribers[eventType] = subscribers
}

func (eb *BoundedEventBus) Publish(ctx context.Context, event Event) error {
	// Add to history
	eb.eventHistory.Add(event)

	select {
	case eb.buffer <- event:
		return nil
	case <-time.After(100 * time.Millisecond):
		return fmt.Errorf("event buffer full, timeout")
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (eb *BoundedEventBus) worker() {
	defer eb.wg.Done()

	for {
		select {
		case event := <-eb.buffer:
			eb.processEvent(event)
		case <-eb.shutdown:
			// Process remaining events
			for {
				select {
				case event := <-eb.buffer:
					eb.processEvent(event)
				default:
					return
				}
			}
		}
	}
}

func (eb *BoundedEventBus) processEvent(event Event) {
	eb.mu.RLock()
	subscribers := eb.subscribers[event.Type]
	eb.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, subscriber := range subscribers {
		// Apply filter
		if subscriber.Filter != nil && !subscriber.Filter(event) {
			continue
		}

		// Execute handler
		if err := subscriber.Handler(ctx, event); err != nil {
			log.Printf("Subscriber %s failed: %v", subscriber.ID, err)
		}
	}
}

func (eb *BoundedEventBus) Shutdown() {
	close(eb.shutdown)
	eb.wg.Wait()
}

// Solution 3: Event Bus with Circuit Breaker
type CircuitBreakerEventBus struct {
	subscribers     map[string][]EventHandler
	mu              sync.RWMutex
	circuitBreakers map[string]*CircuitBreaker
	dlq             []Event // Dead letter queue
	dlqMu           sync.Mutex
}

type CircuitBreaker struct {
	failures      int64
	lastFailTime  time.Time
	state         int32 // 0: closed, 1: open, 2: half-open
	maxFailures   int64
	cooldownTime  time.Duration
	mu            sync.RWMutex
}

const (
	StateClosed = iota
	StateOpen
	StateHalfOpen
)

func NewCircuitBreakerEventBus() *CircuitBreakerEventBus {
	return &CircuitBreakerEventBus{
		subscribers:     make(map[string][]EventHandler),
		circuitBreakers: make(map[string]*CircuitBreaker),
		dlq:            make([]Event, 0),
	}
}

func (eb *CircuitBreakerEventBus) Subscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)

	// Initialize circuit breaker for this event type
	if _, exists := eb.circuitBreakers[eventType]; !exists {
		eb.circuitBreakers[eventType] = &CircuitBreaker{
			maxFailures:  5,
			cooldownTime: 30 * time.Second,
		}
	}
}

func (eb *CircuitBreakerEventBus) Publish(ctx context.Context, event Event) error {
	eb.mu.RLock()
	handlers := eb.subscribers[event.Type]
	cb := eb.circuitBreakers[event.Type]
	eb.mu.RUnlock()

	if len(handlers) == 0 {
		return nil
	}

	// Check circuit breaker
	if cb != nil && !cb.Allow() {
		// Add to DLQ
		eb.dlqMu.Lock()
		eb.dlq = append(eb.dlq, event)
		eb.dlqMu.Unlock()

		return fmt.Errorf("circuit breaker open for event type %s", event.Type)
	}

	var wg sync.WaitGroup
	successCount := int64(0)
	failureCount := int64(0)

	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()

			handlerCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			if err := eb.executeWithRecovery(handlerCtx, h, event); err != nil {
				atomic.AddInt64(&failureCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}(handler)
	}

	wg.Wait()

	// Update circuit breaker
	if cb != nil {
		if failureCount > 0 {
			cb.RecordFailure()
		} else if successCount > 0 {
			cb.RecordSuccess()
		}
	}

	if failureCount > 0 && successCount == 0 {
		return fmt.Errorf("all handlers failed")
	}

	return nil
}

func (eb *CircuitBreakerEventBus) executeWithRecovery(ctx context.Context, handler EventHandler, event Event) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	return handler(ctx, event)
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	state := atomic.LoadInt32(&cb.state)

	switch state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if cooldown period has passed
		if time.Since(cb.lastFailTime) > cb.cooldownTime {
			atomic.StoreInt32(&cb.state, StateHalfOpen)
			return true
		}
		return false

	case StateHalfOpen:
		// Allow limited traffic
		return atomic.LoadInt64(&cb.failures) < cb.maxFailures/2

	default:
		return true
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	failures := atomic.AddInt64(&cb.failures, 1)

	cb.mu.Lock()
	cb.lastFailTime = time.Now()
	cb.mu.Unlock()

	if failures >= cb.maxFailures {
		atomic.StoreInt32(&cb.state, StateOpen)
		log.Printf("Circuit breaker opened after %d failures", failures)
	}
}

func (cb *CircuitBreaker) RecordSuccess() {
	state := atomic.LoadInt32(&cb.state)

	if state == StateHalfOpen {
		atomic.StoreInt32(&cb.state, StateClosed)
		atomic.StoreInt64(&cb.failures, 0)
		log.Println("Circuit breaker closed after successful recovery")
	}
}

func (eb *CircuitBreakerEventBus) ProcessDLQ(ctx context.Context) error {
	eb.dlqMu.Lock()
	events := eb.dlq
	eb.dlq = make([]Event, 0)
	eb.dlqMu.Unlock()

	for _, event := range events {
		event.RetryCount++
		if event.RetryCount <= event.MaxRetries {
			if err := eb.Publish(ctx, event); err != nil {
				log.Printf("Failed to reprocess DLQ event: %v", err)

				// Put back in DLQ
				eb.dlqMu.Lock()
				eb.dlq = append(eb.dlq, event)
				eb.dlqMu.Unlock()
			}
		} else {
			log.Printf("Event %s exceeded max retries, discarding", event.ID)
		}
	}

	return nil
}

// RunSolution17 demonstrates the three solutions
func RunSolution17() {
	fmt.Println("=== Solution 17: Event Bus Pattern ===")

	// Solution 1: Reliable Event Bus
	fmt.Println("\n1. Reliable Event Bus with Deduplication:")
	reliableBus := NewReliableEventBus()

	reliableBus.Subscribe("user.created", func(ctx context.Context, e Event) error {
		fmt.Printf("  Processing user.created: %v\n", e.Payload)
		return nil
	})

	ctx := context.Background()

	// Test deduplication
	event := Event{
		ID:        "evt-1",
		Type:      "user.created",
		Payload:   "User 1",
		Timestamp: time.Now(),
	}

	reliableBus.Publish(ctx, event)
	reliableBus.Publish(ctx, event) // Duplicate, will be ignored

	fmt.Printf("  Metrics - Processed: %d, Duplicates: %d\n",
		reliableBus.metrics.processed,
		reliableBus.metrics.duplicates)

	// Solution 2: Bounded Event Bus
	fmt.Println("\n2. Memory-Efficient Event Bus:")
	boundedBus := NewBoundedEventBus(4, 100)
	defer boundedBus.Shutdown()

	boundedBus.Subscribe("order.placed", EventSubscriber{
		ID:       "high-priority",
		Priority: 10,
		Handler: func(ctx context.Context, e Event) error {
			fmt.Printf("  High priority handler: %v\n", e.Payload)
			return nil
		},
	})

	boundedBus.Subscribe("order.placed", EventSubscriber{
		ID:       "low-priority",
		Priority: 1,
		Handler: func(ctx context.Context, e Event) error {
			fmt.Printf("  Low priority handler: %v\n", e.Payload)
			return nil
		},
	})

	boundedBus.Publish(ctx, Event{
		ID:      "evt-2",
		Type:    "order.placed",
		Payload: "Order 123",
	})

	// Solution 3: Circuit Breaker Event Bus
	fmt.Println("\n3. Event Bus with Circuit Breaker:")
	cbBus := NewCircuitBreakerEventBus()

	failCount := 0
	cbBus.Subscribe("payment.process", func(ctx context.Context, e Event) error {
		failCount++
		if failCount <= 5 {
			return fmt.Errorf("payment processing failed")
		}
		fmt.Printf("  Payment processed successfully: %v\n", e.Payload)
		return nil
	})

	// Trigger circuit breaker
	for i := 0; i < 7; i++ {
		err := cbBus.Publish(ctx, Event{
			ID:         fmt.Sprintf("evt-%d", i),
			Type:       "payment.process",
			Payload:    fmt.Sprintf("Payment %d", i),
			MaxRetries: 3,
		})
		if err != nil {
			fmt.Printf("  Event %d failed: %v\n", i, err)
		}
	}

	// Process DLQ
	time.Sleep(100 * time.Millisecond)
	cbBus.ProcessDLQ(ctx)

	fmt.Println("\nAll solutions demonstrated successfully!")
}