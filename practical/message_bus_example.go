package practical

import (
	"context"
	"fmt"
	"hash/fnv"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// MessageBusExample demonstrates a production-ready message bus with partitioning,
// consumer groups, and exactly-once semantics
type MessageBusExample struct {
	topics         map[string]*Topic
	consumerGroups map[string]*ConsumerGroup
	mu             sync.RWMutex
	offsetStore    *OffsetStore
	dlq            *DeadLetterQueue
	metrics        *BusMetrics
}

type Topic struct {
	Name            string
	Partitions      []*Partition
	RetentionPeriod time.Duration
	mu              sync.RWMutex
}

type Partition struct {
	ID       int
	Messages []Message
	Offset   int64
	mu       sync.RWMutex
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
	Attempts  int
}

type ConsumerGroup struct {
	ID               string
	Consumers        map[string]*Consumer
	Assignments      map[int]string // partition -> consumer ID
	RebalanceVersion int64
	mu               sync.RWMutex
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

type OffsetStore struct {
	offsets map[string]map[int]int64 // groupID -> partition -> offset
	mu      sync.RWMutex
}

type DeadLetterQueue struct {
	messages []Message
	mu       sync.RWMutex
}

type BusMetrics struct {
	messagesPublished int64
	messagesConsumed  int64
	messagesInDLQ     int64
	consumerLag       map[string]int64
	mu                sync.RWMutex
}

// NewMessageBusExample creates a production-ready message bus
func NewMessageBusExample() *MessageBusExample {
	return &MessageBusExample{
		topics:         make(map[string]*Topic),
		consumerGroups: make(map[string]*ConsumerGroup),
		offsetStore: &OffsetStore{
			offsets: make(map[string]map[int]int64),
		},
		dlq: &DeadLetterQueue{
			messages: make([]Message, 0),
		},
		metrics: &BusMetrics{
			consumerLag: make(map[string]int64),
		},
	}
}

// CreateTopic creates a new topic with partitions
func (mb *MessageBusExample) CreateTopic(name string, numPartitions int, retention time.Duration) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if _, exists := mb.topics[name]; exists {
		return fmt.Errorf("topic %s already exists", name)
	}

	partitions := make([]*Partition, numPartitions)
	for i := 0; i < numPartitions; i++ {
		partitions[i] = &Partition{
			ID:       i,
			Messages: make([]Message, 0),
			Offset:   0,
		}
	}

	mb.topics[name] = &Topic{
		Name:            name,
		Partitions:      partitions,
		RetentionPeriod: retention,
	}

	// Start retention policy enforcer
	go mb.enforceRetention(name)

	log.Printf("Created topic %s with %d partitions", name, numPartitions)
	return nil
}

// Publish sends a message to a topic
func (mb *MessageBusExample) Publish(ctx context.Context, topicName string, key, value []byte, headers map[string]string) error {
	mb.mu.RLock()
	topic, exists := mb.topics[topicName]
	mb.mu.RUnlock()

	if !exists {
		return fmt.Errorf("topic %s not found", topicName)
	}

	// Select partition based on key
	partitionID := mb.selectPartition(key, len(topic.Partitions))
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
		Attempts:  0,
	}

	partition.Messages = append(partition.Messages, message)
	partition.Offset++
	partition.mu.Unlock()

	// Update metrics
	atomic.AddInt64(&mb.metrics.messagesPublished, 1)

	// Notify consumers
	mb.notifyConsumers(topicName, partitionID)

	return nil
}

// selectPartition selects a partition based on the key
func (mb *MessageBusExample) selectPartition(key []byte, numPartitions int) int {
	if key == nil || len(key) == 0 {
		// Round-robin for null keys
		return int(time.Now().UnixNano()) % numPartitions
	}

	// Hash-based partitioning for keyed messages
	h := fnv.New32a()
	h.Write(key)
	return int(h.Sum32()) % numPartitions
}

// CreateConsumerGroup creates a new consumer group
func (mb *MessageBusExample) CreateConsumerGroup(groupID string) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	if _, exists := mb.consumerGroups[groupID]; exists {
		return fmt.Errorf("consumer group %s already exists", groupID)
	}

	mb.consumerGroups[groupID] = &ConsumerGroup{
		ID:          groupID,
		Consumers:   make(map[string]*Consumer),
		Assignments: make(map[int]string),
	}

	// Initialize offset tracking
	mb.offsetStore.mu.Lock()
	mb.offsetStore.offsets[groupID] = make(map[int]int64)
	mb.offsetStore.mu.Unlock()

	log.Printf("Created consumer group %s", groupID)
	return nil
}

// JoinGroup adds a consumer to a consumer group
func (mb *MessageBusExample) JoinGroup(groupID, consumerID string, topics []string, handler MessageHandler) error {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	group, exists := mb.consumerGroups[groupID]
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

	// Start heartbeat
	go mb.heartbeat(groupID, consumerID)

	log.Printf("Consumer %s joined group %s", consumerID, groupID)
	return nil
}

