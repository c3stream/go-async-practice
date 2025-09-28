package challenges

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge 16: ストリーム処理の問題
//
// 問題点:
// 1. ウィンドウ処理のメモリリーク
// 2. 遅延データの処理不備
// 3. チェックポイント機能の欠如
// 4. 背圧制御（バックプレッシャー）の不備

type StreamEvent struct {
	ID        string
	Data      interface{}
	Timestamp time.Time
	Partition int
}

type Window struct {
	Start  time.Time
	End    time.Time
	Events []StreamEvent
	mu     sync.RWMutex
}

type StreamProcessor struct {
	inputChan        chan StreamEvent
	windows          map[string]*Window
	windowDuration   time.Duration
	watermark        time.Time
	processedCount   int64
	droppedCount     int64
	checkpointMu     sync.RWMutex
	lastCheckpoint   time.Time
	windowsMu        sync.RWMutex
}

// BUG: ウィンドウ処理のメモリリーク
func (sp *StreamProcessor) ProcessStream(ctx context.Context) error {
	for {
		select {
		case event := <-sp.inputChan:
			windowKey := sp.getWindowKey(event.Timestamp)

			sp.windowsMu.Lock()
			window, exists := sp.windows[windowKey]
			if !exists {
				// 問題: 古いウィンドウが削除されない（メモリリーク）
				window = &Window{
					Start:  sp.getWindowStart(event.Timestamp),
					End:    sp.getWindowEnd(event.Timestamp),
					Events: make([]StreamEvent, 0),
				}
				sp.windows[windowKey] = window
			}
			sp.windowsMu.Unlock()

			// 問題: ウィンドウサイズの制限なし
			window.mu.Lock()
			window.Events = append(window.Events, event)
			window.mu.Unlock()

			atomic.AddInt64(&sp.processedCount, 1)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// BUG: 遅延データの処理不備
func (sp *StreamProcessor) HandleLateData(event StreamEvent) {
	// 問題: watermarkの更新なし
	if event.Timestamp.Before(sp.watermark) {
		// 問題: 単純に破棄（再処理やアラートなし）
		atomic.AddInt64(&sp.droppedCount, 1)
		return
	}

	// 問題: 競合状態 - inputChanがブロックされる可能性
	sp.inputChan <- event
}

// BUG: チェックポイント機能の欠如
func (sp *StreamProcessor) Checkpoint() error {
	// 問題: 状態の永続化なし
	sp.checkpointMu.Lock()
	sp.lastCheckpoint = time.Now()
	sp.checkpointMu.Unlock()

	// 問題: 処理中のイベントの状態が保存されない
	// 障害時に重複処理やデータロスの可能性

	return nil
}

// BUG: 背圧制御の不備
func (sp *StreamProcessor) IngestData(ctx context.Context, source <-chan StreamEvent) {
	for {
		select {
		case event := <-source:
			// 問題: バッファサイズのチェックなし
			// inputChanがフルでもブロックして待つ
			sp.inputChan <- event

		case <-ctx.Done():
			return
		}
	}
}

// BUG: ウィンドウ集計の並行性問題
func (sp *StreamProcessor) AggregateWindow(windowKey string) (interface{}, error) {
	sp.windowsMu.RLock()
	window, exists := sp.windows[windowKey]
	sp.windowsMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("window not found: %s", windowKey)
	}

	// 問題: 集計中にウィンドウが変更される可能性
	var sum float64
	for _, event := range window.Events {
		if val, ok := event.Data.(float64); ok {
			sum += val
		}
		// 問題: 集計処理が長い場合、ロックなしでアクセス
		time.Sleep(time.Millisecond)
	}

	return sum, nil
}

// BUG: パーティション処理の不均衡
func (sp *StreamProcessor) ProcessPartitioned(ctx context.Context, numPartitions int) {
	partitions := make([]chan StreamEvent, numPartitions)
	for i := range partitions {
		partitions[i] = make(chan StreamEvent, 100)
	}

	// 問題: 固定ワーカー数（動的スケーリングなし）
	for i := 0; i < numPartitions; i++ {
		go func(partition int) {
			for event := range partitions[partition] {
				// 問題: パーティションごとの処理速度の違いを考慮しない
				time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
				sp.ProcessEvent(event)
			}
		}(i)
	}

	// イベント振り分け
	for {
		select {
		case event := <-sp.inputChan:
			// 問題: ホットパーティション（データの偏り）を考慮しない
			partition := event.Partition % numPartitions
			partitions[partition] <- event
		case <-ctx.Done():
			return
		}
	}
}

func (sp *StreamProcessor) ProcessEvent(event StreamEvent) {
	// イベント処理のシミュレーション
	atomic.AddInt64(&sp.processedCount, 1)
}

func (sp *StreamProcessor) getWindowKey(t time.Time) string {
	return fmt.Sprintf("window-%d", t.Unix()/int64(sp.windowDuration.Seconds()))
}

func (sp *StreamProcessor) getWindowStart(t time.Time) time.Time {
	seconds := t.Unix() / int64(sp.windowDuration.Seconds()) * int64(sp.windowDuration.Seconds())
	return time.Unix(seconds, 0)
}

func (sp *StreamProcessor) getWindowEnd(t time.Time) time.Time {
	return sp.getWindowStart(t).Add(sp.windowDuration)
}

func Challenge16_StreamProcessingProblem() {
	fmt.Println("Challenge 16: ストリーム処理の問題")
	fmt.Println("問題:")
	fmt.Println("1. ウィンドウ処理のメモリリーク")
	fmt.Println("2. 遅延データの処理不備")
	fmt.Println("3. チェックポイント機能の欠如")
	fmt.Println("4. 背圧制御（バックプレッシャー）の不備")

	processor := &StreamProcessor{
		inputChan:      make(chan StreamEvent, 1000),
		windows:        make(map[string]*Window),
		windowDuration: 10 * time.Second,
		watermark:      time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// ストリーム処理開始
	go processor.ProcessStream(ctx)

	// データ生成
	go func() {
		for i := 0; i < 1000; i++ {
			event := StreamEvent{
				ID:        fmt.Sprintf("event-%d", i),
				Data:      rand.Float64() * 100,
				Timestamp: time.Now().Add(time.Duration(rand.Intn(20)-10) * time.Second),
				Partition: rand.Intn(4),
			}

			// 遅延データのシミュレーション
			if rand.Float32() < 0.1 {
				processor.HandleLateData(event)
			} else {
				processor.inputChan <- event
			}

			time.Sleep(5 * time.Millisecond)
		}
	}()

	// 定期的なチェックポイント
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				processor.Checkpoint()
				fmt.Printf("処理済み: %d, 破棄: %d, ウィンドウ数: %d\n",
					atomic.LoadInt64(&processor.processedCount),
					atomic.LoadInt64(&processor.droppedCount),
					len(processor.windows))
			case <-ctx.Done():
				return
			}
		}
	}()

	<-ctx.Done()

	fmt.Println("\n修正が必要な箇所を特定してください")
}