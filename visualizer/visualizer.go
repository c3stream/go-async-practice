package visualizer

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Visualizer リアルタイムで並行処理を可視化
type Visualizer struct {
	mu            sync.RWMutex
	goroutines    map[int]GoroutineInfo
	channels      map[string]ChannelInfo
	mutexes       map[string]MutexInfo
	nextID        int32
	running       bool
	updateInterval time.Duration
}

// GoroutineInfo ゴルーチンの情報
type GoroutineInfo struct {
	ID        int
	Name      string
	State     string // "running", "blocked", "sleeping", "finished"
	StartTime time.Time
	EndTime   time.Time
	Operations []string
}

// ChannelInfo チャネルの情報
type ChannelInfo struct {
	Name      string
	Capacity  int
	Current   int
	Sends     int64
	Receives  int64
	Blocked   []int // blocked goroutine IDs
}

// MutexInfo Mutexの情報
type MutexInfo struct {
	Name     string
	Locked   bool
	Owner    int   // goroutine ID
	Waiters  []int // waiting goroutine IDs
	LockCount int64
}

// NewVisualizer 新しいビジュアライザーを作成
func NewVisualizer() *Visualizer {
	return &Visualizer{
		goroutines:     make(map[int]GoroutineInfo),
		channels:       make(map[string]ChannelInfo),
		mutexes:        make(map[string]MutexInfo),
		updateInterval: 100 * time.Millisecond,
	}
}

// StartGoroutine ゴルーチンの開始を記録
func (v *Visualizer) StartGoroutine(name string) int {
	id := int(atomic.AddInt32(&v.nextID, 1))

	v.mu.Lock()
	defer v.mu.Unlock()

	v.goroutines[id] = GoroutineInfo{
		ID:        id,
		Name:      name,
		State:     "running",
		StartTime: time.Now(),
		Operations: []string{},
	}

	return id
}

// EndGoroutine ゴルーチンの終了を記録
func (v *Visualizer) EndGoroutine(id int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if g, ok := v.goroutines[id]; ok {
		g.State = "finished"
		g.EndTime = time.Now()
		v.goroutines[id] = g
	}
}

// RecordChannelSend チャネル送信を記録
func (v *Visualizer) RecordChannelSend(channelName string, goroutineID int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	ch, ok := v.channels[channelName]
	if !ok {
		ch = ChannelInfo{Name: channelName}
	}

	atomic.AddInt64(&ch.Sends, 1)
	ch.Current++
	v.channels[channelName] = ch

	if g, ok := v.goroutines[goroutineID]; ok {
		g.Operations = append(g.Operations, fmt.Sprintf("Send to %s", channelName))
		v.goroutines[goroutineID] = g
	}
}

// RecordChannelReceive チャネル受信を記録
func (v *Visualizer) RecordChannelReceive(channelName string, goroutineID int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	ch, ok := v.channels[channelName]
	if !ok {
		ch = ChannelInfo{Name: channelName}
	}

	atomic.AddInt64(&ch.Receives, 1)
	if ch.Current > 0 {
		ch.Current--
	}
	v.channels[channelName] = ch

	if g, ok := v.goroutines[goroutineID]; ok {
		g.Operations = append(g.Operations, fmt.Sprintf("Receive from %s", channelName))
		v.goroutines[goroutineID] = g
	}
}

// RecordMutexLock Mutex取得を記録
func (v *Visualizer) RecordMutexLock(mutexName string, goroutineID int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	mx, ok := v.mutexes[mutexName]
	if !ok {
		mx = MutexInfo{Name: mutexName}
	}

	mx.Locked = true
	mx.Owner = goroutineID
	atomic.AddInt64(&mx.LockCount, 1)
	v.mutexes[mutexName] = mx

	if g, ok := v.goroutines[goroutineID]; ok {
		g.Operations = append(g.Operations, fmt.Sprintf("Lock %s", mutexName))
		v.goroutines[goroutineID] = g
	}
}

// RecordMutexUnlock Mutex解放を記録
func (v *Visualizer) RecordMutexUnlock(mutexName string, goroutineID int) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if mx, ok := v.mutexes[mutexName]; ok {
		mx.Locked = false
		mx.Owner = 0
		v.mutexes[mutexName] = mx
	}

	if g, ok := v.goroutines[goroutineID]; ok {
		g.Operations = append(g.Operations, fmt.Sprintf("Unlock %s", mutexName))
		v.goroutines[goroutineID] = g
	}
}

