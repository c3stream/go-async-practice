package examples

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Example12_CircuitBreaker - サーキットブレーカーパターン
func Example12_CircuitBreaker() {
	fmt.Println("=== Example 12: Circuit Breaker Pattern ===")

	type CircuitBreaker struct {
		mu            sync.RWMutex
		failures      int
		maxFailures   int
		state         string // "closed", "open", "half-open"
		lastFailTime  time.Time
		resetTimeout  time.Duration
		successCount  int
	}

	NewCircuitBreaker := func(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
		return &CircuitBreaker{
			maxFailures:  maxFailures,
			state:        "closed",
			resetTimeout: resetTimeout,
		}
	}

	Call := func(cb *CircuitBreaker, fn func() error) error {
		cb.mu.Lock()
		defer cb.mu.Unlock()

		// Check if we should try half-open
		if cb.state == "open" && time.Since(cb.lastFailTime) > cb.resetTimeout {
			cb.state = "half-open"
			cb.successCount = 0
			fmt.Println("Circuit breaker: half-open (testing recovery)")
		}

		if cb.state == "open" {
			return errors.New("circuit breaker is open")
		}

		err := fn()

		if err != nil {
			cb.failures++
			cb.lastFailTime = time.Now()

			if cb.state == "half-open" {
				cb.state = "open"
				fmt.Println("Circuit breaker: reopened after failure in half-open")
			} else if cb.failures >= cb.maxFailures {
				cb.state = "open"
				fmt.Printf("Circuit breaker: opened after %d failures\n", cb.failures)
			}
			return err
		}

		// Success
		if cb.state == "half-open" {
			cb.successCount++
			if cb.successCount >= 2 {
				cb.state = "closed"
				cb.failures = 0
				fmt.Println("Circuit breaker: closed (recovered)")
			}
		} else {
			cb.failures = 0
		}

		return nil
	}

	// デモ
	cb := NewCircuitBreaker(3, 2*time.Second)

	// 不安定なサービスをシミュレート
	unreliableService := func() error {
		if rand.Float32() < 0.6 {
			return errors.New("service error")
		}
		return nil
	}

	for i := 0; i < 10; i++ {
		err := Call(cb, unreliableService)
		if err != nil {
			fmt.Printf("Request %d failed: %v\n", i+1, err)
		} else {
			fmt.Printf("Request %d succeeded\n", i+1)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// Example13_PubSub - Pub/Subパターン
func Example13_PubSub() {
	fmt.Println("\n=== Example 13: Pub/Sub Pattern ===")

	type Message struct {
		Topic   string
		Payload interface{}
	}

	type PubSub struct {
		mu          sync.RWMutex
		subscribers map[string][]chan Message
		closed      bool
	}

	NewPubSub := func() *PubSub {
		return &PubSub{
			subscribers: make(map[string][]chan Message),
		}
	}

	Subscribe := func(ps *PubSub, topic string) <-chan Message {
		ps.mu.Lock()
		defer ps.mu.Unlock()

		ch := make(chan Message, 10)
		ps.subscribers[topic] = append(ps.subscribers[topic], ch)
		return ch
	}

	Publish := func(ps *PubSub, topic string, payload interface{}) {
		ps.mu.RLock()
		defer ps.mu.RUnlock()

		if ps.closed {
			return
		}

		msg := Message{
			Topic:   topic,
			Payload: payload,
		}

		for _, ch := range ps.subscribers[topic] {
			select {
			case ch <- msg:
			default:
				// Drop message if subscriber is slow
				fmt.Printf("Warning: Dropped message for slow subscriber on topic %s\n", topic)
			}
		}
	}

	Close := func(ps *PubSub) {
		ps.mu.Lock()
		defer ps.mu.Unlock()

		if ps.closed {
			return
		}
		ps.closed = true

		for _, subs := range ps.subscribers {
			for _, ch := range subs {
				close(ch)
			}
		}
	}

	// デモ
	ps := NewPubSub()
	defer Close(ps)

	var wg sync.WaitGroup

	// Subscriber 1: すべてのイベントを購読
	wg.Add(1)
	go func() {
		defer wg.Done()
		events := Subscribe(ps, "events")
		for msg := range events {
			fmt.Printf("Subscriber 1 received: %v\n", msg.Payload)
		}
	}()

	// Subscriber 2: ログのみを購読
	wg.Add(1)
	go func() {
		defer wg.Done()
		logs := Subscribe(ps, "logs")
		for msg := range logs {
			fmt.Printf("Subscriber 2 (logs): %v\n", msg.Payload)
		}
	}()

	// Publisher
	time.Sleep(100 * time.Millisecond)
	Publish(ps, "events", "Event 1")
	Publish(ps, "logs", "Log message 1")
	Publish(ps, "events", "Event 2")
	Publish(ps, "logs", "Log message 2")

	time.Sleep(100 * time.Millisecond)
	Close(ps)
	wg.Wait()
}

// Example14_BoundedParallelism - 制限付き並列処理
func Example14_BoundedParallelism() {
	fmt.Println("\n=== Example 14: Bounded Parallelism ===")

	type Task struct {
		ID     int
		Result int
		Error  error
	}

	processTasks := func(tasks []Task, maxWorkers int) []Task {
		taskChan := make(chan *Task)
		resultChan := make(chan *Task)

		// Start workers
		var wg sync.WaitGroup
		for i := 0; i < maxWorkers; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for task := range taskChan {
					// Simulate work
					fmt.Printf("Worker %d processing task %d\n", workerID, task.ID)
					time.Sleep(time.Duration(100+rand.Intn(100)) * time.Millisecond)

					task.Result = task.ID * task.ID
					resultChan <- task
				}
			}(i)
		}

		// Send tasks
		go func() {
			for i := range tasks {
				taskChan <- &tasks[i]
			}
			close(taskChan)
		}()

		// Collect results
		go func() {
			wg.Wait()
			close(resultChan)
		}()

		results := make([]Task, 0, len(tasks))
		for task := range resultChan {
			results = append(results, *task)
		}

		return results
	}

	// 20個のタスクを3つのワーカーで処理
	tasks := make([]Task, 20)
	for i := range tasks {
		tasks[i].ID = i
	}

	start := time.Now()
	results := processTasks(tasks, 3)
	fmt.Printf("\nProcessed %d tasks in %v\n", len(results), time.Since(start))
}

// Example15_Retry - リトライパターン
func Example15_Retry() {
	fmt.Println("\n=== Example 15: Retry Pattern with Exponential Backoff ===")

	type RetryConfig struct {
		MaxAttempts int
		InitialDelay time.Duration
		MaxDelay     time.Duration
		Multiplier   float64
	}

	RetryWithBackoff := func(ctx context.Context, config RetryConfig, fn func() error) error {
		delay := config.InitialDelay

		for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
			err := fn()
			if err == nil {
				fmt.Printf("Success on attempt %d\n", attempt)
				return nil
			}

			if attempt == config.MaxAttempts {
				return fmt.Errorf("max attempts (%d) exceeded: %w", config.MaxAttempts, err)
			}

			fmt.Printf("Attempt %d failed: %v. Retrying in %v...\n", attempt, err, delay)

			select {
			case <-time.After(delay):
				// Calculate next delay with exponential backoff
				delay = time.Duration(float64(delay) * config.Multiplier)
				if delay > config.MaxDelay {
					delay = config.MaxDelay
				}
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return errors.New("should not reach here")
	}

	// デモ
	ctx := context.Background()
	config := RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
	}

	attempts := 0
	unreliableFunc := func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	}

	err := RetryWithBackoff(ctx, config, unreliableFunc)
	if err != nil {
		fmt.Printf("Final error: %v\n", err)
	}
}

