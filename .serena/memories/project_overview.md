# Go Async Practice - Project Overview

## Purpose
Go言語の並列・並行・非同期プログラミングを包括的に学習する教育環境。
実践的なエンタープライズパターンをDockerベースのインフラストラクチャで体験。

## Tech Stack
- **Language**: Go 1.21
- **Message Queue**: RabbitMQ, Kafka
- **Databases**: 
  - PostgreSQL (RDBMS)
  - Redis (Cache/Pub-Sub)
  - MongoDB (Document DB)
  - Cassandra (Wide Column)
  - Neo4j (Graph DB)
  - InfluxDB (Time Series)
  - CockroachDB (Distributed SQL)
  - DuckDB (Analytics)
- **Object Storage**: MinIO (S3 compatible)
- **AWS Services**: LocalStack (DynamoDB, SQS, SNS emulator)
- **Web Framework**: Echo v4
- **Monitoring**: Prometheus, Grafana, Jaeger

## Project Components
- 16 examples (基本〜高度なパターン)
- 16 challenges (問題のあるコードを修正)
- Solutions (challenges 1-8のみ実装済み、9-16は未実装)
- Interactive learning system (ゲーミフィケーション付き)
- Practical examples (実際のサービスと統合)
- Comprehensive test suites
- Performance benchmarks