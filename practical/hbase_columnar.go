package practical

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// HBaseExample demonstrates wide column store patterns similar to HBase
// This example simulates HBase patterns without actual connection for educational purposes

// Cell represents a single cell in HBase
type Cell struct {
	Value     []byte
	Timestamp int64
	Version   int
}

// Column represents a column with multiple versions
type Column struct {
	Family    string
	Qualifier string
	Cells     []Cell // Sorted by timestamp, newest first
}

// Row represents an HBase row
type Row struct {
	Key     []byte
	Columns map[string]map[string]*Column // family -> qualifier -> column
	mu      sync.RWMutex
}

// HBaseTable simulates an HBase table
type HBaseTable struct {
	mu           sync.RWMutex
	rows         map[string]*Row
	families     map[string]*ColumnFamily
	regions      []*Region
	maxVersions  int
	blockCache   *BlockCache
	writeBuffer  *WriteBuffer
	stats        *TableStats
}

// ColumnFamily represents column family configuration
type ColumnFamily struct {
	Name         string
	MaxVersions  int
	Compression  string
	BlockSize    int
	TimeToLive   time.Duration
}

// Region represents a table region (range of rows)
type Region struct {
	ID        string
	StartKey  string
	EndKey    string
	RowCount  atomic.Int64
	DataSize  atomic.Int64
	mu        sync.RWMutex
}

// BlockCache simulates HBase block cache
type BlockCache struct {
	mu       sync.RWMutex
	cache    map[string]*CacheEntry
	capacity int
	size     atomic.Int64
	hits     atomic.Int64
	misses   atomic.Int64
}

// CacheEntry represents cached data
type CacheEntry struct {
	Data      interface{}
	Size      int64
	AccessTime time.Time
}

// WriteBuffer simulates HBase write buffer (MemStore)
type WriteBuffer struct {
	mu       sync.Mutex
	buffer   []*WriteEntry
	size     atomic.Int64
	maxSize  int64
	flushCh  chan struct{}
}

// WriteEntry represents a pending write
type WriteEntry struct {
	RowKey    string
	Family    string
	Qualifier string
	Value     []byte
	Timestamp int64
}

// TableStats tracks table operation statistics
type TableStats struct {
	Reads       atomic.Int64
	Writes      atomic.Int64
	Scans       atomic.Int64
	CacheHits   atomic.Int64
	CacheMisses atomic.Int64
	Flushes     atomic.Int64
}

// NewHBaseTable creates a new simulated HBase table
func NewHBaseTable(families []string, maxVersions int) *HBaseTable {
	table := &HBaseTable{
		rows:        make(map[string]*Row),
		families:    make(map[string]*ColumnFamily),
		regions:     make([]*Region, 0),
		maxVersions: maxVersions,
		blockCache: &BlockCache{
			cache:    make(map[string]*CacheEntry),
			capacity: 1000,
		},
		writeBuffer: &WriteBuffer{
			buffer:  make([]*WriteEntry, 0),
			maxSize: 1024 * 1024, // 1MB
			flushCh: make(chan struct{}, 1),
		},
		stats: &TableStats{},
	}

	// Initialize column families
	for _, family := range families {
		table.families[family] = &ColumnFamily{
			Name:        family,
			MaxVersions: maxVersions,
			Compression: "SNAPPY",
			BlockSize:   65536,
			TimeToLive:  0, // No TTL by default
		}
	}

	// Create initial region
	table.regions = append(table.regions, &Region{
		ID:       "region001",
		StartKey: "",
		EndKey:   "",
	})

	// Start background flush worker
	go table.backgroundFlush()

	return table
}

// Put writes a single cell
func (ht *HBaseTable) Put(ctx context.Context, rowKey string, family string, qualifier string, value []byte) error {
	ht.stats.Writes.Add(1)

	// Check if family exists
	if _, exists := ht.families[family]; !exists {
		return fmt.Errorf("column family %s does not exist", family)
	}

	// Add to write buffer
	entry := &WriteEntry{
		RowKey:    rowKey,
		Family:    family,
		Qualifier: qualifier,
		Value:     value,
		Timestamp: time.Now().UnixNano(),
	}

	ht.writeBuffer.mu.Lock()
	ht.writeBuffer.buffer = append(ht.writeBuffer.buffer, entry)
	size := ht.writeBuffer.size.Add(int64(len(value)))
	shouldFlush := size >= ht.writeBuffer.maxSize
	ht.writeBuffer.mu.Unlock()

	if shouldFlush {
		select {
		case ht.writeBuffer.flushCh <- struct{}{}:
		default:
		}
	}

	return nil
}

