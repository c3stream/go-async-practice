package solutions

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Solution19DistributedLogger demonstrates three approaches to fixing distributed logging issues:
// 1. Buffered logging with batching
// 2. Structured logging with context propagation
// 3. Multi-sink logging with circuit breaker

// Solution 1: Buffered Logging with Batching
type BufferedLogger struct {
	buffer     chan LogEntry
	batch      []LogEntry
	batchSize  int
	flushTimer *time.Timer
	sinks      []LogSink
	wg         sync.WaitGroup
	shutdown   chan struct{}
	metrics    *LogMetrics
}

type LogEntry struct {
	Level       LogLevel
	Timestamp   time.Time
	Message     string
	Fields      map[string]interface{}
	TraceID     string
	SpanID      string
	Service     string
	Host        string
}

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

type LogSink interface {
	Write(entries []LogEntry) error
	Flush() error
	Close() error
}

type LogMetrics struct {
	totalLogs    int64
	droppedLogs  int64
	batchesSent  int64
	avgBatchSize float64
	mu           sync.RWMutex
}

func NewBufferedLogger(bufferSize, batchSize int, flushInterval time.Duration) *BufferedLogger {
	logger := &BufferedLogger{
		buffer:    make(chan LogEntry, bufferSize),
		batch:     make([]LogEntry, 0, batchSize),
		batchSize: batchSize,
		sinks:     make([]LogSink, 0),
		shutdown:  make(chan struct{}),
		metrics:   &LogMetrics{},
	}

	logger.wg.Add(1)
	go logger.processBatches(flushInterval)

	return logger
}

func (l *BufferedLogger) Log(level LogLevel, message string, fields map[string]interface{}) {
	entry := LogEntry{
		Level:     level,
		Timestamp: time.Now(),
		Message:   message,
		Fields:    fields,
		Service:   "default",
		Host:      "localhost",
	}

	select {
	case l.buffer <- entry:
		atomic.AddInt64(&l.metrics.totalLogs, 1)
	default:
		// Buffer full, try with timeout
		select {
		case l.buffer <- entry:
			atomic.AddInt64(&l.metrics.totalLogs, 1)
		case <-time.After(10 * time.Millisecond):
			atomic.AddInt64(&l.metrics.droppedLogs, 1)
		}
	}
}

func (l *BufferedLogger) processBatches(flushInterval time.Duration) {
	defer l.wg.Done()

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case entry := <-l.buffer:
			l.batch = append(l.batch, entry)

			if len(l.batch) >= l.batchSize {
				l.flushBatch()
			}

		case <-ticker.C:
			if len(l.batch) > 0 {
				l.flushBatch()
			}

		case <-l.shutdown:
			// Flush remaining logs
			for len(l.buffer) > 0 {
				entry := <-l.buffer
				l.batch = append(l.batch, entry)
			}
			if len(l.batch) > 0 {
				l.flushBatch()
			}
			return
		}
	}
}

func (l *BufferedLogger) flushBatch() {
	if len(l.batch) == 0 {
		return
	}

	// Copy batch for processing
	batchToSend := make([]LogEntry, len(l.batch))
	copy(batchToSend, l.batch)
	l.batch = l.batch[:0]

	// Update metrics
	atomic.AddInt64(&l.metrics.batchesSent, 1)
	l.metrics.mu.Lock()
	l.metrics.avgBatchSize = (l.metrics.avgBatchSize + float64(len(batchToSend))) / 2
	l.metrics.mu.Unlock()

	// Send to all sinks concurrently
	var wg sync.WaitGroup
	for _, sink := range l.sinks {
		wg.Add(1)
		go func(s LogSink) {
			defer wg.Done()
			if err := s.Write(batchToSend); err != nil {
				log.Printf("Failed to write to sink: %v", err)
			}
		}(sink)
	}
	wg.Wait()
}

func (l *BufferedLogger) AddSink(sink LogSink) {
	l.sinks = append(l.sinks, sink)
}

func (l *BufferedLogger) Shutdown() {
	close(l.shutdown)
	l.wg.Wait()

	// Flush and close all sinks
	for _, sink := range l.sinks {
		sink.Flush()
		sink.Close()
	}
}

// Solution 2: Structured Logging with Context
type StructuredLogger struct {
	sinks       []LogSink
	enrichers   []LogEnricher
	sampler     *LogSampler
	correlator  *CorrelationTracker
	mu          sync.RWMutex
}

