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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/internal/config"
	grpcHandler "github.com/moroshma/MiniToolStream/MiniToolStreamIngress/internal/delivery/grpc"
	minioRepo "github.com/moroshma/MiniToolStream/MiniToolStreamIngress/internal/repository/minio"
	tarantoolRepo "github.com/moroshma/MiniToolStream/MiniToolStreamIngress/internal/repository/tarantool"
	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/internal/usecase"
	"github.com/moroshma/MiniToolStream/MiniToolStreamIngress/pkg/logger"
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

	appLogger.Info("Starting MiniToolStream Ingress gRPC Server",
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
		appLogger.Info("Vault is disabled - using configuration file values")
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

	// Ensure bucket exists
	if err := storageRepo.EnsureBucket(ctx); err != nil {
		appLogger.Fatal("Failed to ensure MinIO bucket", logger.Error(err))
	}
	appLogger.Info("✓ Connected to MinIO")

	// Initialize use case
	publishUC := usecase.NewPublishUseCase(
		messageRepo,
		storageRepo,
		appLogger,
	)

	// Initialize gRPC handler
	ingressHandler := grpcHandler.NewIngressHandler(publishUC, appLogger)

	// Setup MinIO lifecycle policies for TTL if enabled
	if cfg.TTL.Enabled {
		appLogger.Info("Setting up MinIO TTL policies")
		if err := storageRepo.SetupTTLPolicies(ctx, cfg.TTL); err != nil {
			appLogger.Error("Failed to setup MinIO TTL policies", logger.Error(err))
		} else {
			appLogger.Info("MinIO TTL policies configured successfully")
		}
	}

	// Initialize Tarantool TTL background process
	if cfg.TTL.Enabled {
		appLogger.Info("Starting Tarantool TTL cleanup fiber")
		if err := messageRepo.StartTTLCleanup(cfg.TTL); err != nil {
			appLogger.Error("Failed to start Tarantool TTL cleanup", logger.Error(err))
		} else {
			appLogger.Info("Tarantool TTL cleanup fiber started")
		}
	}

	// Initialize JWT authentication if enabled
	var grpcServer *grpc.Server
	// Set max message size to 1GB (for large file transfers)
	maxMsgSize := 1024 * 1024 * 1024 // 1GB

	if cfg.Auth.Enabled {
		appLogger.Info("JWT authentication enabled",
			logger.String("issuer", cfg.Auth.JWTIssuer),
			logger.String("vault_path", cfg.Auth.JWTVaultPath),
		)

		// Import auth package
		jwtManager, err := initJWTManager(ctx, vaultClient.Client(), &cfg.Auth, appLogger)
		if err != nil {
			appLogger.Fatal("Failed to initialize JWT manager", logger.Error(err))
		}

		// Create gRPC server with JWT interceptors and increased message size limits
		grpcServer = grpc.NewServer(
			grpc.UnaryInterceptor(conditionalAuthInterceptor(jwtManager, cfg.Auth.RequireAuth)),
			grpc.MaxRecvMsgSize(maxMsgSize),
			grpc.MaxSendMsgSize(maxMsgSize),
		)
		appLogger.Info("✓ JWT authentication configured")
	} else {
		appLogger.Info("JWT authentication disabled")
		grpcServer = grpc.NewServer(
			grpc.MaxRecvMsgSize(maxMsgSize),
			grpc.MaxSendMsgSize(maxMsgSize),
		)
	}
	appLogger.Info("gRPC max message size configured", logger.Int("max_mb", maxMsgSize/(1024*1024)))

	pb.RegisterIngressServiceServer(grpcServer, ingressHandler)

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

// conditionalAuthInterceptor creates an interceptor that conditionally requires authentication
func conditionalAuthInterceptor(jwtManager *auth.JWTManager, requireAuth bool) grpc.UnaryServerInterceptor {
	if requireAuth {
		// Require authentication for all requests
		return auth.UnaryServerInterceptor(jwtManager)
	}

	// Optional authentication - validate if present, allow if not
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Try to get token from metadata
		claims, err := tryAuthenticate(ctx, jwtManager)
		if err != nil {
			// Token was provided but invalid - reject the request
			return nil, err
		}
		if claims != nil {
			ctx = context.WithValue(ctx, auth.ClaimsContextKey{}, claims)
		}
		return handler(ctx, req)
	}
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
	claims, err := jwtManager.ValidateToken(token)
	if err != nil {
		// Token was provided but invalid - return the error
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}
	return claims, nil
}
