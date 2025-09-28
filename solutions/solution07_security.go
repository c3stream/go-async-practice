package solutions

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"
)

// Solution07_FixedSecurity - セキュリティ問題の解決版
func Solution07_FixedSecurity() {
	fmt.Println("\n🔧 解答7: セキュリティ問題の修正")
	fmt.Println("=" + repeatString("=", 50))

	// 解法1: タイミング攻撃への対策
	solution1TimingAttackProtection()

	// 解法2: レース条件を利用した攻撃への対策
	solution2RaceConditionSecurity()

	// 解法3: リソース枯渇攻撃への対策
	solution3ResourceExhaustionProtection()

	// 解法4: 並行処理でのセキュアな乱数生成
	solution4SecureRandomGeneration()
}

// 解法1: タイミング攻撃への対策
func solution1TimingAttackProtection() {
	fmt.Println("\n📌 解法1: タイミング攻撃への対策")

	// 脆弱な実装（タイミング攻撃可能）
	vulnerableVerify := func(input, secret string) bool {
		if len(input) != len(secret) {
			return false
		}
		for i := 0; i < len(input); i++ {
			if input[i] != secret[i] {
				return false // 早期リターン = タイミング情報の漏洩
			}
		}
		return true
	}

	// セキュアな実装（定数時間比較）
	secureVerify := func(input, secret string) bool {
		inputHash := sha256.Sum256([]byte(input))
		secretHash := sha256.Sum256([]byte(secret))

		// crypto/subtle を使った定数時間比較
		return subtle.ConstantTimeCompare(inputHash[:], secretHash[:]) == 1
	}

	// より高度な定数時間検証
	advancedSecureVerify := func(input, secret []byte) bool {
		// 長さが異なる場合でも定数時間で比較
		inputHash := sha256.Sum256(input)
		secretHash := sha256.Sum256(secret)

		// HMAC を使った検証も考慮
		result := subtle.ConstantTimeCompare(inputHash[:], secretHash[:])

		// ダミー処理で実行時間を均一化
		dummyWork := 0
		for i := 0; i < 100; i++ {
			dummyWork += i
		}
		_ = dummyWork

		return result == 1
	}

	// テスト実行
	secret := "supersecret123"
	testInputs := []string{
		"wrongpassword",
		"supersecret123",
		"supersecret124",
	}

	fmt.Println("  脆弱な実装のテスト:")
	for _, input := range testInputs {
		start := time.Now()
		result := vulnerableVerify(input, secret)
		elapsed := time.Since(start)
		fmt.Printf("    入力: %s, 結果: %v, 時間: %v\n",
			maskString(input), result, elapsed)
	}

	fmt.Println("\n  セキュアな実装のテスト:")
	for _, input := range testInputs {
		start := time.Now()
		result := secureVerify(input, secret)
		elapsed := time.Since(start)
		fmt.Printf("    入力: %s, 結果: %v, 時間: %v（一定）\n",
			maskString(input), result, elapsed)
	}

	// 並行アクセスでのセキュリティ
	fmt.Println("\n  並行アクセスでのセキュア検証:")
	var successCount int32
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			testInput := fmt.Sprintf("test%d", id)
			if id == 5 {
				testInput = secret
			}

			if advancedSecureVerify([]byte(testInput), []byte(secret)) {
				atomic.AddInt32(&successCount, 1)
				fmt.Printf("    ✓ ゴルーチン %d: 認証成功\n", id)
			}
		}(i)
	}

	wg.Wait()
	fmt.Printf("  認証成功数: %d/10\n", successCount)
}

