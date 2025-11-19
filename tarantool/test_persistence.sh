#!/bin/bash

set -e

echo "🧪 Testing Tarantool Data Persistence"
echo "========================================"

# Step 1: Insert test data
echo ""
echo "Step 1: Inserting test data..."
go run test_tarantool.go > /dev/null 2>&1
echo "✅ Test data inserted"

# Step 2: Count records before restart
echo ""
echo "Step 2: Checking data before restart..."
BEFORE=$(docker exec minitoolstream-tarantool tarantool -e "
local net_box = require('net.box')
local conn = net_box.connect('localhost:3301', {user='minitoolstream', password='changeme'})
local result = conn.space.messages:count()
conn:close()
print(result)
os.exit(0)
" 2>&1 | tail -1)

echo "   Records in messages space: $BEFORE"

# Step 3: Restart container
echo ""
echo "Step 3: Restarting container to test persistence..."
docker-compose restart > /dev/null 2>&1
echo "   Waiting for container to start..."
sleep 5

# Step 4: Count records after restart
echo ""
echo "Step 4: Checking data after restart..."
AFTER=$(docker exec minitoolstream-tarantool tarantool -e "
local net_box = require('net.box')
local conn = net_box.connect('localhost:3301', {user='minitoolstream', password='changeme', wait_connected=true, reconnect_after=0.1})
local result = conn.space.messages:count()
conn:close()
print(result)
os.exit(0)
" 2>&1 | tail -1)

echo "   Records in messages space: $AFTER"

# Step 5: Compare
echo ""
if [ "$BEFORE" == "$AFTER" ]; then
    echo "✅ SUCCESS: Data persisted across restart!"
    echo "   Before: $BEFORE records"
    echo "   After:  $AFTER records"
else
    echo "❌ FAILURE: Data was lost!"
    echo "   Before: $BEFORE records"
    echo "   After:  $AFTER records"
    exit 1
fi

# Step 6: Check WAL files
echo ""
echo "Step 6: Checking WAL and snapshot files..."
docker exec minitoolstream-tarantool sh -c "ls -lh /var/lib/tarantool/sys_env/default/instance-001/*.{xlog,snap} 2>/dev/null | tail -5"

echo ""
echo "🎉 Persistence test completed successfully!"
