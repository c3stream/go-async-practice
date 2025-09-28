package challenges

import (
	"fmt"
	"sync"
	"time"
)

// Challenge09_DistributedLockProblem - 分散ロックの問題
// 問題: 複数のノードからのアクセスを想定した分散ロック機構に問題があります
func Challenge09_DistributedLockProblem() {
	fmt.Println("\n🔥 チャレンジ9: 分散ロックの問題")
	fmt.Println("===================================================")
	fmt.Println("問題: 分散環境でのロック機構が正しく動作しません")
	fmt.Println("症状: デッドロック、二重ロック、ロストアップデート")
	fmt.Println("\n⚠️  このコードには複数の問題があります:")

	// 問題のある分散ロック実装
	type LockInfo struct {
		holder    string
		timestamp time.Time
		// 問題1: TTLの考慮なし
	}

	type DistributedLock struct {
		mu       sync.Mutex
		locks    map[string]*LockInfo
		timeout  time.Duration
	}

	dl := &DistributedLock{
		locks:   make(map[string]*LockInfo),
		timeout: 5 * time.Second,
	}

	// 問題のあるロック取得
	acquire := func(nodeID, resource string) bool {
		dl.mu.Lock()
		defer dl.mu.Unlock()

		if lock, exists := dl.locks[resource]; exists {
			// 問題2: タイムアウトの確認なし
			if lock.holder != nodeID {
				return false
			}
		}

		// 問題3: 既存ロックの上書き
		dl.locks[resource] = &LockInfo{
			holder:    nodeID,
			timestamp: time.Now(),
		}
		return true
	}

	// 問題のあるロック解放
	release := func(nodeID, resource string) bool {
		dl.mu.Lock()
		defer dl.mu.Unlock()

		// 問題4: 所有者チェックなし
		delete(dl.locks, resource)
		return true
	}

	// 問題のあるロック延長
	_ = func(nodeID, resource string) bool {
		// 問題5: ロックを取得せずに延長
		if lock, exists := dl.locks[resource]; exists {
			lock.timestamp = time.Now()
			return true
		}
		return false
	}

	// テストシナリオ
	fmt.Println("\n実行結果:")

	var wg sync.WaitGroup
	resource := "critical_resource"

	// 複数ノードからの同時アクセス
	for i := 0; i < 5; i++ {
		wg.Add(1)
		nodeID := fmt.Sprintf("node_%d", i)

		go func(node string) {
			defer wg.Done()

			// ロック取得試行
			if acquire(node, resource) {
				fmt.Printf("  ✓ %s: ロック取得成功\n", node)

				// クリティカルセクション
				time.Sleep(100 * time.Millisecond)

				// 問題6: エラー処理なし
				release(node, resource)
				fmt.Printf("  ✓ %s: ロック解放\n", node)
			} else {
				fmt.Printf("  ✗ %s: ロック取得失敗\n", node)
			}
		}(nodeID)
	}

	wg.Wait()

	// 問題7: 残留ロックのクリーンアップなし
	fmt.Printf("\n残留ロック数: %d\n", len(dl.locks))
}

// Challenge09_AdditionalProblems - 追加の分散ロック問題
func Challenge09_AdditionalProblems() {
	fmt.Println("\n追加の問題パターン:")

	// 問題8: スプリットブレイン問題
	type ClusterNode struct {
		id        string
		leader    bool
		partition bool // ネットワーク分断
	}

	nodes := []ClusterNode{
		{id: "node1", leader: false},
		{id: "node2", leader: false},
		{id: "node3", leader: false},
	}

	// 問題のあるリーダー選出
	electLeader := func() {
		// 問題: 分断時に複数のリーダーが選出される
		for i := range nodes {
			if !nodes[i].partition {
				nodes[i].leader = true
				break
			}
		}
	}

	// 問題9: 楽観的ロック失敗
	type OptimisticLock struct {
		version int
		data    string
		mu      sync.RWMutex
	}

	ol := &OptimisticLock{version: 1, data: "initial"}

	update := func(expectedVersion int, newData string) bool {
		// 問題: バージョンチェックとアップデートがアトミックでない
		ol.mu.RLock()
		currentVersion := ol.version
		ol.mu.RUnlock()

		if currentVersion != expectedVersion {
			return false
		}

		// 問題: ここでレース条件発生
		ol.mu.Lock()
		ol.data = newData
		ol.version++
		ol.mu.Unlock()

		return true
	}

	// 問題10: フェンシングトークンなし
	type FencedLock struct {
		holder string
		// 問題: フェンシングトークンが実装されていない
	}

	_ = electLeader
	_ = update
}

// Challenge09_Hint - ヒント表示
func Challenge09_Hint() {
	fmt.Println("\n💡 ヒント:")
	fmt.Println("1. TTL（Time To Live）を実装してタイムアウトしたロックを自動解放")
	fmt.Println("2. ロック所有者の検証を厳密に行う")
	fmt.Println("3. フェンシングトークンで古いロックホルダーを無効化")
	fmt.Println("4. リース更新メカニズムの実装")
	fmt.Println("5. コンセンサスアルゴリズム（Raft/Paxos）の考慮")
	fmt.Println("6. レッドロック（Redlock）アルゴリズムの実装")
}

// Challenge09_ExpectedBehavior - 期待される動作
func Challenge09_ExpectedBehavior() {
	fmt.Println("\n✅ 期待される動作:")
	fmt.Println("1. 同一リソースに対して一度に一つのロックのみ")
	fmt.Println("2. タイムアウト時の自動解放")
	fmt.Println("3. ロック所有者のみが解放可能")
	fmt.Println("4. ネットワーク分断時の一貫性維持")
	fmt.Println("5. デッドロックの防止")
}