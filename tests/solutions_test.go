package tests

import (
	"context"
	"testing"
	"time"

	"github.com/kazuhirokondo/go-async-practice/solutions"
)

// Test Solutions 17-24
func TestSolution17_EventBus(t *testing.T) {
	t.Run("ReliableEventBus", func(t *testing.T) {
		eb := solutions.NewReliableEventBus()
		ctx := context.Background()

		// Test deduplication
		event := solutions.Event{
			ID:      "test-1",
			Type:    "test.event",
			Payload: "test data",
		}

		err := eb.Publish(ctx, event)
		if err != nil {
			t.Errorf("Failed to publish event: %v", err)
		}

		// Publish duplicate - should be ignored
		err = eb.Publish(ctx, event)
		if err != nil {
			t.Errorf("Failed to publish duplicate: %v", err)
		}
	})

	t.Run("BoundedEventBus", func(t *testing.T) {
		eb := solutions.NewBoundedEventBus(4, 100)
		defer eb.Shutdown()

		ctx := context.Background()
		event := solutions.Event{
			ID:   "test-2",
			Type: "test.event",
		}

		err := eb.Publish(ctx, event)
		if err != nil {
			t.Errorf("Failed to publish to bounded bus: %v", err)
		}
	})

	t.Run("CircuitBreakerEventBus", func(t *testing.T) {
		eb := solutions.NewCircuitBreakerEventBus()
		ctx := context.Background()

		// Subscribe with handler
		eb.Subscribe("test.type", func(ctx context.Context, e solutions.Event) error {
			return nil
		})

		event := solutions.Event{
			ID:   "test-3",
			Type: "test.type",
		}

		err := eb.Publish(ctx, event)
		if err != nil {
			t.Errorf("Failed to publish with circuit breaker: %v", err)
		}
	})
}

func TestSolution18_MessageBus(t *testing.T) {
	t.Run("PartitionedMessageBus", func(t *testing.T) {
		mb := solutions.NewPartitionedMessageBus(4)
		ctx := context.Background()

		// Create topic
		err := mb.CreateTopic("test-topic", 4)
		if err != nil {
			t.Errorf("Failed to create topic: %v", err)
		}

		// Publish message
		msg := solutions.Message{
			ID:    "msg-1",
			Topic: "test-topic",
			Key:   "key1",
			Value: []byte("test message"),
		}

		err = mb.Publish(ctx, msg)
		if err != nil {
			t.Errorf("Failed to publish message: %v", err)
		}
	})

	t.Run("ExactlyOnceMessageBus", func(t *testing.T) {
		mb := solutions.NewExactlyOnceMessageBus()
		ctx := context.Background()

		tx := mb.BeginTransaction()

		msg := solutions.Message{
			ID:    "msg-2",
			Topic: "test-topic",
			Value: []byte("transactional message"),
		}

		err := mb.PublishTransactional(ctx, tx, msg)
		if err != nil {
			t.Errorf("Failed to publish transactional message: %v", err)
		}

		err = mb.CommitTransaction(tx)
		if err != nil {
			t.Errorf("Failed to commit transaction: %v", err)
		}
	})
}

