package solutions

import (
	"fmt"
	"sync"
	"time"
)

// Solution04_RateLimiter - レートリミッターの実装例
func Solution04_RateLimiter() {
	fmt.Println("=== Solution 04: Rate Limiter Implementations ===")

	// 解法1: time.Tickerを使用
	solution1 := func() {
		fmt.Println("\n解法1: time.Tickerでトークン生成")

		requests := make(chan int, 10)
		for i := 1; i <= 10; i++ {
			requests <- i
		}
		close(requests)

		// 1秒間に3リクエスト = 333ms間隔
		ticker := time.NewTicker(time.Second / 3)
		defer ticker.Stop()

		startTime := time.Now()

		for req := range requests {
			<-ticker.C // トークンを待つ
			fmt.Printf("処理中: リクエスト %d (経過時間: %v)\n",
				req, time.Since(startTime).Round(time.Millisecond))
		}
	}

	// 解法2: トークンバケット方式
	solution2 := func() {
		fmt.Println("\n解法2: トークンバケット実装")

		requests := make(chan int, 10)
		for i := 1; i <= 10; i++ {
			requests <- i
		}
		close(requests)

		// トークンバケット（容量3、1秒ごとに3トークン補充）
		bucket := make(chan struct{}, 3)

		// 初期トークンを追加
		for i := 0; i < 3; i++ {
			bucket <- struct{}{}
		}

		// トークン補充goroutine
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for range ticker.C {
				for i := 0; i < 3; i++ {
					select {
					case bucket <- struct{}{}:
					default:
						// バケットが満杯
					}
				}
			}
		}()

		startTime := time.Now()

		for req := range requests {
			<-bucket // トークンを消費
			fmt.Printf("処理中: リクエスト %d (経過時間: %v)\n",
				req, time.Since(startTime).Round(time.Millisecond))
		}
	}

	// 解法3: レート制限付きワーカープール
	solution3 := func() {
		fmt.Println("\n解法3: レート制限付きワーカープール")

		type RateLimiter struct {
			rate     int
			interval time.Duration
			tokens   chan struct{}
			stop     chan struct{}
			wg       sync.WaitGroup
		}

		NewRateLimiter := func(rate int, interval time.Duration) *RateLimiter {
			rl := &RateLimiter{
				rate:     rate,
				interval: interval,
				tokens:   make(chan struct{}, rate),
				stop:     make(chan struct{}),
			}

			// 初期トークン
			for i := 0; i < rate; i++ {
				rl.tokens <- struct{}{}
			}

			// トークン補充goroutine
			rl.wg.Add(1)
			go func() {
				defer rl.wg.Done()
				ticker := time.NewTicker(interval)
				defer ticker.Stop()

				for {
					select {
					case <-ticker.C:
						for i := 0; i < rl.rate; i++ {
							select {
							case rl.tokens <- struct{}{}:
							default:
							}
						}
					case <-rl.stop:
						return
					}
				}
			}()

			return rl
		}

		rl := NewRateLimiter(3, time.Second)

		requests := make(chan int, 10)
		for i := 1; i <= 10; i++ {
			requests <- i
		}
		close(requests)

		startTime := time.Now()

		for req := range requests {
			<-rl.tokens // トークンを消費
			fmt.Printf("処理中: リクエスト %d (経過時間: %v)\n",
				req, time.Since(startTime).Round(time.Millisecond))
		}

		close(rl.stop)
		rl.wg.Wait()
	}

	// 解法4: context.WithTimeoutを使った制限
	solution4 := func() {
		fmt.Println("\n解法4: Sliding Window Rate Limiter")

		type SlidingWindowLimiter struct {
			mu        sync.Mutex
			requests  []time.Time
			rate      int
			window    time.Duration
		}

		NewSlidingWindowLimiter := func(rate int, window time.Duration) *SlidingWindowLimiter {
			return &SlidingWindowLimiter{
				rate:   rate,
				window: window,
			}
		}

		Allow := func(swl *SlidingWindowLimiter) bool {
			swl.mu.Lock()
			defer swl.mu.Unlock()

			now := time.Now()
			windowStart := now.Add(-swl.window)

			// 古いリクエストを削除
			validRequests := []time.Time{}
			for _, req := range swl.requests {
				if req.After(windowStart) {
					validRequests = append(validRequests, req)
				}
			}
			swl.requests = validRequests

			// レート制限チェック
			if len(swl.requests) < swl.rate {
				swl.requests = append(swl.requests, now)
				return true
			}
			return false
		}

		WaitForSlot := func(swl *SlidingWindowLimiter) {
			for !Allow(swl) {
				time.Sleep(100 * time.Millisecond)
			}
		}

		limiter := NewSlidingWindowLimiter(3, time.Second)

		requests := make(chan int, 10)
		for i := 1; i <= 10; i++ {
			requests <- i
		}
		close(requests)

		startTime := time.Now()

		for req := range requests {
			WaitForSlot(limiter)
			fmt.Printf("処理中: リクエスト %d (経過時間: %v)\n",
				req, time.Since(startTime).Round(time.Millisecond))
		}
	}

	solution1()
	solution2()
	solution3()
	solution4()
}