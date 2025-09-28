package interactive

import (
	"bufio"
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// InteractiveQuiz - インタラクティブな学習クイズ
type InteractiveQuiz struct {
	scanner     *bufio.Scanner
	score       int
	totalQuestions int
}

// NewInteractiveQuiz - クイズシステムを初期化
func NewInteractiveQuiz() *InteractiveQuiz {
	return &InteractiveQuiz{
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// StartQuiz - クイズを開始
func (q *InteractiveQuiz) StartQuiz() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║   🎮 インタラクティブ並行処理クイズ     ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n各問題には実際に動くコードが含まれています。")
	fmt.Println("正しい答えを選んでください！\n")

	questions := []func(){
		q.questionDeadlock,
		q.questionRaceCondition,
		q.questionChannelBuffer,
		q.questionContext,
		q.questionWorkerPool,
	}

	// ランダムな順序で出題
	rand.Shuffle(len(questions), func(i, j int) {
		questions[i], questions[j] = questions[j], questions[i]
	})

	for i, question := range questions {
		fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("問題 %d / %d\n", i+1, len(questions))
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		question()
		q.totalQuestions++
	}

	// 結果発表
	q.showResults()
}

// questionDeadlock - デッドロックに関する問題
func (q *InteractiveQuiz) questionDeadlock() {
	fmt.Println("\n📚 問題: デッドロック")
	fmt.Println("\n以下のコードを実行するとどうなりますか？\n")

	fmt.Println(`
    ch1 := make(chan int)
    ch2 := make(chan int)

    go func() {
        ch1 <- 1
        <-ch2
    }()

    go func() {
        ch2 <- 2
        <-ch1
    }()

    time.Sleep(1 * time.Second)
`)

	fmt.Println("1. 正常に終了する")
	fmt.Println("2. デッドロックが発生する")
	fmt.Println("3. パニックが発生する")
	fmt.Println("4. 無限ループになる")

	fmt.Print("\n答えを入力 (1-4): ")
	q.scanner.Scan()
	answer := q.scanner.Text()

	if answer == "2" {
		fmt.Println("✅ 正解！両方のゴルーチンがお互いの送信を待つためデッドロックが発生します。")

		// 実際に動かしてみる（タイムアウト付き）
		fmt.Println("\n💡 実演: タイムアウト付きで実行してみます...")
		demonstrateDeadlock()

		q.score++
	} else {
		fmt.Println("❌ 不正解。正解は 2 です。")
		fmt.Println("両方のゴルーチンが相手からの受信を待ち続けるため、デッドロックが発生します。")
	}
}

// demonstrateDeadlock - デッドロックを実演
func demonstrateDeadlock() {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	ch1 := make(chan int)
	ch2 := make(chan int)

	go func() {
		select {
		case ch1 <- 1:
			<-ch2
			fmt.Println("  ゴルーチン1: 完了")
		case <-ctx.Done():
			fmt.Println("  ゴルーチン1: タイムアウト！")
		}
	}()

	go func() {
		select {
		case ch2 <- 2:
			<-ch1
			fmt.Println("  ゴルーチン2: 完了")
		case <-ctx.Done():
			fmt.Println("  ゴルーチン2: タイムアウト！")
		}
	}()

	<-ctx.Done()
	fmt.Println("  ⚠️ デッドロックを検出しました！")
}

// questionRaceCondition - レース条件に関する問題
func (q *InteractiveQuiz) questionRaceCondition() {
	fmt.Println("\n📚 問題: レース条件")
	fmt.Println("\n以下のコードで、counterの最終値は？\n")

	fmt.Println(`
    var counter int
    var wg sync.WaitGroup

    for i := 0; i < 1000; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            counter++  // 同期なし！
        }()
    }
    wg.Wait()
`)

	fmt.Println("1. 必ず1000になる")
	fmt.Println("2. 0になる")
	fmt.Println("3. 1000未満になる可能性がある")
	fmt.Println("4. 1000を超える可能性がある")

	fmt.Print("\n答えを入力 (1-4): ")
	q.scanner.Scan()
	answer := q.scanner.Text()

	if answer == "3" {
		fmt.Println("✅ 正解！レース条件により、カウンタが正確に更新されない可能性があります。")

		// 実際に動かしてみる
		fmt.Println("\n💡 実演: 実際に10回実行してみます...")
		demonstrateRaceCondition()

		q.score++
	} else {
		fmt.Println("❌ 不正解。正解は 3 です。")
		fmt.Println("複数のゴルーチンが同時にcounterを更新するため、値が失われることがあります。")
	}
}

// demonstrateRaceCondition - レース条件を実演
func demonstrateRaceCondition() {
	results := make([]int, 10)

	for trial := 0; trial < 10; trial++ {
		var counter int
		var wg sync.WaitGroup

		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				counter++ // レース条件！
			}()
		}
		wg.Wait()
		results[trial] = counter
	}

	fmt.Print("  結果: ")
	for _, r := range results {
		fmt.Printf("%d ", r)
	}
	fmt.Println("\n  ⚠️ 値がバラバラになっています！")
}

