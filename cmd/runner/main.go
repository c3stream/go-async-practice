package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/kazuhirokondo/go-async-practice/examples"
	"github.com/kazuhirokondo/go-async-practice/challenges"
	"github.com/kazuhirokondo/go-async-practice/solutions"
	"github.com/kazuhirokondo/go-async-practice/internal/evaluator"
	"github.com/kazuhirokondo/go-async-practice/interactive"
	"github.com/kazuhirokondo/go-async-practice/scenarios"
)

func main() {
	var (
		mode      = flag.String("mode", "menu", "実行モード: menu, example, challenge, solution, benchmark, evaluate, practical, interactive")
		exampleID = flag.Int("example", 0, "実行する例題番号 (1-16)")
		challengeID = flag.Int("challenge", 0, "実行するチャレンジ番号 (1-24)")
	)
	flag.Parse()

	switch *mode {
	case "menu":
		showMenu()
	case "example":
		runExample(*exampleID)
	case "challenge":
		runChallenge(*challengeID)
	case "solution":
		runSolution(*challengeID)
	case "evaluate":
		runEvaluation()
	case "benchmark":
		fmt.Println("ベンチマークを実行するには以下のコマンドを使用してください:")
		fmt.Println("go test -bench=. ./benchmarks/")
	case "practical":
		runPractical()
	case "interactive":
		runInteractive()
	default:
		fmt.Printf("不明なモード: %s\n", *mode)
		flag.Usage()
		os.Exit(1)
	}
}

func showMenu() {
	fmt.Println(`
╔══════════════════════════════════════════════════════╗
║     🇯🇵 Go 並行・並列・非同期プログラミング学習環境    ║
╚══════════════════════════════════════════════════════╝

📚 学習コンテンツ:

1. 📖 例題で学ぶ - 基本パターンを順番に学習
   実行: go run cmd/runner/main.go -mode=example -example=1

2. 🎯 チャレンジ問題 - 実際のバグを修正して学ぶ
   実行: go run cmd/runner/main.go -mode=challenge -challenge=1

3. 💡 解答例 - 複数の解法を確認
   実行: go run cmd/runner/main.go -mode=solution -challenge=1

4. 📊 自動評価 - あなたのコードを採点
   実行: go run cmd/runner/main.go -mode=evaluate

5. ⚡ ベンチマーク - パフォーマンスを測定
   実行: go test -bench=. ./benchmarks/

📖 例題一覧（全16パターン）:
  【基礎編】
  1. ゴルーチンの基本 - 並行処理の第一歩
  2. レース条件 - 危険なデータ競合を理解する
  3. チャネルの基本 - goroutine間の通信方法
  4. select文 - 複数のチャネルを扱う
  5. コンテキスト - キャンセル処理の実装
  6. タイムアウト - 時間制限の設定方法
  7. 非ブロッキング操作 - 待たない処理の実現

  【応用編】
  8. ワーカープール - 効率的な並列処理
  9. ファンイン・ファンアウト - データの分散と集約
  10. パイプライン - 段階的なデータ処理
  11. セマフォ - 同時実行数の制限

  【実践編】（-example=12〜16）
  12. サーキットブレーカー - 障害の伝播を防ぐ
  13. Pub/Sub - イベント駆動パターン
  14. 制限付き並列処理 - リソース管理
  15. リトライ処理 - エラーハンドリング
  16. バッチ処理 - 効率的なデータ処理

🎯 チャレンジ問題（全24問）:
  【基礎編 1-8】
  1. デッドロックの修正 - お互いを待ち続ける問題を解決
  2. レース条件の修正 - データ競合を安全に
  3. ゴルーチンリークの修正 - メモリリークを防ぐ
  4. レート制限の実装 - API制限を実装する
  5. メモリリークの修正 - 適切なメモリ管理
  6. リソースリークの修正 - ファイル・接続の管理
  7. セキュリティ問題の修正 - 並行処理の安全性
  8. パフォーマンス問題の改善 - 効率的な並行処理

  【分散システム編 9-16】
  9. 分散ロック - 複数ノードでの同期
  10. メッセージ順序保証 - イベントの順序制御
  11. バックプレッシャー - フロー制御
  12. 分散一貫性 - CAP定理の実装
  13. イベントソーシング - イベント駆動設計
  14. Sagaパターン - 分散トランザクション
  15. 分散キャッシュ - キャッシュ戦略
  16. ストリーム処理 - リアルタイム処理

  【エンタープライズ編 17-24】
  17. Event Bus - イベント配信システム
  18. Message Bus - メッセージルーティング
  19. 分散ロギング - ログ集約と分析
  20. ブロックチェーン風コンセンサス - 合意形成
  21. グラフデータベース - グラフ処理
  22. 時系列データベース - 時系列データ管理
  23. カラムナストレージ - 列指向ストレージ
  24. オブジェクトストレージ - S3風ストレージ

🚀 おすすめの学習順序:
  1️⃣ まずは例題1〜7で基礎を固める
  2️⃣ チャレンジ1〜2で理解度をチェック
  3️⃣ 例題8〜11で応用パターンを学ぶ
  4️⃣ チャレンジ3〜4で実践力を養う
  5️⃣ 例題12〜16で実戦的なパターンをマスター

💡 ヒント: 各例題は約2〜3分で実行できます
📝 注意: -race オプションでレース条件を検出できます
`)
}

