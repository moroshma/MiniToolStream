package config

import (
	"fmt"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
)

// Config represents the publisher client configuration
type Config struct {
	Client ClientConfig `yaml:"client"`
	Vault  VaultConfig  `yaml:"vault"`
	Logger LoggerConfig `yaml:"logger"`
}

// ClientConfig represents the client configuration
type ClientConfig struct {
	ServerAddress string        `yaml:"server_address" envconfig:"CLIENT_SERVER_ADDRESS" default:"localhost:50051"`
	Timeout       time.Duration `yaml:"timeout" envconfig:"CLIENT_TIMEOUT" default:"10s"`

	// Authentication
	JWTToken     string `yaml:"jwt_token" envconfig:"JWT_TOKEN"`      // JWT token for authentication
	JWTVaultPath string `yaml:"jwt_vault_path" envconfig:"JWT_VAULT_PATH"` // Path to JWT token in Vault

	// Publishing defaults
	DefaultSubject     string `yaml:"default_subject" envconfig:"CLIENT_DEFAULT_SUBJECT"`
	DefaultContentType string `yaml:"default_content_type" envconfig:"CLIENT_DEFAULT_CONTENT_TYPE" default:"text/plain"`
	BatchSize          int    `yaml:"batch_size" envconfig:"CLIENT_BATCH_SIZE" default:"10"`
}

// VaultConfig represents HashiCorp Vault configuration
type VaultConfig struct {
	Enabled   bool   `yaml:"enabled" envconfig:"VAULT_ENABLED" default:"false"`
	Address   string `yaml:"address" envconfig:"VAULT_ADDR" default:"http://localhost:8200"`
	Token     string `yaml:"token" envconfig:"VAULT_TOKEN"`
	TokenPath string `yaml:"token_path" envconfig:"VAULT_TOKEN_PATH"`
	Namespace string `yaml:"namespace" envconfig:"VAULT_NAMESPACE"`

	// Path to client secrets in Vault (optional)
	SecretsPath string `yaml:"secrets_path" envconfig:"VAULT_SECRETS_PATH"`
}

// LoggerConfig represents logger configuration
type LoggerConfig struct {
	Level  string `yaml:"level" envconfig:"LOG_LEVEL" default:"info"`
	Format string `yaml:"format" envconfig:"LOG_FORMAT" default:"console"` // json or console
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
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	decoder.KnownFields(true) // Strict parsing

	if err := decoder.Decode(cfg); err != nil {
		return fmt.Errorf("failed to decode config file: %w", err)
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Client.ServerAddress == "" {
		return fmt.Errorf("client server address is required")
	}

	if c.Client.Timeout <= 0 {
		return fmt.Errorf("client timeout must be positive")
	}

	if c.Client.BatchSize <= 0 {
		c.Client.BatchSize = 10
	}

	if c.Vault.Enabled && c.Vault.Address == "" {
		return fmt.Errorf("vault address is required when vault is enabled")
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLogLevels[c.Logger.Level] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.Logger.Level)
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
