package solutions

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Solution23 demonstrates three approaches to columnar storage:
// 1. Column-oriented storage with compression and encoding
// 2. Wide column store with column families
// 3. Distributed columnar storage with replication

// Solution 1: Column-oriented Storage with Compression
type ColumnarStorage struct {
	mu         sync.RWMutex
	columns    map[string]*Column
	rowGroups  []*RowGroup
	wal        *WriteAheadLog
	compactor  *Compactor
	statistics *StorageStats
}

type Column struct {
	Name     string
	DataType string
	Values   []interface{}
	Encoding EncodingType
	Stats    *ColumnStats
	mu       sync.RWMutex
}

type ColumnStats struct {
	Min        interface{}
	Max        interface{}
	NullCount  int64
	DistinctCount int64
	Cardinality float64
}

type RowGroup struct {
	ID        int
	StartRow  int64
	EndRow    int64
	Columns   map[string]*ColumnChunk
	Metadata  *RowGroupMetadata
	mu        sync.RWMutex
}

type ColumnChunk struct {
	Data       []byte
	Encoding   EncodingType
	Compressed bool
	Stats      *ColumnStats
}

type RowGroupMetadata struct {
	NumRows      int64
	CompressedSize int64
	UncompressedSize int64
	CreatedAt    time.Time
}

type EncodingType int

const (
	PlainEncoding EncodingType = iota
	DictionaryEncoding
	RunLengthEncoding
	DeltaEncoding
)

type WriteAheadLog struct {
	mu      sync.Mutex
	entries []WALEntry
	file    string
}

type WALEntry struct {
	Timestamp time.Time
	Operation string
	Data      map[string]interface{}
}

type Compactor struct {
	running int32
	trigger chan struct{}
}

type StorageStats struct {
	TotalRows     int64
	TotalColumns  int64
	CompressRatio float64
	mu            sync.RWMutex
}

func NewColumnarStorage() *ColumnarStorage {
	cs := &ColumnarStorage{
		columns:   make(map[string]*Column),
		rowGroups: make([]*RowGroup, 0),
		wal: &WriteAheadLog{
			entries: make([]WALEntry, 0),
			file:    "columnar.wal",
		},
		compactor: &Compactor{
			trigger: make(chan struct{}, 1),
		},
		statistics: &StorageStats{},
	}

	// Start compactor
	go cs.runCompactor()

	return cs
}

func (cs *ColumnarStorage) CreateColumn(name string, dataType string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.columns[name] = &Column{
		Name:     name,
		DataType: dataType,
		Values:   make([]interface{}, 0),
		Encoding: PlainEncoding,
		Stats:    &ColumnStats{},
	}

	atomic.AddInt64(&cs.statistics.TotalColumns, 1)
}

func (cs *ColumnarStorage) Insert(row map[string]interface{}) error {
	// Write to WAL first
	cs.wal.mu.Lock()
	cs.wal.entries = append(cs.wal.entries, WALEntry{
		Timestamp: time.Now(),
		Operation: "INSERT",
		Data:      row,
	})
	cs.wal.mu.Unlock()

	// Insert into columns
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	for colName, value := range row {
		if col, exists := cs.columns[colName]; exists {
			col.mu.Lock()
			col.Values = append(col.Values, value)
			cs.updateColumnStats(col, value)
			col.mu.Unlock()
		}
	}

	atomic.AddInt64(&cs.statistics.TotalRows, 1)

	// Trigger compaction if needed
	if atomic.LoadInt64(&cs.statistics.TotalRows)%10000 == 0 {
		select {
		case cs.compactor.trigger <- struct{}{}:
		default:
		}
	}

	return nil
}

func (cs *ColumnarStorage) updateColumnStats(col *Column, value interface{}) {
	if col.Stats.Min == nil || value.(int) < col.Stats.Min.(int) {
		col.Stats.Min = value
	}
	if col.Stats.Max == nil || value.(int) > col.Stats.Max.(int) {
		col.Stats.Max = value
	}
}