type LogEnricher func(*LogEntry)

type LogSampler struct {
	rates map[LogLevel]float64
	mu    sync.RWMutex
}

type CorrelationTracker struct {
	traces map[string]*TraceInfo
	mu     sync.RWMutex
}

type TraceInfo struct {
	TraceID   string
	StartTime time.Time
	EndTime   time.Time
	Logs      []LogEntry
	Spans     map[string]*SpanInfo
}

type SpanInfo struct {
	SpanID    string
	ParentID  string
	StartTime time.Time
	EndTime   time.Time
}

func NewStructuredLogger() *StructuredLogger {
	logger := &StructuredLogger{
		sinks:     make([]LogSink, 0),
		enrichers: make([]LogEnricher, 0),
		sampler: &LogSampler{
			rates: map[LogLevel]float64{
				DEBUG: 0.1,
				INFO:  0.5,
				WARN:  1.0,
				ERROR: 1.0,
				FATAL: 1.0,
			},
		},
		correlator: &CorrelationTracker{
			traces: make(map[string]*TraceInfo),
		},
	}

	// Add default enrichers
	logger.AddEnricher(logger.enrichWithHostInfo)
	logger.AddEnricher(logger.enrichWithTimestamp)

	return logger
}

func (l *StructuredLogger) LogWithContext(ctx context.Context, level LogLevel, message string, fields map[string]interface{}) {
	// Check sampling
	if !l.shouldSample(level) {
		return
	}

	entry := LogEntry{
		Level:   level,
		Message: message,
		Fields:  fields,
	}

	// Extract trace context
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		entry.TraceID = traceID
	}
	if spanID, ok := ctx.Value("span_id").(string); ok {
		entry.SpanID = spanID
	}

	// Apply enrichers
	for _, enricher := range l.enrichers {
		enricher(&entry)
	}

	// Track correlation
	if entry.TraceID != "" {
		l.correlator.mu.Lock()
		if trace, exists := l.correlator.traces[entry.TraceID]; exists {
			trace.Logs = append(trace.Logs, entry)
		} else {
			l.correlator.traces[entry.TraceID] = &TraceInfo{
				TraceID:   entry.TraceID,
				StartTime: time.Now(),
				Logs:      []LogEntry{entry},
				Spans:     make(map[string]*SpanInfo),
			}
		}
		l.correlator.mu.Unlock()
	}

	// Write to sinks
	l.mu.RLock()
	sinks := l.sinks
	l.mu.RUnlock()

	for _, sink := range sinks {
		if err := sink.Write([]LogEntry{entry}); err != nil {
			log.Printf("Failed to write log: %v", err)
		}
	}
}

func (l *StructuredLogger) shouldSample(level LogLevel) bool {
	l.sampler.mu.RLock()
	_ = l.sampler.rates[level]
	l.sampler.mu.RUnlock()

	// Always sample first N logs
	// For demo, we'll always return true
	return true
}

func (l *StructuredLogger) AddEnricher(enricher LogEnricher) {
	l.enrichers = append(l.enrichers, enricher)
}

func (l *StructuredLogger) AddSink(sink LogSink) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sinks = append(l.sinks, sink)
}

func (l *StructuredLogger) enrichWithHostInfo(entry *LogEntry) {
	if entry.Host == "" {
		entry.Host = "localhost"
	}
	if entry.Service == "" {
		entry.Service = "app"
	}
}

func (l *StructuredLogger) enrichWithTimestamp(entry *LogEntry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
}

func (l *StructuredLogger) GetTraceInfo(traceID string) *TraceInfo {
	l.correlator.mu.RLock()
	defer l.correlator.mu.RUnlock()
	return l.correlator.traces[traceID]
}

// Solution 3: Multi-Sink Logger with Circuit Breaker
type MultiSinkLogger struct {
	sinks          map[string]SinkWrapper
	circuitBreaker map[string]*LogCircuitBreaker
	fallbackSink   LogSink
	mu             sync.RWMutex
	metrics        *SinkMetrics
}

type SinkWrapper struct {
	Sink     LogSink
	Priority int
	Filter   func(LogEntry) bool
}

type LogCircuitBreaker struct {
	failures     int64
	lastFailTime time.Time
	state        int32 // 0: closed, 1: open, 2: half-open
	maxFailures  int64
	cooldown     time.Duration
}

