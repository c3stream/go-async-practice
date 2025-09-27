package solutions

import (
	"fmt"
	"sync"
)

// Solution02_FixedRaceCondition - レース条件を修正した解答例
func Solution02_FixedRaceCondition() {
	fmt.Println("=== Solution 02: Fixed Race Condition ===")

	// 解法1: sync.RWMutexを使用
	solution1 := func() {
		fmt.Println("\n解法1: sync.RWMutexで読み書きを保護")

		scores := make(map[string]int)
		var mu sync.RWMutex // 読み書きロック
		var wg sync.WaitGroup

		// 書き込みgoroutine
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					key := fmt.Sprintf("player_%d", id)

					mu.Lock() // 書き込みロック
					scores[key] = scores[key] + 1
					mu.Unlock()
				}
			}(i)
		}

		// 読み取り専用goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				mu.RLock() // 読み取りロック
				total := 0
				for _, score := range scores {
					total += score
				}
				mu.RUnlock()
				_ = total
			}
		}()

		wg.Wait()

		// 結果を表示
		for player, score := range scores {
			fmt.Printf("%s: %d\n", player, score)
		}
	}

	// 解法2: sync.Mapを使用
	solution2 := func() {
		fmt.Println("\n解法2: sync.Mapを使用（並行安全なmap）")

		var scores sync.Map // 並行安全なmap
		var wg sync.WaitGroup

		// 書き込みgoroutine
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("player_%d", id)
				for j := 0; j < 100; j++ {
					// LoadOrStore と CompareAndSwap で原子的に更新
					for {
						value, _ := scores.LoadOrStore(key, 0)
						oldVal := value.(int)
						if scores.CompareAndSwap(key, oldVal, oldVal+1) {
							break
						}
					}
				}
			}(i)
		}

		// 読み取り専用goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				total := 0
				scores.Range(func(key, value interface{}) bool {
					total += value.(int)
					return true
				})
				_ = total
			}
		}()

		wg.Wait()

		// 結果を表示
		scores.Range(func(key, value interface{}) bool {
			fmt.Printf("%s: %d\n", key, value)
			return true
		})
	}

	// 解法3: チャネルを使った順次アクセス
	solution3 := func() {
		fmt.Println("\n解法3: チャネルで順次アクセスを保証")

		type operation struct {
			action string
			key    string
			value  int
			result chan int
		}

		scores := make(map[string]int)
		ops := make(chan operation)

		// マップを管理する単一のgoroutine
		go func() {
			for op := range ops {
				switch op.action {
				case "add":
					scores[op.key] = scores[op.key] + op.value
					if op.result != nil {
						op.result <- scores[op.key]
					}
				case "sum":
					total := 0
					for _, v := range scores {
						total += v
					}
					op.result <- total
				}
			}
		}()

		var wg sync.WaitGroup

		// 書き込みgoroutine
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				key := fmt.Sprintf("player_%d", id)
				for j := 0; j < 100; j++ {
					ops <- operation{action: "add", key: key, value: 1}
				}
			}(i)
		}

		// 読み取り専用goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				result := make(chan int)
				ops <- operation{action: "sum", result: result}
				<-result
			}
		}()

		wg.Wait()
		close(ops)

		// 結果を表示
		for player, score := range scores {
			fmt.Printf("%s: %d\n", player, score)
		}
	}

	solution1()
	solution2()
	solution3()
}