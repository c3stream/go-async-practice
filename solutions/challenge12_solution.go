package solutions

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge12_ConsistencySolution - 分散システムの一貫性問題の解決
func Challenge12_ConsistencySolution() {
	fmt.Println("\n✅ チャレンジ12: 分散システムの一貫性問題の解決")
	fmt.Println("=" + repeatString("=", 50))

	// 解決策1: 2フェーズコミット
	solution1_TwoPhaseCommit()

	// 解決策2: Sagaパターン with補償
	solution2_SagaPattern()

	// 解決策3: イベントソーシング & CQRS
	solution3_EventSourcingCQRS()
}

// 解決策1: 2フェーズコミット
func solution1_TwoPhaseCommit() {
	fmt.Println("\n📝 解決策1: 2フェーズコミット (2PC)")

	type Transaction struct {
		ID        string
		Timestamp time.Time
		State     string // "prepare", "commit", "abort"
		Data      map[string]interface{}
	}

	type Participant struct {
		ID       string
		mu       sync.RWMutex
		data     map[string]interface{}
		prepared map[string]*Transaction
		versions map[string]int64
	}

	type Coordinator struct {
		participants []*Participant
		txLog        sync.Map // トランザクションログ
	}

	// 参加者の初期化
	coordinator := &Coordinator{
		participants: []*Participant{
			{ID: "primary", data: make(map[string]interface{}), prepared: make(map[string]*Transaction), versions: make(map[string]int64)},
			{ID: "cache", data: make(map[string]interface{}), prepared: make(map[string]*Transaction), versions: make(map[string]int64)},
			{ID: "search", data: make(map[string]interface{}), prepared: make(map[string]*Transaction), versions: make(map[string]int64)},
		},
	}

	// Phase 1: Prepare
	prepare := func(txID string, key string, value interface{}) (bool, []func()) {
		tx := &Transaction{
			ID:        txID,
			Timestamp: time.Now(),
			State:     "prepare",
			Data:      map[string]interface{}{key: value},
		}

		var rollbacks []func()
		votes := make(chan bool, len(coordinator.participants))

		// 全参加者にPrepare要求
		for _, p := range coordinator.participants {
			participant := p
			go func() {
				participant.mu.Lock()
				defer participant.mu.Unlock()

				// バージョンチェック
				currentVersion := participant.versions[key]

				// Prepareを記録
				participant.prepared[txID] = tx

				// ロールバック関数を追加
				rollbacks = append(rollbacks, func() {
					participant.mu.Lock()
					delete(participant.prepared, txID)
					participant.mu.Unlock()
				})

				// 投票（ここでは常に成功とする）
				votes <- true
				fmt.Printf("  📋 %s: PREPARE vote=YES (v%d)\n", participant.ID, currentVersion)
			}()
		}

		// 全員の投票を待つ
		allVotesYes := true
		for i := 0; i < len(coordinator.participants); i++ {
			if vote := <-votes; !vote {
				allVotesYes = false
			}
		}

		return allVotesYes, rollbacks
	}

	// Phase 2: Commit or Abort
	commit := func(txID string, key string, value interface{}) {
		// 全参加者にCommit要求
		var wg sync.WaitGroup
		for _, p := range coordinator.participants {
			wg.Add(1)
			participant := p
			go func() {
				defer wg.Done()

				participant.mu.Lock()
				defer participant.mu.Unlock()

				// Preparedトランザクションをコミット
				if tx, exists := participant.prepared[txID]; exists {
					for k, v := range tx.Data {
						participant.data[k] = v
						participant.versions[k]++
					}
					delete(participant.prepared, txID)
					fmt.Printf("  ✅ %s: COMMITTED (key=%s, v%d)\n",
						participant.ID, key, participant.versions[key])
				}
			}()
		}
		wg.Wait()
	}

	abort := func(txID string, rollbacks []func()) {
		// ロールバック実行
		for _, rollback := range rollbacks {
			rollback()
		}

		// 全参加者にAbort通知
		for _, p := range coordinator.participants {
			participant := p
			go func() {
				participant.mu.Lock()
				delete(participant.prepared, txID)
				participant.mu.Unlock()
				fmt.Printf("  ❌ %s: ABORTED tx=%s\n", participant.ID, txID)
			}()
		}
	}

	// トランザクション実行
	executeTx := func(key string, value interface{}) {
		txID := fmt.Sprintf("tx_%d", time.Now().UnixNano())

		// Phase 1: Prepare
		canCommit, rollbacks := prepare(txID, key, value)

		// Phase 2: Commit or Abort
		if canCommit {
			fmt.Printf("  ▶ Coordinator: COMMIT decision for %s\n", txID)
			commit(txID, key, value)
		} else {
			fmt.Printf("  ▶ Coordinator: ABORT decision for %s\n", txID)
			abort(txID, rollbacks)
		}
	}

	// テスト実行
	fmt.Println("\n実行:")
	executeTx("user_1", map[string]interface{}{"name": "Alice", "age": 30})
	time.Sleep(100 * time.Millisecond)
	executeTx("user_2", map[string]interface{}{"name": "Bob", "age": 25})

	// 一貫性確認
	fmt.Println("\n一貫性チェック:")
	var firstData map[string]interface{}
	for i, p := range coordinator.participants {
		p.mu.RLock()
		if i == 0 {
			firstData = p.data
		}
		fmt.Printf("  %s: %v\n", p.ID, p.data)
		p.mu.RUnlock()
	}
}

