package main

import (
	"context"
	"fmt"
	"log"

	"github.com/tarantool/go-tarantool/v2"
	"github.com/vmihailenco/msgpack/v5"
)

type SimpleMsg struct {
	Text string `msgpack:"text"`
	Num  int    `msgpack:"num"`
}

func main() {
	ctx := context.Background()
	conn, err := tarantool.Connect(ctx, tarantool.NetDialer{
		Address:  "localhost:3301",
		User:     "minitoolstream",
		Password: "changeme",
	}, tarantool.Opts{})
	if err != nil {
		log.Fatalf("Connection failed: %s", err)
	}
	defer conn.Close()

	fmt.Println("✅ Connected")

	// Test 1: Check if space exists by counting
	fmt.Println("\n🧪 Test 1: Check space (via count)")
	resp, err := conn.Call("box.space.message:len", []interface{}{})
	if err != nil {
		log.Printf("   Warning: Space may not exist: %s", err)
	} else {
		fmt.Printf("   Space exists with %v messages\n", resp[0])
	}

	// Test 2: Simple publish with msgpack
	fmt.Println("\n🧪 Test 2: Publish with MessagePack")
	msg := SimpleMsg{Text: "Hello", Num: 123}
	data, _ := msgpack.Marshal(msg)

	fmt.Printf("   Message: %+v\n", msg)
	fmt.Printf("   MessagePack bytes: %d\n", len(data))

	resp, err = conn.Call("grpc_publish_msgpack", []interface{}{
		"test",
		data,
		map[string]interface{}{"type": "test"},
	})
	if err != nil {
		log.Fatalf("Publish failed: %s", err)
	}

	result := resp[0].(map[interface{}]interface{})
	seq := result["sequence"]
	status := result["status_code"]
	errMsg := result["error_message"]
	fmt.Printf("   Published: sequence=%v, status=%v\n", seq, status)
	if status != 0 {
		fmt.Printf("   Error: %v\n", errMsg)
		return
	}

	// Test 3: Count messages
	fmt.Println("\n🧪 Test 3: Count messages")
	resp, err = conn.Call("box.space.message:count", []interface{}{})
	if err != nil {
		log.Fatalf("Count failed: %s", err)
	}
	fmt.Printf("   Total messages: %v\n", resp[0])

	// Test 4: Get by sequence
	fmt.Println("\n🧪 Test 4: Get message by sequence")
	resp, err = conn.Call("get_message_by_sequence", []interface{}{uint64(1)})
	if err != nil {
		log.Fatalf("Get failed: %s", err)
	}

	if resp[0] == nil {
		fmt.Println("   ❌ Message not found (nil)")
	} else {
		tuple := resp[0].([]interface{})
		fmt.Printf("   Message found: sequence=%v, subject=%v\n", tuple[0], tuple[3])
		if len(tuple) >= 6 && tuple[5] != nil {
			msgpackData := tuple[5].([]byte)
			fmt.Printf("   MessagePack size: %d bytes\n", len(msgpackData))

			var decoded SimpleMsg
			if err := msgpack.Unmarshal(msgpackData, &decoded); err == nil {
				fmt.Printf("   Decoded: %+v\n", decoded)
			}
		}
	}

	fmt.Println("\n✅ Tests complete")
}
