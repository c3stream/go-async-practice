package challenges

import (
	"context"
	"fmt"
	"hash/fnv"
	"sync"
	"time"
)

// Challenge 15: 分散キャッシュの問題
//
// 問題点:
// 1. キャッシュスタンピード（大量同時アクセス）
// 2. ホットキー問題（特定キーへの集中アクセス）
// 3. 一貫性の欠如（キャッシュと実データの不整合）
// 4. メモリリークとエビクション戦略の不備

type CacheNode struct {
	mu    sync.RWMutex
	data  map[string]*CacheEntry
	stats map[string]*AccessStats
}

type CacheEntry struct {
	Value      interface{}
	Expiry     time.Time
	Loading    bool
	LoadingMu  sync.Mutex
}

type AccessStats struct {
	Count     int64
	LastAccess time.Time
}

type DistributedCache struct {
	nodes         []*CacheNode
	numNodes      int
	loadFunc      func(string) (interface{}, error)
	defaultExpiry time.Duration
}

// BUG: キャッシュスタンピード問題
func (dc *DistributedCache) Get(ctx context.Context, key string) (interface{}, error) {
	node := dc.getNode(key)

	node.mu.RLock()
	entry, exists := node.data[key]
	node.mu.RUnlock()

	if !exists || time.Now().After(entry.Expiry) {
		// 問題: 複数のgoroutineが同時にキャッシュミスを検出
		// 全てが同じデータをロードしようとする

		// データロード
		value, err := dc.loadFunc(key)
		if err != nil {
			return nil, err
		}

		// 問題: ロード中の重複チェックなし
		node.mu.Lock()
		node.data[key] = &CacheEntry{
			Value:  value,
			Expiry: time.Now().Add(dc.defaultExpiry),
		}
		node.mu.Unlock()

		return value, nil
	}

	// BUG: ホットキー問題 - アクセス統計の更新で競合
	node.mu.Lock()
	if stats, ok := node.stats[key]; ok {
		stats.Count++
		stats.LastAccess = time.Now()
	} else {
		node.stats[key] = &AccessStats{
			Count:      1,
			LastAccess: time.Now(),
		}
	}
	node.mu.Unlock()

	return entry.Value, nil
}

// BUG: 一貫性の欠如
func (dc *DistributedCache) Set(key string, value interface{}, ttl time.Duration) error {
	node := dc.getNode(key)

	// 問題: 他のノードへの伝播なし
	node.mu.Lock()
	node.data[key] = &CacheEntry{
		Value:  value,
		Expiry: time.Now().Add(ttl),
	}
	node.mu.Unlock()

	// 問題: Write-through/Write-backの実装なし
	// データストアとの同期が保証されない

	return nil
}

// BUG: 不適切な削除処理
func (dc *DistributedCache) Delete(key string) error {
	node := dc.getNode(key)

	// 問題: 削除の伝播なし
	node.mu.Lock()
	delete(node.data, key)
	delete(node.stats, key)
	node.mu.Unlock()

	// 問題: 削除中の読み取りで古いデータが返される可能性

	return nil
}

// BUG: メモリリークとエビクション戦略の不備
func (dc *DistributedCache) Evict() {
	for _, node := range dc.nodes {
		node.mu.Lock()

		// 問題: 期限切れエントリの削除のみ
		// LRU/LFUなどの適切なエビクション戦略なし
		now := time.Now()
		for key, entry := range node.data {
			if now.After(entry.Expiry) {
				delete(node.data, key)
				// 問題: statsの削除忘れ（メモリリーク）
			}
		}

		node.mu.Unlock()
	}
}

// BUG: ホットキー検出が不完全
func (dc *DistributedCache) GetHotKeys(threshold int64) []string {
	var hotKeys []string

	// 問題: 全ノードを同時にロック（パフォーマンス問題）
	for _, node := range dc.nodes {
		node.mu.RLock()
		for key, stats := range node.stats {
			if stats.Count > threshold {
				hotKeys = append(hotKeys, key)
			}
		}
		node.mu.RUnlock()
	}

	return hotKeys
}

func (dc *DistributedCache) getNode(key string) *CacheNode {
	h := fnv.New32a()
	h.Write([]byte(key))
	return dc.nodes[h.Sum32()%uint32(dc.numNodes)]
}

func Challenge15_DistributedCacheProblem() {
	fmt.Println("Challenge 15: 分散キャッシュの問題")
	fmt.Println("問題:")
	fmt.Println("1. キャッシュスタンピード（大量同時アクセス）")
	fmt.Println("2. ホットキー問題（特定キーへの集中アクセス）")
	fmt.Println("3. 一貫性の欠如")
	fmt.Println("4. メモリリークとエビクション戦略の不備")

	// キャッシュ作成
	cache := &DistributedCache{
		numNodes:      3,
		defaultExpiry: 5 * time.Second,
		loadFunc: func(key string) (interface{}, error) {
			// 重いデータロード処理のシミュレーション
			time.Sleep(500 * time.Millisecond)
			return fmt.Sprintf("data-%s", key), nil
		},
	}

	// ノード初期化
	for i := 0; i < cache.numNodes; i++ {
		cache.nodes = append(cache.nodes, &CacheNode{
			data:  make(map[string]*CacheEntry),
			stats: make(map[string]*AccessStats),
		})
	}

	ctx := context.Background()

	// キャッシュスタンピードのシミュレーション
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 全て同じキーにアクセス
			cache.Get(ctx, "popular-key")
		}()
	}

	wg.Wait()

	// ホットキー検出
	hotKeys := cache.GetHotKeys(50)
	fmt.Printf("ホットキー: %v\n", hotKeys)

	// エビクション実行
	cache.Evict()

	fmt.Println("\n修正が必要な箇所を特定してください")
}