package entity

import "time"

// Message represents a message entity in the domain
type Message struct {
	Sequence   uint64
	Subject    string
	Data       []byte
	Headers    map[string]string
	ObjectName string
	Timestamp  time.Time
}

// Consumer represents a durable consumer entity
type Consumer struct {
	DurableName  string
	Subject      string
	LastSequence uint64
}

// Notification represents a new message notification
type Notification struct {
	Subject  string
	Sequence uint64
}