// Display 現在の状態を表示
func (v *Visualizer) Display() {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Clear screen (works on Unix-like systems)
	fmt.Print("\033[H\033[2J")

	fmt.Println("🔍 Go Concurrency Visualizer")
	fmt.Println(strings.Repeat("=", 60))

	// System info
	fmt.Printf("📊 System: %d CPUs | %d Goroutines\n",
		runtime.NumCPU(), runtime.NumGoroutine())
	fmt.Println(strings.Repeat("-", 60))

	// Goroutines
	fmt.Println("\n🧵 Goroutines:")
	for _, g := range v.goroutines {
		stateIcon := v.getStateIcon(g.State)
		duration := time.Since(g.StartTime).Round(time.Millisecond)
		if g.State == "finished" {
			duration = g.EndTime.Sub(g.StartTime).Round(time.Millisecond)
		}

		fmt.Printf("  %s G%d: %s (%v)\n", stateIcon, g.ID, g.Name, duration)

		if len(g.Operations) > 0 && g.State == "running" {
			lastOp := g.Operations[len(g.Operations)-1]
			fmt.Printf("     └─ Last: %s\n", lastOp)
		}
	}

	// Channels
	if len(v.channels) > 0 {
		fmt.Println("\n📡 Channels:")
		for _, ch := range v.channels {
			bar := v.makeProgressBar(ch.Current, ch.Capacity)
			fmt.Printf("  %s: %s [%d/%d] S:%d R:%d\n",
				ch.Name, bar, ch.Current, ch.Capacity, ch.Sends, ch.Receives)
		}
	}

	// Mutexes
	if len(v.mutexes) > 0 {
		fmt.Println("\n🔒 Mutexes:")
		for _, mx := range v.mutexes {
			lockIcon := "🔓"
			if mx.Locked {
				lockIcon = "🔒"
			}
			fmt.Printf("  %s %s: ", lockIcon, mx.Name)
			if mx.Locked {
				fmt.Printf("Owner: G%d", mx.Owner)
			} else {
				fmt.Printf("Unlocked")
			}
			fmt.Printf(" (Locks: %d)\n", mx.LockCount)
		}
	}

	fmt.Println(strings.Repeat("=", 60))
}

// getStateIcon 状態に応じたアイコンを返す
func (v *Visualizer) getStateIcon(state string) string {
	switch state {
	case "running":
		return "🟢"
	case "blocked":
		return "🔴"
	case "sleeping":
		return "🟡"
	case "finished":
		return "⚫"
	default:
		return "⚪"
	}
}

// makeProgressBar プログレスバーを作成
func (v *Visualizer) makeProgressBar(current, capacity int) string {
	if capacity == 0 {
		return "[unbuffered]"
	}

	barLength := 10
	filled := 0
	if capacity > 0 {
		filled = (current * barLength) / capacity
	}

	bar := "["
	for i := 0; i < barLength; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "]"

	return bar
}

// StartLiveView ライブビューを開始
func (v *Visualizer) StartLiveView() {
	v.running = true
	go func() {
		ticker := time.NewTicker(v.updateInterval)
		defer ticker.Stop()

		for v.running {
			select {
			case <-ticker.C:
				v.Display()
			}
		}
	}()
}

// StopLiveView ライブビューを停止
func (v *Visualizer) StopLiveView() {
	v.running = false
	time.Sleep(v.updateInterval * 2)
}

// Example 使用例
func ExampleVisualization() {
	viz := NewVisualizer()

	// チャネルを登録
	viz.channels["data"] = ChannelInfo{
		Name:     "data",
		Capacity: 5,
	}

	viz.StartLiveView()
	defer viz.StopLiveView()

	var wg sync.WaitGroup

	// Producer
	wg.Add(1)
	go func() {
		defer wg.Done()
		id := viz.StartGoroutine("Producer")
		defer viz.EndGoroutine(id)

		for i := 0; i < 10; i++ {
			viz.RecordChannelSend("data", id)
			time.Sleep(200 * time.Millisecond)
		}
	}()

	// Consumer
	wg.Add(1)
	go func() {
		defer wg.Done()
		id := viz.StartGoroutine("Consumer")
		defer viz.EndGoroutine(id)

		for i := 0; i < 10; i++ {
			time.Sleep(300 * time.Millisecond)
			viz.RecordChannelReceive("data", id)
		}
	}()

	// Mutex user
	wg.Add(1)
	go func() {
		defer wg.Done()
		id := viz.StartGoroutine("MutexUser")
		defer viz.EndGoroutine(id)

		for i := 0; i < 5; i++ {
			viz.RecordMutexLock("shared", id)
			time.Sleep(100 * time.Millisecond)
			viz.RecordMutexUnlock("shared", id)
			time.Sleep(100 * time.Millisecond)
		}
	}()

	wg.Wait()
	time.Sleep(1 * time.Second)
}