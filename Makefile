.PHONY: help docker-up docker-down docker-clean run-example run-challenge test bench lint

# デフォルトターゲット
help: ## ヘルプを表示
	@echo "Go Async Practice - Makefile Commands"
	@echo "======================================"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Docker関連
docker-up: ## Docker環境を起動
	docker-compose up -d
	@echo "Waiting for services to start..."
	@sleep 10
	@echo "Services are ready!"
	@echo "RabbitMQ UI: http://localhost:15672 (admin/admin)"
	@echo "MinIO Console: http://localhost:9001 (minioadmin/minioadmin)"
	@echo "Grafana: http://localhost:3000 (admin/admin)"
	@echo "Jaeger UI: http://localhost:16686"
	@echo "Prometheus: http://localhost:9090"

docker-down: ## Docker環境を停止
	docker-compose down

docker-clean: ## Docker環境を完全にクリーンアップ
	docker-compose down -v
	docker system prune -f

docker-logs: ## Dockerログを表示
	docker-compose logs -f

# アプリケーション実行
run-example: ## 例題を実行 (例: make run-example EXAMPLE=1)
	go run cmd/runner/main.go -mode=example -example=$(EXAMPLE)

run-challenge: ## チャレンジを実行 (例: make run-challenge CHALLENGE=1)
	go run cmd/runner/main.go -mode=challenge -challenge=$(CHALLENGE)

run-solution: ## 解答を実行 (例: make run-solution CHALLENGE=1)
	go run cmd/runner/main.go -mode=solution -challenge=$(CHALLENGE)

run-interactive: ## インタラクティブ練習を実行
	go run cmd/runner/main.go -mode=interactive

run-practical: ## 実践的な例を実行 (例: make run-practical PATTERN=rabbitmq)
	go run cmd/practical/main.go -pattern=$(PATTERN)

# テストとベンチマーク
test: ## テストを実行
	go test -v ./...

test-race: ## レース条件検出付きテスト
	go test -race -v ./...

test-coverage: ## カバレッジレポートを生成
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report saved to coverage.html"

test-integration: ## 統合テストを実行
	@echo "Starting Docker services for integration tests..."
	docker-compose up -d postgres redis rabbitmq
	@sleep 10
	go test -v -tags=integration ./tests/...
	docker-compose down

test-e2e: ## E2Eテストを実行
	@echo "Starting full Docker environment for E2E tests..."
	docker-compose up -d
	@sleep 15
	go test -v -tags=e2e ./tests/...
	docker-compose down

test-all: test-coverage test-integration test-e2e ## すべてのテストを実行

bench: ## ベンチマークを実行
	go test -bench=. -benchmem ./benchmarks/

bench-compare: ## ベンチマーク結果を比較
	go test -bench=. -benchmem ./benchmarks/ | tee bench_new.txt
	@if [ -f bench_old.txt ]; then \
		benchstat bench_old.txt bench_new.txt; \
		mv bench_new.txt bench_old.txt; \
	else \
		mv bench_new.txt bench_old.txt; \
		echo "Initial benchmark saved to bench_old.txt"; \
	fi

# コード品質
lint: ## リンターを実行
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint is not installed. Running go vet instead..."; \
		go vet ./...; \
	fi

fmt: ## コードフォーマット
	go fmt ./...
	goimports -w .

# モニタリング設定
setup-monitoring: ## モニタリング設定をセットアップ
	mkdir -p monitoring/prometheus
	mkdir -p monitoring/grafana/dashboards
	mkdir -p monitoring/grafana/datasources
	@echo "Creating default Prometheus config..."
	@echo "global:" > monitoring/prometheus.yml
	@echo "  scrape_interval: 15s" >> monitoring/prometheus.yml
	@echo "scrape_configs:" >> monitoring/prometheus.yml
	@echo "  - job_name: 'go-app'" >> monitoring/prometheus.yml
	@echo "    static_configs:" >> monitoring/prometheus.yml
	@echo "      - targets: ['host.docker.internal:8080']" >> monitoring/prometheus.yml

# 開発環境
dev: docker-up ## 開発環境を起動
	@echo "Development environment is ready!"
	@echo "Run 'make run-practical PATTERN=rabbitmq' to test messaging"

clean: ## ビルド成果物をクリーン
	go clean -cache
	rm -rf bin/
	rm -f bench_*.txt

install-deps: ## 依存関係をインストール
	go mod download
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	go install golang.org/x/perf/cmd/benchstat@latest

# 実践パターンのヘルプ
patterns-help: ## 利用可能な実践パターンを表示
	@echo "Available Practical Patterns:"
	@echo "=============================="
	@echo "  rabbitmq    - RabbitMQ message queue example"
	@echo "  kafka       - Kafka event streaming example"
	@echo "  redis-pubsub- Redis Pub/Sub example"
	@echo "  postgres    - PostgreSQL with connection pool"
	@echo "  dynamodb    - DynamoDB-like operations"
	@echo "  s3          - S3-like object storage with MinIO"
	@echo "  echo-server - Echo web server with middleware"
	@echo "  distributed - Distributed system pattern"
	@echo ""
	@echo "Usage: make run-practical PATTERN=<pattern-name>"