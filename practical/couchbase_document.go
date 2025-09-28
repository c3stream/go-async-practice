package practical

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CouchbaseExample demonstrates document database patterns with Couchbase-like operations
// This example simulates Couchbase patterns without actual connection for educational purposes

// Document represents a JSON document in Couchbase
type Document struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	CAS       uint64                 `json:"cas"` // Compare-And-Swap value
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
	TTL       *time.Duration         `json:"ttl,omitempty"`
}

// CouchbaseBucket simulates a Couchbase bucket
type CouchbaseBucket struct {
	mu         sync.RWMutex
	documents  map[string]*Document
	indexes    map[string]map[string][]string // index_name -> field_value -> document_ids
	casCounter uint64
	stats      *BucketStats
}

// BucketStats tracks bucket operation statistics
type BucketStats struct {
	Gets      atomic.Int64
	Sets      atomic.Int64
	Deletes   atomic.Int64
	CASErrors atomic.Int64
	Queries   atomic.Int64
}

// NewCouchbaseBucket creates a new simulated bucket
func NewCouchbaseBucket() *CouchbaseBucket {
	return &CouchbaseBucket{
		documents: make(map[string]*Document),
		indexes:   make(map[string]map[string][]string),
		stats:     &BucketStats{},
	}
}

// Get retrieves a document by ID
func (cb *CouchbaseBucket) Get(ctx context.Context, id string) (*Document, error) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	cb.stats.Gets.Add(1)

	doc, exists := cb.documents[id]
	if !exists {
		return nil, fmt.Errorf("document not found: %s", id)
	}

	// Check TTL
	if doc.TTL != nil && time.Since(doc.CreatedAt) > *doc.TTL {
		return nil, fmt.Errorf("document expired: %s", id)
	}

	// Deep copy to prevent external modifications
	copiedDoc := *doc
	copiedData := make(map[string]interface{})
	for k, v := range doc.Data {
		copiedData[k] = v
	}
	copiedDoc.Data = copiedData

	return &copiedDoc, nil
}

// Set stores a document with optimistic locking
func (cb *CouchbaseBucket) Set(ctx context.Context, doc *Document) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.stats.Sets.Add(1)

	// Generate new CAS value
	cas := atomic.AddUint64(&cb.casCounter, 1)
	doc.CAS = cas
	doc.UpdatedAt = time.Now()

	if _, exists := cb.documents[doc.ID]; !exists {
		doc.CreatedAt = time.Now()
	}

	cb.documents[doc.ID] = doc

	// Update indexes
	cb.updateIndexes(doc)

	return nil
}

// SetWithCAS performs compare-and-swap update
func (cb *CouchbaseBucket) SetWithCAS(ctx context.Context, doc *Document, expectedCAS uint64) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	existing, exists := cb.documents[doc.ID]
	if !exists {
		return fmt.Errorf("document not found for CAS update: %s", doc.ID)
	}

	if existing.CAS != expectedCAS {
		cb.stats.CASErrors.Add(1)
		return fmt.Errorf("CAS mismatch: expected %d, got %d", expectedCAS, existing.CAS)
	}

	cb.stats.Sets.Add(1)
	cas := atomic.AddUint64(&cb.casCounter, 1)
	doc.CAS = cas
	doc.UpdatedAt = time.Now()
	cb.documents[doc.ID] = doc
	cb.updateIndexes(doc)

	return nil
}

// Delete removes a document
func (cb *CouchbaseBucket) Delete(ctx context.Context, id string) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.stats.Deletes.Add(1)

	if _, exists := cb.documents[id]; !exists {
		return fmt.Errorf("document not found: %s", id)
	}

	delete(cb.documents, id)
	cb.removeFromIndexes(id)
	return nil
}

// Query performs a simple N1QL-like query
func (cb *CouchbaseBucket) Query(ctx context.Context, docType string, field string, value interface{}) ([]*Document, error) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	cb.stats.Queries.Add(1)

	var results []*Document
	for _, doc := range cb.documents {
		if doc.Type == docType {
			if fieldValue, ok := doc.Data[field]; ok && fieldValue == value {
				// Check TTL
				if doc.TTL != nil && time.Since(doc.CreatedAt) > *doc.TTL {
					continue
				}
				results = append(results, doc)
			}
		}
	}

	return results, nil
}

