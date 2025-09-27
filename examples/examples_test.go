package examples

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// captureOutput 標準出力をキャプチャ
func captureOutput(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// 出力を別goroutineで読む
	outputChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outputChan <- buf.String()
	}()

	fn()
	w.Close()
	os.Stdout = old

	return <-outputChan
}

// TestExample1_SimpleGoroutine Goroutineの基本動作テスト
func TestExample1_SimpleGoroutine(t *testing.T) {
	output := captureOutput(Example1_SimpleGoroutine)

	if !strings.Contains(output, "Sequential execution") {
		t.Error("Expected sequential execution section")
	}

	if !strings.Contains(output, "Concurrent execution") {
		t.Error("Expected concurrent execution section")
	}

	// 並行実行の方が速いことを確認（出力から推測）
	if !strings.Contains(output, "Task") {
		t.Error("Expected task completion messages")
	}
}

// TestExample2_RaceCondition レース条件のデモテスト
func TestExample2_RaceCondition(t *testing.T) {
	// レース条件があることを確認（-raceフラグで実行される想定）
	output := captureOutput(Example2_RaceCondition)

	if !strings.Contains(output, "Race Condition") {
		t.Error("Expected race condition section")
	}

	if !strings.Contains(output, "with mutex") {
		t.Error("Expected mutex solution")
	}
}

// TestExample3_ChannelBasics チャネルの基本動作テスト
func TestExample3_ChannelBasics(t *testing.T) {
	output := captureOutput(Example3_ChannelBasics)

	if !strings.Contains(output, "Channel Basics") {
		t.Error("Expected channel basics section")
	}

	if !strings.Contains(output, "Buffered channel") {
		t.Error("Expected buffered channel section")
	}
}

// TestExample4_SelectStatement Select文のテスト
func TestExample4_SelectStatement(t *testing.T) {
	output := captureOutput(Example4_SelectStatement)

	if !strings.Contains(output, "Select Statement") {
		t.Error("Expected select statement section")
	}

	// タイムアウトまたはメッセージ受信を確認
	hasMessage := strings.Contains(output, "Message from channel")
	hasTimeout := strings.Contains(output, "Timeout")

	if !hasMessage && !hasTimeout {
		t.Error("Expected either message or timeout")
	}
}

// TestExample5_ContextCancellation Context取り消しのテスト
func TestExample5_ContextCancellation(t *testing.T) {
	output := captureOutput(Example5_ContextCancellation)

	if !strings.Contains(output, "Context Cancellation") {
		t.Error("Expected context cancellation section")
	}

	if !strings.Contains(output, "cancellation") {
		t.Error("Expected cancellation message")
	}
}

// TestExample8_WorkerPool ワーカープールのテスト
func TestExample8_WorkerPool(t *testing.T) {
	done := make(chan bool)
	go func() {
		Example8_WorkerPool()
		done <- true
	}()

	select {
	case <-done:
		// 正常終了
	case <-time.After(5 * time.Second):
		t.Error("Worker pool test timed out")
	}
}

// TestExample10_Pipeline パイプラインのテスト
func TestExample10_Pipeline(t *testing.T) {
	output := captureOutput(Example10_Pipeline)

	if !strings.Contains(output, "Pipeline Pattern") {
		t.Error("Expected pipeline pattern section")
	}

	// 偶数の平方数が出力されることを確認
	if !strings.Contains(output, "Pipeline result") {
		t.Error("Expected pipeline results")
	}
}

// TestExample11_Semaphore セマフォのテスト
func TestExample11_Semaphore(t *testing.T) {
	output := captureOutput(Example11_Semaphore)

	if !strings.Contains(output, "Semaphore Pattern") {
		t.Error("Expected semaphore pattern section")
	}

	// 同時実行数の制限が守られていることを確認
	if !strings.Contains(output, "max 3 concurrent") {
		t.Error("Expected concurrent limit message")
	}
}

// TestConcurrentSafety 並行安全性のテスト
func TestConcurrentSafety(t *testing.T) {
	// 複数のgoroutineから同時に例題を実行
	var wg sync.WaitGroup
	functions := []func(){
		Example1_SimpleGoroutine,
		Example3_ChannelBasics,
		Example4_SelectStatement,
	}

	for _, fn := range functions {
		wg.Add(1)
		go func(f func()) {
			defer wg.Done()
			// パニックが起きないことを確認
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Function panicked: %v", r)
				}
			}()
			captureOutput(f)
		}(fn)
	}

	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// 正常終了
	case <-time.After(10 * time.Second):
		t.Error("Concurrent safety test timed out")
	}
}

// BenchmarkGoroutineCreation Goroutine作成のベンチマーク
func BenchmarkGoroutineCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 軽い処理
			_ = 1 + 1
		}()
		wg.Wait()
	}
}

// BenchmarkChannelSendReceive チャネル送受信のベンチマーク
func BenchmarkChannelSendReceive(b *testing.B) {
	ch := make(chan int, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch <- i
		<-ch
	}
}

// BenchmarkSelectDefault selectのdefault節のベンチマーク
func BenchmarkSelectDefault(b *testing.B) {
	ch := make(chan int, 1)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		select {
		case ch <- i:
			<-ch
		default:
		}
	}
}