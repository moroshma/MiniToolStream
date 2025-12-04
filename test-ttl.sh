#!/bin/bash

# Test script for TTL functionality

set -e

echo "======================================"
echo "Testing TTL Functionality"
echo "======================================"

# Wait for services to be ready
echo "Waiting for services to start..."
sleep 10

# Build publisher and subscriber clients
echo "Building test clients..."
cd example/publisher_client
go build -o publisher_client main.go
cd ../subscriber_client
go build -o subscriber_client main.go
cd ../..

echo "======================================"
echo "Step 1: Publishing test messages"
echo "======================================"

# Publish some test messages
cd example/publisher_client
./publisher_client <<EOF
images
test-message-1.txt
This is test message 1
images
test-message-2.txt
This is test message 2
images
test-message-3.txt
This is test message 3
quit
EOF

cd ../..

echo "======================================"
echo "Step 2: Waiting for TTL cleanup (6 minutes)"
echo "======================================"
echo "TTL is set to 5 minutes, cleanup runs every 1 minute"
echo "Waiting 6 minutes to ensure cleanup has run..."

for i in {360..1}; do
  if [ $((i % 60)) -eq 0 ]; then
    echo "  $((i / 60)) minutes remaining..."
  fi
  sleep 1
done

echo "======================================"
echo "Step 3: Checking if messages were deleted"
echo "======================================"

# Try to subscribe - should not find old messages
cd example/subscriber_client
./subscriber_client <<EOF
images
test-subscriber
quit
EOF

cd ../..

echo "======================================"
echo "Test complete!"
echo "======================================"
echo "Check the logs to verify:"
echo "1. Messages were published successfully"
echo "2. TTL cleanup ran and deleted old messages"
echo "3. Subscriber did not receive old messages"
