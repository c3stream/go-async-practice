package benchmarks

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ==================== 基本的な同期プリミティブ ====================

// BenchmarkMutexVsRWMutex - Mutex vs RWMutex
func BenchmarkMutexVsRWMutex(b *testing.B) {
	data := make(map[int]int)

	b.Run("Mutex_ReadWrite", func(b *testing.B) {
		var mu sync.Mutex
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if rand.Float32() < 0.8 { // 80% read
					mu.Lock()
					_ = data[rand.Intn(100)]
					mu.Unlock()
				} else { // 20% write
					mu.Lock()
					data[rand.Intn(100)] = rand.Int()
					mu.Unlock()
				}
			}
		})
	})

	b.Run("RWMutex_ReadWrite", func(b *testing.B) {
		var mu sync.RWMutex
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if rand.Float32() < 0.8 { // 80% read
					mu.RLock()
					_ = data[rand.Intn(100)]
					mu.RUnlock()
				} else { // 20% write
					mu.Lock()
					data[rand.Intn(100)] = rand.Int()
					mu.Unlock()
				}
			}
		})
	})
}

// BenchmarkAtomicOperations - Atomic操作のベンチマーク
func BenchmarkAtomicOperations(b *testing.B) {
	b.Run("AtomicInt64", func(b *testing.B) {
		var counter int64
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				atomic.AddInt64(&counter, 1)
			}
		})
	})

	b.Run("AtomicCompareAndSwap", func(b *testing.B) {
		var value int64
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				for {
					old := atomic.LoadInt64(&value)
					if atomic.CompareAndSwapInt64(&value, old, old+1) {
						break
					}
				}
			}
		})
	})

	b.Run("AtomicValue", func(b *testing.B) {
		var v atomic.Value
		v.Store(0)
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				v.Store(rand.Int())
				_ = v.Load()
			}
		})
	})
}

// ==================== チャネルパターン ====================

// BenchmarkChannelPatterns - 各種チャネルパターン
func BenchmarkChannelPatterns(b *testing.B) {
	b.Run("UnbufferedChannel", func(b *testing.B) {
		ch := make(chan int)
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

	b.Run("BufferedChannel_10", func(b *testing.B) {
		ch := make(chan int, 10)
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

	b.Run("BufferedChannel_100", func(b *testing.B) {
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

	b.Run("SelectDefault", func(b *testing.B) {
		ch := make(chan int, 1)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			select {
			case ch <- i:
			default:
			}
		}
	})
}

// ==================== ワーカープール ====================

// BenchmarkWorkerPoolSizes - 異なるワーカー数でのパフォーマンス
func BenchmarkWorkerPoolSizes(b *testing.B) {
	workload := func() {
		time.Sleep(time.Microsecond)
	}

	runWorkerPool := func(b *testing.B, numWorkers int) {
		tasks := make(chan func(), 100)
		var wg sync.WaitGroup

		// ワーカー起動
		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for task := range tasks {
					task()
				}
			}()
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			tasks <- workload
		}
		close(tasks)
		wg.Wait()
	}

	for _, workers := range []int{1, 2, 4, 8, 16, 32} {
		b.Run(fmt.Sprintf("Workers_%d", workers), func(b *testing.B) {
			runWorkerPool(b, workers)
		})
	}
}

// ==================== コンテキスト ====================

// BenchmarkContextOperations - Context操作のベンチマーク
func BenchmarkContextOperations(b *testing.B) {
	b.Run("WithCancel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			<-ctx.Done()
		}
	})

	b.Run("WithTimeout", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
			cancel()
			<-ctx.Done()
		}
	})

	b.Run("WithValue", func(b *testing.B) {
		ctx := context.Background()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx = context.WithValue(ctx, "key", i)
			_ = ctx.Value("key")
		}
	})
}

// ==================== 同期パターン比較 ====================

// BenchmarkSyncPatterns - 同期パターンの比較
func BenchmarkSyncPatterns(b *testing.B) {
	// WaitGroup
	b.Run("WaitGroup", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup
			for j := 0; j < 10; j++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					time.Sleep(time.Nanosecond)
				}()
			}
			wg.Wait()
		}
	})

	// Channel同期
	b.Run("ChannelSync", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			done := make(chan bool, 10)
			for j := 0; j < 10; j++ {
				go func() {
					time.Sleep(time.Nanosecond)
					done <- true
				}()
			}
			for j := 0; j < 10; j++ {
				<-done
			}
		}
	})

	// Context同期
	b.Run("ContextSync", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithCancel(context.Background())
			for j := 0; j < 10; j++ {
				go func() {
					<-ctx.Done()
				}()
			}
			cancel()
		}
	})
}

// ==================== メモリアロケーション ====================

// BenchmarkMemoryAllocation - メモリアロケーションの影響
func BenchmarkMemoryAllocation(b *testing.B) {
	b.Run("SlicePreAlloc", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			slice := make([]int, 0, 1000)
			for j := 0; j < 1000; j++ {
				slice = append(slice, j)
			}
		}
	})

	b.Run("SliceNoPreAlloc", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var slice []int
			for j := 0; j < 1000; j++ {
				slice = append(slice, j)
			}
		}
	})

	b.Run("MapPreSize", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			m := make(map[int]int, 1000)
			for j := 0; j < 1000; j++ {
				m[j] = j
			}
		}
	})

	b.Run("MapNoPreSize", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			m := make(map[int]int)
			for j := 0; j < 1000; j++ {
				m[j] = j
			}
		}
	})
}

