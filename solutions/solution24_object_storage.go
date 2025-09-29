package solutions

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Solution24 demonstrates three approaches to object storage:
// 1. S3-compatible object storage with multipart upload
// 2. Content-addressable storage with deduplication
// 3. Distributed object storage with erasure coding

// Solution 1: S3-Compatible Object Storage
type S3CompatibleStorage struct {
	mu            sync.RWMutex
	buckets       map[string]*Bucket
	multipartUploads map[string]*MultipartUpload
	versioning    bool
	lifecycle     *LifecycleManager
	metrics       *StorageMetrics
}

type Bucket struct {
	Name         string
	Created      time.Time
	Objects      map[string]*Object
	Versioning   bool
	ACL          *AccessControlList
	Tags         map[string]string
	mu           sync.RWMutex
}

type Object struct {
	Key          string
	Size         int64
	ETag         string
	LastModified time.Time
	ContentType  string
	Metadata     map[string]string
	Data         []byte
	Versions     []*ObjectVersion
	StorageClass string
}

type ObjectVersion struct {
	VersionID    string
	Size         int64
	ETag         string
	LastModified time.Time
	IsLatest     bool
	Data         []byte
}

type MultipartUpload struct {
	UploadID    string
	Bucket      string
	Key         string
	Parts       map[int]*Part
	InitiatedAt time.Time
	mu          sync.Mutex
}

type Part struct {
	PartNumber int
	ETag       string
	Size       int64
	Data       []byte
}

type AccessControlList struct {
	Owner       string
	Permissions map[string][]string // user -> permissions
	mu          sync.RWMutex
}

type LifecycleManager struct {
	rules   []LifecycleRule
	running int32
	ticker  *time.Ticker
}

type LifecycleRule struct {
	ID         string
	Status     string
	Expiration int // days
	Transition StorageTransition
}

type StorageTransition struct {
	Days         int
	StorageClass string
}

type StorageMetrics struct {
	TotalObjects   int64
	TotalSize      int64
	TotalBuckets   int64
	RequestCount   int64
	UploadBytes    int64
	DownloadBytes  int64
}

func NewS3CompatibleStorage() *S3CompatibleStorage {
	s3 := &S3CompatibleStorage{
		buckets:          make(map[string]*Bucket),
		multipartUploads: make(map[string]*MultipartUpload),
		versioning:       true,
		lifecycle: &LifecycleManager{
			rules:  make([]LifecycleRule, 0),
			ticker: time.NewTicker(1 * time.Hour),
		},
		metrics: &StorageMetrics{},
	}

	// Start lifecycle management
	go s3.runLifecycle()

	return s3
}

func (s3 *S3CompatibleStorage) CreateBucket(name string) error {
	s3.mu.Lock()
	defer s3.mu.Unlock()

	if _, exists := s3.buckets[name]; exists {
		return fmt.Errorf("bucket %s already exists", name)
	}

	s3.buckets[name] = &Bucket{
		Name:       name,
		Created:    time.Now(),
		Objects:    make(map[string]*Object),
		Versioning: s3.versioning,
		ACL: &AccessControlList{
			Owner:       "default",
			Permissions: make(map[string][]string),
		},
		Tags: make(map[string]string),
	}

	atomic.AddInt64(&s3.metrics.TotalBuckets, 1)
	return nil
}

