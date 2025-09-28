package tests

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestSingleflightPattern - Singleflightパターンのテスト
func TestSingleflightPattern(t *testing.T) {
	t.Run("CacheStampedePrevention", func(t *testing.T) {
		var loadCount int64
		var cacheMu sync.Mutex
		cache := make(map[string]interface{})

		// 簡略化したSingleflight実装
		loadOnce := sync.Once{}

		get := func(key string) (interface{}, error) {
			cacheMu.Lock()
			if val, exists := cache[key]; exists {
				cacheMu.Unlock()
				return val, nil
			}
			cacheMu.Unlock()

			// sync.Onceで1回だけロード
			var loadedValue interface{}
			loadOnce.Do(func() {
				atomic.AddInt64(&loadCount, 1)
				time.Sleep(10 * time.Millisecond)
				loadedValue = fmt.Sprintf("value_%s", key)

				cacheMu.Lock()
				cache[key] = loadedValue
				cacheMu.Unlock()
			})

			cacheMu.Lock()
			val := cache[key]
			cacheMu.Unlock()
			return val, nil
		}

		// テスト: 100個の同時リクエスト
		var wg sync.WaitGroup
		results := make([]interface{}, 100)

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				value, err := get("popular_item")
				if err != nil {
					t.Errorf("Request %d failed: %v", id, err)
					return
				}
				results[id] = value
			}(i)
		}

		wg.Wait()

		// 検証: ロード回数は1回のみ
		finalLoadCount := atomic.LoadInt64(&loadCount)
		if finalLoadCount != 1 {
			t.Errorf("Expected 1 load, got %d (cache stampede occurred)", finalLoadCount)
		}

		// 検証: 全てのリクエストが同じ値を受け取った
		expectedValue := "value_popular_item"
		for i, result := range results {
			if result == nil || result != expectedValue {
				t.Errorf("Request %d got wrong value: %v", i, result)
			}
		}

		t.Logf("✅ Successfully handled 100 concurrent requests with 1 load")
	})
}

// TestHotKeyDetection - ホットキー検出のテスト
func TestHotKeyDetection(t *testing.T) {
	t.Run("AutoPromoteToLocalReplica", func(t *testing.T) {
		type hotKeyCache struct {
			globalCache  sync.Map
			localReplica sync.Map
			hotKeys      sync.Map
			threshold    int64
		}

		cache := &hotKeyCache{
			threshold: 100,
		}

		// アクセスカウンター
		incrementAccess := func(key string) int64 {
			actual, _ := cache.hotKeys.LoadOrStore(key, new(int64))
			counter := actual.(*int64)
			newCount := atomic.AddInt64(counter, 1)

			// ホットキー判定
			if newCount == cache.threshold {
				if value, ok := cache.globalCache.Load(key); ok {
					cache.localReplica.Store(key, value)
				}
			}

			return newCount
		}

		// データ設定
		cache.globalCache.Store("hot_item", "Popular Data")

		// 150回アクセス
		var wg sync.WaitGroup
		for i := 0; i < 150; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				incrementAccess("hot_item")
			}()
		}
		wg.Wait()

		// 検証: ローカルレプリカに昇格している
		_, exists := cache.localReplica.Load("hot_item")
		if !exists {
			t.Error("Hot key was not promoted to local replica")
		}

		// 検証: アクセスカウントが正確
		actual, _ := cache.hotKeys.Load("hot_item")
		count := atomic.LoadInt64(actual.(*int64))
		if count != 150 {
			t.Errorf("Expected 150 accesses, got %d", count)
		}

		t.Logf("✅ Hot key detected and promoted after %d accesses", cache.threshold)
	})
}

