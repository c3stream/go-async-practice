package challenges

import (
	"fmt"
	"runtime"
	"time"
)

// Challenge03_FixGoroutineLeak - ゴルーチンリークを修正してください
//
// 問題: ゴルーチンが終了せずにリークしています。
// ヒント: contextやチャネルのクローズを適切に使用してください。
func Challenge03_FixGoroutineLeak() {
	fmt.Println("=== Challenge 03: Fix Goroutine Leak ===")
	fmt.Println("このコードはゴルーチンリークを起こします。修正してください。")

	initialGoroutines := runtime.NumGoroutine()
	fmt.Printf("初期のゴルーチン数: %d\n", initialGoroutines)

	// データを生成する関数（問題のあるコード）
	producer := func() chan int {
		ch := make(chan int)
		go func() {
			i := 0
			for {
				ch <- i // 永遠に送信し続ける（誰も受信しなくなってもリーク！）
				i++
				time.Sleep(10 * time.Millisecond)
			}
		}()
		return ch
	}

	// データを消費（途中で止める）
	dataChan := producer()
	for i := 0; i < 5; i++ {
		fmt.Printf("受信: %d\n", <-dataChan)
	}
	// ここで受信を止めるが、producerのゴルーチンは動き続ける！

	time.Sleep(100 * time.Millisecond)
	currentGoroutines := runtime.NumGoroutine()
	fmt.Printf("現在のゴルーチン数: %d\n", currentGoroutines)

	if currentGoroutines > initialGoroutines {
		fmt.Println("⚠️  ゴルーチンリークが発生しています！")
	}
}

// Challenge03_Hint - ヒント
func Challenge03_Hint() {
	fmt.Println(`
ヒント:
1. contextを使用してキャンセルシグナルを送る
2. done channelを使用して終了を通知
3. select文でキャンセルをチェック
4. チャネルをクローズして受信側に終了を通知
`)
}