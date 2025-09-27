package solutions

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Solution03_FixedGoroutineLeak - ゴルーチンリークを修正した解答例
func Solution03_FixedGoroutineLeak() {
	fmt.Println("=== Solution 03: Fixed Goroutine Leak ===")

	// 解法1: contextでキャンセル
	solution1 := func() {
		fmt.Println("\n解法1: context.WithCancelを使用")
		initialGoroutines := runtime.NumGoroutine()
		fmt.Printf("初期のゴルーチン数: %d\n", initialGoroutines)

		producer := func(ctx context.Context) chan int {
			ch := make(chan int)
			go func() {
				i := 0
				for {
					select {
					case <-ctx.Done():
						fmt.Println("Producer: キャンセルシグナルを受信")
						close(ch)
						return
					case ch <- i:
						i++
						time.Sleep(10 * time.Millisecond)
					}
				}
			}()
			return ch
		}

		ctx, cancel := context.WithCancel(context.Background())
		dataChan := producer(ctx)

		// 5個だけ受信
		for i := 0; i < 5; i++ {
			fmt.Printf("受信: %d\n", <-dataChan)
		}

		// キャンセルしてゴルーチンを終了
		cancel()
		time.Sleep(100 * time.Millisecond)

		currentGoroutines := runtime.NumGoroutine()
		fmt.Printf("現在のゴルーチン数: %d\n", currentGoroutines)

		if currentGoroutines <= initialGoroutines {
			fmt.Println("✅ ゴルーチンリークは修正されました！")
		}
	}

	// 解法2: done channelを使用
	solution2 := func() {
		fmt.Println("\n解法2: done channelでシグナリング")
		initialGoroutines := runtime.NumGoroutine()
		fmt.Printf("初期のゴルーチン数: %d\n", initialGoroutines)

		producer := func(done <-chan struct{}) chan int {
			ch := make(chan int)
			go func() {
				defer close(ch)
				i := 0
				for {
					select {
					case <-done:
						fmt.Println("Producer: 終了シグナルを受信")
						return
					default:
						select {
						case ch <- i:
							i++
							time.Sleep(10 * time.Millisecond)
						case <-done:
							fmt.Println("Producer: 終了シグナルを受信")
							return
						}
					}
				}
			}()
			return ch
		}

		done := make(chan struct{})
		dataChan := producer(done)

		// 5個だけ受信
		for i := 0; i < 5; i++ {
			fmt.Printf("受信: %d\n", <-dataChan)
		}

		// doneをクローズしてゴルーチンを終了
		close(done)
		time.Sleep(100 * time.Millisecond)

		currentGoroutines := runtime.NumGoroutine()
		fmt.Printf("現在のゴルーチン数: %d\n", currentGoroutines)

		if currentGoroutines <= initialGoroutines {
			fmt.Println("✅ ゴルーチンリークは修正されました！")
		}
	}

	// 解法3: WaitGroupとチャネルクローズ
	solution3 := func() {
		fmt.Println("\n解法3: WaitGroupで確実な終了を保証")
		initialGoroutines := runtime.NumGoroutine()
		fmt.Printf("初期のゴルーチン数: %d\n", initialGoroutines)

		producer := func(n int) (chan int, *sync.WaitGroup) {
			ch := make(chan int)
			var wg sync.WaitGroup
			wg.Add(1)

			go func() {
				defer wg.Done()
				defer close(ch)
				for i := 0; i < n; i++ {
					ch <- i
					time.Sleep(10 * time.Millisecond)
				}
				fmt.Println("Producer: 全データ送信完了")
			}()

			return ch, &wg
		}

		// 最初から5個だけ生成するように指定
		dataChan, wg := producer(5)

		// データを受信
		for val := range dataChan {
			fmt.Printf("受信: %d\n", val)
		}

		wg.Wait()
		time.Sleep(100 * time.Millisecond)

		currentGoroutines := runtime.NumGoroutine()
		fmt.Printf("現在のゴルーチン数: %d\n", currentGoroutines)

		if currentGoroutines <= initialGoroutines {
			fmt.Println("✅ ゴルーチンリークは修正されました！")
		}
	}

	solution1()
	solution2()
	solution3()
}