// 解決策2: Sagaパターン with補償
func solution2_SagaPattern() {
	fmt.Println("\n📝 解決策2: Sagaパターン with補償トランザクション")

	type SagaStep struct {
		Name       string
		Execute    func() error
		Compensate func() error
		Executed   bool
		Failed     bool
	}

	type Saga struct {
		ID    string
		Steps []*SagaStep
		mu    sync.Mutex
	}

	// Sagaの実行
	executeSaga := func(saga *Saga) error {
		saga.mu.Lock()
		defer saga.mu.Unlock()

		fmt.Printf("\n🎭 Saga %s 開始\n", saga.ID)

		// 各ステップを順次実行
		for i, step := range saga.Steps {
			fmt.Printf("  ▶ Step %d: %s 実行中...\n", i+1, step.Name)

			if err := step.Execute(); err != nil {
				step.Failed = true
				fmt.Printf("  ❌ Step %d: %s 失敗: %v\n", i+1, step.Name, err)

				// 補償トランザクション実行（逆順）
				fmt.Println("  🔄 補償トランザクション開始")
				for j := i - 1; j >= 0; j-- {
					if saga.Steps[j].Executed {
						fmt.Printf("    ↩ Compensating: %s\n", saga.Steps[j].Name)
						if compErr := saga.Steps[j].Compensate(); compErr != nil {
							fmt.Printf("    ⚠ 補償失敗: %v\n", compErr)
						}
					}
				}
				return err
			}

			step.Executed = true
			fmt.Printf("  ✅ Step %d: %s 成功\n", i+1, step.Name)
		}

		fmt.Printf("🎉 Saga %s 完了\n", saga.ID)
		return nil
	}

	// 注文処理Sagaの例
	var orderID atomic.Int32
	var inventory atomic.Int32
	inventory.Store(10)
	var balance atomic.Int32
	balance.Store(1000)

	createOrderSaga := func(productID string, quantity int32, price int32) *Saga {
		ordID := orderID.Add(1)

		return &Saga{
			ID: fmt.Sprintf("order_%d", ordID),
			Steps: []*SagaStep{
				{
					Name: "在庫予約",
					Execute: func() error {
						current := inventory.Load()
						if current < quantity {
							return fmt.Errorf("在庫不足: %d < %d", current, quantity)
						}
						inventory.Add(-quantity)
						fmt.Printf("    在庫: %d → %d\n", current, inventory.Load())
						return nil
					},
					Compensate: func() error {
						before := inventory.Load()
						inventory.Add(quantity)
						fmt.Printf("    在庫復元: %d → %d\n", before, inventory.Load())
						return nil
					},
				},
				{
					Name: "支払い処理",
					Execute: func() error {
						current := balance.Load()
						total := quantity * price
						if current < total {
							return fmt.Errorf("残高不足: %d < %d", current, total)
						}
						balance.Add(-total)
						fmt.Printf("    残高: %d → %d\n", current, balance.Load())
						return nil
					},
					Compensate: func() error {
						before := balance.Load()
						balance.Add(quantity * price)
						fmt.Printf("    残高復元: %d → %d\n", before, balance.Load())
						return nil
					},
				},
				{
					Name: "配送手配",
					Execute: func() error {
						// 50%の確率で失敗（外部サービスエラーをシミュレート）
						if time.Now().UnixNano()%2 == 0 {
							return fmt.Errorf("配送サービス一時的に利用不可")
						}
						fmt.Printf("    配送手配完了: OrderID=%d\n", ordID)
						return nil
					},
					Compensate: func() error {
						fmt.Printf("    配送キャンセル: OrderID=%d\n", ordID)
						return nil
					},
				},
			},
		}
	}

	// テスト実行
	fmt.Println("\n実行:")

	// 成功ケース
	saga1 := createOrderSaga("product_1", 2, 100)
	executeSaga(saga1)

	time.Sleep(100 * time.Millisecond)

	// 失敗ケース（在庫不足）
	saga2 := createOrderSaga("product_2", 20, 50)
	executeSaga(saga2)

	fmt.Printf("\n最終状態: 在庫=%d, 残高=%d\n", inventory.Load(), balance.Load())
}

