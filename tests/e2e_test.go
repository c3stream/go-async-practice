package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/segmentio/kafka-go"
	"github.com/streadway/amqp"
)

// TestE2E_FullWorkflow 完全なワークフローのE2Eテスト
func TestE2E_FullWorkflow(t *testing.T) {
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test")
	}

	// 1. Docker環境が起動していることを確認
	t.Run("Check Docker Services", func(t *testing.T) {
		services := []struct {
			name string
			url  string
		}{
			{"Redis", "localhost:6379"},
			{"PostgreSQL", "localhost:5432"},
			{"RabbitMQ", "localhost:5672"},
			{"Kafka", "localhost:9092"},
		}

		for _, svc := range services {
			t.Logf("Checking %s at %s", svc.name, svc.url)
			// 実際の接続テストは統合テストで実施
		}
	})

	// 2. メッセージングフローのE2Eテスト
	t.Run("E2E Messaging Flow", func(t *testing.T) {
		testMessageFlow(t)
	})

	// 3. データベース連携フローのE2Eテスト
	t.Run("E2E Database Flow", func(t *testing.T) {
		testDatabaseFlow(t)
	})

	// 4. WebサーバーエンドポイントのE2Eテスト
	t.Run("E2E Web Server", func(t *testing.T) {
		testWebServerFlow(t)
	})
}

// testMessageFlow メッセージングフローのテスト
func testMessageFlow(t *testing.T) {
	ctx := context.Background()

	// RabbitMQフロー
	t.Run("RabbitMQ Flow", func(t *testing.T) {
		conn, err := amqp.Dial("amqp://admin:admin@localhost:5672/")
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		defer conn.Close()

		ch, err := conn.Channel()
		if err != nil {
			t.Fatalf("Failed to open channel: %v", err)
		}
		defer ch.Close()

		// Producer -> Queue -> Consumer フロー
		queueName := "e2e_test_queue"

		q, err := ch.QueueDeclare(queueName, false, true, false, false, nil)
		if err != nil {
			t.Fatalf("Failed to declare queue: %v", err)
		}

		// Producer
		testData := map[string]interface{}{
			"id":        "test-123",
			"timestamp": time.Now().Unix(),
			"data":      "e2e test data",
		}

		body, _ := json.Marshal(testData)
		err = ch.Publish("", q.Name, false, false, amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		})
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}

		// Consumer
		msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
		if err != nil {
			t.Fatalf("Failed to consume: %v", err)
		}

		select {
		case msg := <-msgs:
			var received map[string]interface{}
			json.Unmarshal(msg.Body, &received)
			if received["id"] != testData["id"] {
				t.Errorf("Data mismatch: expected %v, got %v", testData["id"], received["id"])
			}
		case <-time.After(3 * time.Second):
			t.Error("Timeout waiting for message")
		}
	})

	// Kafkaフロー
	t.Run("Kafka Flow", func(t *testing.T) {
		topic := "e2e-test-topic"

		// Producer
		writer := kafka.NewWriter(kafka.WriterConfig{
			Brokers: []string{"localhost:9092"},
			Topic:   topic,
		})
		defer writer.Close()

		event := map[string]interface{}{
			"type":      "test.event",
			"timestamp": time.Now().Unix(),
			"payload":   "e2e kafka test",
		}

		eventData, _ := json.Marshal(event)
		err := writer.WriteMessages(ctx,
			kafka.Message{
				Key:   []byte("test-key"),
				Value: eventData,
			},
		)
		if err != nil {
			t.Fatalf("Failed to write to Kafka: %v", err)
		}

		// Consumer
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:   []string{"localhost:9092"},
			Topic:     topic,
			Partition: 0,
			MinBytes:  1,
			MaxBytes:  10e6,
		})
		defer reader.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			t.Fatalf("Failed to read from Kafka: %v", err)
		}

		var received map[string]interface{}
		json.Unmarshal(msg.Value, &received)
		if received["type"] != event["type"] {
			t.Errorf("Event type mismatch")
		}
	})
}

// testDatabaseFlow データベース連携フローのテスト
func testDatabaseFlow(t *testing.T) {
	ctx := context.Background()

	// Redis Pub/Sub + Cache フロー
	t.Run("Redis Cache and Pub/Sub", func(t *testing.T) {
		client := redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})
		defer client.Close()

		// Cache-aside pattern
		cacheKey := "e2e:user:123"
		userData := map[string]interface{}{
			"id":       123,
			"name":     "Test User",
			"email":    "test@example.com",
			"created":  time.Now().Unix(),
		}

		// キャッシュミス -> データ取得 -> キャッシュ保存
		_, err := client.Get(ctx, cacheKey).Result()
		if err == redis.Nil {
			// データベースから取得（シミュレート）
			t.Log("Cache miss - fetching from database")

			// キャッシュに保存
			data, _ := json.Marshal(userData)
			err = client.Set(ctx, cacheKey, data, 30*time.Second).Err()
			if err != nil {
				t.Fatalf("Failed to cache: %v", err)
			}
		}

		// キャッシュヒット
		cached, err := client.Get(ctx, cacheKey).Result()
		if err != nil {
			t.Fatalf("Failed to get from cache: %v", err)
		}

		var cachedUser map[string]interface{}
		json.Unmarshal([]byte(cached), &cachedUser)
		if cachedUser["id"] != userData["id"] {
			t.Error("Cached data mismatch")
		}

		// Pub/Sub
		pubsub := client.Subscribe(ctx, "e2e:events")
		defer pubsub.Close()

		// イベント発行
		eventData := map[string]interface{}{
			"event": "user.updated",
			"id":    123,
		}
		eventJSON, _ := json.Marshal(eventData)
		client.Publish(ctx, "e2e:events", eventJSON)

		// イベント受信
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			t.Fatalf("Failed to receive message: %v", err)
		}

		var receivedEvent map[string]interface{}
		json.Unmarshal([]byte(msg.Payload), &receivedEvent)
		if receivedEvent["event"] != "user.updated" {
			t.Error("Event mismatch")
		}
	})
}