func (cs *ColumnarStorage) Query(columns []string, predicate func(map[string]interface{}) bool) ([]map[string]interface{}, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	results := make([]map[string]interface{}, 0)

	// Get column data
	columnData := make(map[string][]interface{})
	var numRows int

	for _, colName := range columns {
		if col, exists := cs.columns[colName]; exists {
			col.mu.RLock()
			columnData[colName] = col.Values
			numRows = len(col.Values)
			col.mu.RUnlock()
		}
	}

	// Build rows and apply predicate
	for i := 0; i < numRows; i++ {
		row := make(map[string]interface{})
		for colName, values := range columnData {
			if i < len(values) {
				row[colName] = values[i]
			}
		}

		if predicate == nil || predicate(row) {
			results = append(results, row)
		}
	}

	return results, nil
}

func (cs *ColumnarStorage) createRowGroup(startRow, endRow int64) *RowGroup {
	rg := &RowGroup{
		ID:       len(cs.rowGroups),
		StartRow: startRow,
		EndRow:   endRow,
		Columns:  make(map[string]*ColumnChunk),
		Metadata: &RowGroupMetadata{
			NumRows:   endRow - startRow,
			CreatedAt: time.Now(),
		},
	}

	cs.mu.RLock()
	for name, col := range cs.columns {
		col.mu.RLock()
		chunk := cs.encodeColumn(col, int(startRow), int(endRow))
		rg.Columns[name] = chunk
		col.mu.RUnlock()
	}
	cs.mu.RUnlock()

	return rg
}

func (cs *ColumnarStorage) encodeColumn(col *Column, start, end int) *ColumnChunk {
	if end > len(col.Values) {
		end = len(col.Values)
	}

	values := col.Values[start:end]

	// Choose encoding based on data characteristics
	encoding := cs.selectEncoding(values)

	var encoded []byte
	switch encoding {
	case DictionaryEncoding:
		encoded = cs.dictionaryEncode(values)
	case RunLengthEncoding:
		encoded = cs.runLengthEncode(values)
	default:
		encoded = cs.plainEncode(values)
	}

	return &ColumnChunk{
		Data:       encoded,
		Encoding:   encoding,
		Compressed: true,
		Stats:      col.Stats,
	}
}

func (cs *ColumnarStorage) selectEncoding(values []interface{}) EncodingType {
	// Simple heuristic for encoding selection
	uniqueValues := make(map[interface{}]int)
	for _, v := range values {
		uniqueValues[v]++
	}

	cardinality := float64(len(uniqueValues)) / float64(len(values))

	if cardinality < 0.1 {
		return DictionaryEncoding
	} else if cardinality < 0.3 {
		return RunLengthEncoding
	}
	return PlainEncoding
}

func (cs *ColumnarStorage) dictionaryEncode(values []interface{}) []byte {
	dict := make(map[interface{}]uint32)
	encoded := make([]byte, 0)

	// Build dictionary
	var id uint32
	for _, v := range values {
		if _, exists := dict[v]; !exists {
			dict[v] = id
			id++
		}
	}

	// Encode values as dictionary indices
	for _, v := range values {
		idx := dict[v]
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, idx)
		encoded = append(encoded, buf...)
	}

	return encoded
}

func (cs *ColumnarStorage) runLengthEncode(values []interface{}) []byte {
	encoded := make([]byte, 0)

	if len(values) == 0 {
		return encoded
	}

	currentValue := values[0]
	count := 1

	for i := 1; i < len(values); i++ {
		if values[i] == currentValue {
			count++
		} else {
			// Encode run
			encoded = append(encoded, cs.encodeRun(currentValue, count)...)
			currentValue = values[i]
			count = 1
		}
	}

	// Encode last run
	encoded = append(encoded, cs.encodeRun(currentValue, count)...)

	return encoded
}