func (s3 *S3CompatibleStorage) PutObject(bucket, key string, data []byte, metadata map[string]string) (*Object, error) {
	s3.mu.RLock()
	b, exists := s3.buckets[bucket]
	s3.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("bucket %s not found", bucket)
	}

	// Calculate ETag
	hash := md5.Sum(data)
	etag := hex.EncodeToString(hash[:])

	b.mu.Lock()
	defer b.mu.Unlock()

	obj, exists := b.Objects[key]
	if !exists {
		obj = &Object{
			Key:      key,
			Metadata: make(map[string]string),
			Versions: make([]*ObjectVersion, 0),
		}
		b.Objects[key] = obj
		atomic.AddInt64(&s3.metrics.TotalObjects, 1)
	}

	// Handle versioning
	if b.Versioning {
		// Create new version
		version := &ObjectVersion{
			VersionID:    generateVersionID(),
			Size:         int64(len(data)),
			ETag:         etag,
			LastModified: time.Now(),
			IsLatest:     true,
			Data:         data,
		}

		// Mark previous versions as not latest
		for _, v := range obj.Versions {
			v.IsLatest = false
		}

		obj.Versions = append(obj.Versions, version)
	}

	// Update object
	obj.Size = int64(len(data))
	obj.ETag = etag
	obj.LastModified = time.Now()
	obj.Data = data
	obj.StorageClass = "STANDARD"

	// Update metadata
	for k, v := range metadata {
		obj.Metadata[k] = v
	}

	atomic.AddInt64(&s3.metrics.TotalSize, obj.Size)
	atomic.AddInt64(&s3.metrics.UploadBytes, obj.Size)

	return obj, nil
}

func (s3 *S3CompatibleStorage) GetObject(bucket, key string) (*Object, error) {
	s3.mu.RLock()
	b, exists := s3.buckets[bucket]
	s3.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("bucket %s not found", bucket)
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	obj, exists := b.Objects[key]
	if !exists {
		return nil, fmt.Errorf("object %s not found", key)
	}

	atomic.AddInt64(&s3.metrics.DownloadBytes, obj.Size)
	atomic.AddInt64(&s3.metrics.RequestCount, 1)

	return obj, nil
}

func (s3 *S3CompatibleStorage) InitiateMultipartUpload(bucket, key string) (string, error) {
	uploadID := generateUploadID()

	upload := &MultipartUpload{
		UploadID:    uploadID,
		Bucket:      bucket,
		Key:         key,
		Parts:       make(map[int]*Part),
		InitiatedAt: time.Now(),
	}

	s3.mu.Lock()
	s3.multipartUploads[uploadID] = upload
	s3.mu.Unlock()

	return uploadID, nil
}

func (s3 *S3CompatibleStorage) UploadPart(uploadID string, partNumber int, data []byte) (string, error) {
	s3.mu.RLock()
	upload, exists := s3.multipartUploads[uploadID]
	s3.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("upload %s not found", uploadID)
	}

	// Calculate part ETag
	hash := md5.Sum(data)
	etag := hex.EncodeToString(hash[:])

	part := &Part{
		PartNumber: partNumber,
		ETag:       etag,
		Size:       int64(len(data)),
		Data:       make([]byte, len(data)),
	}
	copy(part.Data, data)

	upload.mu.Lock()
	upload.Parts[partNumber] = part
	upload.mu.Unlock()

	return etag, nil
}

func (s3 *S3CompatibleStorage) CompleteMultipartUpload(uploadID string, parts []int) (*Object, error) {
	s3.mu.RLock()
	upload, exists := s3.multipartUploads[uploadID]
	s3.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("upload %s not found", uploadID)
	}

	// Combine parts
	var buffer bytes.Buffer
	upload.mu.Lock()

	// Sort part numbers
	sort.Ints(parts)

	for _, partNum := range parts {
		part, exists := upload.Parts[partNum]
		if !exists {
			upload.mu.Unlock()
			return nil, fmt.Errorf("part %d not found", partNum)
		}
		buffer.Write(part.Data)
	}
	upload.mu.Unlock()

	// Create object
	obj, err := s3.PutObject(upload.Bucket, upload.Key, buffer.Bytes(), nil)
	if err != nil {
		return nil, err
	}

	// Clean up multipart upload
	s3.mu.Lock()
	delete(s3.multipartUploads, uploadID)
	s3.mu.Unlock()

	return obj, nil
}

