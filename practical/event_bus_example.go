package practical

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// EventBusExample demonstrates a production-ready event bus implementation
// with proper error handling, circuit breaker, and monitoring
type EventBusExample struct {
	subscribers   map[string][]EventSubscriber
	mu            sync.RWMutex
	metrics       *EventMetrics
	circuitBreaker *CircuitBreaker
	buffer        chan Event
	workers       int
	wg            sync.WaitGroup
}

type Event struct {
	ID          string
	Type        string
	Payload     interface{}
	Timestamp   time.Time
	Source      string
	CorrelationID string
}

type EventSubscriber struct {
	ID          string
	Handler     EventHandler
	Filter      EventFilter
	Priority    int
	MaxRetries  int
	Timeout     time.Duration
}

type EventHandler func(ctx context.Context, event Event) error
type EventFilter func(event Event) bool

type EventMetrics struct {
	mu              sync.RWMutex
	totalEvents     int64
	processedEvents int64
	failedEvents    int64
	avgProcessTime  time.Duration
	subscriberMetrics map[string]*SubscriberMetrics
}

type SubscriberMetrics struct {
	processed int64
	failed    int64
	avgTime   time.Duration
}

type CircuitBreaker struct {
	mu          sync.RWMutex
	failures    int
	maxFailures int
	state       string // "closed", "open", "half-open"
	lastFailure time.Time
	cooldown    time.Duration
}

// NewEventBusExample creates a production-ready event bus
func NewEventBusExample(workers int) *EventBusExample {
	eb := &EventBusExample{
		subscribers: make(map[string][]EventSubscriber),
		metrics: &EventMetrics{
			subscriberMetrics: make(map[string]*SubscriberMetrics),
		},
		circuitBreaker: &CircuitBreaker{
			maxFailures: 5,
			cooldown:    30 * time.Second,
			state:       "closed",
		},
		buffer:  make(chan Event, 10000),
		workers: workers,
	}

	// Start worker pool
	for i := 0; i < workers; i++ {
		eb.wg.Add(1)
		go eb.worker(i)
	}

	// Start metrics collector
	go eb.collectMetrics()

	return eb
}

// Subscribe adds a new subscriber with priority and filtering
func (eb *EventBusExample) Subscribe(eventType string, subscriber EventSubscriber) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// Initialize subscriber metrics
	eb.metrics.mu.Lock()
	if _, exists := eb.metrics.subscriberMetrics[subscriber.ID]; !exists {
		eb.metrics.subscriberMetrics[subscriber.ID] = &SubscriberMetrics{}
	}
	eb.metrics.mu.Unlock()

	// Add subscriber sorted by priority
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
	log.Printf("Subscriber %s registered for event type %s", subscriber.ID, eventType)
}

// Publish sends an event through the bus
func (eb *EventBusExample) Publish(ctx context.Context, event Event) error {
	// Check circuit breaker
	if !eb.circuitBreaker.Allow() {
		return fmt.Errorf("circuit breaker is open")
	}

	// Add timestamp if not set
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Update metrics
	eb.metrics.mu.Lock()
	eb.metrics.totalEvents++
	eb.metrics.mu.Unlock()

	// Send to buffer with timeout
	select {
	case eb.buffer <- event:
		return nil
	case <-time.After(5 * time.Second):
		eb.circuitBreaker.RecordFailure()
		return fmt.Errorf("event buffer full, timeout publishing event")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// worker processes events from the buffer
func (eb *EventBusExample) worker(id int) {
	defer eb.wg.Done()

	for event := range eb.buffer {
		start := time.Now()
		eb.processEvent(event)
		duration := time.Since(start)

		// Update metrics
		eb.metrics.mu.Lock()
		eb.metrics.processedEvents++
		eb.metrics.avgProcessTime = (eb.metrics.avgProcessTime + duration) / 2
		eb.metrics.mu.Unlock()
	}
}

// processEvent handles event distribution to subscribers
func (eb *EventBusExample) processEvent(event Event) {
	eb.mu.RLock()
	subscribers := eb.subscribers[event.Type]
	eb.mu.RUnlock()

	// Process wildcard subscribers
	eb.mu.RLock()
	wildcardSubs := eb.subscribers["*"]
	eb.mu.RUnlock()
	subscribers = append(subscribers, wildcardSubs...)

	var wg sync.WaitGroup
	for _, sub := range subscribers {
		// Apply filter
		if sub.Filter != nil && !sub.Filter(event) {
			continue
		}

		wg.Add(1)
		go func(subscriber EventSubscriber) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), subscriber.Timeout)
			defer cancel()

			// Execute with retries
			var lastErr error
			for retry := 0; retry <= subscriber.MaxRetries; retry++ {
				if retry > 0 {
					// Exponential backoff
					time.Sleep(time.Duration(retry*retry) * 100 * time.Millisecond)
				}

				err := eb.executeHandler(ctx, subscriber, event)
				if err == nil {
					eb.updateSubscriberMetrics(subscriber.ID, true, time.Since(event.Timestamp))
					return
				}
				lastErr = err
			}

			// All retries failed
			eb.updateSubscriberMetrics(subscriber.ID, false, time.Since(event.Timestamp))
			log.Printf("Failed to process event %s for subscriber %s: %v", event.ID, subscriber.ID, lastErr)
			eb.circuitBreaker.RecordFailure()
		}(sub)
	}

	wg.Wait()
}

