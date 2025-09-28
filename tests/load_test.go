package tests

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestHighConcurrentLoad - 高並行負荷テスト
func TestHighConcurrentLoad(t *testing.T) {
	// テスト設定
	config := struct {
		Workers       int
		TasksPerWorker int
		TaskDuration  time.Duration
		Timeout       time.Duration
	}{
		Workers:       1000,
		TasksPerWorker: 100,
		TaskDuration:  time.Millisecond,
		Timeout:       30 * time.Second,
	}

	// メトリクス
	var (
		totalTasks     int64
		completedTasks int64
		failedTasks    int64
		totalLatency   int64
	)

	// ワークロード実行
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()

	start := time.Now()
	var wg sync.WaitGroup

	for w := 0; w < config.Workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for i := 0; i < config.TasksPerWorker; i++ {
				atomic.AddInt64(&totalTasks, 1)

				taskStart := time.Now()
				err := executeTask(ctx, config.TaskDuration)
				taskLatency := time.Since(taskStart)

				atomic.AddInt64(&totalLatency, int64(taskLatency))

				if err != nil {
					atomic.AddInt64(&failedTasks, 1)
				} else {
					atomic.AddInt64(&completedTasks, 1)
				}
			}
		}(w)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// 結果分析
	avgLatency := time.Duration(totalLatency) / time.Duration(totalTasks)
	throughput := float64(completedTasks) / elapsed.Seconds()
	successRate := float64(completedTasks) / float64(totalTasks) * 100

	t.Logf("\n負荷テスト結果:")
	t.Logf("  総タスク数: %d", totalTasks)
	t.Logf("  完了タスク: %d", completedTasks)
	t.Logf("  失敗タスク: %d", failedTasks)
	t.Logf("  実行時間: %v", elapsed)
	t.Logf("  平均レイテンシ: %v", avgLatency)
	t.Logf("  スループット: %.2f tasks/sec", throughput)
	t.Logf("  成功率: %.2f%%", successRate)

	// 品質基準チェック
	if successRate < 95 {
		t.Errorf("成功率が低すぎます: %.2f%% < 95%%", successRate)
	}
	if avgLatency > 10*time.Millisecond {
		t.Errorf("レイテンシが高すぎます: %v > 10ms", avgLatency)
	}
}

// executeTask - タスク実行のシミュレーション
func executeTask(ctx context.Context, duration time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(duration + time.Duration(rand.Intn(int(duration)))):
		// ランダムに10%の確率でエラー
		if rand.Float32() < 0.1 {
			return fmt.Errorf("simulated error")
		}
		return nil
	}
}

// TestSpikeLoad - スパイク負荷テスト
func TestSpikeLoad(t *testing.T) {
	// サービスシミュレーション
	type Service struct {
		mu            sync.RWMutex
		requestCount  int64
		capacity      int64
		circuitOpen   bool
		lastReset     time.Time
	}

	service := &Service{
		capacity:  100,
		lastReset: time.Now(),
	}

	// リクエストハンドラー
	handleRequest := func() (bool, time.Duration) {
		start := time.Now()

		// サーキットブレーカーチェック
		service.mu.RLock()
		if service.circuitOpen {
			service.mu.RUnlock()
			return false, time.Since(start)
		}
		service.mu.RUnlock()

		// キャパシティチェック
		current := atomic.AddInt64(&service.requestCount, 1)
		defer atomic.AddInt64(&service.requestCount, -1)

		if current > service.capacity {
			// 過負荷時はサーキットを開く
			service.mu.Lock()
			service.circuitOpen = true
			service.mu.Unlock()

			// 1秒後に自動リセット
			go func() {
				time.Sleep(time.Second)
				service.mu.Lock()
				service.circuitOpen = false
				service.lastReset = time.Now()
				service.mu.Unlock()
			}()

			return false, time.Since(start)
		}

		// 処理シミュレーション
		time.Sleep(10 * time.Millisecond)
		return true, time.Since(start)
	}

	// スパイク負荷パターン
	loadPatterns := []struct {
		name     string
		duration time.Duration
		rps      int // requests per second
	}{
		{"通常負荷", 2 * time.Second, 50},
		{"スパイク開始", 1 * time.Second, 200},
		{"スパイクピーク", 2 * time.Second, 500},
		{"スパイク減少", 1 * time.Second, 200},
		{"通常負荷復帰", 2 * time.Second, 50},
	}

	for _, pattern := range loadPatterns {
		t.Logf("\n%s (RPS: %d)", pattern.name, pattern.rps)

		ctx, cancel := context.WithTimeout(context.Background(), pattern.duration)
		defer cancel()

		var (
			success  int64
			failed   int64
			totalLat int64
		)

		// 負荷生成
		ticker := time.NewTicker(time.Second / time.Duration(pattern.rps))
		defer ticker.Stop()

		done := make(chan struct{})
		go func() {
			for {
				select {
				case <-ticker.C:
					go func() {
						ok, latency := handleRequest()
						atomic.AddInt64(&totalLat, int64(latency))
						if ok {
							atomic.AddInt64(&success, 1)
						} else {
							atomic.AddInt64(&failed, 1)
						}
					}()
				case <-ctx.Done():
					close(done)
					return
				}
			}
		}()

		<-done
		time.Sleep(100 * time.Millisecond) // 処理完了待ち

		total := success + failed
		if total > 0 {
			avgLat := time.Duration(totalLat) / time.Duration(total)
			successRate := float64(success) / float64(total) * 100

			t.Logf("  成功: %d, 失敗: %d, 成功率: %.1f%%, 平均レイテンシ: %v",
				success, failed, successRate, avgLat)

			if service.circuitOpen {
				t.Logf("  ⚠ サーキットブレーカー作動中")
			}
		}
	}
}

