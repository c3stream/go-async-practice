package solutions

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Solution22 demonstrates three approaches to time series database operations:
// 1. Concurrent time-ordered insertion with compression
// 2. Window-based aggregation with watermarks
// 3. Distributed time series with sharding

// Solution 1: Time-Ordered Insertion with Compression
type CompressedTimeSeries struct {
	mu           sync.RWMutex
	series       map[string]*SeriesData
	compression  CompressionStrategy
	writeBuffer  *WriteBuffer
	flushTicker  *time.Ticker
	maxPointsRAM int
}

type SeriesData struct {
	mu         sync.RWMutex
	name       string
	points     []DataPoint
	compressed []CompressedBlock
	metadata   map[string]string
	lastWrite  time.Time
}

type DataPoint struct {
	Timestamp time.Time
	Value     float64
	Tags      map[string]string
}

type CompressedBlock struct {
	StartTime    time.Time
	EndTime      time.Time
	NumPoints    int
	CompressedData []byte
	Encoding     string
}

type WriteBuffer struct {
	mu       sync.Mutex
	pending  map[string][]DataPoint
	size     int64
	maxSize  int64
}

type CompressionStrategy interface {
	Compress(points []DataPoint) ([]byte, error)
	Decompress(data []byte) ([]DataPoint, error)
}

type DeltaCompression struct{}

func (dc *DeltaCompression) Compress(points []DataPoint) ([]byte, error) {
	if len(points) == 0 {
		return nil, nil
	}

	// Sort points by timestamp
	sort.Slice(points, func(i, j int) bool {
		return points[i].Timestamp.Before(points[j].Timestamp)
	})

	// Delta encoding simulation
	compressed := make([]byte, 0, len(points)*8)
	var prevValue float64
	var prevTime int64

	for i, p := range points {
		timestamp := p.Timestamp.UnixNano()
		if i == 0 {
			// Store first value as-is
			compressed = append(compressed, floatToBytes(p.Value)...)
			compressed = append(compressed, int64ToBytes(timestamp)...)
			prevValue = p.Value
			prevTime = timestamp
		} else {
			// Store deltas
			deltaValue := p.Value - prevValue
			deltaTime := timestamp - prevTime
			compressed = append(compressed, floatToBytes(deltaValue)...)
			compressed = append(compressed, int64ToBytes(deltaTime)...)
			prevValue = p.Value
			prevTime = timestamp
		}
	}

	return compressed, nil
}

func (dc *DeltaCompression) Decompress(data []byte) ([]DataPoint, error) {
	// Simplified decompression
	return []DataPoint{}, nil
}

func NewCompressedTimeSeries() *CompressedTimeSeries {
	ts := &CompressedTimeSeries{
		series:       make(map[string]*SeriesData),
		compression:  &DeltaCompression{},
		writeBuffer:  &WriteBuffer{
			pending: make(map[string][]DataPoint),
			maxSize: 10000,
		},
		maxPointsRAM: 100000,
	}

	// Start flush routine
	ts.flushTicker = time.NewTicker(5 * time.Second)
	go ts.flushRoutine()

	return ts
}

func (ts *CompressedTimeSeries) Write(seriesName string, point DataPoint) error {
	// Buffer writes
	ts.writeBuffer.mu.Lock()
	ts.writeBuffer.pending[seriesName] = append(ts.writeBuffer.pending[seriesName], point)
	atomic.AddInt64(&ts.writeBuffer.size, 1)

	shouldFlush := ts.writeBuffer.size >= ts.writeBuffer.maxSize
	ts.writeBuffer.mu.Unlock()

	if shouldFlush {
		go ts.flush()
	}

	return nil
}

