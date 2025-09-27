package interactive

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// InteractiveExercise はインタラクティブな練習問題
type InteractiveExercise struct {
	scanner *bufio.Scanner
	score   int32
}

// NewInteractiveExercise 新しい練習問題セッションを作成
func NewInteractiveExercise() *InteractiveExercise {
	return &InteractiveExercise{
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// Exercise1_ChannelOrdering - Channelの順序を理解する練習
func (ie *InteractiveExercise) Exercise1_ChannelOrdering() {
	fmt.Println("\n🎯 Exercise 1: Channel順序予測")
	fmt.Println("以下のコードを見て、出力される数字の順序を予測してください。")
	fmt.Println("========================================")

	code := `
ch := make(chan int, 2)
go func() {
    ch <- 1
    time.Sleep(100 * time.Millisecond)
    ch <- 2
}()

go func() {
    time.Sleep(50 * time.Millisecond)
    ch <- 3
}()

for i := 0; i < 3; i++ {
    fmt.Println(<-ch)
}
`
	fmt.Println(code)
	fmt.Println("========================================")
	fmt.Print("予想される出力順序（スペース区切りで入力 例: 1 3 2）: ")

	ie.scanner.Scan()
	answer := ie.scanner.Text()

	// 実際に実行
	ch := make(chan int, 2)
	var results []int
	var mu sync.Mutex

	go func() {
		ch <- 1
		time.Sleep(100 * time.Millisecond)
		ch <- 2
	}()

	go func() {
		time.Sleep(50 * time.Millisecond)
		ch <- 3
	}()

	for i := 0; i < 3; i++ {
		val := <-ch
		mu.Lock()
		results = append(results, val)
		mu.Unlock()
	}

	resultStr := fmt.Sprintf("%d %d %d", results[0], results[1], results[2])

	if answer == resultStr {
		fmt.Println("✅ 正解！Channelの動作を理解しています。")
		atomic.AddInt32(&ie.score, 10)
	} else {
		fmt.Printf("❌ 不正解。実際の出力: %s\n", resultStr)
		fmt.Println("💡 ヒント: バッファサイズとgoroutineの実行タイミングを考えてみてください。")
	}
}

// Exercise2_DeadlockPrediction - デッドロックを予測する練習
func (ie *InteractiveExercise) Exercise2_DeadlockPrediction() {
	fmt.Println("\n🎯 Exercise 2: デッドロック予測")
	fmt.Println("以下の各コードスニペットについて、デッドロックが発生するか予測してください。")

	snippets := []struct {
		code     string
		deadlock bool
		hint     string
	}{
		{
			code: `
// Snippet 1
ch := make(chan int)
ch <- 1
<-ch`,
			deadlock: true,
			hint: "unbuffered channelへの送信は受信者がいないとブロックします",
		},
		{
			code: `
// Snippet 2
ch := make(chan int, 1)
ch <- 1
<-ch`,
			deadlock: false,
			hint: "buffered channelは容量まで送信できます",
		},
		{
			code: `
// Snippet 3
ch1 := make(chan int)
ch2 := make(chan int)
go func() { ch1 <- <-ch2 }()
go func() { ch2 <- <-ch1 }()`,
			deadlock: true,
			hint: "循環的な依存関係はデッドロックを引き起こします",
		},
	}

	correctCount := 0
	for i, snippet := range snippets {
		fmt.Printf("\n========================================")
		fmt.Printf("\nSnippet %d:\n", i+1)
		fmt.Println(snippet.code)
		fmt.Print("デッドロックが発生しますか？ (yes/no): ")

		ie.scanner.Scan()
		answer := strings.ToLower(strings.TrimSpace(ie.scanner.Text()))

		isDeadlock := answer == "yes" || answer == "y"

		if isDeadlock == snippet.deadlock {
			fmt.Println("✅ 正解！")
			correctCount++
			atomic.AddInt32(&ie.score, 5)
		} else {
			fmt.Println("❌ 不正解")
			fmt.Printf("💡 ヒント: %s\n", snippet.hint)
		}
	}

	fmt.Printf("\n結果: %d/3 正解\n", correctCount)
}

// Exercise3_RaceConditionFix - レース条件を修正する練習
func (ie *InteractiveExercise) Exercise3_RaceConditionFix() {
	fmt.Println("\n🎯 Exercise 3: レース条件の修正方法選択")
	fmt.Println("以下のコードにはレース条件があります。")
	fmt.Println("========================================")

	code := `
var counter int
var wg sync.WaitGroup

for i := 0; i < 1000; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        counter++  // レース条件！
    }()
}
wg.Wait()
fmt.Println(counter)`

	fmt.Println(code)
	fmt.Println("========================================")
	fmt.Println("\n修正方法を選んでください:")
	fmt.Println("1. sync.Mutexを使う")
	fmt.Println("2. atomic操作を使う")
	fmt.Println("3. channelを使う")
	fmt.Println("4. 全て正しい")
	fmt.Print("\n選択 (1-4): ")

	ie.scanner.Scan()
	choice, _ := strconv.Atoi(ie.scanner.Text())

	if choice == 4 {
		fmt.Println("✅ 正解！すべての方法で修正可能です。")
		atomic.AddInt32(&ie.score, 10)

		fmt.Println("\n📝 各方法の例:")
		fmt.Println("1. Mutex: mu.Lock(); counter++; mu.Unlock()")
		fmt.Println("2. Atomic: atomic.AddInt64(&counter, 1)")
		fmt.Println("3. Channel: ch <- 1 で送信し、別goroutineで集計")
	} else if choice >= 1 && choice <= 3 {
		fmt.Println("⚠️  部分的に正解。実はすべての方法で修正可能です。")
		atomic.AddInt32(&ie.score, 5)
	} else {
		fmt.Println("❌ 不正解")
	}
}

// Exercise4_SelectBehavior - Select文の動作を理解する
func (ie *InteractiveExercise) Exercise4_SelectBehavior() {
	fmt.Println("\n🎯 Exercise 4: Select文の動作予測")
	fmt.Println("以下のselect文の動作を予測してください。")
	fmt.Println("========================================")

	code := `
ch1 := make(chan string)
ch2 := make(chan string)

go func() {
    time.Sleep(100 * time.Millisecond)
    ch1 <- "one"
}()

go func() {
    time.Sleep(200 * time.Millisecond)
    ch2 <- "two"
}()

select {
case msg1 := <-ch1:
    fmt.Println("Received:", msg1)
case msg2 := <-ch2:
    fmt.Println("Received:", msg2)
case <-time.After(50 * time.Millisecond):
    fmt.Println("Timeout!")
}`

	fmt.Println(code)
	fmt.Println("========================================")
	fmt.Println("何が出力されますか？")
	fmt.Println("1. Received: one")
	fmt.Println("2. Received: two")
	fmt.Println("3. Timeout!")
	fmt.Print("\n選択 (1-3): ")

	ie.scanner.Scan()
	choice, _ := strconv.Atoi(ie.scanner.Text())

	if choice == 3 {
		fmt.Println("✅ 正解！50ms後にタイムアウトします。")
		atomic.AddInt32(&ie.score, 10)
		fmt.Println("💡 解説: ch1は100ms後、ch2は200ms後に値を送信しますが、")
		fmt.Println("   time.Afterが50ms後に発火するため、タイムアウトケースが選択されます。")
	} else {
		fmt.Println("❌ 不正解。正解は「3. Timeout!」です。")
		fmt.Println("💡 ヒント: 各channelへの送信タイミングとtime.Afterの時間を比較してください。")
	}
}

// Exercise5_ConcurrencyPatternMatch - 並行パターンのマッチング
func (ie *InteractiveExercise) Exercise5_ConcurrencyPatternMatch() {
	fmt.Println("\n🎯 Exercise 5: 並行パターンの識別")
	fmt.Println("コードスニペットと対応するパターン名をマッチさせてください。")

	patterns := []struct {
		code        string
		pattern     string
		description string
	}{
		{
			code: `
for i := 0; i < numWorkers; i++ {
    go worker(jobs, results)
}`,
			pattern:     "Worker Pool",
			description: "固定数のworkerで並列処理",
		},
		{
			code: `
out := merge(c1, c2, c3)`,
			pattern:     "Fan-In",
			description: "複数のchannelを1つに統合",
		},
		{
			code: `
stage1 := gen(nums)
stage2 := sq(stage1)
stage3 := filter(stage2)`,
			pattern:     "Pipeline",
			description: "データを段階的に処理",
		},
	}

	fmt.Println("\nパターン一覧:")
	fmt.Println("A. Pipeline")
	fmt.Println("B. Worker Pool")
	fmt.Println("C. Fan-In")
	fmt.Println("D. Mutex")

	correctCount := 0
	for i, p := range patterns {
		fmt.Printf("\n========================================")
		fmt.Printf("\nコード %d:\n", i+1)
		fmt.Println(p.code)
		fmt.Print("対応するパターンは？ (A-D): ")

		ie.scanner.Scan()
		answer := strings.ToUpper(strings.TrimSpace(ie.scanner.Text()))

		correct := false
		switch p.pattern {
		case "Pipeline":
			correct = answer == "A"
		case "Worker Pool":
			correct = answer == "B"
		case "Fan-In":
			correct = answer == "C"
		}

		if correct {
			fmt.Printf("✅ 正解！%s: %s\n", p.pattern, p.description)
			correctCount++
			atomic.AddInt32(&ie.score, 5)
		} else {
			fmt.Printf("❌ 不正解。正解は %s です。\n", p.pattern)
			fmt.Printf("💡 %s\n", p.description)
		}
	}

	fmt.Printf("\n結果: %d/3 正解\n", correctCount)
}

// RunAllExercises すべての練習問題を実行
func (ie *InteractiveExercise) RunAllExercises() {
	fmt.Println("\n🏫 インタラクティブ練習問題へようこそ！")
	fmt.Println("各問題に答えて、並行プログラミングの理解を深めましょう。")

	exercises := []func(){
		ie.Exercise1_ChannelOrdering,
		ie.Exercise2_DeadlockPrediction,
		ie.Exercise3_RaceConditionFix,
		ie.Exercise4_SelectBehavior,
		ie.Exercise5_ConcurrencyPatternMatch,
	}

	for i, exercise := range exercises {
		fmt.Printf("\n[%d/%d] ", i+1, len(exercises))
		exercise()
		time.Sleep(500 * time.Millisecond)
	}

	ie.ShowFinalScore()
}

// ShowFinalScore 最終スコアを表示
func (ie *InteractiveExercise) ShowFinalScore() {
	finalScore := atomic.LoadInt32(&ie.score)
	maxScore := int32(50) // 各演習の最大スコアの合計

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("📊 最終結果")
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("スコア: %d/%d点 (%.1f%%)\n", finalScore, maxScore, float64(finalScore)/float64(maxScore)*100)

	percentage := float64(finalScore) / float64(maxScore) * 100
	switch {
	case percentage >= 90:
		fmt.Println("🏆 素晴らしい！並行プログラミングをマスターしています！")
	case percentage >= 70:
		fmt.Println("👍 良い調子！理解が深まっています。")
	case percentage >= 50:
		fmt.Println("📚 もう少し！重要な概念を復習しましょう。")
	default:
		fmt.Println("💪 練習あるのみ！例題をもう一度確認してみましょう。")
	}
	fmt.Println(strings.Repeat("=", 50))
}