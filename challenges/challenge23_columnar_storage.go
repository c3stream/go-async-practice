package challenges

import (
	"context"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// Challenge 23: Columnar Storage Engine (HBase/Cassandra-style)
//
// 問題点:
// 1. カラムファミリーの並行更新問題
// 2. 行キーの衝突とホットスポット
// 3. メモリとディスクの同期問題
// 4. コンパクション戦略の不備
// 5. ブルームフィルターの実装バグ

type ColumnValue struct {
	Value     []byte
	Timestamp int64
	Version   int32
	Tombstone bool // 削除マーカー
}

type ColumnFamily struct {
	Name    string
	Columns map[string][]ColumnValue // column name -> versions
	mu      sync.RWMutex
}

type Row struct {
	Key      []byte
	Families map[string]*ColumnFamily
	mu       sync.RWMutex
	deleted  bool
}

type MemTable struct {
	rows      map[string]*Row
	size      int64
	maxSize   int64
	mu        sync.RWMutex
	lastFlush time.Time
}

type SSTable struct {
	ID         string
	MinKey     []byte
	MaxKey     []byte
	Size       int64
	Level      int
	BloomFilter *BloomFilter
	Index      map[string]int64 // row key -> offset
	Data       []byte
	mu         sync.RWMutex
}

type BloomFilter struct {
	bits     []bool
	size     int
	hashFunc []func([]byte) uint32
	mu       sync.RWMutex
}

type ColumnarStorageEngine struct {
	mu            sync.RWMutex
	memTable      *MemTable
	sstables      []*SSTable
	walLog        *WAL
	compactor     *Compactor
	cache         sync.Map // 問題: 無制限キャッシュ
	bloomFilters  map[string]*BloomFilter
	configuration Config
}

type WAL struct {
	mu      sync.Mutex
	entries []WALEntry
	file    string
	synced  bool
}

type WALEntry struct {
	Timestamp int64
	RowKey    []byte
	Family    string
	Column    string
	Value     []byte
	Type      string // put, delete
}

type Compactor struct {
	mu         sync.Mutex
	running    bool
	strategy   string // sizeTiered, leveled
	threshold  int
	inProgress map[string]bool // 問題: メモリリーク
}

type Config struct {
	MemTableSize      int64
	BlockSize         int
	CompactionRatio   float64
	MaxVersions       int
	BloomFilterSize   int
	CacheSize         int64
}

func NewColumnarStorageEngine() *ColumnarStorageEngine {
	return &ColumnarStorageEngine{
		memTable: &MemTable{
			rows:    make(map[string]*Row),
			maxSize: 64 * 1024 * 1024, // 64MB
		},
		sstables: []*SSTable{},
		walLog: &WAL{
			entries: []WALEntry{},
		},
		compactor: &Compactor{
			strategy:   "sizeTiered",
			threshold:  4,
			inProgress: make(map[string]bool),
		},
		bloomFilters: make(map[string]*BloomFilter),
		configuration: Config{
			MemTableSize:    64 * 1024 * 1024,
			BlockSize:       64 * 1024,
			CompactionRatio: 0.5,
			MaxVersions:     3,
			BloomFilterSize: 10000,
		},
	}
}

// 問題1: Put操作の並行性問題
func (cse *ColumnarStorageEngine) Put(rowKey []byte, family, column string, value []byte) error {
	// 問題: WAL書き込み前のクラッシュでデータロス

	cse.mu.RLock()
	memTable := cse.memTable
	cse.mu.RUnlock()

	// 問題: memTableのサイズチェックが不正確
	if memTable.size > memTable.maxSize {
		// 問題: フラッシュ処理がブロッキング
		cse.flushMemTable()
	}

	key := string(rowKey)

	memTable.mu.Lock()
	row, exists := memTable.rows[key]
	if !exists {
		row = &Row{
			Key:      rowKey,
			Families: make(map[string]*ColumnFamily),
		}
		memTable.rows[key] = row
	}
	memTable.mu.Unlock()

	// 問題: ロック順序によるデッドロック可能性
	row.mu.Lock()
	cf, exists := row.Families[family]
	if !exists {
		cf = &ColumnFamily{
			Name:    family,
			Columns: make(map[string][]ColumnValue),
		}
		row.Families[family] = cf
	}
	row.mu.Unlock()

	// 問題: カラムファミリーレベルのロック
	cf.mu.Lock()
	defer cf.mu.Unlock()

	// 問題: バージョン管理が不完全
	versions := cf.Columns[column]
	newValue := ColumnValue{
		Value:     value,
		Timestamp: time.Now().UnixNano(),
		Version:   int32(len(versions)),
	}

	// 問題: MaxVersionsの制限なし
	cf.Columns[column] = append([]ColumnValue{newValue}, versions...)

	// 問題: memTableサイズの更新忘れ
	// memTable.size += int64(len(value))

	// 問題: WAL書き込みが後
	cse.walLog.mu.Lock()
	cse.walLog.entries = append(cse.walLog.entries, WALEntry{
		Timestamp: time.Now().UnixNano(),
		RowKey:    rowKey,
		Family:    family,
		Column:    column,
		Value:     value,
		Type:      "put",
	})
	cse.walLog.mu.Unlock()

	return nil
}

// 問題2: Get操作の一貫性問題
func (cse *ColumnarStorageEngine) Get(rowKey []byte, family, column string) ([]byte, error) {
	key := string(rowKey)

	// 問題: MemTableチェック
	cse.memTable.mu.RLock()
	row, exists := cse.memTable.rows[key]
	cse.memTable.mu.RUnlock()

	if exists {
		row.mu.RLock()
		cf, exists := row.Families[family]
		row.mu.RUnlock()

		if exists {
			cf.mu.RLock()
			versions := cf.Columns[column]
			cf.mu.RUnlock()

			// 問題: Tombstoneチェックなし
			if len(versions) > 0 {
				return versions[0].Value, nil
			}
		}
	}

	// 問題: SSTableの検索が非効率
	for _, sstable := range cse.sstables {
		// 問題: BloomFilterチェックなし

		// 問題: 範囲チェックが不正確
		if string(sstable.MinKey) <= key && key <= string(sstable.MaxKey) {
			// 問題: 実際のデータ読み込み未実装
			if offset, exists := sstable.Index[key]; exists {
				_ = offset
				// データ読み込みロジックが必要
			}
		}
	}

	// 問題: キャッシュチェックの順序が間違い

	return nil, fmt.Errorf("key not found")
}

// 問題3: MemTableのフラッシュ
func (cse *ColumnarStorageEngine) flushMemTable() error {
	cse.mu.Lock()
	defer cse.mu.Unlock()

	// 問題: 新しいMemTable作成前に古いのをフラッシュ
	oldMemTable := cse.memTable
	cse.memTable = &MemTable{
		rows:    make(map[string]*Row),
		maxSize: cse.configuration.MemTableSize,
	}

	// 問題: 非同期フラッシュでデータロスの可能性
	go func() {
		sstable := cse.createSSTable(oldMemTable)

		cse.mu.Lock()
		cse.sstables = append(cse.sstables, sstable)
		cse.mu.Unlock()

		// 問題: WALクリアのタイミング
		cse.walLog.mu.Lock()
		cse.walLog.entries = []WALEntry{}
		cse.walLog.mu.Unlock()
	}()

	return nil
}

// 問題4: SSTable作成
func (cse *ColumnarStorageEngine) createSSTable(memTable *MemTable) *SSTable {
	sstable := &SSTable{
		ID:    fmt.Sprintf("sstable-%d", time.Now().UnixNano()),
		Index: make(map[string]int64),
		Level: 0,
	}

	// 問題: ソートなしでSSTable作成
	var data []byte
	offset := int64(0)

	for key, row := range memTable.rows {
		// 問題: シリアライゼーションが不完全
		rowData := cse.serializeRow(row)
		sstable.Index[key] = offset
		data = append(data, rowData...)
		offset += int64(len(rowData))

		// 問題: Min/MaxKeyの更新なし
	}

	sstable.Data = data

	// 問題: BloomFilter作成なし

	return sstable
}

// 問題5: コンパクション
func (cse *ColumnarStorageEngine) Compact(ctx context.Context) error {
	cse.compactor.mu.Lock()
	if cse.compactor.running {
		cse.compactor.mu.Unlock()
		return fmt.Errorf("compaction already running")
	}
	cse.compactor.running = true
	cse.compactor.mu.Unlock()

	// 問題: defer でrunningフラグリセット忘れ

	cse.mu.RLock()
	candidates := make([]*SSTable, 0)

	// 問題: コンパクション候補選択ロジックが単純すぎる
	for _, sstable := range cse.sstables {
		if sstable.Level == 0 {
			candidates = append(candidates, sstable)
		}
	}
	cse.mu.RUnlock()

	if len(candidates) < cse.compactor.threshold {
		return nil
	}

	// 問題: マージ処理が未実装
	// 重複排除、Tombstone処理、バージョン管理が必要

	return nil
}

// 問題6: BloomFilter実装
func NewBloomFilter(size int) *BloomFilter {
	return &BloomFilter{
		bits: make([]bool, size),
		size: size,
		// 問題: ハッシュ関数が未定義
	}
}

func (bf *BloomFilter) Add(key []byte) {
	// 問題: 単一ハッシュ関数のみ
	hash := simpleHash(key)
	index := hash % uint32(bf.size)

	bf.mu.Lock()
	bf.bits[index] = true
	bf.mu.Unlock()
}

func (bf *BloomFilter) Contains(key []byte) bool {
	hash := simpleHash(key)
	index := hash % uint32(bf.size)

	bf.mu.RLock()
	result := bf.bits[index]
	bf.mu.RUnlock()

	// 問題: 偽陽性率が高い
	return result
}

func simpleHash(data []byte) uint32 {
	var hash uint32
	for _, b := range data {
		hash = hash*31 + uint32(b)
	}
	return hash
}

func (cse *ColumnarStorageEngine) serializeRow(row *Row) []byte {
	// 簡単なシリアライゼーション
	var result []byte
	result = append(result, row.Key...)

	// 問題: 不完全なシリアライゼーション
	for _, cf := range row.Families {
		for col, versions := range cf.Columns {
			result = append(result, []byte(col)...)
			if len(versions) > 0 {
				result = append(result, versions[0].Value...)
			}
		}
	}

	return result
}

// Challenge: 以下の問題を修正してください
// 1. WAL-first書き込み
// 2. 適切なバージョン管理
// 3. 効率的なコンパクション
// 4. BloomFilter最適化
// 5. ホットスポット回避
// 6. Read/Write分離
// 7. スナップショット分離
// 8. 分散レプリケーション

func RunChallenge23() {
	fmt.Println("Challenge 23: Columnar Storage Engine")
	fmt.Println("Fix the columnar storage implementation")

	engine := NewColumnarStorageEngine()
	ctx := context.Background()
	_ = ctx

	// データ書き込み
	_ = []string{"info", "metrics"} // Column families for the example

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			rowKey := []byte(fmt.Sprintf("user:%05d", id))

			// ユーザー情報
			engine.Put(rowKey, "info", "name", []byte(fmt.Sprintf("User%d", id)))
			engine.Put(rowKey, "info", "email", []byte(fmt.Sprintf("user%d@example.com", id)))

			// メトリクス
			for j := 0; j < 5; j++ {
				engine.Put(rowKey, "metrics", "login_count",
					binary.BigEndian.AppendUint32(nil, uint32(j)))
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// データ読み込み
	value, err := engine.Get([]byte("user:00005"), "info", "name")
	if err == nil {
		fmt.Printf("Retrieved: %s\n", string(value))
	}

	// コンパクション
	engine.Compact(ctx)
}