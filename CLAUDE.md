# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
This is a comprehensive Go learning environment for parallel, concurrent, and asynchronous programming with real-world distributed systems integration. It combines educational content with practical enterprise patterns using Docker-based infrastructure.

## Project Structure
```
├── examples/          # 16 learning examples (basic + advanced patterns)
├── challenges/        # 8 challenges with problematic code to fix
│   ├── challenge01_deadlock.go     # Deadlock issues
│   ├── challenge02_race.go         # Race conditions
│   ├── challenge03_goroutine_leak.go # Goroutine leaks
│   ├── challenge04_rate_limiter.go  # Rate limiting
│   ├── challenge05_memory_leak.go   # Memory leaks
│   ├── challenge06_resource_leak.go # Resource leaks
│   ├── challenge07_security.go     # Security issues
│   └── challenge08_performance.go  # Performance problems
├── solutions/         # Multiple solutions for all 8 challenges
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

# Challenges (1-8) & solutions
go run cmd/runner/main.go -mode=challenge -challenge=1  # Deadlock
go run cmd/runner/main.go -mode=challenge -challenge=5  # Memory leak
go run cmd/runner/main.go -mode=solution -challenge=8   # Performance solution
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
5. **Common Issues**: Race conditions, Goroutine leaks, Deadlocks, Memory leaks
6. **Security**: Timing attacks, Resource exhaustion, Secure random generation
7. **Performance**: Lock contention, Channel buffering, Memory allocation optimization

## When Working on Challenges (8 Problems)
- Challenges are in `challenges/` directory and contain intentional bugs:
  1. Deadlock fixing
  2. Race condition resolution
  3. Goroutine leak prevention
  4. Rate limiter implementation
  5. Memory leak fixing
  6. Resource leak prevention (files, connections)
  7. Security vulnerability fixing (timing attacks, DoS)
  8. Performance optimization (lock contention, buffering)
- Solutions demonstrate multiple approaches with different trade-offs
- Use `go run -race` to detect race conditions
- Monitor goroutines with `runtime.NumGoroutine()`
- Check memory usage with `runtime.MemStats`

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