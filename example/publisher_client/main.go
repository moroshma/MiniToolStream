package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	vault "github.com/hashicorp/vault/api"
	"github.com/moroshma/MiniToolStream/example/publisher_client/internal/config"
	"github.com/moroshma/MiniToolStreamConnector/minitoolstream_connector"
)

var (
	configPath = flag.String("config", "", "Path to configuration file (optional)")
	imagePath  = flag.String("image", "", "Path to image file (optional)")
	subject    = flag.String("subject", "", "Subject/channel name (optional, overrides config)")
	filePath   = flag.String("file", "", "Path to file (optional)")
	data       = flag.String("data", "", "Raw data to publish (optional)")
)

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Setup logging
	setupLogging(cfg.Logger)

	log.Printf("MiniToolStream Publisher Client")
	log.Printf("Server: %s", cfg.Client.ServerAddress)
	log.Printf("Timeout: %s", cfg.Client.Timeout)

	// Initialize Vault if enabled
	ctx := context.Background()
	vaultClient, err := config.NewVaultClient(&cfg.Vault)
	if err != nil {
		log.Fatalf("Failed to create Vault client: %v", err)
	}

	if vaultClient != nil {
		log.Printf("Vault enabled: %s", cfg.Vault.Address)
		if err := config.ApplyVaultSecrets(ctx, cfg, vaultClient); err != nil {
			log.Fatalf("Failed to apply Vault secrets: %v", err)
		}
		log.Printf("✓ Vault secrets applied")

		// Load JWT token from Vault if path is configured
		if cfg.Client.JWTVaultPath != "" {
			jwtToken, err := loadJWTFromVault(ctx, vaultClient, cfg.Client.JWTVaultPath)
			if err != nil {
				log.Printf("Warning: Failed to load JWT token from Vault: %v", err)
			} else {
				cfg.Client.JWTToken = jwtToken
				log.Printf("✓ JWT token loaded from Vault")
			}
		}
	}

	// Create publisher using the library with JWT support
	pubBuilder := minitoolstream_connector.NewPublisherBuilder(cfg.Client.ServerAddress)
	if cfg.Client.JWTToken != "" {
		pubBuilder = pubBuilder.WithJWTToken(cfg.Client.JWTToken)
		log.Printf("✓ JWT authentication enabled")
	}

	pub, err := pubBuilder.Build()
	if err != nil {
		log.Fatalf("Failed to create publisher: %v", err)
	}
	defer pub.Close()

	// Determine subject to use
	publishSubject := determineSubject(*subject, cfg.Client.DefaultSubject)

	// Register handlers based on flags
	registered := false

	if *imagePath != "" {
		if publishSubject == "" {
			log.Fatalf("Subject is required when using -image (use -subject or set default_subject in config)")
		}
		log.Printf("Publishing image: %s -> %s", *imagePath, publishSubject)
		pub.RegisterHandler(minitoolstream_connector.NewImageHandler(&minitoolstream_connector.ImageHandlerConfig{
			Subject:   publishSubject,
			ImagePath: *imagePath,
		}))
		registered = true
	}

	if *filePath != "" {
		if publishSubject == "" {
			log.Fatalf("Subject is required when using -file (use -subject or set default_subject in config)")
		}
		log.Printf("Publishing file: %s -> %s", *filePath, publishSubject)
		pub.RegisterHandler(minitoolstream_connector.NewFileHandler(&minitoolstream_connector.FileHandlerConfig{
			Subject:  publishSubject,
			FilePath: *filePath,
		}))
		registered = true
	}

	if *data != "" {
		if publishSubject == "" {
			log.Fatalf("Subject is required when using -data (use -subject or set default_subject in config)")
		}
		log.Printf("Publishing data: %d bytes -> %s", len(*data), publishSubject)
		pub.RegisterHandler(minitoolstream_connector.NewDataHandler(&minitoolstream_connector.DataHandlerConfig{
			Subject:     publishSubject,
			Data:        []byte(*data),
			ContentType: cfg.Client.DefaultContentType,
		}))
		registered = true
	}

	if !registered {
		log.Printf("No data to publish. Use -image, -file, or -data flags.")
		flag.Usage()
		os.Exit(1)
	}

	// Create context with configured timeout
	publishCtx, cancel := context.WithTimeout(ctx, cfg.Client.Timeout)
	defer cancel()

	// Publish all registered handlers
	if err := pub.PublishAll(publishCtx, nil); err != nil {
		log.Fatalf("Failed to publish: %v", err)
	}

	log.Printf("✓ Publisher client finished")
}

// determineSubject determines which subject to use (flag takes precedence over config)
func determineSubject(flagSubject, configSubject string) string {
	if flagSubject != "" {
		return flagSubject
	}
	return configSubject
}

// setupLogging configures logging based on configuration
func setupLogging(cfg config.LoggerConfig) {
	// Set log flags based on format
	switch cfg.Format {
	case "json":
		// For JSON format, we would use a structured logger (e.g., zap, zerolog)
		// For now, use standard format with timestamp
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	case "console":
		log.SetFlags(log.LstdFlags)
	default:
		log.SetFlags(log.LstdFlags)
	}

	// In a real application, you would also configure log level filtering
	// For now, log level is informational
	if cfg.Level == "debug" {
		log.SetPrefix("[DEBUG] ")
	}
}

// printUsage prints usage information
func printUsage() {
	fmt.Fprintf(os.Stderr, `MiniToolStream Publisher Client

Usage:
  %s [options]

Options:
`, os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, `
Examples:
  # Publish with config file
  %s -config config.yaml -data "Hello World"

  # Publish image
  %s -subject "images.test" -image photo.jpg

  # Publish file
  %s -subject "docs.pdf" -file document.pdf

  # Use Vault for configuration
  VAULT_ENABLED=true VAULT_TOKEN=xxx %s -data "Secret message"

Configuration:
  Configuration can be provided via:
  1. YAML config file (-config flag)
  2. Environment variables (CLIENT_*, VAULT_*)
  3. Command line flags (highest priority)

`, os.Args[0], os.Args[0], os.Args[0], os.Args[0])
}

func init() {
	flag.Usage = printUsage
}

// loadJWTFromVault loads JWT token from Vault
func loadJWTFromVault(ctx context.Context, vaultClient *vault.Client, path string) (string, error) {
	secret, err := vaultClient.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return "", fmt.Errorf("failed to read from Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("no data found at path %s", path)
	}

	// Try to get token from data.token or data.data.token (KV v2)
	var token string
	if data, ok := secret.Data["data"].(map[string]interface{}); ok {
		token, _ = data["token"].(string)
	} else {
		token, _ = secret.Data["token"].(string)
	}

	if token == "" {
		return "", fmt.Errorf("token not found in Vault secret")
	}

	return token, nil
}