// 解法2: レース条件を利用した攻撃への対策
func solution2RaceConditionSecurity() {
	fmt.Println("\n📌 解法2: レース条件によるセキュリティ脆弱性の対策")

	// セキュアなカウンター（atomic使用）
	type SecureCounter struct {
		value int64
		limit int64
	}

	counter := &SecureCounter{
		limit: 100,
	}

	// アトミック操作で安全にインクリメント
	counterIncrement := func() (bool, int64) {
		newVal := atomic.AddInt64(&counter.value, 1)
		if newVal > counter.limit {
			// ロールバック
			atomic.AddInt64(&counter.value, -1)
			return false, counter.limit
		}
		return true, newVal
	}

	// セキュアな権限チェック
	type SecureAccessControl struct {
		mu          sync.RWMutex
		permissions map[string]map[string]bool
	}

	ac := &SecureAccessControl{
		permissions: make(map[string]map[string]bool),
	}

	// 権限の設定（スレッドセーフ）
	acSetPermission := func(user, resource string, allowed bool) {
		ac.mu.Lock()
		defer ac.mu.Unlock()

		if ac.permissions[user] == nil {
			ac.permissions[user] = make(map[string]bool)
		}
		ac.permissions[user][resource] = allowed
		fmt.Printf("  ✓ 権限設定: user=%s, resource=%s, allowed=%v\n",
			user, resource, allowed)
	}

	// 権限のチェック（スレッドセーフ）
	acCheckPermission := func(user, resource string) bool {
		ac.mu.RLock()
		defer ac.mu.RUnlock()

		if userPerms, exists := ac.permissions[user]; exists {
			return userPerms[resource]
		}
		return false
	}

	// トークンバケットによるレート制限
	type SecureRateLimiter struct {
		tokens    int64
		maxTokens int64
		refillMu  sync.Mutex
		lastRefill time.Time
	}

	limiter := &SecureRateLimiter{
		tokens:     10,
		maxTokens:  10,
		lastRefill: time.Now(),
	}

	limiterTryAcquire := func() bool {
		// アトミックにトークンを取得
		if atomic.LoadInt64(&limiter.tokens) <= 0 {
			// リフィルチェック
			limiter.refillMu.Lock()
			now := time.Now()
			if now.Sub(limiter.lastRefill) > time.Second {
				atomic.StoreInt64(&limiter.tokens, limiter.maxTokens)
				limiter.lastRefill = now
			}
			limiter.refillMu.Unlock()
		}

		// トークンを消費
		remaining := atomic.AddInt64(&limiter.tokens, -1)
		if remaining < 0 {
			// ロールバック
			atomic.AddInt64(&limiter.tokens, 1)
			return false
		}
		return true
	}

	// テスト：並行アクセスでの安全性確認
	fmt.Println("\n  並行アクセステスト:")

	// カウンターテスト
	var wg sync.WaitGroup
	for i := 0; i < 150; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if ok, val := counterIncrement(); ok {
				if id%20 == 0 {
					fmt.Printf("    カウンター: %d/100\n", val)
				}
			}
		}(i)
	}
	wg.Wait()
	fmt.Printf("  最終カウンター値: %d（制限: %d）\n",
		atomic.LoadInt64(&counter.value), counter.limit)

	// 権限管理テスト
	acSetPermission("alice", "read", true)
	acSetPermission("alice", "write", false)
	acSetPermission("bob", "read", true)
	acSetPermission("bob", "write", true)

	users := []string{"alice", "bob", "charlie"}
	resources := []string{"read", "write"}

	for _, user := range users {
		for _, resource := range resources {
			if acCheckPermission(user, resource) {
				fmt.Printf("  ✓ %s は %s 権限を持っています\n", user, resource)
			}
		}
	}

	// レート制限テスト
	fmt.Println("\n  レート制限テスト:")
	successfulRequests := 0
	for i := 0; i < 20; i++ {
		if limiterTryAcquire() {
			successfulRequests++
			fmt.Printf("    リクエスト %d: 成功\n", i+1)
		} else {
			fmt.Printf("    リクエスト %d: レート制限\n", i+1)
		}
		if i == 10 {
			time.Sleep(1100 * time.Millisecond) // リフィルを待つ
		}
	}
	fmt.Printf("  成功したリクエスト: %d/20\n", successfulRequests)
}

// SecureCounter メソッド
type SecureCounter struct {
	value     int64
	limit     int64
	Increment func() (bool, int64)
}