func runExample(id int) {
	switch id {
	case 1:
		examples.Example1_SimpleGoroutine()
	case 2:
		examples.Example2_RaceCondition()
	case 3:
		examples.Example3_ChannelBasics()
	case 4:
		examples.Example4_SelectStatement()
	case 5:
		examples.Example5_ContextCancellation()
	case 6:
		examples.Example6_Timeout()
	case 7:
		examples.Example7_NonBlockingChannel()
	case 8:
		examples.Example8_WorkerPool()
	case 9:
		examples.Example9_FanInFanOut()
	case 10:
		examples.Example10_Pipeline()
	case 11:
		examples.Example11_Semaphore()
	default:
		fmt.Printf("例題 %d は存在しません (1-11を指定)\n", id)
	}
}

func runChallenge(id int) {
	fmt.Println("\n⚠️  注意: これはチャレンジ問題です。")
	fmt.Println("コードには問題があり、修正が必要です。")
	fmt.Println("challenges/ ディレクトリのファイルを編集してください。\n")

	switch id {
	case 1:
		challenges.Challenge01_FixDeadlock()
		challenges.Challenge01_ExpectedOutput()
	case 2:
		challenges.Challenge02_FixRaceCondition()
		challenges.Challenge02_Hint()
	case 3:
		challenges.Challenge03_FixGoroutineLeak()
		challenges.Challenge03_Hint()
	case 4:
		challenges.Challenge04_ImplementRateLimiter()
		challenges.Challenge04_Hint()
	case 5:
		challenges.Challenge05_MemoryLeak()
		challenges.Challenge05_Hint()
	case 6:
		challenges.Challenge06_ResourceLeak()
		challenges.Challenge06_Hint()
	case 7:
		challenges.Challenge07_SecurityIssue()
		challenges.Challenge07_Hint()
	case 8:
		challenges.Challenge08_PerformanceIssue()
		challenges.Challenge08_Benchmark()
	case 9:
		challenges.Challenge09_DistributedLockProblem()
		challenges.Challenge09_Hint()
	case 10:
		challenges.Challenge10_MessageOrderingProblem()
		challenges.Challenge10_Hint()
	case 11:
		challenges.Challenge11_BackpressureProblem()
		challenges.Challenge11_Hint()
	case 12:
		challenges.Challenge12_ConsistencyProblem()
		challenges.Challenge12_Hint()
	case 13:
		challenges.Challenge13_EventSourcingProblem()
	case 14:
		challenges.Challenge14_SagaPatternProblem()
	case 15:
		challenges.Challenge15_DistributedCacheProblem()
	case 16:
		challenges.Challenge16_StreamProcessingProblem()
	case 17:
		challenges.RunChallenge17()
	case 18:
		challenges.RunChallenge18()
	case 19:
		challenges.RunChallenge19()
	case 20:
		challenges.RunChallenge20()
	case 21:
		challenges.RunChallenge21()
	case 22:
		challenges.RunChallenge22()
	case 23:
		challenges.RunChallenge23()
	case 24:
		challenges.RunChallenge24()
	default:
		fmt.Printf("チャレンジ %d は存在しません (1-24を指定)\n", id)
	}
}

func runSolution(id int) {
	switch id {
	case 1:
		solutions.Solution01_FixedDeadlock()
	case 2:
		solutions.Solution02_FixedRaceCondition()
	case 3:
		solutions.Solution03_FixedGoroutineLeak()
	case 4:
		solutions.Solution04_RateLimiter()
	case 5:
		solutions.Solution05_FixedMemoryLeak()
	case 6:
		solutions.Solution06_FixedResourceLeak()
	case 7:
		solutions.Solution07_FixedSecurity()
	case 8:
		solutions.Solution08_FixedPerformance()
	case 9:
		solutions.Solution09_DistributedLock()
	case 10:
		solutions.Solution10_MessageOrdering()
	case 11:
		solutions.Solution11_Backpressure()
	case 12:
		solutions.Solution12_Consistency()
	case 13:
		solutions.Solution13_EventSourcing()
	case 14:
		solutions.Solution14_SagaPattern()
	case 15:
		solutions.Solution15_DistributedCache()
	case 16:
		solutions.Solution16_StreamProcessing()
	case 17:
		solutions.RunSolution17()
	case 18:
		solutions.RunSolution18()
	case 19:
		solutions.RunSolution19()
	case 20:
		solutions.RunSolution20()
	case 21:
		solutions.RunSolution21()
	case 22:
		solutions.RunSolution22()
	case 23:
		solutions.RunSolution23()
	case 24:
		solutions.RunSolution24()
	default:
		fmt.Printf("解答 %d は存在しません (1-24を指定)\n", id)
	}
}

