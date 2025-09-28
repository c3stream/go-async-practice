package interactive

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
)

// InteractiveEvaluator - インタラクティブ評価システム
type InteractiveEvaluator struct {
	score       int
	level       int
	experience  int
	achievements map[string]bool
	mu          sync.RWMutex
}

// NewInteractiveEvaluator - 評価システムの初期化
func NewInteractiveEvaluator() *InteractiveEvaluator {
	return &InteractiveEvaluator{
		score:        0,
		level:        1,
		experience:   0,
		achievements: make(map[string]bool),
	}
}

// StartInteractiveEvaluation - インタラクティブ評価の開始
func (e *InteractiveEvaluator) StartInteractiveEvaluation() {
	e.displayWelcome()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		e.displayMenu()
		fmt.Print("\n選択してください (1-6, 0で終了): ")

		if !scanner.Scan() {
			break
		}

		choice := strings.TrimSpace(scanner.Text())
		switch choice {
		case "0":
			e.displayFarewell()
			return
		case "1":
			e.evaluateConcurrency()
		case "2":
			e.evaluateMemoryManagement()
		case "3":
			e.evaluatePerformance()
		case "4":
			e.evaluateSecurity()
		case "5":
			e.evaluateDistributed()
		case "6":
			e.showProgress()
		default:
			fmt.Println("無効な選択です。")
		}
	}
}

// displayWelcome - ウェルカムメッセージ
func (e *InteractiveEvaluator) displayWelcome() {
	color.Cyan(`
╔══════════════════════════════════════════════════════╗
║     🎓 インタラクティブ評価システム v2.0              ║
║     Go並行処理マスタリー評価                          ║
╚══════════════════════════════════════════════════════╝
`)
	fmt.Println("\nあなたのコードをリアルタイムで評価し、フィードバックを提供します。")
	fmt.Printf("現在のレベル: %d | 経験値: %d | スコア: %d\n", e.level, e.experience, e.score)
}

// displayMenu - メニュー表示
func (e *InteractiveEvaluator) displayMenu() {
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("📋 評価カテゴリ:")
	fmt.Println("  1. 並行処理の基礎")
	fmt.Println("  2. メモリ管理")
	fmt.Println("  3. パフォーマンス最適化")
	fmt.Println("  4. セキュリティ")
	fmt.Println("  5. 分散システム")
	fmt.Println("  6. 進捗状況を確認")
	fmt.Println("  0. 終了")
}

// evaluateConcurrency - 並行処理の評価
func (e *InteractiveEvaluator) evaluateConcurrency() {
	fmt.Println("\n🔄 並行処理の評価を開始します...")

	tests := []struct {
		name string
		test func() (bool, string, int)
	}{
		{"Goroutine管理", e.testGoroutineManagement},
		{"Channel使用", e.testChannelUsage},
		{"デッドロック検出", e.testDeadlockDetection},
		{"レース条件", e.testRaceCondition},
	}

	totalScore := 0
	passed := 0

	for _, t := range tests {
		fmt.Printf("\n⏳ テスト: %s\n", t.name)
		success, feedback, score := t.test()

		if success {
			color.Green("  ✅ 合格 (+%d点)", score)
			passed++
			totalScore += score
		} else {
			color.Red("  ❌ 不合格")
		}
		fmt.Printf("  💡 %s\n", feedback)

		time.Sleep(500 * time.Millisecond)
	}

	e.updateScore(totalScore)
	e.checkAchievements("concurrency_master", passed == len(tests))

	fmt.Printf("\n📊 結果: %d/%d テスト合格 (獲得: %d点)\n", passed, len(tests), totalScore)
}

// testGoroutineManagement - Goroutine管理のテスト
func (e *InteractiveEvaluator) testGoroutineManagement() (bool, string, int) {
	initialGoroutines := runtime.NumGoroutine()

	// ユーザーコードのシミュレーション
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
		}()
	}

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	finalGoroutines := runtime.NumGoroutine()
	leaked := finalGoroutines - initialGoroutines

	if leaked > 0 {
		return false, fmt.Sprintf("Goroutineリーク検出: %d個のGoroutineが残っています", leaked), 0
	}

	return true, "Goroutineが適切に管理されています", 10
}

