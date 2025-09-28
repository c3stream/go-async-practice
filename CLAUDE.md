# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview
This is a comprehensive Go learning environment for parallel, concurrent, and asynchronous programming with real-world distributed systems integration. It combines educational content with practical enterprise patterns using Docker-based infrastructure.

## Project Structure
```
├── examples/          # 16 learning examples (basic + advanced patterns)
├── challenges/        # 16 challenges with problematic code to fix
│   ├── challenge01_deadlock.go      # Deadlock issues
│   ├── challenge02_race.go          # Race conditions
│   ├── challenge03_goroutine_leak.go # Goroutine leaks
│   ├── challenge04_rate_limiter.go  # Rate limiting
│   ├── challenge05_memory_leak.go   # Memory leaks
│   ├── challenge06_resource_leak.go # Resource leaks
│   ├── challenge07_security.go      # Security issues
│   ├── challenge08_performance.go   # Performance problems
│   ├── challenge09_distributed_lock.go # Distributed locking issues
│   ├── challenge10_message_ordering.go # Message ordering problems
│   ├── challenge11_backpressure.go  # Backpressure handling
│   ├── challenge12_consistency.go   # Distributed consistency
│   ├── challenge13_event_sourcing.go # Event sourcing problems
│   ├── challenge14_saga_pattern.go  # Saga pattern issues
│   ├── challenge15_distributed_cache.go # Distributed cache problems
│   └── challenge16_stream_processing.go # Stream processing issues
├── solutions/         # Multiple solutions for challenges 1-8
├── benchmarks/        # Performance benchmarks
├── practical/         # Real-world examples with external services
│   ├── rabbitmq_example.go      # Message queue patterns
│   ├── kafka_example.go         # Event streaming
│   ├── database_example.go      # DB connection pools, caching
│   ├── echo_server.go          # Web server concurrency
│   ├── mongodb_example.go       # Document database patterns
│   ├── duckdb_analytics.go     # OLAP analytics
│   ├── cassandra_nosql.go      # NoSQL patterns
│   ├── neo4j_graph.go          # Graph database
│   ├── influxdb_timeseries.go  # Time series database
│   ├── cockroachdb_distributed.go # Distributed SQL
│   ├── couchbase_document.go   # Document DB with CAS
│   ├── hbase_columnar.go       # Wide column store
│   └── event_driven_example.go # Event-driven architecture
├── interactive/       # Interactive learning exercises with gamification
│   ├── quiz.go                 # Interactive quiz system
│   ├── visualizer.go          # Real-time visualization
│   └── evaluator_interactive.go # Gamified evaluation (XP, levels)
├── tests/            # Comprehensive test suites
│   ├── security_test.go        # Security testing
│   ├── load_test.go           # Load & stress testing
│   ├── e2e_test.go            # End-to-end testing
│   └── e2e_comprehensive_test.go # Full workflow E2E
├── tracker/          # Learning progress tracking
├── internal/
│   └── evaluator/    # Automatic code evaluation
└── cmd/
    ├── runner/       # Main CLI application
    └── practical/    # Practical examples runner
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
- **MongoDB** - Document database
- **Cassandra** - Wide column store
- **Neo4j** - Graph database
- **InfluxDB** - Time series database
- **CockroachDB** - Distributed SQL database
- **Prometheus + Grafana** - Monitoring and visualization
- **Jaeger** - Distributed tracing
- **NATS** - Lightweight messaging

### Quick Start with Docker
```bash
# Start all services
make docker-up

# Run practical examples
make run-practical PATTERN=rabbitmq     # Message queue patterns
make run-practical PATTERN=kafka        # Event streaming
make run-practical PATTERN=redis-pubsub # Pub/Sub messaging
make run-practical PATTERN=postgres     # SQL database
make run-practical PATTERN=mongodb      # Document database
make run-practical PATTERN=cassandra    # Wide column store
make run-practical PATTERN=neo4j        # Graph database
make run-practical PATTERN=influxdb     # Time series database
make run-practical PATTERN=cockroachdb  # Distributed SQL
make run-practical PATTERN=couchbase    # Document DB with CAS
make run-practical PATTERN=hbase        # Column-oriented DB
make run-practical PATTERN=duckdb       # Analytics database
make run-practical PATTERN=echo-server  # Web server patterns
make run-practical PATTERN=event-driven # Event-driven architecture

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

# Interactive exercises with gamification
go run cmd/runner/main.go -mode=interactive

# Challenges (1-16) & solutions
go run cmd/runner/main.go -mode=challenge -challenge=1  # Deadlock
go run cmd/runner/main.go -mode=challenge -challenge=5  # Memory leak
go run cmd/runner/main.go -mode=challenge -challenge=13 # Event sourcing
go run cmd/runner/main.go -mode=challenge -challenge=16 # Stream processing
go run cmd/runner/main.go -mode=solution -challenge=8   # Performance solution
```

### Testing and Quality
```bash
# Benchmarks
make bench

# Race detection
go test -race ./...

# Security tests
go test ./tests -run TestSecurity

# Load tests
go test ./tests -run TestLoad

# E2E tests
go test ./tests -run TestE2E

