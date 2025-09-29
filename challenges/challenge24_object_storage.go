package challenges

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Challenge 24: Object Storage System (S3-like)
//
// 問題点:
// 1. マルチパート上传の並行性問題
// 2. メタデータ管理の不整合
// 3. 容量制限とクォータ管理の不備
// 4. 並行削除時のデータ不整合
// 5. バージョニングとライフサイクル管理の問題

type ObjectMetadata struct {
	Key          string
	Size         int64
	ContentType  string
	ETag         string
	LastModified time.Time
	Version      string
	Tags         map[string]string
	ACL          *AccessControlList
	StorageClass string
	Encryption   string
}

type Object struct {
	Metadata ObjectMetadata
	Data     []byte
	mu       sync.RWMutex
}

type Bucket struct {
	Name         string
	Objects      map[string]*Object
	Versions     map[string][]*Object // 問題: メモリリーク
	mu           sync.RWMutex
	CreatedAt    time.Time
	Region       string
	ACL          *AccessControlList
	Versioning   bool
	Lifecycle    *LifecyclePolicy
	Usage        *BucketUsage
}

type BucketUsage struct {
	ObjectCount int64
	TotalSize   int64
	mu          sync.RWMutex
}

type AccessControlList struct {
	Owner       string
	Permissions map[string][]string // user -> permissions
	mu          sync.RWMutex
}

type MultipartUpload struct {
	UploadID    string
	Bucket      string
	Key         string
	Parts       map[int]*Part
	InitiatedAt time.Time
	mu          sync.RWMutex
	completed   bool
}

type Part struct {
	PartNumber int
	ETag       string
	Size       int64
	Data       []byte
	UploadedAt time.Time
}

type LifecyclePolicy struct {
	Rules []LifecycleRule
	mu    sync.RWMutex
}

type LifecycleRule struct {
	ID         string
	Prefix     string
	Enabled    bool
	Expiration int // days
	Transition StorageTransition
}

type StorageTransition struct {
	Days         int
	StorageClass string
}

type ObjectStorageSystem struct {
	mu              sync.RWMutex
	buckets         map[string]*Bucket
	uploads         map[string]*MultipartUpload
	replicationTargets map[string]string // bucket -> target region
	quotas          map[string]*Quota
	cache           sync.Map // 問題: 無制限キャッシュ
	metrics         *StorageMetrics
}

type Quota struct {
	MaxObjects int64
	MaxSize    int64
	Used       int64
	mu         sync.RWMutex
}

type StorageMetrics struct {
	TotalRequests int64
	TotalBytes    int64
	Errors        map[string]int64
	mu            sync.RWMutex
}

func NewObjectStorageSystem() *ObjectStorageSystem {
	return &ObjectStorageSystem{
		buckets:            make(map[string]*Bucket),
		uploads:            make(map[string]*MultipartUpload),
		replicationTargets: make(map[string]string),
		quotas:             make(map[string]*Quota),
		metrics: &StorageMetrics{
			Errors: make(map[string]int64),
		},
	}
}

// 問題1: バケット作成の競合状態
func (oss *ObjectStorageSystem) CreateBucket(name, region string) error {
	// 問題: チェックと作成が別操作
	if _, exists := oss.buckets[name]; exists {
		return fmt.Errorf("bucket already exists")
	}

	bucket := &Bucket{
		Name:      name,
		Objects:   make(map[string]*Object),
		Versions:  make(map[string][]*Object),
		CreatedAt: time.Now(),
		Region:    region,
		ACL: &AccessControlList{
			Owner:       "default",
			Permissions: make(map[string][]string),
		},
		Usage: &BucketUsage{},
	}

	// 問題: ロックなしで書き込み
	oss.buckets[name] = bucket

	return nil
}

