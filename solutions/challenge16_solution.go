package solutions

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge16_StreamProcessingSolution - ストリーム処理問題の解決
func Challenge16_StreamProcessingSolution() {
	fmt.Println("\n✅ チャレンジ16: ストリーム処理問題の解決")
	fmt.Println("===================================================")

	// 解決策1: ウィンドウ管理とメモリリーク対策
	solution1_WindowManagement()

	// 解決策2: 遅延データ処理（Watermark）
	solution2_LateDataHandling()

	// 解決策3: チェックポイントとリカバリ
	solution3_CheckpointRecovery()
}

// 解決策1: ウィンドウ管理とメモリリーク対策
func solution1_WindowManagement() {
	fmt.Println("\n📝 解決策1: ウィンドウ管理とメモリリーク対策")

	type StreamEvent struct {
		ID        string
		Data      interface{}
		Timestamp time.Time
	}

	type Window struct {
		Start      time.Time
		End        time.Time
		Events     []StreamEvent
		Closed     bool
		EventCount int64
		mu         sync.RWMutex
	}

	type WindowManager struct {
		mu             sync.RWMutex
		windows        map[string]*Window
		windowDuration time.Duration
		maxWindows     int
		closedWindows  []string
	}

	manager := &WindowManager{
		windows:        make(map[string]*Window),
		windowDuration: 5 * time.Second,
		maxWindows:     10, // メモリ制限
		closedWindows:  make([]string, 0),
	}

	// ウィンドウキーの生成
	getWindowKey := func(timestamp time.Time) string {
		windowStart := timestamp.Truncate(manager.windowDuration)
		return windowStart.Format(time.RFC3339)
	}

	// ウィンドウの取得または作成
	getOrCreateWindow := func(timestamp time.Time) *Window {
		key := getWindowKey(timestamp)

		manager.mu.RLock()
		window, exists := manager.windows[key]
		manager.mu.RUnlock()

		if exists {
			return window
		}

		// 新しいウィンドウを作成
		manager.mu.Lock()
		defer manager.mu.Unlock()

		// ダブルチェック
		if window, exists := manager.windows[key]; exists {
			return window
		}

		windowStart := timestamp.Truncate(manager.windowDuration)
		window = &Window{
			Start:  windowStart,
			End:    windowStart.Add(manager.windowDuration),
			Events: make([]StreamEvent, 0, 100), // 初期容量設定
			Closed: false,
		}

		manager.windows[key] = window

		fmt.Printf("  📦 新しいウィンドウ作成: %s - %s\n",
			window.Start.Format("15:04:05"),
			window.End.Format("15:04:05"))

		return window
	}

	// イベント追加
	addEvent := func(event StreamEvent) bool {
		window := getOrCreateWindow(event.Timestamp)

		window.mu.Lock()
		defer window.mu.Unlock()

		if window.Closed {
			fmt.Printf("  ⚠ イベント拒否: ウィンドウクローズ済み\n")
			return false
		}

		// ウィンドウサイズ制限チェック
		if len(window.Events) >= 1000 {
			fmt.Printf("  ⚠ イベント拒否: ウィンドウ容量超過\n")
			return false
		}

		window.Events = append(window.Events, event)
		window.EventCount++

		return true
	}

	// 古いウィンドウのクローズと削除（メモリリーク対策）
	closeAndEvictOldWindows := func(currentTime time.Time) {
		manager.mu.Lock()
		defer manager.mu.Unlock()

		var keysToEvict []string

		for key, window := range manager.windows {
			window.mu.Lock()

			// ウィンドウ終了時刻を過ぎている場合
			if currentTime.After(window.End.Add(manager.windowDuration)) {
				if !window.Closed {
					window.Closed = true
					manager.closedWindows = append(manager.closedWindows, key)

					fmt.Printf("  ✓ ウィンドウクローズ: %s (イベント数: %d)\n",
						window.Start.Format("15:04:05"), window.EventCount)
				}

				// さらに古いウィンドウは削除候補
				if currentTime.After(window.End.Add(manager.windowDuration * 2)) {
					keysToEvict = append(keysToEvict, key)
				}
			}

			window.mu.Unlock()
		}

		// 古いウィンドウを削除
		for _, key := range keysToEvict {
			delete(manager.windows, key)
			fmt.Printf("  🗑️  ウィンドウ削除: %s\n", key)
		}

		// ウィンドウ数制限チェック
		if len(manager.windows) > manager.maxWindows {
			fmt.Printf("  ⚠ ウィンドウ数超過: %d/%d\n", len(manager.windows), manager.maxWindows)
		}
	}

	// テスト実行
	fmt.Println("\n実行: ウィンドウ管理テスト")

	baseTime := time.Now()
	events := []StreamEvent{
		{ID: "1", Data: 100, Timestamp: baseTime},
		{ID: "2", Data: 200, Timestamp: baseTime.Add(2 * time.Second)},
		{ID: "3", Data: 300, Timestamp: baseTime.Add(6 * time.Second)},  // 次のウィンドウ
		{ID: "4", Data: 400, Timestamp: baseTime.Add(12 * time.Second)}, // さらに次のウィンドウ
	}

	for _, event := range events {
		addEvent(event)
		time.Sleep(50 * time.Millisecond)
	}

	// 古いウィンドウをクローズ
	closeAndEvictOldWindows(baseTime.Add(20 * time.Second))

	fmt.Printf("\n  ✅ アクティブウィンドウ数: %d\n", len(manager.windows))
	fmt.Printf("  ✅ クローズ済みウィンドウ数: %d\n", len(manager.closedWindows))
}

