package practical

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gocql/gocql"
)

// CassandraNoSQL - Cassandraを使った分散NoSQL処理
type CassandraNoSQL struct {
	cluster *gocql.ClusterConfig
	session *gocql.Session
	mu      sync.RWMutex
}

// NewCassandraNoSQL - Cassandra接続の初期化
func NewCassandraNoSQL(hosts []string) (*CassandraNoSQL, error) {
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = "async_practice"
	cluster.Consistency = gocql.Quorum
	cluster.ProtoVersion = 4
	cluster.Timeout = 5 * time.Second
	cluster.ConnectTimeout = 5 * time.Second

	// 接続プールの設定
	cluster.NumConns = 3
	cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())

	session, err := cluster.CreateSession()
	if err != nil {
		// デモモード（実際のCassandraが利用できない場合）
		fmt.Println("⚠ Cassandraに接続できません。デモモードで実行します。")
		return &CassandraNoSQL{
			cluster: cluster,
			session: nil, // デモモードではnilセッション
		}, nil
	}

	c := &CassandraNoSQL{
		cluster: cluster,
		session: session,
	}

	// キースペースとテーブルの作成
	if err := c.initializeSchema(); err != nil {
		fmt.Printf("スキーマ初期化エラー: %v\n", err)
	}

	return c, nil
}

// initializeSchema - スキーマの初期化
func (c *CassandraNoSQL) initializeSchema() error {
	if c.session == nil {
		return nil // デモモード
	}

	queries := []string{
		// キースペース作成
		`CREATE KEYSPACE IF NOT EXISTS async_practice
		 WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 3}`,

		// タイムシリーズデータテーブル
		`CREATE TABLE IF NOT EXISTS async_practice.timeseries (
			partition_key text,
			timestamp timestamp,
			sensor_id text,
			value double,
			metadata map<text, text>,
			PRIMARY KEY (partition_key, timestamp, sensor_id)
		) WITH CLUSTERING ORDER BY (timestamp DESC)`,

		// ワイドカラムテーブル
		`CREATE TABLE IF NOT EXISTS async_practice.wide_rows (
			row_key text,
			column_name text,
			column_value blob,
			timestamp timestamp,
			PRIMARY KEY (row_key, column_name)
		)`,

		// カウンターテーブル
		`CREATE TABLE IF NOT EXISTS async_practice.counters (
			counter_id text PRIMARY KEY,
			count counter
		)`,

		// マテリアライズドビュー
		`CREATE MATERIALIZED VIEW IF NOT EXISTS async_practice.timeseries_by_sensor AS
			SELECT * FROM async_practice.timeseries
			WHERE sensor_id IS NOT NULL AND partition_key IS NOT NULL AND timestamp IS NOT NULL
			PRIMARY KEY (sensor_id, timestamp, partition_key)
			WITH CLUSTERING ORDER BY (timestamp DESC)`,
	}

	for _, query := range queries {
		if err := c.session.Query(query).Exec(); err != nil {
			// エラーを無視（既存の場合など）
			fmt.Printf("スキーマクエリ実行エラー: %v\n", err)
		}
	}

	return nil
}

