package practical

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	_ "github.com/lib/pq"
)

// CockroachDBDistributed - CockroachDB分散データベースパターン
type CockroachDBDistributed struct {
	db          *sql.DB
	nodes       []string
	currentNode int32
	mu          sync.RWMutex
	stats       *DBStats
}

type DBStats struct {
	Reads           int64
	Writes          int64
	Conflicts       int64
	Retries         int64
	FailedRetries   int64
	TotalLatency    int64
	ConnectionPools map[string]*sql.DB
}

// NewCockroachDBDistributed - 新しいインスタンスを作成
func NewCockroachDBDistributed(nodes []string) (*CockroachDBDistributed, error) {
	if len(nodes) == 0 {
		// デモモード（実際の接続なし）
		fmt.Println("🎮 デモモード: CockroachDBなしで動作シミュレーション")
		return &CockroachDBDistributed{
			nodes: []string{"node1:26257", "node2:26257", "node3:26257"},
			stats: &DBStats{
				ConnectionPools: make(map[string]*sql.DB),
			},
		}, nil
	}

	// 最初のノードに接続
	db, err := sql.Open("postgres", nodes[0])
	if err != nil {
		return nil, err
	}

	c := &CockroachDBDistributed{
		db:    db,
		nodes: nodes,
		stats: &DBStats{
			ConnectionPools: make(map[string]*sql.DB),
		},
	}

	// 各ノードへの接続プール作成
	for _, node := range nodes {
		conn, err := sql.Open("postgres", node)
		if err != nil {
			continue
		}
		conn.SetMaxOpenConns(10)
		conn.SetMaxIdleConns(5)
		c.stats.ConnectionPools[node] = conn
	}

	return c, nil
}

// DistributedTransactionDemo - 分散トランザクションのデモ
func (c *CockroachDBDistributed) DistributedTransactionDemo(ctx context.Context) {
	fmt.Println("\n=== CockroachDB 分散トランザクションデモ ===")

	if c.db == nil {
		c.simulateDistributedTransaction(ctx)
		return
	}

	// テーブル作成
	c.createTables()

	// 1. 分散トランザクション
	fmt.Println("\n1. 分散トランザクション実行")
	c.executeDistributedTransaction(ctx)

	// 2. 楽観的並行制御
	fmt.Println("\n2. 楽観的並行制御")
	c.demonstrateOptimisticConcurrency(ctx)

	// 3. リトライロジック
	fmt.Println("\n3. 自動リトライ機構")
	c.demonstrateRetryLogic(ctx)

	// 4. ゾーン設定
	fmt.Println("\n4. ゾーン設定とレプリケーション")
	c.configureZones(ctx)

	// 統計表示
	c.printStatistics()
}

// simulateDistributedTransaction - デモモードでの分散トランザクション
func (c *CockroachDBDistributed) simulateDistributedTransaction(ctx context.Context) {
	fmt.Println("📊 分散トランザクションのシミュレーション")

	type Account struct {
		ID      int
		Balance float64
		Node    string
	}

	accounts := map[int]*Account{
		1: {ID: 1, Balance: 1000, Node: "node1"},
		2: {ID: 2, Balance: 2000, Node: "node2"},
		3: {ID: 3, Balance: 3000, Node: "node3"},
	}

	// 並行トランザクション実行
	var wg sync.WaitGroup
	conflicts := int32(0)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(txID int) {
			defer wg.Done()

			// ランダムな送金
			from := rand.Intn(3) + 1
			to := (from % 3) + 1
			amount := float64(rand.Intn(100) + 50)

			// トランザクション開始
			fmt.Printf("TX%d: アカウント%d -> %d (%.2f)\n", txID, from, to, amount)

			// シミュレートされた競合検出
			if rand.Float32() < 0.3 {
				atomic.AddInt32(&conflicts, 1)
				atomic.AddInt64(&c.stats.Conflicts, 1)

				// リトライ
				time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
				fmt.Printf("TX%d: 競合検出、リトライ実行\n", txID)
				atomic.AddInt64(&c.stats.Retries, 1)
			}

			// トランザクション実行
			c.mu.Lock()
			if accounts[from].Balance >= amount {
				accounts[from].Balance -= amount
				accounts[to].Balance += amount
				fmt.Printf("TX%d: ✅ 送金成功\n", txID)
				atomic.AddInt64(&c.stats.Writes, 2)
			} else {
				fmt.Printf("TX%d: ❌ 残高不足\n", txID)
			}
			c.mu.Unlock()

		}(i)
	}

	wg.Wait()

	// 最終残高表示
	fmt.Println("\n最終残高:")
	for id, acc := range accounts {
		fmt.Printf("  アカウント%d (%s): %.2f\n", id, acc.Node, acc.Balance)
	}

	fmt.Printf("\n競合検出数: %d\n", conflicts)
}

