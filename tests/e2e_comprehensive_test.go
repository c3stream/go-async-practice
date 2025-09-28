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

// TestE2E_CompleteWorkflow - 完全なワークフローのE2Eテスト
func TestE2E_CompleteWorkflow(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"基本的な並行パターン", testBasicConcurrencyPatterns},
		{"高度な並行パターン", testAdvancedConcurrencyPatterns},
		{"エラーハンドリングと復旧", testErrorHandlingRecovery},
		{"パフォーマンスとスケーラビリティ", testPerformanceScalability},
		{"統合シナリオ", testIntegrationScenario},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

// testBasicConcurrencyPatterns - 基本的な並行パターンのテスト
func testBasicConcurrencyPatterns(t *testing.T) {
	t.Log("基本的な並行パターンのE2Eテスト開始")

	// Goroutineの基本
	t.Run("Goroutine基礎", func(t *testing.T) {
		done := make(chan bool)
		go func() {
			time.Sleep(100 * time.Millisecond)
			done <- true
		}()

		select {
		case <-done:
			t.Log("✅ Goroutine基礎パターン成功")
		case <-time.After(500 * time.Millisecond):
			t.Fatal("タイムアウト: Goroutine基礎")
		}
	})

	// Channel操作
	t.Run("Channel操作", func(t *testing.T) {
		ch := make(chan int, 5)

		// Producer
		go func() {
			for i := 0; i < 5; i++ {
				ch <- i
			}
			close(ch)
		}()

		// Consumer
		sum := 0
		for v := range ch {
			sum += v
		}

		if sum != 10 {
			t.Errorf("Channel操作失敗: 期待値=10, 実際=%d", sum)
		} else {
			t.Log("✅ Channel操作パターン成功")
		}
	})

	// Select文
	t.Run("Select文", func(t *testing.T) {
		ch1 := make(chan int)
		ch2 := make(chan int)
		done := make(chan bool)

		go func() {
			select {
			case v := <-ch1:
				if v != 1 {
					t.Errorf("ch1から予期しない値: %d", v)
				}
			case v := <-ch2:
				if v != 2 {
					t.Errorf("ch2から予期しない値: %d", v)
				}
			case <-time.After(1 * time.Second):
				t.Error("Select文タイムアウト")
			}
			done <- true
		}()

		ch1 <- 1
		<-done
		t.Log("✅ Select文パターン成功")
	})
}

// testAdvancedConcurrencyPatterns - 高度な並行パターンのテスト
func testAdvancedConcurrencyPatterns(t *testing.T) {
	t.Log("高度な並行パターンのE2Eテスト開始")

	// Worker Pool
	t.Run("Worker Pool", func(t *testing.T) {
		jobs := make(chan int, 100)
		results := make(chan int, 100)
		numWorkers := 5

		// Workers起動
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

		// Jobs送信
		numJobs := 50
		for j := 0; j < numJobs; j++ {
			jobs <- j
		}
		close(jobs)

		// 結果収集
		go func() {
			wg.Wait()
			close(results)
		}()

		count := 0
		for range results {
			count++
		}

		if count != numJobs {
			t.Errorf("Worker Pool失敗: 期待値=%d, 実際=%d", numJobs, count)
		} else {
			t.Log("✅ Worker Poolパターン成功")
		}
	})

	// Pipeline
	t.Run("Pipeline", func(t *testing.T) {
		// 3段パイプライン
		stage1 := make(chan int)
		stage2 := make(chan int)
		stage3 := make(chan int)

		// Stage 1: 数値生成
		go func() {
			for i := 1; i <= 10; i++ {
				stage1 <- i
			}
			close(stage1)
		}()

		// Stage 2: 2倍
		go func() {
			for val := range stage1 {
				stage2 <- val * 2
			}
			close(stage2)
		}()

		// Stage 3: +1
		go func() {
			for val := range stage2 {
				stage3 <- val + 1
			}
			close(stage3)
		}()

		// 結果確認
		results := []int{}
		for val := range stage3 {
			results = append(results, val)
		}

		if len(results) != 10 {
			t.Errorf("Pipelineパターン失敗: 結果数=%d", len(results))
		} else {
			t.Log("✅ Pipelineパターン成功")
		}
	})

	// Fan-In/Fan-Out
	t.Run("Fan-In/Fan-Out", func(t *testing.T) {
		// Fan-Out
		input := make(chan int)
		outputs := []chan int{}
		for i := 0; i < 3; i++ {
			out := make(chan int)
			outputs = append(outputs, out)

			go func(in <-chan int, out chan<- int) {
				for val := range in {
					out <- val * val
				}
				close(out)
			}(input, out)
		}

		// データ送信
		go func() {
			for i := 1; i <= 9; i++ {
				input <- i
			}
			close(input)
		}()

		// Fan-In
		result := make(chan int)
		var wg sync.WaitGroup
		for _, ch := range outputs {
			wg.Add(1)
			go func(c <-chan int) {
				defer wg.Done()
				for val := range c {
					result <- val
				}
			}(ch)
		}

		go func() {
			wg.Wait()
			close(result)
		}()

		count := 0
		for range result {
			count++
		}

		t.Logf("✅ Fan-In/Fan-Outパターン成功 (処理数: %d)", count)
	})
}