func (cs *ColumnarStorage) plainEncode(values []interface{}) []byte {
	encoded := make([]byte, 0)
	for _, v := range values {
		// Simplified encoding
		encoded = append(encoded, []byte(fmt.Sprintf("%v", v))...)
		encoded = append(encoded, 0) // null terminator
	}
	return encoded
}

func (cs *ColumnarStorage) encodeRun(value interface{}, count int) []byte {
	// Simplified run-length encoding
	result := make([]byte, 8)
	binary.LittleEndian.PutUint32(result[:4], uint32(count))
	// Encode value in next 4 bytes (simplified)
	return result
}

func (cs *ColumnarStorage) runCompactor() {
	for range cs.compactor.trigger {
		if !atomic.CompareAndSwapInt32(&cs.compactor.running, 0, 1) {
			continue
		}

		cs.compact()

		atomic.StoreInt32(&cs.compactor.running, 0)
	}
}

func (cs *ColumnarStorage) compact() {
	// Create new row group
	totalRows := atomic.LoadInt64(&cs.statistics.TotalRows)
	if totalRows > 0 && int64(len(cs.rowGroups))*10000 < totalRows {
		startRow := int64(len(cs.rowGroups)) * 10000
		endRow := startRow + 10000

		rg := cs.createRowGroup(startRow, endRow)

		cs.mu.Lock()
		cs.rowGroups = append(cs.rowGroups, rg)
		cs.mu.Unlock()

		log.Printf("Created row group %d with %d rows", rg.ID, rg.Metadata.NumRows)
	}
}

// Solution 2: Wide Column Store
type WideColumnStore struct {
	mu            sync.RWMutex
	columnFamilies map[string]*ColumnFamily
	memtable      *Memtable
	sstables      []*SSTable
	compactor     *LeveledCompactor
}

type ColumnFamily struct {
	Name       string
	Columns    map[string]*WideColumn
	Options    *CFOptions
	mu         sync.RWMutex
}

type WideColumn struct {
	Qualifier string
	Versions  []VersionedValue
	TTL       time.Duration
}

type VersionedValue struct {
	Value     []byte
	Timestamp int64
	Deleted   bool
}

type CFOptions struct {
	MaxVersions   int
	Compression   string
	BloomFilter   bool
	CacheEnabled  bool
}

type Memtable struct {
	mu       sync.RWMutex
	data     map[string]map[string]*WideColumn
	size     int64
	maxSize  int64
}

type SSTable struct {
	ID         int
	Level      int
	StartKey   string
	EndKey     string
	Size       int64
	BloomFilter *BloomFilter
	Index      map[string]int64
}

type BloomFilter struct {
	bits []byte
	size int
}

type LeveledCompactor struct {
	levels   []CompactionLevel
	running  int32
}

type CompactionLevel struct {
	Level    int
	SSTables []*SSTable
	MaxSize  int64
}

func NewWideColumnStore() *WideColumnStore {
	wcs := &WideColumnStore{
		columnFamilies: make(map[string]*ColumnFamily),
		memtable: &Memtable{
			data:    make(map[string]map[string]*WideColumn),
			maxSize: 100 * 1024 * 1024, // 100MB
		},
		sstables: make([]*SSTable, 0),
		compactor: &LeveledCompactor{
			levels: make([]CompactionLevel, 5),
		},
	}

	// Initialize compaction levels
	for i := 0; i < 5; i++ {
		wcs.compactor.levels[i] = CompactionLevel{
			Level:    i,
			SSTables: make([]*SSTable, 0),
			MaxSize:  int64(10*(i+1)) * 1024 * 1024, // 10MB, 20MB, ...
		}
	}

	// Start background compaction
	go wcs.runCompaction()

	return wcs
}

func (wcs *WideColumnStore) CreateColumnFamily(name string, options *CFOptions) {
	wcs.mu.Lock()
	defer wcs.mu.Unlock()

	wcs.columnFamilies[name] = &ColumnFamily{
		Name:    name,
		Columns: make(map[string]*WideColumn),
		Options: options,
	}
}

