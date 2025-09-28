package practical

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "github.com/marcboeker/go-duckdb" // DuckDB driver
)

// DuckDBAnalytics - DuckDBを使った並列分析処理
type DuckDBAnalytics struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewDuckDBAnalytics - DuckDB分析エンジンの初期化
func NewDuckDBAnalytics() (*DuckDBAnalytics, error) {
	// In-memoryデータベースとして使用
	db, err := sql.Open("duckdb", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open duckdb: %w", err)
	}

	analytics := &DuckDBAnalytics{
		db: db,
	}

	// テーブル作成
	if err := analytics.initializeTables(); err != nil {
		return nil, err
	}

	return analytics, nil
}

// initializeTables - 分析用テーブルの初期化
func (d *DuckDBAnalytics) initializeTables() error {
	queries := []string{
		// イベントストアテーブル
		`CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY,
			timestamp TIMESTAMP,
			user_id VARCHAR,
			event_type VARCHAR,
			properties JSON,
			value DOUBLE
		)`,
		// 集計結果テーブル
		`CREATE TABLE IF NOT EXISTS metrics (
			metric_name VARCHAR,
			timestamp TIMESTAMP,
			dimensions JSON,
			value DOUBLE,
			count INTEGER
		)`,
		// タイムシリーズデータ
		`CREATE TABLE IF NOT EXISTS timeseries (
			metric VARCHAR,
			timestamp TIMESTAMP,
			tags JSON,
			value DOUBLE
		)`,
	}

	for _, query := range queries {
		if _, err := d.db.Exec(query); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	return nil
}

// StreamingIngestion - ストリーミングデータの並列取り込み
func (d *DuckDBAnalytics) StreamingIngestion(ctx context.Context) {
	fmt.Println("\n🦆 DuckDB ストリーミング分析デモ")
	fmt.Println("=" + repeatString("=", 50))

	// データジェネレーター
	eventStream := make(chan Event, 1000)

	// 並列データ生成
	var producerWg sync.WaitGroup
	for i := 0; i < 3; i++ {
		producerWg.Add(1)
		go func(producerID int) {
			defer producerWg.Done()
			d.generateEvents(ctx, producerID, eventStream)
		}(i)
	}

	// バッチ挿入ワーカー
	var consumerWg sync.WaitGroup
	batchSize := 100
	workers := 4

	for i := 0; i < workers; i++ {
		consumerWg.Add(1)
		go func(workerID int) {
			defer consumerWg.Done()
			d.batchInsertWorker(ctx, workerID, eventStream, batchSize)
		}(i)
	}

	// リアルタイム集計
	go d.realtimeAggregation(ctx)

	// ウィンドウ集計
	go d.windowAggregation(ctx, 5*time.Second)

	// メトリクス表示
	go d.displayMetrics(ctx)

	// 実行時間
	select {
	case <-ctx.Done():
	case <-time.After(10 * time.Second):
	}

	// クリーンアップ
	close(eventStream)
	producerWg.Wait()
	consumerWg.Wait()

	// 最終結果
	d.showFinalResults()
}

// Event - イベントデータ構造
type Event struct {
	ID        int
	Timestamp time.Time
	UserID    string
	EventType string
	Value     float64
	Properties map[string]interface{}
}

// generateEvents - イベント生成
func (d *DuckDBAnalytics) generateEvents(ctx context.Context, producerID int, out chan<- Event) {
	eventTypes := []string{"click", "view", "purchase", "signup"}
	id := producerID * 10000

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			event := Event{
				ID:        id,
				Timestamp: time.Now(),
				UserID:    fmt.Sprintf("user_%d", id%100),
				EventType: eventTypes[id%len(eventTypes)],
				Value:     float64(id%1000) + 0.99,
				Properties: map[string]interface{}{
					"source":   fmt.Sprintf("producer_%d", producerID),
					"session":  fmt.Sprintf("session_%d", id%10),
					"platform": []string{"web", "mobile", "api"}[id%3],
				},
			}
			select {
			case out <- event:
				id++
			default:
				// バッファフルの場合はスキップ
			}
		}
	}
}

