package practical

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// EventDrivenExample - イベント駆動アーキテクチャの実践例
type EventDrivenExample struct {
	nc *nats.Conn
	js nats.JetStreamContext
}

// Event - イベント構造体
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data"`
}

// NewEventDrivenExample - NATS接続を初期化
func NewEventDrivenExample() (*EventDrivenExample, error) {
	// NATS接続
	nc, err := nats.Connect("nats://localhost:4222",
		nats.Name("go-async-client"),
		nats.ReconnectWait(5*time.Second),
		nats.MaxReconnects(10),
	)
	if err != nil {
		return nil, err
	}

	// JetStream有効化
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}

	// ストリーム作成
	streamName := "EVENTS"
	_, err = js.StreamInfo(streamName)
	if err == nats.ErrStreamNotFound {
		_, err = js.AddStream(&nats.StreamConfig{
			Name:     streamName,
			Subjects: []string{"events.>"},
			Storage:  nats.MemoryStorage,
			MaxAge:   24 * time.Hour,
		})
		if err != nil {
			return nil, err
		}
	}

	return &EventDrivenExample{
		nc: nc,
		js: js,
	}, nil
}

// Example1_EventSourcing - イベントソーシングパターン
func (e *EventDrivenExample) Example1_EventSourcing() error {
	fmt.Println("\n📝 イベントソーシング: 状態変更をイベントとして記録")

	// イベントストア
	type OrderEvent struct {
		OrderID   string    `json:"order_id"`
		EventType string    `json:"event_type"`
		Timestamp time.Time `json:"timestamp"`
		Data      interface{} `json:"data"`
	}

	// 注文の状態遷移イベントを発行
	orderEvents := []OrderEvent{
		{OrderID: "order-001", EventType: "created", Timestamp: time.Now(), Data: map[string]interface{}{"items": 3, "total": 5000}},
		{OrderID: "order-001", EventType: "payment_received", Timestamp: time.Now(), Data: map[string]interface{}{"amount": 5000}},
		{OrderID: "order-001", EventType: "shipped", Timestamp: time.Now(), Data: map[string]interface{}{"tracking": "JP123456789"}},
		{OrderID: "order-001", EventType: "delivered", Timestamp: time.Now(), Data: map[string]interface{}{"signature": "山田太郎"}},
	}

	// イベントを順番に発行
	for _, event := range orderEvents {
		data, _ := json.Marshal(event)
		subject := fmt.Sprintf("events.orders.%s", event.OrderID)

		_, err := e.js.Publish(subject, data)
		if err != nil {
			return err
		}
		fmt.Printf("  📤 イベント発行: %s - %s\n", event.OrderID, event.EventType)
		time.Sleep(500 * time.Millisecond)
	}

	// イベントを再生して現在の状態を構築
	fmt.Println("\n  🔄 イベント再生で状態を復元:")
	consumer, _ := e.js.PullSubscribe("events.orders.>", "replay-consumer")
	msgs, _ := consumer.Fetch(10, nats.MaxWait(1*time.Second))

	for _, msg := range msgs {
		var event OrderEvent
		json.Unmarshal(msg.Data, &event)
		fmt.Printf("    %s: %s\n", event.Timestamp.Format("15:04:05"), event.EventType)
		msg.Ack()
	}

	return nil
}

// Example2_CQRS - Command Query Responsibility Segregation
func (e *EventDrivenExample) Example2_CQRS() error {
	fmt.Println("\n🔀 CQRS: コマンドとクエリの分離")

	var wg sync.WaitGroup

	// Command側: 書き込み専用
	commandHandler := func(command string, data interface{}) {
		event := Event{
			ID:        fmt.Sprintf("evt_%d", time.Now().UnixNano()),
			Type:      command,
			Timestamp: time.Now(),
			Source:    "command-handler",
			Data:      data.(map[string]interface{}),
		}

		eventData, _ := json.Marshal(event)
		e.js.Publish("events.commands", eventData)
		fmt.Printf("  ⚡ コマンド実行: %s\n", command)
	}

	// Query側: 読み込み専用（投影を更新）
	wg.Add(1)
	go func() {
		defer wg.Done()

		// ビューモデル（読み込み用）
		projection := make(map[string]interface{})

		sub, _ := e.js.Subscribe("events.commands", func(msg *nats.Msg) {
			var event Event
			json.Unmarshal(msg.Data, &event)

			// 投影を更新
			projection[event.Type] = event.Data
			fmt.Printf("  📊 投影更新: %s\n", event.Type)
			msg.Ack()
		})
		defer sub.Unsubscribe()

		time.Sleep(5 * time.Second)
	}()

	// コマンドを発行
	commands := []struct {
		cmd  string
		data map[string]interface{}
	}{
		{"CreateUser", map[string]interface{}{"id": "user-1", "name": "田中"}},
		{"UpdateProfile", map[string]interface{}{"id": "user-1", "email": "tanaka@example.com"}},
		{"PlaceOrder", map[string]interface{}{"user": "user-1", "items": 3}},
	}

	for _, c := range commands {
		commandHandler(c.cmd, c.data)
		time.Sleep(500 * time.Millisecond)
	}

	wg.Wait()
	return nil
}