func (wcs *WideColumnStore) Put(rowKey string, cfName string, qualifier string, value []byte) error {
	// Write to memtable
	wcs.memtable.mu.Lock()
	defer wcs.memtable.mu.Unlock()

	if wcs.memtable.data[rowKey] == nil {
		wcs.memtable.data[rowKey] = make(map[string]*WideColumn)
	}

	columnKey := fmt.Sprintf("%s:%s", cfName, qualifier)

	if wcs.memtable.data[rowKey][columnKey] == nil {
		wcs.memtable.data[rowKey][columnKey] = &WideColumn{
			Qualifier: qualifier,
			Versions:  make([]VersionedValue, 0),
		}
	}

	// Add new version
	wcs.memtable.data[rowKey][columnKey].Versions = append(
		wcs.memtable.data[rowKey][columnKey].Versions,
		VersionedValue{
			Value:     value,
			Timestamp: time.Now().UnixNano(),
			Deleted:   false,
		},
	)

	wcs.memtable.size += int64(len(value))

	// Flush if memtable is full
	if wcs.memtable.size > wcs.memtable.maxSize {
		go wcs.flushMemtable()
	}

	return nil
}

func (wcs *WideColumnStore) Get(rowKey string, cfName string, qualifier string) ([]byte, error) {
	// Check memtable first
	wcs.memtable.mu.RLock()
	if row, exists := wcs.memtable.data[rowKey]; exists {
		columnKey := fmt.Sprintf("%s:%s", cfName, qualifier)
		if col, exists := row[columnKey]; exists && len(col.Versions) > 0 {
			// Return latest version
			latest := col.Versions[len(col.Versions)-1]
			wcs.memtable.mu.RUnlock()
			if !latest.Deleted {
				return latest.Value, nil
			}
			return nil, fmt.Errorf("deleted")
		}
	}
	wcs.memtable.mu.RUnlock()

	// Check SSTables
	for i := len(wcs.sstables) - 1; i >= 0; i-- {
		sstable := wcs.sstables[i]

		// Use bloom filter for quick check
		if sstable.BloomFilter != nil && !sstable.BloomFilter.Contains(rowKey) {
			continue
		}

		// Would read from disk here
		// For demo, return placeholder
		if sstable.StartKey <= rowKey && rowKey <= sstable.EndKey {
			return []byte("from_sstable"), nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (wcs *WideColumnStore) Scan(startKey, endKey string, limit int) ([]map[string]interface{}, error) {
	results := make([]map[string]interface{}, 0)

	// Scan memtable
	wcs.memtable.mu.RLock()
	keys := make([]string, 0, len(wcs.memtable.data))
	for k := range wcs.memtable.data {
		if k >= startKey && k <= endKey {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	for _, key := range keys {
		if len(results) >= limit {
			break
		}

		row := make(map[string]interface{})
		row["key"] = key

		for colKey, col := range wcs.memtable.data[key] {
			if len(col.Versions) > 0 {
				latest := col.Versions[len(col.Versions)-1]
				if !latest.Deleted {
					row[colKey] = latest.Value
				}
			}
		}

		results = append(results, row)
	}
	wcs.memtable.mu.RUnlock()

	return results, nil
}

func (wcs *WideColumnStore) flushMemtable() {
	if !atomic.CompareAndSwapInt32(&wcs.compactor.running, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&wcs.compactor.running, 0)

	// Create new SSTable
	sstable := &SSTable{
		ID:    len(wcs.sstables),
		Level: 0,
		Size:  wcs.memtable.size,
		Index: make(map[string]int64),
	}

	// Get sorted keys
	wcs.memtable.mu.RLock()
	keys := make([]string, 0, len(wcs.memtable.data))
	for k := range wcs.memtable.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(keys) > 0 {
		sstable.StartKey = keys[0]
		sstable.EndKey = keys[len(keys)-1]
	}
	wcs.memtable.mu.RUnlock()

	// Add to level 0
	wcs.compactor.levels[0].SSTables = append(wcs.compactor.levels[0].SSTables, sstable)

	// Clear memtable
	wcs.memtable.mu.Lock()
	wcs.memtable.data = make(map[string]map[string]*WideColumn)
	wcs.memtable.size = 0
	wcs.memtable.mu.Unlock()

	log.Printf("Flushed memtable to SSTable %d", sstable.ID)
}

func (wcs *WideColumnStore) runCompaction() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		wcs.compact()
	}
}

func (wcs *WideColumnStore) compact() {
	// Leveled compaction
	for i := 0; i < len(wcs.compactor.levels)-1; i++ {
		level := &wcs.compactor.levels[i]
		var totalSize int64

		for _, sst := range level.SSTables {
			totalSize += sst.Size
		}

		if totalSize > level.MaxSize {
			// Merge and promote to next level
			log.Printf("Compacting level %d", i)
			// Actual compaction logic would go here
		}
	}
}

func (bf *BloomFilter) Contains(key string) bool {
	// Simplified bloom filter
	hash := 0
	for _, c := range key {
		hash = (hash * 31 + int(c)) % bf.size
	}
	byteIdx := hash / 8
	bitIdx := uint(hash % 8)

	if byteIdx < len(bf.bits) {
		return (bf.bits[byteIdx] & (1 << bitIdx)) != 0
	}
	return false
}

// Solution 3: Distributed Columnar Storage
type DistributedColumnar struct {
	nodes       []*StorageNode
	coordinator *Coordinator
	replication int
	shardMap    *ConsistentHash
}

type StorageNode struct {
	ID          string
	Address     string
	Storage     *ColumnarStorage
	Replicas    []string
	Status      string
	LastHeartbeat time.Time
	mu          sync.RWMutex
}

type Coordinator struct {
	mu           sync.RWMutex
	nodes        map[string]*StorageNode
	shardAssignments map[string][]string // shard -> nodes
	rebalancing  int32
}

type ConsistentHash struct {
	mu       sync.RWMutex
	ring     map[uint32]string
	sortedKeys []uint32
	replicas int
}

func NewDistributedColumnar(numNodes int, replication int) *DistributedColumnar {
	nodes := make([]*StorageNode, numNodes)
	nodeMap := make(map[string]*StorageNode)

	for i := 0; i < numNodes; i++ {
		nodeID := fmt.Sprintf("node-%d", i)
		node := &StorageNode{
			ID:       nodeID,
			Address:  fmt.Sprintf("localhost:%d", 9000+i),
			Storage:  NewColumnarStorage(),
			Replicas: make([]string, 0),
			Status:   "active",
			LastHeartbeat: time.Now(),
		}
		nodes[i] = node
		nodeMap[nodeID] = node
	}

	dc := &DistributedColumnar{
		nodes:       nodes,
		coordinator: &Coordinator{
			nodes:            nodeMap,
			shardAssignments: make(map[string][]string),
		},
		replication: replication,
		shardMap:    NewConsistentHash(100), // 100 virtual nodes per physical node
	}

	// Add nodes to consistent hash
	for _, node := range nodes {
		dc.shardMap.AddNode(node.ID)
	}

	// Start health check
	go dc.healthCheck()

	return dc
}

func NewConsistentHash(replicas int) *ConsistentHash {
	return &ConsistentHash{
		ring:     make(map[uint32]string),
		replicas: replicas,
	}
}

func (ch *ConsistentHash) AddNode(nodeID string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	for i := 0; i < ch.replicas; i++ {
		hash := ch.hash(fmt.Sprintf("%s-%d", nodeID, i))
		ch.ring[hash] = nodeID
	}

	ch.updateSortedKeys()
}

func (ch *ConsistentHash) GetNodes(key string, n int) []string {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	if len(ch.sortedKeys) == 0 {
		return nil
	}

	hash := ch.hash(key)
	nodes := make([]string, 0, n)
	seen := make(map[string]bool)

	// Find starting position
	idx := ch.search(hash)

	for i := 0; i < len(ch.sortedKeys) && len(nodes) < n; i++ {
		keyIdx := (idx + i) % len(ch.sortedKeys)
		nodeID := ch.ring[ch.sortedKeys[keyIdx]]

		if !seen[nodeID] {
			nodes = append(nodes, nodeID)
			seen[nodeID] = true
		}
	}

	return nodes
}

func (ch *ConsistentHash) hash(key string) uint32 {
	// Simple hash function
	h := uint32(0)
	for _, c := range key {
		h = h*31 + uint32(c)
	}
	return h
}

func (ch *ConsistentHash) search(hash uint32) int {
	// Binary search
	left, right := 0, len(ch.sortedKeys)-1

	for left <= right {
		mid := (left + right) / 2
		if ch.sortedKeys[mid] == hash {
			return mid
		} else if ch.sortedKeys[mid] < hash {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}

	return left % len(ch.sortedKeys)
}

func (ch *ConsistentHash) updateSortedKeys() {
	ch.sortedKeys = make([]uint32, 0, len(ch.ring))
	for k := range ch.ring {
		ch.sortedKeys = append(ch.sortedKeys, k)
	}
	sort.Slice(ch.sortedKeys, func(i, j int) bool {
		return ch.sortedKeys[i] < ch.sortedKeys[j]
	})
}

func (dc *DistributedColumnar) Write(ctx context.Context, table string, row map[string]interface{}) error {
	// Get nodes for this row
	rowKey := fmt.Sprintf("%s-%v", table, row["id"])
	nodes := dc.shardMap.GetNodes(rowKey, dc.replication)

	if len(nodes) == 0 {
		return fmt.Errorf("no nodes available")
	}

	// Write to primary and replicas
	var wg sync.WaitGroup
	errors := make(chan error, len(nodes))

	for _, nodeID := range nodes {
		wg.Add(1)
		go func(nid string) {
			defer wg.Done()

			node := dc.coordinator.nodes[nid]
			if node == nil || node.Status != "active" {
				errors <- fmt.Errorf("node %s not available", nid)
				return
			}

			err := node.Storage.Insert(row)
			if err != nil {
				errors <- err
			}
		}(nodeID)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var writeErrors []error
	for err := range errors {
		if err != nil {
			writeErrors = append(writeErrors, err)
		}
	}

	// Require quorum writes
	if len(writeErrors) > dc.replication/2 {
		return fmt.Errorf("failed to achieve write quorum")
	}

	return nil
}

func (dc *DistributedColumnar) Query(ctx context.Context, table string, columns []string) ([]map[string]interface{}, error) {
	// Query all nodes and merge results
	resultChan := make(chan []map[string]interface{}, len(dc.nodes))
	var wg sync.WaitGroup

	for _, node := range dc.nodes {
		if node.Status != "active" {
			continue
		}

		wg.Add(1)
		go func(n *StorageNode) {
			defer wg.Done()

			results, err := n.Storage.Query(columns, nil)
			if err == nil {
				resultChan <- results
			}
		}(node)
	}

	wg.Wait()
	close(resultChan)

	// Merge results
	allResults := make([]map[string]interface{}, 0)
	seen := make(map[string]bool)

	for results := range resultChan {
		for _, row := range results {
			// Deduplicate based on row ID
			if id, exists := row["id"]; exists {
				key := fmt.Sprintf("%v", id)
				if !seen[key] {
					allResults = append(allResults, row)
					seen[key] = true
				}
			}
		}
	}

	return allResults, nil
}

func (dc *DistributedColumnar) healthCheck() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		dc.coordinator.mu.Lock()
		for _, node := range dc.coordinator.nodes {
			if time.Since(node.LastHeartbeat) > 30*time.Second {
				node.Status = "failed"
				log.Printf("Node %s marked as failed", node.ID)

				// Trigger rebalancing
				if atomic.CompareAndSwapInt32(&dc.coordinator.rebalancing, 0, 1) {
					go dc.rebalance()
				}
			}
		}
		dc.coordinator.mu.Unlock()
	}
}

func (dc *DistributedColumnar) rebalance() {
	defer atomic.StoreInt32(&dc.coordinator.rebalancing, 0)

	log.Println("Starting rebalancing...")

	// Redistribute data from failed nodes
	// Actual implementation would involve data movement

	log.Println("Rebalancing completed")
}

// RunSolution23 demonstrates the three columnar storage solutions
func RunSolution23() {
	fmt.Println("=== Solution 23: Columnar Storage ===")

	ctx := context.Background()

	// Solution 1: Column-oriented Storage
	fmt.Println("\n1. Column-oriented Storage:")
	cs := NewColumnarStorage()

	// Create columns
	cs.CreateColumn("id", "int")
	cs.CreateColumn("name", "string")
	cs.CreateColumn("value", "float")

	// Insert rows
	for i := 0; i < 100; i++ {
		row := map[string]interface{}{
			"id":    i,
			"name":  fmt.Sprintf("item-%d", i),
			"value": float64(i) * 1.5,
		}
		cs.Insert(row)
	}

	// Query specific columns
	results, err := cs.Query([]string{"id", "value"}, func(row map[string]interface{}) bool {
		if val, ok := row["id"].(int); ok {
			return val > 50
		}
		return false
	})

	if err != nil {
		fmt.Printf("  Query error: %v\n", err)
	} else {
		fmt.Printf("  Query returned %d rows\n", len(results))
	}

	fmt.Printf("  Statistics: %d columns, %d rows\n",
		atomic.LoadInt64(&cs.statistics.TotalColumns),
		atomic.LoadInt64(&cs.statistics.TotalRows))

	// Solution 2: Wide Column Store
	fmt.Println("\n2. Wide Column Store:")
	wcs := NewWideColumnStore()

	// Create column families
	wcs.CreateColumnFamily("profile", &CFOptions{
		MaxVersions:  3,
		Compression:  "snappy",
		BloomFilter:  true,
		CacheEnabled: true,
	})

	// Write data
	for i := 0; i < 50; i++ {
		rowKey := fmt.Sprintf("user-%d", i)
		wcs.Put(rowKey, "profile", "name", []byte(fmt.Sprintf("User %d", i)))
		wcs.Put(rowKey, "profile", "email", []byte(fmt.Sprintf("user%d@example.com", i)))
	}

	// Read data
	value, err := wcs.Get("user-10", "profile", "name")
	if err != nil {
		fmt.Printf("  Get error: %v\n", err)
	} else {
		fmt.Printf("  Retrieved: %s\n", string(value))
	}

	// Scan range
	scanResults, err := wcs.Scan("user-10", "user-20", 5)
	if err != nil {
		fmt.Printf("  Scan error: %v\n", err)
	} else {
		fmt.Printf("  Scan returned %d rows\n", len(scanResults))
	}

	// Solution 3: Distributed Columnar Storage
	fmt.Println("\n3. Distributed Columnar Storage:")
	dc := NewDistributedColumnar(3, 2)

	// Write distributed data
	for i := 0; i < 30; i++ {
		row := map[string]interface{}{
			"id":    i,
			"table": "metrics",
			"value": i * 10,
		}
		err := dc.Write(ctx, "metrics", row)
		if err != nil {
			fmt.Printf("  Write error: %v\n", err)
		}
	}

	// Distributed query
	distResults, err := dc.Query(ctx, "metrics", []string{"id", "value"})
	if err != nil {
		fmt.Printf("  Distributed query error: %v\n", err)
	} else {
		fmt.Printf("  Distributed query returned %d rows\n", len(distResults))
	}

	// Show node distribution
	fmt.Printf("\n  Node Status:\n")
	for _, node := range dc.nodes {
		fmt.Printf("    Node %s: %s (last heartbeat: %v ago)\n",
			node.ID, node.Status, time.Since(node.LastHeartbeat))
	}

	fmt.Println("\nAll columnar storage patterns demonstrated successfully!")
}