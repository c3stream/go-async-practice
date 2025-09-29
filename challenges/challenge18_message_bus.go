package challenges

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Challenge 18: Message Bus with Topic-based Routing
//
// 問題点:
// 1. メッセージルーティングの不整合
// 2. パーティショニングのバグ
// 3. Consumer Groupの実装不備
// 4. At-least-once/At-most-once配信の保証なし
// 5. バックプレッシャー処理の欠如

type Message struct {
	ID          string
	Topic       string
	Partition   int
	Key         []byte
	Value       []byte
	Headers     map[string]string
	Timestamp   time.Time
	Offset      int64
	Retries     int
	MaxRetries  int
}

type ConsumerGroup struct {
	ID          string
	Consumers   []Consumer
	Partitions  map[int]string // partition -> consumer ID mapping
	mu          sync.RWMutex
}

type Consumer struct {
	ID       string
	GroupID  string
	Topics   []string
	Handler  func(msg Message) error
	Offset   map[string]int64 // topic -> offset
	Active   bool
	LastSeen time.Time
}

type MessageBus struct {
	mu           sync.RWMutex
	topics       map[string]*Topic
	groups       map[string]*ConsumerGroup
	messages     map[string][]Message // 問題: メモリリーク
	processing   sync.Map
	routingRules map[string]func(Message) int // topic -> partition function
}

type Topic struct {
	Name       string
	Partitions int
	Messages   [][]Message // partition -> messages
	Offsets    []int64
	mu         sync.RWMutex
}

func NewMessageBus() *MessageBus {
	return &MessageBus{
		topics:       make(map[string]*Topic),
		groups:       make(map[string]*ConsumerGroup),
		messages:     make(map[string][]Message),
		routingRules: make(map[string]func(Message) int),
	}
}

// 問題1: トピック作成の競合状態
func (mb *MessageBus) CreateTopic(name string, partitions int) error {
	// 問題: Read-Modify-Write の競合状態
	if _, exists := mb.topics[name]; exists {
		return fmt.Errorf("topic %s already exists", name)
	}

	// 問題: ロックなしで書き込み
	mb.topics[name] = &Topic{
		Name:       name,
		Partitions: partitions,
		Messages:   make([][]Message, partitions),
		Offsets:    make([]int64, partitions),
	}

	return nil
}

// 問題2: メッセージ送信のバグ
func (mb *MessageBus) Send(ctx context.Context, msg Message) error {
	mb.mu.RLock()
	topic, exists := mb.topics[msg.Topic]
	mb.mu.RUnlock()

	if !exists {
		return fmt.Errorf("topic %s not found", msg.Topic)
	}

	// 問題: パーティション選択ロジックのバグ
	partition := 0
	if rule, ok := mb.routingRules[msg.Topic]; ok {
		partition = rule(msg)
	} else if msg.Key != nil {
		// 問題: 負の値になる可能性
		hash := 0
		for _, b := range msg.Key {
			hash += int(b)
		}
		partition = hash % topic.Partitions
	}

	// 問題: 境界チェックなし
	topic.mu.Lock()
	topic.Messages[partition] = append(topic.Messages[partition], msg)
	topic.Offsets[partition]++
	topic.mu.Unlock()

	// 問題: Consumer通知の欠如
	// コンシューマーへの通知メカニズムがない

	return nil
}

// 問題3: Consumer Group管理
func (mb *MessageBus) JoinConsumerGroup(groupID string, consumer Consumer) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	group, exists := mb.groups[groupID]
	if !exists {
		group = &ConsumerGroup{
			ID:         groupID,
			Consumers:  []Consumer{},
			Partitions: make(map[int]string),
		}
		mb.groups[groupID] = group
	}

	// 問題: 重複チェックなし
	group.Consumers = append(group.Consumers, consumer)

	// 問題: パーティション再分配なし
	// 新しいコンシューマーが参加してもパーティションが再分配されない

	return nil
}

// 問題4: メッセージ消費
func (mb *MessageBus) Consume(ctx context.Context, groupID string, topic string) (<-chan Message, error) {
	mb.mu.RLock()
	_, exists := mb.groups[groupID]
	mb.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("consumer group %s not found", groupID)
	}

	msgChan := make(chan Message) // 問題: バッファなし

	// 問題: goroutineリーク
	go func() {
		for {
			select {
			case <-ctx.Done():
				// 問題: チャネルをクローズしない
				return
			default:
				mb.mu.RLock()
				t, exists := mb.topics[topic]
				mb.mu.RUnlock()

				if !exists {
					continue
				}

				// 問題: 全パーティションを消費（非効率）
				for partition := 0; partition < t.Partitions; partition++ {
					t.mu.RLock()
					messages := t.Messages[partition]
					t.mu.RUnlock()

					// 問題: オフセット管理なし
					for _, msg := range messages {
						// 問題: ブロッキング送信
						msgChan <- msg
					}
				}

				// 問題: ビジーウェイト
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	return msgChan, nil
}

// 問題5: リバランシング
func (mb *MessageBus) Rebalance(groupID string) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	group, exists := mb.groups[groupID]
	if !exists {
		return fmt.Errorf("group %s not found", groupID)
	}

	// 問題: 単純すぎるリバランシングロジック
	activeConsumers := 0
	for _, c := range group.Consumers {
		if c.Active {
			activeConsumers++
		}
	}

	if activeConsumers == 0 {
		return fmt.Errorf("no active consumers")
	}

	// 問題: パーティション割り当ての不均衡
	// すべてのトピックで同じパーティション数を仮定

	return nil
}

// Challenge: 以下の問題を修正してください
// 1. スレッドセーフなトピック管理
// 2. 正しいパーティショニング
// 3. Consumer Groupのリバランシング
// 4. オフセット管理
// 5. At-least-once配信保証
// 6. デッドレターキュー
// 7. メトリクスとモニタリング
// 8. グレースフルシャットダウン

func RunChallenge18() {
	fmt.Println("Challenge 18: Message Bus with Routing")
	fmt.Println("Fix the message bus implementation")

	mb := NewMessageBus()
	ctx := context.Background()

	// トピック作成
	mb.CreateTopic("orders", 3)

	// Consumer Group作成
	consumer := Consumer{
		ID:      "consumer-1",
		GroupID: "order-processors",
		Topics:  []string{"orders"},
		Handler: func(msg Message) error {
			fmt.Printf("Processing: %s\n", string(msg.Value))
			return nil
		},
	}

	mb.JoinConsumerGroup("order-processors", consumer)

	// メッセージ送信
	for i := 0; i < 10; i++ {
		msg := Message{
			ID:        fmt.Sprintf("msg-%d", i),
			Topic:     "orders",
			Key:       []byte(fmt.Sprintf("order-%d", i%3)),
			Value:     []byte(fmt.Sprintf("Order data %d", i)),
			Timestamp: time.Now(),
		}

		if err := mb.Send(ctx, msg); err != nil {
			fmt.Printf("Send error: %v\n", err)
		}
	}

	// メッセージ消費
	messages, _ := mb.Consume(ctx, "order-processors", "orders")

	go func() {
		for msg := range messages {
			consumer.Handler(msg)
		}
	}()

	time.Sleep(2 * time.Second)
}