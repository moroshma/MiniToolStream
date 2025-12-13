package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	vault "github.com/hashicorp/vault/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"

	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/config"
	grpcHandler "github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/delivery/grpc"
	minioRepo "github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/repository/minio"
	tarantoolRepo "github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/repository/tarantool"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/internal/usecase"
	"github.com/moroshma/MiniToolStream/MiniToolStreamEgress/pkg/logger"
	"github.com/moroshma/MiniToolStreamConnector/auth"
	pb "github.com/moroshma/MiniToolStreamConnector/model"
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
	appLogger.Info("Vault configuration",
		logger.Bool("enabled", cfg.Vault.Enabled),
		logger.String("address", cfg.Vault.Address),
	)

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
	} else {
		appLogger.Info("Vault client is nil - secrets will not be loaded from Vault")
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
	appLogger.Info("Connecting to MinIO",
		logger.String("endpoint", cfg.MinIO.Endpoint),
		logger.String("bucket", cfg.MinIO.BucketName),
	)
	minioCfg := &minioRepo.Config{
		Endpoint:        cfg.MinIO.Endpoint,
		AccessKeyID:     cfg.MinIO.AccessKeyID,
		SecretAccessKey: cfg.MinIO.SecretAccessKey,
		UseSSL:          cfg.MinIO.UseSSL,
		BucketName:      cfg.MinIO.BucketName,
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

	// Initialize JWT authentication if enabled
	var grpcServer *grpc.Server
	if cfg.Auth.Enabled {
		appLogger.Info("JWT authentication enabled",
			logger.String("issuer", cfg.Auth.JWTIssuer),
			logger.String("vault_path", cfg.Auth.JWTVaultPath),
		)

		jwtManager, err := initJWTManager(ctx, vaultClient.Client(), &cfg.Auth, appLogger)
		if err != nil {
			appLogger.Fatal("Failed to initialize JWT manager", logger.Error(err))
		}

		// Create gRPC server with JWT interceptors (stream interceptor for Subscribe/Fetch)
		grpcServer = grpc.NewServer(
			grpc.StreamInterceptor(conditionalStreamAuthInterceptor(jwtManager, cfg.Auth.RequireAuth)),
		)
		appLogger.Info("✓ JWT authentication configured")
	} else {
		appLogger.Info("JWT authentication disabled")
		grpcServer = grpc.NewServer()
	}

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

// initJWTManager initializes JWT manager from Vault
func initJWTManager(ctx context.Context, vaultClient *vault.Client, cfg *config.AuthConfig, log *logger.Logger) (*auth.JWTManager, error) {
	if vaultClient == nil {
		return nil, fmt.Errorf("vault client is required for JWT authentication")
	}

	log.Info("Loading JWT keys from Vault", logger.String("path", cfg.JWTVaultPath))
	jwtManager, err := auth.NewJWTManagerFromVault(ctx, vaultClient, cfg.JWTVaultPath, cfg.JWTIssuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create JWT manager: %w", err)
	}

	return jwtManager, nil
}

// conditionalStreamAuthInterceptor creates a stream interceptor that conditionally requires authentication
func conditionalStreamAuthInterceptor(jwtManager *auth.JWTManager, requireAuth bool) grpc.StreamServerInterceptor {
	if requireAuth {
		// Require authentication for all requests
		return auth.StreamServerInterceptor(jwtManager)
	}

	// Optional authentication - validate if present, allow if not
	return func(
		srv interface{},
		stream grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// Try to get token from metadata
		claims, _ := tryAuthenticate(stream.Context(), jwtManager)
		if claims != nil {
			wrappedStream := &authenticatedStream{
				ServerStream: stream,
				ctx:          context.WithValue(stream.Context(), auth.ClaimsContextKey{}, claims),
			}
			return handler(srv, wrappedStream)
		}
		return handler(srv, stream)
	}
}

// authenticatedStream wraps grpc.ServerStream with authenticated context
type authenticatedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authenticatedStream) Context() context.Context {
	return s.ctx
}

// tryAuthenticate attempts to authenticate but doesn't fail if no token present
func tryAuthenticate(ctx context.Context, jwtManager *auth.JWTManager) (*auth.Claims, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, nil
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return nil, nil
	}

	token := values[0]
	if !strings.HasPrefix(token, "Bearer ") {
		return nil, nil
	}

	token = strings.TrimPrefix(token, "Bearer ")
	return jwtManager.ValidateToken(token)
}