// TestWriteThroughConsistency - Write-through一貫性のテスト
func TestWriteThroughConsistency(t *testing.T) {
	t.Run("ConsistentCacheInvalidation", func(t *testing.T) {
		type cacheEntry struct {
			Value   interface{}
			Version int64
		}

		type consistentCache struct {
			mu          sync.RWMutex
			cache       map[string]*cacheEntry
			dataStore   map[string]interface{}
			subscribers []chan string
		}

		cache := &consistentCache{
			cache:       make(map[string]*cacheEntry),
			dataStore:   make(map[string]interface{}),
			subscribers: make([]chan string, 0),
		}

		// 購読者登録
		subscribe := func() chan string {
			cache.mu.Lock()
			defer cache.mu.Unlock()
			ch := make(chan string, 100)
			cache.subscribers = append(cache.subscribers, ch)
			return ch
		}

		// Write-through
		set := func(key string, value interface{}) error {
			cache.mu.Lock()

			// 1. データストアに書き込み
			cache.dataStore[key] = value

			// 2. キャッシュを更新
			version := time.Now().UnixNano()
			cache.cache[key] = &cacheEntry{
				Value:   value,
				Version: version,
			}

			subscribers := cache.subscribers
			cache.mu.Unlock()

			// 3. 他のノードに無効化通知
			for _, sub := range subscribers {
				select {
				case sub <- key:
				default:
				}
			}

			return nil
		}

		// 読み取り
		get := func(key string) (interface{}, bool) {
			cache.mu.RLock()
			entry, exists := cache.cache[key]
			cache.mu.RUnlock()

			if !exists {
				cache.mu.RLock()
				value, ok := cache.dataStore[key]
				cache.mu.RUnlock()
				return value, ok
			}

			return entry.Value, true
		}

		// テスト: Write-through動作
		node2Invalidations := subscribe()
		invalidatedKeys := []string{}
		var invMu sync.Mutex

		// 無効化通知の受信
		go func() {
			for key := range node2Invalidations {
				invMu.Lock()
				invalidatedKeys = append(invalidatedKeys, key)
				invMu.Unlock()
			}
		}()

		// データ書き込み
		err := set("user_123", map[string]string{"name": "Alice"})
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		// 検証: キャッシュと データストアの一貫性
		cachedValue, exists := get("user_123")
		if !exists {
			t.Error("Value not found in cache")
		}

		cache.mu.RLock()
		storeValue := cache.dataStore["user_123"]
		cache.mu.RUnlock()

		if fmt.Sprint(cachedValue) != fmt.Sprint(storeValue) {
			t.Error("Cache and data store are inconsistent")
		}

		// 検証: 無効化通知が送信された
		invMu.Lock()
		notified := len(invalidatedKeys) > 0
		invMu.Unlock()

		if !notified {
			t.Error("Invalidation notification was not sent")
		}

		close(node2Invalidations)
		t.Log("✅ Write-through cache consistency verified")
	})
}

// BenchmarkSingleflight - Singleflightパターンのベンチマーク
func BenchmarkSingleflight(b *testing.B) {
	cache := make(map[string]interface{})
	cacheMu := sync.RWMutex{}
	loadingKeys := make(map[string]*struct {
		mu      sync.Mutex
		loading bool
		waiters []chan interface{}
	})
	loadingMu := sync.Mutex{}

	loadFunc := func(key string) (interface{}, error) {
		time.Sleep(time.Millisecond)
		return fmt.Sprintf("value_%s", key), nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx := context.Background()
			key := "benchmark_key"

			// 簡易Singleflight実装
			cacheMu.RLock()
			if val, exists := cache[key]; exists {
				cacheMu.RUnlock()
				_ = val
				continue
			}
			cacheMu.RUnlock()

			loadingMu.Lock()
			lk, exists := loadingKeys[key]
			if !exists {
				lk = &struct {
					mu      sync.Mutex
					loading bool
					waiters []chan interface{}
				}{
					loading: false,
					waiters: make([]chan interface{}, 0),
				}
				loadingKeys[key] = lk
			}
			loadingMu.Unlock()

			lk.mu.Lock()
			if lk.loading {
				waiter := make(chan interface{}, 1)
				lk.waiters = append(lk.waiters, waiter)
				lk.mu.Unlock()
				select {
				case <-waiter:
				case <-ctx.Done():
				}
				continue
			}

			lk.loading = true
			lk.mu.Unlock()

			value, _ := loadFunc(key)

			cacheMu.Lock()
			cache[key] = value
			cacheMu.Unlock()

			lk.mu.Lock()
			for _, w := range lk.waiters {
				w <- value
				close(w)
			}
			lk.loading = false
			lk.mu.Unlock()
		}
	})
}