// testErrorHandlingRecovery - エラーハンドリングと復旧のテスト
func testErrorHandlingRecovery(t *testing.T) {
	t.Log("エラーハンドリングと復旧のE2Eテスト開始")

	// パニック復旧
	t.Run("パニック復旧", func(t *testing.T) {
		recovered := false
		done := make(chan bool)

		go func() {
			defer func() {
				if r := recover(); r != nil {
					recovered = true
					t.Logf("✅ パニック復旧成功: %v", r)
					done <- true
				}
			}()

			// 意図的なパニック
			panic("test panic")
		}()

		select {
		case <-done:
			if !recovered {
				t.Error("パニック復旧失敗")
			}
		case <-time.After(1 * time.Second):
			t.Error("パニック復旧タイムアウト")
		}
	})

	// Context キャンセル
	t.Run("Contextキャンセル", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		var cancelled bool
		done := make(chan bool)

		go func() {
			select {
			case <-ctx.Done():
				cancelled = true
				done <- true
			case <-time.After(2 * time.Second):
				done <- false
			}
		}()

		// 500ms後にキャンセル
		time.Sleep(500 * time.Millisecond)
		cancel()

		if <-done && cancelled {
			t.Log("✅ Contextキャンセル成功")
		} else {
			t.Error("Contextキャンセル失敗")
		}
	})

	// タイムアウト処理
	t.Run("タイムアウト処理", func(t *testing.T) {
		operation := func(ctx context.Context) error {
			select {
			case <-time.After(2 * time.Second):
				return fmt.Errorf("operation completed")
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err := operation(ctx)
		if err == context.DeadlineExceeded {
			t.Log("✅ タイムアウト処理成功")
		} else {
			t.Errorf("予期しないエラー: %v", err)
		}
	})

	// リトライロジック
	t.Run("リトライロジック", func(t *testing.T) {
		attempts := 0
		maxRetries := 3

		operation := func() error {
			attempts++
			if attempts < maxRetries {
				return fmt.Errorf("temporary error")
			}
			return nil
		}

		retryWithBackoff := func() error {
			for i := 0; i < maxRetries; i++ {
				if err := operation(); err == nil {
					return nil
				}
				time.Sleep(time.Duration(1<<uint(i)) * 10 * time.Millisecond)
			}
			return fmt.Errorf("max retries exceeded")
		}

		if err := retryWithBackoff(); err != nil {
			t.Errorf("リトライ失敗: %v", err)
		} else {
			t.Logf("✅ リトライ成功 (試行回数: %d)", attempts)
		}
	})
}

// testPerformanceScalability - パフォーマンスとスケーラビリティのテスト
func testPerformanceScalability(t *testing.T) {
	t.Log("パフォーマンスとスケーラビリティのE2Eテスト開始")

	// ゴルーチンのスケーラビリティ
	t.Run("ゴルーチンスケーラビリティ", func(t *testing.T) {
		initialGoroutines := runtime.NumGoroutine()

		var wg sync.WaitGroup
		numGoroutines := 10000

		start := time.Now()
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				time.Sleep(10 * time.Millisecond)
			}(i)
		}

		// 起動中のゴルーチン数確認
		midGoroutines := runtime.NumGoroutine()
		t.Logf("起動中のゴルーチン数: %d", midGoroutines-initialGoroutines)

		wg.Wait()
		elapsed := time.Since(start)

		finalGoroutines := runtime.NumGoroutine()
		t.Logf("処理時間: %v", elapsed)
		t.Logf("ゴルーチンリーク: %d", finalGoroutines-initialGoroutines)

		if finalGoroutines-initialGoroutines > 10 {
			t.Errorf("ゴルーチンリークの可能性: %d", finalGoroutines-initialGoroutines)
		} else {
			t.Log("✅ ゴルーチンスケーラビリティ成功")
		}
	})

	// Channel throughput
	t.Run("チャネルスループット", func(t *testing.T) {
		ch := make(chan int, 1000)
		numMessages := 100000
		numWorkers := 10

		var sent, received int64

		// Producers
		var wg sync.WaitGroup
		start := time.Now()

		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < numMessages/numWorkers; i++ {
					ch <- i
					atomic.AddInt64(&sent, 1)
				}
			}()
		}

		// Consumers
		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-ch:
						if atomic.AddInt64(&received, 1) >= int64(numMessages) {
							return
						}
					case <-time.After(100 * time.Millisecond):
						return
					}
				}
			}()
		}

		wg.Wait()
		elapsed := time.Since(start)

		throughput := float64(numMessages) / elapsed.Seconds()
		t.Logf("スループット: %.2f messages/sec", throughput)
		t.Logf("送信: %d, 受信: %d", sent, received)

		if received < int64(numMessages)*95/100 {
			t.Errorf("メッセージロス: %d/%d", received, numMessages)
		} else {
			t.Log("✅ チャネルスループット成功")
		}
	})

	// メモリ使用量
	t.Run("メモリ使用量", func(t *testing.T) {
		var m runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&m)
		initialMem := m.Alloc

		// メモリ集約的な処理
		data := make([][]byte, 100)
		for i := range data {
			data[i] = make([]byte, 1024*1024) // 1MB
		}

		runtime.ReadMemStats(&m)
		peakMem := m.Alloc

		// クリーンアップ
		data = nil
		runtime.GC()
		runtime.ReadMemStats(&m)
		finalMem := m.Alloc

		t.Logf("初期メモリ: %d KB", initialMem/1024)
		t.Logf("ピークメモリ: %d KB", peakMem/1024)
		t.Logf("最終メモリ: %d KB", finalMem/1024)

		if finalMem > initialMem*2 {
			t.Errorf("メモリリークの可能性: %d KB増加", (finalMem-initialMem)/1024)
		} else {
			t.Log("✅ メモリ管理成功")
		}
	})
}

