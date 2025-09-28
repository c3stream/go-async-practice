package solutions

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge09_DistributedLockSolution - 分散ロック問題の解決
func Challenge09_DistributedLockSolution() {
	fmt.Println("\n✅ チャレンジ9: 分散ロック問題の解決")
	fmt.Println("=" + repeatString("=", 50))

	// 解決策1: TTL付き分散ロック（フェンストークン対応）
	solution1_TTLWithFencingToken()

	// 解決策2: Redlockアルゴリズム風実装
	solution2_RedlockAlgorithm()

	// 解決策3: リースベースロック
	solution3_LeaseBasedLock()
}

// 解決策1: TTL付き分散ロック（フェンストークン対応）
func solution1_TTLWithFencingToken() {
	fmt.Println("\n📝 解決策1: TTL付き分散ロック（フェンストークン対応）")

	type DistributedLock struct {
		mu           sync.RWMutex
		locks        map[string]*LockInfo
		fencingToken atomic.Uint64
		cleanupStop  chan struct{}
	}

	type LockInfo struct {
		holder       string
		timestamp    time.Time
		ttl          time.Duration
		fencingToken uint64
		version      int64 // 楽観的ロック用
	}

	dl := &DistributedLock{
		locks:       make(map[string]*LockInfo),
		cleanupStop: make(chan struct{}),
	}

	// バックグラウンドでTTL切れのロックをクリーンアップ
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				dl.mu.Lock()
				now := time.Now()
				for resource, lock := range dl.locks {
					if now.Sub(lock.timestamp) > lock.ttl {
						delete(dl.locks, resource)
						fmt.Printf("  🔓 TTL期限切れでロック解放: %s (holder: %s)\n",
							resource, lock.holder)
					}
				}
				dl.mu.Unlock()
			case <-dl.cleanupStop:
				return
			}
		}
	}()

	// ロック取得（TTLとフェンストークン付き）
	acquire := func(ctx context.Context, nodeID, resource string, ttl time.Duration) (uint64, bool) {
		dl.mu.Lock()
		defer dl.mu.Unlock()

		// 既存ロックの確認
		if lock, exists := dl.locks[resource]; exists {
			// TTLチェック
			if time.Since(lock.timestamp) < lock.ttl {
				if lock.holder != nodeID {
					return 0, false // 他のノードが保持
				}
				// 同じノードによる再取得（リエントラント）
				lock.timestamp = time.Now()
				lock.ttl = ttl
				return lock.fencingToken, true
			}
			// TTL切れ
			fmt.Printf("  ⏱️ TTL切れを検出: %s\n", resource)
		}

		// 新規ロック取得
		token := dl.fencingToken.Add(1)
		dl.locks[resource] = &LockInfo{
			holder:       nodeID,
			timestamp:    time.Now(),
			ttl:          ttl,
			fencingToken: token,
			version:      1,
		}
		return token, true
	}

	// ロック解放（所有者確認付き）
	release := func(nodeID, resource string, token uint64) bool {
		dl.mu.Lock()
		defer dl.mu.Unlock()

		lock, exists := dl.locks[resource]
		if !exists {
			return false
		}

		// 所有者とトークンの確認
		if lock.holder != nodeID || lock.fencingToken != token {
			fmt.Printf("  ❌ 不正な解放試行: %s (expected: %s, token: %d)\n",
				nodeID, lock.holder, lock.fencingToken)
			return false
		}

		delete(dl.locks, resource)
		return true
	}

	// ロック延長（ハートビート）
	extend := func(nodeID, resource string, token uint64, newTTL time.Duration) bool {
		dl.mu.RLock()
		defer dl.mu.RUnlock()

		lock, exists := dl.locks[resource]
		if !exists || lock.holder != nodeID || lock.fencingToken != token {
			return false
		}

		lock.timestamp = time.Now()
		lock.ttl = newTTL
		return true
	}

	// テスト実行
	ctx := context.Background()
	var wg sync.WaitGroup
	resource := "critical_resource"

	// 複数ノードからの同時アクセス
	for i := 0; i < 3; i++ {
		wg.Add(1)
		nodeID := fmt.Sprintf("node_%d", i)

		go func(node string, delay time.Duration) {
			defer wg.Done()
			time.Sleep(delay)

			// ロック取得試行
			if token, ok := acquire(ctx, node, resource, 500*time.Millisecond); ok {
				fmt.Printf("  ✓ %s: ロック取得成功 (token: %d)\n", node, token)

				// クリティカルセクション
				time.Sleep(200 * time.Millisecond)

				// ハートビートで延長
				if extend(node, resource, token, 500*time.Millisecond) {
					fmt.Printf("  ♥️ %s: ロック延長成功\n", node)
				}

				// 正常解放
				if release(node, resource, token) {
					fmt.Printf("  ✓ %s: ロック解放成功\n", node)
				}
			} else {
				fmt.Printf("  ✗ %s: ロック取得失敗\n", node)
			}
		}(nodeID, time.Duration(i)*250*time.Millisecond)
	}

	wg.Wait()
	close(dl.cleanupStop)

	fmt.Printf("\n残留ロック数: %d\n", len(dl.locks))
}