// BulkGet retrieves multiple documents concurrently
func (cb *CouchbaseBucket) BulkGet(ctx context.Context, ids []string) (map[string]*Document, error) {
	results := make(map[string]*Document)
	resultsMu := sync.Mutex{}
	errors := make([]error, 0)
	errorsMu := sync.Mutex{}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 10) // Limit concurrent operations

	for _, id := range ids {
		wg.Add(1)
		go func(docID string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			doc, err := cb.Get(ctx, docID)
			if err != nil {
				errorsMu.Lock()
				errors = append(errors, fmt.Errorf("failed to get %s: %w", docID, err))
				errorsMu.Unlock()
				return
			}

			resultsMu.Lock()
			results[docID] = doc
			resultsMu.Unlock()
		}(id)
	}

	wg.Wait()

	if len(errors) > 0 {
		return results, fmt.Errorf("bulk get had %d errors", len(errors))
	}

	return results, nil
}

// updateIndexes updates secondary indexes
func (cb *CouchbaseBucket) updateIndexes(doc *Document) {
	// Simple index on document type
	typeIndex := "type_index"
	if cb.indexes[typeIndex] == nil {
		cb.indexes[typeIndex] = make(map[string][]string)
	}
	cb.indexes[typeIndex][doc.Type] = append(cb.indexes[typeIndex][doc.Type], doc.ID)
}

// removeFromIndexes removes document from indexes
func (cb *CouchbaseBucket) removeFromIndexes(id string) {
	for _, index := range cb.indexes {
		for field, docs := range index {
			for i, docID := range docs {
				if docID == id {
					index[field] = append(docs[:i], docs[i+1:]...)
					break
				}
			}
		}
	}
}

// ViewQuery simulates a MapReduce view query
type ViewQuery struct {
	bucket    *CouchbaseBucket
	mapFunc   func(*Document) (string, interface{})
	reduceFunc func([]interface{}) interface{}
}

// NewViewQuery creates a new view query
func NewViewQuery(bucket *CouchbaseBucket) *ViewQuery {
	return &ViewQuery{
		bucket: bucket,
	}
}

// Map sets the map function
func (vq *ViewQuery) Map(fn func(*Document) (string, interface{})) *ViewQuery {
	vq.mapFunc = fn
	return vq
}

// Reduce sets the reduce function
func (vq *ViewQuery) Reduce(fn func([]interface{}) interface{}) *ViewQuery {
	vq.reduceFunc = fn
	return vq
}

// Execute runs the view query
func (vq *ViewQuery) Execute(ctx context.Context) (map[string]interface{}, error) {
	if vq.mapFunc == nil {
		return nil, fmt.Errorf("map function not set")
	}

	vq.bucket.mu.RLock()
	defer vq.bucket.mu.RUnlock()

	// Map phase
	mapped := make(map[string][]interface{})
	for _, doc := range vq.bucket.documents {
		key, value := vq.mapFunc(doc)
		if key != "" {
			mapped[key] = append(mapped[key], value)
		}
	}

	// Reduce phase
	results := make(map[string]interface{})
	if vq.reduceFunc != nil {
		for key, values := range mapped {
			results[key] = vq.reduceFunc(values)
		}
	} else {
		for key, values := range mapped {
			results[key] = values
		}
	}

	return results, nil
}

