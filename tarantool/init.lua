-- MiniToolStream Tarantool 2.11 initialization script
-- Standalone mode (single node, no replication)

print('MiniToolStream: Starting initialization...')

-- Configure Tarantool
box.cfg {
    listen = '0.0.0.0:3301',

    -- Memory settings
    memtx_memory = 1024 * 1024 * 1024, -- 1GB

    -- WAL (Write-Ahead Log) settings for durability
    wal_mode = 'write',
    wal_dir_rescan_delay = 2,

    -- Standalone mode - no replication
    read_only = false,

    -- Logging
    log_level = 5
}

print('MiniToolStream: box.cfg complete')

-- Wait for database to be ready
box.once('init', function()
    print('MiniToolStream: Running box.once init...')

    -- Space 1: message
    -- Stores metadata about each message in the stream
    -- Now supports MessagePack encoded data storage
    local message = box.schema.space.create('message', {
        if_not_exists = true,
        engine = 'memtx',
        format = {
            {name = 'sequence', type = 'unsigned'},      -- Unique message number (PK)
            {name = 'headers', type = 'map'},            -- Message headers (metadata)
            {name = 'object_name', type = 'string'},     -- S3/MinIO object key (optional, for large payloads)
            {name = 'subject', type = 'string'},         -- Topic/channel name
            {name = 'create_at', type = 'unsigned'},     -- Unix timestamp for TTL
            {name = 'data_msgpack', type = 'scalar', is_nullable = true}  -- MessagePack encoded payload (binary data)
        }
    })

    -- Primary index: sequence (unique, globally incrementing)
    message:create_index('primary', {
        parts = {'sequence'},
        if_not_exists = true,
        unique = true,
        type = 'TREE'
    })

    -- Secondary index: by subject (for filtering by topic)
    message:create_index('subject', {
        parts = {'subject'},
        if_not_exists = true,
        unique = false,
        type = 'TREE'
    })

    -- Secondary index: by subject + sequence (for range queries)
    message:create_index('subject_sequence', {
        parts = {'subject', 'sequence'},
        if_not_exists = true,
        unique = true,
        type = 'TREE'
    })

    -- Secondary index: by create_at (for TTL cleanup)
    message:create_index('create_at', {
        parts = {'create_at'},
        if_not_exists = true,
        unique = false,
        type = 'TREE'
    })

    -- Space 2: consumers
    -- Stores state (read position) for each durable consumer
    local consumers = box.schema.space.create('consumers', {
        if_not_exists = true,
        engine = 'memtx',
        format = {
            {name = 'durable_name', type = 'string'},    -- Consumer group name (part of composite PK)
            {name = 'subject', type = 'string'},         -- Topic subscribed to (part of composite PK)
            {name = 'last_sequence', type = 'unsigned'}  -- Last read message sequence
        }
    })

    -- Primary index: composite key (durable_name, subject)
    consumers:create_index('primary', {
        parts = {'durable_name', 'subject'},
        if_not_exists = true,
        unique = true,
        type = 'TREE'
    })

    -- Secondary index: by subject (for finding all consumers of a topic)
    consumers:create_index('subject', {
        parts = {'subject'},
        if_not_exists = true,
        unique = false,
        type = 'TREE'
    })

    print('MiniToolStream: Spaces and indexes created successfully')
end)

-- Global sequence counter (in-memory, atomically incremented)
local global_sequence = 0

-- Initialize global sequence from existing data
-- This runs on EVERY start to restore sequence from persisted data
local function init_global_sequence()
    local max_seq = box.space.message.index.primary:max()
    if max_seq ~= nil then
        global_sequence = max_seq[1]
    end
    print('MiniToolStream: Global sequence initialized to ' .. global_sequence)
end

-- Call immediately after box.cfg
init_global_sequence()

-- Function to get next global sequence (thread-safe)
function get_next_sequence()
    global_sequence = global_sequence + 1
    return global_sequence
end