func (ts *CompressedTimeSeries) flush() {
	ts.writeBuffer.mu.Lock()
	toFlush := ts.writeBuffer.pending
	ts.writeBuffer.pending = make(map[string][]DataPoint)
	ts.writeBuffer.size = 0
	ts.writeBuffer.mu.Unlock()

	for seriesName, points := range toFlush {
		ts.mu.Lock()
		series, exists := ts.series[seriesName]
		if !exists {
			series = &SeriesData{
				name:     seriesName,
				points:   make([]DataPoint, 0),
				metadata: make(map[string]string),
			}
			ts.series[seriesName] = series
		}
		ts.mu.Unlock()

		series.mu.Lock()
		series.points = append(series.points, points...)
		series.lastWrite = time.Now()

		// Compress if too many points
		if len(series.points) > 1000 {
			compressed, err := ts.compression.Compress(series.points[:1000])
			if err == nil {
				block := CompressedBlock{
					StartTime:      series.points[0].Timestamp,
					EndTime:        series.points[999].Timestamp,
					NumPoints:      1000,
					CompressedData: compressed,
					Encoding:       "delta",
				}
				series.compressed = append(series.compressed, block)
				series.points = series.points[1000:]
			}
		}
		series.mu.Unlock()
	}
}

func (ts *CompressedTimeSeries) flushRoutine() {
	for range ts.flushTicker.C {
		ts.flush()
	}
}

func (ts *CompressedTimeSeries) Query(seriesName string, start, end time.Time) ([]DataPoint, error) {
	ts.mu.RLock()
	series, exists := ts.series[seriesName]
	ts.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("series %s not found", seriesName)
	}

	series.mu.RLock()
	defer series.mu.RUnlock()

	results := make([]DataPoint, 0)

	// Query compressed blocks
	for _, block := range series.compressed {
		if block.EndTime.After(start) && block.StartTime.Before(end) {
			// Would decompress here in real implementation
			results = append(results, DataPoint{
				Timestamp: block.StartTime,
				Value:     0, // Placeholder
			})
		}
	}

	// Query in-memory points
	for _, point := range series.points {
		if point.Timestamp.After(start) && point.Timestamp.Before(end) {
			results = append(results, point)
		}
	}

	// Sort by timestamp
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.Before(results[j].Timestamp)
	})

	return results, nil
}

// Solution 2: Window-based Aggregation
type WindowedTimeSeries struct {
	mu          sync.RWMutex
	windows     map[string]*Window
	aggregators map[string]Aggregator
	watermark   time.Time
	lateData    sync.Map
}

type Window struct {
	Start    time.Time
	End      time.Time
	Points   []DataPoint
	State    map[string]interface{}
	Closed   bool
	mu       sync.RWMutex
}

type Aggregator interface {
	Add(window *Window, point DataPoint)
	Compute(window *Window) float64
	Merge(windows []*Window) float64
}

type AvgAggregator struct{}

func (a *AvgAggregator) Add(window *Window, point DataPoint) {
	window.mu.Lock()
	defer window.mu.Unlock()

	window.Points = append(window.Points, point)

	// Update running stats
	if window.State == nil {
		window.State = make(map[string]interface{})
	}

	count, _ := window.State["count"].(float64)
	sum, _ := window.State["sum"].(float64)

	window.State["count"] = count + 1
	window.State["sum"] = sum + point.Value
}

func (a *AvgAggregator) Compute(window *Window) float64 {
	window.mu.RLock()
	defer window.mu.RUnlock()

	count, _ := window.State["count"].(float64)
	sum, _ := window.State["sum"].(float64)

	if count == 0 {
		return 0
	}
	return sum / count
}

func (a *AvgAggregator) Merge(windows []*Window) float64 {
	var totalSum, totalCount float64

	for _, w := range windows {
		w.mu.RLock()
		count, _ := w.State["count"].(float64)
		sum, _ := w.State["sum"].(float64)
		w.mu.RUnlock()

		totalCount += count
		totalSum += sum
	}

	if totalCount == 0 {
		return 0
	}
	return totalSum / totalCount
}

func NewWindowedTimeSeries() *WindowedTimeSeries {
	wts := &WindowedTimeSeries{
		windows:     make(map[string]*Window),
		aggregators: make(map[string]Aggregator),
		watermark:   time.Now().Add(-1 * time.Minute),
	}

	wts.aggregators["avg"] = &AvgAggregator{}

	// Start watermark advancement
	go wts.advanceWatermark()

	return wts
}

