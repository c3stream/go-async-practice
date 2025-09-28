package solutions

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge15_DistributedCacheSolution - 分散キャッシュ問題の解決
func Challenge15_DistributedCacheSolution() {
	fmt.Println("\n✅ チャレンジ15: 分散キャッシュ問題の解決")
	fmt.Println("===================================================")

	// 解決策1: キャッシュスタンピード対策（Singleflight）
	solution1_CacheStampedePrevention()

	// 解決策2: ホットキー対策（ローカルキャッシュ + レプリケーション）
	solution2_HotKeyHandling()

	// 解決策3: 一貫性保証（Write-through + Invalidation）
	solution3_ConsistencyGuarantee()
}

// 解決策1: キャッシュスタンピード対策
func solution1_CacheStampedePrevention() {
	fmt.Println("\n📝 解決策1: キャッシュスタンピード対策（Singleflight）")

	type LoadingKey struct {
		mu      sync.Mutex
		loading bool
		waiters []chan interface{}
	}

	type CacheEntry struct {
		Value  interface{}
		Expiry time.Time
	}

	type SafeCache struct {
		mu          sync.RWMutex
		data        map[string]*CacheEntry
		loadingKeys map[string]*LoadingKey
		loadFunc    func(string) (interface{}, error)
		defaultTTL  time.Duration
	}

	cache := &SafeCache{
		data:        make(map[string]*CacheEntry),
		loadingKeys: make(map[string]*LoadingKey),
		loadFunc: func(key string) (interface{}, error) {
			// データベースからのロードをシミュレート
			time.Sleep(100 * time.Millisecond)
			return fmt.Sprintf("value_%s", key), nil
		},
		defaultTTL: 5 * time.Second,
	}

	// Singleflightパターンによる重複リクエスト防止
	cacheGet := func(ctx context.Context, key string) (interface{}, error) {
		// 既存のキャッシュをチェック
		cache.mu.RLock()
		if entry, exists := cache.data[key]; exists && time.Now().Before(entry.Expiry) {
			cache.mu.RUnlock()
			return entry.Value, nil
		}
		cache.mu.RUnlock()

		// ロード中のキーを取得または作成
		cache.mu.Lock()
		loadingKey, exists := cache.loadingKeys[key]
		if !exists {
			loadingKey = &LoadingKey{
				loading: false,
				waiters: make([]chan interface{}, 0),
			}
			cache.loadingKeys[key] = loadingKey
		}
		cache.mu.Unlock()

		loadingKey.mu.Lock()

		// 既にロード中の場合、待機
		if loadingKey.loading {
			waiter := make(chan interface{}, 1)
			loadingKey.waiters = append(loadingKey.waiters, waiter)
			loadingKey.mu.Unlock()

			fmt.Printf("  ⏳ %s: ロード待機中...\n", key)

			select {
			case value := <-waiter:
				return value, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// このgoroutineがロードを担当
		loadingKey.loading = true
		waiters := loadingKey.waiters
		loadingKey.waiters = make([]chan interface{}, 0)
		loadingKey.mu.Unlock()

		fmt.Printf("  🔄 %s: データロード開始\n", key)

		// データロード
		value, err := cache.loadFunc(key)
		if err != nil {
			loadingKey.mu.Lock()
			loadingKey.loading = false
			loadingKey.mu.Unlock()
			return nil, err
		}

		// キャッシュに保存
		cache.mu.Lock()
		cache.data[key] = &CacheEntry{
			Value:  value,
			Expiry: time.Now().Add(cache.defaultTTL),
		}
		cache.mu.Unlock()

		// 待機中のリクエストに通知
		for _, waiter := range waiters {
			waiter <- value
			close(waiter)
		}

		loadingKey.mu.Lock()
		loadingKey.loading = false
		delete(cache.loadingKeys, key)
		loadingKey.mu.Unlock()

		fmt.Printf("  ✓ %s: データロード完了\n", key)

		return value, nil
	}

	// テスト: 同時アクセス
	fmt.Println("\n実行: 10個の同時リクエスト")
	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			value, err := cacheGet(ctx, "popular_item")
			if err != nil {
				fmt.Printf("  ❌ Request %d: エラー\n", id)
			} else {
				fmt.Printf("  ✓ Request %d: %v\n", id, value)
			}
		}(i)
	}

	wg.Wait()
	fmt.Println("  ✅ 1回のロードで全リクエストに対応")
}