-- Function to publish a message (legacy - for backward compatibility)
-- @param subject string - topic/channel name
-- @param object_name string - MinIO object key
-- @param headers table - map of headers (metadata)
-- @return sequence number of the published message
function publish_message(subject, object_name, headers)
    local sequence = get_next_sequence()
    local create_at = os.time()

    box.space.message:insert({
        sequence,
        headers or {},
        object_name,
        subject,
        create_at,
        nil  -- data_msgpack is null for legacy API
    })

    return sequence
end

-- Function to publish a message with MessagePack data
-- @param subject string - topic/channel name
-- @param data_msgpack string - MessagePack encoded binary data
-- @param headers table - map of headers (metadata)
-- @param object_name string - MinIO object key (optional, for large payloads)
-- @return sequence number of the published message
function publish_message_msgpack(subject, data_msgpack, headers, object_name)
    local sequence = get_next_sequence()
    local create_at = os.time()

    box.space.message:insert({
        sequence,
        headers or {},
        object_name or "",  -- empty string if not using MinIO
        subject,
        create_at,
        data_msgpack
    })

    return sequence
end

-- Function to get message by sequence
-- @param sequence uint64 - message sequence number
-- @return tuple or nil
function get_message_by_sequence(sequence)
    return box.space.message:get(sequence)
end

-- Function to get message by sequence with MessagePack data decoded
-- Returns message as a table with all fields named
-- @param sequence uint64 - message sequence number
-- @return table {sequence, headers, object_name, subject, create_at, data_msgpack} or nil
function get_message_by_sequence_decoded(sequence)
    local tuple = box.space.message:get(sequence)
    if tuple == nil then
        return nil
    end

    return {
        sequence = tuple[1],
        headers = tuple[2],
        object_name = tuple[3],
        subject = tuple[4],
        create_at = tuple[5],
        data_msgpack = tuple[6]  -- still binary, but accessible by name
    }
end

-- Function to get messages by subject
-- @param subject string - topic name
-- @param start_sequence uint64 - starting sequence (inclusive)
-- @param limit number - max messages to return
-- @return array of tuples
function get_messages_by_subject(subject, start_sequence, limit)
    local messages = {}
    local count = 0

    for _, tuple in box.space.message.index.subject_sequence:pairs({subject, start_sequence}) do
        if tuple[4] ~= subject then
            break
        end

        if count >= limit then
            break
        end

        table.insert(messages, tuple)
        count = count + 1
    end

    return messages
end

-- Function to get latest sequence for a subject
-- @param subject string - topic name
-- @return uint64 - latest sequence or 0
function get_latest_sequence_for_subject(subject)
    local max_tuple = box.space.message.index.subject_sequence:max({subject})
    if max_tuple == nil or max_tuple[4] ~= subject then
        return 0
    end
    return max_tuple[1]
end

-- Function to update consumer position
-- @param durable_name string - consumer group name
-- @param subject string - topic name
-- @param last_sequence uint64 - last read sequence
function update_consumer_position(durable_name, subject, last_sequence)
    local key = {durable_name, subject}
    local existing = box.space.consumers:get(key)

    if existing == nil then
        box.space.consumers:insert({durable_name, subject, last_sequence})
    else
        box.space.consumers:update(key, {{'=', 3, last_sequence}})
    end

    return true
end

-- Function to get consumer position
-- @param durable_name string - consumer group name
-- @param subject string - topic name
-- @return uint64 - last read sequence or 0
function get_consumer_position(durable_name, subject)
    local tuple = box.space.consumers:get({durable_name, subject})
    if tuple == nil then
        return 0
    end
    return tuple[3]
end

-- Function to get all consumers for a subject
-- @param subject string - topic name
-- @return array of tuples
function get_consumers_by_subject(subject)
    local result = {}
    for _, tuple in box.space.consumers.index.subject:pairs(subject) do
        table.insert(result, tuple)
    end
    return result
end