// executeDistributedTransaction - 実際の分散トランザクション
func (c *CockroachDBDistributed) executeDistributedTransaction(ctx context.Context) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 複数のノードにまたがる操作
	queries := []string{
		"UPDATE accounts SET balance = balance - 100 WHERE id = 1",
		"UPDATE accounts SET balance = balance + 100 WHERE id = 2",
		"INSERT INTO transaction_log (from_id, to_id, amount) VALUES (1, 2, 100)",
	}

	for _, query := range queries {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			atomic.AddInt64(&c.stats.Conflicts, 1)
			return err
		}
		atomic.AddInt64(&c.stats.Writes, 1)
	}

	if err := tx.Commit(); err != nil {
		atomic.AddInt64(&c.stats.Conflicts, 1)
		return err
	}

	fmt.Println("✅ 分散トランザクション成功")
	return nil
}

// demonstrateOptimisticConcurrency - 楽観的並行制御のデモ
func (c *CockroachDBDistributed) demonstrateOptimisticConcurrency(ctx context.Context) {
	if c.db == nil {
		c.simulateOptimisticConcurrency(ctx)
		return
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 楽観的ロック実装
			for retry := 0; retry < 3; retry++ {
				// 読み取り
				var version int
				err := c.db.QueryRowContext(ctx,
					"SELECT version FROM inventory WHERE product_id = $1", 1).Scan(&version)
				if err != nil {
					continue
				}

				// 更新（バージョンチェック付き）
				result, err := c.db.ExecContext(ctx,
					"UPDATE inventory SET quantity = quantity - 1, version = version + 1 "+
						"WHERE product_id = $1 AND version = $2", 1, version)

				if err == nil {
					if rowsAffected, _ := result.RowsAffected(); rowsAffected > 0 {
						fmt.Printf("Worker %d: 在庫更新成功\n", id)
						atomic.AddInt64(&c.stats.Writes, 1)
						break
					}
				}

				// 競合検出
				fmt.Printf("Worker %d: 競合検出、リトライ %d/3\n", id, retry+1)
				atomic.AddInt64(&c.stats.Conflicts, 1)
				atomic.AddInt64(&c.stats.Retries, 1)
				time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
}

// simulateOptimisticConcurrency - 楽観的並行制御のシミュレーション
func (c *CockroachDBDistributed) simulateOptimisticConcurrency(ctx context.Context) {
	type Product struct {
		ID       int
		Quantity int
		Version  int
	}

	product := &Product{ID: 1, Quantity: 100, Version: 1}
	var mu sync.RWMutex

	var wg sync.WaitGroup
	successCount := int32(0)
	conflictCount := int32(0)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// 読み取り
			mu.RLock()
			readVersion := product.Version
			readQuantity := product.Quantity
			mu.RUnlock()

			// 処理時間のシミュレーション
			time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)

			// 更新試行
			mu.Lock()
			if product.Version == readVersion && readQuantity > 0 {
				// バージョン一致 - 更新成功
				product.Quantity--
				product.Version++
				atomic.AddInt32(&successCount, 1)
				fmt.Printf("Worker %d: ✅ 在庫更新成功 (残: %d)\n", workerID, product.Quantity)
			} else {
				// バージョン不一致 - 競合
				atomic.AddInt32(&conflictCount, 1)
				fmt.Printf("Worker %d: ⚠️ 競合検出\n", workerID)
			}
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	fmt.Printf("\n結果: 成功=%d, 競合=%d, 最終在庫=%d\n",
		successCount, conflictCount, product.Quantity)
}

// demonstrateRetryLogic - リトライロジックのデモ
func (c *CockroachDBDistributed) demonstrateRetryLogic(ctx context.Context) {
	maxRetries := 5
	backoffBase := 100 * time.Millisecond

	operation := func() error {
		// ランダムに失敗
		if rand.Float32() < 0.6 {
			return fmt.Errorf("transient error")
		}
		return nil
	}

	retryWithBackoff := func(op func() error) error {
		var lastErr error
		for i := 0; i < maxRetries; i++ {
			if err := op(); err == nil {
				fmt.Printf("✅ 成功 (試行 %d/%d)\n", i+1, maxRetries)
				return nil
			} else {
				lastErr = err
				backoff := backoffBase * time.Duration(1<<uint(i))
				fmt.Printf("⚠️ リトライ %d/%d (待機: %v)\n", i+1, maxRetries, backoff)
				atomic.AddInt64(&c.stats.Retries, 1)
				time.Sleep(backoff)
			}
		}
		atomic.AddInt64(&c.stats.FailedRetries, 1)
		return fmt.Errorf("max retries exceeded: %w", lastErr)
	}

	// 複数の操作を並行実行
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			fmt.Printf("\n操作 %d 開始:\n", id)
			if err := retryWithBackoff(operation); err != nil {
				fmt.Printf("❌ 操作 %d 失敗: %v\n", id, err)
			}
		}(i)
	}

	wg.Wait()
}