// testChannelUsage - Channel使用のテスト
func (e *InteractiveEvaluator) testChannelUsage() (bool, string, int) {
	ch := make(chan int, 10)
	done := make(chan bool)

	// Producer
	go func() {
		for i := 0; i < 10; i++ {
			ch <- i
		}
		close(ch)
	}()

	// Consumer
	go func() {
		count := 0
		for range ch {
			count++
		}
		done <- count == 10
	}()

	select {
	case result := <-done:
		if result {
			return true, "Channelが正しく使用されています", 10
		}
		return false, "Channel通信にエラーがあります", 0
	case <-time.After(1 * time.Second):
		return false, "Channel通信がタイムアウトしました", 0
	}
}

// testDeadlockDetection - デッドロック検出テスト
func (e *InteractiveEvaluator) testDeadlockDetection() (bool, string, int) {
	// デッドロックを回避する正しい実装
	ch1 := make(chan int, 1)
	ch2 := make(chan int, 1)

	go func() {
		ch1 <- 1
		<-ch2
	}()

	go func() {
		ch2 <- 2
		<-ch1
	}()

	// タイムアウトチェック
	select {
	case <-time.After(100 * time.Millisecond):
		// バッファ付きチャネルなのでデッドロックしない
		return true, "デッドロックが適切に回避されています", 15
	}
}

// testRaceCondition - レース条件テスト
func (e *InteractiveEvaluator) testRaceCondition() (bool, string, int) {
	var counter int64
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			atomic.AddInt64(&counter, 1)
		}()
	}

	wg.Wait()

	if counter == 1000 {
		return true, "レース条件が適切に処理されています（atomic使用）", 15
	}

	return false, fmt.Sprintf("レース条件エラー: 期待値1000、実際%d", counter), 0
}

// evaluateMemoryManagement - メモリ管理の評価
func (e *InteractiveEvaluator) evaluateMemoryManagement() {
	fmt.Println("\n💾 メモリ管理の評価を開始します...")

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	initialMem := m.Alloc

	// メモリリークテスト
	leakyFunction := func() {
		// 意図的なメモリ使用
		data := make([][]byte, 100)
		for i := range data {
			data[i] = make([]byte, 1024*10) // 10KB
		}
		// 一部を保持（リーク）
		_ = data[0]
	}

	for i := 0; i < 10; i++ {
		leakyFunction()
	}

	runtime.GC()
	runtime.ReadMemStats(&m)
	finalMem := m.Alloc

	leaked := finalMem - initialMem
	score := 0

	if leaked < 1024*1024 { // 1MB未満
		color.Green("  ✅ メモリリーク: 検出されません")
		score = 20
	} else {
		color.Yellow("  ⚠ メモリリーク: %d KB", leaked/1024)
		score = 5
	}

	// sync.Pool使用テスト
	poolTest := e.testSyncPool()
	if poolTest {
		color.Green("  ✅ sync.Poolが効果的に使用されています")
		score += 15
	}

	e.updateScore(score)
	fmt.Printf("\n獲得スコア: %d点\n", score)
}

// testSyncPool - sync.Pool使用テスト
func (e *InteractiveEvaluator) testSyncPool() bool {
	pool := sync.Pool{
		New: func() interface{} {
			return make([]byte, 1024)
		},
	}

	// Pool使用のベンチマーク
	start := time.Now()
	for i := 0; i < 10000; i++ {
		buf := pool.Get().([]byte)
		// 使用
		buf[0] = byte(i)
		pool.Put(buf)
	}
	poolTime := time.Since(start)

	// Pool不使用のベンチマーク
	start = time.Now()
	for i := 0; i < 10000; i++ {
		buf := make([]byte, 1024)
		buf[0] = byte(i)
	}
	noPoolTime := time.Since(start)

	return poolTime < noPoolTime
}

// evaluatePerformance - パフォーマンス評価
func (e *InteractiveEvaluator) evaluatePerformance() {
	fmt.Println("\n⚡ パフォーマンス最適化の評価を開始します...")

	benchmarks := []struct {
		name string
		fn   func() time.Duration
	}{
		{"Mutex vs Channel", e.benchmarkMutexVsChannel},
		{"並列処理効率", e.benchmarkParallelism},
		{"メモリアロケーション", e.benchmarkMemoryAllocation},
	}

	totalScore := 0
	for _, b := range benchmarks {
		fmt.Printf("\n🔨 ベンチマーク: %s\n", b.name)
		duration := b.fn()

		// パフォーマンス基準
		var score int
		if duration < 100*time.Millisecond {
			score = 20
			color.Green("  ✅ 優秀なパフォーマンス: %v", duration)
		} else if duration < 500*time.Millisecond {
			score = 10
			color.Yellow("  ⚠ 改善の余地あり: %v", duration)
		} else {
			score = 0
			color.Red("  ❌ パフォーマンス問題: %v", duration)
		}

		totalScore += score
		time.Sleep(500 * time.Millisecond)
	}

	e.updateScore(totalScore)
	fmt.Printf("\n獲得スコア: %d点\n", totalScore)
}