// batchInsertWorker - バッチ挿入ワーカー
func (d *DuckDBAnalytics) batchInsertWorker(ctx context.Context, workerID int, events <-chan Event, batchSize int) {
	batch := make([]Event, 0, batchSize)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		tx, err := d.db.Begin()
		if err != nil {
			fmt.Printf("  Worker %d: トランザクション開始エラー: %v\n", workerID, err)
			return
		}
		defer tx.Rollback()

		stmt, err := tx.Prepare(`
			INSERT INTO events (id, timestamp, user_id, event_type, properties, value)
			VALUES (?, ?, ?, ?, ?, ?)
		`)
		if err != nil {
			fmt.Printf("  Worker %d: プリペアエラー: %v\n", workerID, err)
			return
		}
		defer stmt.Close()

		for _, event := range batch {
			properties := fmt.Sprintf(`{"source": "%s", "platform": "%s"}`,
				event.Properties["source"], event.Properties["platform"])

			_, err := stmt.Exec(
				event.ID,
				event.Timestamp,
				event.UserID,
				event.EventType,
				properties,
				event.Value,
			)
			if err != nil {
				fmt.Printf("  Worker %d: 挿入エラー: %v\n", workerID, err)
			}
		}

		if err := tx.Commit(); err != nil {
			fmt.Printf("  Worker %d: コミットエラー: %v\n", workerID, err)
		} else {
			fmt.Printf("  Worker %d: %d件挿入完了\n", workerID, len(batch))
		}

		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return
		case event, ok := <-events:
			if !ok {
				flush()
				return
			}
			batch = append(batch, event)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// realtimeAggregation - リアルタイム集計
func (d *DuckDBAnalytics) realtimeAggregation(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// イベントタイプ別の集計
			query := `
				SELECT
					event_type,
					COUNT(*) as count,
					SUM(value) as total_value,
					AVG(value) as avg_value,
					MAX(value) as max_value
				FROM events
				WHERE timestamp > NOW() - INTERVAL 10 SECOND
				GROUP BY event_type
			`

			rows, err := d.db.Query(query)
			if err != nil {
				continue
			}

			fmt.Println("\n📊 リアルタイム集計（過去10秒）:")
			for rows.Next() {
				var eventType string
				var count int
				var totalValue, avgValue, maxValue float64

				rows.Scan(&eventType, &count, &totalValue, &avgValue, &maxValue)
				fmt.Printf("  %s: 件数=%d, 合計=%.2f, 平均=%.2f, 最大=%.2f\n",
					eventType, count, totalValue, avgValue, maxValue)
			}
			rows.Close()
		}
	}
}

// windowAggregation - ウィンドウ集計
func (d *DuckDBAnalytics) windowAggregation(ctx context.Context, window time.Duration) {
	ticker := time.NewTicker(window)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 移動平均の計算
			query := `
				SELECT
					event_type,
					DATE_TRUNC('second', timestamp) as time_bucket,
					COUNT(*) OVER (
						PARTITION BY event_type
						ORDER BY DATE_TRUNC('second', timestamp)
						RANGE BETWEEN INTERVAL 5 SECOND PRECEDING AND CURRENT ROW
					) as moving_count,
					AVG(value) OVER (
						PARTITION BY event_type
						ORDER BY DATE_TRUNC('second', timestamp)
						RANGE BETWEEN INTERVAL 5 SECOND PRECEDING AND CURRENT ROW
					) as moving_avg
				FROM events
				WHERE timestamp > NOW() - INTERVAL 30 SECOND
				ORDER BY time_bucket DESC
				LIMIT 10
			`

			rows, err := d.db.Query(query)
			if err != nil {
				continue
			}

			fmt.Println("\n📈 ウィンドウ集計（5秒移動平均）:")
			for rows.Next() {
				var eventType string
				var timeBucket time.Time
				var movingCount int
				var movingAvg float64

				rows.Scan(&eventType, &timeBucket, &movingCount, &movingAvg)
				fmt.Printf("  [%s] %s: 移動件数=%d, 移動平均=%.2f\n",
					timeBucket.Format("15:04:05"), eventType, movingCount, movingAvg)
			}
			rows.Close()
		}
	}
}

