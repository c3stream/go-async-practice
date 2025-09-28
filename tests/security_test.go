package tests

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestTimingAttackResistance - タイミング攻撃への耐性テスト
func TestTimingAttackResistance(t *testing.T) {
	secret := []byte("supersecretpassword123")

	// 脆弱な比較関数
	vulnerableCompare := func(input, target []byte) bool {
		if len(input) != len(target) {
			return false
		}
		for i := 0; i < len(input); i++ {
			if input[i] != target[i] {
				return false // 早期リターンでタイミング情報漏洩
			}
			time.Sleep(time.Microsecond) // 処理時間をシミュレート
		}
		return true
	}

	// セキュアな比較関数
	secureCompare := func(input, target []byte) bool {
		if len(input) != len(target) {
			return false
		}
		// crypto/subtle による定数時間比較
		return subtle.ConstantTimeCompare(input, target) == 1
	}

	testCases := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{"完全一致", secret, true},
		{"最初の文字が違う", []byte("xupersecretpassword123"), false},
		{"最後の文字が違う", []byte("supersecretpassword124"), false},
		{"長さが違う", []byte("short"), false},
	}

	// 脆弱な実装のタイミング測定
	t.Run("VulnerableImplementation", func(t *testing.T) {
		var timings []time.Duration
		for _, tc := range testCases {
			start := time.Now()
			_ = vulnerableCompare(tc.input, secret)
			elapsed := time.Since(start)
			timings = append(timings, elapsed)
			t.Logf("%s: %v", tc.name, elapsed)
		}

		// タイミングの差が検出可能
		if timings[0] < timings[1]*2 {
			t.Log("警告: タイミング差が検出可能")
		}
	})

	// セキュアな実装のタイミング測定
	t.Run("SecureImplementation", func(t *testing.T) {
		var timings []time.Duration
		for _, tc := range testCases {
			start := time.Now()
			result := secureCompare(tc.input, secret)
			elapsed := time.Since(start)
			timings = append(timings, elapsed)

			if result != tc.expected {
				t.Errorf("%s: 期待値=%v, 実際=%v", tc.name, tc.expected, result)
			}
			t.Logf("%s: %v (一定時間)", tc.name, elapsed)
		}
	})
}

// TestRaceConditionVulnerability - レース条件の脆弱性テスト
func TestRaceConditionVulnerability(t *testing.T) {
	// 脆弱なカウンター実装
	type VulnerableCounter struct {
		value int
		mu    sync.Mutex
	}

	// TOCTOU（Time-Of-Check-Time-Of-Use）脆弱性
	vc := &VulnerableCounter{}

	checkAndIncrement := func() bool {
		// チェックとアップデートが分離（脆弱）
		if vc.value < 100 {
			// ここでレース条件発生の可能性
			time.Sleep(time.Microsecond)
			vc.mu.Lock()
			vc.value++
			vc.mu.Unlock()
			return true
		}
		return false
	}

	// 並行アクセステスト
	var wg sync.WaitGroup
	successCount := 0
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if checkAndIncrement() {
				successCount++
			}
		}()
	}
	wg.Wait()

	if vc.value > 100 {
		t.Logf("脆弱性検出: 制限値100を超えた値=%d", vc.value)
	}

	// セキュアな実装
	type SecureCounter struct {
		value int64
		limit int64
	}

	sc := &SecureCounter{limit: 100}

	secureIncrement := func() bool {
		for {
			current := atomic.LoadInt64(&sc.value)
			if current >= sc.limit {
				return false
			}
			if atomic.CompareAndSwapInt64(&sc.value, current, current+1) {
				return true
			}
		}
	}

	// 並行アクセステスト（セキュア版）
	sc.value = 0
	successCount = 0
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if secureIncrement() {
				successCount++
			}
		}()
	}
	wg.Wait()

	if sc.value != sc.limit {
		t.Errorf("セキュアカウンター: 期待値=%d, 実際=%d", sc.limit, sc.value)
	}
}

