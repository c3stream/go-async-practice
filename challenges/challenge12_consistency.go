package challenges

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge12_ConsistencyProblem - 分散システムの一貫性問題
// 問題: 複数のデータストア間で一貫性が保たれない
func Challenge12_ConsistencyProblem() {
	fmt.Println("\n🔥 チャレンジ12: 分散システムの一貫性問題")
	fmt.Println("=" + repeatString("=", 50))
	fmt.Println("問題: マイクロサービス間でデータ一貫性が崩れます")
	fmt.Println("症状: ダーティリード、ロストアップデート、ファントムリード")
	fmt.Println("\n⚠️  このコードには複数の問題があります:")

	// 問題のある分散トランザクション
	type DistributedSystem struct {
		// 複数のデータストア
		primaryDB   *DataStore
		cacheDB     *DataStore
		searchIndex *DataStore
		// 問題1: トランザクションコーディネーターなし
	}

	type DataStore struct {
		mu   sync.RWMutex
		data map[string]interface{}
		// 問題2: バージョン管理なし
	}

	system := &DistributedSystem{
		primaryDB:   &DataStore{data: make(map[string]interface{})},
		cacheDB:     &DataStore{data: make(map[string]interface{})},
		searchIndex: &DataStore{data: make(map[string]interface{})},
	}

	// 問題のある書き込み操作
	write := func(key string, value interface{}) error {
		// 問題3: 非アトミックな複数データストア更新

		// プライマリDBに書き込み
		system.primaryDB.mu.Lock()
		system.primaryDB.data[key] = value
		system.primaryDB.mu.Unlock()

		// 問題4: エラー処理なし
		time.Sleep(10 * time.Millisecond) // ネットワーク遅延

		// キャッシュに書き込み
		system.cacheDB.mu.Lock()
		system.cacheDB.data[key] = value
		system.cacheDB.mu.Unlock()

		// 問題5: 部分的な失敗の可能性
		if time.Now().Unix()%3 == 0 {
			// 30%の確率で検索インデックス更新失敗
			return fmt.Errorf("search index update failed")
		}

		// 検索インデックスに書き込み
		system.searchIndex.mu.Lock()
		system.searchIndex.data[key] = value
		system.searchIndex.mu.Unlock()

		return nil
	}

	// 問題のある読み取り操作
	read := func(key string, source string) (interface{}, error) {
		switch source {
		case "cache":
			// 問題6: キャッシュの一貫性チェックなし
			system.cacheDB.mu.RLock()
			value, exists := system.cacheDB.data[key]
			system.cacheDB.mu.RUnlock()
			if !exists {
				// キャッシュミス時にプライマリから読む
				system.primaryDB.mu.RLock()
				value = system.primaryDB.data[key]
				system.primaryDB.mu.RUnlock()
				// 問題7: 非同期でキャッシュ更新（レース条件）
				go func() {
					system.cacheDB.mu.Lock()
					system.cacheDB.data[key] = value
					system.cacheDB.mu.Unlock()
				}()
			}
			return value, nil
		case "primary":
			system.primaryDB.mu.RLock()
			value := system.primaryDB.data[key]
			system.primaryDB.mu.RUnlock()
			return value, nil
		default:
			return nil, fmt.Errorf("unknown source")
		}
	}

	// テストシナリオ
	fmt.Println("\n実行結果:")

	var wg sync.WaitGroup

	// 並行書き込み
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("user_%d", id)
			value := fmt.Sprintf("data_%d_%d", id, time.Now().Unix())

			if err := write(key, value); err != nil {
				fmt.Printf("  ✗ 書き込み失敗 %s: %v\n", key, err)
			} else {
				fmt.Printf("  ✓ 書き込み成功 %s\n", key)
			}
		}(i)
	}

	// 並行読み取り
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("user_%d", id)

			// キャッシュから読み取り
			cacheValue, _ := read(key, "cache")
			// プライマリから読み取り
			primaryValue, _ := read(key, "primary")

			if cacheValue != primaryValue {
				fmt.Printf("  ⚠ 不整合検出! key=%s cache=%v primary=%v\n",
					key, cacheValue, primaryValue)
			}
		}(i)
	}

	wg.Wait()

	// 最終的な一貫性チェック
	checkConsistency(system)
}

func checkConsistency(system *DistributedSystem) {
	fmt.Println("\n最終一貫性チェック:")

	inconsistencies := 0

	system.primaryDB.mu.RLock()
	for key, primaryValue := range system.primaryDB.data {
		system.cacheDB.mu.RLock()
		cacheValue := system.cacheDB.data[key]
		system.cacheDB.mu.RUnlock()

		system.searchIndex.mu.RLock()
		searchValue := system.searchIndex.data[key]
		system.searchIndex.mu.RUnlock()

		if cacheValue != primaryValue || searchValue != primaryValue {
			inconsistencies++
			fmt.Printf("  不整合: key=%s primary=%v cache=%v search=%v\n",
				key, primaryValue, cacheValue, searchValue)
		}
	}
	system.primaryDB.mu.RUnlock()

	if inconsistencies == 0 {
		fmt.Println("  ✓ 全データ一貫性あり")
	} else {
		fmt.Printf("  ✗ %d件の不整合を検出\n", inconsistencies)
	}
}

