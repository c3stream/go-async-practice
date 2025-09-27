#!/bin/bash

# Test execution script
set -e

echo "🧪 Go Async Practice - Test Suite"
echo "================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results
PASSED=0
FAILED=0
SKIPPED=0

# Function to run test
run_test() {
    local name=$1
    local cmd=$2

    echo -e "\n${YELLOW}Running: ${name}${NC}"

    if eval "$cmd"; then
        echo -e "${GREEN}✅ ${name} passed${NC}"
        ((PASSED++))
    else
        echo -e "${RED}❌ ${name} failed${NC}"
        ((FAILED++))
    fi
}

# 1. Unit Tests
echo -e "\n📋 Phase 1: Unit Tests"
echo "----------------------"

run_test "Evaluator Tests" "go test -v ./internal/evaluator/"
run_test "Examples Tests" "go test -v ./examples/"
run_test "Interactive Tests" "go test -v ./interactive/"

# 2. Race Detection
echo -e "\n🏁 Phase 2: Race Detection"
echo "--------------------------"

run_test "Race Detection" "go test -race -short ./..."

# 3. Benchmarks
echo -e "\n⚡ Phase 3: Benchmarks"
echo "---------------------"

run_test "Benchmarks" "go test -bench=. -benchtime=1s ./benchmarks/"

# 4. Coverage
echo -e "\n📊 Phase 4: Code Coverage"
echo "------------------------"

echo "Generating coverage report..."
go test -coverprofile=coverage.out -covermode=atomic ./...
go tool cover -func=coverage.out | tail -5

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html
echo "HTML coverage report: coverage.html"

# 5. Integration Tests (if Docker is running)
echo -e "\n🔗 Phase 5: Integration Tests"
echo "-----------------------------"

if docker info > /dev/null 2>&1; then
    echo "Docker is running. Starting integration tests..."

    # Start required services
    docker-compose up -d postgres redis rabbitmq 2>/dev/null || true
    sleep 10

    if SKIP_INTEGRATION="" go test -v ./tests/integration_test.go 2>/dev/null; then
        echo -e "${GREEN}✅ Integration tests passed${NC}"
        ((PASSED++))
    else
        echo -e "${YELLOW}⚠️  Integration tests skipped (services not available)${NC}"
        ((SKIPPED++))
    fi

    # Cleanup
    docker-compose down 2>/dev/null || true
else
    echo -e "${YELLOW}Docker not running. Skipping integration tests.${NC}"
    ((SKIPPED++))
fi

# 6. Lint (if golangci-lint is installed)
echo -e "\n🔍 Phase 6: Linting"
echo "-------------------"

if command -v golangci-lint &> /dev/null; then
    run_test "Lint Check" "golangci-lint run --timeout=5m ./..."
else
    echo -e "${YELLOW}golangci-lint not installed. Skipping.${NC}"
    echo "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"
    ((SKIPPED++))
fi

# 7. Build Test
echo -e "\n🔨 Phase 7: Build Test"
echo "---------------------"

run_test "Build Main" "go build -o bin/runner cmd/runner/main.go"
run_test "Build All" "go build ./..."

# Summary
echo -e "\n${YELLOW}=================================${NC}"
echo -e "${YELLOW}📊 Test Summary${NC}"
echo -e "${YELLOW}=================================${NC}"
echo -e "${GREEN}✅ Passed:${NC} $PASSED"
echo -e "${RED}❌ Failed:${NC} $FAILED"
echo -e "${YELLOW}⏭️  Skipped:${NC} $SKIPPED"

if [ $FAILED -eq 0 ]; then
    echo -e "\n${GREEN}🎉 All tests passed!${NC}"
    exit 0
else
    echo -e "\n${RED}⚠️  Some tests failed. Please review the output above.${NC}"
    exit 1
fi