# Code quality
make lint
make fmt
```

## Concurrency Patterns Covered

### Basic Patterns (Examples 1-8)
1. **Basic Patterns**: Goroutines, WaitGroups, Mutexes
2. **Channel Patterns**: Buffered/Unbuffered, Select, Non-blocking
3. **Synchronization**: Context cancellation, Timeouts, Deadlock prevention
4. **Advanced Patterns**: Worker pools, Pipeline, Fan-in/Fan-out, Semaphore

### Advanced Patterns (Examples 9-16)
5. **Circuit Breaker**: Fault tolerance
6. **Pub/Sub**: Event-driven architecture
7. **Bounded Parallelism**: Resource management
8. **Retry with Backoff**: Resilience patterns
9. **Batch Processing**: Throughput optimization
10. **Rate Limiting**: Traffic control
11. **Graceful Shutdown**: Clean termination
12. **Distributed Patterns**: Consensus, locking, consistency

## Challenge Problems (16 Total)

### Basic Challenges (1-8) - With Solutions
1. **Deadlock**: Classic dining philosophers, resource ordering
2. **Race Condition**: Shared state, atomic operations
3. **Goroutine Leak**: Proper cleanup, context cancellation
4. **Rate Limiting**: Token bucket, sliding window
5. **Memory Leak**: Buffer management, proper cleanup
6. **Resource Leak**: File handles, DB connections
7. **Security Issues**: Timing attacks, DoS prevention
8. **Performance**: Lock contention, channel optimization

### Advanced Challenges (9-16) - Solutions Coming Soon
9. **Distributed Lock**: TTL, ownership, fencing tokens
10. **Message Ordering**: Event sequencing, causality
11. **Backpressure**: Flow control, buffering strategies
12. **Distributed Consistency**: CAP theorem, eventual consistency
13. **Event Sourcing**: Event ordering, snapshots, replay
14. **Saga Pattern**: Compensation, state management
15. **Distributed Cache**: Cache stampede, hot keys, consistency
16. **Stream Processing**: Windowing, late data, checkpointing

## Interactive Learning Features

### Gamification System
- **XP System**: Earn experience for completing challenges
- **Levels**: Progress through difficulty levels
- **Achievements**: Unlock badges for milestones
- **Leaderboard**: Compare progress with others
- **Daily Challenges**: New problems every day

### Real-time Visualization
- Goroutine lifecycle visualization
- Channel flow animation
- Lock contention heatmaps
- Memory usage graphs
- Race condition detection overlay

## Evaluation System
The evaluator (`internal/evaluator/`) checks for:
- Deadlock detection with timeout
- Race condition detection
- Goroutine leak detection
- Memory leak analysis
- Performance benchmarking
- Security vulnerability scanning
- Automatic scoring and feedback
- Progress tracking with XP

## Practical Patterns Implemented

### Message Queue Patterns (RabbitMQ)
- Work Queue with acknowledgments
- Publish/Subscribe (Fanout)
- Topic-based routing
- RPC pattern
- Dead letter queues
- Priority queues

### Event Streaming (Kafka)
- Event sourcing
- Stream processing
- Consumer groups
- Partitioned topics
- Exactly-once semantics
- Compacted topics

### Database Patterns
- Connection pooling
- Cache-aside pattern
- Write-through caching
- Distributed locking with Redis
- Rate limiting
- Analytics with DuckDB
- Graph traversal with Neo4j
- Time series with InfluxDB
- Distributed transactions with CockroachDB

### Web Server Patterns (Echo)
- Concurrent middleware
- Circuit breaker
- Rate limiting
- Graceful shutdown
- Metrics collection
- Server-sent events
- WebSocket handling
- GraphQL subscriptions

## Testing Strategy

### Unit Tests
- Individual function testing
- Mock dependencies
- Table-driven tests
- Property-based testing

### Integration Tests
- Service interaction testing
- Database integration
- Message queue integration
- Cache integration

### E2E Tests
- Complete workflow testing
- Multi-service scenarios
- Failure recovery testing
- Performance under load

### Security Tests
- Timing attack detection
- Resource exhaustion prevention
- Input validation
- Authentication/authorization

### Load Tests
- Spike testing
- Sustained load testing
- Memory leak detection
- Cascading failure scenarios

## Important Development Notes

### Best Practices
- Always handle goroutine lifecycle to prevent leaks
- Use context for cancellation in long-running operations
- Monitor with `runtime.NumGoroutine()` during development
- Use `go run -race` to detect race conditions
- Implement proper error handling and logging
- Use structured logging for debugging
- Implement health checks for all services
- Use graceful shutdown patterns

### Performance Tips
- Profile with `pprof` for CPU and memory
- Use `sync.Pool` for object reuse
- Minimize allocations in hot paths
- Use buffered channels appropriately
- Consider `sync.Map` for concurrent maps
- Batch operations when possible
- Use atomic operations for counters

### Docker Services
- Services must be running for practical examples
- Use health checks to verify service readiness
- Configure appropriate resource limits
- Use volumes for persistent data
- Network isolation between services
- Proper secret management

### Production Readiness
- Implement circuit breakers for external services
- Add retry logic with exponential backoff
- Use distributed tracing (Jaeger)
- Implement metrics collection (Prometheus)
- Add comprehensive logging
- Use feature flags for gradual rollouts
- Implement proper monitoring and alerting
- Document SLIs/SLOs/SLAs