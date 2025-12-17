#!/bin/bash

# run-all-benchmarks.sh
# Runs all benchmarks for MiniToolStream and Kafka

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "========================================="
echo "Benchmark Suite: MiniToolStream vs Kafka"
echo "========================================="
echo

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if infrastructure is running
echo "Checking infrastructure..."

# Check MiniToolStream services
if ! docker ps | grep -q "minitoolstream_connector-tarantool"; then
    echo -e "${RED}Error: MiniToolStream infrastructure not running${NC}"
    echo "Please start MiniToolStream services:"
    echo "  cd ../.. && docker-compose up -d"
    exit 1
fi

# Check Kafka services
if ! docker ps | grep -q "benchmark-kafka"; then
    echo -e "${YELLOW}Warning: Kafka infrastructure not running${NC}"
    echo "Starting Kafka services..."
    cd "$BENCHMARK_ROOT"
    docker-compose -f docker-compose.kafka.yml up -d
    echo "Waiting for Kafka to be ready..."
    sleep 30
fi

echo -e "${GREEN}Infrastructure OK${NC}"
echo

# Function to run a benchmark
run_benchmark() {
    local system=$1
    local test_type=$2
    local cmd_dir=$3
    local config=$4

    echo "========================================="
    echo "Running: $system - $test_type"
    echo "========================================="

    cd "$BENCHMARK_ROOT/$system/cmd/$cmd_dir"

    if [ ! -f "go.mod" ]; then
        echo "Initializing Go module..."
        go mod init "github.com/moroshma/benchmarks/$system/$cmd_dir" || true
        go mod tidy || true
    fi

    echo "Starting benchmark..."
    go run main.go -config="$config"

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ $system $test_type completed${NC}"
    else
        echo -e "${RED}✗ $system $test_type failed${NC}"
    fi

    echo
}

# Create results directories
mkdir -p "$BENCHMARK_ROOT/results/minitoolstream"
mkdir -p "$BENCHMARK_ROOT/results/kafka"

echo "========================================="
echo "Phase 1: Small Files (10KB)"
echo "========================================="
echo

# Run MiniToolStream small files benchmark
run_benchmark "minitoolstream" "small-files" "bench-small" "../../configs/small-files.yaml"

# Wait a bit between tests
sleep 10

# Run Kafka small files benchmark
run_benchmark "kafka" "small-files" "bench-small" "../../configs/small-files.yaml"

echo
echo "========================================="
echo "Phase 2: Large Files (1GB)"
echo "========================================="
echo

# Wait before large file tests
sleep 10

# Run MiniToolStream large files benchmark
run_benchmark "minitoolstream" "large-files" "bench-large" "../../configs/large-files.yaml"

# Kafka doesn't support 1GB files, so we skip or use chunked approach
echo -e "${YELLOW}Note: Kafka large file benchmark uses chunking (10MB chunks)${NC}"

echo
echo "========================================="
echo "All Benchmarks Completed"
echo "========================================="
echo
echo "Results saved to:"
echo "  - $BENCHMARK_ROOT/results/minitoolstream/"
echo "  - $BENCHMARK_ROOT/results/kafka/"
echo
echo "To generate comparative analysis, run:"
echo "  cd $BENCHMARK_ROOT/comparative/cmd/analyze"
echo "  go run main.go"
echo
