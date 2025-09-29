package tests

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Comprehensive integration tests for all patterns
func TestAllPatternsIntegration(t *testing.T) {
	t.Run("BasicPatterns", func(t *testing.T) {
		testBasicConcurrencyPatterns(t)
	})

	t.Run("AdvancedPatterns", func(t *testing.T) {
		testAdvancedConcurrencyPatterns(t)
	})

	t.Run("DistributedPatterns", func(t *testing.T) {
		testDistributedSystemPatterns(t)
	})

	t.Run("EnterprisePatterns", func(t *testing.T) {
		testEnterprisePatterns(t)
	})
}

func testBasicConcurrencyPatterns(t *testing.T) {
	// Worker Pool Pattern
	t.Run("WorkerPool", func(t *testing.T) {
		jobs := make(chan int, 100)
		results := make(chan int, 100)

		// Start workers
		numWorkers := 5
		var wg sync.WaitGroup
		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for job := range jobs {
					results <- job * 2
				}
			}(w)
		}

		// Send jobs
		go func() {
			for j := 0; j < 50; j++ {
				jobs <- j
			}
			close(jobs)
		}()

		// Wait and collect
		go func() {
			wg.Wait()
			close(results)
		}()

		count := 0
		for range results {
			count++
		}

		if count != 50 {
			t.Errorf("Expected 50 results, got %d", count)
		}
	})

	// Pipeline Pattern
	t.Run("Pipeline", func(t *testing.T) {
		// Stage functions
		generate := func(nums ...int) <-chan int {
			out := make(chan int)
			go func() {
				for _, n := range nums {
					out <- n
				}
				close(out)
			}()
			return out
		}

		square := func(in <-chan int) <-chan int {
			out := make(chan int)
			go func() {
				for n := range in {
					out <- n * n
				}
				close(out)
			}()
			return out
		}

		// Setup pipeline
		nums := generate(2, 3, 4)
		squares := square(nums)

		// Consume
		var result []int
		for n := range squares {
			result = append(result, n)
		}

		expected := []int{4, 9, 16}
		if len(result) != len(expected) {
			t.Errorf("Pipeline failed: got %v, want %v", result, expected)
		}
	})

	// Fan-In/Fan-Out Pattern
	t.Run("FanInFanOut", func(t *testing.T) {
		// Fan-out to multiple workers
		work := make(chan int, 10)
		results := make(chan int, 10)

		var wg sync.WaitGroup
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for w := range work {
					results <- w * w
				}
			}()
		}

		// Send work
		go func() {
			for i := 1; i <= 5; i++ {
				work <- i
			}
			close(work)
		}()

		// Wait and close results
		go func() {
			wg.Wait()
			close(results)
		}()

		sum := 0
		for r := range results {
			sum += r
		}

		if sum != 55 { // 1+4+9+16+25
			t.Errorf("Fan-in/out sum: got %d, want 55", sum)
		}
	})
}

