# Task Completion Checklist

## After Code Changes
1. **Format code**: `make fmt`
2. **Run linter**: `make lint`
3. **Run tests**: `make test-race`
4. **Check coverage**: `make test-coverage`
5. **Run benchmarks** (if performance-critical): `make bench`

## For New Features
1. Add unit tests
2. Add integration tests if using external services
3. Update documentation (README.md, CLAUDE.md)
4. Add example code if appropriate
5. Consider adding to interactive learning system

## For Challenge Solutions
1. Ensure problem is properly solved
2. Add multiple solution approaches
3. Include performance considerations
4. Document trade-offs
5. Add to evaluator for automatic checking

## Before Commit
1. Verify all tests pass
2. No linting errors
3. Documentation updated
4. Code properly formatted
5. No debug code or TODOs left