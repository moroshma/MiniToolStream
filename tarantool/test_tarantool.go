package main

import (
	"context"
	"fmt"
	"log"
	"reflect"

	"github.com/tarantool/go-tarantool/v2"
)

// Helper to convert to uint64
func toUint64(val interface{}) uint64 {
	switch v := val.(type) {
	case uint64:
		return v
	case int64:
		return uint64(v)
	case int:
		return uint64(v)
	case int8:
		return uint64(v)
	case int16:
		return uint64(v)
	case int32:
		return uint64(v)
	case uint:
		return uint64(v)
	case uint8:
		return uint64(v)
	case uint16:
		return uint64(v)
	case uint32:
		return uint64(v)
	default:
		log.Fatalf("Cannot convert %v (type %s) to uint64", val, reflect.TypeOf(val))
		return 0
	}
}

func main() {
	ctx := context.Background()

	// Connect to Tarantool
	dialer := tarantool.NetDialer{
		Address:  "localhost:3301",
		User:     "minitoolstream",
		Password: "changeme",
	}

	opts := tarantool.Opts{}
	conn, err := tarantool.Connect(ctx, dialer, opts)
	if err != nil {
		log.Fatalf("Connection failed: %s", err)
	}
	defer conn.Close()

	fmt.Println("✅ Connected to Tarantool successfully")

	// Test 1: Insert a message
	fmt.Println("\n🧪 Test 1: Insert message")
	resp, err := conn.Call("insert_message", []interface{}{
		"test-channel",
		"minio/key/test-123",
		"application/json",
		1024,
	})
	if err != nil {
		log.Fatalf("Insert failed: %s", err)
	}

	sequence := toUint64(resp[0])
	fmt.Printf("   Inserted message with sequence: %d\n", sequence)

	// Test 2: Get message back
	fmt.Println("\n🧪 Test 2: Get message")
	resp, err = conn.Call("get_message", []interface{}{
		"test-channel",
		sequence,
	})
	if err != nil {
		log.Fatalf("Get message failed: %s", err)
	}

	message := resp[0].([]interface{})
	fmt.Printf("   Retrieved message: %v\n", message)

	// Test 3: Get latest sequence
	fmt.Println("\n🧪 Test 3: Get latest sequence")
	resp, err = conn.Call("get_latest_sequence", []interface{}{
		"test-channel",
	})
	if err != nil {
		log.Fatalf("Get latest sequence failed: %s", err)
	}

	latest := toUint64(resp[0])
	fmt.Printf("   Latest sequence for test-channel: %d\n", latest)

	// Test 4: Insert multiple messages
	fmt.Println("\n🧪 Test 4: Insert multiple messages")
	for i := 0; i < 5; i++ {
		resp, err := conn.Call("insert_message", []interface{}{
			"test-channel",
			fmt.Sprintf("minio/key/test-%d", i),
			"application/json",
			512,
		})
		if err != nil {
			log.Fatalf("Insert %d failed: %s", i, err)
		}
		seq := toUint64(resp[0])
		fmt.Printf("   Inserted message %d with sequence: %d\n", i, seq)
	}

	// Test 5: Get messages range
	fmt.Println("\n🧪 Test 5: Get messages range")
	resp, err = conn.Call("get_messages_range", []interface{}{
		"test-channel",
		uint64(1),
		10,
	})
	if err != nil {
		log.Fatalf("Get range failed: %s", err)
	}

	messages := resp[0].([]interface{})
	fmt.Printf("   Retrieved %d messages\n", len(messages))
	for i, msg := range messages {
		msgData := msg.([]interface{})
		fmt.Printf("   Message %d: sequence=%v, channel=%v, key=%v\n",
			i, msgData[0], msgData[1], msgData[2])
	}

	// Test 6: Test with different channel
	fmt.Println("\n🧪 Test 6: Test with different channel")
	resp, err = conn.Call("insert_message", []interface{}{
		"another-channel",
		"minio/key/another-1",
		"text/plain",
		256,
	})
	if err != nil {
		log.Fatalf("Insert to another channel failed: %s", err)
	}

	anotherSeq := toUint64(resp[0])
	fmt.Printf("   Inserted message to another-channel with sequence: %d\n", anotherSeq)

	resp, err = conn.Call("get_latest_sequence", []interface{}{
		"another-channel",
	})
	if err != nil {
		log.Fatalf("Get latest for another channel failed: %s", err)
	}

	anotherLatest := toUint64(resp[0])
	fmt.Printf("   Latest sequence for another-channel: %d\n", anotherLatest)

	fmt.Println("\n✅ All tests passed!")
}
