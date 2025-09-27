package practical

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/streadway/amqp"
)

// RabbitMQExample RabbitMQを使った実践的なメッセージング
type RabbitMQExample struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queues  map[string]amqp.Queue
}

// Order 注文データ
type Order struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ProductID string    `json:"product_id"`
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// NewRabbitMQExample RabbitMQの例を作成
func NewRabbitMQExample() (*RabbitMQExample, error) {
	// RabbitMQに接続
	conn, err := amqp.Dial("amqp://admin:admin@localhost:5672/")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	return &RabbitMQExample{
		conn:    conn,
		channel: channel,
		queues:  make(map[string]amqp.Queue),
	}, nil
}

// Close リソースをクリーンアップ
func (r *RabbitMQExample) Close() {
	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		r.conn.Close()
	}
}

// Example1_WorkQueue ワークキューパターン
func (r *RabbitMQExample) Example1_WorkQueue() error {
	fmt.Println("\n=== RabbitMQ Example 1: Work Queue Pattern ===")

	queueName := "task_queue"

	// キューを宣言（durable: 永続化）
	queue, err := r.channel.QueueDeclare(
		queueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return err
	}

	// QoS設定（一度に1つのメッセージのみ処理）
	err = r.channel.Qos(1, 0, false)
	if err != nil {
		return err
	}

	// Producer: タスクを送信
	go func() {
		for i := 0; i < 10; i++ {
			order := Order{
				ID:        fmt.Sprintf("ORDER-%03d", i),
				UserID:    fmt.Sprintf("USER-%d", i%3),
				ProductID: fmt.Sprintf("PROD-%d", i%5),
				Quantity:  i + 1,
				Price:     float64(i+1) * 99.99,
				Status:    "pending",
				CreatedAt: time.Now(),
			}

			body, _ := json.Marshal(order)

			err := r.channel.Publish(
				"",         // exchange
				queue.Name, // routing key
				false,      // mandatory
				false,      // immediate
				amqp.Publishing{
					DeliveryMode: amqp.Persistent, // メッセージを永続化
					ContentType:  "application/json",
					Body:         body,
				})

			if err == nil {
				fmt.Printf("📤 Sent order: %s\n", order.ID)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	// Consumer: 3つのワーカーで処理
	var wg sync.WaitGroup
	for w := 1; w <= 3; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			msgs, err := r.channel.Consume(
				queue.Name,
				fmt.Sprintf("worker-%d", workerID),
				false, // auto-ack
				false, // exclusive
				false, // no-local
				false, // no-wait
				nil,   // args
			)
			if err != nil {
				log.Printf("Worker %d failed to register consumer: %v", workerID, err)
				return
			}

			for msg := range msgs {
				var order Order
				json.Unmarshal(msg.Body, &order)

				// 処理をシミュレート
				processingTime := time.Duration(100+workerID*50) * time.Millisecond
				time.Sleep(processingTime)

				fmt.Printf("✅ Worker %d processed: %s (took %v)\n",
					workerID, order.ID, processingTime)

				// 手動ACK
				msg.Ack(false)
			}
		}(w)
	}

	// 2秒待って終了
	time.Sleep(2 * time.Second)
	return nil
}

// Example2_PublishSubscribe Pub/Subパターン
func (r *RabbitMQExample) Example2_PublishSubscribe() error {
	fmt.Println("\n=== RabbitMQ Example 2: Publish/Subscribe Pattern ===")

	// Fanout exchangeを宣言
	exchangeName := "logs"
	err := r.channel.ExchangeDeclare(
		exchangeName,
		"fanout", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		return err
	}

	// 複数のサブスクライバーを作成
	subscribers := []string{"logger", "monitor", "analytics"}

	for _, name := range subscribers {
		// 一時的なキューを作成
		q, err := r.channel.QueueDeclare(
			"",    // name (auto-generate)
			false, // durable
			false, // delete when unused
			true,  // exclusive
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			continue
		}

		// キューをexchangeにバインド
		err = r.channel.QueueBind(
			q.Name,       // queue name
			"",           // routing key
			exchangeName, // exchange
			false,
			nil,
		)
		if err != nil {
			continue
		}

		// Consumer
		go func(subName string, queueName string) {
			msgs, _ := r.channel.Consume(
				queueName,
				subName,
				true,  // auto-ack
				false, // exclusive
				false, // no-local
				false, // no-wait
				nil,   // args
			)

			for msg := range msgs {
				fmt.Printf("📨 %s received: %s\n", subName, string(msg.Body))
			}
		}(name, q.Name)
	}

	// Publisher: イベントを発行
	events := []string{
		"User logged in",
		"Order created",
		"Payment processed",
		"Item shipped",
	}

	for _, event := range events {
		err := r.channel.Publish(
			exchangeName, // exchange
			"",           // routing key (ignored for fanout)
			false,        // mandatory
			false,        // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(event),
			})

		if err == nil {
			fmt.Printf("📢 Published: %s\n", event)
		}
		time.Sleep(200 * time.Millisecond)
	}

	time.Sleep(1 * time.Second)
	return nil
}