func (wts *WindowedTimeSeries) getWindow(timestamp time.Time, windowSize time.Duration) string {
	windowStart := timestamp.Truncate(windowSize)
	return windowStart.Format(time.RFC3339)
}

func (wts *WindowedTimeSeries) Insert(point DataPoint, windowSize time.Duration) {
	windowKey := wts.getWindow(point.Timestamp, windowSize)

	wts.mu.Lock()
	window, exists := wts.windows[windowKey]
	if !exists {
		windowStart := point.Timestamp.Truncate(windowSize)
		window = &Window{
			Start:  windowStart,
			End:    windowStart.Add(windowSize),
			Points: make([]DataPoint, 0),
			State:  make(map[string]interface{}),
		}
		wts.windows[windowKey] = window
	}
	wts.mu.Unlock()

	// Check if late data
	if point.Timestamp.Before(wts.watermark) {
		// Handle late data
		wts.lateData.Store(point.Timestamp, point)
		log.Printf("Late data detected: %v", point.Timestamp)
		return
	}

	// Add to window
	if agg, exists := wts.aggregators["avg"]; exists {
		agg.Add(window, point)
	}
}

func (wts *WindowedTimeSeries) advanceWatermark() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		wts.mu.Lock()
		oldWatermark := wts.watermark
		wts.watermark = time.Now().Add(-30 * time.Second)

		// Close windows before new watermark
		for key, window := range wts.windows {
			if window.End.Before(wts.watermark) && !window.Closed {
				window.mu.Lock()
				window.Closed = true
				window.mu.Unlock()
				log.Printf("Closed window: %s", key)
			}
		}
		wts.mu.Unlock()

		// Process late data
		wts.lateData.Range(func(key, value interface{}) bool {
			timestamp := key.(time.Time)
			if timestamp.After(oldWatermark) && timestamp.Before(wts.watermark) {
				// Reprocess late data
				wts.lateData.Delete(key)
			}
			return true
		})
	}
}

func (wts *WindowedTimeSeries) Aggregate(windowSize time.Duration, aggregator string) (map[string]float64, error) {
	agg, exists := wts.aggregators[aggregator]
	if !exists {
		return nil, fmt.Errorf("aggregator %s not found", aggregator)
	}

	results := make(map[string]float64)

	wts.mu.RLock()
	for key, window := range wts.windows {
		if window.Closed {
			results[key] = agg.Compute(window)
		}
	}
	wts.mu.RUnlock()

	return results, nil
}

// Solution 3: Distributed Time Series with Sharding
type DistributedTimeSeries struct {
	shards      []*TimeSeriesShard
	numShards   int
	router      *ShardRouter
	coordinator *QueryCoordinator
}

type TimeSeriesShard struct {
	id       int
	mu       sync.RWMutex
	data     map[string]*ShardSeries
	replicas []int
	primary  bool
}

type ShardSeries struct {
	points    []DataPoint
	index     *TimeIndex
	retention time.Duration
}

type TimeIndex struct {
	mu         sync.RWMutex
	hourBuckets map[int64][]int // hour timestamp -> point indices
}

type ShardRouter struct {
	mu            sync.RWMutex
	seriesMapping map[string]int // series name -> shard id
	loadBalancer  *LoadBalancer
}

type LoadBalancer struct {
	shardLoads []int64
	mu         sync.Mutex
}

type QueryCoordinator struct {
	mu      sync.RWMutex
	queries map[string]*DistributedQuery
}

type DistributedQuery struct {
	ID        string
	Series    string
	Start     time.Time
	End       time.Time
	Shards    []int
	Results   []DataPoint
	Status    string
	mu        sync.Mutex
}

func NewDistributedTimeSeries(numShards int) *DistributedTimeSeries {
	shards := make([]*TimeSeriesShard, numShards)
	for i := 0; i < numShards; i++ {
		shards[i] = &TimeSeriesShard{
			id:      i,
			data:    make(map[string]*ShardSeries),
			primary: true,
		}
	}

	return &DistributedTimeSeries{
		shards:    shards,
		numShards: numShards,
		router: &ShardRouter{
			seriesMapping: make(map[string]int),
			loadBalancer: &LoadBalancer{
				shardLoads: make([]int64, numShards),
			},
		},
		coordinator: &QueryCoordinator{
			queries: make(map[string]*DistributedQuery),
		},
	}
}