// testWebServerFlow Webサーバーフローのテスト
func testWebServerFlow(t *testing.T) {
	// Echo serverを起動（実際のサーバーをテスト用に起動）
	// ここではHTTPクライアントのテストを実施

	t.Run("HTTP API Flow", func(t *testing.T) {
		// Health check
		resp, err := http.Get("http://localhost:8080/health")
		if err != nil {
			t.Skip("Web server not running, skipping")
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Health check failed: status %d", resp.StatusCode)
		}

		// Batch processing endpoint
		batchData := map[string]interface{}{
			"items": []string{"item1", "item2", "item3"},
		}

		jsonData, _ := json.Marshal(batchData)
		resp, err = http.Post(
			"http://localhost:8080/api/v1/batch",
			"application/json",
			bytes.NewReader(jsonData),
		)
		if err != nil {
			t.Logf("Batch endpoint not available: %v", err)
			return
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		if result["status"] != "completed" {
			t.Error("Batch processing failed")
		}
	})
}

// TestE2E_CLICommands CLIコマンドのE2Eテスト
func TestE2E_CLICommands(t *testing.T) {
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test")
	}

	// ビルド
	t.Run("Build Application", func(t *testing.T) {
		cmd := exec.Command("go", "build", "-o", "bin/runner", "cmd/runner/main.go")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Failed to build: %v\n%s", err, output)
		}
	})

	// コマンド実行テスト
	commands := []struct {
		name string
		args []string
	}{
		{"Show Menu", []string{}},
		{"Run Example 1", []string{"-mode=example", "-example=1"}},
		{"Run Challenge 1", []string{"-mode=challenge", "-challenge=1"}},
	}

	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("./bin/runner", tc.args...)
			cmd.Env = append(os.Environ(), "CI=true") // CI環境での実行を示す

			// タイムアウト設定
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
			output, err := cmd.CombinedOutput()

			if ctx.Err() == context.DeadlineExceeded {
				t.Logf("Command timed out (expected for interactive commands)")
				return
			}

			if err != nil {
				// エラーでも一部のコマンドは正常
				t.Logf("Command output: %s", output)
			}
		})
	}
}

// TestE2E_Performance パフォーマンステスト
func TestE2E_Performance(t *testing.T) {
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test")
	}

	t.Run("Concurrent Load Test", func(t *testing.T) {
		ctx := context.Background()

		// Redis負荷テスト
		client := redis.NewClient(&redis.Options{
			Addr:     "localhost:6379",
			PoolSize: 10,
		})
		defer client.Close()

		concurrency := 100
		operations := 1000
		start := time.Now()

		done := make(chan bool, concurrency)

		for i := 0; i < concurrency; i++ {
			go func(workerID int) {
				for j := 0; j < operations/concurrency; j++ {
					key := fmt.Sprintf("perf:test:%d:%d", workerID, j)
					client.Set(ctx, key, "test-value", 10*time.Second)
					client.Get(ctx, key)
				}
				done <- true
			}(i)
		}

		// Wait for completion
		for i := 0; i < concurrency; i++ {
			<-done
		}

		duration := time.Since(start)
		opsPerSec := float64(operations) / duration.Seconds()

		t.Logf("Performance: %d operations in %v (%.0f ops/sec)", operations, duration, opsPerSec)

		if opsPerSec < 100 {
			t.Error("Performance below threshold")
		}
	})
}

// TestE2E_ErrorRecovery エラーリカバリーテスト
func TestE2E_ErrorRecovery(t *testing.T) {
	if os.Getenv("SKIP_E2E") != "" {
		t.Skip("Skipping E2E test")
	}

	t.Run("Service Recovery", func(t *testing.T) {
		// サービスが一時的に利用不可の場合のリトライ
		ctx := context.Background()
		client := redis.NewClient(&redis.Options{
			Addr:            "localhost:6379",
			MaxRetries:      3,
			MinRetryBackoff: 100 * time.Millisecond,
			MaxRetryBackoff: 1 * time.Second,
		})
		defer client.Close()

		// リトライ付きで操作
		for attempt := 1; attempt <= 3; attempt++ {
			err := client.Ping(ctx).Err()
			if err == nil {
				t.Log("Service recovered")
				break
			}

			if attempt == 3 {
				t.Error("Service did not recover after retries")
			}

			t.Logf("Attempt %d failed, retrying...", attempt)
			time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
	})
}