func (s3 *S3CompatibleStorage) ListObjects(bucket string, prefix string, maxKeys int) ([]*Object, error) {
	s3.mu.RLock()
	b, exists := s3.buckets[bucket]
	s3.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("bucket %s not found", bucket)
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	objects := make([]*Object, 0)
	count := 0

	for key, obj := range b.Objects {
		if maxKeys > 0 && count >= maxKeys {
			break
		}

		if prefix == "" || hasPrefix(key, prefix) {
			objects = append(objects, obj)
			count++
		}
	}

	// Sort by key
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].Key < objects[j].Key
	})

	return objects, nil
}

func (s3 *S3CompatibleStorage) runLifecycle() {
	for range s3.lifecycle.ticker.C {
		if !atomic.CompareAndSwapInt32(&s3.lifecycle.running, 0, 1) {
			continue
		}

		s3.applyLifecycleRules()

		atomic.StoreInt32(&s3.lifecycle.running, 0)
	}
}

func (s3 *S3CompatibleStorage) applyLifecycleRules() {
	s3.mu.RLock()
	defer s3.mu.RUnlock()

	for _, bucket := range s3.buckets {
		bucket.mu.Lock()
		for key, obj := range bucket.Objects {
			age := time.Since(obj.LastModified).Hours() / 24

			for _, rule := range s3.lifecycle.rules {
				if rule.Status != "Enabled" {
					continue
				}

				// Check expiration
				if rule.Expiration > 0 && int(age) > rule.Expiration {
					delete(bucket.Objects, key)
					atomic.AddInt64(&s3.metrics.TotalObjects, -1)
					atomic.AddInt64(&s3.metrics.TotalSize, -obj.Size)
					log.Printf("Expired object %s/%s", bucket.Name, key)
				}

				// Check transition
				if rule.Transition.Days > 0 && int(age) > rule.Transition.Days {
					obj.StorageClass = rule.Transition.StorageClass
					log.Printf("Transitioned object %s/%s to %s", bucket.Name, key, obj.StorageClass)
				}
			}
		}
		bucket.mu.Unlock()
	}
}

// Solution 2: Content-Addressable Storage
type ContentAddressableStorage struct {
	mu           sync.RWMutex
	objects      map[string]*CASObject
	index        map[string][]string // content hash -> object keys
	dedup        *DeduplicationEngine
	compression  *CompressionEngine
}

type CASObject struct {
	Key         string
	ContentHash string
	Size        int64
	RefCount    int32
	Created     time.Time
	Data        []byte
	Compressed  bool
}

type DeduplicationEngine struct {
	mu         sync.RWMutex
	hashIndex  map[string]*DedupBlock
	savedSpace int64
}

type DedupBlock struct {
	Hash      string
	Size      int64
	RefCount  int32
	Data      []byte
}

type CompressionEngine struct {
	algorithm string
	level     int
}

func NewContentAddressableStorage() *ContentAddressableStorage {
	return &ContentAddressableStorage{
		objects: make(map[string]*CASObject),
		index:   make(map[string][]string),
		dedup: &DeduplicationEngine{
			hashIndex: make(map[string]*DedupBlock),
		},
		compression: &CompressionEngine{
			algorithm: "zstd",
			level:     3,
		},
	}
}

