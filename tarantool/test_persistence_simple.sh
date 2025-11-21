#!/bin/bash

set -e

echo "====================================="
echo "Tarantool Persistence Test"
echo "====================================="
echo

# Step 1: Write data
echo "Step 1: Writing 3 messages..."
go run test_new_schema.go > /dev/null 2>&1
echo "  ✓ Data written (5 messages, 2 consumers)"
echo

# Step 2: Restart container
echo "Step 2: Restarting container..."
docker-compose stop > /dev/null 2>&1
echo "  ✓ Container stopped"
docker-compose start > /dev/null 2>&1
echo "  ✓ Container started"
echo "  ⏳ Waiting 7 seconds for full initialization..."
sleep 7
echo

# Step 3: Check data
echo "Step 3: Checking persistence..."
go run test_persistence_check.go

echo
echo "====================================="
echo "Test Complete!"
echo "====================================="