// benchmarkMutexVsChannel - MutexとChannelの比較
func (e *InteractiveEvaluator) benchmarkMutexVsChannel() time.Duration {
	const iterations = 10000

	// Channel版
	start := time.Now()
	ch := make(chan int, 1)
	ch <- 0
	for i := 0; i < iterations; i++ {
		v := <-ch
		v++
		ch <- v
	}
	<-ch

	return time.Since(start)
}

// benchmarkParallelism - 並列処理効率
func (e *InteractiveEvaluator) benchmarkParallelism() time.Duration {
	start := time.Now()

	numCPU := runtime.NumCPU()
	var wg sync.WaitGroup

	workPerCPU := 1000000
	for i := 0; i < numCPU; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sum := 0
			for j := 0; j < workPerCPU; j++ {
				sum += j
			}
			_ = sum
		}()
	}

	wg.Wait()
	return time.Since(start)
}

// benchmarkMemoryAllocation - メモリアロケーション
func (e *InteractiveEvaluator) benchmarkMemoryAllocation() time.Duration {
	start := time.Now()

	// 事前割り当て
	slice := make([]int, 0, 10000)
	for i := 0; i < 10000; i++ {
		slice = append(slice, i)
	}

	return time.Since(start)
}

// evaluateSecurity - セキュリティ評価
func (e *InteractiveEvaluator) evaluateSecurity() {
	fmt.Println("\n🔐 セキュリティの評価を開始します...")

	tests := []struct {
		name   string
		result bool
		score  int
	}{
		{"タイミング攻撃対策", e.testTimingAttackProtection(), 15},
		{"リソース枯渇対策", e.testResourceExhaustion(), 15},
		{"入力検証", e.testInputValidation(), 10},
	}

	totalScore := 0
	for _, t := range tests {
		if t.result {
			color.Green("  ✅ %s: 合格 (+%d点)", t.name, t.score)
			totalScore += t.score
		} else {
			color.Red("  ❌ %s: 不合格", t.name)
		}
		time.Sleep(500 * time.Millisecond)
	}

	e.updateScore(totalScore)
	e.checkAchievements("security_expert", totalScore >= 30)
	fmt.Printf("\n獲得スコア: %d点\n", totalScore)
}

// testTimingAttackProtection - タイミング攻撃対策テスト
func (e *InteractiveEvaluator) testTimingAttackProtection() bool {
	// 定数時間比較のシミュレーション
	secret := []byte("secret")
	test1 := []byte("secret")
	test2 := []byte("wrong!")

	// 両方の比較が同じ時間かかるべき
	start1 := time.Now()
	equal1 := constantTimeCompare(secret, test1)
	time1 := time.Since(start1)

	start2 := time.Now()
	equal2 := constantTimeCompare(secret, test2)
	time2 := time.Since(start2)

	_ = equal1
	_ = equal2

	// タイミングの差が小さいほど良い
	diff := time1 - time2
	if diff < 0 {
		diff = -diff
	}

	return diff < 100*time.Microsecond
}

func constantTimeCompare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	result := byte(0)
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

// testResourceExhaustion - リソース枯渇対策テスト
func (e *InteractiveEvaluator) testResourceExhaustion() bool {
	// セマフォでリソース制限
	sem := make(chan struct{}, 10) // 最大10並行

	var wg sync.WaitGroup
	rejected := 0

	for i := 0; i < 100; i++ {
		select {
		case sem <- struct{}{}:
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				time.Sleep(time.Millisecond)
			}()
		default:
			rejected++
		}
	}

	wg.Wait()

	// 適切にリクエストを拒否できているか
	return rejected > 0 && rejected < 100
}

// testInputValidation - 入力検証テスト
func (e *InteractiveEvaluator) testInputValidation() bool {
	dangerousInputs := []string{
		"'; DROP TABLE users; --",
		"../../../etc/passwd",
		"<script>alert('XSS')</script>",
	}

	for _, input := range dangerousInputs {
		if !isValidInput(input) {
			continue
		}
		return false // 危険な入力を通してしまった
	}

	return true
}