// TimeSeriesIngestion - タイムシリーズデータの並列取り込み
func (c *CassandraNoSQL) TimeSeriesIngestion(ctx context.Context) {
	fmt.Println("\n🗄 Cassandra タイムシリーズ取り込みデモ")
	fmt.Println("=" + repeatString("=", 50))

	if c.session == nil {
		c.runDemoMode(ctx)
		return
	}

	// メトリクス
	var (
		totalInserted  int64
		totalFailed    int64
		totalBatches   int64
	)

	// データジェネレーター
	dataStream := make(chan SensorData, 1000)

	// センサーデータ生成（並列）
	var producerWg sync.WaitGroup
	numSensors := 10
	for i := 0; i < numSensors; i++ {
		producerWg.Add(1)
		go func(sensorID int) {
			defer producerWg.Done()
			c.generateSensorData(ctx, sensorID, dataStream)
		}(i)
	}

	// バッチ書き込みワーカー
	var writerWg sync.WaitGroup
	numWriters := 5
	batchSize := 50

	for i := 0; i < numWriters; i++ {
		writerWg.Add(1)
		go func(writerID int) {
			defer writerWg.Done()
			c.batchWriter(ctx, writerID, dataStream, batchSize,
				&totalInserted, &totalFailed, &totalBatches)
		}(i)
	}

	// 統計表示
	go c.displayStats(ctx, &totalInserted, &totalFailed, &totalBatches)

	// 並列読み取りデモ
	go c.parallelReads(ctx)

	// 実行
	select {
	case <-ctx.Done():
	case <-time.After(15 * time.Second):
	}

	// クリーンアップ
	close(dataStream)
	producerWg.Wait()
	writerWg.Wait()

	fmt.Printf("\n📊 最終統計: 挿入=%d, 失敗=%d, バッチ=%d\n",
		atomic.LoadInt64(&totalInserted),
		atomic.LoadInt64(&totalFailed),
		atomic.LoadInt64(&totalBatches))
}

// SensorData - センサーデータ
type SensorData struct {
	SensorID    string
	Timestamp   time.Time
	Value       float64
	Metadata    map[string]string
}

// generateSensorData - センサーデータ生成
func (c *CassandraNoSQL) generateSensorData(ctx context.Context, sensorID int, out chan<- SensorData) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	sensorName := fmt.Sprintf("sensor_%d", sensorID)
	location := []string{"tokyo", "osaka", "nagoya"}[sensorID%3]

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			data := SensorData{
				SensorID:  sensorName,
				Timestamp: time.Now(),
				Value:     float64(sensorID*100) + float64(time.Now().Unix()%100),
				Metadata: map[string]string{
					"location": location,
					"type":     "temperature",
					"unit":     "celsius",
				},
			}

			select {
			case out <- data:
			default:
				// バッファフルの場合はスキップ
			}
		}
	}
}

// batchWriter - バッチ書き込みワーカー
func (c *CassandraNoSQL) batchWriter(ctx context.Context, writerID int,
	dataStream <-chan SensorData, batchSize int,
	totalInserted, totalFailed, totalBatches *int64) {

	batch := make([]SensorData, 0, batchSize)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		// バッチクエリの準備
		batchQuery := c.session.NewBatch(gocql.LoggedBatch)
		batchQuery.SetConsistency(gocql.LocalQuorum)

		for _, data := range batch {
			// パーティションキーの生成（日付ベース）
			partitionKey := fmt.Sprintf("%s_%s",
				data.SensorID,
				data.Timestamp.Format("2006-01-02"))

			batchQuery.Query(`
				INSERT INTO async_practice.timeseries
				(partition_key, timestamp, sensor_id, value, metadata)
				VALUES (?, ?, ?, ?, ?)`,
				partitionKey,
				data.Timestamp,
				data.SensorID,
				data.Value,
				data.Metadata,
			)
		}

		// バッチ実行（リトライ付き）
		var err error
		for retry := 0; retry < 3; retry++ {
			err = c.session.ExecuteBatch(batchQuery)
			if err == nil {
				break
			}
			time.Sleep(time.Duration(retry*100) * time.Millisecond)
		}

		if err != nil {
			atomic.AddInt64(totalFailed, int64(len(batch)))
			fmt.Printf("  Writer %d: バッチ書き込みエラー: %v\n", writerID, err)
		} else {
			atomic.AddInt64(totalInserted, int64(len(batch)))
			atomic.AddInt64(totalBatches, 1)
		}

		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case data, ok := <-dataStream:
			if !ok {
				flush()
				return
			}
			batch = append(batch, data)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// parallelReads - 並列読み取りデモ
func (c *CassandraNoSQL) parallelReads(ctx context.Context) {
	if c.session == nil {
		return // デモモード
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 並列クエリ実行
			var wg sync.WaitGroup
			queries := []struct {
				name  string
				query string
			}{
				{
					name: "最新データ",
					query: `SELECT sensor_id, timestamp, value
							FROM async_practice.timeseries
							LIMIT 10`,
				},
				{
					name: "センサー別集計",
					query: `SELECT sensor_id, COUNT(*) as count
							FROM async_practice.timeseries
							GROUP BY sensor_id
							ALLOW FILTERING`,
				},
			}

			fmt.Println("\n📖 並列読み取り結果:")
			for _, q := range queries {
				wg.Add(1)
				go func(name, query string) {
					defer wg.Done()

					iter := c.session.Query(query).Iter()

					var results []map[string]interface{}
					m := make(map[string]interface{})
					for iter.MapScan(m) {
						results = append(results, m)
						m = make(map[string]interface{})
					}

					if err := iter.Close(); err != nil {
						fmt.Printf("  %s: エラー %v\n", name, err)
					} else {
						fmt.Printf("  %s: %d件取得\n", name, len(results))
					}
				}(q.name, q.query)
			}
			wg.Wait()
		}
	}
}

