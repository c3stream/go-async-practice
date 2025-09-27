package tests

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
	"github.com/segmentio/kafka-go"
	"github.com/streadway/amqp"
)

// TestRedisConnection Redis接続テスト
func TestRedisConnection(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("Skipping integration test")
	}

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()

	// Ping
	_, err := client.Ping(ctx).Result()
	if err != nil {
		t.Fatalf("Failed to ping Redis: %v", err)
	}

	// Set/Get
	key := "test:key"
	value := "test-value"

	err = client.Set(ctx, key, value, 1*time.Second).Err()
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	got, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	if got != value {
		t.Errorf("Expected %s, got %s", value, got)
	}

	// Pub/Sub
	pubsub := client.Subscribe(ctx, "test:channel")
	defer pubsub.Close()

	// Publish
	err = client.Publish(ctx, "test:channel", "test-message").Err()
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}
}

// TestPostgreSQLConnection PostgreSQL接続テスト
func TestPostgreSQLConnection(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("Skipping integration test")
	}

	db, err := sql.Open("postgres",
		"host=localhost port=5432 user=gouser password=gopass dbname=goasync sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// Ping
	err = db.Ping()
	if err != nil {
		t.Fatalf("Failed to ping PostgreSQL: %v", err)
	}

	// Create table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS test_table (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert
	var id int
	err = db.QueryRow(
		"INSERT INTO test_table (name) VALUES ($1) RETURNING id",
		"test-name",
	).Scan(&id)
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	// Select
	var name string
	err = db.QueryRow("SELECT name FROM test_table WHERE id = $1", id).Scan(&name)
	if err != nil {
		t.Fatalf("Failed to select: %v", err)
	}

	if name != "test-name" {
		t.Errorf("Expected test-name, got %s", name)
	}

	// Clean up
	_, err = db.Exec("DROP TABLE test_table")
	if err != nil {
		t.Logf("Failed to drop table: %v", err)
	}
}

// TestRabbitMQConnection RabbitMQ接続テスト
func TestRabbitMQConnection(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("Skipping integration test")
	}

	conn, err := amqp.Dial("amqp://admin:admin@localhost:5672/")
	if err != nil {
		t.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open channel: %v", err)
	}
	defer ch.Close()

	// Declare queue
	q, err := ch.QueueDeclare(
		"test_queue",
		false, // durable
		true,  // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		t.Fatalf("Failed to declare queue: %v", err)
	}

	// Publish
	body := "test message"
	err = ch.Publish(
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        []byte(body),
		})
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	// Consume
	msgs, err := ch.Consume(
		q.Name,
		"",    // consumer
		true,  // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		t.Fatalf("Failed to consume: %v", err)
	}

	select {
	case msg := <-msgs:
		if string(msg.Body) != body {
			t.Errorf("Expected %s, got %s", body, string(msg.Body))
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for message")
	}
}

// TestKafkaConnection Kafka接続テスト
func TestKafkaConnection(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("Skipping integration test")
	}

	topic := "test-topic"
	partition := 0

	// Writer
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{"localhost:9092"},
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	})
	defer writer.Close()

	// Write message
	err := writer.WriteMessages(context.Background(),
		kafka.Message{
			Key:   []byte("test-key"),
			Value: []byte("test-value"),
		},
	)
	if err != nil {
		t.Fatalf("Failed to write message: %v", err)
	}

	// Reader
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{"localhost:9092"},
		Topic:     topic,
		Partition: partition,
		MinBytes:  1,
		MaxBytes:  10e6,
	})
	defer reader.Close()

	// Read message
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	msg, err := reader.ReadMessage(ctx)
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	if string(msg.Value) != "test-value" {
		t.Errorf("Expected test-value, got %s", string(msg.Value))
	}
}

// TestMinIOConnection MinIO (S3) 接続テスト
func TestMinIOConnection(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("Skipping integration test")
	}

	// MinIOのHealth check
	resp, err := http.Get("http://localhost:9000/minio/health/live")
	if err != nil {
		t.Fatalf("Failed to connect to MinIO: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("MinIO health check failed: status %d", resp.StatusCode)
	}
}

// TestServicesIntegration 複数サービスの統合テスト
func TestServicesIntegration(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()

	// Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer redisClient.Close()

	// PostgreSQL
	db, err := sql.Open("postgres",
		"host=localhost port=5432 user=gouser password=gopass dbname=goasync sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer db.Close()

	// ワークフロー: データを保存してキャッシュ
	t.Run("Save and Cache Workflow", func(t *testing.T) {
		// 1. PostgreSQLに保存
		_, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS workflow_test (
				id SERIAL PRIMARY KEY,
				data JSONB
			)
		`)
		if err != nil {
			t.Fatalf("Failed to create table: %v", err)
		}

		data := map[string]interface{}{
			"user":      "test-user",
			"action":    "test-action",
			"timestamp": time.Now().Unix(),
		}

		jsonData, _ := json.Marshal(data)

		var id int
		err = db.QueryRow(
			"INSERT INTO workflow_test (data) VALUES ($1) RETURNING id",
			jsonData,
		).Scan(&id)
		if err != nil {
			t.Fatalf("Failed to insert: %v", err)
		}

		// 2. Redisにキャッシュ
		cacheKey := fmt.Sprintf("workflow:%d", id)
		err = redisClient.Set(ctx, cacheKey, jsonData, 10*time.Second).Err()
		if err != nil {
			t.Fatalf("Failed to cache: %v", err)
		}

		// 3. キャッシュから取得
		cached, err := redisClient.Get(ctx, cacheKey).Result()
		if err != nil {
			t.Fatalf("Failed to get from cache: %v", err)
		}

		var cachedData map[string]interface{}
		json.Unmarshal([]byte(cached), &cachedData)

		if cachedData["user"] != data["user"] {
			t.Errorf("Cached data mismatch")
		}

		// Clean up
		db.Exec("DROP TABLE workflow_test")
	})
}

// TestConcurrentDatabaseAccess 並行データベースアクセステスト
func TestConcurrentDatabaseAccess(t *testing.T) {
	if os.Getenv("SKIP_INTEGRATION") != "" {
		t.Skip("Skipping integration test")
	}

	db, err := sql.Open("postgres",
		"host=localhost port=5432 user=gouser password=gopass dbname=goasync sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer db.Close()

	// コネクションプール設定
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	// テーブル作成
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS concurrent_test (
			id SERIAL PRIMARY KEY,
			counter INT DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	defer db.Exec("DROP TABLE concurrent_test")

	// 初期レコード作成
	var recordID int
	err = db.QueryRow("INSERT INTO concurrent_test (counter) VALUES (0) RETURNING id").Scan(&recordID)
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	// 並行更新
	concurrency := 10
	updates := 100
	done := make(chan bool, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			for j := 0; j < updates; j++ {
				_, err := db.Exec(
					"UPDATE concurrent_test SET counter = counter + 1 WHERE id = $1",
					recordID,
				)
				if err != nil {
					t.Errorf("Update failed: %v", err)
				}
			}
			done <- true
		}()
	}

	// 全goroutine完了待ち
	for i := 0; i < concurrency; i++ {
		<-done
	}

	// 結果確認
	var counter int
	err = db.QueryRow("SELECT counter FROM concurrent_test WHERE id = $1", recordID).Scan(&counter)
	if err != nil {
		t.Fatalf("Failed to select: %v", err)
	}

	expected := concurrency * updates
	if counter != expected {
		t.Errorf("Expected counter=%d, got %d", expected, counter)
	}
}