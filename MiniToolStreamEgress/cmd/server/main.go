package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/minio"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/server"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/tarantool"
	pb "github.com/moroshma/MiniToolStream/model"
)

var (
	port              = flag.Int("port", 50052, "gRPC server port")
	tarantoolAddress  = flag.String("tarantool-addr", "localhost:3301", "Tarantool address")
	tarantoolUser     = flag.String("tarantool-user", "minitoolstream", "Tarantool user")
	tarantoolPassword = flag.String("tarantool-password", "changeme", "Tarantool password")
	minioEndpoint     = flag.String("minio-endpoint", "localhost:9000", "MinIO endpoint")
	minioAccessKey    = flag.String("minio-access-key", "minioadmin", "MinIO access key")
	minioSecretKey    = flag.String("minio-secret-key", "minioadmin", "MinIO secret key")
	minioUseSSL       = flag.Bool("minio-use-ssl", false, "Use SSL for MinIO")
)

func main() {
	flag.Parse()

	log.Printf("Starting MiniToolStream Egress gRPC Server...")
	log.Printf("Tarantool: %s", *tarantoolAddress)
	log.Printf("MinIO: %s", *minioEndpoint)
	log.Printf("gRPC Port: %d", *port)

	// Create Tarantool client
	tarantoolConfig := &tarantool.Config{
		Address:  *tarantoolAddress,
		User:     *tarantoolUser,
		Password: *tarantoolPassword,
		Timeout:  5 * time.Second,
	}

	tarantoolClient, err := tarantool.NewClient(tarantoolConfig)
	if err != nil {
		log.Fatalf("Failed to connect to Tarantool: %v", err)
	}
	defer tarantoolClient.Close()

	// Test Tarantool connection
	if err := tarantoolClient.Ping(); err != nil {
		log.Fatalf("Failed to ping Tarantool: %v", err)
	}
	log.Printf("✓ Connected to Tarantool")

	// Create MinIO client
	minioConfig := &minio.Config{
		Endpoint:        *minioEndpoint,
		AccessKeyID:     *minioAccessKey,
		SecretAccessKey: *minioSecretKey,
		UseSSL:          *minioUseSSL,
	}

	minioClient, err := minio.NewClient(minioConfig)
	if err != nil {
		log.Fatalf("Failed to create MinIO client: %v", err)
	}
	log.Printf("✓ Connected to MinIO")

	// Create gRPC server
	grpcServer := grpc.NewServer()
	egressServer := server.NewEgressServer(tarantoolClient, minioClient)
	pb.RegisterEgressServiceServer(grpcServer, egressServer)

	// Register reflection for grpcurl
	reflection.Register(grpcServer)

	// Start listening
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("✓ gRPC server listening on :%d", *port)
	log.Printf("Ready to accept requests...")

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
