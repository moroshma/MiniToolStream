package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_Validate_Success(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port: 50051,
		},
		Tarantool: TarantoolConfig{
			Address: "localhost:3301",
		},
		MinIO: MinIOConfig{
			Endpoint:   "localhost:9000",
			BucketName: "test-bucket",
		},
		Vault: VaultConfig{
			Enabled: false,
		},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestConfig_Validate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{
			name: "zero port",
			port: 0,
		},
		{
			name: "negative port",
			port: -1,
		},
		{
			name: "port too large",
			port: 65536,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Server: ServerConfig{
					Port: tt.port,
				},
				Tarantool: TarantoolConfig{
					Address: "localhost:3301",
				},
				MinIO: MinIOConfig{
					Endpoint:   "localhost:9000",
					BucketName: "test-bucket",
				},
			}

			err := cfg.Validate()
			if err == nil {
				t.Fatal("expected validation error for invalid port")
			}
		})
	}
}

func TestConfig_Validate_EmptyTarantoolAddress(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port: 50051,
		},
		Tarantool: TarantoolConfig{
			Address: "",
		},
		MinIO: MinIOConfig{
			Endpoint:   "localhost:9000",
			BucketName: "test-bucket",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty tarantool address")
	}
}

func TestConfig_Validate_EmptyMinIOEndpoint(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port: 50051,
		},
		Tarantool: TarantoolConfig{
			Address: "localhost:3301",
		},
		MinIO: MinIOConfig{
			Endpoint:   "",
			BucketName: "test-bucket",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty minio endpoint")
	}
}

func TestConfig_Validate_EmptyBucketName(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port: 50051,
		},
		Tarantool: TarantoolConfig{
			Address: "localhost:3301",
		},
		MinIO: MinIOConfig{
			Endpoint:   "localhost:9000",
			BucketName: "",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for empty bucket name")
	}
}

func TestConfig_Validate_VaultEnabledWithoutAddress(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Port: 50051,
		},
		Tarantool: TarantoolConfig{
			Address: "localhost:3301",
		},
		MinIO: MinIOConfig{
			Endpoint:   "localhost:9000",
			BucketName: "test-bucket",
		},
		Vault: VaultConfig{
			Enabled: true,
			Address: "",
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected validation error for vault enabled without address")
	}
}

func TestLoadFromFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  port: 50052
  poll_interval: 1s

tarantool:
  address: localhost:3301
  user: testuser
  password: testpass
  timeout: 5s

minio:
  endpoint: localhost:9000
  access_key_id: minioadmin
  secret_access_key: minioadmin
  use_ssl: false
  bucket_name: minitoolstream

vault:
  enabled: false
  address: http://localhost:8200

logger:
  level: info
  format: json
  output_path: stdout
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 50052 {
		t.Errorf("expected port 50052, got %d", cfg.Server.Port)
	}
	if cfg.Tarantool.Address != "localhost:3301" {
		t.Errorf("expected tarantool address 'localhost:3301', got '%s'", cfg.Tarantool.Address)
	}
	if cfg.MinIO.Endpoint != "localhost:9000" {
		t.Errorf("expected minio endpoint 'localhost:9000', got '%s'", cfg.MinIO.Endpoint)
	}
	if cfg.Vault.Enabled {
		t.Error("expected vault to be disabled")
	}
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	configContent := `
server:
  port: invalid_port
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err = Load(configFile)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadFromFile_FileNotFound(t *testing.T) {
	_, err := Load("/path/that/does/not/exist/config.yaml")
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestLoad_EmptyPath(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
}

func TestVaultConfig_GetVaultToken_FromToken(t *testing.T) {
	cfg := &VaultConfig{
		Token: "test-token",
	}

	token, err := cfg.GetVaultToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token != "test-token" {
		t.Errorf("expected token 'test-token', got '%s'", token)
	}
}

func TestVaultConfig_GetVaultToken_FromFile(t *testing.T) {
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "vault-token")

	err := os.WriteFile(tokenFile, []byte("file-token"), 0644)
	if err != nil {
		t.Fatalf("failed to write token file: %v", err)
	}

	cfg := &VaultConfig{
		TokenPath: tokenFile,
	}

	token, err := cfg.GetVaultToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token != "file-token" {
		t.Errorf("expected token 'file-token', got '%s'", token)
	}
}

func TestVaultConfig_GetVaultToken_NotConfigured(t *testing.T) {
	cfg := &VaultConfig{}

	_, err := cfg.GetVaultToken()
	if err == nil {
		t.Fatal("expected error for unconfigured token")
	}
}

func TestVaultConfig_GetVaultToken_FileNotFound(t *testing.T) {
	cfg := &VaultConfig{
		TokenPath: "/path/that/does/not/exist/vault-token",
	}

	_, err := cfg.GetVaultToken()
	if err == nil {
		t.Fatal("expected error for non-existent token file")
	}
}

func TestVaultConfig_GetVaultToken_TokenPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "vault-token")

	err := os.WriteFile(tokenFile, []byte("file-token"), 0644)
	if err != nil {
		t.Fatalf("failed to write token file: %v", err)
	}

	cfg := &VaultConfig{
		Token:     "direct-token",
		TokenPath: tokenFile,
	}

	token, err := cfg.GetVaultToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token != "direct-token" {
		t.Errorf("expected direct token to take precedence, got '%s'", token)
	}
}