// 解決策2: 遅延データ処理（Watermark）
func solution2_LateDataHandling() {
	fmt.Println("\n📝 解決策2: 遅延データ処理（Watermark）")

	type StreamEvent struct {
		ID        string
		Data      interface{}
		Timestamp time.Time
	}

	type WatermarkProcessor struct {
		watermark       atomic.Value // time.Time
		allowedLateness time.Duration
		lateEvents      []StreamEvent
		onTimeEvents    []StreamEvent
		mu              sync.RWMutex
	}

	processor := &WatermarkProcessor{
		allowedLateness: 3 * time.Second,
		lateEvents:      make([]StreamEvent, 0),
		onTimeEvents:    make([]StreamEvent, 0),
	}

	// 初期watermark設定
	processor.watermark.Store(time.Now())

	// Watermark更新
	updateWatermark := func(eventTime time.Time) {
		currentWatermark := processor.watermark.Load().(time.Time)

		// Watermarkは単調増加（過去に戻らない）
		if eventTime.After(currentWatermark) {
			processor.watermark.Store(eventTime)
			fmt.Printf("  💧 Watermark更新: %s\n", eventTime.Format("15:04:05.000"))
		}
	}

	// イベント処理
	processEvent := func(event StreamEvent) (accepted bool, reason string) {
		currentWatermark := processor.watermark.Load().(time.Time)
		lateness := currentWatermark.Sub(event.Timestamp)

		processor.mu.Lock()
		defer processor.mu.Unlock()

		// ケース1: On-timeイベント
		if event.Timestamp.After(currentWatermark) || event.Timestamp.Equal(currentWatermark) {
			processor.onTimeEvents = append(processor.onTimeEvents, event)
			fmt.Printf("  ✓ On-time: %s (lateness: %v)\n", event.ID, lateness)
			return true, "on-time"
		}

		// ケース2: 許容範囲内の遅延
		if lateness <= processor.allowedLateness {
			processor.lateEvents = append(processor.lateEvents, event)
			fmt.Printf("  ⏰ Late (accepted): %s (lateness: %v)\n", event.ID, lateness)
			return true, "late-accepted"
		}

		// ケース3: 許容範囲外の遅延
		fmt.Printf("  ❌ Late (dropped): %s (lateness: %v > %v)\n",
			event.ID, lateness, processor.allowedLateness)
		return false, "too-late"
	}

	// サイドアウトプット処理（遅延イベント専用処理）
	processSideOutput := func() {
		processor.mu.RLock()
		lateCount := len(processor.lateEvents)
		processor.mu.RUnlock()

		if lateCount > 0 {
			fmt.Printf("  📤 サイドアウトプット: %d件の遅延イベントを別パイプラインへ\n", lateCount)
		}
	}

	// テスト実行
	fmt.Println("\n実行: 遅延データ処理テスト")

	baseTime := time.Now()

	// Watermarkを進める
	updateWatermark(baseTime)

	events := []StreamEvent{
		{ID: "event_1", Data: 100, Timestamp: baseTime.Add(1 * time.Second)},
		{ID: "event_2", Data: 200, Timestamp: baseTime.Add(2 * time.Second)},
		{ID: "event_3", Data: 300, Timestamp: baseTime.Add(-2 * time.Second)}, // 2秒遅延
		{ID: "event_4", Data: 400, Timestamp: baseTime.Add(-5 * time.Second)}, // 5秒遅延（超過）
	}

	// Watermarkを最新イベント時刻に更新
	updateWatermark(baseTime.Add(2 * time.Second))

	// イベント処理
	for _, event := range events {
		processEvent(event)
		time.Sleep(50 * time.Millisecond)
	}

	// サイドアウトプット処理
	processSideOutput()

	processor.mu.RLock()
	fmt.Printf("\n  ✅ On-timeイベント: %d件\n", len(processor.onTimeEvents))
	fmt.Printf("  ✅ 遅延イベント（処理済み）: %d件\n", len(processor.lateEvents))
	processor.mu.RUnlock()
}

