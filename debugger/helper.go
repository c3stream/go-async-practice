package debugger

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DebugHelper 並行処理のデバッグを支援
type DebugHelper struct {
	mu              sync.RWMutex
	goroutineTraces map[uint64]GoroutineTrace
	channelOps      []ChannelOperation
	mutexOps        []MutexOperation
	deadlockDetector *DeadlockDetector
	enabled         bool
}

// GoroutineTrace ゴルーチンのトレース情報
type GoroutineTrace struct {
	ID         uint64
	Name       string
	StartTime  time.Time
	EndTime    time.Time
	Stack      string
	State      string
	CreatedBy  string
}

// ChannelOperation チャネル操作の記録
type ChannelOperation struct {
	Timestamp   time.Time
	GoroutineID uint64
	Channel     string
	Operation   string // "send", "receive", "close"
	Blocked     bool
	Duration    time.Duration
}

// MutexOperation Mutex操作の記録
type MutexOperation struct {
	Timestamp   time.Time
	GoroutineID uint64
	Mutex       string
	Operation   string // "lock", "unlock", "rlock", "runlock"
	WaitTime    time.Duration
}

// DeadlockDetector デッドロック検出器
type DeadlockDetector struct {
	checkInterval time.Duration
	timeout       time.Duration
	lastActivity  int64 // Unix timestamp
	running       bool
}

// NewDebugHelper デバッグヘルパーを作成
func NewDebugHelper() *DebugHelper {
	dh := &DebugHelper{
		goroutineTraces: make(map[uint64]GoroutineTrace),
		channelOps:      make([]ChannelOperation, 0),
		mutexOps:        make([]MutexOperation, 0),
		enabled:         true,
		deadlockDetector: &DeadlockDetector{
			checkInterval: 1 * time.Second,
			timeout:       5 * time.Second,
		},
	}

	dh.startDeadlockDetector()
	return dh
}

// TraceGoroutine 現在のゴルーチンをトレース
func (dh *DebugHelper) TraceGoroutine(name string) func() {
	if !dh.enabled {
		return func() {}
	}

	gid := getGoroutineID()
	stack := string(debug.Stack())

	dh.mu.Lock()
	dh.goroutineTraces[gid] = GoroutineTrace{
		ID:        gid,
		Name:      name,
		StartTime: time.Now(),
		Stack:     stack,
		State:     "running",
		CreatedBy: extractCreator(stack),
	}
	dh.mu.Unlock()

	// 終了時の処理を返す
	return func() {
		dh.mu.Lock()
		if trace, ok := dh.goroutineTraces[gid]; ok {
			trace.EndTime = time.Now()
			trace.State = "finished"
			dh.goroutineTraces[gid] = trace
		}
		dh.mu.Unlock()
	}
}

// RecordChannelOp チャネル操作を記録
func (dh *DebugHelper) RecordChannelOp(channel, operation string, blocked bool, duration time.Duration) {
	if !dh.enabled {
		return
	}

	atomic.StoreInt64(&dh.deadlockDetector.lastActivity, time.Now().Unix())

	dh.mu.Lock()
	dh.channelOps = append(dh.channelOps, ChannelOperation{
		Timestamp:   time.Now(),
		GoroutineID: getGoroutineID(),
		Channel:     channel,
		Operation:   operation,
		Blocked:     blocked,
		Duration:    duration,
	})
	dh.mu.Unlock()
}

// RecordMutexOp Mutex操作を記録
func (dh *DebugHelper) RecordMutexOp(mutex, operation string, waitTime time.Duration) {
	if !dh.enabled {
		return
	}

	atomic.StoreInt64(&dh.deadlockDetector.lastActivity, time.Now().Unix())

	dh.mu.Lock()
	dh.mutexOps = append(dh.mutexOps, MutexOperation{
		Timestamp:   time.Now(),
		GoroutineID: getGoroutineID(),
		Mutex:       mutex,
		Operation:   operation,
		WaitTime:    waitTime,
	})
	dh.mu.Unlock()
}

// DetectGoroutineLeak ゴルーチンリークを検出
func (dh *DebugHelper) DetectGoroutineLeak() []GoroutineTrace {
	dh.mu.RLock()
	defer dh.mu.RUnlock()

	var leaks []GoroutineTrace
	now := time.Now()

	for _, trace := range dh.goroutineTraces {
		if trace.State == "running" && now.Sub(trace.StartTime) > 10*time.Second {
			leaks = append(leaks, trace)
		}
	}

	return leaks
}

// AnalyzeChannelUsage チャネル使用状況を分析
func (dh *DebugHelper) AnalyzeChannelUsage() map[string]ChannelStats {
	dh.mu.RLock()
	defer dh.mu.RUnlock()

	stats := make(map[string]ChannelStats)

	for _, op := range dh.channelOps {
		s := stats[op.Channel]
		switch op.Operation {
		case "send":
			s.Sends++
			if op.Blocked {
				s.BlockedSends++
				s.TotalBlockTime += op.Duration
			}
		case "receive":
			s.Receives++
			if op.Blocked {
				s.BlockedReceives++
				s.TotalBlockTime += op.Duration
			}
		case "close":
			s.Closes++
		}
		stats[op.Channel] = s
	}

	return stats
}

// ChannelStats チャネル統計情報
type ChannelStats struct {
	Sends           int
	Receives        int
	Closes          int
	BlockedSends    int
	BlockedReceives int
	TotalBlockTime  time.Duration
}

