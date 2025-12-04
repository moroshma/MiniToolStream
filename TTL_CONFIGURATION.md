# TTL (Time-To-Live) Configuration Guide

## Overview

The TTL feature automatically deletes old messages from both Tarantool and MinIO based on a configured time period. This helps manage storage space and maintain system performance.

## Architecture

The TTL cleanup service runs in the **Ingress** service and performs the following operations:

1. **Tarantool Cleanup**: Deletes message metadata from Tarantool database
2. **MinIO Cleanup**: Deletes corresponding object files from MinIO storage
3. **Periodic Execution**: Runs cleanup at regular intervals

## Configuration

### Configuration File (YAML)

```yaml
ttl:
  enabled: true          # Enable/disable TTL cleanup
  duration: 24h          # Delete messages older than this duration
  interval: 1h           # Run cleanup every interval
```

### Environment Variables

You can also configure TTL using environment variables:

```bash
TTL_ENABLED=true
TTL_DURATION=24h
TTL_INTERVAL=1h
```

### Duration Format

The duration values use Go's duration format:
- `5m` - 5 minutes
- `1h` - 1 hour
- `24h` - 24 hours
- `168h` - 1 week (7 days)

## How It Works

### 1. Message Creation
When a message is published:
- A timestamp (`create_at`) is stored in Tarantool
- The corresponding data is uploaded to MinIO

### 2. TTL Cleanup Process
The cleanup service:
1. Runs at the configured `interval`
2. Calculates the cutoff time: `current_time - duration`
3. Queries Tarantool for messages older than the cutoff
4. Deletes messages from Tarantool
5. Deletes corresponding objects from MinIO

### 3. Logging
The service logs:
- Cleanup execution start/completion
- Number of messages deleted from Tarantool
- Number of objects deleted from MinIO
- Any errors during cleanup

## Docker Compose Configuration

### Example `config.yaml` for Ingress

```yaml
server:
  port: 50051

tarantool:
  address: "tarantool:3301"
  user: "minitoolstream_connector"
  password: "changeme"
  timeout: 5s

minio:
  endpoint: "minio:9000"
  access_key_id: "minioadmin"
  secret_access_key: "minioadmin"
  use_ssl: false
  bucket_name: "minitoolstream"

logger:
  level: "info"
  format: "json"
  output_path: "stdout"

ttl:
  enabled: true
  duration: 24h     # Delete messages older than 24 hours
  interval: 1h      # Run cleanup every hour
```

### Docker Compose Service

```yaml
ingress:
  build:
    context: ./MiniToolStreamIngress
    dockerfile: Dockerfile
  ports:
    - "50051:50051"
  volumes:
    - ./MiniToolStreamIngress/config.yaml:/app/config.yaml:ro
  command: ["/app/server", "-config", "/app/config.yaml"]
```

## Kubernetes Configuration

### ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ingress-config
  namespace: minitoolstream
data:
  config.yaml: |
    ttl:
      enabled: true
      duration: 24h
      interval: 1h
```

### Deployment Environment Variables

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ingress
spec:
  template:
    spec:
      containers:
      - name: ingress
        env:
        - name: TTL_ENABLED
          value: "true"
        - name: TTL_DURATION
          value: "24h"
        - name: TTL_INTERVAL
          value: "1h"
```

## Testing TTL Functionality

### Quick Test (Docker Compose)

1. Set short TTL for testing:
```yaml
ttl:
  enabled: true
  duration: 5m   # 5 minutes
  interval: 1m   # 1 minute
```

2. Start services:
```bash
docker-compose up -d
```

3. Publish some messages:
```bash
cd example/publisher_client
./publisher_client
# Publish messages to "test" channel
```

4. Wait 6-7 minutes for cleanup to run

5. Check logs:
```bash
docker-compose logs ingress | grep "TTL cleanup"
```

Expected log output:
```json
{"level":"info","msg":"Running TTL cleanup","ttl_seconds":300}
{"level":"info","msg":"Deleted old messages from Tarantool","count":5}
{"level":"info","msg":"TTL cleanup completed","tarantool_deleted":5,"minio_deleted":5,"minio_failed":0}
```

