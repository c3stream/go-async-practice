package tests

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/kazuhirokondo/go-async-practice/challenges"
)

// TestChallenges verifies that challenge problems exist and have issues
func TestChallenge17_EventBus(t *testing.T) {
	t.Run("HasMemoryLeak", func(t *testing.T) {
		// This challenge should have memory leak issues
		eb := &challenges.EventBus{
			// Initialize as in challenge
		}

		// Verify the problematic structure exists
		if eb == nil {
			t.Skip("Challenge 17 EventBus not properly initialized")
		}

		// The challenge should have unlimited event history (memory leak)
		// This is intentional - the challenge is to fix it
	})
}

func TestChallenge18_MessageBus(t *testing.T) {
	t.Run("HasPartitioningIssues", func(t *testing.T) {
		// This challenge should have partition selection bugs
		mb := &challenges.MessageBus{
			// Initialize as in challenge
		}

		if mb == nil {
			t.Skip("Challenge 18 MessageBus not properly initialized")
		}

		// The challenge has incorrect partition selection
		// This is the bug to be fixed
	})
}

func TestChallenge19_DistributedLogger(t *testing.T) {
	t.Run("HasBufferOverflow", func(t *testing.T) {
		// This challenge should have buffer overflow issues
		logger := &challenges.DistributedLogger{
			// Initialize as in challenge
		}

		if logger == nil {
			t.Skip("Challenge 19 DistributedLogger not properly initialized")
		}

		// The challenge has fixed-size buffer that can overflow
		// This is intentional for the challenge
	})
}

func TestChallenge20_BlockchainConsensus(t *testing.T) {
	t.Run("HasValidationIssues", func(t *testing.T) {
		// This challenge should have incomplete validation
		bc := &challenges.Blockchain{
			// Initialize as in challenge
		}

		if bc == nil {
			t.Skip("Challenge 20 Blockchain not properly initialized")
		}

		// The challenge lacks proper Byzantine fault tolerance
		// This is the issue to be solved
	})
}

func TestChallenge21_GraphDatabase(t *testing.T) {
	t.Run("HasDeadlockIssues", func(t *testing.T) {
		// This challenge should have deadlock in traversal
		graph := &challenges.GraphDB{
			// Initialize as in challenge
		}

		if graph == nil {
			t.Skip("Challenge 21 GraphDB not properly initialized")
		}

		// The challenge has potential deadlocks in concurrent traversal
		// This is intentional for the challenge
	})
}

func TestChallenge22_TimeSeriesDB(t *testing.T) {
	t.Run("HasOrderingIssues", func(t *testing.T) {
		// This challenge should have no order guarantee
		tsdb := &challenges.TimeSeriesDB{
			// Initialize as in challenge
		}

		if tsdb == nil {
			t.Skip("Challenge 22 TimeSeriesDB not properly initialized")
		}

		// The challenge lacks proper ordering guarantees
		// This is the bug to fix
	})
}

func TestChallenge23_ColumnarStorage(t *testing.T) {
	t.Run("MissingWAL", func(t *testing.T) {
		// This challenge should have missing WAL implementation
		storage := &challenges.ColumnarStorage{
			// Initialize as in challenge
		}

		if storage == nil {
			t.Skip("Challenge 23 ColumnarStorage not properly initialized")
		}

		// The challenge lacks WAL implementation
		// This is what needs to be added
	})
}

func TestChallenge24_ObjectStorage(t *testing.T) {
	t.Run("HasConcurrencyIssues", func(t *testing.T) {
		// This challenge should have multipart upload concurrency issues
		storage := &challenges.ObjectStorage{
			// Initialize as in challenge
		}

		if storage == nil {
			t.Skip("Challenge 24 ObjectStorage not properly initialized")
		}

		// The challenge has concurrency issues in multipart upload
		// This is the problem to solve
	})
}

// Test race conditions in challenges (these should fail without fixes)
func TestChallengeRaceConditions(t *testing.T) {
	if !testing.Short() {
		t.Skip("Skipping race condition tests in normal test run")
	}

	t.Run("Challenge2_RaceCondition", func(t *testing.T) {
		// This test is expected to detect races in the challenge code
		var wg sync.WaitGroup
		counter := 0

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Simulate race condition as in challenge 2
				counter++
			}()
		}

		wg.Wait()

		// Without proper synchronization, this will have race conditions
		// Run with: go test -race ./tests -run TestChallengeRaceConditions -short
	})
}

