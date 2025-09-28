# Code Style and Conventions

## Go Conventions
- Go version: 1.21
- Module: github.com/kazuhirokondo/go-async-practice
- Standard Go naming conventions (camelCase for exported, lowercase for internal)
- Error handling: always check and handle errors
- Defer usage for cleanup operations
- Context usage for cancellation and timeouts

## Project Structure
```
├── cmd/           # Entry points
├── examples/      # Learning examples
├── challenges/    # Problems to solve
├── solutions/     # Solutions for challenges
├── practical/     # Real-world service integrations
├── internal/      # Private packages
├── tests/         # Test suites
├── benchmarks/    # Performance tests
├── interactive/   # Interactive learning system
└── tracker/       # Progress tracking
```

## Linting Rules (golangci-lint)
- gofmt and goimports for formatting
- govet for correctness
- errcheck for error handling
- gosec for security
- gocyclo (max complexity: 15)
- funlen (max lines: 100, statements: 60)

## Testing Patterns
- Table-driven tests
- Race detection enabled
- Integration tests with build tag
- E2E tests with full Docker stack
- Benchmarks in dedicated directory

## Comments
- Japanese comments for educational content
- English for technical implementation details
- Clear problem descriptions in challenge files