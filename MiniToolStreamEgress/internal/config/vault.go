package config

import (
	"context"
	"fmt"

	vault "github.com/hashicorp/vault/api"
)

// VaultClient wraps HashiCorp Vault client
type VaultClient struct {
	client *vault.Client
	config *VaultConfig
}

// NewVaultClient creates a new Vault client
func NewVaultClient(cfg *VaultConfig) (*VaultClient, error) {
	if !cfg.Enabled {
		return nil, nil // Vault is disabled
	}

	vaultCfg := vault.DefaultConfig()
	vaultCfg.Address = cfg.Address

	client, err := vault.NewClient(vaultCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	// Set token
	token, err := cfg.GetVaultToken()
	if err != nil {
		return nil, err
	}
	client.SetToken(token)

	// Set namespace if provided
	if cfg.Namespace != "" {
		client.SetNamespace(cfg.Namespace)
	}

	return &VaultClient{
		client: client,
		config: cfg,
	}, nil
}

// GetSecret retrieves a secret from Vault
func (vc *VaultClient) GetSecret(ctx context.Context, path string) (map[string]interface{}, error) {
	if vc == nil {
		return nil, fmt.Errorf("vault client is not initialized")
	}

	secret, err := vc.client.KVv2("secret").Get(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret from vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("secret not found: %s", path)
	}

	return secret.Data, nil
}

// ApplyVaultSecrets applies secrets from Vault to configuration
func ApplyVaultSecrets(ctx context.Context, cfg *Config, vaultClient *VaultClient) error {
	if vaultClient == nil {
		return nil // Vault is disabled
	}

	// Load Tarantool credentials from Vault if path is specified
	if cfg.Tarantool.VaultPath != "" {
		secret, err := vaultClient.GetSecret(ctx, cfg.Tarantool.VaultPath)
		if err != nil {
			return fmt.Errorf("failed to get tarantool secrets: %w", err)
		}

		if user, ok := secret["user"].(string); ok {
			cfg.Tarantool.User = user
		}
		if password, ok := secret["password"].(string); ok {
			cfg.Tarantool.Password = password
		}
	}

	// Load MinIO credentials from Vault if path is specified
	if cfg.MinIO.VaultPath != "" {
		secret, err := vaultClient.GetSecret(ctx, cfg.MinIO.VaultPath)
		if err != nil {
			return fmt.Errorf("failed to get minio secrets: %w", err)
		}

		if accessKey, ok := secret["access_key_id"].(string); ok {
			cfg.MinIO.AccessKeyID = accessKey
		}
		if secretKey, ok := secret["secret_access_key"].(string); ok {
			cfg.MinIO.SecretAccessKey = secretKey
		}
	}

	return nil
}
