package solutions

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Solution05_FixedMemoryLeak - メモリリーク問題の解決版
func Solution05_FixedMemoryLeak() {
	fmt.Println("\n🔧 解答5: メモリリークの修正")
	fmt.Println("=" + repeatString("=", 50))

	// 解法1: Context を使った適切なゴルーチン終了
	solution1ContextControl()

	// 解法2: クローズシグナルを使った明示的な終了
	solution2ExplicitClose()

	// 解法3: sync.Pool を使ったメモリ再利用
	solution3MemoryPool()

	// メモリ使用状況の確認
	verifyMemoryUsage()
}

// 解法1: Context による制御
func solution1ContextControl() {
	fmt.Println("\n📌 解法1: Context を使った適切なゴルーチン管理")

	type DataProcessor struct {
		ctx    context.Context
		cancel context.CancelFunc
		data   chan []byte
		wg     sync.WaitGroup
	}

	processor := &DataProcessor{
		data: make(chan []byte, 100),
	}

	// Context の作成
	processor.ctx, processor.cancel = context.WithCancel(context.Background())

	// ワーカーゴルーチンの起動
	startWorkers := func(count int) {
		for i := 0; i < count; i++ {
			processor.wg.Add(1)
			go func(id int) {
				defer processor.wg.Done()
				fmt.Printf("  ワーカー %d: 起動\n", id)

				for {
					select {
					case <-processor.ctx.Done():
						// Context がキャンセルされたら終了
						fmt.Printf("  ワーカー %d: 正常終了\n", id)
						return
					case data := <-processor.data:
						// データ処理
						if data != nil {
							// 実際の処理をシミュレート
							time.Sleep(time.Millisecond)
						}
					case <-time.After(100 * time.Millisecond):
						// タイムアウトでアイドル状態を防ぐ
						continue
					}
				}
			}(i)
		}
	}

	// ワーカーを起動
	startWorkers(5)

	// データを送信
	go func() {
		for i := 0; i < 10; i++ {
			processor.data <- make([]byte, 1024)
		}
	}()

	// 処理時間を待つ
	time.Sleep(200 * time.Millisecond)

	// 適切なシャットダウン
	fmt.Println("\n  シャットダウン開始...")
	processor.cancel()   // Context をキャンセル
	close(processor.data) // チャネルをクローズ
	processor.wg.Wait()   // すべてのゴルーチンの終了を待つ
	fmt.Println("  ✅ すべてのゴルーチンが正常に終了しました")
}

// 解法2: 明示的なクローズシグナル
func solution2ExplicitClose() {
	fmt.Println("\n📌 解法2: 明示的なクローズシグナルパターン")

	type Worker struct {
		id       int
		tasks    chan func()
		quit     chan struct{}
		finished chan struct{}
	}

	createWorker := func(id int) *Worker {
		w := &Worker{
			id:       id,
			tasks:    make(chan func(), 10),
			quit:     make(chan struct{}),
			finished: make(chan struct{}),
		}

		go func() {
			defer close(w.finished)
			fmt.Printf("  ワーカー %d: 起動\n", id)

			for {
				select {
				case <-w.quit:
					// 終了シグナルを受信
					fmt.Printf("  ワーカー %d: 終了シグナル受信\n", id)
					// 残りのタスクを処理
					for {
						select {
						case task := <-w.tasks:
							if task != nil {
								task()
							}
						default:
							fmt.Printf("  ワーカー %d: クリーンアップ完了\n", id)
							return
						}
					}
				case task := <-w.tasks:
					if task != nil {
						task()
					}
				}
			}
		}()

		return w
	}

	// ワーカーを作成
	workers := make([]*Worker, 3)
	for i := 0; i < 3; i++ {
		workers[i] = createWorker(i)
	}

	// タスクを送信
	for i := 0; i < 9; i++ {
		workerID := i % 3
		taskID := i
		workers[workerID].tasks <- func() {
			fmt.Printf("    タスク %d 実行中...\n", taskID)
			time.Sleep(50 * time.Millisecond)
		}
	}

	time.Sleep(200 * time.Millisecond)

	// 適切なシャットダウン
	fmt.Println("\n  グレースフルシャットダウン開始...")
	for _, w := range workers {
		close(w.quit)
	}
	for _, w := range workers {
		<-w.finished
	}
	fmt.Println("  ✅ すべてのワーカーが正常に終了しました")
}

