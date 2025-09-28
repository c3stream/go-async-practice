package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/kazuhirokondo/go-async-practice/practical"
)

func main() {
	var pattern string
	flag.StringVar(&pattern, "pattern", "", "Pattern to run (rabbitmq, kafka, redis-pubsub, postgres, echo-server, neo4j, influxdb, cockroachdb, couchbase, hbase)")
	flag.Parse()

	if pattern == "" {
		fmt.Println("Usage: go run cmd/practical/main.go -pattern=<pattern-name>")
		fmt.Println("\nAvailable patterns:")
		fmt.Println("  rabbitmq     - RabbitMQ message queue patterns")
		fmt.Println("  kafka        - Kafka event streaming")
		fmt.Println("  redis-pubsub - Redis pub/sub messaging")
		fmt.Println("  postgres     - PostgreSQL with connection pooling")
		fmt.Println("  echo-server  - Echo web server with concurrency")
		fmt.Println("  neo4j        - Neo4j graph database")
		fmt.Println("  influxdb     - InfluxDB time series database")
		fmt.Println("  cockroachdb  - CockroachDB distributed SQL")
		fmt.Println("  couchbase    - Couchbase document database")
		fmt.Println("  hbase        - HBase wide column store")
		fmt.Println("  mongodb      - MongoDB document database")
		fmt.Println("  cassandra    - Cassandra wide column store")
		fmt.Println("  duckdb       - DuckDB analytics database")
		fmt.Println("  event-driven - Event-driven architecture patterns")
		return
	}

	fmt.Printf("Running %s pattern...\n\n", pattern)

	switch pattern {
	case "couchbase":
		// Simulated example - works without Docker
		practical.RunCouchbaseExample()
	case "hbase":
		// Simulated example - works without Docker
		practical.RunHBaseExample()
	case "redis-pubsub":
		// Redis Pub/Sub example
		practical.RunRedisPubSubExamples()
	case "rabbitmq", "kafka", "postgres", "echo-server",
	     "neo4j", "influxdb", "cockroachdb", "mongodb", "cassandra",
	     "duckdb", "event-driven":
		fmt.Printf("⚠️  '%s' pattern requires Docker services to be running.\n", pattern)
		fmt.Println("\nTo start Docker services, run:")
		fmt.Println("  make docker-up")
		fmt.Println("\nNote: The actual implementation files exist in the practical/ directory")
		fmt.Println("but require external services to function properly.")
		fmt.Println("\nCurrently available without Docker:")
		fmt.Println("  - couchbase (document database simulation)")
		fmt.Println("  - hbase (wide column store simulation)")
		fmt.Println("\nAvailable with Docker:")
		fmt.Println("  - redis-pubsub (Redis Pub/Sub messaging)")
	default:
		log.Fatalf("Unknown pattern: %s", pattern)
	}
}