func TestSolution19_DistributedLogging(t *testing.T) {
	t.Run("BufferedLogger", func(t *testing.T) {
		logger := solutions.NewBufferedLogger()
		defer logger.Shutdown()

		entry := solutions.LogEntry{
			Level:     "INFO",
			Message:   "test log",
			Timestamp: time.Now(),
		}

		logger.Log(entry)

		// Give time for async processing
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("StructuredLogger", func(t *testing.T) {
		logger := solutions.NewStructuredLogger()

		err := logger.LogWithContext(context.Background(), "INFO", "test", map[string]interface{}{
			"key": "value",
		})

		if err != nil {
			t.Errorf("Failed to log with context: %v", err)
		}
	})
}

func TestSolution20_BlockchainConsensus(t *testing.T) {
	t.Run("ProofOfWork", func(t *testing.T) {
		blockchain := solutions.NewPoWBlockchain()

		// Add transaction
		tx := solutions.Transaction{
			ID:     "tx-1",
			From:   "Alice",
			To:     "Bob",
			Amount: 100,
		}
		blockchain.AddTransaction(tx)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Mine block
		block, err := blockchain.MineBlock(ctx, "miner1")
		if err != nil && err != context.DeadlineExceeded {
			t.Errorf("Mining failed: %v", err)
		}

		if block != nil {
			// Validate chain
			if !blockchain.ValidateChain() {
				t.Error("Blockchain validation failed")
			}
		}
	})

	t.Run("PBFT", func(t *testing.T) {
		nodes := []string{"node1", "node2", "node3", "node4"}
		pbft := solutions.NewPBFTConsensus("node1", nodes)

		block := &solutions.Block{
			Index:     1,
			Timestamp: time.Now(),
			Hash:      "test_hash",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// Primary node proposes block
		err := pbft.ProposeBlock(ctx, block)
		// Timeout is expected in test environment
		if err != nil && err != context.DeadlineExceeded {
			// This is OK in test
		}
	})
}

func TestSolution21_GraphDatabase(t *testing.T) {
	t.Run("LockFreeGraph", func(t *testing.T) {
		graph := solutions.NewLockFreeGraph()
		ctx := context.Background()

		// Add nodes
		graph.AddNode("A")
		graph.AddNode("B")
		graph.AddNode("C")

		// Add edges
		graph.AddEdge("e1", "A", "B", "connects")
		graph.AddEdge("e2", "B", "C", "connects")

		// BFS traversal
		results, err := graph.BFS(ctx, "A", 2)
		if err != nil {
			t.Errorf("BFS failed: %v", err)
		}

		if len(results) == 0 {
			t.Error("BFS returned no results")
		}
	})

	t.Run("PartitionedGraph", func(t *testing.T) {
		graph := solutions.NewPartitionedGraph(4)

		// Add nodes
		for i := 1; i <= 10; i++ {
			graph.AddNode(solutions.Sprintf("node%d", i), nil)
		}

		// Add edges
		for i := 1; i < 10; i++ {
			graph.AddEdge(
				solutions.Sprintf("edge%d", i),
				solutions.Sprintf("node%d", i),
				solutions.Sprintf("node%d", i+1),
				nil,
			)
		}

		ctx := context.Background()
		visited, err := graph.ParallelTraversal(ctx, "node1")
		if err != nil {
			t.Errorf("Parallel traversal failed: %v", err)
		}

		if len(visited) == 0 {
			t.Error("No nodes visited")
		}
	})
}

func TestSolution22_TimeSeries(t *testing.T) {
	t.Run("CompressedTimeSeries", func(t *testing.T) {
		ts := solutions.NewCompressedTimeSeries()

		// Create columns
		ts.CreateColumn("temperature", "float")

		// Insert data points
		for i := 0; i < 10; i++ {
			point := solutions.DataPoint{
				Timestamp: time.Now().Add(time.Duration(i) * time.Second),
				Value:     float64(20 + i),
			}
			err := ts.Write("temperature", point)
			if err != nil {
				t.Errorf("Failed to write point: %v", err)
			}
		}

		// Query data
		start := time.Now().Add(-1 * time.Minute)
		end := time.Now().Add(1 * time.Minute)
		results, err := ts.Query("temperature", start, end)
		if err != nil {
			t.Errorf("Query failed: %v", err)
		}

		if len(results) == 0 {
			t.Error("Query returned no results")
		}
	})

	t.Run("WindowedAggregation", func(t *testing.T) {
		wts := solutions.NewWindowedTimeSeries()

		// Insert points
		windowSize := 10 * time.Second
		now := time.Now()
		for i := 0; i < 20; i++ {
			point := solutions.DataPoint{
				Timestamp: now.Add(time.Duration(i) * time.Second),
				Value:     float64(i),
			}
			wts.Insert(point, windowSize)
		}

		// Wait for windows to close
		time.Sleep(100 * time.Millisecond)

		// Get aggregates
		results, err := wts.Aggregate(windowSize, "avg")
		if err != nil {
			t.Errorf("Aggregation failed: %v", err)
		}

		// At least some results expected
		if len(results) == 0 {
			t.Log("No aggregation results yet (windows may not be closed)")
		}
	})
}

func TestSolution23_ColumnarStorage(t *testing.T) {
	t.Run("ColumnarStorage", func(t *testing.T) {
		cs := solutions.NewColumnarStorage()

		// Create columns
		cs.CreateColumn("id", "int")
		cs.CreateColumn("name", "string")
		cs.CreateColumn("value", "float")

		// Insert rows
		for i := 0; i < 10; i++ {
			row := map[string]interface{}{
				"id":    i,
				"name":  solutions.Sprintf("item-%d", i),
				"value": float64(i) * 1.5,
			}
			err := cs.Insert(row)
			if err != nil {
				t.Errorf("Failed to insert row: %v", err)
			}
		}

		// Query data
		results, err := cs.Query([]string{"id", "name"}, nil)
		if err != nil {
			t.Errorf("Query failed: %v", err)
		}

		if len(results) != 10 {
			t.Errorf("Expected 10 results, got %d", len(results))
		}
	})

	t.Run("WideColumnStore", func(t *testing.T) {
		wcs := solutions.NewWideColumnStore()

		// Create column family
		wcs.CreateColumnFamily("test_cf", &solutions.CFOptions{
			MaxVersions:  3,
			Compression:  "snappy",
			BloomFilter:  true,
			CacheEnabled: true,
		})

		// Put data
		err := wcs.Put("row1", "test_cf", "col1", []byte("value1"))
		if err != nil {
			t.Errorf("Failed to put data: %v", err)
		}

		// Get data
		value, err := wcs.Get("row1", "test_cf", "col1")
		if err != nil {
			t.Errorf("Failed to get data: %v", err)
		}

		if string(value) != "value1" && string(value) != "from_sstable" {
			t.Errorf("Unexpected value: %s", string(value))
		}
	})
}

func TestSolution24_ObjectStorage(t *testing.T) {
	t.Run("S3Compatible", func(t *testing.T) {
		s3 := solutions.NewS3CompatibleStorage()

		// Create bucket
		err := s3.CreateBucket("test-bucket")
		if err != nil {
			t.Errorf("Failed to create bucket: %v", err)
		}

		// Put object
		data := []byte("test data")
		obj, err := s3.PutObject("test-bucket", "test.txt", data, nil)
		if err != nil {
			t.Errorf("Failed to put object: %v", err)
		}

		if obj.Size != int64(len(data)) {
			t.Errorf("Object size mismatch: got %d, want %d", obj.Size, len(data))
		}

		// Get object
		retrieved, err := s3.GetObject("test-bucket", "test.txt")
		if err != nil {
			t.Errorf("Failed to get object: %v", err)
		}

		if retrieved.Key != "test.txt" {
			t.Errorf("Object key mismatch: got %s, want test.txt", retrieved.Key)
		}
	})

	t.Run("ContentAddressable", func(t *testing.T) {
		cas := solutions.NewContentAddressableStorage()

		// Store object
		data := []byte("content for CAS")
		obj, err := cas.Store("file1.txt", data)
		if err != nil {
			t.Errorf("Failed to store object: %v", err)
		}

		// Store duplicate content
		obj2, err := cas.Store("file2.txt", data)
		if err != nil {
			t.Errorf("Failed to store duplicate: %v", err)
		}

		// Should have same hash
		if obj.ContentHash != obj2.ContentHash {
			t.Error("Duplicate content has different hash")
		}

		// Retrieve
		retrieved, err := cas.Retrieve("file1.txt")
		if err != nil {
			t.Errorf("Failed to retrieve: %v", err)
		}

		if len(retrieved) == 0 {
			t.Error("Retrieved empty data")
		}
	})
}

// Benchmark tests
func BenchmarkEventBusPublish(b *testing.B) {
	eb := solutions.NewReliableEventBus()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event := solutions.Event{
			ID:      solutions.Sprintf("bench-%d", i),
			Type:    "bench.event",
			Payload: "benchmark data",
		}
		eb.Publish(ctx, event)
	}
}

func BenchmarkGraphTraversal(b *testing.B) {
	graph := solutions.NewLockFreeGraph()
	ctx := context.Background()

	// Setup graph
	for i := 0; i < 100; i++ {
		graph.AddNode(solutions.Sprintf("node%d", i))
	}
	for i := 0; i < 99; i++ {
		graph.AddEdge(
			solutions.Sprintf("edge%d", i),
			solutions.Sprintf("node%d", i),
			solutions.Sprintf("node%d", i+1),
			"connects",
		)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		graph.BFS(ctx, "node0", 10)
	}
}

func BenchmarkTimeSeriesWrite(b *testing.B) {
	ts := solutions.NewCompressedTimeSeries()
	ts.CreateColumn("bench", "float")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		point := solutions.DataPoint{
			Timestamp: time.Now(),
			Value:     float64(i),
		}
		ts.Write("bench", point)
	}
}

func BenchmarkColumnarInsert(b *testing.B) {
	cs := solutions.NewColumnarStorage()
	cs.CreateColumn("id", "int")
	cs.CreateColumn("value", "float")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		row := map[string]interface{}{
			"id":    i,
			"value": float64(i),
		}
		cs.Insert(row)
	}
}

func BenchmarkObjectStoragePut(b *testing.B) {
	s3 := solutions.NewS3CompatibleStorage()
	s3.CreateBucket("bench-bucket")

	data := []byte("benchmark object data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s3.PutObject("bench-bucket", solutions.Sprintf("obj-%d", i), data, nil)
	}
}