// 解決策2: ホットキー対策
func solution2_HotKeyHandling() {
	fmt.Println("\n📝 解決策2: ホットキー対策（ローカルレプリカ）")

	type HotKeyCache struct {
		globalCache  sync.Map // グローバルキャッシュ
		localReplica sync.Map // ホットキーのローカルレプリカ
		hotKeys      sync.Map // ホットキー検出
		threshold    int64
	}

	cache := &HotKeyCache{
		threshold: 100, // 100回以上アクセスでホットキー判定
	}

	// アクセスカウンターの更新
	cacheIncrementAccess := func(key string) int64 {
		actual, _ := cache.hotKeys.LoadOrStore(key, new(int64))
		counter := actual.(*int64)
		newCount := atomic.AddInt64(counter, 1)

		// ホットキー判定
		if newCount == cache.threshold {
			fmt.Printf("  🔥 ホットキー検出: %s (アクセス数: %d)\n", key, newCount)

			// グローバルキャッシュからローカルレプリカに昇格
			if value, ok := cache.globalCache.Load(key); ok {
				cache.localReplica.Store(key, value)
				fmt.Printf("  📋 %s をローカルレプリカに昇格\n", key)
			}
		}

		return newCount
	}

	// ホットキー対応のGet
	cacheGet := func(key string) (interface{}, bool) {
		// ローカルレプリカを優先チェック（ロック不要）
		if value, ok := cache.localReplica.Load(key); ok {
			cacheIncrementAccess(key)
			return value, true
		}

		// グローバルキャッシュをチェック
		if value, ok := cache.globalCache.Load(key); ok {
			cacheIncrementAccess(key)
			return value, true
		}

		return nil, false
	}

	// データ設定
	cacheSet := func(key string, value interface{}) {
		cache.globalCache.Store(key, value)

		// 既にホットキーの場合、ローカルレプリカも更新
		if _, isHot := cache.localReplica.Load(key); isHot {
			cache.localReplica.Store(key, value)
		}
	}

	// テスト実行
	fmt.Println("\n実行: ホットキーアクセステスト")

	// データ準備
	cacheSet("hot_item", "Very Popular Item")
	cacheSet("normal_item", "Normal Item")

	// 大量アクセスをシミュレート
	var wg sync.WaitGroup
	for i := 0; i < 150; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if value, ok := cacheGet("hot_item"); ok {
				if id%30 == 0 {
					fmt.Printf("  ✓ Access %d: %v\n", id, value)
				}
			}
		}(i)
	}

	wg.Wait()
	fmt.Println("  ✅ ホットキーをローカルレプリカで高速処理")
}

// 解決策3: 一貫性保証
func solution3_ConsistencyGuarantee() {
	fmt.Println("\n📝 解決策3: 一貫性保証（Write-through + Invalidation）")

	type CacheEntry struct {
		Value   interface{}
		Version int64
		Expiry  time.Time
	}

	type ConsistentCache struct {
		mu          sync.RWMutex
		cache       map[string]*CacheEntry
		dataStore   map[string]interface{} // 実データストア
		subscribers []chan string          // 無効化通知の購読者
	}

	cache := &ConsistentCache{
		cache:       make(map[string]*CacheEntry),
		dataStore:   make(map[string]interface{}),
		subscribers: make([]chan string, 0),
	}

	// 購読者登録（キャッシュノード間の通信用）
	cacheSubscribe := func() chan string {
		cache.mu.Lock()
		defer cache.mu.Unlock()

		ch := make(chan string, 100)
		cache.subscribers = append(cache.subscribers, ch)
		return ch
	}

	// Write-through戦略
	cacheSet := func(key string, value interface{}) error {
		cache.mu.Lock()

		// 1. データストアに書き込み
		cache.dataStore[key] = value

		// 2. キャッシュを更新
		version := time.Now().UnixNano()
		cache.cache[key] = &CacheEntry{
			Value:   value,
			Version: version,
			Expiry:  time.Now().Add(5 * time.Second),
		}

		cache.mu.Unlock()

		// 3. 他のノードに無効化通知
		cache.mu.RLock()
		subscribers := make([]chan string, len(cache.subscribers))
		copy(subscribers, cache.subscribers)
		cache.mu.RUnlock()

		for _, sub := range subscribers {
			select {
			case sub <- key:
			default:
				// ノンブロッキング送信
			}
		}

		fmt.Printf("  ✓ Write-through: %s = %v\n", key, value)
		return nil
	}

	// Read with validation
	cacheGet := func(key string) (interface{}, bool) {
		cache.mu.RLock()
		entry, exists := cache.cache[key]
		cache.mu.RUnlock()

		if !exists || time.Now().After(entry.Expiry) {
			// キャッシュミス: データストアから読み込み
			cache.mu.RLock()
			value, ok := cache.dataStore[key]
			cache.mu.RUnlock()

			if ok {
				// キャッシュに追加
				cache.mu.Lock()
				cache.cache[key] = &CacheEntry{
					Value:   value,
					Version: time.Now().UnixNano(),
					Expiry:  time.Now().Add(5 * time.Second),
				}
				cache.mu.Unlock()

				fmt.Printf("  🔄 Cache miss -> Load from store: %s\n", key)
			}

			return value, ok
		}

		return entry.Value, true
	}

	// 無効化リスナー（他ノードからの通知処理）
	_ = func(invalidations chan string) {
		for key := range invalidations {
			cache.mu.Lock()
			delete(cache.cache, key)
			cache.mu.Unlock()
			fmt.Printf("  🗑️  Invalidated: %s\n", key)
		}
	}

	// テスト実行
	fmt.Println("\n実行: 一貫性保証テスト")

	// 購読者（他ノード）を作成
	node2Invalidations := cacheSubscribe()
	go func() {
		for key := range node2Invalidations {
			fmt.Printf("  📡 Node2 received invalidation: %s\n", key)
		}
	}()

	// Write-throughテスト
	cacheSet("user_123", map[string]string{"name": "Alice", "email": "alice@example.com"})

	time.Sleep(50 * time.Millisecond)

	// 読み取りテスト
	if value, ok := cacheGet("user_123"); ok {
		fmt.Printf("  ✓ Read: %v\n", value)
	}

	// 更新テスト
	cacheSet("user_123", map[string]string{"name": "Alice", "email": "alice.new@example.com"})

	time.Sleep(50 * time.Millisecond)

	fmt.Println("  ✅ データストアとキャッシュの一貫性を保証")
}