func (cas *ContentAddressableStorage) Store(key string, data []byte) (*CASObject, error) {
	// Calculate content hash
	hash := sha256.Sum256(data)
	contentHash := hex.EncodeToString(hash[:])

	cas.mu.Lock()
	defer cas.mu.Unlock()

	// Check for deduplication
	if existingKeys, exists := cas.index[contentHash]; exists {
		// Content already exists, just add reference
		if len(existingKeys) > 0 {
			existingObj := cas.objects[existingKeys[0]]
			atomic.AddInt32(&existingObj.RefCount, 1)

			// Create new object pointing to same content
			obj := &CASObject{
				Key:         key,
				ContentHash: contentHash,
				Size:        existingObj.Size,
				RefCount:    1,
				Created:     time.Now(),
				Data:        existingObj.Data,
				Compressed:  existingObj.Compressed,
			}

			cas.objects[key] = obj
			cas.index[contentHash] = append(cas.index[contentHash], key)

			atomic.AddInt64(&cas.dedup.savedSpace, obj.Size)
			log.Printf("Deduplicated object %s (saved %d bytes)", key, obj.Size)

			return obj, nil
		}
	}

	// Compress data
	compressed := cas.compression.compress(data)

	// Store new object
	obj := &CASObject{
		Key:         key,
		ContentHash: contentHash,
		Size:        int64(len(data)),
		RefCount:    1,
		Created:     time.Now(),
		Data:        compressed,
		Compressed:  true,
	}

	cas.objects[key] = obj
	cas.index[contentHash] = []string{key}

	// Update dedup index
	cas.dedup.mu.Lock()
	cas.dedup.hashIndex[contentHash] = &DedupBlock{
		Hash:     contentHash,
		Size:     obj.Size,
		RefCount: 1,
		Data:     compressed,
	}
	cas.dedup.mu.Unlock()

	return obj, nil
}

func (cas *ContentAddressableStorage) Retrieve(key string) ([]byte, error) {
	cas.mu.RLock()
	obj, exists := cas.objects[key]
	cas.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("object %s not found", key)
	}

	// Decompress if needed
	if obj.Compressed {
		return cas.compression.decompress(obj.Data), nil
	}

	return obj.Data, nil
}

func (cas *ContentAddressableStorage) Delete(key string) error {
	cas.mu.Lock()
	defer cas.mu.Unlock()

	obj, exists := cas.objects[key]
	if !exists {
		return fmt.Errorf("object %s not found", key)
	}

	// Decrement reference count
	if atomic.AddInt32(&obj.RefCount, -1) <= 0 {
		// Remove from dedup index if no more references
		cas.dedup.mu.Lock()
		if block, exists := cas.dedup.hashIndex[obj.ContentHash]; exists {
			if atomic.AddInt32(&block.RefCount, -1) <= 0 {
				delete(cas.dedup.hashIndex, obj.ContentHash)
			}
		}
		cas.dedup.mu.Unlock()
	}

	// Remove from index
	if keys, exists := cas.index[obj.ContentHash]; exists {
		newKeys := make([]string, 0)
		for _, k := range keys {
			if k != key {
				newKeys = append(newKeys, k)
			}
		}
		if len(newKeys) > 0 {
			cas.index[obj.ContentHash] = newKeys
		} else {
			delete(cas.index, obj.ContentHash)
		}
	}

	delete(cas.objects, key)
	return nil
}

func (eng *CompressionEngine) compress(data []byte) []byte {
	// Simplified compression (would use real compression library)
	if len(data) < 100 {
		return data
	}
	// Simulate compression
	compressed := make([]byte, len(data)/2)
	copy(compressed, data[:len(data)/2])
	return compressed
}

func (eng *CompressionEngine) decompress(data []byte) []byte {
	// Simplified decompression
	decompressed := make([]byte, len(data)*2)
	copy(decompressed, data)
	return decompressed
}

// Solution 3: Distributed Object Storage with Erasure Coding
type DistributedObjectStorage struct {
	nodes        []*DataNode
	metadata     *MetadataService
	erasureCoder *ErasureCoder
	placement    *PlacementPolicy
	recovery     *RecoveryService
}

type DataNode struct {
	ID       string
	Address  string
	Capacity int64
	Used     int64
	Objects  map[string]*DataChunk
	Status   string
	mu       sync.RWMutex
}

type DataChunk struct {
	ObjectID string
	ChunkID  int
	Data     []byte
	Checksum string
	Size     int64
}

