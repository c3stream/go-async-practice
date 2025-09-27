package evaluator

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
)

// TestResult はテストの結果を表す
type TestResult struct {
	Name        string
	Passed      bool
	Score       int
	MaxScore    int
	Duration    time.Duration
	MemoryUsed  uint64
	Goroutines  int
	Description string
	Errors      []string
}

// Evaluator は並行プログラムを評価する
type Evaluator struct {
	mu      sync.Mutex
	results []TestResult
}

// NewEvaluator creates a new evaluator
func NewEvaluator() *Evaluator {
	return &Evaluator{
		results: make([]TestResult, 0),
	}
}

// EvaluateDeadlock デッドロックの検出を評価
func (e *Evaluator) EvaluateDeadlock(fn func(), timeout time.Duration) TestResult {
	result := TestResult{
		Name:     "Deadlock Detection",
		MaxScore: 10,
	}

	done := make(chan bool)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("Panic: %v", r))
			}
		}()
		fn()
		done <- true
	}()

	select {
	case <-done:
		result.Passed = true
		result.Score = 10
		result.Description = "関数は正常に終了しました"
	case <-time.After(timeout):
		result.Passed = false
		result.Score = 0
		result.Description = "デッドロックが検出されました"
		result.Errors = append(result.Errors, "Function timed out - possible deadlock")
	}

	e.mu.Lock()
	e.results = append(e.results, result)
	e.mu.Unlock()

	return result
}

// EvaluateRaceCondition レース条件を評価
func (e *Evaluator) EvaluateRaceCondition(fn func() int, expectedValue int, iterations int) TestResult {
	result := TestResult{
		Name:     "Race Condition Detection",
		MaxScore: 10,
	}

	start := time.Now()
	actualValue := fn()
	result.Duration = time.Since(start)

	if actualValue == expectedValue {
		result.Passed = true
		result.Score = 10
		result.Description = fmt.Sprintf("期待値通りの結果: %d", actualValue)
	} else {
		result.Passed = false
		result.Score = 5
		result.Description = fmt.Sprintf("レース条件の可能性: 期待値=%d, 実際=%d", expectedValue, actualValue)
		result.Errors = append(result.Errors, "値が一致しません - レース条件の可能性があります")
	}

	e.mu.Lock()
	e.results = append(e.results, result)
	e.mu.Unlock()

	return result
}

// EvaluateGoroutineLeak ゴルーチンリークを評価
func (e *Evaluator) EvaluateGoroutineLeak(fn func(), acceptableLeaks int) TestResult {
	result := TestResult{
		Name:     "Goroutine Leak Detection",
		MaxScore: 10,
	}

	initialGoroutines := runtime.NumGoroutine()
	fn()

	// GCを実行してゴルーチンのクリーンアップを待つ
	runtime.GC()
	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	leaked := finalGoroutines - initialGoroutines

	result.Goroutines = leaked

	if leaked <= acceptableLeaks {
		result.Passed = true
		result.Score = 10
		result.Description = fmt.Sprintf("ゴルーチンリークなし (差分: %d)", leaked)
	} else {
		result.Passed = false
		result.Score = 10 - leaked
		if result.Score < 0 {
			result.Score = 0
		}
		result.Description = fmt.Sprintf("ゴルーチンリーク検出: %d個のリーク", leaked)
		result.Errors = append(result.Errors, fmt.Sprintf("%d goroutines leaked", leaked))
	}

	e.mu.Lock()
	e.results = append(e.results, result)
	e.mu.Unlock()

	return result
}

// EvaluatePerformance パフォーマンスを評価
func (e *Evaluator) EvaluatePerformance(fn func(), baseline time.Duration) TestResult {
	result := TestResult{
		Name:     "Performance Evaluation",
		MaxScore: 10,
	}

	// メモリ使用量の計測
	var m1, m2 runtime.MemStats
	runtime.ReadMemStats(&m1)

	start := time.Now()
	fn()
	result.Duration = time.Since(start)

	runtime.ReadMemStats(&m2)
	result.MemoryUsed = m2.TotalAlloc - m1.TotalAlloc

	// パフォーマンス評価
	ratio := float64(result.Duration) / float64(baseline)

	if ratio <= 0.5 {
		result.Score = 10
		result.Description = "非常に高速 (ベースラインの50%以下)"
	} else if ratio <= 0.8 {
		result.Score = 9
		result.Description = "高速 (ベースラインの80%以下)"
	} else if ratio <= 1.2 {
		result.Score = 8
		result.Description = "標準的 (ベースライン±20%)"
	} else if ratio <= 1.5 {
		result.Score = 6
		result.Description = "やや遅い (ベースラインの150%以下)"
	} else {
		result.Score = 4
		result.Description = "改善が必要 (ベースラインの150%超)"
	}

	result.Passed = result.Score >= 6

	e.mu.Lock()
	e.results = append(e.results, result)
	e.mu.Unlock()

	return result
}

// GetResults すべての結果を取得
func (e *Evaluator) GetResults() []TestResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.results
}

// PrintSummary 結果のサマリーを表示
func (e *Evaluator) PrintSummary() {
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("評価結果サマリー")
	fmt.Println(strings.Repeat("=", 50))

	totalScore := 0
	maxScore := 0
	passed := 0

	for _, r := range e.results {
		status := "❌"
		if r.Passed {
			status = "✅"
			passed++
		}

		fmt.Printf("%s %s: %d/%d点\n", status, r.Name, r.Score, r.MaxScore)
		fmt.Printf("   %s\n", r.Description)

		if r.Duration > 0 {
			fmt.Printf("   実行時間: %v\n", r.Duration)
		}

		if r.MemoryUsed > 0 {
			fmt.Printf("   メモリ使用: %d bytes\n", r.MemoryUsed)
		}

		if r.Goroutines != 0 {
			fmt.Printf("   ゴルーチン差分: %+d\n", r.Goroutines)
		}

		for _, err := range r.Errors {
			fmt.Printf("   ⚠️  %s\n", err)
		}

		fmt.Println()

		totalScore += r.Score
		maxScore += r.MaxScore
	}

	percentage := float64(totalScore) / float64(maxScore) * 100

	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("総合スコア: %d/%d (%.1f%%)\n", totalScore, maxScore, percentage)
	fmt.Printf("合格テスト: %d/%d\n", passed, len(e.results))

	// 評価
	fmt.Print("\n総合評価: ")
	switch {
	case percentage >= 90:
		fmt.Println("🏆 優秀！並行プログラミングをよく理解しています。")
	case percentage >= 70:
		fmt.Println("👍 良好！基本は理解できていますが、改善の余地があります。")
	case percentage >= 50:
		fmt.Println("📚 もう少し！重要な概念をさらに学習しましょう。")
	default:
		fmt.Println("💪 頑張りましょう！基礎から復習することをお勧めします。")
	}
	fmt.Println(strings.Repeat("=", 50))
}