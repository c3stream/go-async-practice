package practical

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDBExample - MongoDB を使った並行処理の実践例
type MongoDBExample struct {
	client *mongo.Client
	db     *mongo.Database
}

// NewMongoDBExample - MongoDB接続を初期化
func NewMongoDBExample() (*MongoDBExample, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// MongoDB接続
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://admin:password@localhost:27017"))
	if err != nil {
		return nil, err
	}

	// Ping確認
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	return &MongoDBExample{
		client: client,
		db:     client.Database("asyncdb"),
	}, nil
}

// Example1_ConcurrentWrites - 並行書き込みパターン
func (m *MongoDBExample) Example1_ConcurrentWrites() error {
	fmt.Println("\n📦 MongoDB: 並行書き込みパターン")
	collection := m.db.Collection("events")

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// 100個のドキュメントを並行で書き込み
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			doc := bson.M{
				"event_id":   fmt.Sprintf("event_%d", id),
				"timestamp":  time.Now(),
				"type":       "user_action",
				"user_id":    fmt.Sprintf("user_%d", id%10),
				"action":     "click",
				"metadata":   bson.M{"page": fmt.Sprintf("page_%d", id%5)},
			}

			_, err := collection.InsertOne(ctx, doc)
			if err != nil {
				errors <- err
				return
			}
			fmt.Printf("  ✅ イベント %d を保存\n", id)
		}(i)
	}

	// エラーチェック
	go func() {
		wg.Wait()
		close(errors)
	}()

	for err := range errors {
		fmt.Printf("  ❌ エラー: %v\n", err)
	}

	return nil
}