func testAdvancedConcurrencyPatterns(t *testing.T) {
	// Circuit Breaker Pattern
	t.Run("CircuitBreaker", func(t *testing.T) {
		type CircuitBreaker struct {
			failures  int32
			threshold int32
			state     int32 // 0=closed, 1=open, 2=half-open
		}

		cb := &CircuitBreaker{threshold: 3}

		call := func() error {
			state := atomic.LoadInt32(&cb.state)
			if state == 1 { // open
				return fmt.Errorf("circuit breaker open")
			}

			// Simulate failure
			failures := atomic.AddInt32(&cb.failures, 1)
			if failures >= cb.threshold {
				atomic.StoreInt32(&cb.state, 1)
			}
			return fmt.Errorf("service error")
		}

		// Test circuit breaking
		for i := 0; i < 5; i++ {
			err := call()
			if i >= 3 && err.Error() != "circuit breaker open" {
				t.Error("Circuit should be open after threshold")
			}
		}
	})

	// Rate Limiter Pattern
	t.Run("RateLimiter", func(t *testing.T) {
		type TokenBucket struct {
			tokens    int32
			capacity  int32
			refillRate time.Duration
			lastRefill time.Time
			mu        sync.Mutex
		}

		tb := &TokenBucket{
			tokens:     5,
			capacity:   5,
			refillRate: 100 * time.Millisecond,
			lastRefill: time.Now(),
		}

		tryAcquire := func() bool {
			tb.mu.Lock()
			defer tb.mu.Unlock()

			// Refill tokens
			now := time.Now()
			elapsed := now.Sub(tb.lastRefill)
			tokensToAdd := int32(elapsed / tb.refillRate)

			if tokensToAdd > 0 {
				tb.tokens = min32(tb.tokens+tokensToAdd, tb.capacity)
				tb.lastRefill = now
			}

			if tb.tokens > 0 {
				tb.tokens--
				return true
			}
			return false
		}

		// Test rate limiting
		allowed := 0
		for i := 0; i < 10; i++ {
			if tryAcquire() {
				allowed++
			}
			time.Sleep(50 * time.Millisecond)
		}

		if allowed == 0 || allowed == 10 {
			t.Errorf("Rate limiting not working: %d/10 allowed", allowed)
		}
	})

	// Semaphore Pattern
	t.Run("Semaphore", func(t *testing.T) {
		sem := make(chan struct{}, 3) // Allow 3 concurrent operations

		var active int32
		var maxActive int32
		var wg sync.WaitGroup

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				sem <- struct{}{} // Acquire
				defer func() { <-sem }() // Release

				current := atomic.AddInt32(&active, 1)
				defer atomic.AddInt32(&active, -1)

				// Track max concurrent
				for {
					max := atomic.LoadInt32(&maxActive)
					if current <= max || atomic.CompareAndSwapInt32(&maxActive, max, current) {
						break
					}
				}

				time.Sleep(10 * time.Millisecond)
			}()
		}

		wg.Wait()

		if maxActive > 3 {
			t.Errorf("Semaphore allowed %d concurrent, expected max 3", maxActive)
		}
	})
}

func testDistributedSystemPatterns(t *testing.T) {
	// Leader Election Pattern
	t.Run("LeaderElection", func(t *testing.T) {
		type Node struct {
			ID       int
			IsLeader bool
			mu       sync.Mutex
		}

		nodes := make([]*Node, 5)
		for i := 0; i < 5; i++ {
			nodes[i] = &Node{ID: i}
		}

		// Simple leader election (highest ID wins)
		elect := func() {
			maxID := -1
			var leader *Node

			for _, node := range nodes {
				if node.ID > maxID {
					maxID = node.ID
					leader = node
				}
			}

			for _, node := range nodes {
				node.mu.Lock()
				node.IsLeader = (node == leader)
				node.mu.Unlock()
			}
		}

		elect()

		// Verify exactly one leader
		leaders := 0
		for _, node := range nodes {
			node.mu.Lock()
			if node.IsLeader {
				leaders++
			}
			node.mu.Unlock()
		}

		if leaders != 1 {
			t.Errorf("Expected 1 leader, got %d", leaders)
		}
	})

	// Consistent Hashing Pattern
	t.Run("ConsistentHashing", func(t *testing.T) {
		type ConsistentHash struct {
			nodes map[int]string // hash -> node
			mu    sync.RWMutex
		}

		ch := &ConsistentHash{
			nodes: make(map[int]string),
		}

		// Simple hash function
		hash := func(key string) int {
			h := 0
			for _, c := range key {
				h = h*31 + int(c)
			}
			return h % 360 // Ring of 360 degrees
		}

		// Add nodes
		ch.mu.Lock()
		ch.nodes[0] = "node1"
		ch.nodes[120] = "node2"
		ch.nodes[240] = "node3"
		ch.mu.Unlock()

		// Get node for key
		getNode := func(key string) string {
			h := hash(key)

			ch.mu.RLock()
			defer ch.mu.RUnlock()

			// Find next node clockwise
			for i := 0; i < 360; i++ {
				if node, ok := ch.nodes[(h+i)%360]; ok {
					return node
				}
			}
			return ""
		}

		// Test distribution
		distribution := make(map[string]int)
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key%d", i)
			node := getNode(key)
			distribution[node]++
		}

		if len(distribution) != 3 {
			t.Errorf("Expected 3 nodes in distribution, got %d", len(distribution))
		}
	})

	// Two-Phase Commit Pattern
	t.Run("TwoPhaseCommit", func(t *testing.T) {
		type Participant struct {
			ID       int
			Prepared bool
			Committed bool
			mu       sync.Mutex
		}

		participants := make([]*Participant, 3)
		for i := 0; i < 3; i++ {
			participants[i] = &Participant{ID: i}
		}

		// Phase 1: Prepare
		allPrepared := true
		for _, p := range participants {
			p.mu.Lock()
			p.Prepared = true // Simulate prepare success
			if !p.Prepared {
				allPrepared = false
			}
			p.mu.Unlock()
		}

		// Phase 2: Commit or Abort
		if allPrepared {
			for _, p := range participants {
				p.mu.Lock()
				p.Committed = true
				p.mu.Unlock()
			}
		}

		// Verify all committed
		for _, p := range participants {
			p.mu.Lock()
			if !p.Committed {
				t.Errorf("Participant %d not committed", p.ID)
			}
			p.mu.Unlock()
		}
	})
}

