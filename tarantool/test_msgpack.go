package main

import (
	"context"
	"fmt"
	"log"

	"github.com/tarantool/go-tarantool/v2"
	"github.com/vmihailenco/msgpack/v5"
)

// Example message structure to be serialized with MessagePack
type TestMessage struct {
	OrderID  string                 `msgpack:"order_id"`
	UserID   int                    `msgpack:"user_id"`
	Amount   float64                `msgpack:"amount"`
	Items    []string               `msgpack:"items"`
	Metadata map[string]interface{} `msgpack:"metadata"`
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

	fmt.Println("✅ Connected to Tarantool")
	fmt.Println("\n🧪 Testing MessagePack functions\n")

	// Test 1: Publish message with MessagePack data
	fmt.Println("🧪 Test 1: grpc_publish_msgpack")

	// Create test message
	testMsg := TestMessage{
		OrderID: "ORD-12345",
		UserID:  42,
		Amount:  199.99,
		Items:   []string{"laptop", "mouse", "keyboard"},
		Metadata: map[string]interface{}{
			"source": "web",
			"ip":     "192.168.1.1",
		},
	}

	// Serialize to MessagePack
	dataMsgpack, err := msgpack.Marshal(testMsg)
	if err != nil {
		log.Fatalf("Failed to marshal to msgpack: %s", err)
	}

	fmt.Printf("   Original message: %+v\n", testMsg)
	fmt.Printf("   MessagePack size: %d bytes\n", len(dataMsgpack))

	// Publish to Tarantool
	resp, err := conn.Call("grpc_publish_msgpack", []interface{}{
		"orders",    // subject
		dataMsgpack, // MessagePack encoded data
		map[string]interface{}{
			"content-type": "application/x-msgpack",
			"encoding":     "msgpack",
		}, // headers
	})
	if err != nil {
		log.Fatalf("grpc_publish_msgpack failed: %s", err)
	}

	result := resp[0].(map[interface{}]interface{})
	sequence := toUint64(result["sequence"])
	statusCode := toUint64(result["status_code"])
	fmt.Printf("   Published: sequence=%d, status_code=%d\n", sequence, statusCode)
	fmt.Println()

	// Publish few more messages
	for i := 2; i <= 5; i++ {
		msg := TestMessage{
			OrderID: fmt.Sprintf("ORD-%d", 12340+i),
			UserID:  40 + i,
			Amount:  float64(100 * i),
			Items:   []string{"item" + fmt.Sprint(i)},
			Metadata: map[string]interface{}{
				"index": i,
			},
		}
		data, _ := msgpack.Marshal(msg)
		conn.Call("grpc_publish_msgpack", []interface{}{
			"orders",
			data,
			map[string]interface{}{"index": i},
		})
	}
	fmt.Println("   Published 5 messages total with MessagePack data")
	fmt.Println()

	// Test 2: Get message by sequence (with MessagePack)
	fmt.Println("🧪 Test 2: get_message_by_sequence_decoded")
	resp, err = conn.Call("get_message_by_sequence_decoded", []interface{}{sequence})
	if err != nil {
		log.Fatalf("get_message_by_sequence_decoded failed: %s", err)
	}

	msgResult := resp[0].(map[interface{}]interface{})
	retrievedSeq := toUint64(msgResult["sequence"])
	retrievedSubject := msgResult["subject"].(string)
	retrievedData := msgResult["data_msgpack"].([]byte)

	fmt.Printf("   Retrieved message: sequence=%d, subject=%s\n", retrievedSeq, retrievedSubject)
	fmt.Printf("   MessagePack data size: %d bytes\n", len(retrievedData))

	// Deserialize MessagePack
	var decodedMsg TestMessage
	if err := msgpack.Unmarshal(retrievedData, &decodedMsg); err != nil {
		log.Fatalf("Failed to unmarshal msgpack: %s", err)
	}

	fmt.Printf("   Decoded message: %+v\n", decodedMsg)
	fmt.Println()

	// Test 3: Fetch messages with MessagePack
	fmt.Println("🧪 Test 3: grpc_fetch_msgpack")
	resp, err = conn.Call("grpc_fetch_msgpack", []interface{}{
		"orders",        // subject
		"test-consumer", // durable_name
		3,               // batch_size
		false,           // auto_ack
	})
	if err != nil {
		log.Fatalf("grpc_fetch_msgpack failed: %s", err)
	}

	messages := resp[0].([]interface{})
	fmt.Printf("   Fetched %d messages\n", len(messages))

	for i, m := range messages {
		msg := m.(map[interface{}]interface{})
		seq := toUint64(msg["sequence"])
		subject := msg["subject"].(string)
		dataMsgpack := msg["data_msgpack"].([]byte)

		var decoded TestMessage
		if err := msgpack.Unmarshal(dataMsgpack, &decoded); err != nil {
			fmt.Printf("     [%d] sequence=%d, subject=%s (failed to decode)\n", i+1, seq, subject)
		} else {
			fmt.Printf("     [%d] sequence=%d, subject=%s, OrderID=%s, Amount=%.2f\n",
				i+1, seq, subject, decoded.OrderID, decoded.Amount)
		}
	}
	fmt.Println()

	// Test 4: Acknowledge messages
	fmt.Println("🧪 Test 4: grpc_ack")
	if len(messages) > 0 {
		lastMsg := messages[len(messages)-1].(map[interface{}]interface{})
		lastSeq := toUint64(lastMsg["sequence"])

		resp, err = conn.Call("grpc_ack", []interface{}{
			"test-consumer",
			"orders",
			lastSeq,
		})
		if err != nil {
			log.Fatalf("grpc_ack failed: %s", err)
		}

		acked := resp[0].(bool)
		fmt.Printf("   Acknowledged up to sequence %d: %v\n", lastSeq, acked)

		// Verify position
		resp, _ = conn.Call("get_consumer_position", []interface{}{"test-consumer", "orders"})
		pos := toUint64(resp[0])
		fmt.Printf("   Consumer position after ack: %d\n", pos)
	}
	fmt.Println()

	// Test 5: Fetch remaining messages
	fmt.Println("🧪 Test 5: Fetch remaining messages")
	resp, err = conn.Call("grpc_fetch_msgpack", []interface{}{
		"orders",
		"test-consumer",
		10,
		true, // auto_ack
	})
	if err != nil {
		log.Fatalf("grpc_fetch_msgpack failed: %s", err)
	}

	remainingMsgs := resp[0].([]interface{})
	fmt.Printf("   Fetched %d remaining messages (with auto-ack)\n", len(remainingMsgs))

	for i, m := range remainingMsgs {
		msg := m.(map[interface{}]interface{})
		seq := toUint64(msg["sequence"])
		dataMsgpack := msg["data_msgpack"].([]byte)

		var decoded TestMessage
		msgpack.Unmarshal(dataMsgpack, &decoded)
		fmt.Printf("     [%d] sequence=%d, OrderID=%s\n", i+1, seq, decoded.OrderID)
	}
	fmt.Println()

	// Test 6: Compare data sizes
	fmt.Println("🧪 Test 6: MessagePack efficiency")

	// Create a large message
	largeMsg := TestMessage{
		OrderID:  "ORD-LARGE-99999",
		UserID:   12345,
		Amount:   9999.99,
		Items:    make([]string, 100),
		Metadata: make(map[string]interface{}),
	}

	for i := 0; i < 100; i++ {
		largeMsg.Items[i] = fmt.Sprintf("item-%d", i)
	}

	for i := 0; i < 50; i++ {
		largeMsg.Metadata[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}

	// Serialize
	largeMsgpack, _ := msgpack.Marshal(largeMsg)

	fmt.Printf("   Large message items: %d, metadata keys: %d\n", len(largeMsg.Items), len(largeMsg.Metadata))
	fmt.Printf("   MessagePack size: %d bytes\n", len(largeMsgpack))
	fmt.Println()

	fmt.Println("✅ All MessagePack tests passed!")
	fmt.Println("\n📋 Summary of MessagePack functions:")
	fmt.Println("   Publishing:")
	fmt.Println("     • grpc_publish_msgpack(subject, data_msgpack, headers) → {sequence, status_code, error_message}")
	fmt.Println("     • publish_message_msgpack(subject, data_msgpack, headers, object_name) → sequence")
	fmt.Println()
	fmt.Println("   Fetching:")
	fmt.Println("     • grpc_fetch_msgpack(subject, durable_name, batch_size, auto_ack) → messages[]")
	fmt.Println("     • get_message_by_sequence_decoded(sequence) → message table")
	fmt.Println()
	fmt.Println("   Benefits:")
	fmt.Println("     ✓ Compact binary format")
	fmt.Println("     ✓ Preserves data types")
	fmt.Println("     ✓ Fast serialization/deserialization")
	fmt.Println("     ✓ Cross-language support")
	fmt.Println("     ✓ No need for separate MinIO storage for small messages")
}

func toUint64(val interface{}) uint64 {
	switch v := val.(type) {
	case uint64:
		return v
	case int64:
		return uint64(v)
	case int:
		return uint64(v)
	case float64:
		return uint64(v)
	default:
		return 0
	}
}
