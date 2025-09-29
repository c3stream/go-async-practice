package practical

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// DistributedLoggingExample demonstrates a production-ready distributed logging system
// with structured logging, buffering, batching, and multiple sinks
type DistributedLoggingExample struct {
	sinks          []LogSink
	buffer         *RingBuffer
	batcher        *LogBatcher
	sampler        *LogSampler
	enrichers      []LogEnricher
	metrics        *LoggingMetrics
	correlationMap sync.Map
	wg             sync.WaitGroup
	shutdown       chan struct{}
}

type LogLevel int

const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

type LogEntry struct {
	Level         LogLevel               `json:"level"`
	Timestamp     time.Time              `json:"timestamp"`
	Message       string                 `json:"message"`
	Fields        map[string]interface{} `json:"fields"`
	TraceID       string                 `json:"trace_id,omitempty"`
	SpanID        string                 `json:"span_id,omitempty"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Service       string                 `json:"service"`
	Host          string                 `json:"host"`
	Stack         string                 `json:"stack,omitempty"`
}

type LogSink interface {
	Write(entries []LogEntry) error
	Flush() error
	Close() error
	Name() string
}

type LogEnricher func(*LogEntry)

// ConsoleSink writes logs to console
type ConsoleSink struct {
	formatter LogFormatter
}

// FileSink writes logs to file with rotation
type FileSink struct {
	filename    string
	maxSize     int64
	maxBackups  int
	currentSize int64
	mu          sync.Mutex
}

// NetworkSink sends logs to remote endpoint
type NetworkSink struct {
	endpoint   string
	client     HTTPClient
	retryCount int
	timeout    time.Duration
}

// ElasticsearchSink writes logs to Elasticsearch
type ElasticsearchSink struct {
	endpoint string
	index    string
	bulkSize int
	client   HTTPClient
}

type HTTPClient interface {
	Post(url string, data []byte) error
}

type MockHTTPClient struct{}

func (c *MockHTTPClient) Post(url string, data []byte) error {
	// Simulate network call
	time.Sleep(10 * time.Millisecond)
	if rand.Float32() > 0.95 { // 5% failure rate
		return fmt.Errorf("network error")
	}
	return nil
}

type LogFormatter interface {
	Format(entry LogEntry) string
}

type JSONFormatter struct{}

func (f *JSONFormatter) Format(entry LogEntry) string {
	data, _ := json.Marshal(entry)
	return string(data)
}

type TextFormatter struct{}

func (f *TextFormatter) Format(entry LogEntry) string {
	levelStr := []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}[entry.Level]
	return fmt.Sprintf("%s [%s] %s %s - %s | %v",
		entry.Timestamp.Format(time.RFC3339),
		levelStr,
		entry.Service,
		entry.TraceID,
		entry.Message,
		entry.Fields,
	)
}

// RingBuffer provides a lock-free ring buffer for logs
type RingBuffer struct {
	buffer   []LogEntry
	size     int64
	writePos int64
	readPos  int64
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buffer: make([]LogEntry, size),
		size:   int64(size),
	}
}

func (rb *RingBuffer) Write(entry LogEntry) bool {
	writePos := atomic.AddInt64(&rb.writePos, 1) - 1
	readPos := atomic.LoadInt64(&rb.readPos)

	if writePos-readPos >= rb.size {
		// Buffer full
		return false
	}

	rb.buffer[writePos%rb.size] = entry
	return true
}

func (rb *RingBuffer) Read() (LogEntry, bool) {
	readPos := atomic.LoadInt64(&rb.readPos)
	writePos := atomic.LoadInt64(&rb.writePos)

	if readPos >= writePos {
		// Buffer empty
		return LogEntry{}, false
	}

	atomic.AddInt64(&rb.readPos, 1)
	return rb.buffer[readPos%rb.size], true
}

// LogBatcher batches logs for efficient processing
type LogBatcher struct {
	batchSize int
	timeout   time.Duration
	batch     []LogEntry
	mu        sync.Mutex
	flush     chan struct{}
}

func NewLogBatcher(batchSize int, timeout time.Duration) *LogBatcher {
	return &LogBatcher{
		batchSize: batchSize,
		timeout:   timeout,
		batch:     make([]LogEntry, 0, batchSize),
		flush:     make(chan struct{}, 1),
	}
}

func (lb *LogBatcher) Add(entry LogEntry) []LogEntry {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	lb.batch = append(lb.batch, entry)

	if len(lb.batch) >= lb.batchSize {
		batch := lb.batch
		lb.batch = make([]LogEntry, 0, lb.batchSize)
		return batch
	}

	// Signal for timeout-based flush
	select {
	case lb.flush <- struct{}{}:
	default:
	}

	return nil
}

func (lb *LogBatcher) GetBatch() []LogEntry {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.batch) == 0 {
		return nil
	}

	batch := lb.batch
	lb.batch = make([]LogEntry, 0, lb.batchSize)
	return batch
}

// LogSampler implements sampling strategies
type LogSampler struct {
	sampleRate map[LogLevel]float64
	counter    map[LogLevel]int64
	mu         sync.RWMutex
}

func NewLogSampler() *LogSampler {
	return &LogSampler{
		sampleRate: map[LogLevel]float64{
			LevelDebug: 0.1,  // Sample 10% of debug logs
			LevelInfo:  0.5,  // Sample 50% of info logs
			LevelWarn:  1.0,  // Keep all warnings
			LevelError: 1.0,  // Keep all errors
			LevelFatal: 1.0,  // Keep all fatal logs
		},
		counter: make(map[LogLevel]int64),
	}
}

func (ls *LogSampler) ShouldSample(level LogLevel) bool {
	ls.mu.RLock()
	rate := ls.sampleRate[level]
	ls.mu.RUnlock()

	// Always sample first N logs of each level
	// For simplicity, we'll use a basic counter approach
	// In production, you'd use a more sophisticated method
	if rand.Float64() < 0.1 { // Sample 10% of logs after first batch
		return true
	}

	return rand.Float64() < rate
}

// LoggingMetrics tracks logging system metrics
type LoggingMetrics struct {
	totalLogs     int64
	droppedLogs   int64
	errorCount    int64
	sinkMetrics   map[string]*SinkMetrics
	mu            sync.RWMutex
}

type SinkMetrics struct {
	written int64
	failed  int64
	latency time.Duration
}

// NewDistributedLoggingExample creates a new distributed logging system
func NewDistributedLoggingExample() *DistributedLoggingExample {
	dl := &DistributedLoggingExample{
		sinks:    make([]LogSink, 0),
		buffer:   NewRingBuffer(10000),
		batcher:  NewLogBatcher(100, 1*time.Second),
		sampler:  NewLogSampler(),
		enrichers: make([]LogEnricher, 0),
		metrics: &LoggingMetrics{
			sinkMetrics: make(map[string]*SinkMetrics),
		},
		shutdown: make(chan struct{}),
	}

	// Add default enrichers
	dl.AddEnricher(dl.enrichWithHostInfo)
	dl.AddEnricher(dl.enrichWithTimestamp)

	// Start processing goroutines
	dl.wg.Add(2)
	go dl.processBuffer()
	go dl.processBatcher()

	return dl
}

// AddSink adds a new log sink
func (dl *DistributedLoggingExample) AddSink(sink LogSink) {
	dl.sinks = append(dl.sinks, sink)
	dl.metrics.mu.Lock()
	dl.metrics.sinkMetrics[sink.Name()] = &SinkMetrics{}
	dl.metrics.mu.Unlock()
}

// AddEnricher adds a log enricher
func (dl *DistributedLoggingExample) AddEnricher(enricher LogEnricher) {
	dl.enrichers = append(dl.enrichers, enricher)
}

// Log writes a log entry
func (dl *DistributedLoggingExample) Log(level LogLevel, message string, fields map[string]interface{}) {
	// Apply sampling
	if !dl.sampler.ShouldSample(level) {
		atomic.AddInt64(&dl.metrics.droppedLogs, 1)
		return
	}

	entry := LogEntry{
		Level:   level,
		Message: message,
		Fields:  fields,
	}

	// Enrich log entry
	for _, enricher := range dl.enrichers {
		enricher(&entry)
	}

	// Write to buffer
	if !dl.buffer.Write(entry) {
		atomic.AddInt64(&dl.metrics.droppedLogs, 1)
		return
	}

	atomic.AddInt64(&dl.metrics.totalLogs, 1)
}

// LogWithContext logs with trace and span IDs
func (dl *DistributedLoggingExample) LogWithContext(ctx context.Context, level LogLevel, message string, fields map[string]interface{}) {
	// Extract trace context
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		if fields == nil {
			fields = make(map[string]interface{})
		}
		fields["trace_id"] = traceID
	}

	if spanID, ok := ctx.Value("span_id").(string); ok {
		if fields == nil {
			fields = make(map[string]interface{})
		}
		fields["span_id"] = spanID
	}

	dl.Log(level, message, fields)
}

// processBuffer processes logs from the ring buffer
func (dl *DistributedLoggingExample) processBuffer() {
	defer dl.wg.Done()

	for {
		select {
		case <-dl.shutdown:
			// Process remaining logs
			for {
				entry, ok := dl.buffer.Read()
				if !ok {
					return
				}
				dl.processSingleLog(entry)
			}
		default:
			entry, ok := dl.buffer.Read()
			if !ok {
				time.Sleep(10 * time.Millisecond)
				continue
			}
			dl.processSingleLog(entry)
		}
	}
}

// processSingleLog processes a single log entry
func (dl *DistributedLoggingExample) processSingleLog(entry LogEntry) {
	// Add to batcher
	if batch := dl.batcher.Add(entry); batch != nil {
		dl.writeBatch(batch)
	}
}

// processBatcher handles timeout-based batch flushing
func (dl *DistributedLoggingExample) processBatcher() {
	defer dl.wg.Done()

	ticker := time.NewTicker(dl.batcher.timeout)
	defer ticker.Stop()

	for {
		select {
		case <-dl.shutdown:
			// Flush remaining batch
			if batch := dl.batcher.GetBatch(); batch != nil {
				dl.writeBatch(batch)
			}
			return
		case <-ticker.C:
			if batch := dl.batcher.GetBatch(); batch != nil {
				dl.writeBatch(batch)
			}
		case <-dl.batcher.flush:
			// Batch size reached, already handled in Add()
		}
	}
}

// writeBatch writes a batch of logs to all sinks
func (dl *DistributedLoggingExample) writeBatch(batch []LogEntry) {
	var wg sync.WaitGroup

	for _, sink := range dl.sinks {
		wg.Add(1)
		go func(s LogSink) {
			defer wg.Done()

			start := time.Now()
			err := s.Write(batch)
			duration := time.Since(start)

			// Update metrics
			dl.metrics.mu.Lock()
			metrics := dl.metrics.sinkMetrics[s.Name()]
			if err != nil {
				metrics.failed += int64(len(batch))
				atomic.AddInt64(&dl.metrics.errorCount, 1)
			} else {
				metrics.written += int64(len(batch))
			}
			metrics.latency = (metrics.latency + duration) / 2
			dl.metrics.mu.Unlock()
		}(sink)
	}

	wg.Wait()
}

// enrichWithHostInfo adds host information
func (dl *DistributedLoggingExample) enrichWithHostInfo(entry *LogEntry) {
	entry.Host = "localhost" // In production, get actual hostname
	entry.Service = "example-service"
}

// enrichWithTimestamp ensures timestamp is set
func (dl *DistributedLoggingExample) enrichWithTimestamp(entry *LogEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
}

// Shutdown gracefully shuts down the logging system
func (dl *DistributedLoggingExample) Shutdown() {
	close(dl.shutdown)
	dl.wg.Wait()

	// Flush all sinks
	for _, sink := range dl.sinks {
		sink.Flush()
		sink.Close()
	}
}

// GetMetrics returns current metrics
func (dl *DistributedLoggingExample) GetMetrics() map[string]interface{} {
	dl.metrics.mu.RLock()
	defer dl.metrics.mu.RUnlock()

	sinkStats := make(map[string]map[string]interface{})
	for name, metrics := range dl.metrics.sinkMetrics {
		sinkStats[name] = map[string]interface{}{
			"written": metrics.written,
			"failed":  metrics.failed,
			"latency": metrics.latency,
		}
	}

	return map[string]interface{}{
		"total_logs":   dl.metrics.totalLogs,
		"dropped_logs": dl.metrics.droppedLogs,
		"error_count":  dl.metrics.errorCount,
		"sinks":        sinkStats,
	}
}

// Sink implementations

func (s *ConsoleSink) Write(entries []LogEntry) error {
	for _, entry := range entries {
		fmt.Println(s.formatter.Format(entry))
	}
	return nil
}

func (s *ConsoleSink) Flush() error { return nil }
func (s *ConsoleSink) Close() error { return nil }
func (s *ConsoleSink) Name() string { return "console" }

func (s *NetworkSink) Write(entries []LogEntry) error {
	data, err := json.Marshal(entries)
	if err != nil {
		return err
	}

	// Retry logic
	var lastErr error
	for i := 0; i <= s.retryCount; i++ {
		if i > 0 {
			time.Sleep(time.Duration(i*i) * 100 * time.Millisecond) // Exponential backoff
		}

		err = s.client.Post(s.endpoint, data)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return lastErr
}

func (s *NetworkSink) Flush() error { return nil }
func (s *NetworkSink) Close() error { return nil }
func (s *NetworkSink) Name() string { return "network" }

func (s *ElasticsearchSink) Write(entries []LogEntry) error {
	// Build bulk request
	var bulkData []byte
	for _, entry := range entries {
		// Add index action
		action := map[string]interface{}{
			"index": map[string]interface{}{
				"_index": s.index,
			},
		}
		actionData, _ := json.Marshal(action)
		bulkData = append(bulkData, actionData...)
		bulkData = append(bulkData, '\n')

		// Add document
		docData, _ := json.Marshal(entry)
		bulkData = append(bulkData, docData...)
		bulkData = append(bulkData, '\n')
	}

	// Send bulk request
	return s.client.Post(fmt.Sprintf("%s/_bulk", s.endpoint), bulkData)
}

func (s *ElasticsearchSink) Flush() error { return nil }
func (s *ElasticsearchSink) Close() error { return nil }
func (s *ElasticsearchSink) Name() string { return "elasticsearch" }

// RunDistributedLoggingExample demonstrates the distributed logging system
func RunDistributedLoggingExample() {
	fmt.Println("\n=== Distributed Logging Example ===")

	logger := NewDistributedLoggingExample()
	defer logger.Shutdown()

	// Add sinks
	logger.AddSink(&ConsoleSink{formatter: &TextFormatter{}})
	logger.AddSink(&NetworkSink{
		endpoint:   "http://log-aggregator:8080/logs",
		client:     &MockHTTPClient{},
		retryCount: 3,
		timeout:    5 * time.Second,
	})
	logger.AddSink(&ElasticsearchSink{
		endpoint: "http://elasticsearch:9200",
		index:    "logs-2024",
		bulkSize: 100,
		client:   &MockHTTPClient{},
	})

	// Simulate application logging
	var wg sync.WaitGroup

	// Simulate multiple services
	services := []string{"auth-service", "order-service", "payment-service"}
	for _, service := range services {
		wg.Add(1)
		go func(svc string) {
			defer wg.Done()

			// Create context with trace ID
			ctx := context.WithValue(context.Background(), "trace_id", fmt.Sprintf("trace-%s-%d", svc, time.Now().Unix()))

			for i := 0; i < 20; i++ {
				// Vary log levels
				level := LogLevel(i % 5)

				fields := map[string]interface{}{
					"service":   svc,
					"iteration": i,
					"user_id":   rand.Intn(1000),
				}

				// Simulate different log scenarios
				switch i % 4 {
				case 0:
					logger.LogWithContext(ctx, level, fmt.Sprintf("%s processing request", svc), fields)
				case 1:
					fields["error"] = "timeout"
					logger.LogWithContext(ctx, LevelError, fmt.Sprintf("%s encountered error", svc), fields)
				case 2:
					fields["duration_ms"] = rand.Intn(500)
					logger.LogWithContext(ctx, LevelInfo, fmt.Sprintf("%s completed operation", svc), fields)
				case 3:
					logger.LogWithContext(ctx, LevelDebug, fmt.Sprintf("%s debug information", svc), fields)
				}

				time.Sleep(50 * time.Millisecond)
			}
		}(service)
	}

	wg.Wait()

	// Give time for processing
	time.Sleep(2 * time.Second)

	// Display metrics
	metrics := logger.GetMetrics()
	fmt.Printf("\nLogging Metrics:\n")
	fmt.Printf("Total logs: %v\n", metrics["total_logs"])
	fmt.Printf("Dropped logs: %v\n", metrics["dropped_logs"])
	fmt.Printf("Error count: %v\n", metrics["error_count"])

	if sinks, ok := metrics["sinks"].(map[string]map[string]interface{}); ok {
		for name, stats := range sinks {
			fmt.Printf("Sink %s - Written: %v, Failed: %v, Latency: %v\n",
				name, stats["written"], stats["failed"], stats["latency"])
		}
	}

	fmt.Println("\nDistributed logging example completed")
}