type MetadataService struct {
	mu       sync.RWMutex
	objects  map[string]*ObjectMetadata
	location map[string][]ChunkLocation
}

type ObjectMetadata struct {
	Key          string
	Size         int64
	Chunks       int
	ParityChunks int
	Created      time.Time
	Checksum     string
}

type ChunkLocation struct {
	ChunkID int
	NodeID  string
	Type    string // "data" or "parity"
}

type ErasureCoder struct {
	dataChunks   int
	parityChunks int
}

type PlacementPolicy struct {
	replicationFactor int
	rackAwareness    bool
}

type RecoveryService struct {
	running int32
	ticker  *time.Ticker
}

func NewDistributedObjectStorage(numNodes int) *DistributedObjectStorage {
	nodes := make([]*DataNode, numNodes)
	for i := 0; i < numNodes; i++ {
		nodes[i] = &DataNode{
			ID:       fmt.Sprintf("node-%d", i),
			Address:  fmt.Sprintf("localhost:%d", 9000+i),
			Capacity: 1024 * 1024 * 1024, // 1GB
			Used:     0,
			Objects:  make(map[string]*DataChunk),
			Status:   "active",
		}
	}

	dos := &DistributedObjectStorage{
		nodes: nodes,
		metadata: &MetadataService{
			objects:  make(map[string]*ObjectMetadata),
			location: make(map[string][]ChunkLocation),
		},
		erasureCoder: &ErasureCoder{
			dataChunks:   4,
			parityChunks: 2,
		},
		placement: &PlacementPolicy{
			replicationFactor: 3,
			rackAwareness:    true,
		},
		recovery: &RecoveryService{
			ticker: time.NewTicker(30 * time.Second),
		},
	}

	// Start recovery service
	go dos.runRecovery()

	return dos
}

func (dos *DistributedObjectStorage) Put(ctx context.Context, key string, reader io.Reader) error {
	// Read data
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	// Calculate checksum
	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])

	// Split into chunks with erasure coding
	chunks := dos.erasureCoder.encode(data)

	// Select nodes for placement
	selectedNodes := dos.selectNodes(len(chunks))

	if len(selectedNodes) < len(chunks) {
		return fmt.Errorf("not enough nodes available")
	}

	// Store chunks
	locations := make([]ChunkLocation, 0)
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		node := selectedNodes[i]
		chunkType := "data"
		if i >= dos.erasureCoder.dataChunks {
			chunkType = "parity"
		}

		wg.Add(1)
		go func(n *DataNode, chunkID int, chunkData []byte, cType string) {
			defer wg.Done()

			dataChunk := &DataChunk{
				ObjectID: key,
				ChunkID:  chunkID,
				Data:     chunkData,
				Checksum: calculateChecksum(chunkData),
				Size:     int64(len(chunkData)),
			}

			n.mu.Lock()
			n.Objects[fmt.Sprintf("%s-%d", key, chunkID)] = dataChunk
			n.Used += dataChunk.Size
			n.mu.Unlock()

			locations = append(locations, ChunkLocation{
				ChunkID: chunkID,
				NodeID:  n.ID,
				Type:    cType,
			})
		}(node, i, chunk, chunkType)
	}

	wg.Wait()

	// Store metadata
	dos.metadata.mu.Lock()
	dos.metadata.objects[key] = &ObjectMetadata{
		Key:          key,
		Size:         int64(len(data)),
		Chunks:       dos.erasureCoder.dataChunks,
		ParityChunks: dos.erasureCoder.parityChunks,
		Created:      time.Now(),
		Checksum:     checksum,
	}
	dos.metadata.location[key] = locations
	dos.metadata.mu.Unlock()

	return nil
}