// 問題2: オブジェクト上传
func (oss *ObjectStorageSystem) PutObject(bucketName, key string, data []byte, metadata ObjectMetadata) error {
	oss.mu.RLock()
	bucket, exists := oss.buckets[bucketName]
	oss.mu.RUnlock()

	if !exists {
		return fmt.Errorf("bucket not found")
	}

	// 問題: クォータチェックなし

	// ETag計算
	hash := md5.Sum(data)
	metadata.ETag = hex.EncodeToString(hash[:])
	metadata.Key = key
	metadata.Size = int64(len(data))
	metadata.LastModified = time.Now()

	obj := &Object{
		Metadata: metadata,
		Data:     data,
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// 問題: バージョニング処理が不完全
	if bucket.Versioning {
		// 問題: バージョンIDの生成が単純すぎる
		obj.Metadata.Version = fmt.Sprintf("%d", time.Now().Unix())

		if oldVersions, exists := bucket.Versions[key]; exists {
			// 問題: 無制限にバージョンが蓄積
			bucket.Versions[key] = append([]*Object{obj}, oldVersions...)
		} else {
			bucket.Versions[key] = []*Object{obj}
		}
	}

	// 問題: 古いオブジェクトの削除なし
	bucket.Objects[key] = obj

	// 問題: Usage更新が不完全
	bucket.Usage.ObjectCount++
	bucket.Usage.TotalSize += metadata.Size

	return nil
}

// 問題3: マルチパート上传
func (oss *ObjectStorageSystem) InitiateMultipartUpload(bucketName, key string) (string, error) {
	oss.mu.RLock()
	_, exists := oss.buckets[bucketName]
	oss.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("bucket not found")
	}

	uploadID := fmt.Sprintf("upload-%d", time.Now().UnixNano())

	upload := &MultipartUpload{
		UploadID:    uploadID,
		Bucket:      bucketName,
		Key:         key,
		Parts:       make(map[int]*Part),
		InitiatedAt: time.Now(),
	}

	oss.mu.Lock()
	oss.uploads[uploadID] = upload
	oss.mu.Unlock()

	// 問題: タイムアウト処理なし

	return uploadID, nil
}

func (oss *ObjectStorageSystem) UploadPart(uploadID string, partNumber int, data []byte) (string, error) {
	oss.mu.RLock()
	upload, exists := oss.uploads[uploadID]
	oss.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("upload not found")
	}

	// 問題: 完了済みチェックなし
	if upload.completed {
		return "", fmt.Errorf("upload already completed")
	}

	hash := md5.Sum(data)
	etag := hex.EncodeToString(hash[:])

	part := &Part{
		PartNumber: partNumber,
		ETag:       etag,
		Size:       int64(len(data)),
		Data:       data, // 問題: データコピーなし
		UploadedAt: time.Now(),
	}

	upload.mu.Lock()
	// 問題: 重複パート番号チェックなし
	upload.Parts[partNumber] = part
	upload.mu.Unlock()

	return etag, nil
}

func (oss *ObjectStorageSystem) CompleteMultipartUpload(uploadID string, partETags map[int]string) error {
	oss.mu.RLock()
	upload, exists := oss.uploads[uploadID]
	oss.mu.RUnlock()

	if !exists {
		return fmt.Errorf("upload not found")
	}

	upload.mu.Lock()
	defer upload.mu.Unlock()

	// 問題: パート検証が不完全
	var finalData []byte
	for i := 1; i <= len(partETags); i++ {
		part, exists := upload.Parts[i]
		if !exists {
			return fmt.Errorf("part %d missing", i)
		}

		// 問題: ETag検証なし

		finalData = append(finalData, part.Data...)
	}

	// 問題: 最終オブジェクト作成処理が不完全
	metadata := ObjectMetadata{
		Key:          upload.Key,
		Size:         int64(len(finalData)),
		LastModified: time.Now(),
	}

	// 問題: PutObjectを再利用せず、直接書き込み
	oss.PutObject(upload.Bucket, upload.Key, finalData, metadata)

	// 問題: アップロードのクリーンアップなし
	// delete(oss.uploads, uploadID) を忘れている

	upload.completed = true

	return nil
}

// 問題4: オブジェクト削除
func (oss *ObjectStorageSystem) DeleteObject(bucketName, key string) error {
	oss.mu.RLock()
	bucket, exists := oss.buckets[bucketName]
	oss.mu.RUnlock()

	if !exists {
		return fmt.Errorf("bucket not found")
	}

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	obj, exists := bucket.Objects[key]
	if !exists {
		return fmt.Errorf("object not found")
	}

	// 問題: バージョニング時の削除マーカー処理なし
	if bucket.Versioning {
		// 削除マーカーを追加すべき
	}

	// 問題: Usage更新が不完全
	bucket.Usage.ObjectCount--
	bucket.Usage.TotalSize -= obj.Metadata.Size

	delete(bucket.Objects, key)

	// 問題: バージョン履歴のクリーンアップなし

	return nil
}