// Challenge12_EventualConsistencyProblem - 結果整合性の問題
func Challenge12_EventualConsistencyProblem() {
	fmt.Println("\n追加の問題: 結果整合性")

	type ReplicatedData struct {
		replicas [3]struct {
			mu      sync.RWMutex
			data    map[string]int
			version map[string]int64
		}
		// 問題8: ベクタークロックなし
	}

	replicated := &ReplicatedData{}
	for i := range replicated.replicas {
		replicated.replicas[i].data = make(map[string]int)
		replicated.replicas[i].version = make(map[string]int64)
	}

	// 問題のあるレプリケーション
	replicate := func(sourceIdx int, key string, value int) {
		// ソースレプリカに書き込み
		replicated.replicas[sourceIdx].mu.Lock()
		replicated.replicas[sourceIdx].data[key] = value
		replicated.replicas[sourceIdx].version[key]++
		version := replicated.replicas[sourceIdx].version[key]
		replicated.replicas[sourceIdx].mu.Unlock()

		// 問題9: 非同期レプリケーション（順序保証なし）
		for i := range replicated.replicas {
			if i != sourceIdx {
				go func(idx int) {
					// ランダムな遅延
					time.Sleep(time.Duration(10+idx*10) * time.Millisecond)

					replicated.replicas[idx].mu.Lock()
					// 問題10: バージョン競合の解決なし
					if replicated.replicas[idx].version[key] < version {
						replicated.replicas[idx].data[key] = value
						replicated.replicas[idx].version[key] = version
					}
					replicated.replicas[idx].mu.Unlock()
				}(i)
			}
		}
	}

	// 並行更新でコンフリクト
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(replica int) {
			defer wg.Done()
			replicate(replica, "counter", replica*10)
		}(i)
	}
	wg.Wait()

	time.Sleep(100 * time.Millisecond)

	// 結果を確認
	fmt.Println("レプリカの状態:")
	for i := range replicated.replicas {
		replicated.replicas[i].mu.RLock()
		fmt.Printf("  レプリカ%d: counter=%d (version=%d)\n",
			i, replicated.replicas[i].data["counter"],
			replicated.replicas[i].version["counter"])
		replicated.replicas[i].mu.RUnlock()
	}
}

// Challenge12_SagaProblem - Sagaパターンの問題
func Challenge12_SagaProblem() {
	fmt.Println("\n追加の問題: Sagaパターン")

	type SagaTransaction struct {
		steps      []func() error
		compensate []func() error
		// 問題11: ステップの実行状態管理なし
	}

	executeSaga := func(saga *SagaTransaction) error {
		for i, step := range saga.steps {
			if err := step(); err != nil {
				// 問題12: 補償トランザクションが逆順でない
				for j := 0; j < i; j++ {
					saga.compensate[j]()
				}
				return err
			}
		}
		// 問題13: 部分的な成功状態の永続化なし
		return nil
	}

	// 問題14: 冪等性なし
	var orderCount int32
	createOrder := func() error {
		atomic.AddInt32(&orderCount, 1)
		// 問題: リトライ時に重複作成
		return nil
	}

	// 問題15: タイムアウト処理なし
	chargePayment := func() error {
		// 長時間かかる処理
		time.Sleep(5 * time.Second)
		return nil
	}

	_ = executeSaga
	_ = createOrder
	_ = chargePayment
}

// Challenge12_Hint - ヒント表示
func Challenge12_Hint() {
	fmt.Println("\n💡 ヒント:")
	fmt.Println("1. 2フェーズコミットまたは3フェーズコミット")
	fmt.Println("2. Sagaパターンでの補償トランザクション")
	fmt.Println("3. イベントソーシングとCQRS")
	fmt.Println("4. ベクタークロックまたはバージョンベクトル")
	fmt.Println("5. Read-Repair とAnti-Entropy")
	fmt.Println("6. Quorum読み書き（W + R > N）")
	fmt.Println("7. 楽観的ロックとCAS操作")
}

// Challenge12_ExpectedBehavior - 期待される動作
func Challenge12_ExpectedBehavior() {
	fmt.Println("\n✅ 期待される動作:")
	fmt.Println("1. すべてのデータストアで最終的に一貫性が保たれる")
	fmt.Println("2. 部分的な失敗時に適切にロールバック")
	fmt.Println("3. 並行更新時のコンフリクト解決")
	fmt.Println("4. 冪等性のある操作")
	fmt.Println("5. 分散トランザクションの原子性保証")
}