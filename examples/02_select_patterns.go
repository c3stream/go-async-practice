package examples

import (
	"context"
	"fmt"
	"time"
)

// Example4_SelectStatement - select文による多重化
// 複数のチャネルを同時に待ち受ける方法を学びます
func Example4_SelectStatement() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 例題4: select文で複数チャネルを制御   ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n📝 説明: 複数のチャネルからの受信を同時に待機")

	ch1 := make(chan string)
	ch2 := make(chan string)

	go func() {
		time.Sleep(100 * time.Millisecond)
		ch1 <- "チャネル1からのメッセージ"
	}()

	go func() {
		time.Sleep(200 * time.Millisecond)
		ch2 <- "チャネル2からのメッセージ"
	}()

	for i := 0; i < 2; i++ {
		select {
		case msg1 := <-ch1:
			fmt.Printf("  📨 受信: %s\n", msg1)
		case msg2 := <-ch2:
			fmt.Printf("  📨 受信: %s\n", msg2)
		case <-time.After(300 * time.Millisecond):
			fmt.Println("  ⏰ タイムアウト！")
		}
	}
}

// Example5_ContextCancellation - コンテキストによるキャンセル
// contextパッケージを使った安全なキャンセル処理を学びます
func Example5_ContextCancellation() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 例題5: コンテキストによるキャンセル   ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n📝 説明: 実行中のゴルーチンを安全に停止")

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			select {
			case <-ctx.Done():
				fmt.Println("  🛑 ワーカー: キャンセル信号を受信")
				return
			default:
				fmt.Println("  ⚙️ ワーカー: 処理中...")
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	time.Sleep(350 * time.Millisecond)
	fmt.Println("  📣 メイン: キャンセル信号を送信")
	cancel()
	time.Sleep(100 * time.Millisecond) // ワーカーの終了を待つ
	fmt.Println("\n💡 ポイント: contextを使えばリソースリークを防げる")
}

// Example6_Timeout - タイムアウトパターン
// 長時間かかる処理に時間制限を設ける方法を学びます
func Example6_Timeout() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 例題6: タイムアウト処理の実装         ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n📝 説明: 2秒かかる処理を1秒でタイムアウト")

	// ⏳ 時間がかかる処理（2秒）
	slowOperation := func() chan string {
		result := make(chan string)
		go func() {
			time.Sleep(2 * time.Second)
			result <- "処理完了"
		}()
		return result
	}

	// ⏱️ タイムアウト設定（1秒）
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	select {
	case result := <-slowOperation():
		fmt.Printf("  ✅ 成功: %s\n", result)
	case <-ctx.Done():
		fmt.Println("  ⏰ タイムアウト！（1秒経過）")
		fmt.Println("\n💡 ポイント: 外部APIコールなどで必須のパターン")
	}
}

// Example7_NonBlockingChannel - 非ブロッキング操作
// チャネル操作で待機しない方法を学びます
func Example7_NonBlockingChannel() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 例題7: 非ブロッキングチャネル操作     ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n📝 説明: selectとdefaultを組み合わせて待たない処理を実現")

	ch := make(chan int, 1)

	// 🚀 非ブロッキング送信
	select {
	case ch <- 42:
		fmt.Println("  ✅ チャネルに値を送信: 42")
	default:
		fmt.Println("  ⚠️ チャネルが満杯、送信をスキップ")
	}

	// 📥 非ブロッキング受信
	select {
	case value := <-ch:
		fmt.Printf("  ✅ 値を受信: %d\n", value)
	default:
		fmt.Println("  ⚠️ 受信可能な値がない")
	}

	// 🔄 再度受信を試みる（チャネルは空）
	select {
	case value := <-ch:
		fmt.Printf("  ✅ 値を受信: %d\n", value)
	default:
		fmt.Println("  ⚠️ チャネルが空（値がない）")
		fmt.Println("\n💡 ポイント: default節により処理をブロックしない")
	}
}