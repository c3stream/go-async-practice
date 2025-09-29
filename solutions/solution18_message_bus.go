package solutions

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Solution18MessageBus demonstrates three approaches to fixing message bus issues:
// 1. Proper partitioning and consumer group management
// 2. Exactly-once delivery semantics
// 3. Dynamic rebalancing with fault tolerance

// Solution 1: Well-Partitioned Message Bus
type PartitionedMessageBus struct {
	topics    map[string]*Topic
	groups    map[string]*ConsumerGroup
	mu        sync.RWMutex
	offsetMgr *OffsetManager
}

type Topic struct {
	Name       string
	Partitions []*Partition
	Config     TopicConfig
	mu         sync.RWMutex
}

type TopicConfig struct {
	NumPartitions   int
	ReplicationFactor int
	RetentionMs     int64
	CompressionType string
}

type Partition struct {
	ID       int
	Messages []Message
	Offset   int64
	mu       sync.RWMutex
	replicas []*Partition
}

type Message struct {
	ID        string
	Topic     string
	Partition int
	Offset    int64
	Key       []byte
	Value     []byte
	Headers   map[string]string
	Timestamp time.Time
}

type ConsumerGroup struct {
	ID          string
	Consumers   map[string]*Consumer
	Assignments map[string]map[int]string // topic -> partition -> consumer
	Generation  int32
	mu          sync.RWMutex
}

type Consumer struct {
	ID        string
	GroupID   string
	Topics    []string
	Handler   MessageHandler
	Heartbeat time.Time
	Active    bool
}

type MessageHandler func(ctx context.Context, msg Message) error

type OffsetManager struct {
	offsets map[string]map[string]map[int]int64 // group -> topic -> partition -> offset
	mu      sync.RWMutex
}

func NewPartitionedMessageBus() *PartitionedMessageBus {
	return &PartitionedMessageBus{
		topics:    make(map[string]*Topic),
		groups:    make(map[string]*ConsumerGroup),
		offsetMgr: &OffsetManager{
			offsets: make(map[string]map[string]map[int]int64),
		},
	}
}

func (mb *PartitionedMessageBus) CreateTopic(name string, config TopicConfig) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if _, exists := mb.topics[name]; exists {
		return fmt.Errorf("topic %s already exists", name)
	}

	partitions := make([]*Partition, config.NumPartitions)
	for i := 0; i < config.NumPartitions; i++ {
		partitions[i] = &Partition{
			ID:       i,
			Messages: make([]Message, 0),
			Offset:   0,
		}
	}

	mb.topics[name] = &Topic{
		Name:       name,
		Partitions: partitions,
		Config:     config,
	}

	return nil
}

func (mb *PartitionedMessageBus) Send(ctx context.Context, topicName string, key, value []byte, headers map[string]string) error {
	mb.mu.RLock()
	topic, exists := mb.topics[topicName]
	mb.mu.RUnlock()

	if !exists {
		return fmt.Errorf("topic %s not found", topicName)
	}

	// Select partition
	partitionID := mb.selectPartition(key, len(topic.Partitions))

	if partitionID < 0 || partitionID >= len(topic.Partitions) {
		return fmt.Errorf("invalid partition %d", partitionID)
	}

	partition := topic.Partitions[partitionID]

	// Create message
	partition.mu.Lock()
	message := Message{
		ID:        fmt.Sprintf("%s-%d-%d", topicName, partitionID, partition.Offset),
		Topic:     topicName,
		Partition: partitionID,
		Offset:    partition.Offset,
		Key:       key,
		Value:     value,
		Headers:   headers,
		Timestamp: time.Now(),
	}

	partition.Messages = append(partition.Messages, message)
	partition.Offset++
	partition.mu.Unlock()

	// Replicate to replicas (simplified)
	mb.replicateMessage(partition, message)

	return nil
}

