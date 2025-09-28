package solutions

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge13_EventSourcingSolution - イベントソーシング問題の解決
func Challenge13_EventSourcingSolution() {
	fmt.Println("\n✅ チャレンジ13: イベントソーシング問題の解決")
	fmt.Println("=" + repeatString("=", 50))

	// 解決策1: イベントストアとスナップショット
	solution1_EventStoreWithSnapshot()

	// 解決策2: プロジェクション分離（CQRS）
	solution2_ProjectionSeparation()

	// 解決策3: イベントバージョニング
	solution3_EventVersioning()
}

// 解決策1: イベントストアとスナップショット
func solution1_EventStoreWithSnapshot() {
	fmt.Println("\n📝 解決策1: イベントストアとスナップショット")

	type Event struct {
		ID         string
		StreamID   string
		Type       string
		Version    int64
		Data       interface{}
		Timestamp  time.Time
		Metadata   map[string]string
	}

	type Snapshot struct {
		StreamID  string
		Version   int64
		State     interface{}
		Timestamp time.Time
	}

	type EventStore struct {
		mu               sync.RWMutex
		events           map[string][]Event // StreamID -> Events
		snapshots        map[string]*Snapshot
		globalVersion    atomic.Int64
		snapshotInterval int64
	}

	store := &EventStore{
		events:           make(map[string][]Event),
		snapshots:        make(map[string]*Snapshot),
		snapshotInterval: 5, // 5イベントごとにスナップショット
	}

	// イベント追加
	appendEvent := func(streamID string, eventType string, data interface{}) (*Event, error) {
		store.mu.Lock()
		defer store.mu.Unlock()

		// ストリームのバージョン取得
		streamEvents := store.events[streamID]
		streamVersion := int64(len(streamEvents))

		// グローバルバージョン
		globalVersion := store.globalVersion.Add(1)

		event := Event{
			ID:        fmt.Sprintf("evt_%d", globalVersion),
			StreamID:  streamID,
			Type:      eventType,
			Version:   streamVersion + 1,
			Data:      data,
			Timestamp: time.Now(),
			Metadata: map[string]string{
				"source": "api",
			},
		}

		store.events[streamID] = append(store.events[streamID], event)

		// スナップショット作成チェック
		if (streamVersion+1)%store.snapshotInterval == 0 {
			fmt.Printf("  📸 Creating snapshot at version %d\n", streamVersion+1)
			store.snapshots[streamID] = &Snapshot{
				StreamID:  streamID,
				Version:   streamVersion + 1,
				State:     data, // 実際はアグリゲート状態
				Timestamp: time.Now(),
			}
		}

		fmt.Printf("  ✓ Event %s v%d appended to stream %s\n",
			eventType, event.Version, streamID)
		return &event, nil
	}

	// ストリーム取得（スナップショット利用）
	getStream := func(streamID string, fromVersion int64) ([]Event, *Snapshot, error) {
		store.mu.RLock()
		defer store.mu.RUnlock()

		events := store.events[streamID]
		snapshot := store.snapshots[streamID]

		if snapshot != nil && snapshot.Version > fromVersion {
			// スナップショット以降のイベントのみ返す
			var recentEvents []Event
			for _, e := range events {
				if e.Version > snapshot.Version {
					recentEvents = append(recentEvents, e)
				}
			}
			fmt.Printf("  🎯 Using snapshot at v%d + %d events\n",
				snapshot.Version, len(recentEvents))
			return recentEvents, snapshot, nil
		}

		// 指定バージョン以降のイベント
		var filteredEvents []Event
		for _, e := range events {
			if e.Version > fromVersion {
				filteredEvents = append(filteredEvents, e)
			}
		}

		return filteredEvents, nil, nil
	}

	// テスト実行
	fmt.Println("\n実行:")

	// 複数のイベント追加
	streamID := "order_123"
	eventTypes := []string{
		"OrderCreated",
		"ItemAdded",
		"ItemAdded",
		"PaymentInitiated",
		"PaymentCompleted",
		"OrderShipped",
		"OrderDelivered",
	}

	for i, eventType := range eventTypes {
		appendEvent(streamID, eventType, map[string]interface{}{
			"step": i + 1,
			"type": eventType,
		})
		time.Sleep(100 * time.Millisecond)
	}

	// ストリーム再構築
	fmt.Println("\nストリーム再構築:")
	events, snapshot, _ := getStream(streamID, 0)
	if snapshot != nil {
		fmt.Printf("  Snapshot: v%d at %v\n", snapshot.Version, snapshot.Timestamp)
	}
	fmt.Printf("  Events to replay: %d\n", len(events))
}

