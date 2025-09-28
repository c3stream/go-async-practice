package challenges

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Challenge10_MessageOrderingProblem - メッセージ順序保証の問題
// 問題: 分散システムでメッセージの順序が保証されない
func Challenge10_MessageOrderingProblem() {
	fmt.Println("\n🔥 チャレンジ10: メッセージ順序保証の問題")
	fmt.Println("===================================================")
	fmt.Println("問題: イベント駆動システムでメッセージ順序が崩れます")
	fmt.Println("症状: 更新の逆転、状態の不整合、データ競合")
	fmt.Println("\n⚠️  このコードには複数の問題があります:")

	// 問題のあるメッセージハンドラー
	type Message struct {
		ID        int
		Topic     string
		Payload   interface{}
		Timestamp time.Time
		// 問題2: シーケンス番号なし
	}

	type MessageBroker struct {
		// 問題1: 単一チャネルで順序保証なし
		messages chan Message
		handlers map[string][]func(Message)
		mu       sync.RWMutex
		wg       sync.WaitGroup
	}

	broker := &MessageBroker{
		messages: make(chan Message, 100), // 問題3: バッファが大きすぎる
		handlers: make(map[string][]func(Message)),
	}

	// 問題のあるパブリッシュ
	publish := func(topic string, payload interface{}) {
		msg := Message{
			ID:        rand.Intn(1000), // 問題4: ランダムID
			Topic:     topic,
			Payload:   payload,
			Timestamp: time.Now(),
		}

		// 問題5: 非ブロッキング送信で順序保証なし
		select {
		case broker.messages <- msg:
		default:
			fmt.Println("  ⚠ メッセージドロップ")
		}
	}

	// 問題のある購読処理
	subscribe := func(topic string, handler func(Message)) {
		broker.mu.Lock()
		broker.handlers[topic] = append(broker.handlers[topic], handler)
		broker.mu.Unlock()
	}

	// 問題のあるメッセージ配信
	startBroker := func() {
		// 問題6: 複数のゴルーチンで並行処理
		for i := 0; i < 3; i++ {
			broker.wg.Add(1)
			go func() {
				defer broker.wg.Done()
				for msg := range broker.messages {
					broker.mu.RLock()
					handlers := broker.handlers[msg.Topic]
					broker.mu.RUnlock()

					// 問題7: ハンドラーを並行実行
					for _, handler := range handlers {
						go handler(msg) // 順序保証なし
					}
				}
			}()
		}
	}

	// テストシナリオ
	fmt.Println("\n実行結果:")

	// カウンターの更新（順序依存）
	counter := 0
	var counterMu sync.Mutex

	subscribe("counter", func(msg Message) {
		counterMu.Lock()
		defer counterMu.Unlock()

		operation := msg.Payload.(string)
		if operation == "increment" {
			counter++
			fmt.Printf("  カウンター: %d (msg: %d)\n", counter, msg.ID)
		} else if operation == "reset" {
			counter = 0
			fmt.Printf("  カウンターリセット (msg: %d)\n", msg.ID)
		}
	})

	startBroker()

	// 順序を期待するメッセージ送信
	operations := []string{
		"increment", // 1
		"increment", // 2
		"increment", // 3
		"reset",     // 0
		"increment", // 1
		"increment", // 2
	}

	for _, op := range operations {
		publish("counter", op)
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(500 * time.Millisecond)
	close(broker.messages)
	broker.wg.Wait()

	fmt.Printf("\n最終カウンター値: %d (期待値: 2)\n", counter)

	// 問題8: イベントソーシングでの順序問題
	demonstrateEventSourcingProblem()
}

func demonstrateEventSourcingProblem() {
	fmt.Println("\n追加の問題: イベントソーシング")

	type Event struct {
		AggregateID string
		Version     int
		Type        string
		Data        interface{}
		// 問題9: タイムスタンプのみで順序決定
		Timestamp time.Time
	}

	type EventStore struct {
		events []Event
		mu     sync.Mutex
		// 問題10: バージョン管理なし
	}

	store := &EventStore{
		events: make([]Event, 0),
	}

	// 問題のあるイベント追加
	appendEvent := func(event Event) {
		// 問題11: バージョンチェックなし
		store.mu.Lock()
		store.events = append(store.events, event)
		store.mu.Unlock()
	}

	// 並行してイベント追加
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			event := Event{
				AggregateID: "user123",
				Version:     id, // 問題: 競合するバージョン
				Type:        "Updated",
				Data:        fmt.Sprintf("update_%d", id),
				Timestamp:   time.Now(),
			}
			appendEvent(event)
		}(i)
	}

	wg.Wait()

	// 問題12: リプレイ時の順序
	fmt.Println("イベント順序:")
	store.mu.Lock()
	for _, e := range store.events {
		fmt.Printf("  Version %d: %v\n", e.Version, e.Data)
	}
	store.mu.Unlock()
}

// Challenge10_AdditionalProblems - 追加のメッセージング問題
func Challenge10_AdditionalProblems() {
	fmt.Println("\n追加の問題パターン:")

	// 問題13: パーティション間の順序
	type PartitionedQueue struct {
		partitions []chan interface{}
		// 問題: パーティション間で順序保証なし
	}

	// 問題14: 優先度付きキューでの飢餓
	type PriorityMessage struct {
		Priority int
		Data     interface{}
	}

	// 問題: 低優先度メッセージが永遠に処理されない

	// 問題15: At-least-once配信での重複
	type ReliableDelivery struct {
		// 問題: 重複メッセージの検出なし
		// 問題: 冪等性の保証なし
	}
}

// Challenge10_Hint - ヒント表示
func Challenge10_Hint() {
	fmt.Println("\n💡 ヒント:")
	fmt.Println("1. Lamportタイムスタンプまたはベクタークロックの使用")
	fmt.Println("2. メッセージのシーケンス番号管理")
	fmt.Println("3. パーティションキーによる順序保証")
	fmt.Println("4. 単一のコンシューマーで順序処理")
	fmt.Println("5. イベントバージョニングの実装")
	fmt.Println("6. Sagaパターンでの順序制御")
}

// Challenge10_ExpectedBehavior - 期待される動作
func Challenge10_ExpectedBehavior() {
	fmt.Println("\n✅ 期待される動作:")
	fmt.Println("1. 同一エンティティのイベントは順序通り処理")
	fmt.Println("2. カウンター操作が正しい順序で実行")
	fmt.Println("3. イベントバージョンの競合検出")
	fmt.Println("4. メッセージの重複排除")
	fmt.Println("5. 全順序または因果順序の保証")
}