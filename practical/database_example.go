package practical

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/go-redis/redis/v8"
	_ "github.com/marcboeker/go-duckdb"
)

// DatabaseExample データベース連携パターン
type DatabaseExample struct {
	postgres *sql.DB
	redis    *redis.Client
	duckdb   *sql.DB
}

// NewDatabaseExample データベース例を作成
func NewDatabaseExample() (*DatabaseExample, error) {
	// PostgreSQL接続
	pgConn, err := sql.Open("postgres",
		"host=localhost port=5432 user=gouser password=gopass dbname=goasync sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	// コネクションプール設定
	pgConn.SetMaxOpenConns(25)
	pgConn.SetMaxIdleConns(5)
	pgConn.SetConnMaxLifetime(5 * time.Minute)

	// Redis接続
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
		PoolSize: 10,
	})

	// DuckDB接続（インメモリ）
	duckConn, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to DuckDB: %w", err)
	}

	return &DatabaseExample{
		postgres: pgConn,
		redis:    redisClient,
		duckdb:   duckConn,
	}, nil
}

// Close リソースをクリーンアップ
func (d *DatabaseExample) Close() {
	if d.postgres != nil {
		d.postgres.Close()
	}
	if d.redis != nil {
		d.redis.Close()
	}
	if d.duckdb != nil {
		d.duckdb.Close()
	}
}

// Example1_ConnectionPool コネクションプールパターン
func (d *DatabaseExample) Example1_ConnectionPool() error {
	fmt.Println("\n=== Database Example 1: Connection Pool Pattern ===")

	// テーブル作成
	_, err := d.postgres.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id VARCHAR(50),
			amount DECIMAL(10,2),
			status VARCHAR(20),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// 並行処理でデータベースアクセス
	var wg sync.WaitGroup
	start := time.Now()

	// 10個のゴルーチンで同時にアクセス
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// トランザクション開始
			tx, err := d.postgres.Begin()
			if err != nil {
				fmt.Printf("❌ Worker %d: Failed to begin transaction\n", workerID)
				return
			}
			defer tx.Rollback()

			// データ挿入
			_, err = tx.Exec(
				"INSERT INTO orders (user_id, amount, status) VALUES ($1, $2, $3)",
				fmt.Sprintf("user_%d", workerID),
				float64(workerID*100+99),
				"pending",
			)
			if err != nil {
				fmt.Printf("❌ Worker %d: Insert failed\n", workerID)
				return
			}

			// コミット
			if err := tx.Commit(); err != nil {
				fmt.Printf("❌ Worker %d: Commit failed\n", workerID)
				return
			}

			fmt.Printf("✅ Worker %d: Order inserted\n", workerID)
		}(i)
	}

	wg.Wait()
	fmt.Printf("⏱️  Total time: %v\n", time.Since(start))

	// 統計情報表示
	stats := d.postgres.Stats()
	fmt.Printf("📊 DB Stats: Open=%d, InUse=%d, Idle=%d\n",
		stats.OpenConnections, stats.InUse, stats.Idle)

	return nil
}

// Example2_CachePattern キャッシュパターン（Cache-Aside）
func (d *DatabaseExample) Example2_CachePattern() error {
	fmt.Println("\n=== Database Example 2: Cache-Aside Pattern ===")

	ctx := context.Background()

	// ユーザー情報を取得する関数（キャッシュ付き）
	getUser := func(userID string) (map[string]interface{}, error) {
		cacheKey := fmt.Sprintf("user:%s", userID)

		// 1. キャッシュから取得を試みる
		cached, err := d.redis.Get(ctx, cacheKey).Result()
		if err == nil {
			fmt.Printf("🎯 Cache HIT: %s\n", userID)
			var user map[string]interface{}
			json.Unmarshal([]byte(cached), &user)
			return user, nil
		}

		fmt.Printf("❌ Cache MISS: %s\n", userID)

		// 2. データベースから取得
		user := map[string]interface{}{
			"id":         userID,
			"name":       fmt.Sprintf("User %s", userID),
			"email":      fmt.Sprintf("%s@example.com", userID),
			"created_at": time.Now(),
		}

		// 3. キャッシュに保存
		userData, _ := json.Marshal(user)
		d.redis.Set(ctx, cacheKey, userData, 30*time.Second)
		fmt.Printf("💾 Cached: %s (TTL: 30s)\n", userID)

		return user, nil
	}

	// 同じユーザーを複数回取得
	for i := 0; i < 3; i++ {
		user, _ := getUser("user123")
		fmt.Printf("   Retrieved: %s\n", user["name"])
		time.Sleep(500 * time.Millisecond)
	}

	// Write-Through Pattern
	updateUser := func(userID string, updates map[string]interface{}) error {
		cacheKey := fmt.Sprintf("user:%s", userID)

		// 1. データベースを更新
		// (実際のDB更新をシミュレート)
		fmt.Printf("📝 Updating DB: %s\n", userID)

		// 2. キャッシュも更新
		user := map[string]interface{}{
			"id":         userID,
			"updated_at": time.Now(),
		}
		for k, v := range updates {
			user[k] = v
		}

		userData, _ := json.Marshal(user)
		d.redis.Set(ctx, cacheKey, userData, 30*time.Second)
		fmt.Printf("🔄 Cache updated: %s\n", userID)

		return nil
	}

	// ユーザー更新
	updateUser("user123", map[string]interface{}{
		"name": "Updated User",
	})

	// 更新後の取得
	user, _ := getUser("user123")
	fmt.Printf("   After update: %s\n", user["name"])

	return nil
}

