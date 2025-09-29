// Challenge 30: Reactive Streams Implementation
//
// Problem: The reactive streams implementation has critical issues:
// 1. Backpressure is not properly implemented
// 2. Publishers can overwhelm slow subscribers
// 3. Error propagation is broken
// 4. Hot vs Cold observables are confused
// 5. Memory leaks from unsubscribed streams
// 6. Operators don't maintain thread safety
// 7. Completion signals are lost
//
// Your task: Fix the reactive streams to comply with reactive streams specification.

package challenges

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Publisher emits items to subscribers
type Publisher interface {
	Subscribe(subscriber Subscriber) Subscription
}

// Subscriber receives items from publishers
type Subscriber interface {
	OnNext(item interface{})
	OnError(err error)
	OnComplete()
	OnSubscribe(subscription Subscription)
}

// Subscription represents a link between publisher and subscriber
type Subscription interface {
	Request(n int64)
	Cancel()
}

// Processor is both a Subscriber and a Publisher
type Processor interface {
	Subscriber
	Publisher
}

// FlowablePublisher implements a reactive publisher
type FlowablePublisher struct {
	source      func(emitter Emitter)
	subscribers []subscriberWrapper
	hot         bool // Hot vs Cold observable
	mu          sync.RWMutex
	completed   int32
}

// Emitter allows publishing items
type Emitter interface {
	Next(item interface{})
	Error(err error)
	Complete()
	IsCancelled() bool
}

// subscriberWrapper wraps a subscriber with its subscription
type subscriberWrapper struct {
	subscriber   Subscriber
	subscription *flowableSubscription
}

// flowableSubscription implements Subscription
type flowableSubscription struct {
	subscriber  Subscriber
	publisher   *FlowablePublisher
	requested   int64
	cancelled   int32
	buffer      chan interface{} // BUG: Unbounded buffer
	errorBuffer chan error       // BUG: Error handling issues
	mu          sync.Mutex
}

// flowableEmitter implements Emitter
type flowableEmitter struct {
	subscription *flowableSubscription
	completed    int32
}

// NewFlowablePublisher creates a new publisher
func NewFlowablePublisher(source func(emitter Emitter), hot bool) *FlowablePublisher {
	return &FlowablePublisher{
		source:      source,
		subscribers: make([]subscriberWrapper, 0),
		hot:         hot,
	}
}

// Subscribe adds a new subscriber
func (p *FlowablePublisher) Subscribe(subscriber Subscriber) Subscription {
	// BUG: No check for nil subscriber
	subscription := &flowableSubscription{
		subscriber:  subscriber,
		publisher:   p,
		buffer:      make(chan interface{}, 100), // BUG: Fixed buffer size
		errorBuffer: make(chan error, 1),
	}

	wrapper := subscriberWrapper{
		subscriber:   subscriber,
		subscription: subscription,
	}

	p.mu.Lock()
	p.subscribers = append(p.subscribers, wrapper)
	p.mu.Unlock()

	// Call onSubscribe
	subscriber.OnSubscribe(subscription)

	// BUG: Hot observables should share the same stream
	if p.hot {
		// BUG: Starting new goroutine for each subscriber even for hot observables
		go p.startEmitting(subscription)
	} else {
		go p.startEmitting(subscription)
	}

	return subscription
}

// startEmitting starts emitting items
func (p *FlowablePublisher) startEmitting(subscription *flowableSubscription) {
	emitter := &flowableEmitter{
		subscription: subscription,
	}

	// BUG: No panic recovery
	p.source(emitter)
}

// Request requests n items
func (s *flowableSubscription) Request(n int64) {
	// BUG: No validation of n
	atomic.AddInt64(&s.requested, n)

	// BUG: Doesn't actually trigger emission
	go s.drain()
}

// drain sends buffered items to subscriber
func (s *flowableSubscription) drain() {
	// BUG: No backpressure implementation
	for {
		select {
		case item := <-s.buffer:
			// BUG: Not checking requested count
			s.subscriber.OnNext(item)

		case err := <-s.errorBuffer:
			// BUG: Continues after error
			s.subscriber.OnError(err)

		default:
			return
		}
	}
}