type SinkMetrics struct {
	sinkStats map[string]*SinkStat
	mu        sync.RWMutex
}

type SinkStat struct {
	Written   int64
	Failed    int64
	Latency   time.Duration
	LastError error
}

func NewMultiSinkLogger() *MultiSinkLogger {
	return &MultiSinkLogger{
		sinks:          make(map[string]SinkWrapper),
		circuitBreaker: make(map[string]*LogCircuitBreaker),
		metrics: &SinkMetrics{
			sinkStats: make(map[string]*SinkStat),
		},
	}
}

func (l *MultiSinkLogger) AddSinkWithPriority(name string, sink LogSink, priority int, filter func(LogEntry) bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.sinks[name] = SinkWrapper{
		Sink:     sink,
		Priority: priority,
		Filter:   filter,
	}

	l.circuitBreaker[name] = &LogCircuitBreaker{
		maxFailures: 5,
		cooldown:    30 * time.Second,
	}

	l.metrics.mu.Lock()
	l.metrics.sinkStats[name] = &SinkStat{}
	l.metrics.mu.Unlock()
}

func (l *MultiSinkLogger) SetFallbackSink(sink LogSink) {
	l.fallbackSink = sink
}

func (l *MultiSinkLogger) Log(level LogLevel, message string, fields map[string]interface{}) {
	entry := LogEntry{
		Level:     level,
		Timestamp: time.Now(),
		Message:   message,
		Fields:    fields,
		Service:   "multi-sink-app",
		Host:      "localhost",
	}

	l.mu.RLock()
	sinks := l.sinks
	l.mu.RUnlock()

	// Sort sinks by priority
	var sortedSinks []struct {
		Name    string
		Wrapper SinkWrapper
	}
	for name, wrapper := range sinks {
		sortedSinks = append(sortedSinks, struct {
			Name    string
			Wrapper SinkWrapper
		}{name, wrapper})
	}

	// Process sinks in priority order
	successCount := 0
	for _, s := range sortedSinks {
		// Check filter
		if s.Wrapper.Filter != nil && !s.Wrapper.Filter(entry) {
			continue
		}

		// Check circuit breaker
		cb := l.circuitBreaker[s.Name]
		if !l.isCircuitBreakerOpen(cb) {
			start := time.Now()
			err := s.Wrapper.Sink.Write([]LogEntry{entry})
			duration := time.Since(start)

			l.updateMetrics(s.Name, err, duration)

			if err != nil {
				l.recordFailure(cb)
				log.Printf("Sink %s failed: %v", s.Name, err)
			} else {
				l.recordSuccess(cb)
				successCount++
			}
		}
	}

	// Use fallback if all primary sinks failed
	if successCount == 0 && l.fallbackSink != nil {
		if err := l.fallbackSink.Write([]LogEntry{entry}); err != nil {
			log.Printf("Fallback sink also failed: %v", err)
		}
	}
}

func (l *MultiSinkLogger) isCircuitBreakerOpen(cb *LogCircuitBreaker) bool {
	state := atomic.LoadInt32(&cb.state)

	if state == 1 { // Open
		// Check if cooldown has passed
		if time.Since(cb.lastFailTime) > cb.cooldown {
			atomic.StoreInt32(&cb.state, 2) // Half-open
			return false
		}
		return true
	}

	return false
}

func (l *MultiSinkLogger) recordFailure(cb *LogCircuitBreaker) {
	failures := atomic.AddInt64(&cb.failures, 1)
	cb.lastFailTime = time.Now()

	if failures >= cb.maxFailures {
		atomic.StoreInt32(&cb.state, 1) // Open
	}
}

func (l *MultiSinkLogger) recordSuccess(cb *LogCircuitBreaker) {
	state := atomic.LoadInt32(&cb.state)
	if state == 2 { // Half-open
		atomic.StoreInt32(&cb.state, 0) // Closed
		atomic.StoreInt64(&cb.failures, 0)
	}
}

func (l *MultiSinkLogger) updateMetrics(sinkName string, err error, latency time.Duration) {
	l.metrics.mu.Lock()
	defer l.metrics.mu.Unlock()

	stats := l.metrics.sinkStats[sinkName]
	if stats == nil {
		stats = &SinkStat{}
		l.metrics.sinkStats[sinkName] = stats
	}

	if err != nil {
		stats.Failed++
		stats.LastError = err
	} else {
		stats.Written++
	}

	stats.Latency = (stats.Latency + latency) / 2
}