// Get retrieves a single cell or row
func (ht *HBaseTable) Get(ctx context.Context, rowKey string, family string, qualifier string) (*Cell, error) {
	ht.stats.Reads.Add(1)

	// Check cache first
	cacheKey := fmt.Sprintf("%s:%s:%s", rowKey, family, qualifier)
	if cached := ht.getFromCache(cacheKey); cached != nil {
		ht.stats.CacheHits.Add(1)
		if cell, ok := cached.(*Cell); ok {
			return cell, nil
		}
	}
	ht.stats.CacheMisses.Add(1)

	ht.mu.RLock()
	row, exists := ht.rows[rowKey]
	ht.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("row not found: %s", rowKey)
	}

	row.mu.RLock()
	defer row.mu.RUnlock()

	if familyColumns, exists := row.Columns[family]; exists {
		if column, exists := familyColumns[qualifier]; exists && len(column.Cells) > 0 {
			cell := &column.Cells[0] // Return latest version
			ht.putToCache(cacheKey, cell, int64(len(cell.Value)))
			return cell, nil
		}
	}

	return nil, fmt.Errorf("cell not found: %s:%s:%s", rowKey, family, qualifier)
}

// Scan performs a range scan
func (ht *HBaseTable) Scan(ctx context.Context, startRow string, stopRow string, limit int) ([]*Row, error) {
	ht.stats.Scans.Add(1)

	ht.mu.RLock()
	defer ht.mu.RUnlock()

	// Get all row keys and sort them
	keys := make([]string, 0, len(ht.rows))
	for key := range ht.rows {
		if (startRow == "" || key >= startRow) && (stopRow == "" || key < stopRow) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	// Apply limit
	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
	}

	// Collect rows
	results := make([]*Row, 0, len(keys))
	for _, key := range keys {
		if row, exists := ht.rows[key]; exists {
			results = append(results, row)
		}
	}

	return results, nil
}

// BatchPut performs batch writes
func (ht *HBaseTable) BatchPut(ctx context.Context, puts []struct {
	RowKey    string
	Family    string
	Qualifier string
	Value     []byte
}) error {
	// Use goroutines for parallel writes
	var wg sync.WaitGroup
	errors := make([]error, len(puts))

	semaphore := make(chan struct{}, 10) // Limit concurrent writes

	for i, put := range puts {
		wg.Add(1)
		go func(idx int, p struct {
			RowKey    string
			Family    string
			Qualifier string
			Value     []byte
		}) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			errors[idx] = ht.Put(ctx, p.RowKey, p.Family, p.Qualifier, p.Value)
		}(i, put)
	}

	wg.Wait()

	// Check for errors
	for _, err := range errors {
		if err != nil {
			return fmt.Errorf("batch put had errors: %w", err)
		}
	}

	return nil
}

// flush writes buffered data to storage
func (ht *HBaseTable) flush() {
	ht.writeBuffer.mu.Lock()
	entries := ht.writeBuffer.buffer
	ht.writeBuffer.buffer = make([]*WriteEntry, 0)
	ht.writeBuffer.size.Store(0)
	ht.writeBuffer.mu.Unlock()

	if len(entries) == 0 {
		return
	}

	ht.stats.Flushes.Add(1)

	// Group by row key for efficiency
	rowGroups := make(map[string][]*WriteEntry)
	for _, entry := range entries {
		rowGroups[entry.RowKey] = append(rowGroups[entry.RowKey], entry)
	}

	ht.mu.Lock()
	defer ht.mu.Unlock()

	for rowKey, rowEntries := range rowGroups {
		row, exists := ht.rows[rowKey]
		if !exists {
			row = &Row{
				Key:     []byte(rowKey),
				Columns: make(map[string]map[string]*Column),
			}
			ht.rows[rowKey] = row
		}

		row.mu.Lock()
		for _, entry := range rowEntries {
			if row.Columns[entry.Family] == nil {
				row.Columns[entry.Family] = make(map[string]*Column)
			}

			column, exists := row.Columns[entry.Family][entry.Qualifier]
			if !exists {
				column = &Column{
					Family:    entry.Family,
					Qualifier: entry.Qualifier,
					Cells:     make([]Cell, 0),
				}
				row.Columns[entry.Family][entry.Qualifier] = column
			}

			// Add new cell
			cell := Cell{
				Value:     entry.Value,
				Timestamp: entry.Timestamp,
				Version:   len(column.Cells) + 1,
			}
			column.Cells = append([]Cell{cell}, column.Cells...)

			// Trim to max versions
			if len(column.Cells) > ht.maxVersions {
				column.Cells = column.Cells[:ht.maxVersions]
			}
		}
		row.mu.Unlock()

		// Update region statistics
		for _, region := range ht.regions {
			if rowKey >= region.StartKey && (region.EndKey == "" || rowKey < region.EndKey) {
				region.RowCount.Add(1)
				region.DataSize.Add(int64(len(rowEntries) * 100)) // Approximate size
				break
			}
		}
	}
}