// AnalyzeMutexContention Mutex競合を分析
func (dh *DebugHelper) AnalyzeMutexContention() map[string]MutexStats {
	dh.mu.RLock()
	defer dh.mu.RUnlock()

	stats := make(map[string]MutexStats)

	for _, op := range dh.mutexOps {
		s := stats[op.Mutex]
		switch op.Operation {
		case "lock":
			s.Locks++
			s.TotalWaitTime += op.WaitTime
			if op.WaitTime > s.MaxWaitTime {
				s.MaxWaitTime = op.WaitTime
			}
		case "unlock":
			s.Unlocks++
		case "rlock":
			s.RLocks++
		case "runlock":
			s.RUnlocks++
		}
		stats[op.Mutex] = s
	}

	return stats
}

// MutexStats Mutex統計情報
type MutexStats struct {
	Locks         int
	Unlocks       int
	RLocks        int
	RUnlocks      int
	TotalWaitTime time.Duration
	MaxWaitTime   time.Duration
}

// startDeadlockDetector デッドロック検出器を開始
func (dh *DebugHelper) startDeadlockDetector() {
	dh.deadlockDetector.running = true
	atomic.StoreInt64(&dh.deadlockDetector.lastActivity, time.Now().Unix())

	go func() {
		ticker := time.NewTicker(dh.deadlockDetector.checkInterval)
		defer ticker.Stop()

		for dh.deadlockDetector.running {
			select {
			case <-ticker.C:
				lastActivity := atomic.LoadInt64(&dh.deadlockDetector.lastActivity)
				if time.Since(time.Unix(lastActivity, 0)) > dh.deadlockDetector.timeout {
					dh.reportPotentialDeadlock()
				}
			}
		}
	}()
}

// reportPotentialDeadlock デッドロックの可能性を報告
func (dh *DebugHelper) reportPotentialDeadlock() {
	fmt.Println("\n⚠️  WARNING: Potential deadlock detected!")
	fmt.Printf("No activity for %v\n", dh.deadlockDetector.timeout)

	// 現在のゴルーチン状態を出力
	buf := make([]byte, 1024*64)
	n := runtime.Stack(buf, true)
	fmt.Println("\nCurrent goroutine stacks:")
	fmt.Println(string(buf[:n]))

	// 最近のオペレーションを表示
	dh.mu.RLock()
	defer dh.mu.RUnlock()

	fmt.Println("\nRecent operations:")
	recentOps := 5
	if len(dh.channelOps) > 0 {
		start := len(dh.channelOps) - recentOps
		if start < 0 {
			start = 0
		}
		for _, op := range dh.channelOps[start:] {
			fmt.Printf("  Channel %s: %s by G%d at %v\n",
				op.Channel, op.Operation, op.GoroutineID, op.Timestamp)
		}
	}
}

// PrintReport デバッグレポートを出力
func (dh *DebugHelper) PrintReport() {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🔍 Concurrency Debug Report")
	fmt.Println(strings.Repeat("=", 60))

	// ゴルーチン情報
	dh.mu.RLock()
	fmt.Printf("\n📊 Goroutines: %d tracked, %d currently running\n",
		len(dh.goroutineTraces), runtime.NumGoroutine())

	// 長時間実行中のゴルーチン
	leaks := dh.DetectGoroutineLeak()
	if len(leaks) > 0 {
		fmt.Println("\n⚠️  Potential goroutine leaks:")
		for _, leak := range leaks {
			fmt.Printf("  G%d (%s): running for %v\n",
				leak.ID, leak.Name, time.Since(leak.StartTime))
		}
	}
	dh.mu.RUnlock()

	// チャネル統計
	channelStats := dh.AnalyzeChannelUsage()
	if len(channelStats) > 0 {
		fmt.Println("\n📡 Channel Statistics:")
		for name, stats := range channelStats {
			fmt.Printf("  %s:\n", name)
			fmt.Printf("    Sends: %d (blocked: %d)\n", stats.Sends, stats.BlockedSends)
			fmt.Printf("    Receives: %d (blocked: %d)\n", stats.Receives, stats.BlockedReceives)
			if stats.TotalBlockTime > 0 {
				fmt.Printf("    Total block time: %v\n", stats.TotalBlockTime)
			}
		}
	}

	// Mutex統計
	mutexStats := dh.AnalyzeMutexContention()
	if len(mutexStats) > 0 {
		fmt.Println("\n🔒 Mutex Statistics:")
		for name, stats := range mutexStats {
			fmt.Printf("  %s:\n", name)
			fmt.Printf("    Lock/Unlock: %d/%d\n", stats.Locks, stats.Unlocks)
			if stats.TotalWaitTime > 0 {
				fmt.Printf("    Total wait time: %v\n", stats.TotalWaitTime)
				fmt.Printf("    Max wait time: %v\n", stats.MaxWaitTime)
			}
		}
	}

	fmt.Println(strings.Repeat("=", 60))
}

// getGoroutineID 現在のゴルーチンIDを取得
func getGoroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	var id uint64
	fmt.Sscanf(idField, "%d", &id)
	return id
}

// extractCreator スタックトレースから作成元を抽出
func extractCreator(stack string) string {
	lines := strings.Split(stack, "\n")
	for i, line := range lines {
		if strings.Contains(line, "created by") && i+1 < len(lines) {
			return strings.TrimSpace(lines[i+1])
		}
	}
	return "unknown"
}

// Stop デバッグヘルパーを停止
func (dh *DebugHelper) Stop() {
	dh.deadlockDetector.running = false
	dh.enabled = false
}