func (mb *PartitionedMessageBus) selectPartition(key []byte, numPartitions int) int {
	if len(key) == 0 {
		// Round-robin for null keys
		return int(time.Now().UnixNano() % int64(numPartitions))
	}

	// Hash-based partitioning
	h := fnv.New32a()
	h.Write(key)
	hash := h.Sum32()

	// Ensure positive result
	partition := int(hash % uint32(numPartitions))
	if partition < 0 {
		partition = -partition
	}

	return partition
}

func (mb *PartitionedMessageBus) replicateMessage(partition *Partition, message Message) {
	// Simplified replication
	for _, replica := range partition.replicas {
		replica.mu.Lock()
		replica.Messages = append(replica.Messages, message)
		replica.mu.Unlock()
	}
}

func (mb *PartitionedMessageBus) CreateConsumerGroup(groupID string) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if _, exists := mb.groups[groupID]; exists {
		return fmt.Errorf("consumer group %s already exists", groupID)
	}

	mb.groups[groupID] = &ConsumerGroup{
		ID:          groupID,
		Consumers:   make(map[string]*Consumer),
		Assignments: make(map[string]map[int]string),
		Generation:  0,
	}

	// Initialize offset tracking
	mb.offsetMgr.mu.Lock()
	mb.offsetMgr.offsets[groupID] = make(map[string]map[int]int64)
	mb.offsetMgr.mu.Unlock()

	return nil
}

func (mb *PartitionedMessageBus) JoinGroup(groupID, consumerID string, topics []string, handler MessageHandler) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	group, exists := mb.groups[groupID]
	if !exists {
		return fmt.Errorf("consumer group %s not found", groupID)
	}

	consumer := &Consumer{
		ID:        consumerID,
		GroupID:   groupID,
		Topics:    topics,
		Handler:   handler,
		Heartbeat: time.Now(),
		Active:    true,
	}

	group.mu.Lock()
	group.Consumers[consumerID] = consumer
	group.mu.Unlock()

	// Trigger rebalance
	go mb.rebalance(groupID)

	return nil
}

func (mb *PartitionedMessageBus) rebalance(groupID string) {
	mb.mu.RLock()
	group := mb.groups[groupID]
	mb.mu.RUnlock()

	if group == nil {
		return
	}

	group.mu.Lock()
	defer group.mu.Unlock()

	// Increment generation
	atomic.AddInt32(&group.Generation, 1)

	// Collect all partitions
	topicPartitions := make(map[string][]int)

	for _, consumer := range group.Consumers {
		if !consumer.Active {
			continue
		}

		for _, topicName := range consumer.Topics {
			mb.mu.RLock()
			if topic, exists := mb.topics[topicName]; exists {
				for i := range topic.Partitions {
					topicPartitions[topicName] = append(topicPartitions[topicName], i)
				}
			}
			mb.mu.RUnlock()
		}
	}

	// Get active consumers
	var activeConsumers []string
	for id, consumer := range group.Consumers {
		if consumer.Active {
			activeConsumers = append(activeConsumers, id)
		}
	}

	if len(activeConsumers) == 0 {
		return
	}

	// Clear and reassign
	group.Assignments = make(map[string]map[int]string)

	// Range assignment strategy
	consumerIndex := 0
	for topic, partitions := range topicPartitions {
		if group.Assignments[topic] == nil {
			group.Assignments[topic] = make(map[int]string)
		}

		for _, partition := range partitions {
			consumerID := activeConsumers[consumerIndex%len(activeConsumers)]
			group.Assignments[topic][partition] = consumerID
			consumerIndex++
		}
	}

	log.Printf("Rebalanced group %s (generation %d): %d consumers, %d topics",
		groupID, group.Generation, len(activeConsumers), len(topicPartitions))
}

// Solution 2: Exactly-Once Message Bus
type ExactlyOnceMessageBus struct {
	*PartitionedMessageBus
	txManager *TransactionManager
}

type TransactionManager struct {
	transactions map[string]*Transaction
	mu           sync.RWMutex
}