// 解決策3: チェックポイントとリカバリ
func solution3_CheckpointRecovery() {
	fmt.Println("\n📝 解決策3: チェックポイントとリカバリ")

	type StreamEvent struct {
		ID        string
		Offset    int64
		Data      interface{}
		Timestamp time.Time
	}

	type Checkpoint struct {
		Offset        int64
		ProcessedIDs  map[string]bool
		WindowStates  map[string]int64 // ウィンドウごとの集計値
		CheckpointTime time.Time
	}

	type CheckpointManager struct {
		mu               sync.RWMutex
		currentCheckpoint *Checkpoint
		checkpointHistory []*Checkpoint
		maxHistory       int
		processedCount   atomic.Int64
	}

	manager := &CheckpointManager{
		currentCheckpoint: &Checkpoint{
			Offset:       0,
			ProcessedIDs: make(map[string]bool),
			WindowStates: make(map[string]int64),
		},
		checkpointHistory: make([]*Checkpoint, 0),
		maxHistory:       5,
	}

	// イベント処理（冪等性保証）
	processEvent := func(event StreamEvent) bool {
		manager.mu.Lock()
		defer manager.mu.Unlock()

		// 重複チェック（exactly-once保証）
		if manager.currentCheckpoint.ProcessedIDs[event.ID] {
			fmt.Printf("  ⏭️  スキップ: %s (重複)\n", event.ID)
			return false
		}

		// イベント処理
		windowKey := event.Timestamp.Truncate(5 * time.Second).Format(time.RFC3339)
		if value, ok := event.Data.(int); ok {
			manager.currentCheckpoint.WindowStates[windowKey] += int64(value)
		}

		// 処理済みマーク
		manager.currentCheckpoint.ProcessedIDs[event.ID] = true
		manager.currentCheckpoint.Offset = event.Offset

		manager.processedCount.Add(1)

		fmt.Printf("  ✓ 処理: %s (offset: %d)\n", event.ID, event.Offset)
		return true
	}

	// チェックポイント作成
	createCheckpoint := func() *Checkpoint {
		manager.mu.Lock()
		defer manager.mu.Unlock()

		// 現在の状態をスナップショット
		checkpoint := &Checkpoint{
			Offset:         manager.currentCheckpoint.Offset,
			ProcessedIDs:   make(map[string]bool),
			WindowStates:   make(map[string]int64),
			CheckpointTime: time.Now(),
		}

		// ディープコピー
		for k, v := range manager.currentCheckpoint.ProcessedIDs {
			checkpoint.ProcessedIDs[k] = v
		}
		for k, v := range manager.currentCheckpoint.WindowStates {
			checkpoint.WindowStates[k] = v
		}

		// 履歴に追加
		manager.checkpointHistory = append(manager.checkpointHistory, checkpoint)

		// 履歴サイズ制限
		if len(manager.checkpointHistory) > manager.maxHistory {
			manager.checkpointHistory = manager.checkpointHistory[1:]
		}

		fmt.Printf("  💾 チェックポイント作成: offset=%d, processed=%d, windows=%d\n",
			checkpoint.Offset, len(checkpoint.ProcessedIDs), len(checkpoint.WindowStates))

		return checkpoint
	}

	// チェックポイントからのリカバリ
	recoverFromCheckpoint := func(checkpoint *Checkpoint) {
		manager.mu.Lock()
		defer manager.mu.Unlock()

		manager.currentCheckpoint = &Checkpoint{
			Offset:       checkpoint.Offset,
			ProcessedIDs: make(map[string]bool),
			WindowStates: make(map[string]int64),
		}

		// ディープコピー
		for k, v := range checkpoint.ProcessedIDs {
			manager.currentCheckpoint.ProcessedIDs[k] = v
		}
		for k, v := range checkpoint.WindowStates {
			manager.currentCheckpoint.WindowStates[k] = v
		}

		fmt.Printf("  🔄 リカバリ完了: offset=%d, processed=%d\n",
			checkpoint.Offset, len(checkpoint.ProcessedIDs))
	}

	// テスト実行
	fmt.Println("\n実行: チェックポイント＆リカバリテスト")

	baseTime := time.Now()
	events := []StreamEvent{
		{ID: "evt_1", Offset: 1, Data: 100, Timestamp: baseTime},
		{ID: "evt_2", Offset: 2, Data: 200, Timestamp: baseTime.Add(1 * time.Second)},
		{ID: "evt_3", Offset: 3, Data: 300, Timestamp: baseTime.Add(2 * time.Second)},
	}

	// イベント処理
	for _, event := range events {
		processEvent(event)
	}

	// チェックポイント作成
	checkpoint1 := createCheckpoint()

	// さらにイベント処理
	moreEvents := []StreamEvent{
		{ID: "evt_4", Offset: 4, Data: 400, Timestamp: baseTime.Add(3 * time.Second)},
		{ID: "evt_5", Offset: 5, Data: 500, Timestamp: baseTime.Add(4 * time.Second)},
	}

	for _, event := range moreEvents {
		processEvent(event)
	}

	fmt.Println("\n  ⚠ 障害発生をシミュレート...")

	// チェックポイントからリカバリ
	recoverFromCheckpoint(checkpoint1)

	// 重複イベント処理テスト（exactly-once保証）
	fmt.Println("\n  🔄 リカバリ後の再処理:")
	processEvent(events[0]) // 重複 - スキップされるべき
	processEvent(events[1]) // 重複 - スキップされるべき
	processEvent(moreEvents[0]) // 新規 - 処理されるべき

	fmt.Printf("\n  ✅ 処理済みイベント数: %d\n", manager.processedCount.Load())

	// ウィンドウ状態の確認
	manager.mu.RLock()
	fmt.Println("  ✅ ウィンドウ集計値:")

	// ソート済みのキーリストを作成
	keys := make([]string, 0, len(manager.currentCheckpoint.WindowStates))
	for k := range manager.currentCheckpoint.WindowStates {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := manager.currentCheckpoint.WindowStates[key]
		fmt.Printf("      %s: %d\n", key, value)
	}
	manager.mu.RUnlock()
}