package tests

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestWindowManagement - ウィンドウ管理のテスト
func TestWindowManagement(t *testing.T) {
	t.Run("TumblingWindowWithEviction", func(t *testing.T) {
		type window struct {
			Start  time.Time
			End    time.Time
			Events []int
			Closed bool
		}

		type windowManager struct {
			mu             sync.RWMutex
			windows        map[string]*window
			windowDuration time.Duration
			maxWindows     int
		}

		manager := &windowManager{
			windows:        make(map[string]*window),
			windowDuration: time.Second,
			maxWindows:     10,
		}

		// ウィンドウへのイベント追加
		addEvent := func(eventTime time.Time, value int) {
			windowKey := eventTime.Truncate(manager.windowDuration).Format(time.RFC3339)

			manager.mu.Lock()
			defer manager.mu.Unlock()

			w, exists := manager.windows[windowKey]
			if !exists {
				w = &window{
					Start:  eventTime.Truncate(manager.windowDuration),
					End:    eventTime.Truncate(manager.windowDuration).Add(manager.windowDuration),
					Events: []int{},
					Closed: false,
				}
				manager.windows[windowKey] = w
			}

			if !w.Closed {
				w.Events = append(w.Events, value)
			}
		}

		// 古いウィンドウの削除
		evictOldWindows := func(currentTime time.Time) int {
			manager.mu.Lock()
			defer manager.mu.Unlock()

			keysToEvict := []string{}
			for key, w := range manager.windows {
				// ウィンドウ終了時刻 + 2倍の期間が過ぎたら削除
				if currentTime.After(w.End.Add(manager.windowDuration * 2)) {
					if !w.Closed {
						w.Closed = true
					}
					keysToEvict = append(keysToEvict, key)
				}
			}

			for _, key := range keysToEvict {
				delete(manager.windows, key)
			}

			return len(keysToEvict)
		}

		// テスト: イベント追加
		now := time.Now()
		for i := 0; i < 100; i++ {
			eventTime := now.Add(time.Duration(i) * 100 * time.Millisecond)
			addEvent(eventTime, i)
		}

		// 検証: ウィンドウが作成された
		manager.mu.RLock()
		initialWindowCount := len(manager.windows)
		manager.mu.RUnlock()

		if initialWindowCount == 0 {
			t.Error("No windows were created")
		}

		// テスト: 古いウィンドウの削除
		futureTime := now.Add(time.Second * 20)
		evicted := evictOldWindows(futureTime)

		if evicted == 0 {
			t.Error("No windows were evicted")
		}

		// 検証: メモリリークが防止された
		manager.mu.RLock()
		finalWindowCount := len(manager.windows)
		manager.mu.RUnlock()

		if finalWindowCount >= initialWindowCount {
			t.Errorf("Windows were not properly evicted: initial=%d, final=%d",
				initialWindowCount, finalWindowCount)
		}

		t.Logf("✅ Window eviction: initial=%d, evicted=%d, final=%d",
			initialWindowCount, evicted, finalWindowCount)
	})
}