// backgroundFlush runs periodic flush operations
func (ht *HBaseTable) backgroundFlush() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ht.flush()
		case <-ht.writeBuffer.flushCh:
			ht.flush()
		}
	}
}

// getFromCache retrieves from block cache
func (ht *HBaseTable) getFromCache(key string) interface{} {
	ht.blockCache.mu.RLock()
	defer ht.blockCache.mu.RUnlock()

	if entry, exists := ht.blockCache.cache[key]; exists {
		entry.AccessTime = time.Now()
		ht.blockCache.hits.Add(1)
		return entry.Data
	}

	ht.blockCache.misses.Add(1)
	return nil
}

// putToCache adds to block cache
func (ht *HBaseTable) putToCache(key string, data interface{}, size int64) {
	ht.blockCache.mu.Lock()
	defer ht.blockCache.mu.Unlock()

	// Simple LRU eviction if needed
	if len(ht.blockCache.cache) >= ht.blockCache.capacity {
		// Find oldest entry
		var oldestKey string
		var oldestTime time.Time
		for k, v := range ht.blockCache.cache {
			if oldestTime.IsZero() || v.AccessTime.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.AccessTime
			}
		}
		if oldestKey != "" {
			delete(ht.blockCache.cache, oldestKey)
		}
	}

	ht.blockCache.cache[key] = &CacheEntry{
		Data:       data,
		Size:       size,
		AccessTime: time.Now(),
	}
}

// Coprocessor simulates HBase coprocessor for server-side processing
type Coprocessor struct {
	table *HBaseTable
}

// NewCoprocessor creates a new coprocessor
func NewCoprocessor(table *HBaseTable) *Coprocessor {
	return &Coprocessor{table: table}
}

// Aggregate performs server-side aggregation
func (cp *Coprocessor) Aggregate(ctx context.Context, startRow string, stopRow string, family string, qualifier string) (int64, error) {
	rows, err := cp.table.Scan(ctx, startRow, stopRow, 0)
	if err != nil {
		return 0, err
	}

	var sum int64
	for _, row := range rows {
		row.mu.RLock()
		if familyColumns, exists := row.Columns[family]; exists {
			if column, exists := familyColumns[qualifier]; exists && len(column.Cells) > 0 {
				// Assume values are numeric for this example
				sum += int64(len(column.Cells[0].Value))
			}
		}
		row.mu.RUnlock()
	}

	return sum, nil
}