// ==================== 高度なパターン ====================

// BenchmarkAdvancedPatterns - 高度な並行パターン
func BenchmarkAdvancedPatterns(b *testing.B) {
	// Pipeline
	b.Run("Pipeline_3Stages", func(b *testing.B) {
		stage1 := func(in <-chan int) <-chan int {
			out := make(chan int, 100)
			go func() {
				for n := range in {
					out <- n * 2
				}
				close(out)
			}()
			return out
		}

		stage2 := func(in <-chan int) <-chan int {
			out := make(chan int, 100)
			go func() {
				for n := range in {
					out <- n + 10
				}
				close(out)
			}()
			return out
		}

		stage3 := func(in <-chan int) {
			for range in {
			}
		}

		b.ResetTimer()
		input := make(chan int, 100)
		go func() {
			for i := 0; i < b.N; i++ {
				input <- i
			}
			close(input)
		}()

		stage3(stage2(stage1(input)))
	})

	// Fan-out/Fan-in
	b.Run("FanOutFanIn", func(b *testing.B) {
		input := make(chan int, 100)

		fanOut := func(in <-chan int, workers int) []<-chan int {
			outs := make([]<-chan int, workers)
			for i := 0; i < workers; i++ {
				out := make(chan int, 100)
				outs[i] = out
				go func(out chan int) {
					for n := range in {
						out <- n * 2
					}
					close(out)
				}(out)
			}
			return outs
		}

		fanIn := func(channels []<-chan int) <-chan int {
			out := make(chan int, 100)
			var wg sync.WaitGroup
			for _, ch := range channels {
				wg.Add(1)
				go func(c <-chan int) {
					defer wg.Done()
					for n := range c {
						out <- n
					}
				}(ch)
			}
			go func() {
				wg.Wait()
				close(out)
			}()
			return out
		}

		b.ResetTimer()
		go func() {
			for i := 0; i < b.N; i++ {
				input <- i
			}
			close(input)
		}()

		result := fanIn(fanOut(input, 4))
		for range result {
		}
	})
}

// ==================== 実世界シミュレーション ====================

// BenchmarkRealWorldScenarios - 実世界のシナリオ
func BenchmarkRealWorldScenarios(b *testing.B) {
	// HTTP Server Simulation
	b.Run("HTTPServer_Simulation", func(b *testing.B) {
		type Request struct {
			ID   int
			Data string
		}

		type Response struct {
			ID     int
			Result string
		}

		handler := func(req Request) Response {
			// 処理をシミュレート
			time.Sleep(time.Microsecond)
			return Response{
				ID:     req.ID,
				Result: fmt.Sprintf("Processed: %s", req.Data),
			}
		}

		requests := make(chan Request, 100)
		responses := make(chan Response, 100)

		// ワーカープール
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for req := range requests {
					responses <- handler(req)
				}
			}()
		}

		b.ResetTimer()
		go func() {
			for i := 0; i < b.N; i++ {
				requests <- Request{ID: i, Data: "test"}
			}
			close(requests)
		}()

		go func() {
			wg.Wait()
			close(responses)
		}()

		for range responses {
		}
	})

	// Database Connection Pool
	b.Run("DBConnectionPool", func(b *testing.B) {
		type Connection struct {
			ID   int
			InUse bool
			mu   sync.Mutex
		}

		pool := make([]*Connection, 10)
		for i := range pool {
			pool[i] = &Connection{ID: i}
		}

		getConnection := func() *Connection {
			for {
				for _, conn := range pool {
					conn.mu.Lock()
					if !conn.InUse {
						conn.InUse = true
						conn.mu.Unlock()
						return conn
					}
					conn.mu.Unlock()
				}
				runtime.Gosched()
			}
		}

		releaseConnection := func(conn *Connection) {
			conn.mu.Lock()
			conn.InUse = false
			conn.mu.Unlock()
		}

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				conn := getConnection()
				time.Sleep(time.Nanosecond) // クエリをシミュレート
				releaseConnection(conn)
			}
		})
	})
}

// ==================== ベンチマーク結果の分析 ====================

// PrintBenchmarkSummary - ベンチマーク結果のサマリーを出力
func PrintBenchmarkSummary(b *testing.B) {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║     📊 ベンチマーク結果サマリー         ║")
	fmt.Println("╚════════════════════════════════════════╝")

	fmt.Println("\n推奨事項:")
	fmt.Println("  • 読み込みが多い場合はRWMutexを使用")
	fmt.Println("  • 単純なカウンタにはatomicを使用")
	fmt.Println("  • チャネルは適切なバッファサイズを設定")
	fmt.Println("  • ワーカー数はCPUコア数の2-4倍が効率的")
	fmt.Println("  • メモリは事前割り当てで性能向上")
}