// TestWatermarkProcessing - Watermark処理のテスト
func TestWatermarkProcessing(t *testing.T) {
	t.Run("LateDataHandling", func(t *testing.T) {
		type streamEvent struct {
			ID        string
			Timestamp time.Time
			Value     int
		}

		type watermarkProcessor struct {
			watermark       atomic.Value // time.Time
			allowedLateness time.Duration
			onTimeEvents    []streamEvent
			lateEvents      []streamEvent
			tooLateEvents   []streamEvent
			mu              sync.Mutex
		}

		processor := &watermarkProcessor{
			allowedLateness: 5 * time.Second,
			onTimeEvents:    []streamEvent{},
			lateEvents:      []streamEvent{},
			tooLateEvents:   []streamEvent{},
		}
		processor.watermark.Store(time.Now())

		// Watermark更新
		updateWatermark := func(eventTime time.Time) {
			currentWatermark := processor.watermark.Load().(time.Time)
			if eventTime.After(currentWatermark) {
				processor.watermark.Store(eventTime)
			}
		}

		// イベント処理
		processEvent := func(event streamEvent) (accepted bool, reason string) {
			currentWatermark := processor.watermark.Load().(time.Time)
			lateness := currentWatermark.Sub(event.Timestamp)

			processor.mu.Lock()
			defer processor.mu.Unlock()

			// ケース1: On-timeイベント
			if event.Timestamp.After(currentWatermark) {
				processor.onTimeEvents = append(processor.onTimeEvents, event)
				return true, "on-time"
			}

			// ケース2: 許容範囲内の遅延
			if lateness <= processor.allowedLateness {
				processor.lateEvents = append(processor.lateEvents, event)
				return true, "late-accepted"
			}

			// ケース3: 許容範囲外の遅延
			processor.tooLateEvents = append(processor.tooLateEvents, event)
			return false, "too-late"
		}

		// テスト: イベント処理
		baseTime := time.Now()

		testCases := []struct {
			name          string
			eventTime     time.Time
			expectedType  string
		}{
			{"On-time event", baseTime.Add(time.Second), "on-time"},
			{"Late within threshold", baseTime.Add(-3 * time.Second), "late-accepted"},
			{"Too late event", baseTime.Add(-10 * time.Second), "too-late"},
		}

		// Watermarkを現在時刻に設定
		updateWatermark(baseTime)

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				event := streamEvent{
					ID:        tc.name,
					Timestamp: tc.eventTime,
					Value:     100,
				}

				accepted, reason := processEvent(event)

				if reason != tc.expectedType {
					t.Errorf("Expected %s, got %s", tc.expectedType, reason)
				}

				if tc.expectedType == "too-late" && accepted {
					t.Error("Too-late event should not be accepted")
				}

				if tc.expectedType != "too-late" && !accepted {
					t.Error("Event should be accepted")
				}
			})
		}

		// 検証: 分類が正確
		processor.mu.Lock()
		onTimeCount := len(processor.onTimeEvents)
		lateCount := len(processor.lateEvents)
		tooLateCount := len(processor.tooLateEvents)
		processor.mu.Unlock()

		t.Logf("✅ Events classified: on-time=%d, late-accepted=%d, too-late=%d",
			onTimeCount, lateCount, tooLateCount)

		if onTimeCount == 0 {
			t.Error("No on-time events were processed")
		}
		if lateCount == 0 {
			t.Error("No late events were accepted")
		}
		if tooLateCount == 0 {
			t.Error("No too-late events were rejected")
		}
	})
}