// Example2_StreamProcessing - Change Streams を使ったリアルタイム処理
func (m *MongoDBExample) Example2_StreamProcessing() error {
	fmt.Println("\n🔄 MongoDB: Change Streams でリアルタイム処理")
	collection := m.db.Collection("realtime")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Change Stream を開く
	pipeline := mongo.Pipeline{}
	stream, err := collection.Watch(ctx, pipeline)
	if err != nil {
		return err
	}
	defer stream.Close(ctx)

	// リアルタイムで変更を監視
	go func() {
		for stream.Next(ctx) {
			var event bson.M
			if err := stream.Decode(&event); err != nil {
				fmt.Printf("  ❌ デコードエラー: %v\n", err)
				continue
			}
			fmt.Printf("  📨 変更検出: %v\n", event["operationType"])
		}
	}()

	// データを挿入してストリームをトリガー
	for i := 0; i < 5; i++ {
		doc := bson.M{
			"id":      i,
			"message": fmt.Sprintf("リアルタイムメッセージ %d", i),
			"time":    time.Now(),
		}
		_, err := collection.InsertOne(ctx, doc)
		if err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

// Example3_AggregationPipeline - 並列集計処理
func (m *MongoDBExample) Example3_AggregationPipeline() error {
	fmt.Println("\n📊 MongoDB: 並列集計パイプライン")
	collection := m.db.Collection("analytics")

	// サンプルデータ挿入
	ctx := context.Background()
	docs := []interface{}{}
	for i := 0; i < 1000; i++ {
		docs = append(docs, bson.M{
			"user_id":  fmt.Sprintf("user_%d", i%100),
			"product":  fmt.Sprintf("product_%d", i%20),
			"amount":   float64(100 + i%500),
			"category": fmt.Sprintf("category_%d", i%5),
			"date":     time.Now().Add(-time.Duration(i) * time.Hour),
		})
	}
	collection.InsertMany(ctx, docs)

	// 複数の集計を並列実行
	var wg sync.WaitGroup

	// 集計1: カテゴリ別売上
	wg.Add(1)
	go func() {
		defer wg.Done()
		pipeline := mongo.Pipeline{
			{{"$group", bson.D{
				{"_id", "$category"},
				{"total", bson.D{{"$sum", "$amount"}}},
				{"count", bson.D{{"$sum", 1}}},
			}}},
			{{"$sort", bson.D{{"total", -1}}}},
		}

		cursor, err := collection.Aggregate(ctx, pipeline)
		if err != nil {
			fmt.Printf("  ❌ 集計エラー: %v\n", err)
			return
		}
		defer cursor.Close(ctx)

		fmt.Println("  📈 カテゴリ別売上:")
		for cursor.Next(ctx) {
			var result bson.M
			cursor.Decode(&result)
			fmt.Printf("    %s: ¥%.2f (件数: %.0f)\n",
				result["_id"], result["total"], result["count"])
		}
	}()

	// 集計2: ユーザー別購入数TOP10
	wg.Add(1)
	go func() {
		defer wg.Done()
		pipeline := mongo.Pipeline{
			{{"$group", bson.D{
				{"_id", "$user_id"},
				{"purchase_count", bson.D{{"$sum", 1}}},
			}}},
			{{"$sort", bson.D{{"purchase_count", -1}}}},
			{{"$limit", 10}},
		}

		cursor, err := collection.Aggregate(ctx, pipeline)
		if err != nil {
			fmt.Printf("  ❌ 集計エラー: %v\n", err)
			return
		}
		defer cursor.Close(ctx)

		fmt.Println("  🏆 購入数TOP10ユーザー:")
		for cursor.Next(ctx) {
			var result bson.M
			cursor.Decode(&result)
			fmt.Printf("    %s: %v回\n", result["_id"], result["purchase_count"])
		}
	}()

	wg.Wait()
	return nil
}

// Example4_ConcurrentAggregationStreaming - ストリーミング集計処理
func (m *MongoDBExample) Example4_ConcurrentAggregationStreaming() error {
	fmt.Println("\n📊 MongoDB: 並行ストリーミング集計処理")
	collection := m.db.Collection("streaming_data")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. リアルタイムデータ生成器
	dataStream := make(chan bson.M, 1000)
	var dataGenWg sync.WaitGroup

	// データ生成goroutine
	dataGenWg.Add(1)
	go func() {
		defer dataGenWg.Done()
		defer close(dataStream)

		for i := 0; i < 5000; i++ {
			select {
			case <-ctx.Done():
				return
			case dataStream <- bson.M{
				"event_id":   fmt.Sprintf("event_%d", i),
				"user_id":    fmt.Sprintf("user_%d", i%100),
				"product_id": fmt.Sprintf("product_%d", i%20),
				"amount":     float64(100 + i%500),
				"timestamp":  time.Now(),
				"category":   fmt.Sprintf("cat_%d", i%5),
			}:
			}
			if i%100 == 0 {
				time.Sleep(10 * time.Millisecond) // バッチ間隔
			}
		}
	}()

	// 2. 並行書き込み（バッチ処理）
	var writeWg sync.WaitGroup
	numWriters := 3
	batchSize := 100

	for w := 0; w < numWriters; w++ {
		writeWg.Add(1)
		go func(workerID int) {
			defer writeWg.Done()

			batch := make([]interface{}, 0, batchSize)
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					// 残りのバッチを処理
					if len(batch) > 0 {
						collection.InsertMany(ctx, batch)
						fmt.Printf("  📦 Writer%d: 最終バッチ %d件\n", workerID, len(batch))
					}
					return

				case data, ok := <-dataStream:
					if !ok {
						// ストリーム終了
						if len(batch) > 0 {
							collection.InsertMany(ctx, batch)
							fmt.Printf("  📦 Writer%d: 最終バッチ %d件\n", workerID, len(batch))
						}
						return
					}

					batch = append(batch, data)
					if len(batch) >= batchSize {
						if _, err := collection.InsertMany(ctx, batch); err == nil {
							fmt.Printf("  📦 Writer%d: バッチ %d件書き込み完了\n", workerID, len(batch))
						}
						batch = batch[:0] // クリア
					}

				case <-ticker.C:
					// 定期バッチ処理
					if len(batch) > 0 {
						if _, err := collection.InsertMany(ctx, batch); err == nil {
							fmt.Printf("  📦 Writer%d: 定期バッチ %d件\n", workerID, len(batch))
						}
						batch = batch[:0]
					}
				}
			}
		}(w)
	}

	// 3. リアルタイム集計（並行処理）
	var aggWg sync.WaitGroup
	aggregations := []struct {
		name     string
		pipeline mongo.Pipeline
	}{
		{
			name: "カテゴリ別リアルタイム売上",
			pipeline: mongo.Pipeline{
				{{"$match", bson.D{{"timestamp", bson.D{{"$gte", time.Now().Add(-1 * time.Minute)}}}}}},
				{{"$group", bson.D{
					{"_id", "$category"},
					{"total_amount", bson.D{{"$sum", "$amount"}}},
					{"count", bson.D{{"$sum", 1}}},
					{"avg_amount", bson.D{{"$avg", "$amount"}}},
				}}},
				{{"$sort", bson.D{{"total_amount", -1}}}},
			},
		},
		{
			name: "TOP購入ユーザー",
			pipeline: mongo.Pipeline{
				{{"$group", bson.D{
					{"_id", "$user_id"},
					{"total_spent", bson.D{{"$sum", "$amount"}}},
					{"purchase_count", bson.D{{"$sum", 1}}},
				}}},
				{{"$sort", bson.D{{"total_spent", -1}}}},
				{{"$limit", 5}},
			},
		},
	}

	// 集計を定期実行
	aggWg.Add(len(aggregations))
	for _, agg := range aggregations {
		go func(name string, pipeline mongo.Pipeline) {
			defer aggWg.Done()

			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					cursor, err := collection.Aggregate(ctx, pipeline)
					if err != nil {
						continue
					}

					fmt.Printf("\n  📈 %s:\n", name)
					count := 0
					for cursor.Next(ctx) && count < 3 {
						var result bson.M
						cursor.Decode(&result)
						if name == "カテゴリ別リアルタイム売上" {
							fmt.Printf("    %v: ¥%.0f (件数: %.0f, 平均: ¥%.0f)\n",
								result["_id"], result["total_amount"], result["count"], result["avg_amount"])
						} else {
							fmt.Printf("    %v: ¥%.0f (%v回購入)\n",
								result["_id"], result["total_spent"], result["purchase_count"])
						}
						count++
					}
					cursor.Close(ctx)
				}
			}
		}(agg.name, agg.pipeline)
	}

	// 実行と待機
	go func() {
		dataGenWg.Wait()
		writeWg.Wait()
		time.Sleep(2 * time.Second) // 最後の集計を表示
		cancel()
	}()

	aggWg.Wait()
	fmt.Println("  ✅ ストリーミング集計処理完了")
	return nil
}

