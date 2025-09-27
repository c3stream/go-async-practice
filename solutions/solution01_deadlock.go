package solutions

import (
	"fmt"
	"sync"
	"time"
)

// Solution01_FixedDeadlock - デッドロックを修正した解答例
func Solution01_FixedDeadlock() {
	fmt.Println("=== Solution 01: Fixed Deadlock ===")

	// 解法1: バッファ付きチャネルを使用
	solution1 := func() {
		fmt.Println("\n解法1: バッファ付きチャネルを使用")
		ch1 := make(chan int, 1) // バッファサイズ1
		ch2 := make(chan int, 1) // バッファサイズ1

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			ch1 <- 1
			val := <-ch2
			fmt.Println("Goroutine 1 received:", val)
		}()

		go func() {
			defer wg.Done()
			ch2 <- 2
			val := <-ch1
			fmt.Println("Goroutine 2 received:", val)
		}()

		wg.Wait()
	}

	// 解法2: 送受信の順序を変更
	solution2 := func() {
		fmt.Println("\n解法2: 送受信の順序を統一")
		ch1 := make(chan int)
		ch2 := make(chan int)

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			val := <-ch1 // 先に受信
			fmt.Println("Goroutine 1 received:", val)
			ch2 <- 2 // 後で送信
		}()

		go func() {
			defer wg.Done()
			ch1 <- 1 // 先に送信
			val := <-ch2 // 後で受信
			fmt.Println("Goroutine 2 received:", val)
		}()

		wg.Wait()
	}

	// 解法3: selectを使用
	solution3 := func() {
		fmt.Println("\n解法3: selectで非ブロッキング操作")
		ch1 := make(chan int, 1)
		ch2 := make(chan int, 1)

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			ch1 <- 1
			select {
			case val := <-ch2:
				fmt.Println("Goroutine 1 received:", val)
			case <-time.After(100 * time.Millisecond):
				fmt.Println("Goroutine 1 timeout")
			}
		}()

		go func() {
			defer wg.Done()
			ch2 <- 2
			select {
			case val := <-ch1:
				fmt.Println("Goroutine 2 received:", val)
			case <-time.After(100 * time.Millisecond):
				fmt.Println("Goroutine 2 timeout")
			}
		}()

		wg.Wait()
	}

	solution1()
	solution2()
	solution3()

	fmt.Println("\nプログラム終了")
}