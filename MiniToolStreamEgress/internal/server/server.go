package server

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/minio"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/tarantool"
	pb "github.com/moroshma/MiniToolStream/model"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// EgressServer implements the gRPC EgressService
type EgressServer struct {
	pb.UnimplementedEgressServiceServer
	tarantoolClient *tarantool.Client
	minioClient     *minio.Client
	pollInterval    time.Duration
}

// NewEgressServer creates a new gRPC server instance
func NewEgressServer(tarantoolClient *tarantool.Client, minioClient *minio.Client) *EgressServer {
	return &EgressServer{
		tarantoolClient: tarantoolClient,
		minioClient:     minioClient,
		pollInterval:    1 * time.Second, // Default polling interval
	}
}

// Subscribe implements the Subscribe RPC method with polling
// Sends notifications when new messages are available for the consumer
func (s *EgressServer) Subscribe(req *pb.SubscribeRequest, stream pb.EgressService_SubscribeServer) error {
	log.Printf("Subscribe: subject=%s, durable_name=%s, start_sequence=%v",
		req.Subject, req.DurableName, req.StartSequence)

	if req.Subject == "" {
		return fmt.Errorf("subject cannot be empty")
	}

	if req.DurableName == "" {
		return fmt.Errorf("durable_name cannot be empty")
	}

	// Get initial consumer position
	lastSequence, err := s.tarantoolClient.GetConsumerPosition(req.DurableName, req.Subject)
	if err != nil {
		return fmt.Errorf("failed to get consumer position: %w", err)
	}

	// If start_sequence is provided, use it instead
	if req.StartSequence != nil && *req.StartSequence > lastSequence {
		lastSequence = *req.StartSequence
	}

	log.Printf("Subscribe: starting from sequence %d", lastSequence)

	// Polling loop
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	// Send initial notification if there are messages
	latestSeq, err := s.tarantoolClient.GetLatestSequenceForSubject(req.Subject)
	if err != nil {
		return fmt.Errorf("failed to get latest sequence: %w", err)
	}

	if latestSeq > lastSequence {
		err = stream.Send(&pb.Notification{
			Subject:  req.Subject,
			Sequence: latestSeq,
		})
		if err != nil {
			return fmt.Errorf("failed to send notification: %w", err)
		}
		lastSequence = latestSeq
	}

	// Poll for new messages
	for {
		select {
		case <-stream.Context().Done():
			log.Printf("Subscribe: client disconnected")
			return nil
		case <-ticker.C:
			// Check for new messages
			latestSeq, err := s.tarantoolClient.GetLatestSequenceForSubject(req.Subject)
			if err != nil {
				log.Printf("Subscribe: error getting latest sequence: %v", err)
				continue
			}

			if latestSeq > lastSequence {
				// New messages available, send notification
				err = stream.Send(&pb.Notification{
					Subject:  req.Subject,
					Sequence: latestSeq,
				})
				if err != nil {
					return fmt.Errorf("failed to send notification: %w", err)
				}
				log.Printf("Subscribe: sent notification for sequence %d", latestSeq)
				lastSequence = latestSeq
			}
		}
	}
}

// Fetch implements the Fetch RPC method
// Fetches a batch of messages for a durable consumer
func (s *EgressServer) Fetch(req *pb.FetchRequest, stream pb.EgressService_FetchServer) error {
	log.Printf("Fetch: subject=%s, durable_name=%s, batch_size=%d",
		req.Subject, req.DurableName, req.BatchSize)

	if req.Subject == "" {
		return fmt.Errorf("subject cannot be empty")
	}

	if req.DurableName == "" {
		return fmt.Errorf("durable_name cannot be empty")
	}

	if req.BatchSize <= 0 {
		req.BatchSize = 10 // Default batch size
	}

	// Get consumer position
	lastSequence, err := s.tarantoolClient.GetConsumerPosition(req.DurableName, req.Subject)
	if err != nil {
		return fmt.Errorf("failed to get consumer position: %w", err)
	}

	log.Printf("Fetch: consumer at sequence %d", lastSequence)

	// Fetch messages from Tarantool starting after lastSequence
	messages, err := s.tarantoolClient.GetMessagesBySubject(req.Subject, lastSequence+1, int(req.BatchSize))
	if err != nil {
		return fmt.Errorf("failed to fetch messages: %w", err)
	}

	log.Printf("Fetch: found %d messages", len(messages))

	// Send each message
	for _, msg := range messages {
		// Get data from MinIO if object_name is present
		var data []byte
		if msg.ObjectName != "" {
			data, err = s.minioClient.GetObject(stream.Context(), msg.Subject, msg.ObjectName)
			if err != nil {
				log.Printf("Fetch: warning - failed to get data from MinIO for %s: %v", msg.ObjectName, err)
				// Continue anyway, send message without data
				data = nil
			}
		}

		// Convert create_at to timestamp
		timestamp := timestamppb.New(time.Unix(int64(msg.CreateAt), 0))

		// Send message to client
		err = stream.Send(&pb.Message{
			Subject:   msg.Subject,
			Sequence:  msg.Sequence,
			Data:      data,
			Headers:   msg.Headers,
			Timestamp: timestamp,
		})
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}

		// Update consumer position
		err = s.tarantoolClient.UpdateConsumerPosition(req.DurableName, req.Subject, msg.Sequence)
		if err != nil {
			log.Printf("Fetch: warning - failed to update consumer position: %v", err)
		}

		log.Printf("Fetch: sent message sequence=%d, data_size=%d", msg.Sequence, len(data))
	}

	return nil
}

// GetLastSequence implements the GetLastSequence RPC method
// Returns the latest sequence number for a subject
func (s *EgressServer) GetLastSequence(ctx context.Context, req *pb.GetLastSequenceRequest) (*pb.GetLastSequenceResponse, error) {
	log.Printf("GetLastSequence: subject=%s", req.Subject)

	if req.Subject == "" {
		return nil, fmt.Errorf("subject cannot be empty")
	}

	// Get latest sequence from Tarantool
	latestSeq, err := s.tarantoolClient.GetLatestSequenceForSubject(req.Subject)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest sequence: %w", err)
	}

	log.Printf("GetLastSequence: latest sequence=%d", latestSeq)

	return &pb.GetLastSequenceResponse{
		LastSequence: latestSeq,
	}, nil
}