// RunHBaseExample demonstrates wide column store patterns
func RunHBaseExample() {
	fmt.Println("=== HBase Wide Column Store Example ===\n")

	// Create table with column families
	families := []string{"personal", "professional", "metadata"}
	table := NewHBaseTable(families, 3) // Keep 3 versions
	ctx := context.Background()

	// 1. Basic Put and Get operations
	fmt.Println("1. Basic Put and Get Operations:")

	// Put some data
	table.Put(ctx, "user001", "personal", "name", []byte("John Doe"))
	table.Put(ctx, "user001", "personal", "email", []byte("john@example.com"))
	table.Put(ctx, "user001", "professional", "title", []byte("Software Engineer"))
	table.Put(ctx, "user001", "professional", "company", []byte("Tech Corp"))

	// Force flush to storage
	table.flush()

	// Get data
	cell, err := table.Get(ctx, "user001", "personal", "name")
	if err != nil {
		fmt.Printf("Error getting cell: %v\n", err)
	} else {
		fmt.Printf("Retrieved: %s\n", string(cell.Value))
	}

	// 2. Version history
	fmt.Println("\n2. Version History:")

	// Update the same cell multiple times
	table.Put(ctx, "user001", "professional", "title", []byte("Senior Engineer"))
	time.Sleep(10 * time.Millisecond)
	table.Put(ctx, "user001", "professional", "title", []byte("Lead Engineer"))
	time.Sleep(10 * time.Millisecond)
	table.Put(ctx, "user001", "professional", "title", []byte("Principal Engineer"))

	table.flush()

	// Get with version history
	table.mu.RLock()
	if row, exists := table.rows["user001"]; exists {
		row.mu.RLock()
		if column, exists := row.Columns["professional"]["title"]; exists {
			fmt.Printf("Version history for professional:title:\n")
			for _, cell := range column.Cells {
				fmt.Printf("  Version %d: %s (timestamp: %d)\n",
					cell.Version, string(cell.Value), cell.Timestamp)
			}
		}
		row.mu.RUnlock()
	}
	table.mu.RUnlock()

	// 3. Batch operations
	fmt.Println("\n3. Batch Operations:")

	// Prepare batch data
	batchData := make([]struct {
		RowKey    string
		Family    string
		Qualifier string
		Value     []byte
	}, 0)

	for i := 2; i <= 10; i++ {
		rowKey := fmt.Sprintf("user%03d", i)
		batchData = append(batchData, struct {
			RowKey    string
			Family    string
			Qualifier string
			Value     []byte
		}{
			RowKey:    rowKey,
			Family:    "personal",
			Qualifier: "name",
			Value:     []byte(fmt.Sprintf("User %d", i)),
		})
		batchData = append(batchData, struct {
			RowKey    string
			Family    string
			Qualifier string
			Value     []byte
		}{
			RowKey:    rowKey,
			Family:    "metadata",
			Qualifier: "created",
			Value:     []byte(time.Now().Format(time.RFC3339)),
		})
	}

	// Execute batch put
	if err := table.BatchPut(ctx, batchData); err != nil {
		fmt.Printf("Batch put error: %v\n", err)
	} else {
		fmt.Printf("Successfully inserted %d cells\n", len(batchData))
	}

	table.flush()

	// 4. Range scan
	fmt.Println("\n4. Range Scan:")

	rows, err := table.Scan(ctx, "user001", "user006", 5)
	if err != nil {
		fmt.Printf("Scan error: %v\n", err)
	} else {
		fmt.Printf("Scanned %d rows:\n", len(rows))
		for _, row := range rows {
			fmt.Printf("  Row: %s\n", string(row.Key))
		}
	}

	// 5. Coprocessor aggregation
	fmt.Println("\n5. Coprocessor Aggregation:")

	coprocessor := NewCoprocessor(table)

	// Add some numeric data
	for j := 1; j <= 5; j++ {
		rowKey := fmt.Sprintf("metric%03d", j)
		value := make([]byte, j*10) // Size represents the metric value
		table.Put(ctx, rowKey, "metadata", "value", value)
	}
	table.flush()

	// Perform aggregation
	sum, err := coprocessor.Aggregate(ctx, "metric001", "metric999", "metadata", "value")
	if err != nil {
		fmt.Printf("Aggregation error: %v\n", err)
	} else {
		fmt.Printf("Aggregated sum: %d\n", sum)
	}

	// 6. Cache performance
	fmt.Println("\n6. Cache Performance:")

	// Perform multiple reads to test cache
	for i := 0; i < 10; i++ {
		table.Get(ctx, "user001", "personal", "name")
	}

	fmt.Printf("Cache hits: %d\n", table.blockCache.hits.Load())
	fmt.Printf("Cache misses: %d\n", table.blockCache.misses.Load())
	hitRate := float64(table.blockCache.hits.Load()) / float64(table.blockCache.hits.Load()+table.blockCache.misses.Load()) * 100
	fmt.Printf("Cache hit rate: %.2f%%\n", hitRate)

	// 7. Region statistics
	fmt.Println("\n7. Region Statistics:")

	for _, region := range table.regions {
		fmt.Printf("Region %s:\n", region.ID)
		fmt.Printf("  Row count: %d\n", region.RowCount.Load())
		fmt.Printf("  Data size: %d bytes\n", region.DataSize.Load())
	}

	// Print overall statistics
	fmt.Println("\n=== Table Statistics ===")
	fmt.Printf("Total Reads: %d\n", table.stats.Reads.Load())
	fmt.Printf("Total Writes: %d\n", table.stats.Writes.Load())
	fmt.Printf("Total Scans: %d\n", table.stats.Scans.Load())
	fmt.Printf("Total Flushes: %d\n", table.stats.Flushes.Load())
	fmt.Printf("Cache Hits: %d\n", table.stats.CacheHits.Load())
	fmt.Printf("Cache Misses: %d\n", table.stats.CacheMisses.Load())
}