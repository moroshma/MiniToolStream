package config

import (
	"fmt"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Tarantool TarantoolConfig `yaml:"tarantool"`
	MinIO     MinIOConfig     `yaml:"minio"`
	Vault     VaultConfig     `yaml:"vault"`
	Logger    LoggerConfig    `yaml:"logger"`
	TTL       TTLConfig       `yaml:"ttl"`
}

// ServerConfig represents gRPC server configuration
type ServerConfig struct {
	Port int `yaml:"port" envconfig:"SERVER_PORT" default:"50051"`
}

// TarantoolConfig represents Tarantool connection configuration
type TarantoolConfig struct {
	Address  string        `yaml:"address" envconfig:"TARANTOOL_ADDRESS" default:"localhost:3301"`
	User     string        `yaml:"user" envconfig:"TARANTOOL_USER" default:"minitoolstream_connector"`
	Password string        `yaml:"password" envconfig:"TARANTOOL_PASSWORD" default:"changeme"`
	Timeout  time.Duration `yaml:"timeout" envconfig:"TARANTOOL_TIMEOUT" default:"5s"`

	// Vault path for credentials (optional)
	VaultPath string `yaml:"vault_path" envconfig:"TARANTOOL_VAULT_PATH"`
}

// MinIOConfig represents MinIO connection configuration
type MinIOConfig struct {
	Endpoint        string `yaml:"endpoint" envconfig:"MINIO_ENDPOINT" default:"localhost:9000"`
	AccessKeyID     string `yaml:"access_key_id" envconfig:"MINIO_ACCESS_KEY_ID" default:"minioadmin"`
	SecretAccessKey string `yaml:"secret_access_key" envconfig:"MINIO_SECRET_ACCESS_KEY" default:"minioadmin"`
	UseSSL          bool   `yaml:"use_ssl" envconfig:"MINIO_USE_SSL" default:"false"`
	BucketName      string `yaml:"bucket_name" envconfig:"MINIO_BUCKET_NAME" default:"minitoolstream"`

	// Vault path for credentials (optional)
	VaultPath string `yaml:"vault_path" envconfig:"MINIO_VAULT_PATH"`
}

// ChannelTTLConfig represents TTL configuration for a specific channel
type ChannelTTLConfig struct {
	Channel  string        `yaml:"channel"`
	Duration time.Duration `yaml:"duration"`
}

// TTLConfig represents Time-To-Live configuration
type TTLConfig struct {
	Enabled  bool               `yaml:"enabled" envconfig:"TTL_ENABLED" default:"false"`
	Default  time.Duration      `yaml:"default" envconfig:"TTL_DEFAULT" default:"24h"`
	Channels []ChannelTTLConfig `yaml:"channels"`
}

// VaultConfig represents HashiCorp Vault configuration
type VaultConfig struct {
	Enabled   bool   `yaml:"enabled" envconfig:"VAULT_ENABLED" default:"false"`
	Address   string `yaml:"address" envconfig:"VAULT_ADDR" default:"http://localhost:8200"`
	Token     string `yaml:"token" envconfig:"VAULT_TOKEN"`
	TokenPath string `yaml:"token_path" envconfig:"VAULT_TOKEN_PATH"`
	Namespace string `yaml:"namespace" envconfig:"VAULT_NAMESPACE"`
}

// LoggerConfig represents logger configuration
type LoggerConfig struct {
	Level      string `yaml:"level" envconfig:"LOG_LEVEL" default:"info"`
	Format     string `yaml:"format" envconfig:"LOG_FORMAT" default:"json"` // json or console
	OutputPath string `yaml:"output_path" envconfig:"LOG_OUTPUT_PATH" default:"stdout"`
}

// Load loads configuration from file and environment variables
// Environment variables take precedence over file configuration
func Load(configPath string) (*Config, error) {
	cfg := &Config{}

	// Load from file if exists
	fileLoaded := false
	if configPath != "" {
		if err := loadFromFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config from file: %w", err)
		}
		fileLoaded = true
	}

	// Store original Vault.Enabled value from file before envconfig processes it
	originalVaultEnabled := cfg.Vault.Enabled

	// Override with environment variables
	if err := envconfig.Process("", cfg); err != nil {
		return nil, fmt.Errorf("failed to process environment variables: %w", err)
	}

	// If file was loaded and VAULT_ENABLED env var is not set, restore the file value
	// This prevents envconfig from applying its default value over the file value
	if fileLoaded && os.Getenv("VAULT_ENABLED") == "" {
		cfg.Vault.Enabled = originalVaultEnabled
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// loadFromFile loads configuration from YAML file
func loadFromFile(path string, cfg *Config) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	decoder.KnownFields(true) // Strict parsing

	if err := decoder.Decode(cfg); err != nil {
		return err
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Tarantool.Address == "" {
		return fmt.Errorf("tarantool address is required")
	}

	if c.MinIO.Endpoint == "" {
		return fmt.Errorf("minio endpoint is required")
	}

	if c.MinIO.BucketName == "" {
		return fmt.Errorf("minio bucket name is required")
	}

	if c.Vault.Enabled && c.Vault.Address == "" {
		return fmt.Errorf("vault address is required when vault is enabled")
	}

	return nil
}

// GetVaultToken returns the Vault token from config or file
func (c *VaultConfig) GetVaultToken() (string, error) {
	if c.Token != "" {
		return c.Token, nil
	}

	if c.TokenPath != "" {
		token, err := os.ReadFile(c.TokenPath)
		if err != nil {
			return "", fmt.Errorf("failed to read vault token from file: %w", err)
		}
		return string(token), nil
	}

	return "", fmt.Errorf("vault token not configured")
}