func runEvaluation() {
	fmt.Println("\n🔍 コード評価を開始します...\n")

	eval := evaluator.NewEvaluator()

	// デッドロックテスト
	fmt.Println("1. デッドロックのテスト...")
	eval.EvaluateDeadlock(func() {
		// ここに評価したいコードを配置
		ch := make(chan int, 1)
		ch <- 1
		<-ch
	}, 1*time.Second)

	// レース条件テスト
	fmt.Println("2. レース条件のテスト...")
	counter := 0
	eval.EvaluateRaceCondition(func() int {
		var wg sync.WaitGroup
		var mu sync.Mutex
		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				mu.Lock()
				counter++
				mu.Unlock()
			}()
		}
		wg.Wait()
		return counter
	}, 1000, 1000)

	// ゴルーチンリークテスト
	fmt.Println("3. ゴルーチンリークのテスト...")
	eval.EvaluateGoroutineLeak(func() {
		done := make(chan struct{})
		go func() {
			<-done
		}()
		close(done)
		time.Sleep(100 * time.Millisecond)
	}, 0)

	// パフォーマンステスト
	fmt.Println("4. パフォーマンステスト...")
	eval.EvaluatePerformance(func() {
		var wg sync.WaitGroup
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				time.Sleep(1 * time.Millisecond)
			}()
		}
		wg.Wait()
	}, 110*time.Millisecond)

	// 結果表示
	eval.PrintSummary()
}

// runQuiz - インタラクティブクイズを実行
func runQuiz() {
	quiz := interactive.NewInteractiveQuiz()
	quiz.StartQuiz()
}

// runScenario - 実世界シナリオを実行
func runScenario(id int) {
	scenarios := scenarios.NewRealWorldScenarios()

	switch id {
	case 0:
		// 全シナリオ実行
		scenarios.Scenario1_ECommercePlatform()
		scenarios.Scenario2_RealTimeChat()
		scenarios.Scenario3_LoadBalancer()
		scenarios.Scenario4_DataPipeline()
	case 1:
		scenarios.Scenario1_ECommercePlatform()
	case 2:
		scenarios.Scenario2_RealTimeChat()
	case 3:
		scenarios.Scenario3_LoadBalancer()
	case 4:
		scenarios.Scenario4_DataPipeline()
	default:
		fmt.Printf("シナリオ %d は存在しません (1-4を指定)\n", id)
	}
}

// runVisualization - リアルタイム可視化を実行
func runVisualization() {
	vis := interactive.NewVisualization()
	vis.VisualizeWorkerPool()
}

// runPractical - 実践的な例を実行
func runPractical() {
	fmt.Println("\n🚀 実践的な分散システムパターン\n")
	fmt.Println("以下のコマンドで実行できます:")
	fmt.Println()
	fmt.Println("1. RabbitMQ メッセージキュー:")
	fmt.Println("   make run-practical PATTERN=rabbitmq")
	fmt.Println()
	fmt.Println("2. Kafka イベントストリーミング:")
	fmt.Println("   make run-practical PATTERN=kafka")
	fmt.Println()
	fmt.Println("3. Redis Pub/Sub:")
	fmt.Println("   make run-practical PATTERN=redis-pubsub")
	fmt.Println()
	fmt.Println("4. PostgreSQL データベース:")
	fmt.Println("   make run-practical PATTERN=postgres")
	fmt.Println()
	fmt.Println("5. MongoDB ドキュメントDB:")
	fmt.Println("   make run-practical PATTERN=mongodb")
	fmt.Println()
	fmt.Println("6. Echo Webサーバー:")
	fmt.Println("   make run-practical PATTERN=echo-server")
	fmt.Println()
	fmt.Println("注意: 事前に 'make docker-up' でDockerサービスを起動してください")
}

// runInteractive - インタラクティブモードを実行
func runInteractive() {
	fmt.Println("\n🎮 インタラクティブ学習モード\n")
	fmt.Println("1. クイズモード - 並行プログラミングの理解度をチェック")
	fmt.Println("2. ビジュアライザー - リアルタイムでgoroutineの動作を可視化")
	fmt.Println("3. XPシステム - 課題をクリアして経験値を獲得")
	fmt.Println()
	fmt.Print("選択 (1-3): ")

	var choice int
	fmt.Scanf("%d", &choice)

	switch choice {
	case 1:
		runQuiz()
	case 2:
		runVisualization()
	case 3:
		fmt.Println("\nXPシステムは各チャレンジをクリアすることで自動的に経験値が蓄積されます")
		fmt.Println("チャレンジモードで問題を解いて、レベルアップを目指しましょう！")
	default:
		fmt.Println("無効な選択です")
	}
}