// Example3_Saga - 分散トランザクションパターン
func (e *EventDrivenExample) Example3_Saga() error {
	fmt.Println("\n🎭 Saga: 分散トランザクションの実装")

	type SagaStep struct {
		Name         string
		Execute      func() error
		Compensate   func() error
	}

	// Sagaステップを定義
	steps := []SagaStep{
		{
			Name: "在庫予約",
			Execute: func() error {
				fmt.Println("  1️⃣ 在庫を予約中...")
				e.js.Publish("events.saga.inventory", []byte(`{"action":"reserve","items":5}`))
				return nil
			},
			Compensate: func() error {
				fmt.Println("  ↩️ 在庫予約をキャンセル")
				e.js.Publish("events.saga.inventory", []byte(`{"action":"cancel","items":5}`))
				return nil
			},
		},
		{
			Name: "支払い処理",
			Execute: func() error {
				fmt.Println("  2️⃣ 支払いを処理中...")
				e.js.Publish("events.saga.payment", []byte(`{"action":"charge","amount":10000}`))
				// 支払い失敗をシミュレート
				return fmt.Errorf("支払い失敗: カード限度額超過")
			},
			Compensate: func() error {
				fmt.Println("  ↩️ 支払いを返金")
				e.js.Publish("events.saga.payment", []byte(`{"action":"refund","amount":10000}`))
				return nil
			},
		},
		{
			Name: "配送手配",
			Execute: func() error {
				fmt.Println("  3️⃣ 配送を手配中...")
				e.js.Publish("events.saga.shipping", []byte(`{"action":"schedule"}`))
				return nil
			},
			Compensate: func() error {
				fmt.Println("  ↩️ 配送をキャンセル")
				e.js.Publish("events.saga.shipping", []byte(`{"action":"cancel"}`))
				return nil
			},
		},
	}

	// Sagaを実行
	completedSteps := []SagaStep{}

	for _, step := range steps {
		fmt.Printf("\n  ▶️ ステップ実行: %s\n", step.Name)
		if err := step.Execute(); err != nil {
			fmt.Printf("  ❌ エラー発生: %v\n", err)

			// 補償トランザクションを実行（ロールバック）
			fmt.Println("\n  🔙 補償トランザクション開始:")
			for i := len(completedSteps) - 1; i >= 0; i-- {
				completedSteps[i].Compensate()
			}
			return err
		}
		completedSteps = append(completedSteps, step)
	}

	fmt.Println("\n  ✅ Saga完了: すべてのステップが成功")
	return nil
}

// Example4_EventDrivenMicroservices - マイクロサービス間通信
func (e *EventDrivenExample) Example4_EventDrivenMicroservices() error {
	fmt.Println("\n🏢 イベント駆動マイクロサービス")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	// サービス1: ユーザーサービス
	wg.Add(1)
	go func() {
		defer wg.Done()
		sub, _ := e.nc.Subscribe("service.user.>", func(msg *nats.Msg) {
			fmt.Printf("  👤 ユーザーサービス: %s を処理\n", string(msg.Data))

			// 応答を返す
			response := map[string]interface{}{
				"service": "user",
				"status":  "processed",
				"data":    string(msg.Data),
			}
			respData, _ := json.Marshal(response)
			msg.Respond(respData)
		})
		defer sub.Unsubscribe()
		<-ctx.Done()
	}()

	// サービス2: 注文サービス
	wg.Add(1)
	go func() {
		defer wg.Done()
		sub, _ := e.nc.Subscribe("service.order.>", func(msg *nats.Msg) {
			fmt.Printf("  📦 注文サービス: %s を処理\n", string(msg.Data))

			// 他のサービスにイベントを発行
			e.nc.Publish("service.notification.order", msg.Data)
		})
		defer sub.Unsubscribe()
		<-ctx.Done()
	}()

	// サービス3: 通知サービス
	wg.Add(1)
	go func() {
		defer wg.Done()
		sub, _ := e.nc.Subscribe("service.notification.>", func(msg *nats.Msg) {
			fmt.Printf("  📧 通知サービス: 通知を送信 - %s\n", string(msg.Data))
		})
		defer sub.Unsubscribe()
		<-ctx.Done()
	}()

	// イベントを発行してサービス間通信をトリガー
	time.Sleep(100 * time.Millisecond) // サブスクリプション確立を待つ

	// リクエスト・レスポンスパターン
	response, err := e.nc.Request("service.user.get", []byte(`{"id":"user-123"}`), 1*time.Second)
	if err == nil {
		fmt.Printf("\n  📨 応答受信: %s\n", string(response.Data))
	}

	// パブリッシュ・サブスクライブパターン
	e.nc.Publish("service.order.create", []byte(`{"order_id":"ORD-456","user":"user-123"}`))

	time.Sleep(2 * time.Second)
	cancel()
	wg.Wait()

	return nil
}

// Close - 接続をクローズ
func (e *EventDrivenExample) Close() {
	e.nc.Close()
}