// Example16_BatchProcessor - バッチ処理パターン
func Example16_BatchProcessor() {
	fmt.Println("\n=== Example 16: Batch Processing Pattern ===")

	type BatchProcessor struct {
		batchSize    int
		flushInterval time.Duration
		processFn    func([]interface{})
		items        []interface{}
		itemsChan    chan interface{}
		done         chan struct{}
		mu           sync.Mutex
	}

	var flush func(bp *BatchProcessor)
	var run func(bp *BatchProcessor)

	flush = func(bp *BatchProcessor) {
		if len(bp.items) == 0 {
			return
		}

		batch := make([]interface{}, len(bp.items))
		copy(batch, bp.items)
		bp.items = bp.items[:0]

		go bp.processFn(batch)
	}

	run = func(bp *BatchProcessor) {
		ticker := time.NewTicker(bp.flushInterval)
		defer ticker.Stop()

		for {
			select {
			case item := <-bp.itemsChan:
				bp.mu.Lock()
				bp.items = append(bp.items, item)
				if len(bp.items) >= bp.batchSize {
					flush(bp)
				}
				bp.mu.Unlock()

			case <-ticker.C:
				bp.mu.Lock()
				if len(bp.items) > 0 {
					flush(bp)
				}
				bp.mu.Unlock()

			case <-bp.done:
				bp.mu.Lock()
				if len(bp.items) > 0 {
					flush(bp)
				}
				bp.mu.Unlock()
				return
			}
		}
	}

	NewBatchProcessor := func(batchSize int, flushInterval time.Duration, processFn func([]interface{})) *BatchProcessor {
		bp := &BatchProcessor{
			batchSize:     batchSize,
			flushInterval: flushInterval,
			processFn:     processFn,
			itemsChan:     make(chan interface{}, batchSize*2),
			done:          make(chan struct{}),
		}

		go run(bp)
		return bp
	}

	Add := func(bp *BatchProcessor, item interface{}) {
		select {
		case bp.itemsChan <- item:
		case <-time.After(100 * time.Millisecond):
			fmt.Println("Warning: Batch processor queue is full")
		}
	}

	Stop := func(bp *BatchProcessor) {
		close(bp.done)
		time.Sleep(100 * time.Millisecond)
	}

	// デモ
	processor := NewBatchProcessor(5, 1*time.Second, func(batch []interface{}) {
		fmt.Printf("Processing batch of %d items: %v\n", len(batch), batch)
	})

	// アイテムを追加
	for i := 0; i < 12; i++ {
		Add(processor, fmt.Sprintf("item-%d", i))
		time.Sleep(100 * time.Millisecond)
	}

	time.Sleep(2 * time.Second)
	Stop(processor)
}