// Cancel cancels the subscription
func (s *flowableSubscription) Cancel() {
	// BUG: Doesn't properly clean up resources
	atomic.StoreInt32(&s.cancelled, 1)
	// BUG: Channels are not closed
}

// Next emits a new item
func (e *flowableEmitter) Next(item interface{}) {
	// BUG: No check if completed or cancelled
	select {
	case e.subscription.buffer <- item:
		// Item buffered
	default:
		// BUG: Item dropped silently
	}
}

// Error emits an error
func (e *flowableEmitter) Error(err error) {
	// BUG: Can emit multiple errors
	e.subscription.errorBuffer <- err
}

// Complete signals completion
func (e *flowableEmitter) Complete() {
	// BUG: Completion signal is not propagated properly
	atomic.StoreInt32(&e.completed, 1)
}

// IsCancelled checks if subscription is cancelled
func (e *flowableEmitter) IsCancelled() bool {
	return atomic.LoadInt32(&e.subscription.cancelled) == 1
}

// Operators

// MapOperator transforms items
type MapOperator struct {
	source    Publisher
	transform func(interface{}) interface{}
}

// Subscribe to MapOperator
func (m *MapOperator) Subscribe(subscriber Subscriber) Subscription {
	// BUG: Operator doesn't handle backpressure
	mapSubscriber := &MapSubscriber{
		downstream: subscriber,
		transform:  m.transform,
	}

	return m.source.Subscribe(mapSubscriber)
}

// MapSubscriber implements transformed subscription
type MapSubscriber struct {
	downstream   Subscriber
	upstream     Subscription
	transform    func(interface{}) interface{}
	mu           sync.Mutex // BUG: Not used properly
}

func (m *MapSubscriber) OnNext(item interface{}) {
	// BUG: Transform can panic
	transformed := m.transform(item)
	m.downstream.OnNext(transformed)
}

func (m *MapSubscriber) OnError(err error) {
	m.downstream.OnError(err)
}

func (m *MapSubscriber) OnComplete() {
	m.downstream.OnComplete()
}

func (m *MapSubscriber) OnSubscribe(subscription Subscription) {
	m.upstream = subscription
	m.downstream.OnSubscribe(subscription)
}

// FilterOperator filters items
type FilterOperator struct {
	source    Publisher
	predicate func(interface{}) bool
}

// Subscribe to FilterOperator
func (f *FilterOperator) Subscribe(subscriber Subscriber) Subscription {
	// BUG: No null checks
	filterSubscriber := &FilterSubscriber{
		downstream: subscriber,
		predicate:  f.predicate,
	}

	return f.source.Subscribe(filterSubscriber)
}

// FilterSubscriber implements filtered subscription
type FilterSubscriber struct {
	downstream Subscriber
	upstream   Subscription
	predicate  func(interface{}) bool
}

func (f *FilterSubscriber) OnNext(item interface{}) {
	// BUG: Predicate can panic
	if f.predicate(item) {
		f.downstream.OnNext(item)
	}
	// BUG: Doesn't request more items after filtering
}

func (f *FilterSubscriber) OnError(err error) {
	f.downstream.OnError(err)
}

func (f *FilterSubscriber) OnComplete() {
	f.downstream.OnComplete()
}

func (f *FilterSubscriber) OnSubscribe(subscription Subscription) {
	f.upstream = subscription
	f.downstream.OnSubscribe(subscription)
}

// MergeOperator merges multiple publishers
type MergeOperator struct {
	sources []Publisher
}

// Subscribe to MergeOperator
func (m *MergeOperator) Subscribe(subscriber Subscriber) Subscription {
	// BUG: Doesn't coordinate between multiple sources
	mergeSubscriber := &MergeSubscriber{
		downstream: subscriber,
		sources:    m.sources,
	}

	// BUG: Subscribes to all sources immediately
	for _, source := range m.sources {
		source.Subscribe(mergeSubscriber)
	}

	// BUG: Returns nil subscription
	return nil
}

// MergeSubscriber implements merged subscription
type MergeSubscriber struct {
	downstream    Subscriber
	sources       []Publisher
	subscriptions []Subscription
	completed     int32
	mu            sync.Mutex
}

func (m *MergeSubscriber) OnNext(item interface{}) {
	// BUG: No synchronization
	m.downstream.OnNext(item)
}