// TestMemoryUnderLoad - 負荷時のメモリ使用量テスト
func TestMemoryUnderLoad(t *testing.T) {
	// 初期メモリ状態
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	initialMem := m.Alloc

	t.Logf("初期メモリ: %d KB", initialMem/1024)

	// メモリ集約的なタスク
	type DataProcessor struct {
		buffers [][]byte
		mu      sync.Mutex
		pool    *sync.Pool
	}

	processor := &DataProcessor{
		buffers: make([][]byte, 0),
		pool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 1024*1024) // 1MB
			},
		},
	}

	// 並行処理でメモリ負荷
	var wg sync.WaitGroup
	numWorkers := 100
	tasksPerWorker := 100

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for i := 0; i < tasksPerWorker; i++ {
				// プールからバッファ取得
				buffer := processor.pool.Get().([]byte)

				// データ処理シミュレーション
				for j := range buffer {
					buffer[j] = byte(id + i + j)
				}

				// 10%の確率でバッファを保持（リーク風）
				if rand.Float32() < 0.1 {
					processor.mu.Lock()
					processor.buffers = append(processor.buffers, buffer)
					processor.mu.Unlock()
				} else {
					// プールに返却
					processor.pool.Put(buffer)
				}

				// メモリプレッシャーシミュレーション
				if i%10 == 0 {
					runtime.GC()
				}
			}
		}(w)
	}

	// 定期的なメモリ監視
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		maxMem := initialMem
		for {
			select {
			case <-ticker.C:
				runtime.ReadMemStats(&m)
				if m.Alloc > maxMem {
					maxMem = m.Alloc
				}
				t.Logf("  現在のメモリ: %d KB, GC回数: %d",
					m.Alloc/1024, m.NumGC)
			case <-done:
				return
			}
		}
	}()

	wg.Wait()
	close(done)

	// 最終メモリ状態
	runtime.GC()
	runtime.ReadMemStats(&m)
	finalMem := m.Alloc

	t.Logf("\n最終メモリ: %d KB", finalMem/1024)
	t.Logf("保持されたバッファ: %d", len(processor.buffers))
	t.Logf("メモリ増加: %d KB", (finalMem-initialMem)/1024)

	// メモリリークチェック
	if finalMem > initialMem*10 {
		t.Errorf("メモリ使用量が異常: %d KB (初期の10倍以上)",
			finalMem/1024)
	}
}