### Automated Test Script

Run the provided test script:
```bash
./test-ttl.sh
```

This script will:
1. Publish test messages
2. Wait for TTL cleanup to run
3. Verify messages were deleted
4. Report results

## Monitoring

### Check TTL Service Status

View Ingress logs to monitor TTL cleanup:
```bash
# Docker Compose
docker-compose logs -f ingress

# Kubernetes
kubectl logs -f -n minitoolstream deployment/ingress
```

### Key Log Messages

- **Service Start**: `"Starting TTL cleanup service"`
- **Cleanup Execution**: `"Running TTL cleanup"`
- **Success**: `"TTL cleanup completed"`
- **Errors**: `"Failed to delete object from MinIO"`

### Metrics to Monitor

- `tarantool_deleted`: Messages deleted from Tarantool
- `minio_deleted`: Objects deleted from MinIO
- `minio_failed`: Failed deletions from MinIO
- `duration`: Time taken for cleanup operation

## Best Practices

### 1. Choose Appropriate Duration
- **Short-term data** (logs, temp files): 1-7 days
- **Medium-term data** (analytics): 30-90 days
- **Long-term data** (archives): 365+ days

### 2. Set Reasonable Interval
- For large datasets: Run less frequently (e.g., every 6-12 hours)
- For small datasets: Run more frequently (e.g., every 1 hour)
- Avoid very short intervals (< 10 minutes) in production

### 3. Monitor Storage Usage
- Track MinIO storage usage over time
- Adjust TTL duration based on storage capacity
- Alert on failed deletions

### 4. Test in Staging First
- Test TTL configuration with short durations
- Verify both Tarantool and MinIO cleanup work correctly
- Monitor for any errors or issues

## Troubleshooting

### TTL Not Running

**Problem**: No cleanup logs appear

**Solutions**:
1. Check if TTL is enabled: `enabled: true`
2. Verify Ingress service is running
3. Check for startup errors in logs

### Messages Not Being Deleted

**Problem**: Old messages still present after TTL duration

**Solutions**:
1. Verify `create_at` timestamps in Tarantool
2. Check TTL interval has passed
3. Review cleanup logs for errors
4. Ensure clock synchronization across services

### MinIO Deletion Failures

**Problem**: `minio_failed` count is high

**Solutions**:
1. Verify MinIO connectivity
2. Check MinIO credentials
3. Ensure bucket exists
4. Review MinIO server logs

### Performance Issues

**Problem**: Cleanup takes too long or impacts performance

**Solutions**:
1. Increase cleanup interval
2. Implement batch deletion limits
3. Run cleanup during off-peak hours
4. Consider archiving instead of deletion

## API Reference

### Tarantool Function

```lua
-- Delete messages older than TTL
-- @param ttl_seconds number - TTL in seconds
-- @return deleted_count, array of deleted message info
function delete_old_messages(ttl_seconds)
```

### Go Service Interface

```go
// TTL Service configuration
type Config struct {
    Enabled     bool
    TTLDuration time.Duration
    Interval    time.Duration
}

// Create new TTL service
func NewService(
    messageRepo MessageRepository,
    storageRepo StorageRepository,
    cfg Config,
    log *logger.Logger,
) *Service

// Start periodic cleanup
func (s *Service) Start(ctx context.Context) error

// Stop cleanup service
func (s *Service) Stop()

// Run cleanup once (for testing)
func (s *Service) RunOnce(ctx context.Context) error
```

## Security Considerations

1. **Credentials**: Store MinIO and Tarantool credentials securely (use Vault)
2. **Access Control**: Limit deletion permissions to TTL service only
3. **Audit Logging**: Log all deletion operations for compliance
4. **Backup**: Ensure backups exist before enabling aggressive TTL

## Future Enhancements

Potential improvements:
- Per-channel TTL configuration
- Archiving to cold storage before deletion
- Custom retention policies based on message metadata
- TTL override for specific messages
- Deletion metrics and dashboards