// questionChannelBuffer - チャネルバッファに関する問題
func (q *InteractiveQuiz) questionChannelBuffer() {
	fmt.Println("\n📚 問題: チャネルバッファ")
	fmt.Println("\n以下のコードの出力は？\n")

	fmt.Println(`
    ch := make(chan int, 2)  // バッファサイズ2

    ch <- 1
    ch <- 2

    fmt.Println(<-ch)

    ch <- 3

    fmt.Println(<-ch)
    fmt.Println(<-ch)
`)

	fmt.Println("1. 1, 2, 3")
	fmt.Println("2. 3, 2, 1")
	fmt.Println("3. 1, 3, 2")
	fmt.Println("4. デッドロック")

	fmt.Print("\n答えを入力 (1-4): ")
	q.scanner.Scan()
	answer := q.scanner.Text()

	if answer == "1" {
		fmt.Println("✅ 正解！チャネルはFIFO（先入先出）で動作します。")

		// 実際に動かしてみる
		fmt.Println("\n💡 実演: 実際に実行してみます...")
		demonstrateChannelBuffer()

		q.score++
	} else {
		fmt.Println("❌ 不正解。正解は 1 です。")
		fmt.Println("バッファ付きチャネルは順番通り（FIFO）に値を取り出します。")
	}
}

// demonstrateChannelBuffer - チャネルバッファを実演
func demonstrateChannelBuffer() {
	ch := make(chan int, 2)

	ch <- 1
	fmt.Println("  送信: 1")
	ch <- 2
	fmt.Println("  送信: 2")

	fmt.Printf("  受信: %d\n", <-ch)

	ch <- 3
	fmt.Println("  送信: 3")

	fmt.Printf("  受信: %d\n", <-ch)
	fmt.Printf("  受信: %d\n", <-ch)
}

// questionContext - コンテキストに関する問題
func (q *InteractiveQuiz) questionContext() {
	fmt.Println("\n📚 問題: コンテキストキャンセル")
	fmt.Println("\ncontext.WithTimeout で1秒のタイムアウトを設定しました。")
	fmt.Println("2秒かかる処理を実行するとどうなりますか？\n")

	fmt.Println("1. 処理が完了するまで待つ")
	fmt.Println("2. 1秒でキャンセルされる")
	fmt.Println("3. パニックが発生する")
	fmt.Println("4. デッドロックになる")

	fmt.Print("\n答えを入力 (1-4): ")
	q.scanner.Scan()
	answer := q.scanner.Text()

	if answer == "2" {
		fmt.Println("✅ 正解！タイムアウトでコンテキストがキャンセルされます。")

		// 実際に動かしてみる
		fmt.Println("\n💡 実演: 実際に実行してみます...")
		demonstrateContext()

		q.score++
	} else {
		fmt.Println("❌ 不正解。正解は 2 です。")
		fmt.Println("WithTimeoutで設定した時間が経過するとコンテキストが自動的にキャンセルされます。")
	}
}

// demonstrateContext - コンテキストを実演
func demonstrateContext() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	fmt.Println("  開始: 2秒かかる処理を実行...")

	select {
	case <-time.After(2 * time.Second):
		fmt.Println("  処理完了！")
	case <-ctx.Done():
		fmt.Println("  ⏰ タイムアウト！処理がキャンセルされました。")
	}
}

// questionWorkerPool - ワーカープールに関する問題
func (q *InteractiveQuiz) questionWorkerPool() {
	fmt.Println("\n📚 問題: ワーカープール")
	fmt.Println("\n3つのワーカーで10個のタスクを処理します。")
	fmt.Println("最も効率的な実装は？\n")

	fmt.Println("1. 各タスクごとに新しいゴルーチンを作成")
	fmt.Println("2. 3つの固定ワーカーでタスクキューを処理")
	fmt.Println("3. 1つのゴルーチンで順番に処理")
	fmt.Println("4. 10個のゴルーチンを同時に起動")

	fmt.Print("\n答えを入力 (1-4): ")
	q.scanner.Scan()
	answer := q.scanner.Text()

	if answer == "2" {
		fmt.Println("✅ 正解！固定数のワーカーでキューを処理するのが効率的です。")

		// 実際に動かしてみる
		fmt.Println("\n💡 実演: ワーカープールを実行してみます...")
		demonstrateWorkerPool()

		q.score++
	} else {
		fmt.Println("❌ 不正解。正解は 2 です。")
		fmt.Println("固定数のワーカーを使うことで、ゴルーチンの作成コストを削減できます。")
	}
}

// demonstrateWorkerPool - ワーカープールを実演
func demonstrateWorkerPool() {
	tasks := make(chan int, 10)
	results := make(chan string, 10)

	// 3つのワーカーを起動
	var wg sync.WaitGroup
	for w := 1; w <= 3; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for task := range tasks {
				time.Sleep(100 * time.Millisecond) // 作業をシミュレート
				results <- fmt.Sprintf("ワーカー%d: タスク%d完了", id, task)
			}
		}(w)
	}

	// タスクを投入
	for i := 1; i <= 10; i++ {
		tasks <- i
	}
	close(tasks)

	// 結果を収集
	go func() {
		wg.Wait()
		close(results)
	}()

	// 結果を表示
	for result := range results {
		fmt.Printf("  %s\n", result)
	}
}