func testEnterprisePatterns(t *testing.T) {
	// Event Bus Pattern
	t.Run("EventBus", func(t *testing.T) {
		type EventBus struct {
			subscribers map[string][]chan interface{}
			mu          sync.RWMutex
		}

		eb := &EventBus{
			subscribers: make(map[string][]chan interface{}),
		}

		// Subscribe
		subscribe := func(topic string) <-chan interface{} {
			ch := make(chan interface{}, 10)
			eb.mu.Lock()
			eb.subscribers[topic] = append(eb.subscribers[topic], ch)
			eb.mu.Unlock()
			return ch
		}

		// Publish
		publish := func(topic string, data interface{}) {
			eb.mu.RLock()
			subs := eb.subscribers[topic]
			eb.mu.RUnlock()

			for _, ch := range subs {
				select {
				case ch <- data:
				default: // Don't block
				}
			}
		}

		// Test
		sub1 := subscribe("test.topic")
		sub2 := subscribe("test.topic")

		publish("test.topic", "message")

		msg1 := <-sub1
		msg2 := <-sub2

		if msg1 != "message" || msg2 != "message" {
			t.Error("Event bus failed to deliver message")
		}
	})

	// Saga Pattern
	t.Run("SagaPattern", func(t *testing.T) {
		type SagaStep struct {
			Name        string
			Execute     func() error
			Compensate  func() error
			Executed    bool
			mu          sync.Mutex
		}

		steps := []*SagaStep{
			{
				Name:       "step1",
				Execute:    func() error { return nil },
				Compensate: func() error { return nil },
			},
			{
				Name:       "step2",
				Execute:    func() error { return nil },
				Compensate: func() error { return nil },
			},
			{
				Name:       "step3",
				Execute:    func() error { return fmt.Errorf("step3 failed") },
				Compensate: func() error { return nil },
			},
		}

		// Execute saga
		var failedStep int
		for i, step := range steps {
			step.mu.Lock()
			err := step.Execute()
			if err != nil {
				failedStep = i
				step.mu.Unlock()
				break
			}
			step.Executed = true
			step.mu.Unlock()
		}

		// Compensate on failure
		if failedStep > 0 {
			for i := failedStep - 1; i >= 0; i-- {
				steps[i].mu.Lock()
				if steps[i].Executed {
					steps[i].Compensate()
					steps[i].Executed = false
				}
				steps[i].mu.Unlock()
			}
		}

		// Verify rollback
		for _, step := range steps {
			step.mu.Lock()
			if step.Executed {
				t.Errorf("Step %s should be rolled back", step.Name)
			}
			step.mu.Unlock()
		}
	})

	// CQRS Pattern
	t.Run("CQRS", func(t *testing.T) {
		type Command struct {
			Type string
			Data interface{}
		}

		type Query struct {
			Type string
			ID   string
		}

		type Store struct {
			writeModel map[string]interface{}
			readModel  map[string]interface{}
			events     []interface{}
			mu         sync.RWMutex
		}

		store := &Store{
			writeModel: make(map[string]interface{}),
			readModel:  make(map[string]interface{}),
			events:     make([]interface{}, 0),
		}

		// Command handler
		handleCommand := func(cmd Command) {
			store.mu.Lock()
			defer store.mu.Unlock()

			// Update write model
			store.writeModel[cmd.Type] = cmd.Data

			// Emit event
			store.events = append(store.events, cmd)

			// Eventually update read model
			store.readModel[cmd.Type] = cmd.Data
		}

		// Query handler
		handleQuery := func(q Query) interface{} {
			store.mu.RLock()
			defer store.mu.RUnlock()
			return store.readModel[q.Type]
		}

		// Test
		handleCommand(Command{Type: "user", Data: "John"})
		result := handleQuery(Query{Type: "user"})

		if result != "John" {
			t.Error("CQRS pattern failed")
		}
	})
}

