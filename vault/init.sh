#!/bin/sh
set -e

echo "Waiting for Vault to be ready..."
sleep 3

echo "Enabling KV secrets engine..."
vault secrets enable -version=2 -path=secret kv || echo "KV engine already enabled"

echo "Creating Tarantool credentials..."
vault kv put secret/minitoolstream_connector/tarantool \
  user=minitoolstream_connector \
  password=changeme

echo "Creating MinIO credentials..."
vault kv put secret/minitoolstream_connector/minio \
  access_key_id=minioadmin \
  secret_access_key=minioadmin

echo "Verifying secrets..."
vault kv get secret/minitoolstream_connector/tarantool
vault kv get secret/minitoolstream_connector/minio

echo "✓ Vault initialized successfully with MiniToolStream secrets"