// RunCouchbaseExample demonstrates document database patterns
func RunCouchbaseExample() {
	fmt.Println("=== Couchbase Document Database Example ===\n")

	bucket := NewCouchbaseBucket()
	ctx := context.Background()

	// 1. Basic CRUD operations
	fmt.Println("1. Basic CRUD Operations:")

	// Create documents
	user1 := &Document{
		ID:   "user:1001",
		Type: "user",
		Data: map[string]interface{}{
			"name":  "Alice",
			"email": "alice@example.com",
			"age":   30,
		},
	}

	user2 := &Document{
		ID:   "user:1002",
		Type: "user",
		Data: map[string]interface{}{
			"name":  "Bob",
			"email": "bob@example.com",
			"age":   25,
		},
	}

	// Set documents
	if err := bucket.Set(ctx, user1); err != nil {
		fmt.Printf("Error setting user1: %v\n", err)
	}
	if err := bucket.Set(ctx, user2); err != nil {
		fmt.Printf("Error setting user2: %v\n", err)
	}

	// Get document
	retrieved, err := bucket.Get(ctx, "user:1001")
	if err != nil {
		fmt.Printf("Error getting user: %v\n", err)
	} else {
		fmt.Printf("Retrieved user: %+v\n", retrieved.Data)
	}

	// 2. Optimistic locking with CAS
	fmt.Println("\n2. Optimistic Locking with CAS:")

	// Simulate concurrent updates
	var wg sync.WaitGroup
	errors := make([]error, 0)
	errorsMu := sync.Mutex{}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(attempt int) {
			defer wg.Done()

			// Get current document
			doc, err := bucket.Get(ctx, "user:1001")
			if err != nil {
				errorsMu.Lock()
				errors = append(errors, err)
				errorsMu.Unlock()
				return
			}

			// Modify document
			doc.Data["last_update"] = fmt.Sprintf("attempt_%d", attempt)

			// Try to update with CAS
			err = bucket.SetWithCAS(ctx, doc, doc.CAS)
			if err != nil {
				errorsMu.Lock()
				errors = append(errors, err)
				errorsMu.Unlock()
				fmt.Printf("CAS conflict in attempt %d\n", attempt)
			} else {
				fmt.Printf("Successfully updated in attempt %d\n", attempt)
			}
		}(i)
	}

	wg.Wait()
	fmt.Printf("CAS errors: %d\n", bucket.stats.CASErrors.Load())

	// 3. Bulk operations
	fmt.Println("\n3. Bulk Operations:")

	// Create more documents
	for i := 3; i <= 10; i++ {
		doc := &Document{
			ID:   fmt.Sprintf("user:%d", 1000+i),
			Type: "user",
			Data: map[string]interface{}{
				"name": fmt.Sprintf("User%d", i),
				"age":  20 + i,
			},
		}
		bucket.Set(ctx, doc)
	}

	// Bulk get
	ids := []string{"user:1001", "user:1002", "user:1003", "user:1004", "user:1005"}
	bulkResults, err := bucket.BulkGet(ctx, ids)
	if err != nil {
		fmt.Printf("Bulk get error: %v\n", err)
	} else {
		fmt.Printf("Retrieved %d documents in bulk\n", len(bulkResults))
	}

	// 4. Query operations
	fmt.Println("\n4. Query Operations:")

	// Add some products
	for i := 1; i <= 5; i++ {
		product := &Document{
			ID:   fmt.Sprintf("product:%d", i),
			Type: "product",
			Data: map[string]interface{}{
				"name":     fmt.Sprintf("Product %d", i),
				"category": "electronics",
				"price":    float64(100 * i),
			},
		}
		bucket.Set(ctx, product)
	}

	// Query by type and field
	products, err := bucket.Query(ctx, "product", "category", "electronics")
	if err != nil {
		fmt.Printf("Query error: %v\n", err)
	} else {
		fmt.Printf("Found %d electronics products\n", len(products))
	}

	// 5. MapReduce View
	fmt.Println("\n5. MapReduce View:")

	view := NewViewQuery(bucket)

	// Count users by age group
	results, err := view.
		Map(func(doc *Document) (string, interface{}) {
			if doc.Type == "user" {
				if age, ok := doc.Data["age"].(int); ok {
					if age < 30 {
						return "young", 1
					}
					return "adult", 1
				}
			}
			return "", nil
		}).
		Reduce(func(values []interface{}) interface{} {
			sum := 0
			for _, v := range values {
				if count, ok := v.(int); ok {
					sum += count
				}
			}
			return sum
		}).
		Execute(ctx)

	if err != nil {
		fmt.Printf("View error: %v\n", err)
	} else {
		fmt.Printf("User age groups: %+v\n", results)
	}

	// 6. TTL Example
	fmt.Println("\n6. TTL (Time-To-Live) Example:")

	ttl := 2 * time.Second
	tempDoc := &Document{
		ID:   "temp:1",
		Type: "temporary",
		Data: map[string]interface{}{
			"message": "This will expire in 2 seconds",
		},
		TTL: &ttl,
	}

	bucket.Set(ctx, tempDoc)
	fmt.Println("Set temporary document with 2 second TTL")

	// Try to get immediately
	if _, err := bucket.Get(ctx, "temp:1"); err == nil {
		fmt.Println("Document retrieved immediately")
	}

	// Wait for expiration
	time.Sleep(3 * time.Second)
	if _, err := bucket.Get(ctx, "temp:1"); err != nil {
		fmt.Println("Document expired after TTL")
	}

	// Print statistics
	fmt.Println("\n=== Operation Statistics ===")
	fmt.Printf("Gets: %d\n", bucket.stats.Gets.Load())
	fmt.Printf("Sets: %d\n", bucket.stats.Sets.Load())
	fmt.Printf("Deletes: %d\n", bucket.stats.Deletes.Load())
	fmt.Printf("CAS Errors: %d\n", bucket.stats.CASErrors.Load())
	fmt.Printf("Queries: %d\n", bucket.stats.Queries.Load())
}