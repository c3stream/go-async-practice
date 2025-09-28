# Development Commands

## Basic Operations
```bash
# Show menu
go run cmd/runner/main.go

# Run examples (1-16)
go run cmd/runner/main.go -mode=example -example=1

# Run challenges (1-16)
go run cmd/runner/main.go -mode=challenge -challenge=1

# Run solutions (1-8 only)
go run cmd/runner/main.go -mode=solution -challenge=1

# Interactive mode
go run cmd/runner/main.go -mode=interactive
```

## Docker Infrastructure
```bash
# Start all services
make docker-up

# Stop services
make docker-down

# Clean up completely
make docker-clean

# View logs
make docker-logs
```

## Practical Examples
```bash
# Run with Docker services
make run-practical PATTERN=rabbitmq
make run-practical PATTERN=kafka
make run-practical PATTERN=redis-pubsub
make run-practical PATTERN=postgres
make run-practical PATTERN=echo-server
make run-practical PATTERN=neo4j
make run-practical PATTERN=influxdb
make run-practical PATTERN=cockroachdb
```

## Testing
```bash
# All tests
make test

# Race detection
make test-race

# Coverage report
make test-coverage

# Integration tests
make test-integration

# E2E tests
make test-e2e

# Benchmarks
make bench
```

## Code Quality
```bash
# Lint
make lint

# Format
make fmt

# Install dependencies
make install-deps
```