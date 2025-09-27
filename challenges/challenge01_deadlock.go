package challenges

import (
	"fmt"
	"time"
)

// Challenge01_FixDeadlock - デッドロックの修正
//
// 🎆 問題: 2つのgoroutineが互いのリソースを待ち続けてデッドロックが発生します。
// 💡 ヒント: channelの送受信の順序を考えてみてください。
func Challenge01_FixDeadlock() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 🎆 チャレンジ1: デッドロックを修正せよ    ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n⚠️ 現在の状態: デッドロックが発生します！")

	ch1 := make(chan int)
	ch2 := make(chan int)

	// Goroutine 1
	go func() {
		ch1 <- 1 // ch1に送信
		val := <-ch2 // ch2から受信を待つ
		fmt.Printf("✅ ゴルーチン1が受信: %d\n", val)
	}()

	// Goroutine 2
	go func() {
		ch2 <- 2 // ch2に送信
		val := <-ch1 // ch1から受信を待つ
		fmt.Printf("✅ ゴルーチン2が受信: %d\n", val)
	}()

	time.Sleep(1 * time.Second)
	fmt.Println("🎯 プログラム正常終了")
}

// Challenge01_ExpectedOutput - 期待される出力
func Challenge01_ExpectedOutput() {
	fmt.Println("\n📦 期待される出力:")
	fmt.Println("───────────────────────────")
	fmt.Println("✅ ゴルーチン1が受信: 2")
	fmt.Println("✅ ゴルーチン2が受信: 1")
	fmt.Println("🎯 プログラム正常終了")
	fmt.Println("\n🔧 修正方法のヒント:")
	fmt.Println("  1. バッファ付きチャネルを使用")
	fmt.Println("  2. select文を使用")
	fmt.Println("  3. 送受信の順序を変更")
}