// configureZones - ゾーン設定のデモ
func (c *CockroachDBDistributed) configureZones(ctx context.Context) {
	zones := []struct {
		Name         string
		Replicas     int
		Constraints  []string
		LeasePrefs   []string
	}{
		{
			Name:        "hot-data",
			Replicas:    5,
			Constraints: []string{"ssd", "region=us-east"},
			LeasePrefs:  []string{"region=us-east"},
		},
		{
			Name:        "archive",
			Replicas:    3,
			Constraints: []string{"hdd", "region=us-west"},
			LeasePrefs:  []string{"region=us-west"},
		},
		{
			Name:        "global",
			Replicas:    7,
			Constraints: []string{"region!=antarctica"},
			LeasePrefs:  []string{"region=us-east", "region=eu-west"},
		},
	}

	fmt.Println("📍 ゾーン設定:")
	for _, zone := range zones {
		fmt.Printf("\nゾーン: %s\n", zone.Name)
		fmt.Printf("  レプリカ数: %d\n", zone.Replicas)
		fmt.Printf("  制約: %v\n", zone.Constraints)
		fmt.Printf("  リース優先度: %v\n", zone.LeasePrefs)

		// シミュレートされたデータ配置
		nodes := c.selectNodesForZone(zone.Replicas, zone.Constraints)
		fmt.Printf("  配置ノード: %v\n", nodes)
	}
}

// selectNodesForZone - ゾーンに適したノード選択（シミュレーション）
func (c *CockroachDBDistributed) selectNodesForZone(replicas int, constraints []string) []string {
	availableNodes := []string{
		"node1-us-east-ssd",
		"node2-us-east-ssd",
		"node3-us-west-hdd",
		"node4-us-west-ssd",
		"node5-eu-west-ssd",
		"node6-ap-south-hdd",
		"node7-global-ssd",
	}

	selected := []string{}
	for i := 0; i < replicas && i < len(availableNodes); i++ {
		selected = append(selected, availableNodes[i])
	}
	return selected
}

// createTables - テーブル作成
func (c *CockroachDBDistributed) createTables() {
	if c.db == nil {
		return
	}

	tables := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id INT PRIMARY KEY,
			balance DECIMAL(10,2),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS transaction_log (
			id SERIAL PRIMARY KEY,
			from_id INT,
			to_id INT,
			amount DECIMAL(10,2),
			timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS inventory (
			product_id INT PRIMARY KEY,
			quantity INT,
			version INT DEFAULT 1
		)`,
	}

	for _, table := range tables {
		c.db.Exec(table)
	}
}

// printStatistics - 統計情報表示
func (c *CockroachDBDistributed) printStatistics() {
	fmt.Println("\n📊 実行統計:")
	fmt.Printf("  読み取り: %d\n", atomic.LoadInt64(&c.stats.Reads))
	fmt.Printf("  書き込み: %d\n", atomic.LoadInt64(&c.stats.Writes))
	fmt.Printf("  競合: %d\n", atomic.LoadInt64(&c.stats.Conflicts))
	fmt.Printf("  リトライ: %d\n", atomic.LoadInt64(&c.stats.Retries))
	fmt.Printf("  失敗したリトライ: %d\n", atomic.LoadInt64(&c.stats.FailedRetries))

	if c.stats.TotalLatency > 0 && c.stats.Reads+c.stats.Writes > 0 {
		avgLatency := c.stats.TotalLatency / (c.stats.Reads + c.stats.Writes)
		fmt.Printf("  平均レイテンシ: %dms\n", avgLatency)
	}
}

// Close - リソースクリーンアップ
func (c *CockroachDBDistributed) Close() error {
	if c.db != nil {
		c.db.Close()
	}
	for _, conn := range c.stats.ConnectionPools {
		conn.Close()
	}
	return nil
}