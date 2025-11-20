package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/config"
	grpcHandler "github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/delivery/grpc"
	minioRepo "github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/repository/minio"
	tarantoolRepo "github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/repository/tarantool"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/usecase"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/pkg/logger"
	pb "github.com/moroshma/MiniToolStream/model"
)

var (
	configPath = flag.String("config", "", "Path to configuration file (optional)")
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	appLogger, err := logger.New(logger.Config{
		Level:      cfg.Logger.Level,
		Format:     cfg.Logger.Format,
		OutputPath: cfg.Logger.OutputPath,
	})
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer appLogger.Sync()

	appLogger.Info("Starting MiniToolStream Egress gRPC Server",
		logger.String("version", "1.0.0"),
		logger.Int("grpc_port", cfg.Server.Port),
	)

	// Initialize Vault client if enabled
	ctx := context.Background()
	vaultClient, err := config.NewVaultClient(&cfg.Vault)
	if err != nil {
		appLogger.Fatal("Failed to create Vault client", logger.Error(err))
	}

	// Apply Vault secrets to configuration
	if vaultClient != nil {
		appLogger.Info("Loading secrets from Vault")
		if err := config.ApplyVaultSecrets(ctx, cfg, vaultClient); err != nil {
			appLogger.Fatal("Failed to apply Vault secrets", logger.Error(err))
		}
		appLogger.Info("Secrets loaded from Vault successfully")
	}

	// Initialize Tarantool repository
	appLogger.Info("Connecting to Tarantool", logger.String("address", cfg.Tarantool.Address))
	tarantoolCfg := &tarantoolRepo.Config{
		Address:  cfg.Tarantool.Address,
		User:     cfg.Tarantool.User,
		Password: cfg.Tarantool.Password,
		Timeout:  cfg.Tarantool.Timeout,
	}

	messageRepo, err := tarantoolRepo.NewRepository(tarantoolCfg, appLogger)
	if err != nil {
		appLogger.Fatal("Failed to connect to Tarantool", logger.Error(err))
	}
	defer messageRepo.Close()

	// Test Tarantool connection
	if err := messageRepo.Ping(); err != nil {
		appLogger.Fatal("Failed to ping Tarantool", logger.Error(err))
	}
	appLogger.Info("✓ Connected to Tarantool")

	// Initialize MinIO repository
	appLogger.Info("Connecting to MinIO", logger.String("endpoint", cfg.MinIO.Endpoint))
	minioCfg := &minioRepo.Config{
		Endpoint:        cfg.MinIO.Endpoint,
		AccessKeyID:     cfg.MinIO.AccessKeyID,
		SecretAccessKey: cfg.MinIO.SecretAccessKey,
		UseSSL:          cfg.MinIO.UseSSL,
	}

	storageRepo, err := minioRepo.NewRepository(minioCfg, appLogger)
	if err != nil {
		appLogger.Fatal("Failed to create MinIO client", logger.Error(err))
	}
	appLogger.Info("✓ Connected to MinIO")

	// Initialize use case
	messageUC := usecase.NewMessageUseCase(
		messageRepo,
		storageRepo,
		appLogger,
		cfg.Server.PollInterval,
	)

	// Initialize gRPC handler
	egressHandler := grpcHandler.NewEgressHandler(messageUC, appLogger)

	// Create gRPC server
	grpcServer := grpc.NewServer()
	pb.RegisterEgressServiceServer(grpcServer, egressHandler)

	// Register reflection for grpcurl
	reflection.Register(grpcServer)

	// Start listening
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.Port))
	if err != nil {
		appLogger.Fatal("Failed to listen", logger.Error(err), logger.Int("port", cfg.Server.Port))
	}

	appLogger.Info("✓ gRPC server listening", logger.Int("port", cfg.Server.Port))
	appLogger.Info("Ready to accept requests...")

	// Handle graceful shutdown
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		appLogger.Info("Received shutdown signal, shutting down gracefully...")
		grpcServer.GracefulStop()
	}()

	// Start serving
	if err := grpcServer.Serve(listener); err != nil {
		appLogger.Fatal("Failed to serve", logger.Error(err))
	}
}
