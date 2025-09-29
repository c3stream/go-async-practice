package challenges

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Challenge 19: Distributed Logger with Aggregation
//
// 問題点:
// 1. ログのロスト
// 2. バッファオーバーフロー
// 3. シンク（出力先）の障害処理なし
// 4. 構造化ログの不整合
// 5. パフォーマンスボトルネック

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

type LogEntry struct {
	Level     LogLevel
	Timestamp time.Time
	Message   string
	Fields    map[string]interface{}
	TraceID   string
	SpanID    string
	Service   string
	Host      string
}

type LogSink interface {
	Write(entry LogEntry) error
	Flush() error
	Close() error
}

type FileSink struct {
	mu       sync.Mutex
	buffer   []LogEntry
	filename string
	maxSize  int
	current  int
}

type NetworkSink struct {
	endpoint string
	buffer   chan LogEntry // 問題: バッファサイズ固定
	client   *fakeHTTPClient
	retries  int
}

type fakeHTTPClient struct{}

func (c *fakeHTTPClient) Post(endpoint string, data []byte) error {
	// Simulate network call
	if len(data) > 1000 {
		return fmt.Errorf("payload too large")
	}
	return nil
}

type DistributedLogger struct {
	mu         sync.RWMutex
	level      LogLevel
	sinks      []LogSink
	buffer     chan LogEntry // 問題: 固定サイズ
	sampling   float64
	aggregator *LogAggregator
	closed     chan struct{}
	wg         sync.WaitGroup
}

type LogAggregator struct {
	mu      sync.Mutex
	metrics map[string]int64  // 問題: 無制限に成長
	errors  map[string]int    // 問題: メモリリーク
	window  time.Duration
	lastFlush time.Time
}

func NewDistributedLogger() *DistributedLogger {
	logger := &DistributedLogger{
		level:      INFO,
		sinks:      []LogSink{},
		buffer:     make(chan LogEntry, 100), // 問題: 固定サイズ
		sampling:   1.0,
		aggregator: &LogAggregator{
			metrics:   make(map[string]int64),
			errors:    make(map[string]int),
			window:    1 * time.Minute,
			lastFlush: time.Now(),
		},
		closed: make(chan struct{}),
	}

	// 問題: goroutineリークの可能性
	go logger.processLogs()

	return logger
}

// 問題1: ログ処理のボトルネック
func (dl *DistributedLogger) processLogs() {
	for {
		select {
		case log := <-dl.buffer:
			// 問題: シーケンシャル処理
			for _, sink := range dl.sinks {
				// 問題: エラーハンドリングなし
				sink.Write(log)
			}

			// 問題: アグリゲーション処理がボトルネック
			dl.aggregator.mu.Lock()
			key := fmt.Sprintf("%s:%d", log.Service, log.Level)
			dl.aggregator.metrics[key]++
			dl.aggregator.mu.Unlock()

		case <-dl.closed:
			// 問題: バッファに残ったログを処理しない
			return
		}
	}
}

// 問題2: ログ送信のロスト
func (dl *DistributedLogger) Log(level LogLevel, message string, fields map[string]interface{}) {
	// 問題: レベルチェックが不正確
	if level < dl.level {
		return
	}

	entry := LogEntry{
		Level:     level,
		Timestamp: time.Now(),
		Message:   message,
		Fields:    fields,
		// 問題: TraceIDとSpanIDが設定されていない
	}

	// 問題: ノンブロッキング送信でログがロストする可能性
	select {
	case dl.buffer <- entry:
		// OK
	default:
		// 問題: ログがドロップされる
		fmt.Println("Log buffer full, dropping log")
	}
}

// 問題3: シンク管理
func (dl *DistributedLogger) AddSink(sink LogSink) {
	// 問題: 並行アクセスの保護なし
	dl.sinks = append(dl.sinks, sink)
}

// 問題4: NetworkSinkの実装
func NewNetworkSink(endpoint string) *NetworkSink {
	return &NetworkSink{
		endpoint: endpoint,
		buffer:   make(chan LogEntry, 50), // 問題: 小さすぎるバッファ
		client:   &fakeHTTPClient{},
		retries:  3,
	}
}

func (ns *NetworkSink) Write(entry LogEntry) error {
	// 問題: ブロッキング送信
	select {
	case ns.buffer <- entry:
		return nil
	default:
		return fmt.Errorf("network buffer full")
	}
}

func (ns *NetworkSink) processBuffer() {
	batch := make([]LogEntry, 0, 10)
	ticker := time.NewTicker(1 * time.Second)

	for {
		select {
		case entry := <-ns.buffer:
			batch = append(batch, entry)

			// 問題: バッチサイズ制限なし
			if len(batch) >= 10 {
				ns.sendBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			if len(batch) > 0 {
				ns.sendBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func (ns *NetworkSink) sendBatch(batch []LogEntry) error {
	data, err := json.Marshal(batch)
	if err != nil {
		return err
	}

	// 問題: リトライロジックが不完全
	for i := 0; i < ns.retries; i++ {
		err = ns.client.Post(ns.endpoint, data)
		if err == nil {
			return nil
		}
		// 問題: 固定待機時間
		time.Sleep(100 * time.Millisecond)
	}

	// 問題: 失敗したログは失われる
	return err
}

func (ns *NetworkSink) Flush() error {
	// 問題: 実装されていない
	return nil
}

func (ns *NetworkSink) Close() error {
	// 問題: リソースクリーンアップなし
	return nil
}

// 問題5: ログアグリゲーション
func (la *LogAggregator) GetMetrics() map[string]interface{} {
	la.mu.Lock()
	defer la.mu.Unlock()

	// 問題: メトリクスのコピーが非効率
	result := make(map[string]interface{})
	for k, v := range la.metrics {
		result[k] = v
	}

	// 問題: 古いメトリクスのクリーンアップなし

	return result
}

// Challenge: 以下の問題を修正してください
// 1. ログロストの防止
// 2. 動的バッファサイズ
// 3. 適切なバッチ処理
// 4. Circuit Breakerパターン
// 5. 構造化ログの改善
// 6. 分散トレーシング対応
// 7. メトリクスのウィンドウ管理
// 8. グレースフルシャットダウン

func RunChallenge19() {
	fmt.Println("Challenge 19: Distributed Logger")
	fmt.Println("Fix the distributed logging system")

	logger := NewDistributedLogger()

	// ネットワークシンク追加
	netSink := NewNetworkSink("http://log-aggregator:8080/logs")
	logger.AddSink(netSink)

	// 高頻度ログ生成
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				logger.Log(INFO, fmt.Sprintf("Worker %d message %d", id, j), map[string]interface{}{
					"worker_id": id,
					"iteration": j,
				})
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// メトリクス表示
	metrics := logger.aggregator.GetMetrics()
	fmt.Printf("Metrics: %+v\n", metrics)
}