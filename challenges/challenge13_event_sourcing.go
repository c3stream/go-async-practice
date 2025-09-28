package challenges

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Challenge 13: イベントソーシングの問題
//
// 問題点:
// 1. イベントの順序が保証されていない
// 2. スナップショットが適切に作成されていない
// 3. イベントリプレイ時のメモリ使用量が制御されていない
// 4. 同時実行制御が不適切

type Event struct {
	ID        string
	Type      string
	Data      json.RawMessage
	Timestamp time.Time
	Version   int64
}

type EventStore struct {
	mu       sync.RWMutex
	events   []Event
	snapshot interface{}
}

// BUG: イベントの順序保証がない
func (es *EventStore) AppendEvent(event Event) error {
	// 問題: 並行アクセス時にイベントの順序が崩れる
	go func() {
		es.mu.Lock()
		defer es.mu.Unlock()
		es.events = append(es.events, event)
	}()
	return nil
}

// BUG: スナップショット作成が非効率
func (es *EventStore) CreateSnapshot() error {
	es.mu.RLock()
	defer es.mu.RUnlock()

	// 問題: 全イベントを毎回リプレイ
	var state interface{}
	for _, event := range es.events {
		// 重い処理
		time.Sleep(10 * time.Millisecond)
		state = applyEvent(state, event)
	}

	// 問題: スナップショット作成中に新しいイベントが追加される可能性
	es.snapshot = state
	return nil
}

// BUG: メモリ使用量が制御されていない
func (es *EventStore) ReplayEvents(ctx context.Context, fromVersion int64) (interface{}, error) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	// 問題: 大量のイベントを一度にメモリに載せる
	var allEvents []Event
	for _, event := range es.events {
		if event.Version >= fromVersion {
			allEvents = append(allEvents, event)
		}
	}

	// 問題: バッチ処理なし
	state := es.snapshot
	for _, event := range allEvents {
		state = applyEvent(state, event)
	}

	return state, nil
}

// BUG: 同時実行制御が不適切
func (es *EventStore) GetEventStream(aggregateID string) ([]Event, error) {
	// 問題: Read lockなし
	var result []Event
	for _, event := range es.events {
		if event.ID == aggregateID {
			result = append(result, event)
		}
	}
	return result, nil
}

func applyEvent(state interface{}, event Event) interface{} {
	// イベント適用のシミュレーション
	return state
}

func Challenge13_EventSourcingProblem() {
	fmt.Println("Challenge 13: イベントソーシングの問題")
	fmt.Println("問題:")
	fmt.Println("1. イベントの順序が保証されていない")
	fmt.Println("2. スナップショットが適切に作成されていない")
	fmt.Println("3. メモリ使用量が制御されていない")
	fmt.Println("4. 同時実行制御が不適切")

	store := &EventStore{
		events: make([]Event, 0),
	}

	// 並行でイベントを追加
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			event := Event{
				ID:        fmt.Sprintf("agg-%d", id%10),
				Type:      "updated",
				Timestamp: time.Now(),
				Version:   int64(id),
			}
			store.AppendEvent(event)
		}(i)
	}

	wg.Wait()

	// スナップショット作成
	go store.CreateSnapshot()

	// イベントリプレイ
	ctx := context.Background()
	state, _ := store.ReplayEvents(ctx, 50)

	fmt.Printf("最終状態: %v\n", state)
	fmt.Println("\n修正が必要な箇所を特定してください")
}