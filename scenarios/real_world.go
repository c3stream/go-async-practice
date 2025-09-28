package scenarios

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// RealWorldScenarios - 実世界のシナリオベース問題
type RealWorldScenarios struct{}

// NewRealWorldScenarios - シナリオ問題を初期化
func NewRealWorldScenarios() *RealWorldScenarios {
	return &RealWorldScenarios{}
}

// Scenario1_ECommercePlatform - ECサイトの注文処理システム
func (s *RealWorldScenarios) Scenario1_ECommercePlatform() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║   🛒 シナリオ1: ECサイト注文処理        ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n要件:")
	fmt.Println("  • 在庫確認、決済、配送手配を並行処理")
	fmt.Println("  • いずれかが失敗したら全てロールバック")
	fmt.Println("  • 5秒以内に処理完了")

	type Order struct {
		ID       string
		Items    []string
		Amount   float64
		Status   string
		mu       sync.Mutex
	}

	processOrder := func(order *Order) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 結果を格納
		type Result struct {
			Step  string
			Error error
		}

		results := make(chan Result, 3)
		var wg sync.WaitGroup

		// 1. 在庫確認
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				results <- Result{"inventory", ctx.Err()}
			case <-time.After(time.Duration(500+rand.Intn(1000)) * time.Millisecond):
				if rand.Float32() < 0.9 { // 90%成功
					fmt.Printf("  ✅ 在庫確認完了: 注文 %s\n", order.ID)
					results <- Result{"inventory", nil}
				} else {
					results <- Result{"inventory", fmt.Errorf("在庫不足")}
				}
			}
		}()

		// 2. 決済処理
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				results <- Result{"payment", ctx.Err()}
			case <-time.After(time.Duration(1000+rand.Intn(2000)) * time.Millisecond):
				if rand.Float32() < 0.95 { // 95%成功
					fmt.Printf("  💳 決済完了: ¥%.2f\n", order.Amount)
					results <- Result{"payment", nil}
				} else {
					results <- Result{"payment", fmt.Errorf("決済失敗")}
				}
			}
		}()

		// 3. 配送手配
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				results <- Result{"shipping", ctx.Err()}
			case <-time.After(time.Duration(800+rand.Intn(1500)) * time.Millisecond):
				if rand.Float32() < 0.98 { // 98%成功
					fmt.Printf("  🚚 配送手配完了: 注文 %s\n", order.ID)
					results <- Result{"shipping", nil}
				} else {
					results <- Result{"shipping", fmt.Errorf("配送業者エラー")}
				}
			}
		}()

		// 結果を収集
		go func() {
			wg.Wait()
			close(results)
		}()

		// 全ての結果をチェック
		var errors []error
		for result := range results {
			if result.Error != nil {
				errors = append(errors, fmt.Errorf("%s: %v", result.Step, result.Error))
			}
		}

		// エラーがあればロールバック
		if len(errors) > 0 {
			fmt.Printf("  ❌ 注文処理失敗: %s\n", order.ID)
			for _, err := range errors {
				fmt.Printf("    - %v\n", err)
			}

			// ロールバック処理
			fmt.Println("  🔙 ロールバック実行中...")
			rollback(order)
			return fmt.Errorf("order processing failed")
		}

		order.mu.Lock()
		order.Status = "completed"
		order.mu.Unlock()

		fmt.Printf("  ✨ 注文処理完了: %s\n", order.ID)
		return nil
	}

	// 複数の注文を並行処理
	orders := []*Order{
		{ID: "ORD-001", Items: []string{"商品A", "商品B"}, Amount: 5000},
		{ID: "ORD-002", Items: []string{"商品C"}, Amount: 3000},
		{ID: "ORD-003", Items: []string{"商品D", "商品E", "商品F"}, Amount: 8000},
	}

	fmt.Println("\n📦 注文処理開始...")
	var wg sync.WaitGroup
	for _, order := range orders {
		wg.Add(1)
		go func(o *Order) {
			defer wg.Done()
			processOrder(o)
		}(order)
	}
	wg.Wait()

	// 処理結果サマリー
	fmt.Println("\n📊 処理結果:")
	for _, order := range orders {
		fmt.Printf("  注文 %s: %s\n", order.ID, order.Status)
	}
}

// rollback - ロールバック処理
func rollback(order interface{}) {
	time.Sleep(500 * time.Millisecond)
	fmt.Println("  ↩️ ロールバック完了")
}