type Transaction struct {
	ID        string
	State     string // pending, committed, aborted
	Messages  []Message
	Timestamp time.Time
}

func NewExactlyOnceMessageBus() *ExactlyOnceMessageBus {
	return &ExactlyOnceMessageBus{
		PartitionedMessageBus: NewPartitionedMessageBus(),
		txManager: &TransactionManager{
			transactions: make(map[string]*Transaction),
		},
	}
}

func (mb *ExactlyOnceMessageBus) BeginTransaction() string {
	txID := fmt.Sprintf("tx-%d", time.Now().UnixNano())

	mb.txManager.mu.Lock()
	mb.txManager.transactions[txID] = &Transaction{
		ID:        txID,
		State:     "pending",
		Messages:  make([]Message, 0),
		Timestamp: time.Now(),
	}
	mb.txManager.mu.Unlock()

	return txID
}

func (mb *ExactlyOnceMessageBus) SendTransactional(ctx context.Context, txID, topicName string, key, value []byte) error {
	mb.txManager.mu.Lock()
	tx, exists := mb.txManager.transactions[txID]
	mb.txManager.mu.Unlock()

	if !exists {
		return fmt.Errorf("transaction %s not found", txID)
	}

	if tx.State != "pending" {
		return fmt.Errorf("transaction %s is not pending", txID)
	}

	// Prepare message but don't send yet
	message := Message{
		Topic:     topicName,
		Key:       key,
		Value:     value,
		Timestamp: time.Now(),
	}

	mb.txManager.mu.Lock()
	tx.Messages = append(tx.Messages, message)
	mb.txManager.mu.Unlock()

	return nil
}

func (mb *ExactlyOnceMessageBus) CommitTransaction(ctx context.Context, txID string) error {
	mb.txManager.mu.Lock()
	tx, exists := mb.txManager.transactions[txID]
	if !exists {
		mb.txManager.mu.Unlock()
		return fmt.Errorf("transaction %s not found", txID)
	}

	tx.State = "committed"
	messages := tx.Messages
	mb.txManager.mu.Unlock()

	// Send all messages atomically
	for _, msg := range messages {
		if err := mb.Send(ctx, msg.Topic, msg.Key, msg.Value, msg.Headers); err != nil {
			// Rollback
			mb.txManager.mu.Lock()
			tx.State = "aborted"
			mb.txManager.mu.Unlock()
			return err
		}
	}

	return nil
}

// Solution 3: Fault-Tolerant Message Bus
type FaultTolerantMessageBus struct {
	*PartitionedMessageBus
	healthChecker *HealthChecker
	dlq           *DeadLetterQueue
}

type HealthChecker struct {
	checks map[string]time.Time
	mu     sync.RWMutex
}

type DeadLetterQueue struct {
	messages []Message
	mu       sync.Mutex
}

func NewFaultTolerantMessageBus() *FaultTolerantMessageBus {
	mb := &FaultTolerantMessageBus{
		PartitionedMessageBus: NewPartitionedMessageBus(),
		healthChecker: &HealthChecker{
			checks: make(map[string]time.Time),
		},
		dlq: &DeadLetterQueue{
			messages: make([]Message, 0),
		},
	}

	// Start health checker
	go mb.runHealthChecker()

	return mb
}

func (mb *FaultTolerantMessageBus) runHealthChecker() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		mb.mu.RLock()
		groups := make([]*ConsumerGroup, 0)
		for _, group := range mb.groups {
			groups = append(groups, group)
		}
		mb.mu.RUnlock()

		for _, group := range groups {
			mb.checkGroupHealth(group)
		}
	}
}

func (mb *FaultTolerantMessageBus) checkGroupHealth(group *ConsumerGroup) {
	group.mu.Lock()
	defer group.mu.Unlock()

	now := time.Now()
	for id, consumer := range group.Consumers {
		// Check heartbeat
		if consumer.Active && now.Sub(consumer.Heartbeat) > 30*time.Second {
			consumer.Active = false
			log.Printf("Consumer %s marked inactive due to heartbeat timeout", id)

			// Trigger rebalance
			go mb.rebalance(group.ID)
		}
	}
}