func (m *MergeSubscriber) OnError(err error) {
	// BUG: One error terminates all
	m.downstream.OnError(err)
}

func (m *MergeSubscriber) OnComplete() {
	// BUG: Doesn't track which source completed
	m.downstream.OnComplete()
}

func (m *MergeSubscriber) OnSubscribe(subscription Subscription) {
	// BUG: Multiple subscriptions not handled
	m.subscriptions = append(m.subscriptions, subscription)
}

// TestSubscriber for testing
type TestSubscriber struct {
	received  []interface{}
	errors    []error
	completed bool
	mu        sync.Mutex
}

func (t *TestSubscriber) OnNext(item interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.received = append(t.received, item)
}

func (t *TestSubscriber) OnError(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errors = append(t.errors, err)
}

func (t *TestSubscriber) OnComplete() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.completed = true
}

func (t *TestSubscriber) OnSubscribe(subscription Subscription) {
	// Request unbounded items (bad practice)
	subscription.Request(9223372036854775807)
}

// Challenge30ReactiveStreams demonstrates the buggy reactive streams
func Challenge30ReactiveStreams() {
	fmt.Println("Challenge 30: Reactive Streams Implementation")
	fmt.Println("=============================================")

	// Create a publisher that emits numbers
	numberPublisher := NewFlowablePublisher(func(emitter Emitter) {
		for i := 0; i < 100; i++ {
			// BUG: No check if cancelled
			emitter.Next(i)
			time.Sleep(10 * time.Millisecond)
		}
		emitter.Complete()
	}, false)

	// Test basic subscription
	subscriber1 := &TestSubscriber{}
	sub1 := numberPublisher.Subscribe(subscriber1)

	// Request some items
	sub1.Request(10)

	time.Sleep(500 * time.Millisecond)

	// Create a hot publisher (shared stream)
	hotPublisher := NewFlowablePublisher(func(emitter Emitter) {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// BUG: No cancellation check
				emitter.Next(rand.Intn(100))
			}
		}
	}, true)

	// Multiple subscribers to hot publisher
	subscriber2 := &TestSubscriber{}
	subscriber3 := &TestSubscriber{}

	hotPublisher.Subscribe(subscriber2)
	time.Sleep(200 * time.Millisecond)
	hotPublisher.Subscribe(subscriber3)

	time.Sleep(500 * time.Millisecond)

	// Test operators
	mappedPublisher := &MapOperator{
		source: numberPublisher,
		transform: func(item interface{}) interface{} {
			// BUG: Type assertion can panic
			return item.(int) * 2
		},
	}

	subscriber4 := &TestSubscriber{}
	mappedPublisher.Subscribe(subscriber4)

	// Test filter operator
	filteredPublisher := &FilterOperator{
		source: numberPublisher,
		predicate: func(item interface{}) bool {
			// BUG: Type assertion can panic
			return item.(int)%2 == 0
		},
	}

	subscriber5 := &TestSubscriber{}
	filteredPublisher.Subscribe(subscriber5)

	// Create a publisher that errors
	errorPublisher := NewFlowablePublisher(func(emitter Emitter) {
		emitter.Next(1)
		emitter.Next(2)
		emitter.Error(fmt.Errorf("something went wrong"))
		emitter.Next(3) // BUG: Emitting after error
	}, false)

	subscriber6 := &TestSubscriber{}
	errorPublisher.Subscribe(subscriber6)

	time.Sleep(1 * time.Second)

	// Print results
	fmt.Printf("\nSubscriber 1 received: %d items\n", len(subscriber1.received))
	fmt.Printf("Subscriber 2 (hot) received: %d items\n", len(subscriber2.received))
	fmt.Printf("Subscriber 3 (hot) received: %d items\n", len(subscriber3.received))
	fmt.Printf("Subscriber 6 errors: %v\n", subscriber6.errors)

	fmt.Println("\nIssues to fix:")
	fmt.Println("1. Implement proper backpressure")
	fmt.Println("2. Fix hot vs cold observable behavior")
	fmt.Println("3. Proper error propagation and terminal state")
	fmt.Println("4. Memory leak prevention")
	fmt.Println("5. Thread-safe operators")
	fmt.Println("6. Completion signal handling")
	fmt.Println("7. Subscription lifecycle management")
	fmt.Println("8. Request(n) flow control")
}