// SecureAccessControl メソッド
type SecureAccessControl struct {
	mu             sync.RWMutex
	permissions    map[string]map[string]bool
	SetPermission  func(string, string, bool)
	CheckPermission func(string, string) bool
}

// SecureRateLimiter メソッド
type SecureRateLimiter struct {
	tokens     int64
	maxTokens  int64
	refillMu   sync.Mutex
	lastRefill time.Time
	TryAcquire func() bool
}

// 解法3: リソース枯渇攻撃への対策
func solution3ResourceExhaustionProtection() {
	fmt.Println("\n📌 解法3: リソース枯渇攻撃（DoS）への対策")

	// リソース制限付きワーカープール
	type SecureWorkerPool struct {
		workers   chan struct{}
		tasks     chan func()
		mu        sync.Mutex
		taskCount int64
		maxTasks  int64
	}

	pool := &SecureWorkerPool{
		workers:  make(chan struct{}, 5),  // 最大5ワーカー
		tasks:    make(chan func(), 100),  // タスクキューの上限
		maxTasks: 1000,                    // 総タスク数の上限
	}

	// ワーカー起動
	for i := 0; i < cap(pool.workers); i++ {
		go func(id int) {
			for task := range pool.tasks {
				pool.workers <- struct{}{}        // ワーカー取得
				task()
				<-pool.workers                    // ワーカー解放
				atomic.AddInt64(&pool.taskCount, -1)
			}
		}(i)
	}

	// タスク送信（制限付き）
	poolSubmit := func(task func()) error {
		// タスク数チェック
		if atomic.LoadInt64(&pool.taskCount) >= pool.maxTasks {
			return fmt.Errorf("タスク数が上限に達しました")
		}

		// ノンブロッキング送信
		select {
		case pool.tasks <- task:
			atomic.AddInt64(&pool.taskCount, 1)
			return nil
		case <-time.After(100 * time.Millisecond):
			return fmt.Errorf("タスクキューがフルです")
		}
	}

	// メモリ使用量の監視
	type MemoryGuard struct {
		maxAllocation int64
		current       int64
		mu            sync.Mutex
	}

	memGuard := &MemoryGuard{
		maxAllocation: 100 * 1024 * 1024, // 100MB
	}

	memGuardAllocate := func(size int64) ([]byte, error) {
		memGuard.mu.Lock()
		defer memGuard.mu.Unlock()

		if memGuard.current+size > memGuard.maxAllocation {
			return nil, fmt.Errorf("メモリ割り当て上限超過")
		}

		memGuard.current += size
		return make([]byte, size), nil
	}

	memGuardFree := func(size int64) {
		memGuard.mu.Lock()
		defer memGuard.mu.Unlock()
		memGuard.current -= size
	}

	// タイムアウト付き処理
	executeWithTimeout := func(f func(), timeout time.Duration) error {
		done := make(chan struct{})
		go func() {
			f()
			close(done)
		}()

		select {
		case <-done:
			return nil
		case <-time.After(timeout):
			return fmt.Errorf("処理がタイムアウトしました")
		}
	}

	// テスト実行
	fmt.Println("\n  リソース制限テスト:")

	// 大量タスクの送信試行
	rejectedTasks := 0
	for i := 0; i < 20; i++ {
		taskID := i
		err := poolSubmit(func() {
			time.Sleep(10 * time.Millisecond)
			if taskID%5 == 0 {
				fmt.Printf("    タスク %d 完了\n", taskID)
			}
		})
		if err != nil {
			rejectedTasks++
		}
	}
	fmt.Printf("  拒否されたタスク: %d/20\n", rejectedTasks)

	// メモリ割り当てテスト
	fmt.Println("\n  メモリ保護テスト:")
	allocations := []int64{
		10 * 1024 * 1024,  // 10MB
		20 * 1024 * 1024,  // 20MB
		50 * 1024 * 1024,  // 50MB
		30 * 1024 * 1024,  // 30MB（超過）
	}

	for i, size := range allocations {
		if data, err := memGuardAllocate(size); err != nil {
			fmt.Printf("    割り当て %d (%dMB): ❌ %v\n", i+1, size/1024/1024, err)
		} else {
			fmt.Printf("    割り当て %d (%dMB): ✓ 成功\n", i+1, size/1024/1024)
			_ = data
			// 実際のアプリケーションでは適切に解放
			defer memGuardFree(size)
		}
	}

	// タイムアウト保護テスト
	fmt.Println("\n  タイムアウト保護テスト:")
	tasks := []struct {
		name     string
		duration time.Duration
		timeout  time.Duration
	}{
		{"高速タスク", 50 * time.Millisecond, 100 * time.Millisecond},
		{"低速タスク", 150 * time.Millisecond, 100 * time.Millisecond},
	}

	for _, task := range tasks {
		err := executeWithTimeout(func() {
			time.Sleep(task.duration)
		}, task.timeout)

		if err != nil {
			fmt.Printf("    %s: ❌ %v\n", task.name, err)
		} else {
			fmt.Printf("    %s: ✓ 完了\n", task.name)
		}
	}
}