// showResults - クイズ結果を表示
func (q *InteractiveQuiz) showResults() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║           📊 クイズ結果                ║")
	fmt.Println("╚════════════════════════════════════════╝")

	percentage := float64(q.score) / float64(q.totalQuestions) * 100
	fmt.Printf("\nスコア: %d / %d (%.1f%%)\n", q.score, q.totalQuestions, percentage)

	// 評価
	switch {
	case percentage >= 80:
		fmt.Println("\n🏆 素晴らしい！並行処理をよく理解しています！")
	case percentage >= 60:
		fmt.Println("\n👍 良いですね！もう少しで完璧です！")
	case percentage >= 40:
		fmt.Println("\n💪 頑張りましょう！基礎を復習することをお勧めします。")
	default:
		fmt.Println("\n📚 もう一度基礎から学習しましょう！")
	}

	fmt.Println("\n💡 ヒント: 各問題の実演コードを参考に、実際に手を動かして学習しましょう！")
}

// RealTimeVisualization - リアルタイム可視化
type RealTimeVisualization struct {
	activeGoroutines int64
	tasksProcessed   int64
	errors           int64
	startTime        time.Time
}

// NewVisualization - 可視化システムを初期化
func NewVisualization() *RealTimeVisualization {
	return &RealTimeVisualization{
		startTime: time.Now(),
	}
}

// VisualizeWorkerPool - ワーカープールを可視化
func (v *RealTimeVisualization) VisualizeWorkerPool() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║    📊 ワーカープール リアルタイム監視    ║")
	fmt.Println("╚════════════════════════════════════════╝")

	const numWorkers = 5
	const numTasks = 20

	tasks := make(chan int, numTasks)

	// 監視用のティッカー
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	done := make(chan bool)

	// 監視ゴルーチン
	go func() {
		for {
			select {
			case <-ticker.C:
				v.printStatus()
			case <-done:
				return
			}
		}
	}()

	// ワーカープール
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		atomic.AddInt64(&v.activeGoroutines, 1)

		go func(id int) {
			defer wg.Done()
			defer atomic.AddInt64(&v.activeGoroutines, -1)

			for task := range tasks {
				// ランダムな処理時間
				processingTime := time.Duration(100+rand.Intn(400)) * time.Millisecond
				time.Sleep(processingTime)

				// ランダムにエラーを発生させる
				if rand.Float32() < 0.1 {
					atomic.AddInt64(&v.errors, 1)
					fmt.Printf("  ❌ ワーカー%d: タスク%d でエラー発生\n", id, task)
				} else {
					atomic.AddInt64(&v.tasksProcessed, 1)
					fmt.Printf("  ✅ ワーカー%d: タスク%d 完了 (%dms)\n",
						id, task, processingTime.Milliseconds())
				}
			}
		}(i)
	}

	// タスク投入
	fmt.Println("\n📋 タスクを投入中...")
	for i := 0; i < numTasks; i++ {
		tasks <- i
		time.Sleep(200 * time.Millisecond)
	}
	close(tasks)

	wg.Wait()
	done <- true

	// 最終統計
	v.printFinalStats()
}

// printStatus - 現在の状態を表示
func (v *RealTimeVisualization) printStatus() {
	elapsed := time.Since(v.startTime).Seconds()
	throughput := float64(atomic.LoadInt64(&v.tasksProcessed)) / elapsed

	fmt.Printf("\n━━━ 状態更新 [%.1fs経過] ━━━\n", elapsed)
	fmt.Printf("  🏃 アクティブ: %d ゴルーチン\n", atomic.LoadInt64(&v.activeGoroutines))
	fmt.Printf("  ✅ 完了: %d タスク\n", atomic.LoadInt64(&v.tasksProcessed))
	fmt.Printf("  ❌ エラー: %d 件\n", atomic.LoadInt64(&v.errors))
	fmt.Printf("  📈 スループット: %.2f タスク/秒\n", throughput)
}

// printFinalStats - 最終統計を表示
func (v *RealTimeVisualization) printFinalStats() {
	totalTime := time.Since(v.startTime)
	totalTasks := atomic.LoadInt64(&v.tasksProcessed)
	totalErrors := atomic.LoadInt64(&v.errors)

	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║            📊 最終統計                 ║")
	fmt.Println("╚════════════════════════════════════════╝")

	fmt.Printf("\n  ⏱️ 総処理時間: %v\n", totalTime)
	fmt.Printf("  ✅ 成功タスク: %d\n", totalTasks)
	fmt.Printf("  ❌ エラー数: %d\n", totalErrors)
	fmt.Printf("  📈 平均スループット: %.2f タスク/秒\n",
		float64(totalTasks)/totalTime.Seconds())

	successRate := float64(totalTasks) / float64(totalTasks+totalErrors) * 100
	fmt.Printf("  🎯 成功率: %.1f%%\n", successRate)
}