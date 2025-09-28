package tests

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestConcurrencyPerformance - 並行処理のパフォーマンステスト
func TestConcurrencyPerformance(t *testing.T) {
	tests := []struct {
		name      string
		workers   int
		tasks     int
		workload  time.Duration
	}{
		{"Light_10Workers_100Tasks", 10, 100, 1 * time.Millisecond},
		{"Medium_50Workers_1000Tasks", 50, 1000, 5 * time.Millisecond},
		{"Heavy_100Workers_5000Tasks", 100, 5000, 10 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()

			var wg sync.WaitGroup
			tasks := make(chan int, tt.tasks)
			var processed int64

			// ワーカー起動
			for i := 0; i < tt.workers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for task := range tasks {
						// 作業をシミュレート
						time.Sleep(tt.workload)
						atomic.AddInt64(&processed, 1)
						_ = task
					}
				}()
			}

			// タスク投入
			for i := 0; i < tt.tasks; i++ {
				tasks <- i
			}
			close(tasks)

			wg.Wait()
			elapsed := time.Since(start)

			t.Logf("Processed %d tasks in %v (%.2f tasks/sec)",
				processed, elapsed, float64(processed)/elapsed.Seconds())

			// パフォーマンス基準
			expectedTime := time.Duration(tt.tasks/tt.workers) * tt.workload * 2
			if elapsed > expectedTime {
				t.Errorf("Performance too slow: %v > %v", elapsed, expectedTime)
			}
		})
	}
}

// TestMemoryLeakDetection - メモリリーク検出テスト
func TestMemoryLeakDetection(t *testing.T) {
	var m runtime.MemStats

	// ベースラインメモリ使用量
	runtime.GC()
	runtime.ReadMemStats(&m)
	baseline := m.Alloc

	// メモリリークの可能性があるコード
	leakyFunction := func() {
		channels := make([]chan int, 100)
		for i := range channels {
			channels[i] = make(chan int, 100)
			// チャネルに送信するが受信しない（リーク）
			go func(ch chan int) {
				for j := 0; j < 100; j++ {
					ch <- j
				}
				// close(ch) // これがないとリーク
			}(channels[i])
		}
		// チャネルをクローズしない
		time.Sleep(100 * time.Millisecond)
	}

	// 複数回実行
	for i := 0; i < 10; i++ {
		leakyFunction()
	}

	// メモリ使用量を再測定
	runtime.GC()
	runtime.ReadMemStats(&m)
	leaked := m.Alloc - baseline

	t.Logf("Memory baseline: %d bytes", baseline)
	t.Logf("Memory after: %d bytes", m.Alloc)
	t.Logf("Potential leak: %d bytes", leaked)

	// メモリリークの閾値チェック（10MB以上はエラー）
	if leaked > 10*1024*1024 {
		t.Errorf("Memory leak detected: %d bytes leaked", leaked)
	}
}

// TestRaceConditionDetection - レース条件検出テスト
func TestRaceConditionDetection(t *testing.T) {
	// このテストは -race フラグ付きで実行する必要がある
	// go test -race ./tests

	t.Run("UnsafeCounter", func(t *testing.T) {
		var counter int
		var wg sync.WaitGroup

		// レース条件あり（意図的）
		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				counter++ // データレース！
			}()
		}
		wg.Wait()

		// カウンタが正確でない可能性
		if counter != 1000 {
			t.Logf("Race condition detected: counter = %d (expected 1000)", counter)
		}
	})

	t.Run("SafeCounter", func(t *testing.T) {
		var counter int64
		var wg sync.WaitGroup

		// レース条件なし（atomic使用）
		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				atomic.AddInt64(&counter, 1)
			}()
		}
		wg.Wait()

		if counter != 1000 {
			t.Errorf("Unexpected counter value: %d", counter)
		}
	})
}

// BenchmarkChannelVsSync - チャネル vs 同期プリミティブのベンチマーク
func BenchmarkChannelVsSync(b *testing.B) {
	b.Run("Channel", func(b *testing.B) {
		ch := make(chan int, 100)
		go func() {
			for range ch {
			}
		}()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ch <- i
		}
		close(ch)
	})

	b.Run("Mutex", func(b *testing.B) {
		var mu sync.Mutex
		var value int

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			mu.Lock()
			value = i
			mu.Unlock()
		}
		_ = value
	})

	b.Run("Atomic", func(b *testing.B) {
		var value int64

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			atomic.StoreInt64(&value, int64(i))
		}
	})
}

// TestLoadBalancing - 負荷分散テスト
func TestLoadBalancing(t *testing.T) {
	numWorkers := 5
	numTasks := 100

	workerLoads := make([]int64, numWorkers)
	tasks := make(chan int, numTasks)

	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for range tasks {
				atomic.AddInt64(&workerLoads[workerID], 1)
				time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
			}
		}(i)
	}

	// タスク投入
	for i := 0; i < numTasks; i++ {
		tasks <- i
	}
	close(tasks)
	wg.Wait()

	// 負荷分散の検証
	var total int64
	var min, max int64 = int64(numTasks), 0

	for i, load := range workerLoads {
		t.Logf("Worker %d processed %d tasks", i, load)
		total += load
		if load < min {
			min = load
		}
		if load > max {
			max = load
		}
	}

	// 統計
	avg := total / int64(numWorkers)
	imbalance := float64(max-min) / float64(avg) * 100

	t.Logf("Total tasks: %d", total)
	t.Logf("Average per worker: %d", avg)
	t.Logf("Min/Max: %d/%d", min, max)
	t.Logf("Imbalance: %.2f%%", imbalance)

	// 負荷の偏りが50%以上ある場合は警告
	if imbalance > 50 {
		t.Logf("Warning: High load imbalance detected: %.2f%%", imbalance)
	}
}

// TestStressTest - ストレステスト
func TestStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	// システムリソースの監視
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	initialMem := m.Alloc
	initialGoroutines := runtime.NumGoroutine()

	fmt.Printf("Initial state: Memory=%d KB, Goroutines=%d\n",
		initialMem/1024, initialGoroutines)

	// 高負荷をかける
	stressLevel := 10000
	var wg sync.WaitGroup
	errors := make(chan error, stressLevel)

	start := time.Now()
	for i := 0; i < stressLevel; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// ランダムな処理
			switch rand.Intn(4) {
			case 0:
				// CPU負荷
				sum := 0
				for j := 0; j < 1000; j++ {
					sum += j * j
				}
			case 1:
				// メモリ割り当て
				data := make([]byte, 1024)
				_ = data
			case 2:
				// チャネル操作
				ch := make(chan int, 1)
				ch <- id
				<-ch
			case 3:
				// スリープ
				time.Sleep(time.Microsecond)
			}
		}(i)

		// 並行数を制限
		if i%100 == 0 {
			time.Sleep(time.Millisecond)
		}
	}

	wg.Wait()
	close(errors)

	elapsed := time.Since(start)

	// 最終状態
	runtime.ReadMemStats(&m)
	finalMem := m.Alloc
	finalGoroutines := runtime.NumGoroutine()

	fmt.Printf("Final state: Memory=%d KB, Goroutines=%d\n",
		finalMem/1024, finalGoroutines)
	fmt.Printf("Stress test completed: %d operations in %v\n",
		stressLevel, elapsed)
	fmt.Printf("Operations per second: %.2f\n",
		float64(stressLevel)/elapsed.Seconds())

	// ゴルーチンリークチェック
	if finalGoroutines > initialGoroutines+10 {
		t.Errorf("Potential goroutine leak: %d -> %d",
			initialGoroutines, finalGoroutines)
	}
}