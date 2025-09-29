package challenges

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Challenge 25: Distributed Tracing
//
// Problems to fix:
// 1. Lost trace context propagation across services
// 2. Span leaks - spans created but never finished
// 3. Circular span dependencies causing deadlocks
// 4. Missing correlation between async operations
// 5. Race conditions in span attributes
// 6. Memory leaks from unbounded trace storage
// 7. Incorrect parent-child relationships
// 8. Missing critical spans for debugging

type TraceContext struct {
	TraceID    string
	SpanID     string
	ParentID   string
	Baggage    map[string]string
	StartTime  time.Time
	mu         sync.Mutex // BUG: Not protecting all accesses
}

type Span struct {
	Context    *TraceContext
	Name       string
	Service    string
	StartTime  time.Time
	EndTime    time.Time
	Attributes map[string]interface{}
	Events     []SpanEvent
	Status     string
	Children   []*Span
	mu         sync.RWMutex // BUG: Incorrect lock usage
}

type SpanEvent struct {
	Name      string
	Timestamp time.Time
	Attrs     map[string]interface{}
}

type Tracer struct {
	spans      map[string]*Span
	traces     map[string][]*Span
	exporters  []Exporter
	sampler    Sampler
	mu         sync.RWMutex
	bufferSize int           // BUG: No limit enforcement
	buffer     chan *Span    // BUG: Can cause deadlock
	shutdown   chan struct{}
}

type Exporter interface {
	Export(span *Span) error
}

type Sampler interface {
	ShouldSample(traceID string) bool
}

func Challenge25DistributedTracing() {
	tracer := &Tracer{
		spans:      make(map[string]*Span),
		traces:     make(map[string][]*Span),
		buffer:     make(chan *Span), // BUG: Unbuffered channel
		shutdown:   make(chan struct{}),
		bufferSize: 1000,
	}

	// Start background processor
	go tracer.processSpans()

	// Simulate distributed service calls
	var wg sync.WaitGroup

	// Service A - Entry point
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx := context.Background()
		span := tracer.StartSpan(ctx, "ServiceA.HandleRequest", "service-a")

		// BUG: Not passing context to child operations
		processOrder(tracer, span)

		// BUG: Forgot to end span
		// span.End()
	}()

	// Service B - Parallel processing
	wg.Add(1)
	go func() {
		defer wg.Done()
		// BUG: Creating orphan spans without parent context
		span := tracer.StartSpan(context.Background(), "ServiceB.Process", "service-b")

		// Simulate work
		time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

		// BUG: Race condition when updating attributes
		span.Attributes["processed"] = true
		span.Attributes["count"] = 42
	}()

	// Simulate async message processing
	msgChan := make(chan string, 10)

	// Producer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 5; i++ {
			// BUG: Not propagating trace context in messages
			msgChan <- fmt.Sprintf("message-%d", i)
		}
		close(msgChan)
	}()

	// Consumers
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for msg := range msgChan {
				// BUG: Creating new traces instead of continuing existing ones
				span := tracer.StartSpan(context.Background(),
					fmt.Sprintf("Consumer%d.Process", id), "consumer")

				// Process message
				processMessage(tracer, span, msg)

				// BUG: Not ending span in all code paths
				if rand.Float32() > 0.5 {
					span.End()
				}
			}
		}(i)
	}

	// Circular dependency scenario
	wg.Add(1)
	go func() {
		defer wg.Done()
		span1 := tracer.StartSpan(context.Background(), "Circular1", "service-x")
		span2 := tracer.StartSpan(context.Background(), "Circular2", "service-y")

		// BUG: Creating circular parent-child relationship
		span1.Children = append(span1.Children, span2)
		span2.Children = append(span2.Children, span1)
	}()

	wg.Wait()

	// BUG: Not properly shutting down tracer
	// Missing flush of pending spans
}

func (t *Tracer) StartSpan(ctx context.Context, name, service string) *Span {
	span := &Span{
		Name:       name,
		Service:    service,
		StartTime:  time.Now(),
		Attributes: make(map[string]interface{}), // BUG: Not thread-safe
		Events:     []SpanEvent{},                // BUG: Can grow unbounded
		Children:   []*Span{},
		Context: &TraceContext{
			TraceID:   generateID(),
			SpanID:    generateID(),
			Baggage:   make(map[string]string),
			StartTime: time.Now(),
		},
	}

	// BUG: Not checking sampler decision
	// if t.sampler != nil && !t.sampler.ShouldSample(span.Context.TraceID) {
	//     return &NoOpSpan{}
	// }

	t.mu.Lock()
	t.spans[span.Context.SpanID] = span
	// BUG: Growing unbounded
	t.traces[span.Context.TraceID] = append(t.traces[span.Context.TraceID], span)
	t.mu.Unlock()

	// BUG: Can block if buffer is full
	t.buffer <- span

	return span
}

func (t *Tracer) processSpans() {
	for {
		select {
		case span := <-t.buffer:
			// BUG: Not handling exporter failures
			for _, exporter := range t.exporters {
				exporter.Export(span)
			}

			// BUG: Never cleaning up old spans from memory
			// Should implement TTL or max size

		case <-t.shutdown:
			// BUG: Not draining buffer before shutdown
			return
		}
	}
}

func (s *Span) End() {
	s.mu.Lock() // BUG: Should use RLock for reading
	s.EndTime = time.Now()
	s.Status = "completed"
	s.mu.Unlock()
}

func (s *Span) AddEvent(name string, attrs map[string]interface{}) {
	// BUG: Not protecting concurrent access
	event := SpanEvent{
		Name:      name,
		Timestamp: time.Now(),
		Attrs:     attrs, // BUG: Sharing reference, not copying
	}
	s.Events = append(s.Events, event)
}

func processOrder(tracer *Tracer, parentSpan *Span) {
	// BUG: Not creating child span with parent context
	span := tracer.StartSpan(context.Background(), "ProcessOrder", "order-service")

	// Simulate processing
	time.Sleep(50 * time.Millisecond)

	// BUG: Race condition when accessing parent span
	parentSpan.Attributes["orderProcessed"] = true

	span.End()
}

func processMessage(tracer *Tracer, span *Span, msg string) {
	// Add event without protection
	span.AddEvent("MessageReceived", map[string]interface{}{
		"message": msg,
	})

	// Simulate processing
	time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)

	// BUG: Concurrent map write
	span.Attributes["messageProcessed"] = msg
}

func generateID() string {
	return fmt.Sprintf("%016x", rand.Int63())
}

// Missing implementations:
// 1. Proper context propagation
// 2. Span batching for export
// 3. Sampling decisions
// 4. Trace context injection/extraction for HTTP headers
// 5. Baggage propagation
// 6. Span status and error handling
// 7. Resource cleanup and limits
// 8. Metrics about tracing itself