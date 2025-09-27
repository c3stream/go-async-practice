package challenges

import (
	"fmt"
	"time"
)

// Challenge04_ImplementRateLimiter - レート制限を実装してください
//
// 課題: 1秒間に最大3リクエストまで処理するレートリミッターを実装してください。
// 要件:
// - 10個のリクエストを受け取る
// - 1秒間に最大3つまで処理
// - time.Tickerやトークンバケットアルゴリズムを使用
func Challenge04_ImplementRateLimiter() {
	fmt.Println("=== Challenge 04: Implement Rate Limiter ===")
	fmt.Println("1秒間に3リクエストまで処理するレートリミッターを実装してください。")

	requests := make(chan int, 10)
	// 10個のリクエストを送信
	for i := 1; i <= 10; i++ {
		requests <- i
	}
	close(requests)

	// TODO: ここにレートリミッターを実装
	// ヒント: time.NewTicker(time.Second / 3) を使用

	startTime := time.Now()

	// リクエストを処理（現在は制限なし - 修正が必要）
	for req := range requests {
		fmt.Printf("処理中: リクエスト %d (経過時間: %v)\n",
			req, time.Since(startTime).Round(time.Millisecond))
	}
}

// Challenge04_Hint - ヒント
func Challenge04_Hint() {
	fmt.Println(`
ヒント:
1. time.NewTicker()を使ってトークンを定期的に生成
2. バッファ付きチャネルをセマフォとして使用
3. select文で複数のチャネルを監視
4. トークンバケットアルゴリズムの実装

期待される出力パターン:
処理中: リクエスト 1 (経過時間: 0ms)
処理中: リクエスト 2 (経過時間: 333ms)
処理中: リクエスト 3 (経過時間: 666ms)
処理中: リクエスト 4 (経過時間: 1s)
...
`)
}