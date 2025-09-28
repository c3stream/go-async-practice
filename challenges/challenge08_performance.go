package challenges

import (
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge08_PerformanceIssue - パフォーマンス問題
//
// 🎆 問題: このコードには深刻なパフォーマンス問題があります。ボトルネックを特定し最適化してください。
// 💡 ヒント: ロック競合、不適切なチャネル使用、CPU使用率を確認してください。
func Challenge08_PerformanceIssue() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 🎆 チャレンジ8: パフォーマンスを改善    ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n⚠️ 現在の状態: 処理が非常に遅いです！")

	start := time.Now()

	// 問題1: グローバルロックによるボトルネック
	var globalCounter int64
	var mutex sync.Mutex

	// 問題2: バッファなしチャネル
	results := make(chan int) // バッファなし！

	// 問題3: 過剰なゴルーチン生成
	var wg sync.WaitGroup
	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 問題4: ロックの持続時間が長い
			mutex.Lock()
			// 重い処理をロック中に実行
			time.Sleep(time.Microsecond)
			globalCounter++
			result := globalCounter
			mutex.Unlock()

			// 問題5: ブロッキングする送信
			select {
			case results <- int(result):
			case <-time.After(10 * time.Millisecond):
				// タイムアウトが多発
			}
		}(i)
	}

	// 問題6: 非効率な集計
	go func() {
		wg.Wait()
		close(results)
	}()

	var sum int64
	count := 0
	for r := range results {
		// 問題7: 不要な型変換とメモリアロケーション
		str := fmt.Sprintf("%d", r)
		val := len(str)
		sum += int64(val)
		count++
	}

	// 問題8: スリープによる待機（ポーリング）
	for {
		if atomic.LoadInt64(&globalCounter) >= 10000 {
			break
		}
		time.Sleep(1 * time.Millisecond) // CPU使用率が上がる
	}

	elapsed := time.Since(start)
	fmt.Printf("\n⏱️ 処理時間: %v\n", elapsed)
	fmt.Printf("📊 処理済みアイテム: %d\n", count)

	// 問題9: メモリリーク（未使用のマップ）
	cache := make(map[int][]byte)
	for i := 0; i < 1000; i++ {
		cache[i] = make([]byte, 1024*1024) // 1MB
	}
	_ = cache // 使用されない

	fmt.Println("\n🔧 最適化が必要な箇所:")
	fmt.Println("  1. ロックの粒度を小さくする（atomic操作）")
	fmt.Println("  2. バッファ付きチャネルを使用")
	fmt.Println("  3. ワーカープールパターンの採用")
	fmt.Println("  4. 不要な型変換の削除")
	fmt.Println("  5. 適切な待機メカニズム（ポーリング回避）")
}

// Challenge08_Benchmark - パフォーマンステスト
func Challenge08_Benchmark() {
	fmt.Println("\n📊 ベンチマーク比較:")
	fmt.Println("────────────────────────")

	// 悪い例：Mutex
	start := time.Now()
	var counter1 int64
	var mu sync.Mutex
	var wg1 sync.WaitGroup
	for i := 0; i < 10000; i++ {
		wg1.Add(1)
		go func() {
			defer wg1.Done()
			mu.Lock()
			counter1++
			mu.Unlock()
		}()
	}
	wg1.Wait()
	fmt.Printf("Mutex版: %v\n", time.Since(start))

	// 良い例：Atomic
	start = time.Now()
	var counter2 int64
	var wg2 sync.WaitGroup
	for i := 0; i < 10000; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			atomic.AddInt64(&counter2, 1)
		}()
	}
	wg2.Wait()
	fmt.Printf("Atomic版: %v\n", time.Since(start))

	// 最適化のヒント
	fmt.Println("\n💡 パフォーマンス改善のヒント:")
	fmt.Println("1. sync/atomic パッケージを使用")
	fmt.Println("2. sync.Pool でオブジェクトを再利用")
	fmt.Println("3. ワーカープールで並行数を制限")
	fmt.Println("4. プロファイリング（pprof）で分析")
}