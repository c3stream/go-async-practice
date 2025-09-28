package solutions

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Solution08_FixedPerformance - パフォーマンス問題の解決版
func Solution08_FixedPerformance() {
	fmt.Println("\n🔧 解答8: パフォーマンス問題の最適化")
	fmt.Println("=" + repeatString("=", 50))

	// 解法1: ロック競合の最小化
	solution1MinimizeLockContention()

	// 解法2: チャネルバッファリングの最適化
	solution2OptimizeChannelBuffering()

	// 解法3: ゴルーチンプールの最適化
	solution3OptimizeGoroutinePool()

	// 解法4: メモリアロケーションの最適化
	solution4OptimizeMemoryAllocation()

	// パフォーマンス比較
	performanceComparison()
}

// 解法1: ロック競合の最小化
func solution1MinimizeLockContention() {
	fmt.Println("\n📌 解法1: ロック競合の最小化")

	// 問題のあるコード（グローバルロック）
	type SlowCounter struct {
		mu    sync.Mutex
		value int64
	}

	slowCounter := &SlowCounter{}
	slowIncrement := func() {
		slowCounter.mu.Lock()
		time.Sleep(time.Microsecond) // 処理をシミュレート
		slowCounter.value++
		slowCounter.mu.Unlock()
	}

	// 改善1: 細粒度ロック（シャーディング）
	type ShardedCounter struct {
		shards [16]struct {
			mu    sync.Mutex
			value int64
		}
	}

	shardedCounter := &ShardedCounter{}
	shardedIncrement := func(id int) {
		shard := &shardedCounter.shards[id%16]
		shard.mu.Lock()
		shard.value++
		shard.mu.Unlock()
	}

	shardedTotal := func() int64 {
		var total int64
		for i := range shardedCounter.shards {
			shardedCounter.shards[i].mu.Lock()
			total += shardedCounter.shards[i].value
			shardedCounter.shards[i].mu.Unlock()
		}
		return total
	}

	// 改善2: アトミック操作
	var atomicCounter int64
	atomicIncrement := func() {
		atomic.AddInt64(&atomicCounter, 1)
	}

	// 改善3: ロックフリーデータ構造
	type LockFreeQueue struct {
		head atomic.Pointer[node]
		tail atomic.Pointer[node]
	}

	type node struct {
		value int
		next  atomic.Pointer[node]
	}

	queue := &LockFreeQueue{}
	sentinel := &node{}
	queue.head.Store(sentinel)
	queue.tail.Store(sentinel)

	enqueue := func(value int) {
		newNode := &node{value: value}
		for {
			last := queue.tail.Load()
			next := last.next.Load()
			if last == queue.tail.Load() {
				if next == nil {
					if last.next.CompareAndSwap(next, newNode) {
						queue.tail.CompareAndSwap(last, newNode)
						return
					}
				} else {
					queue.tail.CompareAndSwap(last, next)
				}
			}
		}
	}

	// ベンチマーク
	const numOperations = 10000
	numWorkers := runtime.NumCPU()

	// Slow version
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations/numWorkers; j++ {
				slowIncrement()
			}
		}()
	}
	wg.Wait()
	slowTime := time.Since(start)

	// Sharded version
	start = time.Now()
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations/numWorkers; j++ {
				shardedIncrement(id)
			}
		}(i)
	}
	wg.Wait()
	shardedTime := time.Since(start)

	// Atomic version
	start = time.Now()
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numOperations/numWorkers; j++ {
				atomicIncrement()
			}
		}()
	}
	wg.Wait()
	atomicTime := time.Since(start)

	// 結果表示
	fmt.Printf("\n  📊 パフォーマンス比較 (%d操作, %dワーカー):\n", numOperations, numWorkers)
	fmt.Printf("    グローバルロック: %v\n", slowTime)
	fmt.Printf("    シャーディング: %v (%.2fx高速)\n", shardedTime, float64(slowTime)/float64(shardedTime))
	fmt.Printf("    アトミック操作: %v (%.2fx高速)\n", atomicTime, float64(slowTime)/float64(atomicTime))
	fmt.Printf("\n  検証:\n")
	fmt.Printf("    Slow Counter: %d\n", slowCounter.value)
	fmt.Printf("    Sharded Counter: %d\n", shardedTotal())
	fmt.Printf("    Atomic Counter: %d\n", atomicCounter)
}