// SecureWorkerPool メソッド
type SecureWorkerPool struct {
	workers   chan struct{}
	tasks     chan func()
	mu        sync.Mutex
	taskCount int64
	maxTasks  int64
	Submit    func(func()) error
}

// MemoryGuard メソッド
type MemoryGuard struct {
	maxAllocation int64
	current       int64
	mu            sync.Mutex
	Allocate      func(int64) ([]byte, error)
	Free          func(int64)
}

// 解法4: セキュアな乱数生成
func solution4SecureRandomGeneration() {
	fmt.Println("\n📌 解法4: 並行処理でのセキュアな乱数生成")

	// 暗号学的に安全な乱数生成
	generateSecureToken := func(length int) (string, error) {
		bytes := make([]byte, length)
		if _, err := rand.Read(bytes); err != nil {
			return "", err
		}
		return hex.EncodeToString(bytes), nil
	}

	// セキュアな乱数整数生成
	generateSecureInt := func(max int64) (int64, error) {
		n, err := rand.Int(rand.Reader, big.NewInt(max))
		if err != nil {
			return 0, err
		}
		return n.Int64(), nil
	}

	// セッショントークン管理
	type SecureSessionManager struct {
		mu       sync.RWMutex
		sessions map[string]time.Time
		ttl      time.Duration
	}

	sessionMgr := &SecureSessionManager{
		sessions: make(map[string]time.Time),
		ttl:      5 * time.Minute,
	}

	sessionMgrCreateSession := func() (string, error) {
		token, err := generateSecureToken(32)
		if err != nil {
			return "", err
		}

		sessionMgr.mu.Lock()
		sessionMgr.sessions[token] = time.Now()
		sessionMgr.mu.Unlock()

		return token, nil
	}

	sessionMgrValidateSession := func(token string) bool {
		sessionMgr.mu.RLock()
		createdAt, exists := sessionMgr.sessions[token]
		sessionMgr.mu.RUnlock()

		if !exists {
			return false
		}

		// TTLチェック
		if time.Since(createdAt) > sessionMgr.ttl {
			// 期限切れセッションを削除
			sessionMgr.mu.Lock()
			delete(sessionMgr.sessions, token)
			sessionMgr.mu.Unlock()
			return false
		}

		return true
	}

	// ノンス生成器（リプレイ攻撃対策）
	type NonceManager struct {
		mu     sync.RWMutex
		nonces map[string]time.Time
		ttl    time.Duration
	}

	nonceMgr := &NonceManager{
		nonces: make(map[string]time.Time),
		ttl:    1 * time.Minute,
	}

	nonceMgrGenerateNonce := func() (string, error) {
		nonce, err := generateSecureToken(16)
		if err != nil {
			return "", err
		}

		nonceMgr.mu.Lock()
		nonceMgr.nonces[nonce] = time.Now()
		nonceMgr.mu.Unlock()

		return nonce, nil
	}

	nonceMgrValidateNonce := func(nonce string) bool {
		nonceMgr.mu.Lock()
		defer nonceMgr.mu.Unlock()

		usedAt, exists := nonceMgr.nonces[nonce]
		if !exists {
			return false
		}

		// 使用済みノンスを削除（一度だけ使用可能）
		delete(nonceMgr.nonces, nonce)

		// TTLチェック
		return time.Since(usedAt) <= nonceMgr.ttl
	}

	// テスト実行
	fmt.Println("\n  セキュアトークン生成テスト:")
	var wg sync.WaitGroup
	tokens := make([]string, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if token, err := generateSecureToken(16); err == nil {
				tokens[id] = token
				fmt.Printf("    トークン %d: %s...\n", id, token[:8])
			}
		}(i)
	}
	wg.Wait()

	// トークンの一意性チェック
	uniqueTokens := make(map[string]bool)
	for _, token := range tokens {
		uniqueTokens[token] = true
	}
	fmt.Printf("  生成されたトークンの一意性: %d/%d\n", len(uniqueTokens), len(tokens))

	// セッション管理テスト
	fmt.Println("\n  セッション管理テスト:")
	sessions := make([]string, 5)
	for i := 0; i < 5; i++ {
		if session, err := sessionMgrCreateSession(); err == nil {
			sessions[i] = session
			fmt.Printf("    セッション %d 作成\n", i+1)
		}
	}

	// 検証
	for i, session := range sessions {
		if sessionMgrValidateSession(session) {
			fmt.Printf("    セッション %d: ✓ 有効\n", i+1)
		}
	}

	// ノンス検証テスト
	fmt.Println("\n  ノンス（リプレイ攻撃対策）テスト:")
	nonce, _ := nonceMgrGenerateNonce()
	fmt.Printf("    ノンス生成: %s\n", nonce[:8]+"...")

	// 初回検証
	if nonceMgrValidateNonce(nonce) {
		fmt.Println("    初回検証: ✓ 成功")
	}

	// リプレイ試行
	if !nonceMgrValidateNonce(nonce) {
		fmt.Println("    リプレイ検証: ✓ 正しく拒否")
	}

	// 乱数整数生成
	fmt.Println("\n  セキュア乱数整数生成:")
	for i := 0; i < 5; i++ {
		if n, err := generateSecureInt(100); err == nil {
			fmt.Printf("    乱数 %d: %d\n", i+1, n)
		}
	}
}

