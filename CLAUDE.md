# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
This is a comprehensive Go learning environment for parallel, concurrent, and asynchronous programming with real-world distributed systems integration. It combines educational content with practical enterprise patterns using Docker-based infrastructure.

## Project Structure
```
├── examples/          # 16 learning examples (basic + advanced patterns)
├── challenges/        # 4 challenges with problematic code to fix
├── solutions/         # Multiple solutions for each challenge
├── benchmarks/        # Performance benchmarks
├── practical/         # Real-world examples with external services
│   ├── rabbitmq_example.go  # Message queue patterns
│   ├── kafka_example.go     # Event streaming
│   ├── database_example.go  # DB connection pools, caching
│   └── echo_server.go       # Web server concurrency
├── interactive/       # Interactive learning exercises
├── visualizer/        # Real-time concurrency visualizer
├── debugger/          # Debugging helper tools
├── tracker/           # Learning progress tracking
├── internal/
│   └── evaluator/     # Automatic code evaluation
└── cmd/
    ├── runner/        # Main CLI application
    └── practical/     # Practical examples runner
```

## Docker Infrastructure
The project includes a complete Docker Compose setup with:
- **PostgreSQL** - Relational database with connection pooling
- **Redis** - In-memory cache and pub/sub
- **RabbitMQ** - Message queue (AMQP)
- **Kafka + Zookeeper** - Event streaming platform
- **MinIO** - S3-compatible object storage
- **LocalStack** - AWS services emulator (DynamoDB, SQS, SNS)
- **DuckDB** - Embedded analytics database
- **Prometheus + Grafana** - Monitoring and visualization
- **Jaeger** - Distributed tracing

### Quick Start with Docker
```bash
# Start all services
make docker-up

# Run practical examples
make run-practical PATTERN=rabbitmq
make run-practical PATTERN=kafka
make run-practical PATTERN=redis-pubsub
make run-practical PATTERN=postgres
make run-practical PATTERN=echo-server

# Stop services
make docker-down
```

## Key Commands

### Basic Learning
```bash
# Show menu
go run cmd/runner/main.go

# Examples (1-16)
go run cmd/runner/main.go -mode=example -example=1

# Interactive exercises
go run cmd/runner/main.go -mode=interactive

# Challenges & solutions
go run cmd/runner/main.go -mode=challenge -challenge=1
go run cmd/runner/main.go -mode=solution -challenge=1
```

### Testing and Quality
```bash
# Benchmarks
make bench

# Race detection
go test -race ./...

# Code quality
make lint
make fmt
```

## Concurrency Patterns Covered
1. **Basic Patterns**: Goroutines, WaitGroups, Mutexes
2. **Channel Patterns**: Buffered/Unbuffered, Select, Non-blocking
3. **Synchronization**: Context cancellation, Timeouts, Deadlock prevention
4. **Advanced Patterns**: Worker pools, Pipeline, Fan-in/Fan-out, Semaphore
5. **Common Issues**: Race conditions, Goroutine leaks, Deadlocks

## When Working on Challenges
- Challenges are in `challenges/` directory and contain intentional bugs
- Solutions demonstrate multiple approaches with different trade-offs
- Use `go run -race` to detect race conditions
- Monitor goroutines with `runtime.NumGoroutine()`

## Evaluation System
The evaluator (`internal/evaluator/`) checks for:
- Deadlock detection with timeout
- Race condition detection
- Goroutine leak detection
- Performance benchmarking
- Automatic scoring and feedback

## Practical Patterns Implemented

### Message Queue Patterns (RabbitMQ)
- Work Queue with acknowledgments
- Publish/Subscribe (Fanout)
- Topic-based routing
- RPC pattern

### Event Streaming (Kafka)
- Event sourcing
- Stream processing
- Consumer groups
- Partitioned topics

### Database Patterns
- Connection pooling
- Cache-aside pattern
- Write-through caching
- Distributed locking with Redis
- Rate limiting
- Analytics with DuckDB

### Web Server Patterns (Echo)
- Concurrent middleware
- Circuit breaker
- Rate limiting
- Graceful shutdown
- Metrics collection
- Server-sent events

## Important Development Notes
- Always handle goroutine lifecycle to prevent leaks
- Use context for cancellation in long-running operations
- Monitor with `runtime.NumGoroutine()` during development
- Use `go run -race` to detect race conditions
- Docker services must be running for practical examples
- Each practical example demonstrates production-ready patterns