-- Function to delete old messages (TTL cleanup)
-- @param ttl_seconds number - time to live in seconds
-- @return deleted_count, array of deleted message info
function delete_old_messages(ttl_seconds)
    local current_time = os.time()
    local cutoff_time = current_time - ttl_seconds
    local deleted_count = 0
    local deleted_messages = {}

    for _, tuple in box.space.message.index.create_at:pairs() do
        if tuple[5] < cutoff_time then
            table.insert(deleted_messages, {
                sequence = tuple[1],
                subject = tuple[4],
                object_name = tuple[3]
            })
            box.space.message:delete(tuple[1])
            deleted_count = deleted_count + 1
        end
    end

    return deleted_count, deleted_messages
end

-- ==============================================================================
-- Additional functions for gRPC API support
-- ==============================================================================

-- Function for IngressService.Publish (legacy - using MinIO)
-- Publishes a message and returns sequence with status
-- @param subject string - topic/channel name
-- @param object_name string - MinIO object key (or data identifier)
-- @param headers table - map of headers (metadata)
-- @return table {sequence, status_code, error_message}
function grpc_publish(subject, object_name, headers)
    local success, result = pcall(function()
        local sequence = get_next_sequence()
        local create_at = os.time()

        box.space.message:insert({
            sequence,
            headers or {},
            object_name,
            subject,
            create_at,
            nil  -- no msgpack data
        })

        return sequence
    end)

    if success then
        return {
            sequence = result,
            status_code = 0,
            error_message = nil
        }
    else
        return {
            sequence = 0,
            status_code = 1,
            error_message = tostring(result)
        }
    end
end

-- Function for IngressService.Publish with MessagePack
-- Publishes a message with MessagePack encoded data
-- @param subject string - topic/channel name
-- @param data_msgpack string - MessagePack encoded binary data (from protobuf bytes)
-- @param headers table - map of headers (metadata)
-- @return table {sequence, status_code, error_message}
function grpc_publish_msgpack(subject, data_msgpack, headers)
    local success, result = pcall(function()
        local sequence = get_next_sequence()
        local create_at = os.time()

        -- Validate that data_msgpack is provided
        if not data_msgpack or #data_msgpack == 0 then
            error("data_msgpack is required and cannot be empty")
        end

        box.space.message:insert({
            sequence,
            headers or {},
            "",  -- no object_name when using inline data
            subject,
            create_at,
            data_msgpack
        })

        return sequence
    end)

    if success then
        return {
            sequence = result,
            status_code = 0,
            error_message = nil
        }
    else
        return {
            sequence = 0,
            status_code = 1,
            error_message = tostring(result)
        }
    end
end

