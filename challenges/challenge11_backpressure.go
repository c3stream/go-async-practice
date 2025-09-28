package challenges

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge11_BackpressureProblem - バックプレッシャー処理の問題
// 問題: システムが過負荷時に適切にバックプレッシャーを処理できない
func Challenge11_BackpressureProblem() {
	fmt.Println("\n🔥 チャレンジ11: バックプレッシャー処理の問題")
	fmt.Println("=" + repeatString("=", 50))
	fmt.Println("問題: 高負荷時にシステムが適切に負荷制御できません")
	fmt.Println("症状: OOM、レスポンス遅延、カスケード障害")
	fmt.Println("\n⚠️  このコードには複数の問題があります:")

	// 問題のあるプロデューサー・コンシューマーシステム
	type Pipeline struct {
		// 問題1: 無制限バッファ
		input  chan Task
		output chan Result
		// 問題2: バックプレッシャーシグナルなし
		workers int
		wg      sync.WaitGroup
	}

	type Task struct {
		ID   int
		Data []byte
	}

	type Result struct {
		TaskID int
		Output []byte
		Error  error
	}

	pipeline := &Pipeline{
		input:   make(chan Task, 1000), // 問題3: 固定バッファサイズ
		output:  make(chan Result, 1000),
		workers: 10,
	}

	// 問題のあるプロデューサー
	produce := func(ctx context.Context) {
		id := 0
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// 問題4: 無制限にタスク生成
				task := Task{
					ID:   id,
					Data: make([]byte, 1024*1024), // 1MB
				}
				// 問題5: ブロッキングなしで送信
				pipeline.input <- task
				id++
				// 問題6: プロダクション速度の調整なし
			}
		}
	}

	// 問題のあるコンシューマー
	consume := func(ctx context.Context, workerID int) {
		defer pipeline.wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			case task := <-pipeline.input:
				// 重い処理をシミュレート
				time.Sleep(100 * time.Millisecond)

				result := Result{
					TaskID: task.ID,
					Output: make([]byte, len(task.Data)*2), // 問題7: メモリ使用量2倍
				}

				// 問題8: 出力チャネルがフルでもブロック
				pipeline.output <- result
			}
		}
	}

	// 問題のある結果処理
	processResults := func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			case result := <-pipeline.output:
				// 問題9: 遅い処理
				time.Sleep(200 * time.Millisecond)
				_ = result
			}
		}
	}

	// 実行
	fmt.Println("\n実行結果:")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// ワーカー起動
	for i := 0; i < pipeline.workers; i++ {
		pipeline.wg.Add(1)
		go consume(ctx, i)
	}

	// プロデューサー起動
	go produce(ctx)

	// 結果処理起動
	go processResults(ctx)

	// メトリクス監視
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				inputLen := len(pipeline.input)
				outputLen := len(pipeline.output)
				fmt.Printf("  入力キュー: %d, 出力キュー: %d\n", inputLen, outputLen)

				// 問題10: 過負荷検出なし
				if inputLen > 500 {
					fmt.Println("  ⚠ 入力キューが詰まっています！")
				}
			}
		}
	}()

	<-ctx.Done()
	close(pipeline.input)
	pipeline.wg.Wait()
	close(pipeline.output)

	fmt.Println("\n問題: キューが詰まり、メモリ使用量が増大")
}

// Challenge11_AdditionalProblems - 追加のバックプレッシャー問題
func Challenge11_AdditionalProblems() {
	fmt.Println("\n追加の問題パターン:")

	// 問題11: リアクティブストリームでの問題
	type ReactiveStream struct {
		subscriber chan interface{}
		// 問題: リクエスト数の管理なし
		// 問題: キャンセル処理なし
	}

	// 問題12: サーキットブレーカー未実装
	type Service struct {
		requestCount int64
		errorCount   int64
		// 問題: サーキットブレーカーなし
		// 問題: レート制限なし
	}

	service := &Service{}

	callService := func() error {
		atomic.AddInt64(&service.requestCount, 1)

		// 問題13: 無制限リトライ
		for i := 0; i < 100; i++ {
			// エラー時に即座にリトライ
			// 問題14: エクスポネンシャルバックオフなし
			time.Sleep(time.Millisecond)
		}

		return nil
	}

	// 問題15: アダプティブな負荷制御なし
	type LoadBalancer struct {
		servers []string
		// 問題: 各サーバーの負荷状況を考慮しない
		// 問題: ヘルスチェックなし
	}

	_ = callService
}

// Challenge11_StreamProcessingProblem - ストリーム処理の問題
func Challenge11_StreamProcessingProblem() {
	fmt.Println("\n追加: ストリーム処理のバックプレッシャー問題")

	type StreamProcessor struct {
		input  chan []byte
		stages []chan []byte
		output chan []byte
	}

	processor := &StreamProcessor{
		input:  make(chan []byte, 100),
		stages: make([]chan []byte, 3),
		output: make(chan []byte, 100),
	}

	// 各ステージの初期化
	for i := range processor.stages {
		// 問題16: 各ステージのバッファサイズが同じ
		processor.stages[i] = make(chan []byte, 100)
	}

	// 問題17: プッシュ型でプル型制御なし
	runStage := func(in, out chan []byte, processTime time.Duration) {
		for data := range in {
			// 処理
			time.Sleep(processTime)
			processed := append([]byte("processed_"), data...)
			// 問題18: フルでもブロック
			out <- processed
		}
	}

	// 各ステージで処理速度が異なる
	go runStage(processor.input, processor.stages[0], 10*time.Millisecond)   // 高速
	go runStage(processor.stages[0], processor.stages[1], 100*time.Millisecond) // 低速（ボトルネック）
	go runStage(processor.stages[1], processor.stages[2], 20*time.Millisecond)  // 中速
	go runStage(processor.stages[2], processor.output, 10*time.Millisecond)     // 高速
}

// Challenge11_Hint - ヒント表示
func Challenge11_Hint() {
	fmt.Println("\n💡 ヒント:")
	fmt.Println("1. 動的バッファサイズまたは無限バッファの回避")
	fmt.Println("2. レート制限（トークンバケット、リーキーバケット）")
	fmt.Println("3. プル型アーキテクチャの採用")
	fmt.Println("4. サーキットブレーカーパターン")
	fmt.Println("5. アダプティブな同時実行数制御")
	fmt.Println("6. ドロップポリシー（tail drop, random drop）")
	fmt.Println("7. リアクティブストリームの仕様準拠")
}

// Challenge11_ExpectedBehavior - 期待される動作
func Challenge11_ExpectedBehavior() {
	fmt.Println("\n✅ 期待される動作:")
	fmt.Println("1. プロデューサーの速度が自動調整される")
	fmt.Println("2. メモリ使用量が制限内に収まる")
	fmt.Println("3. 過負荷時に適切にリクエストを拒否")
	fmt.Println("4. システム全体のスループット最適化")
	fmt.Println("5. カスケード障害の防止")
}