// displayMetrics - メトリクス表示
func (d *DuckDBAnalytics) displayMetrics(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// テーブル統計
			var totalEvents int
			d.db.QueryRow("SELECT COUNT(*) FROM events").Scan(&totalEvents)

			// ユニークユーザー数
			var uniqueUsers int
			d.db.QueryRow("SELECT COUNT(DISTINCT user_id) FROM events").Scan(&uniqueUsers)

			// データレート
			var eventsPerSecond float64
			d.db.QueryRow(`
				SELECT COUNT(*) * 1.0 /
					EXTRACT(EPOCH FROM (MAX(timestamp) - MIN(timestamp)))
				FROM events
				WHERE timestamp > NOW() - INTERVAL 10 SECOND
			`).Scan(&eventsPerSecond)

			fmt.Printf("\n📊 システムメトリクス: 総イベント=%d, ユニークユーザー=%d, レート=%.1f/秒\n",
				totalEvents, uniqueUsers, eventsPerSecond)
		}
	}
}

// showFinalResults - 最終結果表示
func (d *DuckDBAnalytics) showFinalResults() {
	fmt.Println("\n🏁 最終分析結果:")
	fmt.Println("=" + repeatString("=", 50))

	// Top N分析
	query := `
		WITH user_stats AS (
			SELECT
				user_id,
				COUNT(*) as event_count,
				SUM(value) as total_value,
				COUNT(DISTINCT event_type) as unique_events
			FROM events
			GROUP BY user_id
		)
		SELECT * FROM user_stats
		ORDER BY total_value DESC
		LIMIT 5
	`

	rows, err := d.db.Query(query)
	if err != nil {
		fmt.Printf("クエリエラー: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("\n🏆 Top 5 ユーザー:")
	rank := 1
	for rows.Next() {
		var userID string
		var eventCount, uniqueEvents int
		var totalValue float64

		rows.Scan(&userID, &eventCount, &totalValue, &uniqueEvents)
		fmt.Printf("  %d. %s: イベント数=%d, 合計値=%.2f, ユニークイベント=%d\n",
			rank, userID, eventCount, totalValue, uniqueEvents)
		rank++
	}

	// パーセンタイル分析
	fmt.Println("\n📊 値の分布（パーセンタイル）:")
	percentiles := []float64{0.25, 0.5, 0.75, 0.95, 0.99}
	for _, p := range percentiles {
		var value float64
		query := fmt.Sprintf(`
			SELECT PERCENTILE_CONT(%.2f) WITHIN GROUP (ORDER BY value)
			FROM events
		`, p)
		d.db.QueryRow(query).Scan(&value)
		fmt.Printf("  P%d: %.2f\n", int(p*100), value)
	}
}

// ParallelOLAPQueries - 並列OLAP分析
func (d *DuckDBAnalytics) ParallelOLAPQueries(ctx context.Context) {
	fmt.Println("\n🔄 並列OLAP分析デモ")
	fmt.Println("=" + repeatString("=", 50))

	// サンプルデータ生成
	d.generateSampleData()

	queries := []struct {
		name  string
		query string
	}{
		{
			name: "日次集計",
			query: `
				SELECT
					DATE_TRUNC('day', timestamp) as day,
					COUNT(*) as daily_events,
					SUM(value) as daily_value
				FROM events
				GROUP BY day
				ORDER BY day
			`,
		},
		{
			name: "イベントタイプ別分析",
			query: `
				SELECT
					event_type,
					COUNT(*) as count,
					AVG(value) as avg_value,
					STDDEV(value) as stddev_value
				FROM events
				GROUP BY event_type
			`,
		},
		{
			name: "ユーザーセグメント分析",
			query: `
				WITH user_segments AS (
					SELECT
						user_id,
						CASE
							WHEN SUM(value) > 1000 THEN 'High'
							WHEN SUM(value) > 500 THEN 'Medium'
							ELSE 'Low'
						END as segment
					FROM events
					GROUP BY user_id
				)
				SELECT segment, COUNT(*) as user_count
				FROM user_segments
				GROUP BY segment
			`,
		},
		{
			name: "時間帯分析",
			query: `
				SELECT
					EXTRACT(HOUR FROM timestamp) as hour,
					COUNT(*) as event_count,
					AVG(value) as avg_value
				FROM events
				GROUP BY hour
				ORDER BY hour
			`,
		},
	}

	// 並列クエリ実行
	var wg sync.WaitGroup
	results := make(chan QueryResult, len(queries))

	for _, q := range queries {
		wg.Add(1)
		go func(name, query string) {
			defer wg.Done()

			start := time.Now()
			rows, err := d.db.Query(query)
			elapsed := time.Since(start)

			if err != nil {
				results <- QueryResult{
					Name:    name,
					Error:   err,
					Elapsed: elapsed,
				}
				return
			}
			defer rows.Close()

			// 結果を収集
			var resultData [][]interface{}
			cols, _ := rows.Columns()

			for rows.Next() {
				values := make([]interface{}, len(cols))
				valuePtrs := make([]interface{}, len(cols))
				for i := range values {
					valuePtrs[i] = &values[i]
				}
				rows.Scan(valuePtrs...)
				resultData = append(resultData, values)
			}

			results <- QueryResult{
				Name:    name,
				Data:    resultData,
				Elapsed: elapsed,
			}
		}(q.name, q.query)
	}

	// 結果収集
	go func() {
		wg.Wait()
		close(results)
	}()

	// 結果表示
	for result := range results {
		if result.Error != nil {
			fmt.Printf("\n❌ %s: エラー %v\n", result.Name, result.Error)
		} else {
			fmt.Printf("\n✅ %s (実行時間: %v):\n", result.Name, result.Elapsed)
			for i, row := range result.Data {
				if i < 5 { // 最初の5行のみ表示
					fmt.Printf("  %v\n", row)
				}
			}
			if len(result.Data) > 5 {
				fmt.Printf("  ... 他 %d 行\n", len(result.Data)-5)
			}
		}
	}
}

// QueryResult - クエリ結果
type QueryResult struct {
	Name    string
	Data    [][]interface{}
	Error   error
	Elapsed time.Duration
}

// generateSampleData - サンプルデータ生成
func (d *DuckDBAnalytics) generateSampleData() {
	// バルクインサート用のデータ生成
	tx, _ := d.db.Begin()
	stmt, _ := tx.Prepare(`
		INSERT INTO events (id, timestamp, user_id, event_type, properties, value)
		VALUES (?, ?, ?, ?, ?, ?)
	`)

	eventTypes := []string{"click", "view", "purchase", "signup", "logout"}
	baseTime := time.Now().Add(-24 * time.Hour)

	for i := 0; i < 10000; i++ {
		timestamp := baseTime.Add(time.Duration(i) * time.Minute)
		userID := fmt.Sprintf("user_%d", i%100)
		eventType := eventTypes[i%len(eventTypes)]
		value := float64(i%1000) + float64(i%100)/100

		stmt.Exec(i, timestamp, userID, eventType, "{}", value)
	}

	tx.Commit()
	fmt.Println("  ✓ 10,000件のサンプルデータを生成しました")
}

// Close - リソースクリーンアップ
func (d *DuckDBAnalytics) Close() error {
	return d.db.Close()
}