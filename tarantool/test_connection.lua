local fiber = require('fiber')
local net_box = require('net.box')

-- Connect to Tarantool
local conn = net_box.connect('localhost:3301', {
    wait_connected = true,
    reconnect_after = 0.1
})

if not conn:is_connected() then
    print('ERROR: Cannot connect to Tarantool')
    os.exit(1)
end

print('Connected to Tarantool successfully')

-- Check if spaces exist
local messages_exists = conn.space.messages ~= nil
local sequences_exists = conn.space.sequences ~= nil

print('Space messages exists:', messages_exists)
print('Space sequences exists:', sequences_exists)

if messages_exists and sequences_exists then
    print('All spaces created successfully!')

    -- Test insert_message function
    local seq = conn:call('insert_message', {
        'test-channel',
        'minio/key/test-123',
        'application/json',
        1024
    })

    print('Inserted message with sequence:', seq[1])

    -- Get message back
    local msg = conn:call('get_message', {'test-channel', seq[1]})
    print('Retrieved message:', msg[1])

    -- Get latest sequence
    local latest = conn:call('get_latest_sequence', {'test-channel'})
    print('Latest sequence for test-channel:', latest[1])
else
    print('ERROR: Spaces not created!')
end

conn:close()
os.exit(0)
