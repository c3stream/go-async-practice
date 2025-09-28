package challenges

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Challenge06_ResourceLeak - リソースリークの問題
//
// 🎆 問題: HTTPクライアントとファイルハンドルのリソースリークがあります。修正してください。
// 💡 ヒント: Closeの呼び出しとコンテキストのキャンセルを確認してください。
func Challenge06_ResourceLeak() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 🎆 チャレンジ6: リソースリークを修正    ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n⚠️ 現在の状態: コネクションが閉じられません！")

	var wg sync.WaitGroup
	urls := []string{
		"https://httpbin.org/delay/1",
		"https://httpbin.org/delay/2",
		"https://httpbin.org/delay/3",
		"https://httpbin.org/status/500",
		"https://httpbin.org/status/404",
	}

	// 問題1: HTTPクライアントの使い回しなし
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 毎回新しいクライアントを作成（リソースの無駄）
			client := &http.Client{
				Timeout: 10 * time.Second,
			}

			url := urls[id%len(urls)]
			resp, err := client.Get(url)
			if err != nil {
				fmt.Printf("  ❌ リクエスト%d失敗: %v\n", id, err)
				return
			}

			// 問題2: response bodyを閉じていない
			// defer resp.Body.Close() が必要

			fmt.Printf("  📡 リクエスト%d: Status %d\n", id, resp.StatusCode)
		}(i)
	}

	// 問題3: タイムアウトなしで待機
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Println("✅ 全リクエスト完了")
	case <-time.After(5 * time.Second):
		fmt.Println("⏰ タイムアウト: まだ実行中のゴルーチンがあります")
	}

	// 問題4: ファイルハンドルリーク（疑似コード）
	/*
	for i := 0; i < 100; i++ {
		file, _ := os.Open("data.txt")
		// file.Close() が必要
		// データ処理...
	}
	*/

	fmt.Println("\n🔧 修正が必要な箇所:")
	fmt.Println("  1. HTTPクライアントの再利用")
	fmt.Println("  2. Response Bodyのクローズ")
	fmt.Println("  3. コネクションプールの設定")
	fmt.Println("  4. ファイルハンドルの適切な管理")
}

// Challenge06_Hint - 修正のヒント
func Challenge06_Hint() {
	fmt.Println("\n💡 修正のヒント:")
	fmt.Println("────────────────────────")
	fmt.Println("1. http.DefaultClient を使うか、グローバルなクライアントを作成")
	fmt.Println("2. 必ず defer resp.Body.Close() を使用")
	fmt.Println("3. Transport の MaxIdleConnsPerHost を設定")
	fmt.Println("4. コンテキストでタイムアウトを管理")
	fmt.Println("5. io.Copy(ioutil.Discard, resp.Body) で完全に読み込む")
}