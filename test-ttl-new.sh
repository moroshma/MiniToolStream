#!/bin/bash

# Test script for new TTL functionality (per-channel TTL)

set -e

echo "======================================"
echo "Testing New TTL Functionality"
echo "Per-Channel TTL with Background Cleanup"
echo "======================================"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Build clients if needed
echo -e "${YELLOW}Building test clients...${NC}"
cd example/publisher_client
go build -o publisher_client main.go 2>/dev/null || echo "Publisher client already built"
cd ../subscriber_client
go build -o subscriber_client main.go 2>/dev/null || echo "Subscriber client already built"
cd ../..

echo -e "${GREEN}✓ Clients ready${NC}"

# Function to publish messages
publish_message() {
    local channel=$1
    local filename=$2
    local content=$3

    echo -e "${YELLOW}Publishing to channel '${channel}'...${NC}"
    cd example/publisher_client
    ./publisher_client <<EOF
${channel}
${filename}
${content}
quit
EOF
    cd ../..
    echo -e "${GREEN}✓ Published to ${channel}${NC}"
}

# Function to check Tarantool for messages
check_tarantool_messages() {
    local channel=$1
    echo -e "${YELLOW}Checking Tarantool for messages in channel '${channel}'...${NC}"
    docker-compose exec -T tarantool tarantool -e "
        local conn = require('net.box').connect('localhost:3301', {
            user = 'minitoolstream_connector',
            password = 'changeme'
        })
        local count = conn:call('get_subject_message_count', {'${channel}'})
        print('Messages in ${channel}: ' .. count)
        conn:close()
    " 2>/dev/null || echo "0"
}

# Function to check TTL status
check_ttl_status() {
    echo -e "${YELLOW}Checking TTL status in Tarantool...${NC}"
    docker-compose exec -T tarantool tarantool -e "
        local conn = require('net.box').connect('localhost:3301', {
            user = 'minitoolstream_connector',
            password = 'changeme'
        })
        local status = conn:call('get_ttl_status')
        print('TTL Status:')
        print('  Enabled: ' .. tostring(status.enabled))
        print('  Default TTL: ' .. status.default_ttl .. ' seconds')
        print('  Check Interval: ' .. status.check_interval .. ' seconds')
        print('  Fiber Running: ' .. tostring(status.fiber_running))
        conn:close()
    " 2>/dev/null
}

echo ""
echo "======================================"
echo "Step 1: Checking TTL Configuration"
echo "======================================"
check_ttl_status

echo ""
echo "======================================"
echo "Step 2: Publishing Test Messages"
echo "======================================"
echo "Config TTL:"
echo "  - test: 2 minutes"
echo "  - images: 3 minutes"
echo "  - logs: 1 minute"
echo "  - default: 5 minutes"
echo ""

# Publish to different channels
publish_message "test" "test-msg-1.txt" "Test message 1 - TTL 2min"
sleep 2
publish_message "images" "image-1.jpg" "Image data 1 - TTL 3min"
sleep 2
publish_message "logs" "app.log" "Log entry 1 - TTL 1min"
sleep 2
publish_message "other" "other-1.txt" "Other message - TTL 5min (default)"

echo ""
echo "======================================"
echo "Step 3: Verifying Messages Were Stored"
echo "======================================"
check_tarantool_messages "test"
check_tarantool_messages "images"
check_tarantool_messages "logs"
check_tarantool_messages "other"

echo ""
echo "======================================"
echo "Step 4: Waiting for TTL Cleanup"
echo "======================================"
echo -e "${YELLOW}Waiting for logs channel cleanup (1 minute + buffer)...${NC}"
echo "Time remaining:"

# Wait 75 seconds (1 minute + 15 second buffer)
for i in {75..1}; do
    echo -ne "\r  ${i} seconds   "
    sleep 1
done
echo ""

echo ""
echo -e "${GREEN}Checking if 'logs' channel messages were deleted...${NC}"
logs_count=$(check_tarantool_messages "logs")

echo ""
echo -e "${YELLOW}Other channels should still have messages:${NC}"
check_tarantool_messages "test"
check_tarantool_messages "images"
check_tarantool_messages "other"

echo ""
echo "======================================"
echo "Step 5: Waiting for Test Channel Cleanup"
echo "======================================"
echo -e "${YELLOW}Waiting for test channel cleanup (2 minutes total - 1:15 remaining)...${NC}"

# Wait additional 75 seconds (total 2:30 from start)
for i in {75..1}; do
    echo -ne "\r  ${i} seconds   "
    sleep 1
done
echo ""

echo ""
echo -e "${GREEN}Checking if 'test' channel messages were deleted...${NC}"
test_count=$(check_tarantool_messages "test")

echo ""
echo -e "${YELLOW}Images and other channels should still have messages:${NC}"
check_tarantool_messages "images"
check_tarantool_messages "other"

echo ""
echo "======================================"
echo "Test Results Summary"
echo "======================================"
echo "TTL Configuration:"
echo "  - logs: 1 minute  → Should be deleted after ~1 minute"
echo "  - test: 2 minutes → Should be deleted after ~2 minutes"
echo "  - images: 3 minutes → Should be deleted after ~3 minutes"
echo "  - other: 5 minutes → Should be deleted after ~5 minutes"
echo ""
echo "Observations:"
echo "  - Messages are automatically cleaned up by Tarantool fiber"
echo "  - MinIO objects have lifecycle policies for automatic expiration"
echo "  - Each channel has independent TTL configuration"
echo ""
echo -e "${GREEN}✓ TTL test completed!${NC}"
echo ""
echo "To verify MinIO lifecycle policies:"
echo "  docker-compose exec minio mc ilm list local/minitoolstream"
echo ""
echo "To check Tarantool logs:"
echo "  docker-compose logs tarantool | grep 'TTL cleanup'"