// 解法2: チャネルバッファリングの最適化
func solution2OptimizeChannelBuffering() {
	fmt.Println("\n📌 解法2: チャネルバッファリングの最適化")

	const numItems = 10000

	// 非効率: バッファなしチャネル
	benchmarkUnbuffered := func() time.Duration {
		ch := make(chan int)
		done := make(chan bool)

		go func() {
			for range ch {
				// 処理
			}
			done <- true
		}()

		start := time.Now()
		for i := 0; i < numItems; i++ {
			ch <- i
		}
		close(ch)
		<-done
		return time.Since(start)
	}

	// 改善: 適切なバッファサイズ
	benchmarkBuffered := func(bufferSize int) time.Duration {
		ch := make(chan int, bufferSize)
		done := make(chan bool)

		go func() {
			for range ch {
				// 処理
			}
			done <- true
		}()

		start := time.Now()
		for i := 0; i < numItems; i++ {
			ch <- i
		}
		close(ch)
		<-done
		return time.Since(start)
	}

	// バッチ処理の実装
	type Batch struct {
		items []int
	}

	benchmarkBatched := func(batchSize int) time.Duration {
		ch := make(chan Batch, 10)
		done := make(chan bool)

		go func() {
			for batch := range ch {
				// バッチ処理
				_ = batch
			}
			done <- true
		}()

		start := time.Now()
		batch := Batch{items: make([]int, 0, batchSize)}
		for i := 0; i < numItems; i++ {
			batch.items = append(batch.items, i)
			if len(batch.items) == batchSize {
				ch <- batch
				batch = Batch{items: make([]int, 0, batchSize)}
			}
		}
		if len(batch.items) > 0 {
			ch <- batch
		}
		close(ch)
		<-done
		return time.Since(start)
	}

	// ベンチマーク実行
	fmt.Println("\n  📊 チャネルバッファリング比較:")
	unbufferedTime := benchmarkUnbuffered()
	fmt.Printf("    バッファなし: %v\n", unbufferedTime)

	bufferSizes := []int{1, 10, 100, 1000}
	for _, size := range bufferSizes {
		bufferedTime := benchmarkBuffered(size)
		speedup := float64(unbufferedTime) / float64(bufferedTime)
		fmt.Printf("    バッファ=%d: %v (%.2fx高速)\n", size, bufferedTime, speedup)
	}

	batchSizes := []int{10, 50, 100}
	for _, size := range batchSizes {
		batchedTime := benchmarkBatched(size)
		speedup := float64(unbufferedTime) / float64(batchedTime)
		fmt.Printf("    バッチ=%d: %v (%.2fx高速)\n", size, batchedTime, speedup)
	}

	// select with default パターン
	fmt.Println("\n  非ブロッキング送信パターン:")
	ch := make(chan int, 100)
	sent, dropped := 0, 0

	for i := 0; i < 200; i++ {
		select {
		case ch <- i:
			sent++
		default:
			dropped++
		}
	}

	fmt.Printf("    送信成功: %d, ドロップ: %d\n", sent, dropped)
}