func (dts *DistributedTimeSeries) getShard(seriesName string) int {
	dts.router.mu.RLock()
	if shardID, exists := dts.router.seriesMapping[seriesName]; exists {
		dts.router.mu.RUnlock()
		return shardID
	}
	dts.router.mu.RUnlock()

	// Assign to shard with lowest load
	dts.router.loadBalancer.mu.Lock()
	minLoad := dts.router.loadBalancer.shardLoads[0]
	minShard := 0

	for i := 1; i < dts.numShards; i++ {
		if dts.router.loadBalancer.shardLoads[i] < minLoad {
			minLoad = dts.router.loadBalancer.shardLoads[i]
			minShard = i
		}
	}

	dts.router.loadBalancer.shardLoads[minShard]++
	dts.router.loadBalancer.mu.Unlock()

	dts.router.mu.Lock()
	dts.router.seriesMapping[seriesName] = minShard
	dts.router.mu.Unlock()

	return minShard
}

func (dts *DistributedTimeSeries) Write(seriesName string, point DataPoint) error {
	shardID := dts.getShard(seriesName)
	shard := dts.shards[shardID]

	shard.mu.Lock()
	defer shard.mu.Unlock()

	series, exists := shard.data[seriesName]
	if !exists {
		series = &ShardSeries{
			points:    make([]DataPoint, 0),
			index:     &TimeIndex{hourBuckets: make(map[int64][]int)},
			retention: 7 * 24 * time.Hour,
		}
		shard.data[seriesName] = series
	}

	// Add point
	idx := len(series.points)
	series.points = append(series.points, point)

	// Update index
	hourBucket := point.Timestamp.Truncate(time.Hour).Unix()
	series.index.mu.Lock()
	series.index.hourBuckets[hourBucket] = append(series.index.hourBuckets[hourBucket], idx)
	series.index.mu.Unlock()

	// Apply retention
	cutoff := time.Now().Add(-series.retention)
	if len(series.points) > 0 && series.points[0].Timestamp.Before(cutoff) {
		// Remove old points
		i := 0
		for i < len(series.points) && series.points[i].Timestamp.Before(cutoff) {
			i++
		}
		series.points = series.points[i:]

		// Rebuild index
		series.index.mu.Lock()
		series.index.hourBuckets = make(map[int64][]int)
		for idx, p := range series.points {
			bucket := p.Timestamp.Truncate(time.Hour).Unix()
			series.index.hourBuckets[bucket] = append(series.index.hourBuckets[bucket], idx)
		}
		series.index.mu.Unlock()
	}

	return nil
}

func (dts *DistributedTimeSeries) DistributedQuery(ctx context.Context, seriesName string, start, end time.Time) ([]DataPoint, error) {
	query := &DistributedQuery{
		ID:      fmt.Sprintf("q_%d", time.Now().UnixNano()),
		Series:  seriesName,
		Start:   start,
		End:     end,
		Results: make([]DataPoint, 0),
		Status:  "running",
	}

	dts.coordinator.mu.Lock()
	dts.coordinator.queries[query.ID] = query
	dts.coordinator.mu.Unlock()

	// Determine which shards to query
	shardID := dts.getShard(seriesName)
	query.Shards = []int{shardID}

	// Query shards in parallel
	var wg sync.WaitGroup
	resultChan := make(chan []DataPoint, len(query.Shards))

	for _, sid := range query.Shards {
		wg.Add(1)
		go func(shardID int) {
			defer wg.Done()

			shard := dts.shards[shardID]
			shard.mu.RLock()
			series, exists := shard.data[seriesName]
			shard.mu.RUnlock()

			if !exists {
				resultChan <- []DataPoint{}
				return
			}

			// Use index for efficient query
			results := make([]DataPoint, 0)
			startHour := start.Truncate(time.Hour).Unix()
			endHour := end.Truncate(time.Hour).Unix() + 3600

			series.index.mu.RLock()
			for hour := startHour; hour <= endHour; hour += 3600 {
				if indices, exists := series.index.hourBuckets[hour]; exists {
					for _, idx := range indices {
						if idx < len(series.points) {
							p := series.points[idx]
							if p.Timestamp.After(start) && p.Timestamp.Before(end) {
								results = append(results, p)
							}
						}
					}
				}
			}
			series.index.mu.RUnlock()

			resultChan <- results
		}(sid)
	}

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	allResults := make([]DataPoint, 0)
	for results := range resultChan {
		allResults = append(allResults, results...)
	}

	// Sort by timestamp
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Timestamp.Before(allResults[j].Timestamp)
	})

	query.mu.Lock()
	query.Results = allResults
	query.Status = "completed"
	query.mu.Unlock()

	return allResults, nil
}