// 解決策2: プロジェクション分離（CQRS）
func solution2_ProjectionSeparation() {
	fmt.Println("\n📝 解決策2: プロジェクション分離（CQRS）")

	// イベント定義
	type OrderEvent struct {
		OrderID   string
		Type      string
		Timestamp time.Time
		Data      map[string]interface{}
	}

	// 書き込みモデル
	type WriteModel struct {
		mu     sync.Mutex
		events []OrderEvent
	}

	// 読み取りモデル（複数のプロジェクション）
	type OrderSummary struct {
		OrderID     string
		Status      string
		TotalAmount float64
		ItemCount   int
		LastUpdated time.Time
	}

	type CustomerView struct {
		CustomerID   string
		OrderCount   int
		TotalSpent   float64
		LastOrderDate time.Time
	}

	type ReadModel struct {
		mu            sync.RWMutex
		orderSummary  map[string]*OrderSummary
		customerView  map[string]*CustomerView
	}

	writeModel := &WriteModel{
		events: make([]OrderEvent, 0),
	}

	readModel := &ReadModel{
		orderSummary: make(map[string]*OrderSummary),
		customerView: make(map[string]*CustomerView),
	}

	// プロジェクション更新
	var updateProjections func(event OrderEvent, model *ReadModel)
	updateProjections = func(event OrderEvent, model *ReadModel) {
		model.mu.Lock()
		defer model.mu.Unlock()

		// 注文サマリー更新
		summary, exists := model.orderSummary[event.OrderID]
		if !exists {
			summary = &OrderSummary{
				OrderID: event.OrderID,
			}
			model.orderSummary[event.OrderID] = summary
		}

		switch event.Type {
		case "OrderCreated":
			summary.Status = "Created"
			summary.LastUpdated = event.Timestamp
		case "ItemAdded":
			summary.ItemCount++
			if amount, ok := event.Data["amount"].(float64); ok {
				summary.TotalAmount += amount
			}
			summary.LastUpdated = event.Timestamp
		case "OrderShipped":
			summary.Status = "Shipped"
			summary.LastUpdated = event.Timestamp
		}

		// 顧客ビュー更新
		if customerID, ok := event.Data["customer_id"].(string); ok {
			view, exists := model.customerView[customerID]
			if !exists {
				view = &CustomerView{
					CustomerID: customerID,
				}
				model.customerView[customerID] = view
			}

			if event.Type == "OrderCreated" {
				view.OrderCount++
				view.LastOrderDate = event.Timestamp
			}
			if amount, ok := event.Data["amount"].(float64); ok {
				view.TotalSpent += amount
			}
		}

		fmt.Printf("  📊 Projections updated for %s\n", event.OrderID)
	}

	// イベント発行（書き込み側）
	publishEvent := func(event OrderEvent) {
		writeModel.mu.Lock()
		writeModel.events = append(writeModel.events, event)
		writeModel.mu.Unlock()

		fmt.Printf("  📝 Event: %s for order %s\n", event.Type, event.OrderID)

		// 非同期でプロジェクション更新
		go updateProjections(event, readModel)
	}

	// クエリ（読み取り側）
	getOrderSummary := func(orderID string) *OrderSummary {
		readModel.mu.RLock()
		defer readModel.mu.RUnlock()
		return readModel.orderSummary[orderID]
	}

	getCustomerView := func(customerID string) *CustomerView {
		readModel.mu.RLock()
		defer readModel.mu.RUnlock()
		return readModel.customerView[customerID]
	}

	// テスト実行
	fmt.Println("\n実行:")

	// イベント発行
	events := []OrderEvent{
		{
			OrderID:   "order_001",
			Type:      "OrderCreated",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"customer_id": "customer_123",
			},
		},
		{
			OrderID:   "order_001",
			Type:      "ItemAdded",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"item_id": "item_1",
				"amount":  99.99,
			},
		},
		{
			OrderID:   "order_001",
			Type:      "ItemAdded",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"item_id": "item_2",
				"amount":  49.99,
			},
		},
		{
			OrderID:   "order_001",
			Type:      "OrderShipped",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{},
		},
	}

	for _, event := range events {
		publishEvent(event)
		time.Sleep(100 * time.Millisecond)
	}

	// クエリ実行
	time.Sleep(500 * time.Millisecond) // プロジェクション更新待ち

	fmt.Println("\nクエリ結果:")
	if summary := getOrderSummary("order_001"); summary != nil {
		fmt.Printf("  Order Summary: Status=%s, Items=%d, Total=$%.2f\n",
			summary.Status, summary.ItemCount, summary.TotalAmount)
	}

	if view := getCustomerView("customer_123"); view != nil {
		fmt.Printf("  Customer View: Orders=%d, Total Spent=$%.2f\n",
			view.OrderCount, view.TotalSpent)
	}
}