// 解法3: ゴルーチンプールの最適化
func solution3OptimizeGoroutinePool() {
	fmt.Println("\n📌 解法3: ゴルーチンプールの最適化")

	// タスク定義
	type Task func()

	// 非効率: ゴルーチンを都度作成
	runWithoutPool := func(tasks []Task) time.Duration {
		start := time.Now()
		var wg sync.WaitGroup
		for _, task := range tasks {
			wg.Add(1)
			go func(t Task) {
				defer wg.Done()
				t()
			}(task)
		}
		wg.Wait()
		return time.Since(start)
	}

	// 改善: ワーカープール
	type WorkerPool struct {
		workers int
		tasks   chan Task
		wg      sync.WaitGroup
	}

	newWorkerPool := func(workers int) *WorkerPool {
		pool := &WorkerPool{
			workers: workers,
			tasks:   make(chan Task, workers*2),
		}

		for i := 0; i < workers; i++ {
			pool.wg.Add(1)
			go func() {
				defer pool.wg.Done()
				for task := range pool.tasks {
					task()
				}
			}()
		}

		return pool
	}

	runWithPool := func(pool *WorkerPool, tasks []Task) time.Duration {
		start := time.Now()
		for _, task := range tasks {
			pool.tasks <- task
		}
		return time.Since(start)
	}

	// ダイナミックワーカープール
	type DynamicPool struct {
		minWorkers  int
		maxWorkers  int
		tasks       chan Task
		workerCount int32
		mu          sync.Mutex
	}

	newDynamicPool := func(min, max int) *DynamicPool {
		pool := &DynamicPool{
			minWorkers: min,
			maxWorkers: max,
			tasks:      make(chan Task, max*2),
		}

		// 最小ワーカー数を起動
		for i := 0; i < min; i++ {
			atomic.AddInt32(&pool.workerCount, 1)
			go pool.worker()
		}

		// 負荷監視
		go pool.monitor()

		return pool
	}

	(func(p *DynamicPool) {
		p.worker = func() {
			for task := range p.tasks {
				task()
			}
			atomic.AddInt32(&p.workerCount, -1)
		}

		p.monitor = func() {
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()

			for range ticker.C {
				queueSize := len(p.tasks)
				currentWorkers := atomic.LoadInt32(&p.workerCount)

				// スケールアップ
				if queueSize > int(currentWorkers)*2 && currentWorkers < int32(p.maxWorkers) {
					atomic.AddInt32(&p.workerCount, 1)
					go p.worker()
				}
				// スケールダウン（簡略化）
				// 実際の実装では、アイドルワーカーの管理が必要
			}
		}
	})(nil) // メソッド定義のためのダミー呼び出し

	// テストタスク作成
	createTasks := func(count int) []Task {
		tasks := make([]Task, count)
		for i := range tasks {
			tasks[i] = func() {
				time.Sleep(time.Microsecond * 10)
			}
		}
		return tasks
	}

	// ベンチマーク
	numTasks := 1000
	tasks := createTasks(numTasks)

	fmt.Println("\n  📊 ワーカープール比較:")

	// Without pool
	noPoolTime := runWithoutPool(tasks)
	fmt.Printf("    プールなし: %v\n", noPoolTime)

	// With different pool sizes
	poolSizes := []int{4, 8, 16, 32}
	for _, size := range poolSizes {
		pool := newWorkerPool(size)
		poolTime := runWithPool(pool, tasks)
		close(pool.tasks)
		pool.wg.Wait()

		speedup := float64(noPoolTime) / float64(poolTime)
		fmt.Printf("    プール(workers=%d): %v (%.2fx高速)\n", size, poolTime, speedup)
	}

	// 最適なワーカー数の推定
	optimalWorkers := runtime.NumCPU() * 2
	fmt.Printf("\n  💡 推奨ワーカー数: %d (CPU数×2)\n", optimalWorkers)
}

// DynamicPool メソッド
type DynamicPool struct {
	minWorkers  int
	maxWorkers  int
	tasks       chan func()
	workerCount int32
	mu          sync.Mutex
	worker      func()
	monitor     func()
}

