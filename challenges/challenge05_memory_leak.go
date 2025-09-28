package challenges

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Challenge05_MemoryLeak - メモリリークの問題
//
// 🎆 問題: このコードにはメモリリークがあります。どこに問題があるか特定し、修正してください。
// 💡 ヒント: チャネルのクローズとゴルーチンの終了条件を確認してください。
func Challenge05_MemoryLeak() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 🎆 チャレンジ5: メモリリークを修正せよ   ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n⚠️ 現在の状態: メモリが解放されません！")

	// メモリ使用量を記録
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	before := m.Alloc

	// 問題のあるコード
	type DataProcessor struct {
		data chan []byte
		done chan struct{}
		wg   sync.WaitGroup
	}

	processor := &DataProcessor{
		data: make(chan []byte, 100),
		done: make(chan struct{}),
	}

	// ワーカーを起動（問題：終了条件がない）
	for i := 0; i < 10; i++ {
		processor.wg.Add(1)
		go func(id int) {
			defer processor.wg.Done()
			for {
				select {
				case d := <-processor.data:
					// データを処理
					_ = len(d)
					fmt.Printf("  ワーカー%d: %d バイトを処理\n", id, len(d))
				default:
					// ビジーウェイト（CPU使用率が高くなる）
					continue
				}
			}
		}(i)
	}

	// データを送信
	for i := 0; i < 100; i++ {
		data := make([]byte, 1024*1024) // 1MB
		processor.data <- data
	}

	// 処理を停止しようとするが...
	close(processor.done)
	// processor.wg.Wait() // これは永遠に待つことになる

	time.Sleep(1 * time.Second)

	// メモリ使用量を確認
	runtime.GC()
	runtime.ReadMemStats(&m)
	after := m.Alloc
	leaked := after - before

	fmt.Printf("\n⚠️ メモリリーク: %d バイト\n", leaked)
	fmt.Println("🔧 修正が必要な箇所:")
	fmt.Println("  1. ワーカーの終了条件")
	fmt.Println("  2. チャネルのクローズ")
	fmt.Println("  3. リソースの適切な解放")
}

// Challenge05_Hint - 修正のヒント
func Challenge05_Hint() {
	fmt.Println("\n💡 修正のヒント:")
	fmt.Println("────────────────────────")
	fmt.Println("1. done チャネルをselectで監視する")
	fmt.Println("2. default節でのbusy-waitを避ける")
	fmt.Println("3. 全てのゴルーチンが確実に終了することを保証する")
	fmt.Println("4. defer を使って適切にリソースをクリーンアップする")
}