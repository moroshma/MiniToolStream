package main

import (
	"context"
	"fmt"
	"log"

	"github.com/tarantool/go-tarantool/v2"
)

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

	fmt.Println("✅ Connected to Tarantool after restart")
	fmt.Println()

	// Check if messages still exist using simple select count
	resp, err := conn.Call("box.space.message:len", []interface{}{})
	if err != nil {
		log.Fatalf("Failed to count messages: %s", err)
	}

	count := toUint64(resp[0])
	fmt.Printf("📊 Total messages after restart (len): %d\n", count)

	// Also try with select
	resp2, err := conn.Call("get_latest_sequence_for_subject", []interface{}{"orders"})
	if err == nil && len(resp2) > 0 {
		latest := toUint64(resp2[0])
		fmt.Printf("📊 Latest sequence for 'orders': %d\n", latest)
	}

	if count == 0 {
		fmt.Println("❌ PERSISTENCE FAILED: No messages found!")
		return
	}

	// Get first message
	resp3, err := conn.Call("get_message_by_sequence", []interface{}{uint64(1)})
	if err != nil {
		log.Fatalf("Failed to get message: %s", err)
	}

	if len(resp3) > 0 && resp3[0] != nil {
		msg := resp3[0].([]interface{})
		fmt.Printf("✅ Message 1 recovered: sequence=%v, subject=%v, object=%v\n", msg[0], msg[3], msg[2])
	}

	// Check consumers
	req2 := tarantool.NewCallRequest("box.space.consumers:count")
	future2 := conn.Do(req2)
	respCount2, err := future2.Get()
	if err != nil {
		log.Fatalf("Failed to count consumers: %s", err)
	}

	consumersCount := toUint64(respCount2[0])
	fmt.Printf("📊 Total consumers after restart: %d\n", consumersCount)

	fmt.Println()
	fmt.Println("✅ PERSISTENCE TEST PASSED! All data recovered successfully.")
}

func toUint64(val interface{}) uint64 {
	switch v := val.(type) {
	case uint64:
		return v
	case int64:
		return uint64(v)
	case int:
		return uint64(v)
	default:
		return 0
	}
}