// 問題5: オブジェクトのコピー
func (oss *ObjectStorageSystem) CopyObject(srcBucket, srcKey, dstBucket, dstKey string) error {
	// 問題: ソースとデスティネーションが同じ場合の処理なし

	oss.mu.RLock()
	src, srcExists := oss.buckets[srcBucket]
	dst, dstExists := oss.buckets[dstBucket]
	oss.mu.RUnlock()

	if !srcExists || !dstExists {
		return fmt.Errorf("bucket not found")
	}

	// 問題: デッドロックの可能性（ロック順序）
	src.mu.RLock()
	srcObj, exists := src.Objects[srcKey]
	src.mu.RUnlock()

	if !exists {
		return fmt.Errorf("source object not found")
	}

	// 問題: メタデータのディープコピーなし
	newObj := &Object{
		Metadata: srcObj.Metadata,
		Data:     srcObj.Data, // 問題: データの参照コピー
	}

	newObj.Metadata.Key = dstKey
	newObj.Metadata.LastModified = time.Now()

	dst.mu.Lock()
	dst.Objects[dstKey] = newObj
	dst.mu.Unlock()

	// 問題: Usage更新なし

	return nil
}

// 問題6: ライフサイクル処理
func (oss *ObjectStorageSystem) ProcessLifecycle(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Hour):
			oss.mu.RLock()
			buckets := make([]*Bucket, 0, len(oss.buckets))
			for _, b := range oss.buckets {
				buckets = append(buckets, b)
			}
			oss.mu.RUnlock()

			for _, bucket := range buckets {
				if bucket.Lifecycle == nil {
					continue
				}

				bucket.mu.Lock()
				// 問題: ライフサイクルルール処理が不完全
				for key, obj := range bucket.Objects {
					age := time.Since(obj.Metadata.LastModified).Hours() / 24

					for _, rule := range bucket.Lifecycle.Rules {
						if !rule.Enabled {
							continue
						}

						// 問題: プレフィックスマッチング未実装

						if int(age) > rule.Expiration {
							// 問題: 削除処理が直接実行（バッチ処理すべき）
							delete(bucket.Objects, key)
						}
					}
				}
				bucket.mu.Unlock()
			}
		}
	}
}

// 問題7: レプリケーション
func (oss *ObjectStorageSystem) ReplicateObject(bucketName, key, targetRegion string) error {
	// 問題: 非同期レプリケーションの実装が不完全
	oss.mu.RLock()
	bucket, exists := oss.buckets[bucketName]
	oss.mu.RUnlock()

	if !exists {
		return fmt.Errorf("bucket not found")
	}

	bucket.mu.RLock()
	obj, exists := bucket.Objects[key]
	bucket.mu.RUnlock()

	if !exists {
		return fmt.Errorf("object not found")
	}

	// 問題: レプリケーション先への実際の転送なし
	_ = obj
	_ = targetRegion

	// 問題: レプリケーション状態の追跡なし

	return nil
}

// Challenge: 以下の問題を修正してください
// 1. アトミックな操作保証
// 2. 適切なバージョニング
// 3. マルチパート上传の最適化
// 4. ライフサイクル管理
// 5. クォータと使用量管理
// 6. レプリケーション実装
// 7. 暗号化サポート
// 8. イベント通知システム

func RunChallenge24() {
	fmt.Println("Challenge 24: Object Storage System")
	fmt.Println("Fix the object storage implementation")

	oss := NewObjectStorageSystem()
	ctx := context.Background()

	// バケット作成
	oss.CreateBucket("my-bucket", "us-east-1")

	// シングル上传
	data := []byte("Hello, Object Storage!")
	metadata := ObjectMetadata{
		ContentType: "text/plain",
		Tags:        map[string]string{"env": "production"},
	}
	oss.PutObject("my-bucket", "file1.txt", data, metadata)

	// マルチパート上传
	uploadID, _ := oss.InitiateMultipartUpload("my-bucket", "large-file.bin")

	// パート上传（並行）
	var wg sync.WaitGroup
	partETags := make(map[int]string)
	mu := sync.Mutex{}

	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(partNum int) {
			defer wg.Done()
			partData := make([]byte, 1024*1024) // 1MB
			etag, err := oss.UploadPart(uploadID, partNum, partData)
			if err == nil {
				mu.Lock()
				partETags[partNum] = etag
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// マルチパート完了
	oss.CompleteMultipartUpload(uploadID, partETags)

	// オブジェクトコピー
	oss.CopyObject("my-bucket", "file1.txt", "my-bucket", "file1-copy.txt")

	// ライフサイクル処理
	go oss.ProcessLifecycle(ctx)

	fmt.Println("Object storage operations completed")
}