// Scenario2_RealTimeChat - リアルタイムチャットシステム
func (s *RealWorldScenarios) Scenario2_RealTimeChat() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║   💬 シナリオ2: リアルタイムチャット     ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n要件:")
	fmt.Println("  • 複数ユーザーの同時接続")
	fmt.Println("  • メッセージのブロードキャスト")
	fmt.Println("  • 接続/切断の通知")

	type ChatRoom struct {
		users     map[string]chan Message
		join      chan User
		leave     chan User
		broadcast chan Message
		mu        sync.RWMutex
	}

	type User struct {
		ID   string
		Name string
	}

	type Message struct {
		From    string
		Content string
		Time    time.Time
	}

	// チャットルーム作成
	room := &ChatRoom{
		users:     make(map[string]chan Message),
		join:      make(chan User),
		leave:     make(chan User),
		broadcast: make(chan Message),
	}

	// チャットルーム管理
	go func() {
		for {
			select {
			case user := <-room.join:
				room.mu.Lock()
				room.users[user.ID] = make(chan Message, 10)
				room.mu.Unlock()

				// 参加通知
				notification := Message{
					From:    "System",
					Content: fmt.Sprintf("%s が参加しました", user.Name),
					Time:    time.Now(),
				}
				room.broadcast <- notification

			case user := <-room.leave:
				room.mu.Lock()
				if ch, ok := room.users[user.ID]; ok {
					close(ch)
					delete(room.users, user.ID)
				}
				room.mu.Unlock()

				// 退出通知
				notification := Message{
					From:    "System",
					Content: fmt.Sprintf("%s が退出しました", user.Name),
					Time:    time.Now(),
				}
				room.broadcast <- notification

			case msg := <-room.broadcast:
				// 全ユーザーにブロードキャスト
				room.mu.RLock()
				for _, ch := range room.users {
					select {
					case ch <- msg:
					default:
						// バッファが満杯の場合はスキップ
					}
				}
				room.mu.RUnlock()
			}
		}
	}()

	// ユーザーをシミュレート
	users := []User{
		{ID: "user1", Name: "田中"},
		{ID: "user2", Name: "鈴木"},
		{ID: "user3", Name: "佐藤"},
	}

	var wg sync.WaitGroup

	// 各ユーザーの処理
	for _, user := range users {
		wg.Add(1)
		go func(u User) {
			defer wg.Done()

			// 参加
			room.join <- u
			ch := make(chan Message, 10)
			room.mu.Lock()
			room.users[u.ID] = ch
			room.mu.Unlock()

			// メッセージ受信
			go func() {
				for msg := range ch {
					if msg.From != u.Name {
						fmt.Printf("  [%s] %s: %s\n",
							msg.Time.Format("15:04:05"),
							msg.From,
							msg.Content)
					}
				}
			}()

			// メッセージ送信をシミュレート
			messages := []string{
				"こんにちは！",
				"今日はいい天気ですね",
				"Go言語楽しいです",
			}

			for _, content := range messages {
				time.Sleep(time.Duration(500+rand.Intn(1000)) * time.Millisecond)
				msg := Message{
					From:    u.Name,
					Content: content,
					Time:    time.Now(),
				}
				room.broadcast <- msg
			}

			// しばらく待機
			time.Sleep(2 * time.Second)

			// 退出
			room.leave <- u
		}(user)
	}

	wg.Wait()
	fmt.Println("\n✅ チャットセッション終了")
}

// Scenario3_LoadBalancer - ロードバランサーの実装
func (s *RealWorldScenarios) Scenario3_LoadBalancer() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║   ⚖️ シナリオ3: ロードバランサー        ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n要件:")
	fmt.Println("  • 複数のサーバーにリクエストを分散")
	fmt.Println("  • ヘルスチェック機能")
	fmt.Println("  • 故障サーバーの自動除外")

	type Server struct {
		ID          string
		Healthy     bool
		Load        int64
		mu          sync.RWMutex
		lastCheck   time.Time
	}

	type LoadBalancer struct {
		servers []*Server
		current uint64
	}

	// ヘルスチェック
	healthCheck := func(server *Server) {
		for {
			time.Sleep(2 * time.Second)

			server.mu.Lock()
			// ランダムに健康状態を変更（95%健康）
			wasHealthy := server.Healthy
			server.Healthy = rand.Float32() < 0.95
			server.lastCheck = time.Now()

			if !wasHealthy && server.Healthy {
				fmt.Printf("  ✅ サーバー %s が復旧しました\n", server.ID)
			} else if wasHealthy && !server.Healthy {
				fmt.Printf("  ❌ サーバー %s がダウンしました\n", server.ID)
			}
			server.mu.Unlock()
		}
	}

	// ラウンドロビンでサーバー選択
	selectServer := func(lb *LoadBalancer) *Server {
		attempts := 0
		for attempts < len(lb.servers)*2 {
			n := atomic.AddUint64(&lb.current, 1)
			server := lb.servers[n%uint64(len(lb.servers))]

			server.mu.RLock()
			healthy := server.Healthy
			server.mu.RUnlock()

			if healthy {
				return server
			}
			attempts++
		}
		return nil
	}

	// リクエスト処理
	handleRequest := func(lb *LoadBalancer, requestID int) {
		server := selectServer(lb)
		if server == nil {
			fmt.Printf("  ⚠️ リクエスト %d: 利用可能なサーバーがありません\n", requestID)
			return
		}

		// 負荷を増加
		atomic.AddInt64(&server.Load, 1)
		defer atomic.AddInt64(&server.Load, -1)

		// 処理時間をシミュレート
		processingTime := time.Duration(100+rand.Intn(400)) * time.Millisecond
		time.Sleep(processingTime)

		fmt.Printf("  📡 リクエスト %d → サーバー %s (処理時間: %dms, 負荷: %d)\n",
			requestID, server.ID, processingTime.Milliseconds(), atomic.LoadInt64(&server.Load))
	}

	// ロードバランサー初期化
	lb := &LoadBalancer{
		servers: []*Server{
			{ID: "srv-1", Healthy: true},
			{ID: "srv-2", Healthy: true},
			{ID: "srv-3", Healthy: true},
			{ID: "srv-4", Healthy: true},
		},
	}

	// ヘルスチェック開始
	for _, server := range lb.servers {
		go healthCheck(server)
	}

	// 統計情報の定期表示
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			fmt.Println("\n📊 サーバー状態:")
			for _, srv := range lb.servers {
				srv.mu.RLock()
				status := "🔴"
				if srv.Healthy {
					status = "🟢"
				}
				fmt.Printf("  %s %s: 負荷=%d\n",
					status, srv.ID, atomic.LoadInt64(&srv.Load))
				srv.mu.RUnlock()
			}
		}
	}()

	// リクエストを並行送信
	fmt.Println("\n🌐 リクエスト処理開始...")
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			handleRequest(lb, id)
		}(i)

		// リクエスト間隔
		time.Sleep(100 * time.Millisecond)
	}

	wg.Wait()
	fmt.Println("\n✅ ロードバランシング完了")
}