func (dos *DistributedObjectStorage) Get(ctx context.Context, key string) ([]byte, error) {
	dos.metadata.mu.RLock()
	metadata, exists := dos.metadata.objects[key]
	locations := dos.metadata.location[key]
	dos.metadata.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("object %s not found", key)
	}

	// Retrieve chunks
	chunks := make([][]byte, metadata.Chunks+metadata.ParityChunks)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, loc := range locations {
		wg.Add(1)
		go func(location ChunkLocation) {
			defer wg.Done()

			node := dos.getNode(location.NodeID)
			if node == nil || node.Status != "active" {
				return
			}

			node.mu.RLock()
			chunk, exists := node.Objects[fmt.Sprintf("%s-%d", key, location.ChunkID)]
			node.mu.RUnlock()

			if exists {
				mu.Lock()
				chunks[location.ChunkID] = chunk.Data
				mu.Unlock()
			}
		}(loc)
	}

	wg.Wait()

	// Reconstruct data from chunks
	data := dos.erasureCoder.decode(chunks, metadata.Chunks)

	// Verify checksum
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != metadata.Checksum {
		return nil, fmt.Errorf("checksum mismatch")
	}

	return data, nil
}

func (dos *DistributedObjectStorage) selectNodes(numChunks int) []*DataNode {
	selected := make([]*DataNode, 0, numChunks)

	// Simple round-robin selection
	for i := 0; i < numChunks && i < len(dos.nodes); i++ {
		node := dos.nodes[i%len(dos.nodes)]
		if node.Status == "active" && node.Capacity-node.Used > 0 {
			selected = append(selected, node)
		}
	}

	return selected
}

func (dos *DistributedObjectStorage) getNode(nodeID string) *DataNode {
	for _, node := range dos.nodes {
		if node.ID == nodeID {
			return node
		}
	}
	return nil
}

func (ec *ErasureCoder) encode(data []byte) [][]byte {
	chunkSize := len(data) / ec.dataChunks
	if len(data)%ec.dataChunks != 0 {
		chunkSize++
	}

	chunks := make([][]byte, ec.dataChunks+ec.parityChunks)

	// Create data chunks
	for i := 0; i < ec.dataChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunk := make([]byte, chunkSize)
		if start < len(data) {
			copy(chunk, data[start:end])
		}
		chunks[i] = chunk
	}

	// Create parity chunks (simplified XOR)
	for i := 0; i < ec.parityChunks; i++ {
		parity := make([]byte, chunkSize)
		for j := 0; j < ec.dataChunks; j++ {
			for k := 0; k < chunkSize && k < len(chunks[j]); k++ {
				parity[k] ^= chunks[j][k]
			}
		}
		chunks[ec.dataChunks+i] = parity
	}

	return chunks
}

func (ec *ErasureCoder) decode(chunks [][]byte, dataChunks int) []byte {
	// Simplified decoding - just concatenate data chunks
	var buffer bytes.Buffer

	for i := 0; i < dataChunks; i++ {
		if chunks[i] != nil {
			buffer.Write(chunks[i])
		}
	}

	return buffer.Bytes()
}

func (dos *DistributedObjectStorage) runRecovery() {
	for range dos.recovery.ticker.C {
		if !atomic.CompareAndSwapInt32(&dos.recovery.running, 0, 1) {
			continue
		}

		dos.checkAndRecover()

		atomic.StoreInt32(&dos.recovery.running, 0)
	}
}

func (dos *DistributedObjectStorage) checkAndRecover() {
	// Check for failed nodes and recover data
	for _, node := range dos.nodes {
		if node.Status == "failed" {
			log.Printf("Recovering data from failed node %s", node.ID)
			// Actual recovery logic would go here
		}
	}
}

// Helper functions
func generateVersionID() string {
	return fmt.Sprintf("v-%d", time.Now().UnixNano())
}