// 解決策2: Redlockアルゴリズム風実装
func solution2_RedlockAlgorithm() {
	fmt.Println("\n📝 解決策2: Redlockアルゴリズム風実装")

	type RedlockNode struct {
		id    string
		mu    sync.RWMutex
		locks map[string]*LockInfo
	}

	type LockInfo struct {
		holder    string
		timestamp time.Time
		ttl       time.Duration
		value     string // ユニークな値
	}

	type Redlock struct {
		nodes   []*RedlockNode
		quorum  int
		retries int
		drift   time.Duration
	}

	// 5つのRedisノードを模擬
	nodeCount := 5
	nodes := make([]*RedlockNode, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes[i] = &RedlockNode{
			id:    fmt.Sprintf("redis_%d", i),
			locks: make(map[string]*LockInfo),
		}
	}

	redlock := &Redlock{
		nodes:   nodes,
		quorum:  (nodeCount / 2) + 1, // 過半数
		retries: 3,
		drift:   10 * time.Millisecond,
	}

	// 単一ノードでのロック取得
	lockSingleNode := func(node *RedlockNode, resource, value string, ttl time.Duration) bool {
		node.mu.Lock()
		defer node.mu.Unlock()

		if lock, exists := node.locks[resource]; exists {
			if time.Since(lock.timestamp) < lock.ttl {
				return false // まだ有効なロックが存在
			}
		}

		node.locks[resource] = &LockInfo{
			holder:    value,
			timestamp: time.Now(),
			ttl:       ttl,
			value:     value,
		}
		return true
	}

	// 単一ノードでのロック解放
	unlockSingleNode := func(node *RedlockNode, resource, value string) bool {
		node.mu.Lock()
		defer node.mu.Unlock()

		if lock, exists := node.locks[resource]; exists && lock.value == value {
			delete(node.locks, resource)
			return true
		}
		return false
	}

	// Redlockアルゴリズムでロック取得
	acquireRedlock := func(resource, value string, ttl time.Duration) (bool, time.Duration) {
		for retry := 0; retry < redlock.retries; retry++ {
			startTime := time.Now()
			lockedNodes := 0

			// 全ノードに対してロック取得を試行
			for _, node := range redlock.nodes {
				if lockSingleNode(node, resource, value, ttl) {
					lockedNodes++
				}
			}

			// 経過時間計算
			elapsed := time.Since(startTime)
			validity := ttl - elapsed - redlock.drift

			// クォーラムを満たし、まだ有効時間が残っている場合
			if lockedNodes >= redlock.quorum && validity > 0 {
				return true, validity
			}

			// 失敗した場合は全ノードでアンロック
			for _, node := range redlock.nodes {
				unlockSingleNode(node, resource, value)
			}

			// リトライ前に少し待機
			time.Sleep(time.Duration(retry) * 10 * time.Millisecond)
		}

		return false, 0
	}

	// Redlockアルゴリズムでロック解放
	releaseRedlock := func(resource, value string) {
		for _, node := range redlock.nodes {
			unlockSingleNode(node, resource, value)
		}
	}

	// テスト実行
	var wg sync.WaitGroup
	resource := "distributed_resource"

	for i := 0; i < 3; i++ {
		wg.Add(1)
		clientID := fmt.Sprintf("client_%d", i)

		go func(client string, delay time.Duration) {
			defer wg.Done()
			time.Sleep(delay)

			value := fmt.Sprintf("%s_%d", client, time.Now().UnixNano())

			if ok, validity := acquireRedlock(resource, value, 500*time.Millisecond); ok {
				fmt.Printf("  ✓ %s: Redlock取得成功 (有効時間: %v)\n", client, validity)

				// クリティカルセクション
				time.Sleep(100 * time.Millisecond)

				// ロック解放
				releaseRedlock(resource, value)
				fmt.Printf("  ✓ %s: Redlock解放成功\n", client)
			} else {
				fmt.Printf("  ✗ %s: Redlock取得失敗\n", client)
			}
		}(clientID, time.Duration(i)*200*time.Millisecond)
	}

	wg.Wait()
}

