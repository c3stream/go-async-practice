package solutions

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge11_BackpressureSolution - バックプレッシャー処理の解決
func Challenge11_BackpressureSolution() {
	fmt.Println("\n✅ チャレンジ11: バックプレッシャー処理の解決")
	fmt.Println("=" + repeatString("=", 50))

	// 解決策1: プル型アーキテクチャ
	solution1_PullBasedArchitecture()

	// 解決策2: トークンバケット＆サーキットブレーカー
	solution2_TokenBucketWithCircuitBreaker()

	// 解決策3: アダプティブバッファリング
	solution3_AdaptiveBuffering()
}

// 解決策1: プル型アーキテクチャ
func solution1_PullBasedArchitecture() {
	fmt.Println("\n📝 解決策1: プル型アーキテクチャ")

	type Task struct {
		ID   int
		Data []byte
	}

	type Result struct {
		TaskID int
		Output []byte
		Error  error
	}

	type PullBasedPipeline struct {
		taskGenerator func() *Task
		workers       int
		maxInflight   int32
		inflight      atomic.Int32
		results       chan Result
		wg            sync.WaitGroup
	}

	// タスク生成器（プル型）
	var taskID atomic.Int32
	pipeline := &PullBasedPipeline{
		taskGenerator: func() *Task {
			id := taskID.Add(1)
			return &Task{
				ID:   int(id),
				Data: make([]byte, 1024), // 1KB
			}
		},
		workers:     5,
		maxInflight: 10, // 同時処理数の制限
		results:     make(chan Result, 5),
	}

	// ワーカー（プル型）
	worker := func(ctx context.Context, workerID int) {
		defer pipeline.wg.Done()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// インフライト数をチェック
				current := pipeline.inflight.Load()
				if current >= pipeline.maxInflight {
					// バックプレッシャー: 待機
					time.Sleep(10 * time.Millisecond)
					continue
				}

				// タスクをプル
				pipeline.inflight.Add(1)
				task := pipeline.taskGenerator()

				// 処理
				time.Sleep(50 * time.Millisecond)

				result := Result{
					TaskID: task.ID,
					Output: make([]byte, len(task.Data)),
				}

				// ノンブロッキング送信
				select {
				case pipeline.results <- result:
					fmt.Printf("  ✓ Worker %d: Task %d 完了\n", workerID, task.ID)
				case <-time.After(100 * time.Millisecond):
					fmt.Printf("  ⚠ Worker %d: 結果送信タイムアウト\n", workerID)
				}

				pipeline.inflight.Add(-1)
			}
		}
	}

	// 結果処理（プル型）
	processResults := func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case result := <-pipeline.results:
				// 処理速度に応じて自動調整
				time.Sleep(30 * time.Millisecond)
				fmt.Printf("  📊 Result processed: Task %d\n", result.TaskID)
			}
		}
	}

	// 実行
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// ワーカー起動
	for i := 0; i < pipeline.workers; i++ {
		pipeline.wg.Add(1)
		go worker(ctx, i)
	}

	// 結果処理起動
	go processResults(ctx)

	// メトリクス監視
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				inflight := pipeline.inflight.Load()
				fmt.Printf("  📈 インフライト: %d/%d\n", inflight, pipeline.maxInflight)
			}
		}
	}()

	<-ctx.Done()
	pipeline.wg.Wait()
	close(pipeline.results)
}

// 解決策2: トークンバケット＆サーキットブレーカー
func solution2_TokenBucketWithCircuitBreaker() {
	fmt.Println("\n📝 解決策2: トークンバケット＆サーキットブレーカー")

	// トークンバケット
	type TokenBucket struct {
		tokens    int32
		maxTokens int32
		refillRate time.Duration
		mu        sync.RWMutex
	}

	bucket := &TokenBucket{
		tokens:     10,
		maxTokens:  10,
		refillRate: 100 * time.Millisecond,
	}

	// トークン補充
	go func() {
		ticker := time.NewTicker(bucket.refillRate)
		defer ticker.Stop()
		for range ticker.C {
			bucket.mu.Lock()
			if bucket.tokens < bucket.maxTokens {
				bucket.tokens++
				fmt.Printf("  🪙 トークン補充: %d/%d\n", bucket.tokens, bucket.maxTokens)
			}
			bucket.mu.Unlock()
		}
	}()

	// トークン取得
	acquireToken := func() bool {
		bucket.mu.Lock()
		defer bucket.mu.Unlock()
		if bucket.tokens > 0 {
			bucket.tokens--
			return true
		}
		return false
	}

	// サーキットブレーカー
	type CircuitBreaker struct {
		failures      atomic.Int32
		successCount  atomic.Int32
		state         atomic.Value // "closed", "open", "half-open"
		threshold     int32
		resetTimeout  time.Duration
		lastFailTime  atomic.Value
	}

	breaker := &CircuitBreaker{
		threshold:    3,
		resetTimeout: 1 * time.Second,
	}
	breaker.state.Store("closed")
	breaker.lastFailTime.Store(time.Now())

	// サーキットブレーカーのチェック
	checkCircuit := func() bool {
		state := breaker.state.Load().(string)

		switch state {
		case "open":
			// タイムアウト経過をチェック
			lastFail := breaker.lastFailTime.Load().(time.Time)
			if time.Since(lastFail) > breaker.resetTimeout {
				breaker.state.Store("half-open")
				fmt.Println("  🔄 Circuit: OPEN → HALF-OPEN")
				return true
			}
			return false
		case "half-open":
			return true
		default: // closed
			return true
		}
	}

	// リクエスト処理
	processRequest := func(id int) error {
		// サーキットブレーカーチェック
		if !checkCircuit() {
			fmt.Printf("  ❌ Request %d: Circuit OPEN\n", id)
			return fmt.Errorf("circuit open")
		}

		// トークン取得
		if !acquireToken() {
			fmt.Printf("  ⏳ Request %d: Rate limited\n", id)
			return fmt.Errorf("rate limited")
		}

		// 処理シミュレート（失敗率20%）
		time.Sleep(50 * time.Millisecond)

		if id%5 == 0 { // 20%失敗
			breaker.failures.Add(1)
			breaker.lastFailTime.Store(time.Now())

			if breaker.failures.Load() >= breaker.threshold {
				breaker.state.Store("open")
				fmt.Printf("  🚨 Circuit breaker OPENED (failures: %d)\n", breaker.failures.Load())
			}
			return fmt.Errorf("processing failed")
		}

		// 成功処理
		state := breaker.state.Load().(string)
		if state == "half-open" {
			breaker.successCount.Add(1)
			if breaker.successCount.Load() >= 2 {
				breaker.state.Store("closed")
				breaker.failures.Store(0)
				breaker.successCount.Store(0)
				fmt.Println("  ✅ Circuit: HALF-OPEN → CLOSED")
			}
		}

		fmt.Printf("  ✓ Request %d: Success\n", id)
		return nil
	}

	// リクエスト送信
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(reqID int) {
			defer wg.Done()
			time.Sleep(time.Duration(reqID*100) * time.Millisecond)

			select {
			case <-ctx.Done():
				return
			default:
				processRequest(reqID)
			}
		}(i)
	}

	wg.Wait()
}