// 解法4: メモリアロケーションの最適化
func solution4OptimizeMemoryAllocation() {
	fmt.Println("\n📌 解法4: メモリアロケーションの最適化")

	const numOperations = 10000

	// 非効率: 頻繁なアロケーション
	inefficientAllocation := func() time.Duration {
		start := time.Now()
		for i := 0; i < numOperations; i++ {
			data := make([]byte, 1024)
			// データ処理
			_ = data
		}
		return time.Since(start)
	}

	// 改善1: sync.Pool を使った再利用
	bufferPool := sync.Pool{
		New: func() interface{} {
			return make([]byte, 1024)
		},
	}

	pooledAllocation := func() time.Duration {
		start := time.Now()
		for i := 0; i < numOperations; i++ {
			data := bufferPool.Get().([]byte)
			// データ処理
			// 使用後プールに戻す
			bufferPool.Put(data)
		}
		return time.Since(start)
	}

	// 改善2: 事前割り当て
	preallocatedSlice := func() time.Duration {
		// 容量を事前に確保
		results := make([]int, 0, numOperations)

		start := time.Now()
		for i := 0; i < numOperations; i++ {
			results = append(results, i*2)
		}
		return time.Since(start)
	}

	dynamicSlice := func() time.Duration {
		// 容量指定なし（動的拡張）
		var results []int

		start := time.Now()
		for i := 0; i < numOperations; i++ {
			results = append(results, i*2)
		}
		return time.Since(start)
	}

	// 改善3: スライスの再利用
	type DataProcessor struct {
		buffer []byte
		mu     sync.Mutex
	}

	processor := &DataProcessor{
		buffer: make([]byte, 0, 1024),
	}

	reuseSlice := func() time.Duration {
		start := time.Now()
		processor.mu.Lock()
		defer processor.mu.Unlock()

		for i := 0; i < numOperations; i++ {
			// スライスをリセットして再利用
			processor.buffer = processor.buffer[:0]
			processor.buffer = append(processor.buffer, byte(i))
		}
		return time.Since(start)
	}

	// ベンチマーク実行
	fmt.Println("\n  📊 メモリアロケーション比較:")

	inefficientTime := inefficientAllocation()
	fmt.Printf("    頻繁なアロケーション: %v\n", inefficientTime)

	pooledTime := pooledAllocation()
	fmt.Printf("    sync.Pool使用: %v (%.2fx高速)\n",
		pooledTime, float64(inefficientTime)/float64(pooledTime))

	dynamicTime := dynamicSlice()
	fmt.Printf("    動的スライス: %v\n", dynamicTime)

	preallocTime := preallocatedSlice()
	fmt.Printf("    事前割り当てスライス: %v (%.2fx高速)\n",
		preallocTime, float64(dynamicTime)/float64(preallocTime))

	reuseTime := reuseSlice()
	fmt.Printf("    スライス再利用: %v (%.2fx高速)\n",
		reuseTime, float64(inefficientTime)/float64(reuseTime))

	// GC統計
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("\n  📊 メモリ統計:\n")
	fmt.Printf("    Alloc: %d KB\n", m.Alloc/1024)
	fmt.Printf("    TotalAlloc: %d KB\n", m.TotalAlloc/1024)
	fmt.Printf("    NumGC: %d\n", m.NumGC)
}

// パフォーマンス比較
func performanceComparison() {
	fmt.Println("\n🎯 総合パフォーマンス比較")
	fmt.Println("=" + repeatString("=", 50))

	type BenchResult struct {
		name     string
		duration time.Duration
		ops      int
	}

	results := []BenchResult{}

	// CPUバウンドタスク
	cpuBoundTask := func() {
		sum := 0
		for i := 0; i < 1000; i++ {
			sum += i * i
		}
		_ = sum
	}

	// IOバウンドタスク
	ioBoundTask := func() {
		time.Sleep(time.Microsecond * 100)
	}

	// 並列処理のベンチマーク
	benchmarkParallel := func(name string, task func(), numTasks int) {
		numWorkers := runtime.NumCPU()
		start := time.Now()

		var wg sync.WaitGroup
		taskChan := make(chan int, numTasks)

		// ワーカー起動
		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range taskChan {
					task()
				}
			}()
		}

		// タスク投入
		for i := 0; i < numTasks; i++ {
			taskChan <- i
		}
		close(taskChan)
		wg.Wait()

		duration := time.Since(start)
		results = append(results, BenchResult{
			name:     name,
			duration: duration,
			ops:      numTasks,
		})
	}

	// ベンチマーク実行
	fmt.Println("\n  実行中...")
	benchmarkParallel("CPU-bound (parallel)", cpuBoundTask, 10000)
	benchmarkParallel("IO-bound (parallel)", ioBoundTask, 100)

	// 結果表示
	fmt.Println("\n  📊 結果:")
	for _, r := range results {
		opsPerSec := float64(r.ops) / r.duration.Seconds()
		fmt.Printf("    %s:\n", r.name)
		fmt.Printf("      時間: %v\n", r.duration)
		fmt.Printf("      スループット: %.0f ops/sec\n", opsPerSec)
	}

	// 最適化のヒント
	fmt.Println("\n  💡 最適化のヒント:")
	hints := []string{
		"CPUバウンド: ワーカー数 = CPU数",
		"IOバウンド: ワーカー数 > CPU数",
		"メモリ効率: sync.Pool の活用",
		"ロック競合: シャーディングまたはatomic",
		"チャネル: 適切なバッファサイズ",
	}

	for _, hint := range hints {
		fmt.Printf("    • %s\n", hint)
	}
}