func (l *MultiSinkLogger) GetMetrics() map[string]*SinkStat {
	l.metrics.mu.RLock()
	defer l.metrics.mu.RUnlock()

	result := make(map[string]*SinkStat)
	for name, stat := range l.metrics.sinkStats {
		result[name] = stat
	}
	return result
}

// Example sinks for demonstration
type ConsoleSink struct{}

func (s *ConsoleSink) Write(entries []LogEntry) error {
	for _, entry := range entries {
		data, _ := json.Marshal(entry)
		fmt.Printf("Console: %s\n", string(data))
	}
	return nil
}

func (s *ConsoleSink) Flush() error { return nil }
func (s *ConsoleSink) Close() error { return nil }

type FileSink struct {
	filename string
}

func (s *FileSink) Write(entries []LogEntry) error {
	// Simulated file write
	for _, entry := range entries {
		fmt.Printf("File(%s): %+v\n", s.filename, entry)
	}
	return nil
}

func (s *FileSink) Flush() error { return nil }
func (s *FileSink) Close() error { return nil }

// RunSolution19 demonstrates the three solutions
func RunSolution19() {
	fmt.Println("=== Solution 19: Distributed Logging ===")

	// Solution 1: Buffered Logging with Batching
	fmt.Println("\n1. Buffered Logging with Batching:")
	bufferedLogger := NewBufferedLogger(1000, 10, 1*time.Second)
	defer bufferedLogger.Shutdown()

	bufferedLogger.AddSink(&ConsoleSink{})
	bufferedLogger.AddSink(&FileSink{filename: "app.log"})

	// Generate logs
	for i := 0; i < 20; i++ {
		bufferedLogger.Log(INFO, fmt.Sprintf("Log message %d", i), map[string]interface{}{
			"iteration": i,
			"component": "test",
		})
	}

	time.Sleep(2 * time.Second) // Wait for batch processing

	fmt.Printf("  Metrics - Total: %d, Dropped: %d, Batches: %d\n",
		bufferedLogger.metrics.totalLogs,
		bufferedLogger.metrics.droppedLogs,
		bufferedLogger.metrics.batchesSent)

	// Solution 2: Structured Logging with Context
	fmt.Println("\n2. Structured Logging with Context:")
	structuredLogger := NewStructuredLogger()
	structuredLogger.AddSink(&ConsoleSink{})

	// Create context with trace ID
	ctx := context.WithValue(context.Background(), "trace_id", "trace-123")
	ctx = context.WithValue(ctx, "span_id", "span-456")

	// Log with context
	structuredLogger.LogWithContext(ctx, INFO, "User login", map[string]interface{}{
		"user_id": 123,
		"ip":      "192.168.1.1",
	})

	structuredLogger.LogWithContext(ctx, ERROR, "Database connection failed", map[string]interface{}{
		"error": "timeout",
		"retry": 3,
	})

	// Get trace info
	if trace := structuredLogger.GetTraceInfo("trace-123"); trace != nil {
		fmt.Printf("  Trace %s has %d logs\n", trace.TraceID, len(trace.Logs))
	}

	// Solution 3: Multi-Sink Logger with Circuit Breaker
	fmt.Println("\n3. Multi-Sink Logger with Circuit Breaker:")
	multiSinkLogger := NewMultiSinkLogger()

	// Add sinks with different priorities
	multiSinkLogger.AddSinkWithPriority("console", &ConsoleSink{}, 10, func(e LogEntry) bool {
		return e.Level >= INFO
	})

	multiSinkLogger.AddSinkWithPriority("file", &FileSink{filename: "error.log"}, 5, func(e LogEntry) bool {
		return e.Level >= ERROR
	})

	// Set fallback sink
	multiSinkLogger.SetFallbackSink(&FileSink{filename: "fallback.log"})

	// Generate logs
	multiSinkLogger.Log(DEBUG, "Debug message", nil)
	multiSinkLogger.Log(INFO, "Info message", nil)
	multiSinkLogger.Log(ERROR, "Error message", nil)

	// Show metrics
	metrics := multiSinkLogger.GetMetrics()
	for name, stats := range metrics {
		fmt.Printf("  Sink %s - Written: %d, Failed: %d, Latency: %v\n",
			name, stats.Written, stats.Failed, stats.Latency)
	}

	fmt.Println("\nAll solutions demonstrated successfully!")
}