// displayStats - 統計表示
func (c *CassandraNoSQL) displayStats(ctx context.Context,
	totalInserted, totalFailed, totalBatches *int64) {

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastInserted int64
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			inserted := atomic.LoadInt64(totalInserted)
			failed := atomic.LoadInt64(totalFailed)
			batches := atomic.LoadInt64(totalBatches)

			// スループット計算
			throughput := float64(inserted-lastInserted) / 2.0 // per second
			elapsed := time.Since(startTime).Seconds()
			avgThroughput := float64(inserted) / elapsed

			fmt.Printf("\n📊 統計: 挿入=%d, 失敗=%d, バッチ=%d, "+
				"スループット=%.1f/秒, 平均=%.1f/秒\n",
				inserted, failed, batches, throughput, avgThroughput)

			lastInserted = inserted
		}
	}
}

// WideColumnOperations - ワイドカラム操作デモ
func (c *CassandraNoSQL) WideColumnOperations(ctx context.Context) {
	fmt.Println("\n📋 Cassandra ワイドカラムデモ")
	fmt.Println("=" + repeatString("=", 50))

	if c.session == nil {
		fmt.Println("デモモード: ワイドカラム操作のシミュレーション")
		c.simulateWideColumns()
		return
	}

	// ワイドローの作成
	rowKey := fmt.Sprintf("user_%d", time.Now().Unix())

	// 並列でカラムを追加
	var wg sync.WaitGroup
	numColumns := 1000

	for i := 0; i < numColumns; i++ {
		wg.Add(1)
		go func(colIndex int) {
			defer wg.Done()

			columnName := fmt.Sprintf("col_%d", colIndex)
			columnValue := []byte(fmt.Sprintf("value_%d_%d", colIndex, time.Now().Unix()))

			err := c.session.Query(`
				INSERT INTO async_practice.wide_rows
				(row_key, column_name, column_value, timestamp)
				VALUES (?, ?, ?, ?)`,
				rowKey, columnName, columnValue, time.Now(),
			).Exec()

			if err != nil && colIndex%100 == 0 {
				fmt.Printf("  カラム挿入エラー %d: %v\n", colIndex, err)
			}
		}(i)
	}

	wg.Wait()
	fmt.Printf("  ✓ %d個のカラムを挿入しました (row_key: %s)\n", numColumns, rowKey)

	// スライスクエリ
	fmt.Println("\n  カラムスライス読み取り:")
	iter := c.session.Query(`
		SELECT column_name, column_value
		FROM async_practice.wide_rows
		WHERE row_key = ?
		LIMIT 10`,
		rowKey,
	).Iter()

	var columnName string
	var columnValue []byte
	count := 0
	for iter.Scan(&columnName, &columnValue) {
		fmt.Printf("    %s: %s\n", columnName, string(columnValue))
		count++
	}

	if err := iter.Close(); err != nil {
		fmt.Printf("  読み取りエラー: %v\n", err)
	} else {
		fmt.Printf("  ✓ %d個のカラムを読み取りました\n", count)
	}
}