// 解決策3: リースベースロック
func solution3_LeaseBasedLock() {
	fmt.Println("\n📝 解決策3: リースベースロック")

	type Lease struct {
		ID        string
		Holder    string
		ExpiresAt time.Time
		Version   int64
	}

	type LeaseManager struct {
		mu      sync.RWMutex
		leases  map[string]*Lease
		counter atomic.Int64
	}

	lm := &LeaseManager{
		leases: make(map[string]*Lease),
	}

	// リース取得
	acquireLease := func(resource, holder string, duration time.Duration) (*Lease, error) {
		lm.mu.Lock()
		defer lm.mu.Unlock()

		now := time.Now()

		// 既存リースの確認
		if lease, exists := lm.leases[resource]; exists {
			if now.Before(lease.ExpiresAt) {
				if lease.Holder == holder {
					// 同じホルダーによる更新
					lease.ExpiresAt = now.Add(duration)
					lease.Version++
					return lease, nil
				}
				return nil, fmt.Errorf("resource already leased by %s", lease.Holder)
			}
			// 期限切れリースの削除
			delete(lm.leases, resource)
		}

		// 新規リース作成
		lease := &Lease{
			ID:        fmt.Sprintf("lease_%d", lm.counter.Add(1)),
			Holder:    holder,
			ExpiresAt: now.Add(duration),
			Version:   1,
		}
		lm.leases[resource] = lease
		return lease, nil
	}

	// リース更新（ハートビート）
	renewLease := func(resource string, lease *Lease, duration time.Duration) error {
		lm.mu.Lock()
		defer lm.mu.Unlock()

		current, exists := lm.leases[resource]
		if !exists {
			return fmt.Errorf("lease not found")
		}

		if current.ID != lease.ID || current.Version != lease.Version {
			return fmt.Errorf("lease has been modified")
		}

		current.ExpiresAt = time.Now().Add(duration)
		current.Version++
		*lease = *current // 更新されたリース情報を返す
		return nil
	}

	// リース解放
	releaseLease := func(resource string, lease *Lease) error {
		lm.mu.Lock()
		defer lm.mu.Unlock()

		current, exists := lm.leases[resource]
		if !exists {
			return fmt.Errorf("lease not found")
		}

		if current.ID != lease.ID {
			return fmt.Errorf("invalid lease ID")
		}

		delete(lm.leases, resource)
		return nil
	}

	// バックグラウンドでの期限切れリースクリーンアップ
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				lm.mu.Lock()
				now := time.Now()
				for resource, lease := range lm.leases {
					if now.After(lease.ExpiresAt) {
						delete(lm.leases, resource)
						fmt.Printf("  🔓 期限切れリース削除: %s (holder: %s)\n",
							resource, lease.Holder)
					}
				}
				lm.mu.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()

	// テスト実行
	var wg sync.WaitGroup
	resource := "shared_resource"

	// ワーカーシミュレーション
	worker := func(id string, workTime time.Duration) {
		defer wg.Done()

		// リース取得
		lease, err := acquireLease(resource, id, 300*time.Millisecond)
		if err != nil {
			fmt.Printf("  ✗ %s: リース取得失敗: %v\n", id, err)
			return
		}

		fmt.Printf("  ✓ %s: リース取得成功 (ID: %s, expires: %v)\n",
			id, lease.ID, time.Until(lease.ExpiresAt))

		// 作業実行とハートビート
		workDone := make(chan struct{})
		go func() {
			time.Sleep(workTime)
			close(workDone)
		}()

		// ハートビートループ
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := renewLease(resource, lease, 300*time.Millisecond); err != nil {
					fmt.Printf("  ⚠️ %s: リース更新失敗: %v\n", id, err)
					return
				}
				fmt.Printf("  ♥️ %s: リース更新成功\n", id)
			case <-workDone:
				// 作業完了、リース解放
				if err := releaseLease(resource, lease); err != nil {
					fmt.Printf("  ⚠️ %s: リース解放失敗: %v\n", id, err)
				} else {
					fmt.Printf("  ✓ %s: リース解放成功\n", id)
				}
				return
			}
		}
	}

	// 3つのワーカーを順次起動
	for i := 0; i < 3; i++ {
		wg.Add(1)
		workerID := fmt.Sprintf("worker_%d", i)

		go func(id string, delay time.Duration) {
			time.Sleep(delay)
			worker(id, 250*time.Millisecond)
		}(workerID, time.Duration(i)*400*time.Millisecond)
	}

	wg.Wait()
	cancel()

	// 最終状態確認
	lm.mu.RLock()
	fmt.Printf("\n残留リース数: %d\n", len(lm.leases))
	lm.mu.RUnlock()
}

// repeatString - 文字列を繰り返す
func repeatString(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}