// 解法3: sync.Pool によるメモリ再利用
func solution3MemoryPool() {
	fmt.Println("\n📌 解法3: sync.Pool を使ったメモリ効率化")

	// バッファプール
	bufferPool := sync.Pool{
		New: func() interface{} {
			// 新しいバッファを作成
			return make([]byte, 1024*10) // 10KB
		},
	}

	// 統計情報
	var (
		allocated   int64
		reused      int64
		mu          sync.Mutex
	)

	// データ処理関数
	processData := func(id int) {
		// プールからバッファを取得
		buffer := bufferPool.Get().([]byte)

		mu.Lock()
		if buffer[0] == 0 {
			allocated++
		} else {
			reused++
		}
		mu.Unlock()

		// データ処理のシミュレーション
		for i := range buffer {
			buffer[i] = byte(id % 256)
		}
		time.Sleep(10 * time.Millisecond)

		// 使い終わったらプールに戻す
		bufferPool.Put(buffer)
	}

	// 並行処理
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			processData(id)
		}(i)
	}

	wg.Wait()

	fmt.Printf("\n  📊 メモリ使用統計:\n")
	fmt.Printf("    新規割り当て: %d バッファ\n", allocated)
	fmt.Printf("    再利用: %d バッファ\n", reused)
	fmt.Printf("    再利用率: %.1f%%\n", float64(reused)/float64(allocated+reused)*100)
	fmt.Println("  ✅ メモリプールによる効率的なメモリ管理")
}

// メモリ使用状況の確認
func verifyMemoryUsage() {
	fmt.Println("\n📊 メモリ使用状況の確認")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("  Alloc: %d KB\n", m.Alloc/1024)
	fmt.Printf("  TotalAlloc: %d KB\n", m.TotalAlloc/1024)
	fmt.Printf("  Sys: %d KB\n", m.Sys/1024)
	fmt.Printf("  NumGC: %d\n", m.NumGC)
	fmt.Printf("  Goroutines: %d\n", runtime.NumGoroutine())

	// GCを強制実行
	runtime.GC()
	runtime.ReadMemStats(&m)

	fmt.Println("\n  GC後:")
	fmt.Printf("  Alloc: %d KB\n", m.Alloc/1024)
	fmt.Printf("  Goroutines: %d\n", runtime.NumGoroutine())
	fmt.Println("\n✅ メモリリークが解決されました！")
}

// Solution05_MultipleSolutions - 複数の解法を提示
func Solution05_MultipleSolutions() {
	fmt.Println("\n📚 メモリリーク問題の解決パターン")
	fmt.Println("=" + repeatString("=", 50))

	patterns := []struct {
		name        string
		description string
		example     func()
	}{
		{
			name:        "Context パターン",
			description: "Context を使用してゴルーチンのライフサイクルを管理",
			example:     exampleContextPattern,
		},
		{
			name:        "Worker Pool パターン",
			description: "固定数のワーカーでゴルーチン数を制限",
			example:     exampleWorkerPoolPattern,
		},
		{
			name:        "Ticker Cleanup パターン",
			description: "time.Ticker を適切にクリーンアップ",
			example:     exampleTickerCleanup,
		},
		{
			name:        "Channel Buffering パターン",
			description: "適切なバッファサイズでブロッキングを防ぐ",
			example:     exampleChannelBuffering,
		},
	}

	for i, p := range patterns {
		fmt.Printf("\n🎯 パターン %d: %s\n", i+1, p.name)
		fmt.Printf("  説明: %s\n", p.description)
		p.example()
	}
}

func exampleContextPattern() {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		fmt.Println("    ✓ Context によりゴルーチン終了")
	}()

	<-done
}

func exampleWorkerPoolPattern() {
	const numWorkers = 3
	jobs := make(chan int, 10)
	var wg sync.WaitGroup

	// ワーカープール作成
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for job := range jobs {
				_ = job // 処理
			}
			fmt.Printf("    ✓ ワーカー %d 終了\n", id)
		}(w)
	}

	// ジョブ送信
	for j := 0; j < 5; j++ {
		jobs <- j
	}
	close(jobs)
	wg.Wait()
}

func exampleTickerCleanup() {
	ticker := time.NewTicker(50 * time.Millisecond)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-ticker.C:
				// 定期処理
			case <-time.After(100 * time.Millisecond):
				ticker.Stop() // 重要: Ticker を停止
				fmt.Println("    ✓ Ticker を適切に停止")
				return
			}
		}
	}()

	<-done
}

func exampleChannelBuffering() {
	// バッファ付きチャネルで非ブロッキング送信
	ch := make(chan int, 5)

	go func() {
		for i := 0; i < 5; i++ {
			select {
			case ch <- i:
				// 送信成功
			default:
				// バッファフルの場合はスキップ
				fmt.Println("    ⚠ バッファフル、データ破棄")
			}
		}
		close(ch)
	}()

	// 受信
	for v := range ch {
		_ = v
	}
	fmt.Println("    ✓ 適切なバッファリングで deadlock 回避")
}

func repeatString(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}