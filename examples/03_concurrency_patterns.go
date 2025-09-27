package examples

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Example8_WorkerPool - ワーカープールパターン
// 固定数のワーカーでタスクを効率的に処理する方法を学びます
func Example8_WorkerPool() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 例題8: ワーカープールパターン         ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n📝 説明: 3人のワーカーが10個のジョブを分担処理")

	const numWorkers = 3
	const numJobs = 10

	jobs := make(chan int, numJobs)
	results := make(chan int, numJobs)

	// 👷 ワーカーを起動
	var wg sync.WaitGroup
	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for job := range jobs {
				fmt.Printf("  👷 ワーカー %d: ジョブ %d を処理中\n", id, job)
				time.Sleep(100 * time.Millisecond) // 作業をシミュレート
				results <- job * 2
			}
		}(w)
	}

	// 📨 ジョブを送信
	go func() {
		for j := 1; j <= numJobs; j++ {
			jobs <- j
		}
		close(jobs)
	}()

	// ⏳ ワーカーの終了を待機
	go func() {
		wg.Wait()
		close(results)
	}()

	// 📥 結果を収集
	for result := range results {
		fmt.Printf("  📊 結果: %d\n", result)
	}
}

// Example9_FanInFanOut - ファンイン・ファンアウトパターン
// データを分散処理し、結果を集約する方法を学びます
func Example9_FanInFanOut() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 例題9: ファンイン・ファンアウトパターン ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n📝 説明: データを複数のワーカーに分散し、結果を集約")

	// 🎉 ファンアウト: データを複数のゴルーチンに分散
	gen := func(nums ...int) chan int {
		out := make(chan int)
		go func() {
			for _, n := range nums {
				out <- n
			}
			close(out)
		}()
		return out
	}

	// 🔄 二乗計算関数（ワーカー）
	sq := func(in chan int) chan int {
		out := make(chan int)
		go func() {
			for n := range in {
				out <- n * n
				time.Sleep(50 * time.Millisecond) // 作業をシミュレート
			}
			close(out)
		}()
		return out
	}

	// 🌐 ファンイン: 複数のチャネルを一つに集約
	merge := func(cs ...chan int) chan int {
		var wg sync.WaitGroup
		out := make(chan int)

		output := func(c chan int) {
			defer wg.Done()
			for n := range c {
				out <- n
			}
		}

		wg.Add(len(cs))
		for _, c := range cs {
			go output(c)
		}

		go func() {
			wg.Wait()
			close(out)
		}()
		return out
	}

	// 🔧 パイプラインを構築
	in := gen(2, 3, 4, 5, 6)

	// 📤 ファンアウト（分散）
	c1 := sq(in)
	c2 := sq(in)

	// 📥 ファンイン（集約）
	for n := range merge(c1, c2) {
		fmt.Printf("  📊 二乗の結果: %d\n", n)
	}
}

// Example10_Pipeline - パイプラインパターン
// データを段階的に処理する方法を学びます
func Example10_Pipeline() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 例題10: パイプラインパターン           ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n📝 説明: 生成→二乗→偶数フィルタの3段階処理")

	// 🎯 ステージ1: 数値を生成
	generate := func(nums ...int) chan int {
		out := make(chan int)
		go func() {
			for _, n := range nums {
				out <- n
			}
			close(out)
		}()
		return out
	}

	// 🔢 ステージ2: 数値を二乗
	square := func(in chan int) chan int {
		out := make(chan int)
		go func() {
			for n := range in {
				out <- n * n
			}
			close(out)
		}()
		return out
	}

	// 🎯 ステージ3: 偶数のみをフィルタ
	filterEven := func(in chan int) chan int {
		out := make(chan int)
		go func() {
			for n := range in {
				if n%2 == 0 {
					out <- n
				}
			}
			close(out)
		}()
		return out
	}

	// 🔗 パイプラインを組み立て
	numbers := generate(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
	squared := square(numbers)
	evens := filterEven(squared)

	// 📤 出力を消費
	fmt.Println("\n🔄 パイプライン処理中...")
	for n := range evens {
		fmt.Printf("  🎯 結果（二乗した偶数）: %d\n", n)
	}
}

// Example11_Semaphore - セマフォパターン
// 同時実行数を制限する方法を学びます
func Example11_Semaphore() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 例題11: セマフォで同時実行数制限      ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n📝 説明: 10個のタスクを最大3個ずつ同時実行")

	const maxConcurrent = 3
	semaphore := make(chan struct{}, maxConcurrent)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 🔒 セマフォを取得
			semaphore <- struct{}{}
			defer func() { <-semaphore }() // 🔓 セマフォを解放

			fmt.Printf("  ▶️ タスク %d: 開始（最大%d同時実行）\n", id, maxConcurrent)
			time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)
			fmt.Printf("  ✅ タスク %d: 完了\n", id)
		}(i)
	}
	wg.Wait()
}