func generateUploadID() string {
	return fmt.Sprintf("upload-%d", time.Now().UnixNano())
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func calculateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// RunSolution24 demonstrates the three object storage solutions
func RunSolution24() {
	fmt.Println("=== Solution 24: Object Storage ===")

	ctx := context.Background()

	// Solution 1: S3-Compatible Storage
	fmt.Println("\n1. S3-Compatible Object Storage:")
	s3 := NewS3CompatibleStorage()

	// Create bucket
	err := s3.CreateBucket("my-bucket")
	if err != nil {
		fmt.Printf("  Create bucket error: %v\n", err)
	}

	// Put object
	data := []byte("Hello, World!")
	metadata := map[string]string{
		"Content-Type": "text/plain",
		"Author":       "System",
	}

	obj, err := s3.PutObject("my-bucket", "hello.txt", data, metadata)
	if err != nil {
		fmt.Printf("  Put object error: %v\n", err)
	} else {
		fmt.Printf("  Stored object: %s (ETag: %s)\n", obj.Key, obj.ETag)
	}

	// Multipart upload
	uploadID, _ := s3.InitiateMultipartUpload("my-bucket", "large-file.dat")

	// Upload parts
	for i := 1; i <= 3; i++ {
		partData := []byte(fmt.Sprintf("Part %d data", i))
		etag, _ := s3.UploadPart(uploadID, i, partData)
		fmt.Printf("  Uploaded part %d (ETag: %s)\n", i, etag)
	}

	// Complete multipart
	multipartObj, err := s3.CompleteMultipartUpload(uploadID, []int{1, 2, 3})
	if err != nil {
		fmt.Printf("  Complete multipart error: %v\n", err)
	} else {
		fmt.Printf("  Completed multipart upload: %s\n", multipartObj.Key)
	}

	// List objects
	objects, _ := s3.ListObjects("my-bucket", "", 10)
	fmt.Printf("  Bucket contains %d objects\n", len(objects))

	// Solution 2: Content-Addressable Storage
	fmt.Println("\n2. Content-Addressable Storage:")
	cas := NewContentAddressableStorage()

	// Store objects with deduplication
	content1 := []byte("Duplicate content")
	content2 := []byte("Duplicate content") // Same content

	obj1, _ := cas.Store("file1.txt", content1)
	obj2, _ := cas.Store("file2.txt", content2)

	fmt.Printf("  Object 1 hash: %s\n", obj1.ContentHash)
	fmt.Printf("  Object 2 hash: %s\n", obj2.ContentHash)
	fmt.Printf("  Saved space: %d bytes\n", cas.dedup.savedSpace)

	// Retrieve object
	retrieved, err := cas.Retrieve("file1.txt")
	if err != nil {
		fmt.Printf("  Retrieve error: %v\n", err)
	} else {
		fmt.Printf("  Retrieved %d bytes\n", len(retrieved))
	}

	// Solution 3: Distributed Object Storage
	fmt.Println("\n3. Distributed Object Storage with Erasure Coding:")
	dos := NewDistributedObjectStorage(6)

	// Store object with erasure coding
	objectData := []byte("This is distributed data with erasure coding for fault tolerance")
	reader := bytes.NewReader(objectData)

	err = dos.Put(ctx, "distributed.dat", reader)
	if err != nil {
		fmt.Printf("  Put error: %v\n", err)
	} else {
		fmt.Printf("  Stored with %d data + %d parity chunks\n",
			dos.erasureCoder.dataChunks, dos.erasureCoder.parityChunks)
	}

	// Retrieve object
	retrieved2, err := dos.Get(ctx, "distributed.dat")
	if err != nil {
		fmt.Printf("  Get error: %v\n", err)
	} else {
		fmt.Printf("  Retrieved %d bytes successfully\n", len(retrieved2))
	}

	// Show node distribution
	fmt.Printf("\n  Node Status:\n")
	for _, node := range dos.nodes[:3] {
		fmt.Printf("    Node %s: %d/%d bytes used\n",
			node.ID, node.Used, node.Capacity)
	}

	fmt.Println("\nAll object storage patterns demonstrated successfully!")
}