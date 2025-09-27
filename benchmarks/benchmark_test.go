package benchmarks

import (
	"sync"
	"sync/atomic"
	"testing"
)

// BenchmarkMutexVsChannel - MutexとChannelのパフォーマンス比較
func BenchmarkMutexCounter(b *testing.B) {
	var counter int
	var mu sync.Mutex

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			counter++
			mu.Unlock()
		}
	})
}

func BenchmarkChannelCounter(b *testing.B) {
	type op struct {
		delta  int
		result chan int
	}
	ops := make(chan op, 100)

	go func() {
		counter := 0
		for op := range ops {
			counter += op.delta
			if op.result != nil {
				op.result <- counter
			}
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ops <- op{delta: 1}
		}
	})
}

func BenchmarkAtomicCounter(b *testing.B) {
	var counter int64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			atomic.AddInt64(&counter, 1)
		}
	})
}

// BenchmarkBufferedVsUnbuffered - バッファ付きチャネル vs バッファなしチャネル
func BenchmarkUnbufferedChannel(b *testing.B) {
	ch := make(chan int)

	go func() {
		for range ch {
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch <- i
	}
}

func BenchmarkBufferedChannel(b *testing.B) {
	ch := make(chan int, 100)

	go func() {
		for range ch {
		}
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch <- i
	}
}

// BenchmarkGoroutineCreation - ゴルーチン作成のオーバーヘッド
func BenchmarkGoroutineCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
		}()
		wg.Wait()
	}
}

func BenchmarkGoroutinePool(b *testing.B) {
	workerCount := 10
	jobs := make(chan func(), 100)

	// ワーカープール作成
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				job()
			}
		}()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		done := make(chan struct{})
		jobs <- func() {
			close(done)
		}
		<-done
	}

	close(jobs)
	wg.Wait()
}

// BenchmarkSelectVsIf - select vs if-else のパフォーマンス
func BenchmarkSelectDefault(b *testing.B) {
	ch := make(chan int, 1)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		select {
		case ch <- i:
		default:
		}

		select {
		case <-ch:
		default:
		}
	}
}

func BenchmarkIfElse(b *testing.B) {
	ch := make(chan int, 1)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if len(ch) < cap(ch) {
			ch <- i
		}

		if len(ch) > 0 {
			<-ch
		}
	}
}

// BenchmarkMapVsSyncMap - 通常のmap+Mutex vs sync.Map
func BenchmarkMapWithMutex(b *testing.B) {
	m := make(map[int]int)
	var mu sync.RWMutex

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Write
			mu.Lock()
			m[i%100] = i
			mu.Unlock()

			// Read
			mu.RLock()
			_ = m[i%100]
			mu.RUnlock()

			i++
		}
	})
}

func BenchmarkSyncMap(b *testing.B) {
	var m sync.Map

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Write
			m.Store(i%100, i)

			// Read
			m.Load(i % 100)

			i++
		}
	})
}