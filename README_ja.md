# 🇯🇵 Go 並行・並列・非同期プログラミング学習環境

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)
[![CI Status](https://img.shields.io/badge/CI-Passing-success?style=for-the-badge)](https://github.com/kazuhirokondo/go-async-practice/actions)

## 📚 概要

Goの並行処理パターンを実践的に学べる総合学習環境です。基礎から応用まで段階的に習得できます。

### ✨ 特徴

- 🎓 **16種類の並行処理パターン** - 基礎から実践まで
- 🎯 **16個のチャレンジ問題** - 実際のバグや分散システムの問題を修正して学習
- 📊 **自動評価システム** - コードを即座に採点
- 🐳 **Docker統合** - 15種類以上のデータベース・メッセージングサービス
- 🧪 **包括的なテスト** - Unit/Integration/E2E完備
- 🚀 **ベンチマーク** - パフォーマンス測定

## 🚀 クイックスタート

```bash
# リポジトリをクローン
git clone https://github.com/kazuhirokondo/go-async-practice.git
cd go-async-practice

# 依存関係をインストール
go mod tidy

# メニューを表示
go run cmd/runner/main.go
```

## 📖 学習コンテンツ

### 基礎編（例題1-7）
- **ゴルーチンの基本** - 並行処理の第一歩
- **レース条件** - データ競合の理解と対策
- **チャネルの基本** - 安全な通信方法
- **select文** - 複数チャネルの制御
- **コンテキスト** - キャンセル処理
- **タイムアウト** - 時間制限の実装
- **非ブロッキング操作** - 待たない処理

### 応用編（例題8-11）
- **ワーカープール** - 効率的なタスク処理
- **ファンイン・ファンアウト** - データ分散と集約
- **パイプライン** - 段階的データ処理
- **セマフォ** - 同時実行数制限

### 実践編（例題12-16）
- **サーキットブレーカー** - 障害の伝播防止
- **Pub/Sub** - イベント駆動アーキテクチャ
- **制限付き並列処理** - リソース管理
- **リトライ処理** - エラーハンドリング
- **バッチ処理** - 効率的なデータ処理

## 🎯 チャレンジ問題（全16問）

### 基本的な並行処理の問題（1-4）
1. **デッドロックの修正** - 相互待機の解決
2. **レース条件の修正** - データ競合の防止
3. **ゴルーチンリークの修正** - メモリリーク対策
4. **レート制限の実装** - API制限の実装

### 高度なシステム問題（5-8）
5. **メモリリーク修正** - メモリ使用量の最適化
6. **リソースリーク防止** - ファイル、接続の適切な管理
7. **セキュリティ問題修正** - タイミング攻撃、DoS対策
8. **パフォーマンス最適化** - ロック競合、バッファリング改善

### 分散システムの問題（9-12）
9. **分散ロック** - 複数ノード間のロック機構
10. **メッセージ順序保証** - イベントの順序性維持
11. **バックプレッシャー処理** - 過負荷時の制御
12. **分散一貫性** - 複数データストア間の整合性

### 高度な分散パターン（13-16）
13. **イベントソーシング** - イベントストアとスナップショット
14. **Sagaパターン** - 分散トランザクション管理
15. **分散キャッシュ** - キャッシュ一貫性とスタンピード対策
16. **ストリーム処理** - リアルタイムデータパイプライン

## 🏃 実行方法

### 例題の実行
```bash
# 例題1を実行（ゴルーチンの基本）
go run cmd/runner/main.go -mode=example -example=1

# 全ての基礎例題を順番に実行
for i in {1..7}; do
    go run cmd/runner/main.go -mode=example -example=$i
done
```

### チャレンジ問題
```bash
# チャレンジ1-16から選択
go run cmd/runner/main.go -mode=challenge -challenge=1   # デッドロック
go run cmd/runner/main.go -mode=challenge -challenge=9   # 分散ロック
go run cmd/runner/main.go -mode=challenge -challenge=13  # イベントソーシング

# 解答例を確認（1-14完全対応）
go run cmd/runner/main.go -mode=solution -challenge=1   # デッドロック
go run cmd/runner/main.go -mode=solution -challenge=11  # バックプレッシャー
go run cmd/runner/main.go -mode=solution -challenge=12  # 分散一貫性
go run cmd/runner/main.go -mode=solution -challenge=13  # イベントソーシング
go run cmd/runner/main.go -mode=solution -challenge=14  # Sagaパターン

# チャレンジ15-16は解答作成中
```

### 自動評価
```bash
# あなたのコードを評価
go run cmd/runner/main.go -mode=evaluate
```

## 🐳 Docker環境

### 実践的なマイクロサービス環境

```bash
# 環境を起動
make docker-up

# 実践例を実行
make run-practical PATTERN=rabbitmq     # メッセージキュー
make run-practical PATTERN=kafka        # イベントストリーミング
make run-practical PATTERN=mongodb      # ドキュメントDB
make run-practical PATTERN=cassandra    # 分散NoSQL
make run-practical PATTERN=neo4j        # グラフDB
make run-practical PATTERN=influxdb     # 時系列DB
make run-practical PATTERN=cockroachdb  # 分散SQL
make run-practical PATTERN=couchbase    # ドキュメントDB（CAS対応）
make run-practical PATTERN=hbase        # カラム指向DB
make run-practical PATTERN=duckdb       # 分析DB

# クリーンアップ
make docker-down
```

### 含まれるサービス（拡張版）

#### データベース
- **PostgreSQL** - リレーショナルDB
- **CockroachDB** - 分散SQL
- **DuckDB** - OLAP分析データベース
- **Redis** - キャッシュ/Pub-Sub
- **MongoDB** - ドキュメント指向NoSQL
- **Couchbase** - ドキュメントDB（CAS対応）
- **Cassandra** - ワイドカラムストア
- **HBase** - カラム指向ストア
- **Neo4j** - グラフデータベース
- **InfluxDB** - 時系列データベース

#### メッセージング
- **RabbitMQ** - メッセージキュー
- **Kafka** - イベントストリーミング
- **NATS** - 軽量イベントバス

#### ストレージ＆モニタリング
- **MinIO** - S3互換オブジェクトストレージ
- **Prometheus** - メトリクス収集
- **Grafana** - 可視化
- **Jaeger** - 分散トレーシング
- **Elasticsearch** - 全文検索・ログ分析

## 🧪 テスト実行

```bash
# 全テストを実行
./test.sh

# ユニットテスト
go test ./...

# レース条件検出
go test -race ./...

# カバレッジレポート
make test-coverage

# ベンチマーク
go test -bench=. ./benchmarks/
```

## 📊 パフォーマンス比較

```
BenchmarkMutexCounter:        90.55 ns/op
BenchmarkChannelCounter:      105.9 ns/op
BenchmarkAtomicCounter:       33.52 ns/op ← 最速！
BenchmarkBufferedChannel:     46.07 ns/op
BenchmarkUnbufferedChannel:   141.9 ns/op
```

## 🗂 プロジェクト構造

```
go-async-practice/
├── cmd/runner/         # CLIエントリーポイント
├── examples/           # 学習用例題（16パターン）
├── challenges/         # チャレンジ問題（16問）
├── solutions/          # 解答例（14問完全対応、Challenge 1-14）
├── practical/          # 実践的な例（Docker必須）
├── benchmarks/         # パフォーマンス測定
├── internal/evaluator/ # 自動評価システム
├── tests/              # 統合・E2Eテスト
├── interactive/        # インタラクティブ学習（ゲーミフィケーション）
└── docker-compose.yml  # Docker環境設定
```

## 📚 学習の進め方

### 推奨学習パス

```mermaid
graph LR
    A[基礎編 1-7] --> B[チャレンジ 1-4]
    B --> C[応用編 8-11]
    C --> D[チャレンジ 5-8]
    D --> E[実践編 12-16]
    E --> F[チャレンジ 9-14]
    F --> G[Docker実践]
    G --> H[チャレンジ 15-16]
```

### 段階的アプローチ

1. **基礎を固める** - 例題1-7でGoroutineとChannelを理解
2. **基本問題** - チャレンジ1-4でデッドロック、レース条件に対処
3. **応用パターン** - 例題8-11でWorker Pool、Pipeline等を習得
4. **システム問題** - チャレンジ5-8でメモリ、セキュリティ、パフォーマンス最適化
5. **実践編** - 例題12-16でCircuit Breaker、Pub/Sub等を実装
6. **分散システム** - チャレンジ9-14で分散ロック、一貫性、イベントソーシング
7. **Docker実践** - 各種データベース、メッセージングシステムとの統合
8. **高度な課題** - チャレンジ15-16で分散キャッシュ、ストリーム処理に挑戦

## 🛠 開発環境

### 必要要件
- Go 1.21以上
- Docker & Docker Compose（実践編用）
- Make（オプション）

### 推奨ツール
```bash
# golangci-lint（コード品質）
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# air（ホットリロード）
go install github.com/cosmtrek/air@latest
```

## 🤝 コントリビューション

プルリクエスト歓迎です！以下の手順でご協力ください：

1. フォーク
2. フィーチャーブランチ作成（`git checkout -b feature/AmazingFeature`）
3. コミット（`git commit -m '素晴らしい機能を追加'`）
4. プッシュ（`git push origin feature/AmazingFeature`）
5. プルリクエスト作成

## 📄 ライセンス

MITライセンス - 詳細は[LICENSE](LICENSE)を参照

## 🙏 謝辞

このプロジェクトは以下のリソースに触発されました：

- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [Effective Go](https://go.dev/doc/effective_go)
- Goコミュニティの素晴らしい貢献者たち

## 💬 サポート

質問や問題がある場合は：

- 📝 [Issue](https://github.com/kazuhirokondo/go-async-practice/issues)を作成
- 💡 [Discussions](https://github.com/kazuhirokondo/go-async-practice/discussions)で議論
- 📧 メール: your-email@example.com

---

<div align="center">
  <strong>Happy Learning! 🚀</strong><br>
  Made with ❤️ using Claude Code
</div>