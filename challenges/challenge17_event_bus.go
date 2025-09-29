package challenges

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Challenge 17: Event Bus Pattern with Distributed Events
//
// 問題点:
// 1. イベントの重複配信
// 2. サブスクライバーのメモリリーク
// 3. デッドロックの可能性
// 4. イベント順序の保証なし
// 5. エラーハンドリング不足

type EventBusEvent struct {
	ID        string
	Type      string
	Payload   interface{}
	Timestamp time.Time
}

type EventHandler func(ctx context.Context, event EventBusEvent) error

type EventBus struct {
	mu           sync.RWMutex
	subscribers  map[string][]EventHandler
	eventHistory []EventBusEvent // 問題: 無制限に成長
	processing   map[string]bool
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers:  make(map[string][]EventHandler),
		eventHistory: make([]EventBusEvent, 0),
		processing:   make(map[string]bool),
	}
}

// 問題1: デッドロックの可能性
func (eb *EventBus) Subscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// 問題: 同じハンドラーが複数回登録される可能性
	eb.subscribers[eventType] = append(eb.subscribers[eventType], handler)
}

// 問題2: イベント配信の信頼性
func (eb *EventBus) Publish(ctx context.Context, event EventBusEvent) error {
	eb.mu.Lock()
	// 問題: イベント履歴が無制限に成長
	eb.eventHistory = append(eb.eventHistory, event)

	// 問題: 重複チェックが不完全
	if eb.processing[event.ID] {
		eb.mu.Unlock()
		return fmt.Errorf("event %s already processing", event.ID)
	}
	eb.processing[event.ID] = true

	handlers := eb.subscribers[event.Type]
	eb.mu.Unlock()

	// 問題3: エラーハンドリング
	var wg sync.WaitGroup
	for _, handler := range handlers {
		wg.Add(1)
		go func(h EventHandler) {
			defer wg.Done()
			// 問題: パニックリカバリーなし
			// 問題: タイムアウトなし
			h(ctx, event)
		}(handler)
	}

	wg.Wait()

	// 問題: processingフラグのクリーンアップ忘れ
	// eb.processing[event.ID] = false を忘れている

	return nil
}

// 問題4: リソースリーク
func (eb *EventBus) Unsubscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	// 問題: ハンドラーの比較方法が不正確
	// 関数の等価性チェックは動作しない
	handlers := eb.subscribers[eventType]
	for i, h := range handlers {
		// これは動作しない！
		if &h == &handler {
			eb.subscribers[eventType] = append(handlers[:i], handlers[i+1:]...)
			break
		}
	}
}

// 問題5: メモリリーク
func (eb *EventBus) GetHistory() []EventBusEvent {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// 問題: 大きなスライスのコピー
	history := make([]EventBusEvent, len(eb.eventHistory))
	copy(history, eb.eventHistory)
	return history
}

// Challenge: 以下の問題を修正してください
// 1. デッドロックの防止
// 2. イベントの重複配信防止
// 3. メモリリークの修正
// 4. 適切なエラーハンドリング
// 5. グレースフルシャットダウン
// 6. イベント順序の保証（オプション）
// 7. Circuit Breakerパターンの実装
// 8. メトリクスとロギング

func RunChallenge17() {
	fmt.Println("Challenge 17: Event Bus Pattern")
	fmt.Println("Fix the event bus implementation issues")

	eb := NewEventBus()
	ctx := context.Background()

	// サブスクライバー登録
	eb.Subscribe("user.created", func(ctx context.Context, e EventBusEvent) error {
		fmt.Printf("Handler 1: %+v\n", e)
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	eb.Subscribe("user.created", func(ctx context.Context, e EventBusEvent) error {
		fmt.Printf("Handler 2: %+v\n", e)
		// 問題: パニックを起こす可能性
		if e.Payload == nil {
			panic("nil payload")
		}
		return nil
	})

	// イベント発行
	for i := 0; i < 10; i++ {
		event := EventBusEvent{
			ID:        fmt.Sprintf("evt-%d", i),
			Type:      "user.created",
			Payload:   map[string]interface{}{"userID": i},
			Timestamp: time.Now(),
		}

		if err := eb.Publish(ctx, event); err != nil {
			fmt.Printf("Error publishing event: %v\n", err)
		}
	}

	// メモリリークのデモ
	fmt.Printf("Event history size: %d\n", len(eb.GetHistory()))
}