// Stress tests
func TestStressPatterns(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress tests in short mode")
	}

	t.Run("HighConcurrency", func(t *testing.T) {
		var counter int64
		var wg sync.WaitGroup

		// Launch many goroutines
		for i := 0; i < 10000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				atomic.AddInt64(&counter, 1)
			}()
		}

		wg.Wait()

		if counter != 10000 {
			t.Errorf("Counter mismatch: got %d, want 10000", counter)
		}
	})

	t.Run("ChannelStress", func(t *testing.T) {
		ch := make(chan int, 1000)
		done := make(chan bool)

		// Producer
		go func() {
			for i := 0; i < 100000; i++ {
				ch <- i
			}
			close(ch)
		}()

		// Consumer
		go func() {
			count := 0
			for range ch {
				count++
			}
			if count != 100000 {
				t.Errorf("Received %d messages, expected 100000", count)
			}
			done <- true
		}()

		select {
		case <-done:
			// Success
		case <-time.After(10 * time.Second):
			t.Error("Channel stress test timeout")
		}
	})
}

// Memory leak detection
func TestMemoryLeaks(t *testing.T) {
	detectLeak := func(name string, fn func()) {
		runtime.GC()
		before := runtime.NumGoroutine()

		fn()

		time.Sleep(100 * time.Millisecond)
		runtime.GC()
		after := runtime.NumGoroutine()

		if after > before {
			t.Errorf("%s: Possible goroutine leak (before: %d, after: %d)",
				name, before, after)
		}
	}

	t.Run("CleanFunction", func(t *testing.T) {
		detectLeak("clean", func() {
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
			}()
			wg.Wait()
		})
	})
}

// Benchmarks
func BenchmarkConcurrentCounter(b *testing.B) {
	var counter int64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			atomic.AddInt64(&counter, 1)
		}
	})
}

func BenchmarkChannelThroughput(b *testing.B) {
	ch := make(chan int, 100)
	done := make(chan bool)

	go func() {
		for range ch {
		}
		done <- true
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch <- i
	}
	close(ch)
	<-done
}

func BenchmarkMutexVsChannel(b *testing.B) {
	b.Run("Mutex", func(b *testing.B) {
		var mu sync.Mutex
		counter := 0

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mu.Lock()
				counter++
				mu.Unlock()
			}
		})
	})

	b.Run("Channel", func(b *testing.B) {
		ch := make(chan int, 1)
		ch <- 0

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				val := <-ch
				val++
				ch <- val
			}
		})
	})
}

// Helper functions
func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}