// TestCheckpointRecovery - チェックポイント/リカバリのテスト
func TestCheckpointRecovery(t *testing.T) {
	t.Run("ExactlyOnceProcessing", func(t *testing.T) {
		type checkpoint struct {
			Offset       int64
			ProcessedIDs map[string]bool
			WindowStates map[string]int64
		}

		type checkpointManager struct {
			mu                sync.RWMutex
			currentCheckpoint *checkpoint
			checkpoints       []*checkpoint
		}

		manager := &checkpointManager{
			currentCheckpoint: &checkpoint{
				Offset:       0,
				ProcessedIDs: make(map[string]bool),
				WindowStates: make(map[string]int64),
			},
			checkpoints: []*checkpoint{},
		}

		// イベント処理（冪等性保証）
		processEvent := func(eventID string, offset int64, windowKey string, value int64) bool {
			manager.mu.Lock()
			defer manager.mu.Unlock()

			// 重複チェック
			if manager.currentCheckpoint.ProcessedIDs[eventID] {
				return false // スキップ（既に処理済み）
			}

			// イベント処理
			manager.currentCheckpoint.WindowStates[windowKey] += value
			manager.currentCheckpoint.ProcessedIDs[eventID] = true
			manager.currentCheckpoint.Offset = offset

			return true
		}

		// チェックポイント保存
		saveCheckpoint := func() *checkpoint {
			manager.mu.Lock()
			defer manager.mu.Unlock()

			// ディープコピー
			cp := &checkpoint{
				Offset:       manager.currentCheckpoint.Offset,
				ProcessedIDs: make(map[string]bool),
				WindowStates: make(map[string]int64),
			}

			for k, v := range manager.currentCheckpoint.ProcessedIDs {
				cp.ProcessedIDs[k] = v
			}
			for k, v := range manager.currentCheckpoint.WindowStates {
				cp.WindowStates[k] = v
			}

			manager.checkpoints = append(manager.checkpoints, cp)
			return cp
		}

		// リカバリ
		recover := func(cp *checkpoint) {
			manager.mu.Lock()
			defer manager.mu.Unlock()

			manager.currentCheckpoint = &checkpoint{
				Offset:       cp.Offset,
				ProcessedIDs: make(map[string]bool),
				WindowStates: make(map[string]int64),
			}

			for k, v := range cp.ProcessedIDs {
				manager.currentCheckpoint.ProcessedIDs[k] = v
			}
			for k, v := range cp.WindowStates {
				manager.currentCheckpoint.WindowStates[k] = v
			}
		}

		// テスト: イベント処理
		events := []struct {
			id     string
			offset int64
			window string
			value  int64
		}{
			{"event1", 1, "window1", 100},
			{"event2", 2, "window1", 200},
			{"event3", 3, "window2", 150},
			{"event1", 1, "window1", 100}, // 重複
		}

		processedCount := 0
		for _, e := range events {
			if processEvent(e.id, e.offset, e.window, e.value) {
				processedCount++
			}
		}

		// 検証: 重複が除外された
		if processedCount != 3 {
			t.Errorf("Expected 3 unique events processed, got %d", processedCount)
		}

		// チェックポイント保存
		cp := saveCheckpoint()

		// 検証: チェックポイントの内容
		if cp.Offset != 3 {
			t.Errorf("Expected offset 3, got %d", cp.Offset)
		}
		if len(cp.ProcessedIDs) != 3 {
			t.Errorf("Expected 3 processed IDs, got %d", len(cp.ProcessedIDs))
		}

		// 新しいマネージャーでリカバリ
		newManager := &checkpointManager{}
		newManager.currentCheckpoint = &checkpoint{
			Offset:       0,
			ProcessedIDs: make(map[string]bool),
			WindowStates: make(map[string]int64),
		}
		manager = newManager
		recover(cp)

		// 検証: リカバリ後の状態
		manager.mu.RLock()
		recoveredOffset := manager.currentCheckpoint.Offset
		recoveredIDs := len(manager.currentCheckpoint.ProcessedIDs)
		manager.mu.RUnlock()

		if recoveredOffset != 3 {
			t.Errorf("Expected recovered offset 3, got %d", recoveredOffset)
		}
		if recoveredIDs != 3 {
			t.Errorf("Expected 3 recovered IDs, got %d", recoveredIDs)
		}

		// 検証: リカバリ後の重複除外
		duplicate := processEvent("event1", 1, "window1", 100)
		if duplicate {
			t.Error("Duplicate event was processed after recovery")
		}

		t.Log("✅ Exactly-once processing with checkpoint/recovery verified")
	})
}

// BenchmarkWindowProcessing - ウィンドウ処理のベンチマーク
func BenchmarkWindowProcessing(b *testing.B) {
	type window struct {
		Events []int
		mu     sync.Mutex
	}

	windows := make(map[int]*window)
	windowsMu := sync.RWMutex{}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			windowID := i % 10
			i++

			windowsMu.RLock()
			w, exists := windows[windowID]
			windowsMu.RUnlock()

			if !exists {
				windowsMu.Lock()
				w, exists = windows[windowID]
				if !exists {
					w = &window{Events: []int{}}
					windows[windowID] = w
				}
				windowsMu.Unlock()
			}

			w.mu.Lock()
			w.Events = append(w.Events, i)
			w.mu.Unlock()
		}
	})
}

