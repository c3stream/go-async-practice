package practical

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"
)

// KafkaExample Kafkaを使ったイベントストリーミング
type KafkaExample struct {
	brokers []string
	writers map[string]*kafka.Writer
	readers map[string]*kafka.Reader
}

// KafkaEvent イベントデータ
type KafkaEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// NewKafkaExample Kafkaの例を作成
func NewKafkaExample() *KafkaExample {
	return &KafkaExample{
		brokers: []string{"localhost:9092"},
		writers: make(map[string]*kafka.Writer),
		readers: make(map[string]*kafka.Reader),
	}
}

// Close リソースをクリーンアップ
func (k *KafkaExample) Close() {
	for _, writer := range k.writers {
		writer.Close()
	}
	for _, reader := range k.readers {
		reader.Close()
	}
}

// Example1_EventStreaming イベントストリーミング
func (k *KafkaExample) Example1_EventStreaming() error {
	fmt.Println("\n=== Kafka Example 1: Event Streaming ===")

	topic := "user-events"

	// Producer設定
	writer := &kafka.Writer{
		Addr:     kafka.TCP(k.brokers...),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	// イベント生成
	eventTypes := []string{"user.created", "user.updated", "user.login", "user.logout"}

	// Producer goroutine
	go func() {
		for i := 0; i < 20; i++ {
			event := KafkaEvent{
				ID:        fmt.Sprintf("evt-%d", i),
				Type:      eventTypes[i%len(eventTypes)],
				Source:    "user-service",
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"user_id": fmt.Sprintf("user-%d", i%5),
					"action":  eventTypes[i%len(eventTypes)],
				},
			}

			data, _ := json.Marshal(event)

			err := writer.WriteMessages(context.Background(),
				kafka.Message{
					Key:   []byte(event.ID),
					Value: data,
				})

			if err == nil {
				fmt.Printf("📤 Published event: %s [%s]\n", event.ID, event.Type)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Consumer Groups
	var wg sync.WaitGroup

	// Consumer Group 1: ログ処理
	wg.Add(1)
	go func() {
		defer wg.Done()
		k.consumeEvents("logging-service", topic, func(event KafkaEvent) {
			fmt.Printf("📝 Logging: %s - %s\n", event.Type, event.ID)
		})
	}()

	// Consumer Group 2: 分析処理
	wg.Add(1)
	go func() {
		defer wg.Done()
		k.consumeEvents("analytics-service", topic, func(event KafkaEvent) {
			fmt.Printf("📊 Analytics: Processing %s event\n", event.Type)
		})
	}()

	time.Sleep(3 * time.Second)
	return nil
}

// Example2_EventSourcing イベントソーシング
func (k *KafkaExample) Example2_EventSourcing() error {
	fmt.Println("\n=== Kafka Example 2: Event Sourcing ===")

	topic := "order-events"

	// Writer作成
	writer := &kafka.Writer{
		Addr:     kafka.TCP(k.brokers...),
		Topic:    topic,
		Balancer: &kafka.Hash{},
	}
	defer writer.Close()

	// 注文イベントシーケンス
	orderID := "ORDER-001"
	events := []KafkaEvent{
		{
			ID:        "evt-1",
			Type:      "order.created",
			Source:    "order-service",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"order_id": orderID,
				"items":    []string{"item-1", "item-2"},
				"total":    299.99,
			},
		},
		{
			ID:        "evt-2",
			Type:      "payment.initiated",
			Source:    "payment-service",
			Timestamp: time.Now().Add(1 * time.Second),
			Data: map[string]interface{}{
				"order_id": orderID,
				"method":   "credit_card",
			},
		},
		{
			ID:        "evt-3",
			Type:      "payment.completed",
			Source:    "payment-service",
			Timestamp: time.Now().Add(2 * time.Second),
			Data: map[string]interface{}{
				"order_id":       orderID,
				"transaction_id": "TXN-123",
			},
		},
		{
			ID:        "evt-4",
			Type:      "order.shipped",
			Source:    "shipping-service",
			Timestamp: time.Now().Add(3 * time.Second),
			Data: map[string]interface{}{
				"order_id":    orderID,
				"tracking_no": "TRACK-456",
			},
		},
	}

	// イベント発行
	for _, event := range events {
		data, _ := json.Marshal(event)

		err := writer.WriteMessages(context.Background(),
			kafka.Message{
				Key:   []byte(orderID), // 同じキーで順序保証
				Value: data,
			})

		if err == nil {
			fmt.Printf("📤 Event: %s -> %s\n", event.Type, orderID)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// イベントリプレイ（状態再構築）
	fmt.Println("\n🔄 Replaying events to rebuild state...")

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  k.brokers,
		Topic:    topic,
		GroupID:  "replay-consumer",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer reader.Close()

	orderState := make(map[string]interface{})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			break
		}

		var event KafkaEvent
		json.Unmarshal(msg.Value, &event)

		// 状態を更新
		switch event.Type {
		case "order.created":
			orderState["status"] = "created"
			orderState["items"] = event.Data["items"]
			orderState["total"] = event.Data["total"]
		case "payment.completed":
			orderState["status"] = "paid"
			orderState["transaction_id"] = event.Data["transaction_id"]
		case "order.shipped":
			orderState["status"] = "shipped"
			orderState["tracking_no"] = event.Data["tracking_no"]
		}

		fmt.Printf("📖 State after %s: %+v\n", event.Type, orderState)
	}

	return nil
}

// Example3_StreamProcessing ストリーム処理
func (k *KafkaExample) Example3_StreamProcessing() error {
	fmt.Println("\n=== Kafka Example 3: Stream Processing ===")

	inputTopic := "raw-metrics"
	outputTopic := "processed-metrics"

	// Input writer
	inputWriter := &kafka.Writer{
		Addr:     kafka.TCP(k.brokers...),
		Topic:    inputTopic,
		Balancer: &kafka.RoundRobin{},
	}
	defer inputWriter.Close()

	// Output writer
	outputWriter := &kafka.Writer{
		Addr:     kafka.TCP(k.brokers...),
		Topic:    outputTopic,
		Balancer: &kafka.RoundRobin{},
	}
	defer outputWriter.Close()

	// メトリクス生成
	go func() {
		for i := 0; i < 15; i++ {
			metric := map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"server":    fmt.Sprintf("server-%d", i%3),
				"cpu_usage": 20 + (i * 5 % 60),
				"memory":    30 + (i * 3 % 40),
			}

			data, _ := json.Marshal(metric)
			inputWriter.WriteMessages(context.Background(),
				kafka.Message{Value: data})

			fmt.Printf("📊 Raw metric: server-%d, CPU: %v%%\n",
				i%3, metric["cpu_usage"])
			time.Sleep(200 * time.Millisecond)
		}
	}()

	// Stream processor: 5秒間のウィンドウで集計
	go func() {
		reader := kafka.NewReader(kafka.ReaderConfig{
			Brokers:  k.brokers,
			Topic:    inputTopic,
			GroupID:  "stream-processor",
			MinBytes: 1,
			MaxBytes: 10e6,
		})
		defer reader.Close()

		window := make([]map[string]interface{}, 0)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if len(window) > 0 {
					// ウィンドウ内のデータを集計
					avgCPU := 0.0
					avgMem := 0.0
					for _, m := range window {
						avgCPU += m["cpu_usage"].(float64)
						avgMem += m["memory"].(float64)
					}
					avgCPU /= float64(len(window))
					avgMem /= float64(len(window))

					aggregated := map[string]interface{}{
						"timestamp":   time.Now().Unix(),
						"window_size": len(window),
						"avg_cpu":     avgCPU,
						"avg_memory":  avgMem,
						"alert":       avgCPU > 50,
					}

					data, _ := json.Marshal(aggregated)
					outputWriter.WriteMessages(context.Background(),
						kafka.Message{Value: data})

					fmt.Printf("⚡ Processed: Window size=%d, Avg CPU=%.1f%%, Alert=%v\n",
						len(window), avgCPU, avgCPU > 50)

					window = window[:0] // Clear window
				}

			default:
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				msg, err := reader.ReadMessage(ctx)
				cancel()

				if err == nil {
					var metric map[string]interface{}
					json.Unmarshal(msg.Value, &metric)
					window = append(window, metric)
				}
			}
		}
	}()

	time.Sleep(5 * time.Second)
	return nil
}

// Helper: イベント消費
func (k *KafkaExample) consumeEvents(groupID, topic string, handler func(KafkaEvent)) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     k.brokers,
		Topic:       topic,
		GroupID:     groupID,
		MinBytes:    1,
		MaxBytes:    10e6,
		StartOffset: kafka.FirstOffset,
	})
	defer reader.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	for {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			break
		}

		var event KafkaEvent
		if err := json.Unmarshal(msg.Value, &event); err == nil {
			handler(event)
		}
	}
}