// Example5_TransactionalProcessing - トランザクション処理
func (m *MongoDBExample) Example5_TransactionalProcessing() error {
	fmt.Println("\n🔐 MongoDB: 並行トランザクション処理")

	// セッション開始
	session, err := m.client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(context.Background())

	// トランザクション実行
	callback := func(sessionContext mongo.SessionContext) (interface{}, error) {
		accounts := m.db.Collection("accounts")
		transactions := m.db.Collection("transactions")

		// 送金処理（アトミック）
		sender := "user_001"
		receiver := "user_002"
		amount := 1000.0

		// 送金元から引き落とし
		_, err := accounts.UpdateOne(sessionContext,
			bson.M{"user_id": sender},
			bson.M{"$inc": bson.M{"balance": -amount}},
		)
		if err != nil {
			return nil, err
		}

		// 送金先に入金
		_, err = accounts.UpdateOne(sessionContext,
			bson.M{"user_id": receiver},
			bson.M{"$inc": bson.M{"balance": amount}},
		)
		if err != nil {
			return nil, err
		}

		// トランザクション記録
		_, err = transactions.InsertOne(sessionContext, bson.M{
			"from":      sender,
			"to":        receiver,
			"amount":    amount,
			"timestamp": time.Now(),
			"status":    "completed",
		})

		return nil, err
	}

	// トランザクション実行
	_, err = session.WithTransaction(context.Background(), callback)
	if err != nil {
		fmt.Printf("  ❌ トランザクション失敗: %v\n", err)
		return err
	}

	fmt.Println("  ✅ トランザクション完了")
	return nil
}