// 解決策3: アダプティブバッファリング
func solution3_AdaptiveBuffering() {
	fmt.Println("\n📝 解決策3: アダプティブバッファリング")

	type AdaptiveBuffer struct {
		data          []interface{}
		maxSize       int
		currentSize   atomic.Int32
		dropCount     atomic.Int64
		throughput    atomic.Int64
		lastAdjust    time.Time
		mu            sync.RWMutex
		dropPolicy    string // "tail", "head", "random"
	}

	buffer := &AdaptiveBuffer{
		data:       make([]interface{}, 0, 100),
		maxSize:    100,
		dropPolicy: "tail",
		lastAdjust: time.Now(),
	}

	// アダプティブサイズ調整
	adjustBufferSize := func() {
		buffer.mu.Lock()
		defer buffer.mu.Unlock()

		// スループットベースの調整
		tp := buffer.throughput.Load()
		drops := buffer.dropCount.Load()

		if drops > 5 && buffer.maxSize > 20 {
			// ドロップが多い場合はバッファを縮小
			buffer.maxSize = buffer.maxSize * 8 / 10
			fmt.Printf("  📉 バッファサイズ縮小: %d\n", buffer.maxSize)
		} else if drops == 0 && tp > 20 && buffer.maxSize < 200 {
			// ドロップがなく高スループットの場合は拡大
			buffer.maxSize = buffer.maxSize * 12 / 10
			fmt.Printf("  📈 バッファサイズ拡大: %d\n", buffer.maxSize)
		}

		buffer.dropCount.Store(0)
		buffer.throughput.Store(0)
		buffer.lastAdjust = time.Now()
	}

	// エンキュー（ドロップポリシー付き）
	enqueue := func(item interface{}) bool {
		buffer.mu.Lock()
		defer buffer.mu.Unlock()

		currentLen := len(buffer.data)
		if currentLen >= buffer.maxSize {
			// ドロップポリシー適用
			switch buffer.dropPolicy {
			case "tail":
				// 新しいアイテムをドロップ
				buffer.dropCount.Add(1)
				fmt.Printf("  🗑️ Tail drop: バッファフル (%d/%d)\n", currentLen, buffer.maxSize)
				return false
			case "head":
				// 古いアイテムをドロップ
				buffer.data = buffer.data[1:]
				buffer.dropCount.Add(1)
				fmt.Printf("  🗑️ Head drop: 古いアイテム削除\n")
			}
		}

		buffer.data = append(buffer.data, item)
		buffer.currentSize.Store(int32(len(buffer.data)))
		return true
	}

	// デキュー
	dequeue := func() (interface{}, bool) {
		buffer.mu.Lock()
		defer buffer.mu.Unlock()

		if len(buffer.data) == 0 {
			return nil, false
		}

		item := buffer.data[0]
		buffer.data = buffer.data[1:]
		buffer.currentSize.Store(int32(len(buffer.data)))
		buffer.throughput.Add(1)
		return item, true
	}

	// プロデューサー
	produce := func(ctx context.Context) {
		id := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// 負荷に応じた生成速度
				size := buffer.currentSize.Load()
				delay := time.Duration(10+size) * time.Millisecond
				time.Sleep(delay)

				enqueue(id)
				id++
			}
		}
	}

	// コンシューマー
	consume := func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if item, ok := dequeue(); ok {
					// 処理
					time.Sleep(30 * time.Millisecond)
					fmt.Printf("  ✓ Processed: %v\n", item)
				} else {
					time.Sleep(10 * time.Millisecond)
				}
			}
		}
	}

	// 実行
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go produce(ctx)
	go consume(ctx)

	// 定期的な調整
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				adjustBufferSize()
				size := buffer.currentSize.Load()
				drops := buffer.dropCount.Load()
				fmt.Printf("  📊 Stats: Size=%d/%d, Drops=%d\n",
					size, buffer.maxSize, drops)
			}
		}
	}()

	<-ctx.Done()
}