// testIntegrationScenario - 統合シナリオのテスト
func testIntegrationScenario(t *testing.T) {
	t.Log("統合シナリオのE2Eテスト開始")

	// マイクロサービス風のシナリオ
	t.Run("マイクロサービス統合", func(t *testing.T) {
		type Service struct {
			name     string
			endpoint chan interface{}
			handler  func(interface{}) (interface{}, error)
		}

		// サービス定義
		services := map[string]*Service{
			"auth": {
				name:     "auth",
				endpoint: make(chan interface{}, 10),
				handler: func(req interface{}) (interface{}, error) {
					// 認証処理
					return map[string]string{"token": "valid-token"}, nil
				},
			},
			"inventory": {
				name:     "inventory",
				endpoint: make(chan interface{}, 10),
				handler: func(req interface{}) (interface{}, error) {
					// 在庫確認
					return map[string]int{"stock": 100}, nil
				},
			},
			"payment": {
				name:     "payment",
				endpoint: make(chan interface{}, 10),
				handler: func(req interface{}) (interface{}, error) {
					// 決済処理
					return map[string]string{"status": "success"}, nil
				},
			},
		}

		// サービス起動
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for _, svc := range services {
			go func(s *Service) {
				for {
					select {
					case req := <-s.endpoint:
						resp, _ := s.handler(req)
						t.Logf("%s service processed: %v", s.name, resp)
					case <-ctx.Done():
						return
					}
				}
			}(svc)
		}

		// ワークフロー実行
		workflow := func() error {
			// 1. 認証
			services["auth"].endpoint <- map[string]string{"user": "test"}

			// 2. 在庫確認
			services["inventory"].endpoint <- map[string]string{"item": "product-1"}

			// 3. 決済
			services["payment"].endpoint <- map[string]float64{"amount": 100.0}

			return nil
		}

		if err := workflow(); err != nil {
			t.Errorf("ワークフロー失敗: %v", err)
		} else {
			t.Log("✅ マイクロサービス統合成功")
		}

		// グレースフルシャットダウン
		cancel()
		time.Sleep(100 * time.Millisecond)
	})

	// データパイプライン
	t.Run("データパイプライン", func(t *testing.T) {
		type DataPoint struct {
			ID        int
			Value     float64
			Timestamp time.Time
		}

		// パイプラインステージ
		ingest := make(chan DataPoint, 100)
		process := make(chan DataPoint, 100)
		store := make(chan DataPoint, 100)

		var processed int64

		// Ingestion
		go func() {
			for i := 0; i < 100; i++ {
				ingest <- DataPoint{
					ID:        i,
					Value:     float64(i) * 1.5,
					Timestamp: time.Now(),
				}
			}
			close(ingest)
		}()

		// Processing
		go func() {
			for dp := range ingest {
				dp.Value = dp.Value * 2 // 変換処理
				process <- dp
				atomic.AddInt64(&processed, 1)
			}
			close(process)
		}()

		// Storage
		go func() {
			for dp := range process {
				store <- dp
			}
			close(store)
		}()

		// 結果収集
		stored := 0
		for range store {
			stored++
		}

		t.Logf("処理済み: %d, 保存済み: %d", processed, stored)

		if stored != 100 {
			t.Errorf("データパイプライン失敗: 期待値=100, 実際=%d", stored)
		} else {
			t.Log("✅ データパイプライン成功")
		}
	})
}

// BenchmarkE2E_ConcurrencyPatterns - 並行パターンのベンチマーク
func BenchmarkE2E_ConcurrencyPatterns(b *testing.B) {
	b.Run("Channel", func(b *testing.B) {
		ch := make(chan int, 100)
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				go func() { ch <- 1 }()
				<-ch
			}
		})
	})

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

	b.Run("Atomic", func(b *testing.B) {
		var counter int64
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				atomic.AddInt64(&counter, 1)
			}
		})
	})

	b.Run("WorkerPool", func(b *testing.B) {
		jobs := make(chan int, 100)
		results := make(chan int, 100)

		for w := 0; w < 10; w++ {
			go func() {
				for j := range jobs {
					results <- j * 2
				}
			}()
		}

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				jobs <- 1
				<-results
			}
		})
	})
}