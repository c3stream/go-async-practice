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

// Example4_TransactionalProcessing - トランザクション処理
func (m *MongoDBExample) Example4_TransactionalProcessing() error {
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

// Close - 接続をクローズ
func (m *MongoDBExample) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.client.Disconnect(ctx)
}