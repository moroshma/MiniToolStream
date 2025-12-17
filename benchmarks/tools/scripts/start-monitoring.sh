#!/bin/bash

# start-monitoring.sh
# Starts Prometheus, Grafana, and related monitoring infrastructure

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BENCHMARK_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "========================================="
echo "Starting Monitoring Infrastructure"
echo "========================================="
echo

cd "$BENCHMARK_ROOT"

# Check if Docker is running
if ! docker info >/dev/null 2>&1; then
    echo "Error: Docker is not running"
    echo "Please start Docker Desktop first"
    exit 1
fi

echo "Starting Prometheus, Grafana, and exporters..."
docker-compose -f docker-compose.monitoring.yml up -d

echo
echo "Waiting for services to be ready..."
sleep 10

# Check if services are running
echo
echo "Service Status:"
docker-compose -f docker-compose.monitoring.yml ps

echo
echo "========================================="
echo "Monitoring Stack Started Successfully!"
echo "========================================="
echo
echo "Access URLs:"
echo "  Prometheus:  http://localhost:9090"
echo "  Grafana:     http://localhost:3000"
echo "    - Username: admin"
echo "    - Password: admin"
echo "  Pushgateway: http://localhost:9091"
echo "  cAdvisor:    http://localhost:8081"
echo "  Node Exp:    http://localhost:9100/metrics"
echo
echo "Grafana Dashboard:"
echo "  Navigate to Dashboards -> Benchmarks -> MiniToolStream vs Kafka"
echo