// SecureSessionManager メソッド
type SecureSessionManager struct {
	mu              sync.RWMutex
	sessions        map[string]time.Time
	ttl             time.Duration
	CreateSession   func() (string, error)
	ValidateSession func(string) bool
}

// NonceManager メソッド
type NonceManager struct {
	mu            sync.RWMutex
	nonces        map[string]time.Time
	ttl           time.Duration
	GenerateNonce func() (string, error)
	ValidateNonce func(string) bool
}

// Solution07_SecurityBestPractices - セキュリティのベストプラクティス
func Solution07_SecurityBestPractices() {
	fmt.Println("\n🔒 並行処理でのセキュリティベストプラクティス")
	fmt.Println("=" + repeatString("=", 50))

	practices := []struct {
		category string
		items    []string
	}{
		{
			"タイミング攻撃対策",
			[]string{
				"crypto/subtle を使った定数時間比較",
				"ハッシュ化してから比較",
				"ダミー処理で実行時間を均一化",
			},
		},
		{
			"レース条件対策",
			[]string{
				"atomic パッケージの使用",
				"sync.Mutex/RWMutex での適切なロック",
				"チャネルを使った同期",
			},
		},
		{
			"リソース枯渇対策",
			[]string{
				"レート制限の実装",
				"タイムアウトの設定",
				"リソースプールの上限設定",
			},
		},
		{
			"暗号化とランダム性",
			[]string{
				"crypto/rand による安全な乱数生成",
				"使い捨てノンスの使用",
				"適切なトークン長の確保",
			},
		},
	}

	for _, p := range practices {
		fmt.Printf("\n  📍 %s:\n", p.category)
		for _, item := range p.items {
			fmt.Printf("    ✓ %s\n", item)
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// ヘルパー関数
func maskString(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:2] + "****" + s[len(s)-2:]
}