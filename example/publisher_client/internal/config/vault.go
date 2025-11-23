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

	// Load client secrets from Vault if path is specified
	if cfg.Vault.SecretsPath != "" {
		secret, err := vaultClient.GetSecret(ctx, cfg.Vault.SecretsPath)
		if err != nil {
			return fmt.Errorf("failed to get client secrets: %w", err)
		}

		// Apply secrets to configuration
		if serverAddress, ok := secret["server_address"].(string); ok && serverAddress != "" {
			cfg.Client.ServerAddress = serverAddress
		}

		if defaultSubject, ok := secret["default_subject"].(string); ok && defaultSubject != "" {
			cfg.Client.DefaultSubject = defaultSubject
		}

		// Other optional fields can be added here
	}

	return nil
}