// TestResourceExhaustionAttack - リソース枯渇攻撃テスト
func TestResourceExhaustionAttack(t *testing.T) {
	// 脆弱なサーバーシミュレーション
	type VulnerableServer struct {
		connections []chan struct{}
		mu          sync.Mutex
	}

	vs := &VulnerableServer{
		connections: make([]chan struct{}, 0),
	}

	// 無制限の接続受け入れ（脆弱）
	acceptConnection := func() {
		vs.mu.Lock()
		conn := make(chan struct{})
		vs.connections = append(vs.connections, conn)
		vs.mu.Unlock()

		// リソースを消費し続ける
		go func() {
			<-conn
		}()
	}

	// DoS攻撃シミュレーション
	initialGoroutines := runtime.NumGoroutine()
	for i := 0; i < 1000; i++ {
		acceptConnection()
	}

	currentGoroutines := runtime.NumGoroutine()
	if currentGoroutines-initialGoroutines > 900 {
		t.Logf("脆弱性: %d個のgoroutineが生成された（DoS可能）",
			currentGoroutines-initialGoroutines)
	}

	// クリーンアップ
	for _, conn := range vs.connections {
		close(conn)
	}

	// セキュアな実装（接続数制限）
	type SecureServer struct {
		semaphore chan struct{}
		active    int32
		maxConns  int32
	}

	ss := &SecureServer{
		semaphore: make(chan struct{}, 10), // 最大10接続
		maxConns:  10,
	}

	secureAccept := func() bool {
		select {
		case ss.semaphore <- struct{}{}:
			atomic.AddInt32(&ss.active, 1)
			go func() {
				defer func() {
					<-ss.semaphore
					atomic.AddInt32(&ss.active, -1)
				}()
				time.Sleep(10 * time.Millisecond)
			}()
			return true
		default:
			return false // 接続拒否
		}
	}

	// 大量接続試行
	accepted := 0
	rejected := 0
	for i := 0; i < 100; i++ {
		if secureAccept() {
			accepted++
		} else {
			rejected++
		}
	}

	t.Logf("セキュアサーバー: 受け入れ=%d, 拒否=%d", accepted, rejected)

	if atomic.LoadInt32(&ss.active) > ss.maxConns {
		t.Errorf("接続数制限超過: %d > %d", ss.active, ss.maxConns)
	}
}

// TestCryptographicRandomness - 暗号学的乱数のテスト
func TestCryptographicRandomness(t *testing.T) {
	// セキュアな乱数生成
	generateToken := func(length int) ([]byte, error) {
		token := make([]byte, length)
		_, err := rand.Read(token)
		return token, err
	}

	// ユニーク性テスト
	tokens := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		token, err := generateToken(16)
		if err != nil {
			t.Fatalf("トークン生成エラー: %v", err)
		}

		key := fmt.Sprintf("%x", token)
		if tokens[key] {
			t.Errorf("重複トークン検出: %s", key)
		}
		tokens[key] = true
	}

	t.Logf("1000個のユニークトークンを生成")

	// エントロピーテスト（簡易版）
	token, _ := generateToken(1024)
	zeros := 0
	ones := 0
	for _, b := range token {
		for i := 0; i < 8; i++ {
			if b&(1<<uint(i)) != 0 {
				ones++
			} else {
				zeros++
			}
		}
	}

	ratio := float64(ones) / float64(zeros+ones)
	if ratio < 0.45 || ratio > 0.55 {
		t.Logf("警告: ビット分布が偏っている（1の割合: %.2f%%）", ratio*100)
	} else {
		t.Logf("良好なビット分布（1の割合: %.2f%%）", ratio*100)
	}
}

// TestInputValidation - 入力検証のセキュリティテスト
func TestInputValidation(t *testing.T) {
	// SQLインジェクション対策のテスト
	testCases := []struct {
		name     string
		input    string
		isSafe   bool
	}{
		{"通常の入力", "john_doe", true},
		{"SQLインジェクション試行", "'; DROP TABLE users; --", false},
		{"特殊文字", "../../../etc/passwd", false},
		{"長すぎる入力", string(make([]byte, 10000)), false},
	}

	validateInput := func(input string) bool {
		// 長さチェック
		if len(input) > 100 {
			return false
		}

		// 危険な文字のチェック
		dangerous := []string{"'", ";", "--", "/*", "*/", "..", "\\"}
		for _, d := range dangerous {
			if strings.Contains(input, d) {
				return false
			}
		}

		return true
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := validateInput(tc.input)
			if result != tc.isSafe {
				t.Errorf("入力 '%s': 期待=%v, 実際=%v", tc.input, tc.isSafe, result)
			}
		})
	}
}

// BenchmarkSecurityMeasures - セキュリティ対策のパフォーマンステスト
func BenchmarkSecurityMeasures(b *testing.B) {
	secret := []byte("secretpassword")
	input := []byte("secretpassword")

	b.Run("VulnerableCompare", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			equal := true
			for j := 0; j < len(input); j++ {
				if input[j] != secret[j] {
					equal = false
					break
				}
			}
			_ = equal
		}
	})

	b.Run("ConstantTimeCompare", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = subtle.ConstantTimeCompare(input, secret)
		}
	})

	b.Run("AtomicCounter", func(b *testing.B) {
		var counter int64
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				atomic.AddInt64(&counter, 1)
			}
		})
	})

	b.Run("MutexCounter", func(b *testing.B) {
		var counter int
		var mu sync.Mutex
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				mu.Lock()
				counter++
				mu.Unlock()
			}
		})
	})
}