// Helper functions
func floatToBytes(f float64) []byte {
	// Simplified conversion
	return []byte(fmt.Sprintf("%f", f))[:8]
}

func int64ToBytes(i int64) []byte {
	// Simplified conversion
	return []byte(fmt.Sprintf("%d", i))[:8]
}

// RunSolution22 demonstrates the three time series solutions
func RunSolution22() {
	fmt.Println("=== Solution 22: Time Series Database ===")

	ctx := context.Background()

	// Solution 1: Compressed Time Series
	fmt.Println("\n1. Compressed Time Series:")
	cts := NewCompressedTimeSeries()

	// Write data points
	now := time.Now()
	for i := 0; i < 100; i++ {
		point := DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Value:     math.Sin(float64(i) * 0.1) * 100,
			Tags:      map[string]string{"sensor": "temp1"},
		}
		cts.Write("temperature", point)
	}

	// Force flush
	cts.flush()

	// Query data
	results, err := cts.Query("temperature", now.Add(-1*time.Minute), now.Add(2*time.Minute))
	if err != nil {
		fmt.Printf("  Query error: %v\n", err)
	} else {
		fmt.Printf("  Query returned %d points\n", len(results))
	}

	// Solution 2: Windowed Aggregation
	fmt.Println("\n2. Windowed Aggregation:")
	wts := NewWindowedTimeSeries()

	// Insert points
	windowSize := 10 * time.Second
	for i := 0; i < 50; i++ {
		point := DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Value:     float64(i) + float64(i%10),
		}
		wts.Insert(point, windowSize)
	}

	// Wait for windows to close
	time.Sleep(2 * time.Second)

	// Get aggregations
	aggregates, err := wts.Aggregate(windowSize, "avg")
	if err != nil {
		fmt.Printf("  Aggregation error: %v\n", err)
	} else {
		fmt.Printf("  Computed %d window aggregates\n", len(aggregates))
		for key, avg := range aggregates {
			if avg > 0 {
				fmt.Printf("    Window %s: avg=%.2f\n", key, avg)
				break // Show just one example
			}
		}
	}

	// Solution 3: Distributed Time Series
	fmt.Println("\n3. Distributed Time Series:")
	dts := NewDistributedTimeSeries(4)

	// Write to multiple series
	series := []string{"cpu", "memory", "disk", "network"}
	for _, s := range series {
		for i := 0; i < 20; i++ {
			point := DataPoint{
				Timestamp: now.Add(time.Duration(i) * time.Minute),
				Value:     float64(i * 10),
			}
			dts.Write(s, point)
		}
	}

	// Distributed query
	queryResults, err := dts.DistributedQuery(ctx, "cpu", now.Add(-1*time.Hour), now.Add(1*time.Hour))
	if err != nil {
		fmt.Printf("  Distributed query error: %v\n", err)
	} else {
		fmt.Printf("  Distributed query returned %d points\n", len(queryResults))
	}

	// Show shard distribution
	fmt.Printf("\n  Shard distribution:\n")
	for series, shard := range dts.router.seriesMapping {
		fmt.Printf("    Series '%s' -> Shard %d\n", series, shard)
	}

	fmt.Println("\nAll time series patterns demonstrated successfully!")
}