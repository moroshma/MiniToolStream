package config

import (
	"context"
	"testing"
)

func TestNewVaultClient_Disabled(t *testing.T) {
	cfg := &VaultConfig{
		Enabled: false,
	}

	client, err := NewVaultClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if client != nil {
		t.Error("expected nil client when vault is disabled")
	}
}

func TestNewVaultClient_NoToken(t *testing.T) {
	cfg := &VaultConfig{
		Enabled: true,
		Address: "http://localhost:8200",
	}

	_, err := NewVaultClient(cfg)
	if err == nil {
		t.Fatal("expected error when token is not configured")
	}
}

func TestVaultClient_GetSecret_NilClient(t *testing.T) {
	var vc *VaultClient
	ctx := context.Background()

	_, err := vc.GetSecret(ctx, "secret/path")
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if err.Error() != "vault client is not initialized" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestApplyVaultSecrets_NilClient(t *testing.T) {
	cfg := &Config{
		Tarantool: TarantoolConfig{
			User:     "original_user",
			Password: "original_pass",
		},
	}
	ctx := context.Background()

	err := ApplyVaultSecrets(ctx, cfg, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Tarantool.User != "original_user" {
		t.Error("expected original user to remain unchanged")
	}
	if cfg.Tarantool.Password != "original_pass" {
		t.Error("expected original password to remain unchanged")
	}
}
