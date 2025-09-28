# Go Async Practice - エンタープライズ対応 並列・並行・非同期プログラミング学習環境

🚀 Go言語で並列・並行・非同期プログラミングを基礎から実践まで体系的に学べる統合学習環境。Docker完備で実際の分散システムパターンも体験可能。

## 🎯 目的

- Goroutineとchannelの基本から応用まで体系的に学習
- レース条件、デッドロック、ゴルーチンリークなどの問題を実践的に体験
- パフォーマンスベンチマークで各手法の特性を理解
- 自動評価システムで理解度をチェック

## 🆕 新機能と実践環境

### 🐳 Docker完備のエンタープライズ環境
```bash
# 全サービス起動（1コマンド）
make docker-up

# 実践例の実行
make run-practical PATTERN=rabbitmq  # メッセージキュー
make run-practical PATTERN=kafka      # イベントストリーミング
make run-practical PATTERN=postgres   # DB連携
make run-practical PATTERN=echo-server # Webサーバー
```

利用可能なサービス:
- PostgreSQL, Redis, DuckDB
- RabbitMQ, Kafka
- MinIO (S3互換), LocalStack (AWS互換)
- Prometheus, Grafana, Jaeger

### 🎮 インタラクティブ学習機能
- リアルタイムビジュアライザー
- 対話型練習問題
- 学習進捗トラッキング（XP/レベルシステム）
- デバッグヘルパーツール

## 📚 コンテンツ構成

### 1. Examples（例題）- 16パターン
基本的な並行パターン：
- Goroutineの基礎
- Channel操作
- Select文
- Context
- Worker Pool
- Pipeline
- Fan-In/Fan-Out
- Semaphore

高度なパターン：
- Circuit Breaker
- Pub/Sub
- Bounded Parallelism
- Retry with Exponential Backoff
- Batch Processing

### 2. Challenges（チャレンジ）- 12問
実際の問題を解いて理解を深める：
- Challenge 1: デッドロックの修正
- Challenge 2: レース条件の解決
- Challenge 3: ゴルーチンリークの防止
- Challenge 4: レート制限の実装
- Challenge 5: メモリリークの修正
- Challenge 6: リソースリークの防止
- Challenge 7: セキュリティ問題の修正
- Challenge 8: パフォーマンス問題の改善
- Challenge 9: 分散ロックの問題
- Challenge 10: メッセージ順序保証の問題
- Challenge 11: バックプレッシャー処理の問題
- Challenge 12: 分散一貫性の問題

### 3. Solutions（解答例）- 8問完全対応（1-8）
各チャレンジの複数の解法を提示：
- 各問題に対して2-4パターンの解法
- ベストプラクティスの解説付き
- パフォーマンス比較データ込み

### 4. Benchmarks（ベンチマーク）
パフォーマンス特性を理解：
- Mutex vs Channel vs Atomic
- Buffered vs Unbuffered Channel
- Goroutine作成 vs Pool
- map+Mutex vs sync.Map

### 5. Evaluator（評価システム）
コードの品質を自動評価：
- デッドロック検出
- レース条件検出
- ゴルーチンリーク検出
- パフォーマンス測定

## 🚀 使い方

### セットアップ
```bash
# 依存関係のインストール
go mod download
```

### メニューを表示
```bash
go run cmd/runner/main.go
```

### 例題を実行
```bash
# 例題1を実行
go run cmd/runner/main.go -mode=example -example=1
```

### チャレンジに挑戦
```bash
# チャレンジを実行（1-8の問題から選択）
go run cmd/runner/main.go -mode=challenge -challenge=1  # デッドロック
go run cmd/runner/main.go -mode=challenge -challenge=5  # メモリリーク

# challenges/challenge0X_*.go を編集して修正

# 解答例を確認（全8問対応）
go run cmd/runner/main.go -mode=solution -challenge=1
go run cmd/runner/main.go -mode=solution -challenge=8  # パフォーマンス改善

# チャレンジ9-12は解答作成中
```

### ベンチマークを実行
```bash
go test -bench=. ./benchmarks/
```

### レース条件の検出
```bash
go run -race cmd/runner/main.go -mode=example -example=2
```

### コード評価
```bash
go run cmd/runner/main.go -mode=evaluate
```

## 📂 ディレクトリ構造

```
.
├── docker-compose.yml # 全インフラ定義
├── Makefile          # 便利コマンド集
├── examples/         # 学習用例題（16パターン）
├── challenges/       # 修正が必要な問題コード（8問）
├── solutions/        # 複数の解答例（1-8完全対応）
├── practical/        # 実践的な分散システム例
│   ├── rabbitmq_example.go
│   ├── kafka_example.go
│   ├── database_example.go
│   └── echo_server.go
├── interactive/      # インタラクティブ練習
├── visualizer/       # リアルタイム可視化
├── debugger/         # デバッグ支援
├── tracker/          # 進捗管理
├── benchmarks/       # パフォーマンステスト
└── cmd/              # CLIアプリケーション
```

## 🎓 学習の進め方

1. **基礎理解**: まずexamplesを1から順に実行し、各パターンを理解
2. **問題解決**: challengesで実際の問題を解く
3. **解法比較**: solutionsで複数の解法を学ぶ
4. **性能理解**: benchmarksで各手法のパフォーマンスを確認
5. **自己評価**: evaluatorで理解度をチェック

## 💡 学習のポイント

- `go run -race` でレース条件を検出しながら開発
- ゴルーチンの数を `runtime.NumGoroutine()` で監視
- `go test -bench` でパフォーマンスを定量的に理解
- 複数の解法を試して、それぞれのトレードオフを理解

## 🔧 開発ツール

```bash
# コードフォーマット
go fmt ./...

# 静的解析
go vet ./...

# レース検出付きテスト
go test -race ./...
```

## 🏆 学習到達目標

このコースを完了すると以下ができるようになります：

1. **基礎スキル**
   - Goroutineとchannelを適切に使える
   - レース条件やデッドロックを回避できる
   - contextを使った適切なキャンセル処理

2. **実践スキル**
   - メッセージキューを使った非同期処理
   - イベント駆動アーキテクチャの実装
   - データベース連携での並行制御
   - Webサーバーでの効率的な並行処理

3. **運用スキル**
   - 分散トレーシングとメトリクス収集
   - サーキットブレーカーによる障害対応
   - グレースフルシャットダウンの実装

## 📖 参考資料

- [Effective Go - Concurrency](https://golang.org/doc/effective_go#concurrency)
- [Go Concurrency Patterns](https://talks.golang.org/2012/concurrency.slide)
- [Share Memory By Communicating](https://blog.golang.org/share-memory-by-communicating)

## 🤝 コントリビューション

新しいパターンや改善提案は大歓迎です！