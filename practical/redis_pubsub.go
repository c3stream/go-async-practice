package practical

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisPubSubExample Redis Pub/Subパターンの実装
type RedisPubSubExample struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisPubSubExample Redis Pub/Subの例を作成
func NewRedisPubSubExample() *RedisPubSubExample {
	return &RedisPubSubExample{
		client: redis.NewClient(&redis.Options{
			Addr:     "localhost:6379",
			Password: "",
			DB:       0,
			PoolSize: 10,
		}),
		ctx: context.Background(),
	}
}

// Close リソースをクリーンアップ
func (r *RedisPubSubExample) Close() {
	if r.client != nil {
		r.client.Close()
	}
}

// Message Pub/Subメッセージ構造
type PubSubMessage struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Channel   string                 `json:"channel"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// Example1_BasicPubSub 基本的なPub/Subパターン
func (r *RedisPubSubExample) Example1_BasicPubSub() error {
	fmt.Println("\n=== Redis Example 1: Basic Pub/Sub ===")

	// チャネル定義
	channels := []string{"news", "sports", "weather"}

	// Subscriber goroutines
	var wg sync.WaitGroup
	for _, channel := range channels {
		wg.Add(1)
		go func(ch string) {
			defer wg.Done()
			r.subscriber(ch, func(msg PubSubMessage) {
				fmt.Printf("📨 [%s] Received: %s - %v\n", ch, msg.Type, msg.Data)
			})
		}(channel)
	}

	// Publisher
	go func() {
		time.Sleep(100 * time.Millisecond) // Subscriberの準備を待つ

		messages := []PubSubMessage{
			{
				ID:        "msg-1",
				Type:      "breaking",
				Channel:   "news",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"headline": "Breaking news!", "priority": "high"},
			},
			{
				ID:        "msg-2",
				Type:      "score",
				Channel:   "sports",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"team1": "TeamA", "team2": "TeamB", "score": "2-1"},
			},
			{
				ID:        "msg-3",
				Type:      "forecast",
				Channel:   "weather",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"temperature": 25, "condition": "sunny"},
			},
		}

		for _, msg := range messages {
			r.publish(msg.Channel, msg)
			time.Sleep(200 * time.Millisecond)
		}
	}()

	time.Sleep(2 * time.Second)
	return nil
}

// Example2_PatternSubscription パターンサブスクリプション
func (r *RedisPubSubExample) Example2_PatternSubscription() error {
	fmt.Println("\n=== Redis Example 2: Pattern Subscription ===")

	// パターンサブスクライバー
	patterns := map[string]string{
		"user:*":    "User events",
		"order:*":   "Order events",
		"payment:*": "Payment events",
	}

	var wg sync.WaitGroup
	for pattern, description := range patterns {
		wg.Add(1)
		go func(p, desc string) {
			defer wg.Done()
			r.patternSubscriber(p, func(channel string, msg PubSubMessage) {
				fmt.Printf("🎯 [%s] %s on channel %s: %v\n",
					desc, msg.Type, channel, msg.Data)
			})
		}(pattern, description)
	}

	// イベント発行
	go func() {
		time.Sleep(100 * time.Millisecond)

		events := []struct {
			channel string
			msg     PubSubMessage
		}{
			{
				channel: "user:login",
				msg: PubSubMessage{
					ID:   "evt-1",
					Type: "login",
					Data: map[string]interface{}{"user_id": "u123", "ip": "192.168.1.1"},
				},
			},
			{
				channel: "user:logout",
				msg: PubSubMessage{
					ID:   "evt-2",
					Type: "logout",
					Data: map[string]interface{}{"user_id": "u123", "session_duration": "45m"},
				},
			},
			{
				channel: "order:created",
				msg: PubSubMessage{
					ID:   "evt-3",
					Type: "created",
					Data: map[string]interface{}{"order_id": "o456", "amount": 299.99},
				},
			},
			{
				channel: "payment:processed",
				msg: PubSubMessage{
					ID:   "evt-4",
					Type: "processed",
					Data: map[string]interface{}{"payment_id": "p789", "status": "success"},
				},
			},
		}

		for _, event := range events {
			event.msg.Channel = event.channel
			event.msg.Timestamp = time.Now()
			r.publish(event.channel, event.msg)
			time.Sleep(300 * time.Millisecond)
		}
	}()

	time.Sleep(3 * time.Second)
	return nil
}

// Example3_RealtimeChatRoom リアルタイムチャットルーム
func (r *RedisPubSubExample) Example3_RealtimeChatRoom() error {
	fmt.Println("\n=== Redis Example 3: Realtime Chat Room ===")

	type ChatMessage struct {
		RoomID    string    `json:"room_id"`
		UserID    string    `json:"user_id"`
		Username  string    `json:"username"`
		Message   string    `json:"message"`
		Timestamp time.Time `json:"timestamp"`
		Type      string    `json:"type"` // message, join, leave
	}

	rooms := []string{"room:general", "room:tech", "room:random"}

	// チャットルームのサブスクライバー
	for _, room := range rooms {
		go func(roomID string) {
			pubsub := r.client.Subscribe(r.ctx, roomID)
			defer pubsub.Close()

			ch := pubsub.Channel()
			for msg := range ch {
				var chatMsg ChatMessage
				if err := json.Unmarshal([]byte(msg.Payload), &chatMsg); err == nil {
					switch chatMsg.Type {
					case "join":
						fmt.Printf("👋 [%s] %s joined the room\n", roomID, chatMsg.Username)
					case "leave":
						fmt.Printf("👋 [%s] %s left the room\n", roomID, chatMsg.Username)
					case "message":
						fmt.Printf("💬 [%s] %s: %s\n", roomID, chatMsg.Username, chatMsg.Message)
					}
				}
			}
		}(room)
	}

	time.Sleep(100 * time.Millisecond) // サブスクライバー準備待ち

	// ユーザーアクティビティシミュレート
	users := []struct {
		id       string
		username string
	}{
		{"u1", "Alice"},
		{"u2", "Bob"},
		{"u3", "Charlie"},
	}

	// ユーザーがルームに参加
	for _, user := range users {
		for i, room := range rooms {
			if i == 0 || (i == 1 && user.id != "u3") { // 一部のルームに参加
				joinMsg := ChatMessage{
					RoomID:    room,
					UserID:    user.id,
					Username:  user.username,
					Type:      "join",
					Timestamp: time.Now(),
				}
				r.publishChat(room, joinMsg)
				time.Sleep(100 * time.Millisecond)
			}
		}
	}

	// メッセージ送信
	messages := []struct {
		userIdx int
		roomIdx int
		message string
	}{
		{0, 0, "Hello everyone!"},
		{1, 0, "Hi Alice! How are you?"},
		{0, 0, "I'm good, thanks! Working on Redis Pub/Sub"},
		{1, 1, "Anyone here interested in Go?"},
		{2, 0, "Hey folks, just joined!"},
	}

	for _, msg := range messages {
		if msg.userIdx < len(users) && msg.roomIdx < len(rooms) {
			chatMsg := ChatMessage{
				RoomID:    rooms[msg.roomIdx],
				UserID:    users[msg.userIdx].id,
				Username:  users[msg.userIdx].username,
				Message:   msg.message,
				Type:      "message",
				Timestamp: time.Now(),
			}
			r.publishChat(rooms[msg.roomIdx], chatMsg)
			time.Sleep(500 * time.Millisecond)
		}
	}

	return nil
}

// Example4_EventBroadcast イベントブロードキャスト
func (r *RedisPubSubExample) Example4_EventBroadcast() error {
	fmt.Println("\n=== Redis Example 4: Event Broadcast System ===")

	// システムイベント
	type SystemEvent struct {
		ID       string                 `json:"id"`
		Type     string                 `json:"type"`
		Severity string                 `json:"severity"` // info, warning, error, critical
		Source   string                 `json:"source"`
		Message  string                 `json:"message"`
		Metadata map[string]interface{} `json:"metadata"`
		Time     time.Time              `json:"time"`
	}

	// イベントハンドラー登録
	handlers := map[string]func(SystemEvent){
		"info": func(e SystemEvent) {
			fmt.Printf("ℹ️  [INFO] %s: %s\n", e.Source, e.Message)
		},
		"warning": func(e SystemEvent) {
			fmt.Printf("⚠️  [WARN] %s: %s\n", e.Source, e.Message)
		},
		"error": func(e SystemEvent) {
			fmt.Printf("❌ [ERROR] %s: %s (metadata: %v)\n",
				e.Source, e.Message, e.Metadata)
		},
		"critical": func(e SystemEvent) {
			fmt.Printf("🚨 [CRITICAL] %s: %s - IMMEDIATE ACTION REQUIRED!\n",
				e.Source, e.Message)
		},
	}

	// ブロードキャストチャネルのサブスクライバー
	go func() {
		pubsub := r.client.Subscribe(r.ctx, "system:events")
		defer pubsub.Close()

		ch := pubsub.Channel()
		for msg := range ch {
			var event SystemEvent
			if err := json.Unmarshal([]byte(msg.Payload), &event); err == nil {
				if handler, exists := handlers[event.Severity]; exists {
					handler(event)
				}
			}
		}
	}()

	time.Sleep(100 * time.Millisecond)

	// システムイベント生成
	events := []SystemEvent{
		{
			ID:       "evt-001",
			Type:     "startup",
			Severity: "info",
			Source:   "api-server",
			Message:  "API server started successfully",
			Time:     time.Now(),
		},
		{
			ID:       "evt-002",
			Type:     "performance",
			Severity: "warning",
			Source:   "database",
			Message:  "Query response time exceeding threshold",
			Metadata: map[string]interface{}{"query_time_ms": 1500, "threshold_ms": 1000},
			Time:     time.Now(),
		},
		{
			ID:       "evt-003",
			Type:     "connection",
			Severity: "error",
			Source:   "cache",
			Message:  "Failed to connect to Redis replica",
			Metadata: map[string]interface{}{"host": "replica-2", "attempts": 3},
			Time:     time.Now(),
		},
		{
			ID:       "evt-004",
			Type:     "security",
			Severity: "critical",
			Source:   "auth-service",
			Message:  "Multiple failed authentication attempts detected",
			Metadata: map[string]interface{}{"ip": "192.168.1.100", "attempts": 50},
			Time:     time.Now(),
		},
		{
			ID:       "evt-005",
			Type:     "recovery",
			Severity: "info",
			Source:   "cache",
			Message:  "Connection to Redis replica restored",
			Time:     time.Now(),
		},
	}

	// イベントブロードキャスト
	for _, event := range events {
		data, _ := json.Marshal(event)
		r.client.Publish(r.ctx, "system:events", data)
		time.Sleep(400 * time.Millisecond)
	}

	return nil
}

// Example5_DistributedQueue 分散キューパターン
func (r *RedisPubSubExample) Example5_DistributedQueue() error {
	fmt.Println("\n=== Redis Example 5: Distributed Queue Pattern ===")

	type Job struct {
		ID         string                 `json:"id"`
		Type       string                 `json:"type"`
		Priority   int                    `json:"priority"`
		Payload    map[string]interface{} `json:"payload"`
		CreatedAt  time.Time              `json:"created_at"`
		ProcessedBy string                `json:"processed_by,omitempty"`
		ProcessedAt *time.Time            `json:"processed_at,omitempty"`
		Result     interface{}            `json:"result,omitempty"`
	}

	// ワーカープール
	const numWorkers = 3
	jobQueue := "job:queue"
	resultChannel := "job:results"

	// 結果サブスクライバー
	go func() {
		pubsub := r.client.Subscribe(r.ctx, resultChannel)
		defer pubsub.Close()

		ch := pubsub.Channel()
		for msg := range ch {
			var job Job
			if err := json.Unmarshal([]byte(msg.Payload), &job); err == nil {
				fmt.Printf("✅ Job %s completed by %s: %v\n",
					job.ID, job.ProcessedBy, job.Result)
			}
		}
	}()

	// ワーカー
	for i := 0; i < numWorkers; i++ {
		workerID := fmt.Sprintf("worker-%d", i)
		go func(id string) {
			for {
				// ジョブ取得（BLPOP - ブロッキングポップ）
				result, err := r.client.BLPop(r.ctx, 2*time.Second, jobQueue).Result()
				if err != nil || len(result) < 2 {
					continue
				}

				var job Job
				if err := json.Unmarshal([]byte(result[1]), &job); err != nil {
					continue
				}

				// ジョブ処理
				fmt.Printf("🔧 %s processing job %s (priority: %d)\n",
					id, job.ID, job.Priority)

				time.Sleep(500 * time.Millisecond) // 処理シミュレート

				// 結果を設定
				now := time.Now()
				job.ProcessedBy = id
				job.ProcessedAt = &now
				job.Result = fmt.Sprintf("Processed %s", job.Type)

				// 結果を発行
				data, _ := json.Marshal(job)
				r.client.Publish(r.ctx, resultChannel, data)
			}
		}(workerID)
	}

	time.Sleep(100 * time.Millisecond)

	// ジョブ生成
	jobs := []Job{
		{
			ID:       "job-001",
			Type:     "email",
			Priority: 1,
			Payload:  map[string]interface{}{"to": "user@example.com", "subject": "Welcome"},
		},
		{
			ID:       "job-002",
			Type:     "image-resize",
			Priority: 2,
			Payload:  map[string]interface{}{"url": "image.jpg", "size": "thumbnail"},
		},
		{
			ID:       "job-003",
			Type:     "report",
			Priority: 3,
			Payload:  map[string]interface{}{"type": "monthly", "format": "pdf"},
		},
		{
			ID:       "job-004",
			Type:     "notification",
			Priority: 1,
			Payload:  map[string]interface{}{"user_id": "u123", "message": "Update available"},
		},
		{
			ID:       "job-005",
			Type:     "backup",
			Priority: 2,
			Payload:  map[string]interface{}{"database": "users", "incremental": true},
		},
	}

	// ジョブをキューに追加
	for _, job := range jobs {
		job.CreatedAt = time.Now()
		data, _ := json.Marshal(job)

		// 優先度に基づいて追加（優先度が高いほど先頭に）
		if job.Priority == 1 {
			r.client.LPush(r.ctx, jobQueue, data)
		} else {
			r.client.RPush(r.ctx, jobQueue, data)
		}

		fmt.Printf("📥 Job %s added to queue (priority: %d)\n", job.ID, job.Priority)
		time.Sleep(200 * time.Millisecond)
	}

	time.Sleep(3 * time.Second)
	return nil
}

// Helper: メッセージ発行
func (r *RedisPubSubExample) publish(channel string, msg PubSubMessage) {
	data, _ := json.Marshal(msg)
	r.client.Publish(r.ctx, channel, data)
}

// Helper: チャットメッセージ発行
func (r *RedisPubSubExample) publishChat(room string, msg interface{}) {
	data, _ := json.Marshal(msg)
	r.client.Publish(r.ctx, room, data)
}

// Helper: サブスクライバー
func (r *RedisPubSubExample) subscriber(channel string, handler func(PubSubMessage)) {
	pubsub := r.client.Subscribe(r.ctx, channel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	timeout := time.After(2 * time.Second)

	for {
		select {
		case msg := <-ch:
			var pubsubMsg PubSubMessage
			if err := json.Unmarshal([]byte(msg.Payload), &pubsubMsg); err == nil {
				handler(pubsubMsg)
			}
		case <-timeout:
			return
		}
	}
}

// Helper: パターンサブスクライバー
func (r *RedisPubSubExample) patternSubscriber(pattern string, handler func(string, PubSubMessage)) {
	pubsub := r.client.PSubscribe(r.ctx, pattern)
	defer pubsub.Close()

	ch := pubsub.Channel()
	timeout := time.After(3 * time.Second)

	for {
		select {
		case msg := <-ch:
			var pubsubMsg PubSubMessage
			if err := json.Unmarshal([]byte(msg.Payload), &pubsubMsg); err == nil {
				handler(msg.Channel, pubsubMsg)
			}
		case <-timeout:
			return
		}
	}
}

// RunRedisPubSubExamples 全例を実行
func RunRedisPubSubExamples() {
	example := NewRedisPubSubExample()
	defer example.Close()

	// Ping test
	if err := example.client.Ping(example.ctx).Err(); err != nil {
		fmt.Printf("❌ Redis connection failed: %v\n", err)
		fmt.Println("Please ensure Redis is running on localhost:6379")
		return
	}

	fmt.Println("🚀 Redis Pub/Sub Examples")
	fmt.Println("=" + repeatString("=", 50))

	// 基本的なPub/Sub
	if err := example.Example1_BasicPubSub(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	time.Sleep(1 * time.Second)

	// パターンサブスクリプション
	if err := example.Example2_PatternSubscription(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	time.Sleep(1 * time.Second)

	// リアルタイムチャット
	if err := example.Example3_RealtimeChatRoom(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	time.Sleep(1 * time.Second)

	// イベントブロードキャスト
	if err := example.Example4_EventBroadcast(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	time.Sleep(1 * time.Second)

	// 分散キュー
	if err := example.Example5_DistributedQueue(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	fmt.Println("\n✅ All Redis Pub/Sub examples completed!")
}