// Solution08_OptimizationTechniques - 最適化テクニック集
func Solution08_OptimizationTechniques() {
	fmt.Println("\n📚 並行処理の最適化テクニック")
	fmt.Println("=" + repeatString("=", 50))

	techniques := []struct {
		name        string
		description string
		example     func()
	}{
		{
			"バッチ処理",
			"複数のアイテムをまとめて処理",
			exampleBatchProcessing,
		},
		{
			"パイプライン",
			"ステージごとに並列化",
			examplePipeline,
		},
		{
			"ファンアウト・ファンイン",
			"作業を分散して結果を集約",
			exampleFanOutFanIn,
		},
		{
			"セマフォ",
			"同時実行数を制限",
			exampleSemaphore,
		},
	}

	for i, t := range techniques {
		fmt.Printf("\n  %d. %s\n", i+1, t.name)
		fmt.Printf("     %s\n", t.description)
		t.example()
	}
}

func exampleBatchProcessing() {
	items := make(chan int, 100)
	batches := make(chan []int, 10)

	// バッチ作成
	go func() {
		batch := make([]int, 0, 10)
		for item := range items {
			batch = append(batch, item)
			if len(batch) == 10 {
				batches <- batch
				batch = make([]int, 0, 10)
			}
		}
		if len(batch) > 0 {
			batches <- batch
		}
		close(batches)
	}()

	// バッチ処理
	go func() {
		for batch := range batches {
			fmt.Printf("     バッチ処理: %d items\n", len(batch))
		}
	}()

	// データ投入
	for i := 0; i < 25; i++ {
		items <- i
	}
	close(items)
	time.Sleep(100 * time.Millisecond)
}

func examplePipeline() {
	// ステージ1: 生成
	generate := func() <-chan int {
		out := make(chan int)
		go func() {
			for i := 0; i < 5; i++ {
				out <- i
			}
			close(out)
		}()
		return out
	}

	// ステージ2: 変換
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

	// パイプライン実行
	for n := range square(generate()) {
		fmt.Printf("     結果: %d\n", n)
	}
}

func exampleFanOutFanIn() {
	// ファンアウト
	work := func(id int, jobs <-chan int, results chan<- int) {
		for j := range jobs {
			results <- j * 2
		}
	}

	jobs := make(chan int, 10)
	results := make(chan int, 10)

	// ワーカー起動
	for w := 0; w < 3; w++ {
		go work(w, jobs, results)
	}

	// ジョブ送信
	go func() {
		for j := 0; j < 5; j++ {
			jobs <- j
		}
		close(jobs)
	}()

	// 結果集約
	for i := 0; i < 5; i++ {
		fmt.Printf("     結果: %d\n", <-results)
	}
}

func exampleSemaphore() {
	sem := make(chan struct{}, 2) // 同時実行数2

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sem <- struct{}{}        // セマフォ取得
			defer func() { <-sem }() // セマフォ解放

			fmt.Printf("     タスク %d 実行中\n", id)
			time.Sleep(50 * time.Millisecond)
		}(i)
	}
	wg.Wait()
}