-- Function for EgressService.Fetch
-- Fetches batch of messages for a consumer and optionally updates position
-- @param subject string - topic name
-- @param durable_name string - consumer group name
-- @param batch_size number - max messages to fetch
-- @param auto_ack boolean - automatically update consumer position (default: false)
-- @return array of message tuples
function grpc_fetch(subject, durable_name, batch_size, auto_ack)
    -- Get current consumer position
    local current_pos = get_consumer_position(durable_name, subject)
    local start_sequence = current_pos + 1

    -- Fetch messages
    local messages = get_messages_by_subject(subject, start_sequence, batch_size)

    -- Auto-acknowledge: update position to last fetched message
    if auto_ack and #messages > 0 then
        local last_message = messages[#messages]
        local last_seq = last_message[1]
        update_consumer_position(durable_name, subject, last_seq)
    end

    return messages
end

-- Function for EgressService.Fetch with MessagePack support
-- Returns messages as array of structured tables (easier to work with)
-- @param subject string - topic name
-- @param durable_name string - consumer group name
-- @param batch_size number - max messages to fetch
-- @param auto_ack boolean - automatically update consumer position (default: false)
-- @return array of message tables {sequence, headers, object_name, subject, create_at, data_msgpack}
function grpc_fetch_msgpack(subject, durable_name, batch_size, auto_ack)
    -- Get current consumer position
    local current_pos = get_consumer_position(durable_name, subject)
    local start_sequence = current_pos + 1

    -- Fetch messages
    local tuples = get_messages_by_subject(subject, start_sequence, batch_size)

    -- Convert tuples to structured tables
    local messages = {}
    for _, tuple in ipairs(tuples) do
        table.insert(messages, {
            sequence = tuple[1],
            headers = tuple[2],
            object_name = tuple[3],
            subject = tuple[4],
            create_at = tuple[5],
            data_msgpack = tuple[6]  -- MessagePack binary data
        })
    end

    -- Auto-acknowledge: update position to last fetched message
    if auto_ack and #messages > 0 then
        local last_seq = messages[#messages].sequence
        update_consumer_position(durable_name, subject, last_seq)
    end

    return messages
end

-- Function for EgressService.GetLastSequence
-- Already exists as get_latest_sequence_for_subject, but adding alias for clarity
function grpc_get_last_sequence(subject)
    return {
        last_sequence = get_latest_sequence_for_subject(subject)
    }
end

-- Function to get new messages count since consumer position
-- Useful for Subscribe notifications
-- @param subject string - topic name
-- @param durable_name string - consumer group name (optional)
-- @param since_sequence uint64 - check messages after this sequence (optional)
-- @return uint64 - count of new messages
function get_new_messages_count(subject, durable_name, since_sequence)
    local start_seq

    if durable_name then
        -- Use consumer position
        start_seq = get_consumer_position(durable_name, subject)
    elseif since_sequence then
        -- Use provided sequence
        start_seq = since_sequence
    else
        -- Get all messages count
        start_seq = 0
    end

    local latest_seq = get_latest_sequence_for_subject(subject)

    if latest_seq > start_seq then
        return latest_seq - start_seq
    else
        return 0
    end
end

-- Function to check if new messages are available
-- Useful for Subscribe stream to notify about new messages
-- @param subject string - topic name
-- @param consumer_group string - consumer group name
-- @return table {has_new, latest_sequence, consumer_position}
function check_new_messages(subject, consumer_group)
    local latest_seq = get_latest_sequence_for_subject(subject)
    local consumer_pos = get_consumer_position(consumer_group, subject)

    return {
        has_new = latest_seq > consumer_pos,
        latest_sequence = latest_seq,
        consumer_position = consumer_pos,
        new_count = math.max(0, latest_seq - consumer_pos)
    }
end

-- Function to acknowledge (commit) message consumption
-- Updates consumer position to specific sequence
-- @param durable_name string - consumer group name
-- @param subject string - topic name
-- @param sequence uint64 - sequence to acknowledge up to (inclusive)
-- @return boolean - success
function grpc_ack(durable_name, subject, sequence)
    local current_pos = get_consumer_position(durable_name, subject)

    -- Only update if new sequence is greater
    if sequence > current_pos then
        update_consumer_position(durable_name, subject, sequence)
        return true
    end

    return false
end

-- Function to get message count in a subject
-- @param subject string - topic name
-- @return uint64 - total message count for subject
function get_subject_message_count(subject)
    local count = 0
    for _, _ in box.space.message.index.subject:pairs(subject) do
        count = count + 1
    end
    return count
end

-- Function to peek next messages without updating consumer position
-- Useful for previewing messages before committing
-- @param subject string - topic name
-- @param durable_name string - consumer group name
-- @param batch_size number - max messages to peek
-- @return array of message tuples
function grpc_peek(subject, durable_name, batch_size)
    local current_pos = get_consumer_position(durable_name, subject)
    local start_sequence = current_pos + 1

    return get_messages_by_subject(subject, start_sequence, batch_size)
end

-- ==============================================================================
-- End of gRPC API support functions
-- ==============================================================================

-- Create user for application access
box.once('create_app_user', function()
    box.schema.user.create('minitoolstream', {
        password = 'changeme',
        if_not_exists = true
    })

    box.schema.user.grant('minitoolstream', 'read,write,execute', 'universe', nil, {
        if_not_exists = true
    })

    print('MiniToolStream: Application user created')
end)

print('MiniToolStream: Initialization complete - ready to accept requests')