// Example3_RedisPatterns Redis活用パターン
func (d *DatabaseExample) Example3_RedisPatterns() error {
	fmt.Println("\n=== Database Example 3: Redis Patterns ===")

	ctx := context.Background()

	// 1. 分散ロック
	fmt.Println("\n🔒 Distributed Lock Pattern:")

	acquireLock := func(key string, ttl time.Duration) bool {
		success, err := d.redis.SetNX(ctx, key, "locked", ttl).Result()
		return err == nil && success
	}

	releaseLock := func(key string) {
		d.redis.Del(ctx, key)
	}

	// 複数のワーカーが同じリソースにアクセス
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			lockKey := "resource:lock"
			for attempts := 0; attempts < 5; attempts++ {
				if acquireLock(lockKey, 1*time.Second) {
					fmt.Printf("✅ Worker %d: Acquired lock\n", workerID)

					// クリティカルセクション
					time.Sleep(500 * time.Millisecond)

					releaseLock(lockKey)
					fmt.Printf("🔓 Worker %d: Released lock\n", workerID)
					break
				}
				fmt.Printf("⏳ Worker %d: Waiting for lock...\n", workerID)
				time.Sleep(200 * time.Millisecond)
			}
		}(i)
	}
	wg.Wait()

	// 2. Rate Limiting with Redis
	fmt.Println("\n⏱️  Rate Limiting Pattern:")

	checkRateLimit := func(userID string, limit int, window time.Duration) bool {
		key := fmt.Sprintf("rate:%s:%d", userID, time.Now().Unix()/int64(window.Seconds()))

		count, err := d.redis.Incr(ctx, key).Result()
		if err != nil {
			return false
		}

		if count == 1 {
			d.redis.Expire(ctx, key, window)
		}

		return count <= int64(limit)
	}

	// APIリクエストのレート制限をシミュレート
	for i := 0; i < 10; i++ {
		if checkRateLimit("user1", 5, 5*time.Second) {
			fmt.Printf("✅ Request %d: Allowed\n", i+1)
		} else {
			fmt.Printf("🚫 Request %d: Rate limited\n", i+1)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 3. Pub/Sub Pattern
	fmt.Println("\n📡 Pub/Sub Pattern:")

	// Subscriber
	go func() {
		pubsub := d.redis.Subscribe(ctx, "events")
		defer pubsub.Close()

		ch := pubsub.Channel()
		for msg := range ch {
			fmt.Printf("📨 Received: [%s] %s\n", msg.Channel, msg.Payload)
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// Publisher
	events := []string{"user.login", "order.created", "payment.processed"}
	for _, event := range events {
		d.redis.Publish(ctx, "events", event)
		fmt.Printf("📢 Published: %s\n", event)
		time.Sleep(200 * time.Millisecond)
	}

	return nil
}

// Example4_DuckDBAnalytics DuckDBで分析処理
func (d *DatabaseExample) Example4_DuckDBAnalytics() error {
	fmt.Println("\n=== Database Example 4: DuckDB Analytics ===")

	// サンプルデータを作成
	_, err := d.duckdb.Exec(`
		CREATE TABLE IF NOT EXISTS sales (
			date DATE,
			product VARCHAR,
			region VARCHAR,
			quantity INTEGER,
			revenue DECIMAL(10,2)
		)
	`)
	if err != nil {
		return err
	}

	// データを挿入
	products := []string{"Laptop", "Phone", "Tablet"}
	regions := []string{"North", "South", "East", "West"}

	tx, _ := d.duckdb.Begin()
	stmt, _ := tx.Prepare("INSERT INTO sales VALUES (?, ?, ?, ?, ?)")

	for i := 0; i < 100; i++ {
		date := time.Now().AddDate(0, 0, -i)
		product := products[i%len(products)]
		region := regions[i%len(regions)]
		quantity := 10 + i%20
		revenue := float64(quantity) * (99.99 + float64(i%3)*50)

		stmt.Exec(date, product, region, quantity, revenue)
	}
	stmt.Close()
	tx.Commit()

	// 並列分析クエリ
	queries := []struct {
		name string
		sql  string
	}{
		{
			name: "Total Revenue by Product",
			sql: `SELECT product, SUM(revenue) as total
			      FROM sales GROUP BY product ORDER BY total DESC`,
		},
		{
			name: "Average Quantity by Region",
			sql: `SELECT region, AVG(quantity) as avg_qty
			      FROM sales GROUP BY region`,
		},
		{
			name: "Top 5 Sales Days",
			sql: `SELECT date, SUM(revenue) as daily_revenue
			      FROM sales GROUP BY date
			      ORDER BY daily_revenue DESC LIMIT 5`,
		},
	}

	var wg sync.WaitGroup
	for _, query := range queries {
		wg.Add(1)
		go func(name, sql string) {
			defer wg.Done()

			start := time.Now()
			rows, err := d.duckdb.Query(sql)
			if err != nil {
				fmt.Printf("❌ %s: Failed\n", name)
				return
			}
			defer rows.Close()

			fmt.Printf("\n📊 %s (took %v):\n", name, time.Since(start))

			// 結果を表示
			columns, _ := rows.Columns()
			for rows.Next() {
				values := make([]interface{}, len(columns))
				valuePtrs := make([]interface{}, len(columns))
				for i := range values {
					valuePtrs[i] = &values[i]
				}

				rows.Scan(valuePtrs...)
				fmt.Printf("   ")
				for i, col := range columns {
					fmt.Printf("%s: %v  ", col, values[i])
				}
				fmt.Println()
			}
		}(query.name, query.sql)
	}

	wg.Wait()
	return nil
}