// BenchmarkWatermarkUpdate - Watermark更新のベンチマーク
func BenchmarkWatermarkUpdate(b *testing.B) {
	var watermark atomic.Value
	watermark.Store(time.Now())

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			newTime := time.Now()
			currentWatermark := watermark.Load().(time.Time)
			if newTime.After(currentWatermark) {
				watermark.Store(newTime)
			}
		}
	})
}

// TestWatermarkMonotonicity - Watermark単調増加のテスト
func TestWatermarkMonotonicity(t *testing.T) {
	var watermark atomic.Value
	baseTime := time.Now()
	watermark.Store(baseTime)

	var wg sync.WaitGroup
	var violations atomic.Int64

	// 並行でWatermark更新
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()

			newTime := baseTime.Add(time.Duration(offset) * time.Millisecond)

			for {
				current := watermark.Load().(time.Time)
				if newTime.After(current) {
					if watermark.CompareAndSwap(current, newTime) {
						break
					}
				} else {
					// 単調性違反
					violations.Add(1)
					break
				}
			}
		}(i)
	}

	wg.Wait()

	// 検証: 最終的なwatermarkが最大値
	finalWatermark := watermark.Load().(time.Time)
	expectedMax := baseTime.Add(99 * time.Millisecond)

	if !finalWatermark.Equal(expectedMax) && !finalWatermark.After(expectedMax) {
		t.Errorf("Final watermark not at maximum: got %v, expected >= %v",
			finalWatermark, expectedMax)
	}

	t.Logf("✅ Watermark monotonicity maintained (violations: %d)", violations.Load())
}

// TestStreamProcessingIntegration - ストリーム処理統合テスト
func TestStreamProcessingIntegration(t *testing.T) {
	t.Run("EndToEndStreamPipeline", func(t *testing.T) {
		// ウィンドウ、Watermark、チェックポイントの統合
		type streamEvent struct {
			ID        string
			Timestamp time.Time
			Value     int
		}

		var (
			watermark       atomic.Value
			processedEvents sync.Map
			windowResults   sync.Map
		)

		baseTime := time.Now()
		watermark.Store(baseTime)

		// イベント生成
		events := []streamEvent{}
		for i := 0; i < 1000; i++ {
			events = append(events, streamEvent{
				ID:        fmt.Sprintf("event_%d", i),
				Timestamp: baseTime.Add(time.Duration(i) * time.Millisecond),
				Value:     i,
			})
		}

		// ストリーム処理
		var wg sync.WaitGroup
		for _, event := range events {
			wg.Add(1)
			go func(e streamEvent) {
				defer wg.Done()

				// 重複チェック
				if _, exists := processedEvents.LoadOrStore(e.ID, true); exists {
					return
				}

				// Watermark更新
				current := watermark.Load().(time.Time)
				if e.Timestamp.After(current) {
					watermark.Store(e.Timestamp)
				}

				// ウィンドウ集計
				windowKey := e.Timestamp.Truncate(time.Second).Unix()
				actual, _ := windowResults.LoadOrStore(windowKey, new(int64))
				counter := actual.(*int64)
				atomic.AddInt64(counter, int64(e.Value))
			}(event)
		}

		wg.Wait()

		// 検証: 全イベントが処理された
		processedCount := 0
		processedEvents.Range(func(key, value interface{}) bool {
			processedCount++
			return true
		})

		if processedCount != 1000 {
			t.Errorf("Expected 1000 events processed, got %d", processedCount)
		}

		// 検証: ウィンドウ結果が生成された
		windowCount := 0
		windowResults.Range(func(key, value interface{}) bool {
			windowCount++
			return true
		})

		if windowCount == 0 {
			t.Error("No window results were generated")
		}

		t.Logf("✅ Stream pipeline processed %d events into %d windows",
			processedCount, windowCount)
	})
}

func repeatString(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}

// ヘルパー関数
func sortInt64Slice(slice []int64) {
	sort.Slice(slice, func(i, j int) bool {
		return slice[i] < slice[j]
	})
}