package challenges

import (
	"fmt"
	"sync"
)

// Challenge02_FixRaceCondition - レース条件を修正してください
//
// 問題: 複数のgoroutineが同時にmapにアクセスしており、レース条件が発生します。
// ヒント: sync.Mutexまたはsync.RWMutexを使用してください。
// `go run -race` で実行するとレース条件が検出されます。
func Challenge02_FixRaceCondition() {
	fmt.Println("=== Challenge 02: Fix Race Condition ===")
	fmt.Println("このコードにはレース条件があります。修正してください。")

	scores := make(map[string]int)
	var wg sync.WaitGroup

	// 10個のgoroutineが同時にmapを更新
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				key := fmt.Sprintf("player_%d", id)
				scores[key] = scores[key] + 1 // レース条件！
			}
		}(i)
	}

	// 読み取り専用のgoroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			total := 0
			for _, score := range scores { // レース条件！
				total += score
			}
			_ = total // 使用しない警告を抑制
		}
	}()

	wg.Wait()

	// 結果を表示
	for player, score := range scores {
		fmt.Printf("%s: %d\n", player, score)
	}
}

// Challenge02_Hint - ヒント
func Challenge02_Hint() {
	fmt.Println(`
ヒント:
1. sync.RWMutexを使用して読み書きを保護
2. 書き込み時はLock()、読み取り時はRLock()を使用
3. deferを使ってUnlock()を確実に実行
`)
}