// TestCascadingFailure - カスケード障害テスト
func TestCascadingFailure(t *testing.T) {
	// マイクロサービスシミュレーション
	type Microservice struct {
		name         string
		healthy      bool
		dependencies []string
		mu           sync.RWMutex
	}

	services := map[string]*Microservice{
		"frontend": {
			name:         "frontend",
			healthy:      true,
			dependencies: []string{"api"},
		},
		"api": {
			name:         "api",
			healthy:      true,
			dependencies: []string{"database", "cache"},
		},
		"database": {
			name:         "database",
			healthy:      true,
			dependencies: []string{},
		},
		"cache": {
			name:         "cache",
			healthy:      true,
			dependencies: []string{},
		},
		"analytics": {
			name:         "analytics",
			healthy:      true,
			dependencies: []string{"database"},
		},
	}

	// サービスヘルスチェック（再帰関数のための前方宣言）
	var checkHealth func(name string, visited map[string]bool) bool
	checkHealth = func(name string, visited map[string]bool) bool {
		if visited[name] {
			return false // 循環依存検出
		}
		visited[name] = true

		svc := services[name]
		svc.mu.RLock()
		defer svc.mu.RUnlock()

		if !svc.healthy {
			return false
		}

		// 依存サービスのチェック
		for _, dep := range svc.dependencies {
			if !checkHealth(dep, visited) {
				return false
			}
		}

		return true
	}

	// 障害注入
	injectFailure := func(serviceName string) {
		if svc, ok := services[serviceName]; ok {
			svc.mu.Lock()
			svc.healthy = false
			svc.mu.Unlock()
			t.Logf("⚠ %s サービスに障害を注入", serviceName)
		}
	}

	// 復旧
	recover := func(serviceName string) {
		if svc, ok := services[serviceName]; ok {
			svc.mu.Lock()
			svc.healthy = true
			svc.mu.Unlock()
			t.Logf("✅ %s サービスが復旧", serviceName)
		}
	}

	// カスケード障害のテスト
	scenarios := []struct {
		name        string
		failService string
		expected    map[string]bool
	}{
		{
			name:        "データベース障害",
			failService: "database",
			expected: map[string]bool{
				"frontend":  false,
				"api":       false,
				"database":  false,
				"cache":     true,
				"analytics": false,
			},
		},
		{
			name:        "キャッシュ障害",
			failService: "cache",
			expected: map[string]bool{
				"frontend":  false,
				"api":       false,
				"database":  true,
				"cache":     false,
				"analytics": true,
			},
		},
	}

	for _, scenario := range scenarios {
		t.Logf("\nシナリオ: %s", scenario.name)

		// 全サービスを健全な状態に
		for _, svc := range services {
			svc.mu.Lock()
			svc.healthy = true
			svc.mu.Unlock()
		}

		// 障害注入
		injectFailure(scenario.failService)

		// 各サービスの状態確認
		for name, expectedHealthy := range scenario.expected {
			visited := make(map[string]bool)
			actual := checkHealth(name, visited)

			status := "❌"
			if actual {
				status = "✅"
			}

			t.Logf("  %s %s: 健全=%v (期待値=%v)",
				status, name, actual, expectedHealthy)

			if actual != expectedHealthy {
				t.Errorf("    予期しない状態: %s", name)
			}
		}

		// 復旧テスト
		time.Sleep(100 * time.Millisecond)
		recover(scenario.failService)

		// 復旧後の確認
		allHealthy := true
		for name := range services {
			visited := make(map[string]bool)
			if !checkHealth(name, visited) {
				allHealthy = false
				break
			}
		}

		if allHealthy {
			t.Log("  ✅ 全サービスが正常に復旧")
		} else {
			t.Error("  ❌ 一部のサービスが復旧していません")
		}
	}
}

// BenchmarkLoadPatterns - 負荷パターンのベンチマーク
func BenchmarkLoadPatterns(b *testing.B) {
	b.Run("ConstantLoad", func(b *testing.B) {
		// 一定負荷
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				time.Sleep(time.Microsecond)
			}
		})
	})

	b.Run("BurstLoad", func(b *testing.B) {
		// バースト負荷
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if rand.Float32() < 0.1 {
					// 10%の確率で重い処理
					time.Sleep(10 * time.Microsecond)
				} else {
					time.Sleep(time.Microsecond)
				}
			}
		})
	})

	b.Run("WaveLoad", func(b *testing.B) {
		// 波状負荷
		start := time.Now()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				// sin波で負荷を変動
				elapsed := time.Since(start).Seconds()
				load := (1 + math.Sin(elapsed)) / 2
				sleepTime := time.Duration(float64(time.Microsecond) * (1 + load*9))
				time.Sleep(sleepTime)
			}
		})
	})
}