// CounterOperations - カウンター操作デモ
func (c *CassandraNoSQL) CounterOperations(ctx context.Context) {
	fmt.Println("\n🔢 Cassandra カウンターデモ")
	fmt.Println("=" + repeatString("=", 50))

	if c.session == nil {
		fmt.Println("デモモード: カウンター操作のシミュレーション")
		c.simulateCounters()
		return
	}

	counterID := fmt.Sprintf("counter_%d", time.Now().Unix())

	// カウンターの並列更新
	var wg sync.WaitGroup
	numWorkers := 10
	incrementsPerWorker := 100

	fmt.Printf("  %d個のワーカーで並列カウンター更新...\n", numWorkers)

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for i := 0; i < incrementsPerWorker; i++ {
				err := c.session.Query(`
					UPDATE async_practice.counters
					SET count = count + ?
					WHERE counter_id = ?`,
					1, counterID,
				).Exec()

				if err != nil && i == 0 {
					fmt.Printf("    Worker %d: カウンター更新エラー: %v\n", workerID, err)
				}
			}
		}(w)
	}

	wg.Wait()

	// カウンター値の読み取り
	var count int64
	err := c.session.Query(`
		SELECT count FROM async_practice.counters
		WHERE counter_id = ?`,
		counterID,
	).Scan(&count)

	if err != nil {
		fmt.Printf("  カウンター読み取りエラー: %v\n", err)
	} else {
		expected := numWorkers * incrementsPerWorker
		fmt.Printf("  ✓ カウンター値: %d (期待値: %d)\n", count, expected)
	}
}

// runDemoMode - デモモード実行
func (c *CassandraNoSQL) runDemoMode(ctx context.Context) {
	fmt.Println("\n🎭 デモモード: Cassandraシミュレーション")

	// 仮想的なデータストア
	type VirtualCassandra struct {
		mu    sync.RWMutex
		data  map[string]map[string]interface{}
		count int64
	}

	vc := &VirtualCassandra{
		data: make(map[string]map[string]interface{}),
	}

	// 並列書き込みシミュレーション
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				vc.mu.Lock()
				key := fmt.Sprintf("key_%d_%d", id, j)
				vc.data[key] = map[string]interface{}{
					"value":     j,
					"timestamp": time.Now(),
				}
				atomic.AddInt64(&vc.count, 1)
				vc.mu.Unlock()

				if j%20 == 0 {
					fmt.Printf("  Worker %d: %d件処理\n", id, j)
				}
			}
		}(i)
	}

	wg.Wait()
	fmt.Printf("\n  ✓ デモ完了: %d件のデータを処理\n", atomic.LoadInt64(&vc.count))
}

// simulateWideColumns - ワイドカラムのシミュレーション
func (c *CassandraNoSQL) simulateWideColumns() {
	row := make(map[string]interface{})
	var mu sync.Mutex

	// 並列カラム追加
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(col int) {
			defer wg.Done()
			mu.Lock()
			row[fmt.Sprintf("col_%d", col)] = fmt.Sprintf("value_%d", col)
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	fmt.Printf("  ✓ シミュレーション: %d個のカラムを持つワイドロー\n", len(row))
}

// simulateCounters - カウンターのシミュレーション
func (c *CassandraNoSQL) simulateCounters() {
	var counter int64

	// 並列インクリメント
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				atomic.AddInt64(&counter, 1)
			}
		}()
	}
	wg.Wait()

	fmt.Printf("  ✓ シミュレーション: カウンター値 = %d\n", counter)
}

// Close - リソースクリーンアップ
func (c *CassandraNoSQL) Close() {
	if c.session != nil {
		c.session.Close()
	}
}