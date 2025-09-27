package evaluator

import (
	"testing"
	"time"
)

// TestEvaluateDeadlock デッドロック検出のテスト
func TestEvaluateDeadlock(t *testing.T) {
	eval := NewEvaluator()

	tests := []struct {
		name          string
		fn            func()
		timeout       time.Duration
		expectPassed  bool
		expectScore   int
	}{
		{
			name: "正常終了する関数",
			fn: func() {
				ch := make(chan int, 1)
				ch <- 1
				<-ch
			},
			timeout:      1 * time.Second,
			expectPassed: true,
			expectScore:  10,
		},
		{
			name: "デッドロックする関数",
			fn: func() {
				ch := make(chan int)
				ch <- 1 // デッドロック
			},
			timeout:      100 * time.Millisecond,
			expectPassed: false,
			expectScore:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.EvaluateDeadlock(tt.fn, tt.timeout)

			if result.Passed != tt.expectPassed {
				t.Errorf("expected Passed=%v, got %v", tt.expectPassed, result.Passed)
			}

			if result.Score != tt.expectScore {
				t.Errorf("expected Score=%d, got %d", tt.expectScore, result.Score)
			}
		})
	}
}

// TestEvaluateRaceCondition レース条件検出のテスト
func TestEvaluateRaceCondition(t *testing.T) {
	eval := NewEvaluator()

	tests := []struct {
		name          string
		fn            func() int
		expectedValue int
		expectPassed  bool
	}{
		{
			name: "レース条件なし",
			fn: func() int {
				return 100
			},
			expectedValue: 100,
			expectPassed:  true,
		},
		{
			name: "レース条件あり",
			fn: func() int {
				return 99 // 期待値と異なる
			},
			expectedValue: 100,
			expectPassed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.EvaluateRaceCondition(tt.fn, tt.expectedValue, 100)

			if result.Passed != tt.expectPassed {
				t.Errorf("expected Passed=%v, got %v", tt.expectPassed, result.Passed)
			}
		})
	}
}

// TestEvaluateGoroutineLeak ゴルーチンリーク検出のテスト
func TestEvaluateGoroutineLeak(t *testing.T) {
	eval := NewEvaluator()

	tests := []struct {
		name            string
		fn              func()
		acceptableLeaks int
		expectPassed    bool
	}{
		{
			name: "リークなし",
			fn: func() {
				done := make(chan struct{})
				go func() {
					<-done
				}()
				close(done)
				time.Sleep(50 * time.Millisecond)
			},
			acceptableLeaks: 1,
			expectPassed:    true,
		},
		{
			name: "リークあり",
			fn: func() {
				for i := 0; i < 3; i++ {
					go func() {
						time.Sleep(10 * time.Second)
					}()
				}
			},
			acceptableLeaks: 0,
			expectPassed:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := eval.EvaluateGoroutineLeak(tt.fn, tt.acceptableLeaks)

			if result.Passed != tt.expectPassed {
				t.Errorf("expected Passed=%v, got %v", tt.expectPassed, result.Passed)
			}
		})
	}
}

// TestEvaluatePerformance パフォーマンス評価のテスト
func TestEvaluatePerformance(t *testing.T) {
	eval := NewEvaluator()

	fastFunc := func() {
		time.Sleep(10 * time.Millisecond)
	}

	result := eval.EvaluatePerformance(fastFunc, 100*time.Millisecond)

	if result.Score < 8 {
		t.Errorf("expected high score for fast function, got %d", result.Score)
	}

	if !result.Passed {
		t.Error("expected fast function to pass")
	}
}

// TestGetResults 結果取得のテスト
func TestGetResults(t *testing.T) {
	eval := NewEvaluator()

	// いくつかのテストを実行
	eval.EvaluateDeadlock(func() {}, 1*time.Second)
	eval.EvaluateRaceCondition(func() int { return 10 }, 10, 10)

	results := eval.GetResults()

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

// BenchmarkEvaluateDeadlock デッドロック評価のベンチマーク
func BenchmarkEvaluateDeadlock(b *testing.B) {
	eval := NewEvaluator()
	fn := func() {
		ch := make(chan int, 1)
		ch <- 1
		<-ch
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eval.EvaluateDeadlock(fn, 100*time.Millisecond)
	}
}

// BenchmarkEvaluatePerformance パフォーマンス評価のベンチマーク
func BenchmarkEvaluatePerformance(b *testing.B) {
	eval := NewEvaluator()
	fn := func() {
		time.Sleep(1 * time.Millisecond)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eval.EvaluatePerformance(fn, 10*time.Millisecond)
	}
}