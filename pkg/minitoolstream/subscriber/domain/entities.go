package domain

import "time"

// Message represents a message received from a subject
type Message struct {
	Subject   string
	Sequence  uint64
	Data      []byte
	Headers   map[string]string
	Timestamp time.Time
}

// Notification represents a notification about new messages
type Notification struct {
	Subject  string
	Sequence uint64
}

// SubscriptionConfig represents subscription configuration
type SubscriptionConfig struct {
	Subject       string
	DurableName   string
	StartSequence *uint64 // optional
	BatchSize     int32
}