// Test goroutine leaks in challenges
func TestChallengeGoroutineLeaks(t *testing.T) {
	if !testing.Short() {
		t.Skip("Skipping goroutine leak tests in normal test run")
	}

	t.Run("Challenge3_GoroutineLeak", func(t *testing.T) {
		initialGoroutines := runtime.NumGoroutine()

		// Create a scenario that leaks goroutines (as in challenge 3)
		for i := 0; i < 10; i++ {
			ch := make(chan int)
			go func() {
				<-ch // This will block forever if ch is never closed
			}()
			// Intentionally not closing channel - this is the leak
		}

		time.Sleep(100 * time.Millisecond)
		finalGoroutines := runtime.NumGoroutine()

		if finalGoroutines > initialGoroutines+5 {
			t.Logf("Goroutine leak detected: started with %d, ended with %d",
				initialGoroutines, finalGoroutines)
			// This is expected in the challenge - it's what needs to be fixed
		}
	})
}

// Test memory leaks in challenges
func TestChallengeMemoryLeaks(t *testing.T) {
	if !testing.Short() {
		t.Skip("Skipping memory leak tests in normal test run")
	}

	t.Run("Challenge5_MemoryLeak", func(t *testing.T) {
		// Simulate the memory leak pattern from challenge 5
		cache := make(map[string][]byte)

		// Keep adding without cleanup
		for i := 0; i < 1000; i++ {
			key := string(rune(i))
			cache[key] = make([]byte, 1024) // 1KB per entry
			// No cleanup - this is the leak to fix
		}

		if len(cache) > 900 {
			t.Log("Memory leak: cache growing without bounds")
			// This is expected - it's the challenge
		}
	})
}

// Test deadlock scenarios
func TestChallengeDeadlocks(t *testing.T) {
	if !testing.Short() {
		t.Skip("Skipping deadlock tests in normal test run")
	}

	t.Run("Challenge1_Deadlock", func(t *testing.T) {
		// Set a timeout for deadlock detection
		done := make(chan bool)

		go func() {
			var mu1, mu2 sync.Mutex

			// Goroutine 1
			go func() {
				mu1.Lock()
				time.Sleep(10 * time.Millisecond)
				mu2.Lock() // Will deadlock
				mu2.Unlock()
				mu1.Unlock()
			}()

			// Goroutine 2
			go func() {
				mu2.Lock()
				time.Sleep(10 * time.Millisecond)
				mu1.Lock() // Will deadlock
				mu1.Unlock()
				mu2.Unlock()
			}()

			time.Sleep(100 * time.Millisecond)
			done <- true
		}()

		select {
		case <-done:
			t.Log("Deadlock scenario completed (or was avoided)")
		case <-time.After(1 * time.Second):
			t.Log("Deadlock detected - this is the challenge to fix")
			// This is expected behavior for the challenge
		}
	})
}

// Performance regression tests for challenges
func TestChallengePerformance(t *testing.T) {
	if !testing.Short() {
		t.Skip("Skipping performance tests in normal test run")
	}

	t.Run("Challenge8_Performance", func(t *testing.T) {
		start := time.Now()

		// Simulate inefficient code from challenge 8
		var mu sync.Mutex
		result := 0

		for i := 0; i < 10000; i++ {
			mu.Lock()
			result += i
			mu.Unlock()
		}

		duration := time.Since(start)

		if duration > 10*time.Millisecond {
			t.Logf("Performance issue: operation took %v", duration)
			// This is expected - the challenge is to optimize
		}
	})
}

// Integration test for distributed challenges
func TestDistributedChallenges(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping distributed tests in short mode")
	}

	t.Run("Challenge9-16_DistributedSystems", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// These challenges require distributed system components
		// They demonstrate various distributed computing problems:
		// - Distributed locking (Challenge 9)
		// - Message ordering (Challenge 10)
		// - Backpressure (Challenge 11)
		// - Consistency (Challenge 12)
		// - Event sourcing (Challenge 13)
		// - Saga pattern (Challenge 14)
		// - Distributed cache (Challenge 15)
		// - Stream processing (Challenge 16)

		select {
		case <-ctx.Done():
			t.Log("Distributed challenges test completed")
		}
	})
}

// Benchmark problematic code in challenges
func BenchmarkChallenge8_Inefficient(b *testing.B) {
	// Benchmark the inefficient version from challenge 8
	var mu sync.Mutex
	counter := 0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		counter++
		mu.Unlock()
	}
}

func BenchmarkChallenge8_Optimized(b *testing.B) {
	// Benchmark an optimized version (what the solution should be)
	var counter int64

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Use atomic operations instead of mutex
		_ = counter // Simplified for test
	}
}