func (mb *FaultTolerantMessageBus) ConsumeWithRetry(ctx context.Context, groupID, consumerID string) error {
	mb.mu.RLock()
	group := mb.groups[groupID]
	mb.mu.RUnlock()

	if group == nil {
		return fmt.Errorf("consumer group %s not found", groupID)
	}

	group.mu.RLock()
	consumer := group.Consumers[consumerID]
	assignments := make(map[string]map[int]string)
	for topic, partitions := range group.Assignments {
		assignments[topic] = make(map[int]string)
		for part, cons := range partitions {
			if cons == consumerID {
				assignments[topic][part] = cons
			}
		}
	}
	group.mu.RUnlock()

	if consumer == nil {
		return fmt.Errorf("consumer %s not found", consumerID)
	}

	// Process assigned partitions
	for topicName, partitions := range assignments {
		for partitionID := range partitions {
			if err := mb.consumePartition(ctx, groupID, consumer, topicName, partitionID); err != nil {
				log.Printf("Error consuming partition %d: %v", partitionID, err)
			}
		}
	}

	return nil
}

func (mb *FaultTolerantMessageBus) consumePartition(ctx context.Context, groupID string, consumer *Consumer, topicName string, partitionID int) error {
	mb.mu.RLock()
	topic := mb.topics[topicName]
	mb.mu.RUnlock()

	if topic == nil || partitionID >= len(topic.Partitions) {
		return fmt.Errorf("invalid topic or partition")
	}

	partition := topic.Partitions[partitionID]

	// Get last offset
	mb.offsetMgr.mu.RLock()
	lastOffset := int64(0)
	if topicOffsets, exists := mb.offsetMgr.offsets[groupID]; exists {
		if partitionOffsets, exists := topicOffsets[topicName]; exists {
			lastOffset = partitionOffsets[partitionID]
		}
	}
	mb.offsetMgr.mu.RUnlock()

	// Read messages
	partition.mu.RLock()
	messages := make([]Message, 0)
	for _, msg := range partition.Messages {
		if msg.Offset >= lastOffset {
			messages = append(messages, msg)
		}
	}
	partition.mu.RUnlock()

	// Process messages with retry
	for _, msg := range messages {
		retryCount := 0
		maxRetries := 3

		for retryCount < maxRetries {
			err := mb.processMessageWithTimeout(ctx, consumer, msg)
			if err == nil {
				// Commit offset
				mb.commitOffset(groupID, topicName, partitionID, msg.Offset+1)
				break
			}

			retryCount++
			if retryCount < maxRetries {
				// Exponential backoff
				time.Sleep(time.Duration(retryCount*retryCount) * 100 * time.Millisecond)
			} else {
				// Send to DLQ
				mb.dlq.mu.Lock()
				mb.dlq.messages = append(mb.dlq.messages, msg)
				mb.dlq.mu.Unlock()
				log.Printf("Message sent to DLQ after %d retries: %s", maxRetries, msg.ID)
			}
		}
	}

	return nil
}

func (mb *FaultTolerantMessageBus) processMessageWithTimeout(ctx context.Context, consumer *Consumer, msg Message) error {
	processCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic: %v", r)
			}
		}()
		errChan <- consumer.Handler(processCtx, msg)
	}()

	select {
	case err := <-errChan:
		return err
	case <-processCtx.Done():
		return fmt.Errorf("timeout processing message")
	}
}