// Scenario4_DataPipeline - データ処理パイプライン
func (s *RealWorldScenarios) Scenario4_DataPipeline() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║   📊 シナリオ4: データ処理パイプライン   ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n要件:")
	fmt.Println("  • データの収集、変換、集計を段階的に処理")
	fmt.Println("  • 各段階でエラーハンドリング")
	fmt.Println("  • バックプレッシャー対応")

	type DataPoint struct {
		ID        int
		Value     float64
		Timestamp time.Time
		Category  string
	}

	// Stage 1: データ収集
	collect := func() <-chan DataPoint {
		out := make(chan DataPoint, 10)
		go func() {
			defer close(out)
			categories := []string{"A", "B", "C", "D"}

			for i := 0; i < 50; i++ {
				dp := DataPoint{
					ID:        i,
					Value:     rand.Float64() * 100,
					Timestamp: time.Now(),
					Category:  categories[rand.Intn(len(categories))],
				}
				out <- dp

				// バックプレッシャーシミュレーション
				if len(out) > 8 {
					fmt.Println("  ⚠️ バックプレッシャー検出: 収集を減速")
					time.Sleep(200 * time.Millisecond)
				} else {
					time.Sleep(50 * time.Millisecond)
				}
			}
			fmt.Println("  ✅ データ収集完了")
		}()
		return out
	}

	// Stage 2: データ変換
	transform := func(in <-chan DataPoint) <-chan DataPoint {
		out := make(chan DataPoint, 10)
		go func() {
			defer close(out)
			for dp := range in {
				// データの正規化
				dp.Value = dp.Value * 1.1 // 10%増加

				// エラーシミュレーション
				if rand.Float32() < 0.05 { // 5%エラー
					fmt.Printf("  ❌ 変換エラー: データID %d をスキップ\n", dp.ID)
					continue
				}

				out <- dp
			}
			fmt.Println("  ✅ データ変換完了")
		}()
		return out
	}

	// Stage 3: データ集計
	aggregate := func(in <-chan DataPoint) {
		categoryTotals := make(map[string]float64)
		categoryCount := make(map[string]int)
		var total float64
		var count int

		for dp := range in {
			categoryTotals[dp.Category] += dp.Value
			categoryCount[dp.Category]++
			total += dp.Value
			count++

			// 定期的に中間結果を表示
			if count%10 == 0 {
				fmt.Printf("  📈 処理済み: %d件, 合計値: %.2f\n", count, total)
			}
		}

		// 最終結果
		fmt.Println("\n📊 集計結果:")
		fmt.Printf("  総データ数: %d\n", count)
		fmt.Printf("  合計値: %.2f\n", total)
		fmt.Printf("  平均値: %.2f\n", total/float64(count))

		fmt.Println("\n📊 カテゴリ別集計:")
		for cat, sum := range categoryTotals {
			avg := sum / float64(categoryCount[cat])
			fmt.Printf("  カテゴリ %s: 件数=%d, 合計=%.2f, 平均=%.2f\n",
				cat, categoryCount[cat], sum, avg)
		}
	}

	// パイプライン実行
	fmt.Println("\n🔄 パイプライン開始...")
	start := time.Now()

	collected := collect()
	transformed := transform(collected)
	aggregate(transformed)

	elapsed := time.Since(start)
	fmt.Printf("\n✅ パイプライン完了: 処理時間 %v\n", elapsed)
}