// 解決策3: イベントバージョニング
func solution3_EventVersioning() {
	fmt.Println("\n📝 解決策3: イベントバージョニング")

	// V1イベント
	type OrderCreatedV1 struct {
		OrderID    string
		CustomerID string
		Amount     float64
		Version    string
	}

	// V2イベント（新しいフィールド追加）
	type OrderCreatedV2 struct {
		OrderID    string
		CustomerID string
		Amount     float64
		Currency   string // 新フィールド
		Tax        float64 // 新フィールド
		Version    string
	}

	// イベントハンドラー
	type EventHandler struct {
		handlers map[string]map[string]func(interface{})
	}

	handler := &EventHandler{
		handlers: make(map[string]map[string]func(interface{})),
	}

	// ハンドラー登録
	registerHandler := func(eventType, version string, fn func(interface{})) {
		if handler.handlers[eventType] == nil {
			handler.handlers[eventType] = make(map[string]func(interface{}))
		}
		handler.handlers[eventType][version] = fn
	}

	// イベント処理
	processEvent := func(eventType, version string, event interface{}) {
		if handlers, exists := handler.handlers[eventType]; exists {
			if h, versionExists := handlers[version]; versionExists {
				h(event)
			} else {
				// バージョン互換性処理
				fmt.Printf("  ⚠ No handler for %s %s, attempting compatibility\n",
					eventType, version)

				// V1からV2への自動変換
				if version == "v1" && handlers["v2"] != nil {
					// アップグレード処理
					if v1Event, ok := event.(OrderCreatedV1); ok {
						v2Event := OrderCreatedV2{
							OrderID:    v1Event.OrderID,
							CustomerID: v1Event.CustomerID,
							Amount:     v1Event.Amount,
							Currency:   "USD", // デフォルト値
							Tax:        v1Event.Amount * 0.1, // 計算値
							Version:    "v2",
						}
						handlers["v2"](v2Event)
					}
				}
			}
		}
	}

	// ハンドラー実装
	registerHandler("OrderCreated", "v1", func(e interface{}) {
		if event, ok := e.(OrderCreatedV1); ok {
			fmt.Printf("  ✓ V1 Handler: Order %s created for $%.2f\n",
				event.OrderID, event.Amount)
		}
	})

	registerHandler("OrderCreated", "v2", func(e interface{}) {
		if event, ok := e.(OrderCreatedV2); ok {
			fmt.Printf("  ✓ V2 Handler: Order %s created for $%.2f %s (tax: $%.2f)\n",
				event.OrderID, event.Amount, event.Currency, event.Tax)
		}
	})

	// テスト実行
	fmt.Println("\n実行:")

	// V1イベント
	v1Event := OrderCreatedV1{
		OrderID:    "order_001",
		CustomerID: "customer_123",
		Amount:     100.00,
		Version:    "v1",
	}

	// V2イベント
	v2Event := OrderCreatedV2{
		OrderID:    "order_002",
		CustomerID: "customer_456",
		Amount:     200.00,
		Currency:   "EUR",
		Tax:        40.00,
		Version:    "v2",
	}

	fmt.Println("V1イベント処理:")
	processEvent("OrderCreated", "v1", v1Event)

	fmt.Println("\nV2イベント処理:")
	processEvent("OrderCreated", "v2", v2Event)

	fmt.Println("\nV1ハンドラー削除後のV1イベント処理（自動アップグレード）:")
	delete(handler.handlers["OrderCreated"], "v1")
	processEvent("OrderCreated", "v1", v1Event)
}