// Example3_TopicRouting トピックルーティングパターン
func (r *RabbitMQExample) Example3_TopicRouting() error {
	fmt.Println("\n=== RabbitMQ Example 3: Topic Routing Pattern ===")

	exchangeName := "topic_logs"

	// Topic exchangeを宣言
	err := r.channel.ExchangeDeclare(
		exchangeName,
		"topic", // type
		true,    // durable
		false,   // auto-deleted
		false,   // internal
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		return err
	}

	// サブスクライバーと購読するトピック
	subscribers := map[string][]string{
		"error-handler":   {"*.error", "*.fatal"},
		"warning-monitor": {"*.warning", "*.error"},
		"info-logger":     {"*.info", "#"}, // #は全てにマッチ
		"auth-tracker":    {"auth.*"},
	}

	for name, patterns := range subscribers {
		q, _ := r.channel.QueueDeclare("", false, false, true, false, nil)

		for _, pattern := range patterns {
			r.channel.QueueBind(q.Name, pattern, exchangeName, false, nil)
		}

		go func(subName string, queueName string) {
			msgs, _ := r.channel.Consume(queueName, subName, true, false, false, false, nil)
			for msg := range msgs {
				fmt.Printf("🎯 %s [%s]: %s\n", subName, msg.RoutingKey, string(msg.Body))
			}
		}(name, q.Name)
	}

	// メッセージを発行
	messages := []struct {
		routingKey string
		body       string
	}{
		{"auth.info", "User login successful"},
		{"auth.error", "Invalid password"},
		{"payment.info", "Payment initiated"},
		{"payment.error", "Card declined"},
		{"system.warning", "High memory usage"},
		{"system.fatal", "Database connection lost"},
	}

	for _, msg := range messages {
		err := r.channel.Publish(
			exchangeName,
			msg.routingKey,
			false,
			false,
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(msg.body),
			})

		if err == nil {
			fmt.Printf("📮 Sent [%s]: %s\n", msg.routingKey, msg.body)
		}
		time.Sleep(100 * time.Millisecond)
	}

	time.Sleep(1 * time.Second)
	return nil
}

// Example4_RPC RPCパターン
func (r *RabbitMQExample) Example4_RPC() error {
	fmt.Println("\n=== RabbitMQ Example 4: RPC Pattern ===")

	// RPCサーバー
	serverQueue := "rpc_queue"
	q, _ := r.channel.QueueDeclare(serverQueue, false, false, false, false, nil)
	r.channel.Qos(1, 0, false)

	// サーバー: リクエストを処理
	go func() {
		msgs, _ := r.channel.Consume(q.Name, "", false, false, false, false, nil)

		for msg := range msgs {
			// リクエストを処理（フィボナッチ数列の計算）
			n := int(msg.Body[0])
			result := fibonacci(n)

			fmt.Printf("🔧 RPC Server: fib(%d) = %d\n", n, result)

			// レスポンスを送信
			r.channel.Publish(
				"",          // exchange
				msg.ReplyTo, // routing key (reply queue)
				false,       // mandatory
				false,       // immediate
				amqp.Publishing{
					ContentType:   "text/plain",
					CorrelationId: msg.CorrelationId,
					Body:          []byte(fmt.Sprintf("%d", result)),
				})

			msg.Ack(false)
		}
	}()

	// RPCクライアント
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// コールバックキューを作成
	replyQueue, _ := r.channel.QueueDeclare("", false, false, true, false, nil)
	msgs, _ := r.channel.Consume(replyQueue.Name, "", true, false, false, false, nil)

	corrID := generateCorrelationID()

	// リクエストを送信
	for n := 1; n <= 10; n++ {
		err := r.channel.Publish(
			"",
			serverQueue,
			false,
			false,
			amqp.Publishing{
				ContentType:   "text/plain",
				CorrelationId: corrID,
				ReplyTo:       replyQueue.Name,
				Body:          []byte{byte(n)},
			})

		if err == nil {
			fmt.Printf("📞 RPC Client: Calling fib(%d)\n", n)
		}

		// レスポンスを待つ
		select {
		case msg := <-msgs:
			if msg.CorrelationId == corrID {
				fmt.Printf("📥 RPC Client: Result = %s\n", string(msg.Body))
			}
		case <-ctx.Done():
			fmt.Println("RPC timeout")
		}

		time.Sleep(200 * time.Millisecond)
	}

	return nil
}

// ヘルパー関数
func fibonacci(n int) int {
	if n <= 1 {
		return n
	}
	return fibonacci(n-1) + fibonacci(n-2)
}

func generateCorrelationID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}