// Example6_GraphLookupPattern - グラフルックアップパターン
func (m *MongoDBExample) Example6_GraphLookupPattern() error {
	fmt.Println("\n🕸️ MongoDB: グラフルックアップパターン")
	collection := m.db.Collection("employees")

	ctx := context.Background()

	// サンプルデータ挿入
	employees := []interface{}{
		bson.M{"_id": "ceo", "name": "CEO", "reports_to": nil},
		bson.M{"_id": "vp1", "name": "VP Engineering", "reports_to": "ceo"},
		bson.M{"_id": "vp2", "name": "VP Sales", "reports_to": "ceo"},
		bson.M{"_id": "mgr1", "name": "Dev Manager 1", "reports_to": "vp1"},
		bson.M{"_id": "mgr2", "name": "Dev Manager 2", "reports_to": "vp1"},
		bson.M{"_id": "dev1", "name": "Developer 1", "reports_to": "mgr1"},
		bson.M{"_id": "dev2", "name": "Developer 2", "reports_to": "mgr1"},
		bson.M{"_id": "dev3", "name": "Developer 3", "reports_to": "mgr2"},
	}
	collection.InsertMany(ctx, employees)

	// グラフルックアップ集計
	pipeline := mongo.Pipeline{
		{{
			"$graphLookup", bson.D{
				{"from", "employees"},
				{"startWith", "$_id"},
				{"connectFromField", "_id"},
				{"connectToField", "reports_to"},
				{"as", "subordinates"},
				{"maxDepth", 3},
			},
		}},
		{{
			"$addFields", bson.D{
				{"total_subordinates", bson.D{{"$size", "$subordinates"}}},
			},
		}},
		{{"$sort", bson.D{{"total_subordinates", -1}}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	fmt.Println("  👥 組織階層:")
	for cursor.Next(ctx) {
		var result bson.M
		cursor.Decode(&result)
		fmt.Printf("    %s: %v人の部下\n", result["name"], result["total_subordinates"])
	}

	return nil
}

// Example7_ConcurrentBulkOperations - 並行バルク操作
func (m *MongoDBExample) Example7_ConcurrentBulkOperations() error {
	fmt.Println("\n⚡ MongoDB: 並行バルク操作")
	collection := m.db.Collection("bulk_data")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	numWorkers := 3
	batchSize := 1000

	// 並行でバルク操作実行
	for worker := 0; worker < numWorkers; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// バルク書き込み操作
			for batch := 0; batch < 5; batch++ {
				operations := make([]mongo.WriteModel, 0, batchSize)

				for i := 0; i < batchSize; i++ {
					docID := fmt.Sprintf("worker_%d_batch_%d_doc_%d", workerID, batch, i)
					doc := bson.M{
						"_id":       docID,
						"worker_id": workerID,
						"batch_id":  batch,
						"data":      fmt.Sprintf("data_%d", i),
						"timestamp": time.Now(),
						"random":    fmt.Sprintf("%d", time.Now().UnixNano()%1000),
					}

					operations = append(operations, mongo.NewInsertOneModel().SetDocument(doc))
				}

				// バルク実行
				opts := options.BulkWrite().SetOrdered(false) // 順序なし（高速）
				result, err := collection.BulkWrite(ctx, operations, opts)
				if err != nil {
					fmt.Printf("  ❌ Worker%d バッチ%d エラー: %v\n", workerID, batch, err)
					continue
				}

				fmt.Printf("  📦 Worker%d バッチ%d: %d件挿入完了\n",
					workerID, batch, result.InsertedCount)
			}
		}(worker)
	}

	wg.Wait()

	// 結果確認
	count, _ := collection.CountDocuments(ctx, bson.M{})
	fmt.Printf("  ✅ 総挿入件数: %d件\n", count)

	return nil
}

// Example8_TextSearchWithConcurrency - 並行テキスト検索
func (m *MongoDBExample) Example8_TextSearchWithConcurrency() error {
	fmt.Println("\n🔍 MongoDB: 並行テキスト検索")
	collection := m.db.Collection("articles")

	ctx := context.Background()

	// テキストインデックス作成
	indexModel := mongo.IndexModel{
		Keys: bson.D{
			{"title", "text"},
			{"content", "text"},
		},
		Options: options.Index().SetDefaultLanguage("japanese"),
	}
	collection.Indexes().CreateOne(ctx, indexModel)

	// サンプルデータ挿入
	articles := []interface{}{
		bson.M{"title": "Go言語 並行プログラミング入門", "content": "Goroutineとchannelの基本的な使い方", "category": "技術"},
		bson.M{"title": "MongoDB パフォーマンス最適化", "content": "インデックスとクエリ最適化のベストプラクティス", "category": "データベース"},
		bson.M{"title": "マイクロサービス アーキテクチャ", "content": "分散システムの設計パターンと実装", "category": "アーキテクチャ"},
		bson.M{"title": "Kubernetes 運用ガイド", "content": "コンテナオーケストレーションの実践", "category": "インフラ"},
		bson.M{"title": "React フロントエンド開発", "content": "モダンなWebアプリケーション開発手法", "category": "フロントエンド"},
	}
	collection.InsertMany(ctx, articles)

	// 並行でテキスト検索実行
	searchTerms := []string{"Go", "MongoDB", "Kubernetes", "React", "アーキテクチャ"}
	var wg sync.WaitGroup
	results := make(chan map[string]interface{}, len(searchTerms))

	for _, term := range searchTerms {
		wg.Add(1)
		go func(searchTerm string) {
			defer wg.Done()

			filter := bson.M{"$text": bson.M{"$search": searchTerm}}
			projection := bson.M{"score": bson.M{"$meta": "textScore"}}
			opts := options.Find().SetProjection(projection).SetSort(bson.M{"score": bson.M{"$meta": "textScore"}})

			cursor, err := collection.Find(ctx, filter, opts)
			if err != nil {
				fmt.Printf("  ❌ 検索エラー '%s': %v\n", searchTerm, err)
				return
			}
			defer cursor.Close(ctx)

			var docs []bson.M
			if err = cursor.All(ctx, &docs); err != nil {
				fmt.Printf("  ❌ 結果取得エラー '%s': %v\n", searchTerm, err)
				return
			}

			results <- map[string]interface{}{
				"term":    searchTerm,
				"count":   len(docs),
				"results": docs,
			}
		}(term)
	}

	// 結果収集
	go func() {
		wg.Wait()
		close(results)
	}()

	// 結果表示
	for result := range results {
		term := result["term"].(string)
		count := result["count"].(int)
		docs := result["results"].([]bson.M)

		fmt.Printf("  🔍 検索語 '%s': %d件ヒット\n", term, count)
		for _, doc := range docs {
			if title, ok := doc["title"]; ok {
				if score, ok := doc["score"]; ok {
					fmt.Printf("    - %s (スコア: %.2f)\n", title, score)
				}
			}
		}
	}

	return nil
}

// Close - 接続をクローズ
func (m *MongoDBExample) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.client.Disconnect(ctx)
}