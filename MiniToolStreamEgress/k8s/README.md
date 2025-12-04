# MiniToolStreamEgress - Kubernetes Deployment

This directory contains Kubernetes manifests for deploying MiniToolStreamEgress to k3s.

## Prerequisites

- k3s cluster running
- kubectl configured to access the cluster
- Docker for building images
- MinIO and Tarantool deployed in the same cluster

## Quick Start

### 1. Build Docker Image

```bash
# Build and tag the image
./scripts/build-image.sh latest

# Or build with specific tag
./scripts/build-image.sh v1.0.0
```

### 2. Configure Secrets

Edit `k8s/secret.yaml` and update the following values:

```yaml
stringData:
  TARANTOOL_PASSWORD: "your-secure-password"
  MINIO_ACCESS_KEY_ID: "your-minio-access-key"
  MINIO_SECRET_ACCESS_KEY: "your-minio-secret-key"
```

**Important:** Never commit secrets to version control. Consider using:
- Sealed Secrets
- External Secrets Operator
- HashiCorp Vault

### 3. Deploy to k3s

```bash
# Deploy using script
./scripts/deploy.sh

# Or deploy manually
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/rbac.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/hpa.yaml
kubectl apply -f k8s/pdb.yaml

# Or use kustomize
kubectl apply -k k8s/
```

### 4. Verify Deployment

```bash
# Check pods
kubectl get pods -n minitoolstream -l app=minitoolstream-egress

# Check services
kubectl get svc -n minitoolstream -l app=minitoolstream-egress

# Check logs
kubectl logs -f -n minitoolstream -l app=minitoolstream-egress

# Check HPA status
kubectl get hpa -n minitoolstream
```

## Manifest Files

### Core Resources

- **namespace.yaml** - Creates the `minitoolstream` namespace
- **configmap.yaml** - Application configuration
- **secret.yaml** - Sensitive credentials (Tarantool, MinIO)
- **rbac.yaml** - ServiceAccount, Role, and RoleBinding
- **deployment.yaml** - Main application deployment (3 replicas)
- **service.yaml** - ClusterIP and Headless services
- **hpa.yaml** - Horizontal Pod Autoscaler (3-10 replicas)
- **pdb.yaml** - Pod Disruption Budget (min 2 available)

### Supporting Files

- **kustomization.yaml** - Kustomize configuration for overlay management

## Configuration

### Environment Variables

The deployment uses environment variables for configuration. Key variables:

#### Server
- `SERVER_PORT` - gRPC server port (default: 50052)
- `SERVER_POLL_INTERVAL` - Polling interval (default: 1s)

#### Tarantool
- `TARANTOOL_ADDRESS` - Tarantool service address
- `TARANTOOL_USER` - Database user
- `TARANTOOL_PASSWORD` - Database password (from secret)
- `TARANTOOL_TIMEOUT` - Connection timeout

#### MinIO
- `MINIO_ENDPOINT` - MinIO service endpoint
- `MINIO_ACCESS_KEY_ID` - Access key (from secret)
- `MINIO_SECRET_ACCESS_KEY` - Secret key (from secret)
- `MINIO_USE_SSL` - Use SSL/TLS
- `MINIO_BUCKET_NAME` - Bucket name

#### Vault (Optional)
- `VAULT_ENABLED` - Enable Vault integration
- `VAULT_ADDR` - Vault server address

#### Logging
- `LOG_LEVEL` - Log level (debug, info, warn, error)
- `LOG_FORMAT` - Log format (json, console)
- `LOG_OUTPUT_PATH` - Output path (stdout, file path)

## Scaling

### Manual Scaling

```bash
# Scale to 5 replicas
kubectl scale deployment minitoolstream-egress -n minitoolstream --replicas=5
```

### Auto-scaling

HPA is configured to auto-scale based on:
- CPU utilization (target: 70%)
- Memory utilization (target: 80%)

Min replicas: 3, Max replicas: 10

## Monitoring

### Prometheus Metrics

The deployment is annotated for Prometheus scraping:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "50052"
  prometheus.io/path: "/metrics"
```

### Health Checks

- **Liveness Probe**: Checks if the application is alive
- **Readiness Probe**: Checks if the application is ready to serve traffic

## High Availability

### Pod Distribution

- **Anti-affinity rules**: Pods are distributed across different nodes
- **PodDisruptionBudget**: Ensures minimum 2 pods are always available
- **Multiple replicas**: 3 replicas by default

### Graceful Shutdown

The deployment handles graceful shutdown:
1. SIGTERM signal received
2. gRPC server stops accepting new connections
3. Existing connections are drained
4. Pod terminates

## Security

### Pod Security

- Runs as non-root user (UID 1000)
- Read-only root filesystem
- Drops all capabilities
- No privilege escalation

### Network Security

- ClusterIP service (internal only)
- No external exposure by default

### Secret Management

- Kubernetes Secrets for credentials
- Consider using External Secrets Operator for production

## Troubleshooting

### Check Pod Status

```bash
kubectl get pods -n minitoolstream -l app=minitoolstream-egress
kubectl describe pod <pod-name> -n minitoolstream
```

### View Logs

```bash
# All pods
kubectl logs -f -n minitoolstream -l app=minitoolstream-egress

# Specific pod
kubectl logs -f <pod-name> -n minitoolstream

# Previous pod (if crashed)
kubectl logs <pod-name> -n minitoolstream --previous
```

### Check Events

```bash
kubectl get events -n minitoolstream --sort-by='.lastTimestamp'
```

### Port Forward for Testing

```bash
# Forward local port 50052 to service
kubectl port-forward -n minitoolstream svc/minitoolstream-egress-service 50052:50052

# Test with grpcurl
grpcurl -plaintext localhost:50052 list
```

### Common Issues

1. **Pods not starting**
   - Check if Tarantool and MinIO services are available
   - Verify secrets are correctly configured
   - Check resource limits

2. **Connection errors**
   - Verify service names and ports
   - Check network policies
   - Ensure dependencies are running

3. **Performance issues**
   - Check HPA metrics
   - Review resource requests/limits
   - Monitor logs for errors

## Cleanup

```bash
# Remove deployment
./scripts/undeploy.sh

# Or manually
kubectl delete -k k8s/
```

## Production Considerations

1. **Resource Limits**: Adjust based on actual usage patterns
2. **Secrets Management**: Use Vault or External Secrets Operator
3. **Monitoring**: Integrate with Prometheus and Grafana
4. **Logging**: Configure centralized logging (ELK, Loki)
5. **Backup**: Implement backup strategies for data
6. **Security**: Enable network policies, pod security policies
7. **CI/CD**: Automate deployment with GitOps (ArgoCD, Flux)

## References

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [k3s Documentation](https://docs.k3s.io/)
- [Kustomize Documentation](https://kustomize.io/)