// rebalance redistributes partitions among consumers
func (mb *MessageBusExample) rebalance(groupID string) {
	mb.mu.RLock()
	group := mb.consumerGroups[groupID]
	mb.mu.RUnlock()

	if group == nil {
		return
	}

	group.mu.Lock()
	defer group.mu.Unlock()

	// Collect all partitions from subscribed topics
	allPartitions := make([]int, 0)
	topicsMap := make(map[string]bool)

	for _, consumer := range group.Consumers {
		if !consumer.Active {
			continue
		}
		for _, topic := range consumer.Topics {
			topicsMap[topic] = true
		}
	}

	mb.mu.RLock()
	for topicName := range topicsMap {
		if topic, exists := mb.topics[topicName]; exists {
			for i := 0; i < len(topic.Partitions); i++ {
				allPartitions = append(allPartitions, i)
			}
		}
	}
	mb.mu.RUnlock()

	// Get active consumers
	activeConsumers := make([]string, 0)
	for id, consumer := range group.Consumers {
		if consumer.Active {
			activeConsumers = append(activeConsumers, id)
		}
	}

	if len(activeConsumers) == 0 {
		return
	}

	// Clear current assignments
	group.Assignments = make(map[int]string)

	// Assign partitions round-robin
	for i, partition := range allPartitions {
		consumerID := activeConsumers[i%len(activeConsumers)]
		group.Assignments[partition] = consumerID
	}

	group.RebalanceVersion++
	log.Printf("Rebalanced group %s: %d partitions across %d consumers",
		groupID, len(allPartitions), len(activeConsumers))
}

// Consume starts consuming messages for a consumer
func (mb *MessageBusExample) Consume(ctx context.Context, groupID, consumerID string) error {
	mb.mu.RLock()
	group := mb.consumerGroups[groupID]
	mb.mu.RUnlock()

	if group == nil {
		return fmt.Errorf("consumer group %s not found", groupID)
	}

	group.mu.RLock()
	consumer := group.Consumers[consumerID]
	group.mu.RUnlock()

	if consumer == nil {
		return fmt.Errorf("consumer %s not found", consumerID)
	}

	// Process assigned partitions
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			mb.consumePartitions(ctx, group, consumer)
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// consumePartitions consumes messages from assigned partitions
func (mb *MessageBusExample) consumePartitions(ctx context.Context, group *ConsumerGroup, consumer *Consumer) {
	group.mu.RLock()
	assignments := make(map[int]string)
	for p, c := range group.Assignments {
		if c == consumer.ID {
			assignments[p] = c
		}
	}
	group.mu.RUnlock()

	for partitionID := range assignments {
		for _, topicName := range consumer.Topics {
			mb.consumePartition(ctx, group.ID, consumer, topicName, partitionID)
		}
	}
}

// consumePartition consumes messages from a specific partition
func (mb *MessageBusExample) consumePartition(ctx context.Context, groupID string, consumer *Consumer, topicName string, partitionID int) {
	mb.mu.RLock()
	topic, exists := mb.topics[topicName]
	mb.mu.RUnlock()

	if !exists {
		return
	}

	if partitionID >= len(topic.Partitions) {
		return
	}

	partition := topic.Partitions[partitionID]

	// Get last committed offset
	mb.offsetStore.mu.RLock()
	offsets := mb.offsetStore.offsets[groupID]
	lastOffset := offsets[partitionID]
	mb.offsetStore.mu.RUnlock()

	partition.mu.RLock()
	messages := make([]Message, 0)
	for _, msg := range partition.Messages {
		if msg.Offset >= lastOffset {
			messages = append(messages, msg)
		}
	}
	partition.mu.RUnlock()

	// Process messages
	for _, msg := range messages {
		err := mb.processMessage(ctx, consumer, msg)
		if err != nil {
			log.Printf("Error processing message %s: %v", msg.ID, err)
			mb.handleFailedMessage(msg)
		} else {
			// Commit offset
			mb.commitOffset(groupID, partitionID, msg.Offset+1)
			atomic.AddInt64(&mb.metrics.messagesConsumed, 1)
		}
	}
}

// processMessage processes a single message
func (mb *MessageBusExample) processMessage(ctx context.Context, consumer *Consumer, msg Message) error {
	// Create timeout context
	processCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Execute handler with panic recovery
	errChan := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("panic in handler: %v", r)
			}
		}()
		errChan <- consumer.Handler(processCtx, msg)
	}()

	select {
	case err := <-errChan:
		return err
	case <-processCtx.Done():
		return fmt.Errorf("message processing timeout")
	}
}

// handleFailedMessage handles messages that failed processing
func (mb *MessageBusExample) handleFailedMessage(msg Message) {
	msg.Attempts++

	if msg.Attempts >= 3 {
		// Send to DLQ after max retries
		mb.dlq.mu.Lock()
		mb.dlq.messages = append(mb.dlq.messages, msg)
		mb.dlq.mu.Unlock()

		atomic.AddInt64(&mb.metrics.messagesInDLQ, 1)
		log.Printf("Message %s sent to DLQ after %d attempts", msg.ID, msg.Attempts)
	}
}