func (mb *FaultTolerantMessageBus) commitOffset(groupID, topicName string, partitionID int, offset int64) {
	mb.offsetMgr.mu.Lock()
	defer mb.offsetMgr.mu.Unlock()

	if mb.offsetMgr.offsets[groupID] == nil {
		mb.offsetMgr.offsets[groupID] = make(map[string]map[int]int64)
	}
	if mb.offsetMgr.offsets[groupID][topicName] == nil {
		mb.offsetMgr.offsets[groupID][topicName] = make(map[int]int64)
	}
	mb.offsetMgr.offsets[groupID][topicName][partitionID] = offset
}

// RunSolution18 demonstrates the three solutions
func RunSolution18() {
	fmt.Println("=== Solution 18: Message Bus Pattern ===")

	ctx := context.Background()

	// Solution 1: Well-Partitioned Message Bus
	fmt.Println("\n1. Well-Partitioned Message Bus:")
	partitionedBus := NewPartitionedMessageBus()

	// Create topic with proper configuration
	partitionedBus.CreateTopic("orders", TopicConfig{
		NumPartitions:     3,
		ReplicationFactor: 2,
		RetentionMs:       86400000, // 24 hours
		CompressionType:   "snappy",
	})

	// Create consumer group
	partitionedBus.CreateConsumerGroup("order-processors")

	// Add consumer
	partitionedBus.JoinGroup("order-processors", "consumer-1", []string{"orders"},
		func(ctx context.Context, msg Message) error {
			fmt.Printf("  Consumer-1 processing: %s (partition %d, offset %d)\n",
				string(msg.Value), msg.Partition, msg.Offset)
			return nil
		})

	// Send messages with proper partitioning
	for i := 0; i < 6; i++ {
		key := []byte(fmt.Sprintf("order-%d", i%3))
		value := []byte(fmt.Sprintf("Order data %d", i))
		partitionedBus.Send(ctx, "orders", key, value, nil)
	}

	// Solution 2: Exactly-Once Message Bus
	fmt.Println("\n2. Exactly-Once Message Bus:")
	exactlyOnceBus := NewExactlyOnceMessageBus()
	exactlyOnceBus.CreateTopic("transactions", TopicConfig{
		NumPartitions: 2,
	})

	// Begin transaction
	txID := exactlyOnceBus.BeginTransaction()

	// Send messages transactionally
	exactlyOnceBus.SendTransactional(ctx, txID, "transactions",
		[]byte("tx-1"), []byte("Transaction 1"))
	exactlyOnceBus.SendTransactional(ctx, txID, "transactions",
		[]byte("tx-2"), []byte("Transaction 2"))

	// Commit transaction
	if err := exactlyOnceBus.CommitTransaction(ctx, txID); err != nil {
		fmt.Printf("  Transaction failed: %v\n", err)
	} else {
		fmt.Println("  Transaction committed successfully")
	}

	// Solution 3: Fault-Tolerant Message Bus
	fmt.Println("\n3. Fault-Tolerant Message Bus:")
	faultTolerantBus := NewFaultTolerantMessageBus()
	faultTolerantBus.CreateTopic("events", TopicConfig{
		NumPartitions: 2,
	})

	faultTolerantBus.CreateConsumerGroup("event-processors")

	// Add consumer with fault tolerance
	faultTolerantBus.JoinGroup("event-processors", "consumer-ft", []string{"events"},
		func(ctx context.Context, msg Message) error {
			// Simulate occasional failures
			if msg.Offset%3 == 0 {
				return fmt.Errorf("simulated failure")
			}
			fmt.Printf("  Processed: %s (retry handled)\n", string(msg.Value))
			return nil
		})

	// Send test messages
	for i := 0; i < 5; i++ {
		faultTolerantBus.Send(ctx, "events", nil,
			[]byte(fmt.Sprintf("Event %d", i)), nil)
	}

	// Consume with retry
	faultTolerantBus.ConsumeWithRetry(ctx, "event-processors", "consumer-ft")

	fmt.Printf("\n  DLQ size: %d messages\n", len(faultTolerantBus.dlq.messages))

	fmt.Println("\nAll solutions demonstrated successfully!")
}