// executeHandler safely executes a subscriber's handler
func (eb *EventBusExample) executeHandler(ctx context.Context, subscriber EventSubscriber, event Event) (err error) {
	// Panic recovery
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in handler %s: %v", subscriber.ID, r)
		}
	}()

	// Execute handler
	return subscriber.Handler(ctx, event)
}

// updateSubscriberMetrics updates metrics for a specific subscriber
func (eb *EventBusExample) updateSubscriberMetrics(subscriberID string, success bool, duration time.Duration) {
	eb.metrics.mu.Lock()
	defer eb.metrics.mu.Unlock()

	metrics := eb.metrics.subscriberMetrics[subscriberID]
	if metrics == nil {
		metrics = &SubscriberMetrics{}
		eb.metrics.subscriberMetrics[subscriberID] = metrics
	}

	if success {
		metrics.processed++
	} else {
		metrics.failed++
		eb.metrics.failedEvents++
	}

	metrics.avgTime = (metrics.avgTime + duration) / 2
}

// Allow checks if circuit breaker allows operations
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case "open":
		// Check if cooldown period has passed
		if time.Since(cb.lastFailure) > cb.cooldown {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = "half-open"
			cb.failures = 0
			cb.mu.Unlock()
			cb.mu.RLock()
			return true
		}
		return false
	case "half-open":
		// Allow limited traffic
		return cb.failures < cb.maxFailures/2
	default: // closed
		return true
	}
}

// RecordFailure records a failure in the circuit breaker
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()

	if cb.failures >= cb.maxFailures {
		cb.state = "open"
		log.Printf("Circuit breaker opened after %d failures", cb.failures)
	}
}

// RecordSuccess records a success in the circuit breaker
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == "half-open" && cb.failures == 0 {
		cb.state = "closed"
		log.Println("Circuit breaker closed")
	}

	if cb.failures > 0 {
		cb.failures--
	}
}

// collectMetrics periodically logs metrics
func (eb *EventBusExample) collectMetrics() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		eb.metrics.mu.RLock()
		log.Printf("Event Bus Metrics - Total: %d, Processed: %d, Failed: %d, Avg Time: %v",
			eb.metrics.totalEvents,
			eb.metrics.processedEvents,
			eb.metrics.failedEvents,
			eb.metrics.avgProcessTime,
		)

		for subID, metrics := range eb.metrics.subscriberMetrics {
			log.Printf("  Subscriber %s - Processed: %d, Failed: %d, Avg Time: %v",
				subID, metrics.processed, metrics.failed, metrics.avgTime)
		}
		eb.metrics.mu.RUnlock()
	}
}

// Shutdown gracefully shuts down the event bus
func (eb *EventBusExample) Shutdown() {
	close(eb.buffer)
	eb.wg.Wait()
	log.Println("Event bus shut down gracefully")
}

// RunEventBusExample demonstrates the event bus in action
func RunEventBusExample() {
	fmt.Println("\n=== Event Bus Example ===")

	eb := NewEventBusExample(4) // 4 worker goroutines
	defer eb.Shutdown()

	// Register subscribers with different priorities
	eb.Subscribe("user.created", EventSubscriber{
		ID:       "email-service",
		Priority: 10,
		MaxRetries: 3,
		Timeout:  5 * time.Second,
		Handler: func(ctx context.Context, event Event) error {
			log.Printf("Email service processing user.created: %v", event.Payload)
			time.Sleep(100 * time.Millisecond) // Simulate processing
			return nil
		},
		Filter: func(event Event) bool {
			// Only process events with email field
			if payload, ok := event.Payload.(map[string]interface{}); ok {
				_, hasEmail := payload["email"]
				return hasEmail
			}
			return false
		},
	})

	eb.Subscribe("user.created", EventSubscriber{
		ID:       "analytics-service",
		Priority: 5,
		MaxRetries: 2,
		Timeout:  3 * time.Second,
		Handler: func(ctx context.Context, event Event) error {
			log.Printf("Analytics service tracking user.created: %v", event.Payload)
			return nil
		},
	})

	// Subscribe to all events
	eb.Subscribe("*", EventSubscriber{
		ID:       "audit-service",
		Priority: 1,
		MaxRetries: 5,
		Timeout:  10 * time.Second,
		Handler: func(ctx context.Context, event Event) error {
			log.Printf("Audit service logging event %s of type %s", event.ID, event.Type)
			return nil
		},
	})

	// Publish events
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		event := Event{
			ID:   fmt.Sprintf("evt-%d", i),
			Type: "user.created",
			Payload: map[string]interface{}{
				"id":    i,
				"name":  fmt.Sprintf("User %d", i),
				"email": fmt.Sprintf("user%d@example.com", i),
			},
			Source:        "user-service",
			CorrelationID: fmt.Sprintf("req-%d", i),
		}

		if err := eb.Publish(ctx, event); err != nil {
			log.Printf("Failed to publish event: %v", err)
		}
	}

	// Wait for processing
	time.Sleep(2 * time.Second)

	fmt.Println("Event bus example completed")
}