// 解決策3: イベントソーシング & CQRS
func solution3_EventSourcingCQRS() {
	fmt.Println("\n📝 解決策3: イベントソーシング & CQRS")

	// イベントタイプ
	type EventType string
	const (
		UserCreated EventType = "UserCreated"
		UserUpdated EventType = "UserUpdated"
		UserDeleted EventType = "UserDeleted"
	)

	type Event struct {
		ID        string
		Type      EventType
		AggregateID string
		Timestamp time.Time
		Version   int64
		Data      map[string]interface{}
	}

	// イベントストア
	type EventStore struct {
		mu     sync.RWMutex
		events []Event
		snapshots map[string]interface{} // AggregateID -> Snapshot
		version atomic.Int64
	}

	store := &EventStore{
		events: make([]Event, 0),
		snapshots: make(map[string]interface{}),
	}

	// イベント追加（書き込み側）
	appendEvent := func(eventType EventType, aggregateID string, data map[string]interface{}) *Event {
		store.mu.Lock()
		defer store.mu.Unlock()

		version := store.version.Add(1)
		event := Event{
			ID:          fmt.Sprintf("evt_%d", time.Now().UnixNano()),
			Type:        eventType,
			AggregateID: aggregateID,
			Timestamp:   time.Now(),
			Version:     version,
			Data:        data,
		}

		store.events = append(store.events, event)
		fmt.Printf("  📝 Event: %s v%d - %s\n", event.Type, event.Version, aggregateID)

		return &event
	}

	// 読み取りモデル（CQRS）
	type ReadModel struct {
		mu    sync.RWMutex
		users map[string]map[string]interface{}
		userVersions map[string]int64
	}

	readModel := &ReadModel{
		users: make(map[string]map[string]interface{}),
		userVersions: make(map[string]int64),
	}

	// イベントハンドラー（投影）
	projectEvent := func(event *Event) {
		readModel.mu.Lock()
		defer readModel.mu.Unlock()

		switch event.Type {
		case UserCreated:
			readModel.users[event.AggregateID] = event.Data
			readModel.userVersions[event.AggregateID] = event.Version
			fmt.Printf("  📊 Projection: User created %s (v%d)\n",
				event.AggregateID, event.Version)

		case UserUpdated:
			if existing, exists := readModel.users[event.AggregateID]; exists {
				// マージ更新
				for k, v := range event.Data {
					existing[k] = v
				}
				readModel.userVersions[event.AggregateID] = event.Version
				fmt.Printf("  📊 Projection: User updated %s (v%d)\n",
					event.AggregateID, event.Version)
			}

		case UserDeleted:
			delete(readModel.users, event.AggregateID)
			delete(readModel.userVersions, event.AggregateID)
			fmt.Printf("  📊 Projection: User deleted %s\n", event.AggregateID)
		}
	}

	// イベントリプレイ（スナップショットから）
	replayEvents := func(fromVersion int64) {
		store.mu.RLock()
		events := make([]Event, len(store.events))
		copy(events, store.events)
		store.mu.RUnlock()

		fmt.Printf("\n🔄 Replaying events from version %d\n", fromVersion)
		for _, event := range events {
			if event.Version > fromVersion {
				projectEvent(&event)
			}
		}
	}

	// コマンド処理
	createUser := func(userID string, name string, email string) {
		data := map[string]interface{}{
			"name":  name,
			"email": email,
			"created_at": time.Now(),
		}
		event := appendEvent(UserCreated, userID, data)
		projectEvent(event)
	}

	updateUser := func(userID string, updates map[string]interface{}) {
		updates["updated_at"] = time.Now()
		event := appendEvent(UserUpdated, userID, updates)
		projectEvent(event)
	}

	// クエリ処理
	getUser := func(userID string) (map[string]interface{}, int64, bool) {
		readModel.mu.RLock()
		defer readModel.mu.RUnlock()

		user, exists := readModel.users[userID]
		version := readModel.userVersions[userID]
		return user, version, exists
	}

	// テスト実行
	fmt.Println("\n実行:")

	// ユーザー作成
	createUser("user_1", "Alice", "alice@example.com")
	createUser("user_2", "Bob", "bob@example.com")

	// ユーザー更新
	updateUser("user_1", map[string]interface{}{"email": "alice.new@example.com"})

	// 読み取り
	fmt.Println("\n現在の読み取りモデル:")
	if user, version, exists := getUser("user_1"); exists {
		fmt.Printf("  User1: %v (v%d)\n", user, version)
	}
	if user, version, exists := getUser("user_2"); exists {
		fmt.Printf("  User2: %v (v%d)\n", user, version)
	}

	// イベントストアの状態
	fmt.Printf("\nイベントストア: %d events\n", len(store.events))

	// リプレイテスト
	readModel.users = make(map[string]map[string]interface{})
	readModel.userVersions = make(map[string]int64)
	replayEvents(0)

	fmt.Println("\nリプレイ後の読み取りモデル:")
	for id := range readModel.users {
		if user, version, exists := getUser(id); exists {
			fmt.Printf("  %s: %v (v%d)\n", id, user, version)
		}
	}
}