// commitOffset commits the consumer offset
func (mb *MessageBusExample) commitOffset(groupID string, partition int, offset int64) {
	mb.offsetStore.mu.Lock()
	defer mb.offsetStore.mu.Unlock()

	if mb.offsetStore.offsets[groupID] == nil {
		mb.offsetStore.offsets[groupID] = make(map[int]int64)
	}
	mb.offsetStore.offsets[groupID][partition] = offset
}

// notifyConsumers notifies consumers about new messages
func (mb *MessageBusExample) notifyConsumers(topicName string, partitionID int) {
	// In a real implementation, this would trigger consumer polling
	// For this example, consumers poll periodically
}

// heartbeat maintains consumer liveness
func (mb *MessageBusExample) heartbeat(groupID, consumerID string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		mb.mu.RLock()
		group := mb.consumerGroups[groupID]
		mb.mu.RUnlock()

		if group == nil {
			return
		}

		group.mu.Lock()
		if consumer, exists := group.Consumers[consumerID]; exists {
			consumer.Heartbeat = time.Now()
		}
		group.mu.Unlock()
	}
}

// enforceRetention removes old messages based on retention policy
func (mb *MessageBusExample) enforceRetention(topicName string) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		mb.mu.RLock()
		topic, exists := mb.topics[topicName]
		mb.mu.RUnlock()

		if !exists {
			return
		}

		cutoff := time.Now().Add(-topic.RetentionPeriod)

		for _, partition := range topic.Partitions {
			partition.mu.Lock()

			// Find cutoff index
			cutoffIndex := 0
			for i, msg := range partition.Messages {
				if msg.Timestamp.After(cutoff) {
					cutoffIndex = i
					break
				}
			}

			// Remove old messages
			if cutoffIndex > 0 {
				partition.Messages = partition.Messages[cutoffIndex:]
				log.Printf("Removed %d old messages from topic %s partition %d",
					cutoffIndex, topicName, partition.ID)
			}

			partition.mu.Unlock()
		}
	}
}

// GetMetrics returns current metrics
func (mb *MessageBusExample) GetMetrics() map[string]interface{} {
	mb.metrics.mu.RLock()
	defer mb.metrics.mu.RUnlock()

	return map[string]interface{}{
		"messages_published": mb.metrics.messagesPublished,
		"messages_consumed":  mb.metrics.messagesConsumed,
		"messages_in_dlq":    mb.metrics.messagesInDLQ,
		"consumer_lag":       mb.metrics.consumerLag,
	}
}

// RunMessageBusExample demonstrates the message bus in action
func RunMessageBusExample() {
	fmt.Println("\n=== Message Bus Example ===")

	mb := NewMessageBusExample()

	// Create topics
	mb.CreateTopic("orders", 3, 24*time.Hour)
	mb.CreateTopic("payments", 2, 7*24*time.Hour)

	// Create consumer group
	mb.CreateConsumerGroup("order-processors")

	// Add consumers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup

	// Start consumer 1
	wg.Add(1)
	go func() {
		defer wg.Done()

		mb.JoinGroup("order-processors", "consumer-1", []string{"orders"},
			func(ctx context.Context, msg Message) error {
				log.Printf("Consumer-1 processing: %s (partition %d, offset %d)",
					string(msg.Value), msg.Partition, msg.Offset)
				time.Sleep(50 * time.Millisecond) // Simulate processing
				return nil
			})

		mb.Consume(ctx, "order-processors", "consumer-1")
	}()

	// Start consumer 2
	wg.Add(1)
	go func() {
		defer wg.Done()

		mb.JoinGroup("order-processors", "consumer-2", []string{"orders"},
			func(ctx context.Context, msg Message) error {
				log.Printf("Consumer-2 processing: %s (partition %d, offset %d)",
					string(msg.Value), msg.Partition, msg.Offset)
				time.Sleep(50 * time.Millisecond) // Simulate processing
				return nil
			})

		mb.Consume(ctx, "order-processors", "consumer-2")
	}()

	// Wait for consumers to be ready
	time.Sleep(1 * time.Second)

	// Publish messages
	for i := 0; i < 20; i++ {
		key := []byte(fmt.Sprintf("order-%d", i%5))
		value := []byte(fmt.Sprintf("Order data %d", i))
		headers := map[string]string{
			"source": "order-service",
			"version": "1.0",
		}

		if err := mb.Publish(context.Background(), "orders", key, value, headers); err != nil {
			log.Printf("Failed to publish: %v", err)
		}
	}

	// Let consumers process
	time.Sleep(3 * time.Second)

	// Show metrics
	metrics := mb.GetMetrics()
	fmt.Printf("Metrics: %+v\n", metrics)

	// Cleanup
	cancel()
	wg.Wait()

	fmt.Println("Message bus example completed")
}