package examples

import (
	"fmt"
	"sync"
	"time"
)

// Example1_SimpleGoroutine - ゴルーチンの基本
// 同期処理と並行処理の実行時間を比較します
func Example1_SimpleGoroutine() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 例題1: ゴルーチンの基本                ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n📝 説明: 3つのタスクを同期的・並行的に実行して")
	fmt.Println("        実行時間の違いを比較します。")

	// 同期的な処理（順番に実行）
	fmt.Println("\n1️⃣ 同期処理（順番に実行）:")
	start := time.Now()
	for i := 0; i < 3; i++ {
		time.Sleep(100 * time.Millisecond)
		fmt.Printf("  タスク %d 完了\n", i)
	}
	fmt.Printf("⏱️ 同期処理の実行時間: %v\n", time.Since(start))

	// 並行処理（goroutineで同時実行）
	fmt.Println("\n2️⃣ 並行処理（ゴルーチンで同時実行）:")
	start = time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			time.Sleep(100 * time.Millisecond)
			fmt.Printf("  タスク %d 完了\n", id)
		}(i)
	}
	wg.Wait()
	fmt.Printf("⏱️ 並行処理の実行時間: %v\n", time.Since(start))
	fmt.Println("\n💡 ポイント: 並行処理により約3倍高速化！")
}

// Example2_RaceCondition - レース条件の理解
// 複数のゴルーチンが同じ変数にアクセスする危険性を示します
func Example2_RaceCondition() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 例題2: レース条件の危険性              ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n📝 説明: 1000個のゴルーチンが同じカウンターを")
	fmt.Println("        インクリメントする実験")

	// ⚠️ 危険な例（レース条件あり）
	fmt.Println("\n1️⃣ Mutexなし（危険）:")
	counter := 0
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter++ // 非同期で同じ変数にアクセス
		}()
	}
	wg.Wait()
	fmt.Printf("  最終値: %d (期待値: 1000)\n", counter)
	if counter != 1000 {
		fmt.Println("  ❌ データ競合により値が不正！")
	}

	// ✅ 安全な例（Mutexを使用）
	fmt.Println("\n2️⃣ Mutex使用（安全）:")
	counter = 0
	var mu sync.Mutex
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			counter++
			mu.Unlock()
		}()
	}
	wg.Wait()
	fmt.Printf("  最終値: %d\n", counter)
	if counter == 1000 {
		fmt.Println("  ✅ Mutexにより正しい値を保証！")
	}
	fmt.Println("\n💡 ポイント: 共有リソースへのアクセスは必ず同期化！")
}

// Example3_ChannelBasics - チャネルの基本
// ゴルーチン間の安全な通信方法を学びます
func Example3_ChannelBasics() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 例題3: チャネルによる通信              ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n📝 説明: チャネルを使った安全なデータ受け渡し")

	// 📡 バッファなしチャネル（同期的）
	fmt.Println("\n1️⃣ バッファなしチャネル:")
	ch := make(chan int)

	go func() {
		for i := 0; i < 5; i++ {
			ch <- i
			fmt.Printf("  送信: %d\n", i)
		}
		close(ch)
	}()

	for value := range ch {
		fmt.Printf("  受信: %d\n", value)
		time.Sleep(100 * time.Millisecond) // Simulate processing
	}

	// 📦 バッファ付きチャネル（非同期的）
	fmt.Println("\n2️⃣ バッファ付きチャネル（容量3）:")
	bufferedCh := make(chan int, 3)

	go func() {
		for i := 0; i < 5; i++ {
			bufferedCh <- i
			fmt.Printf("  バッファへ送信: %d\n", i)
		}
		close(bufferedCh)
	}()

	time.Sleep(500 * time.Millisecond) // バッファを満たす時間を確保
	for value := range bufferedCh {
		fmt.Printf("  バッファから受信: %d\n", value)
	}
}