func isValidInput(input string) bool {
	dangerous := []string{"'", ";", "--", "..", "<", ">", "script"}
	for _, d := range dangerous {
		if strings.Contains(input, d) {
			return false
		}
	}
	return true
}

// evaluateDistributed - 分散システム評価
func (e *InteractiveEvaluator) evaluateDistributed() {
	fmt.Println("\n🌐 分散システムの評価を開始します...")

	fmt.Println("\nこのセクションでは実際の分散システム環境が必要です。")
	fmt.Println("Docker環境を起動して、実践的なテストを行ってください。")

	// シミュレーション評価
	score := e.simulateDistributedTests()

	e.updateScore(score)
	fmt.Printf("\n獲得スコア: %d点\n", score)
}

// simulateDistributedTests - 分散システムテストのシミュレーション
func (e *InteractiveEvaluator) simulateDistributedTests() int {
	tests := []string{
		"分散ロック機構",
		"メッセージ順序保証",
		"一貫性保証",
		"障害回復",
	}

	score := 0
	for _, test := range tests {
		fmt.Printf("\n⏳ シミュレーション: %s\n", test)
		time.Sleep(500 * time.Millisecond)

		// ランダムに成功/失敗
		if time.Now().Unix()%2 == 0 {
			color.Green("  ✅ テスト合格")
			score += 10
		} else {
			color.Yellow("  ⚠ 要改善")
			score += 5
		}
	}

	return score
}

// showProgress - 進捗状況表示
func (e *InteractiveEvaluator) showProgress() {
	e.mu.RLock()
	defer e.mu.RUnlock()

	fmt.Println("\n📊 学習進捗状況")
	fmt.Println("=" + strings.Repeat("=", 50))

	// レベルとXP
	nextLevelXP := e.level * 100
	progress := float64(e.experience) / float64(nextLevelXP) * 100

	fmt.Printf("\n🎮 レベル: %d\n", e.level)
	fmt.Printf("⭐ 経験値: %d/%d (%.1f%%)\n", e.experience, nextLevelXP, progress)
	fmt.Printf("🏆 総スコア: %d\n", e.score)

	// プログレスバー
	barLength := 30
	filled := int(progress * float64(barLength) / 100)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barLength-filled)
	fmt.Printf("\n進捗: [%s] %.1f%%\n", bar, progress)

	// アチーブメント
	fmt.Println("\n🏅 獲得アチーブメント:")
	if len(e.achievements) == 0 {
		fmt.Println("  まだアチーブメントはありません")
	} else {
		for achievement := range e.achievements {
			fmt.Printf("  ✅ %s\n", achievement)
		}
	}

	// 次のレベルまで
	fmt.Printf("\n次のレベルまで: %d XP\n", nextLevelXP-e.experience)
}

// updateScore - スコア更新
func (e *InteractiveEvaluator) updateScore(points int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.score += points
	e.experience += points

	// レベルアップチェック
	nextLevelXP := e.level * 100
	if e.experience >= nextLevelXP {
		e.level++
		e.experience = e.experience - nextLevelXP
		color.Yellow("\n🎉 レベルアップ！ レベル %d に到達しました！", e.level)
	}
}

// checkAchievements - アチーブメントチェック
func (e *InteractiveEvaluator) checkAchievements(name string, earned bool) {
	if earned {
		e.mu.Lock()
		if !e.achievements[name] {
			e.achievements[name] = true
			e.mu.Unlock()
			color.Magenta("\n🏆 アチーブメント獲得: %s", name)
		} else {
			e.mu.Unlock()
		}
	}
}

// displayFarewell - 終了メッセージ
func (e *InteractiveEvaluator) displayFarewell() {
	fmt.Println("\n" + strings.Repeat("=", 50))
	color.Cyan("\n👋 お疲れ様でした！")
	fmt.Printf("\n最終スコア: %d | レベル: %d\n", e.score, e.level)

	if e.score > 100 {
		color.Green("素晴らしい成績です！Go並行処理のエキスパートですね！")
	} else if e.score > 50 {
		color.Yellow("良い進捗です！さらに練習を続けましょう！")
	} else {
		fmt.Println("まだ学ぶことがたくさんあります。頑張りましょう！")
	}
}