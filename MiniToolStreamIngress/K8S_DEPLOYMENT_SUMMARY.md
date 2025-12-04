# MiniToolStreamIngress - K8s Deployment Summary

## Created Files

### Docker
- **Dockerfile** - Multi-stage build with security best practices
- **.dockerignore** - Optimized build context

### Kubernetes Manifests (k8s/)
1. **namespace.yaml** - minitoolstream namespace
2. **configmap.yaml** - Application configuration
3. **secret.yaml** - Sensitive credentials
4. **rbac.yaml** - ServiceAccount, Role, RoleBinding
5. **deployment.yaml** - 3 replicas with health checks
6. **service.yaml** - ClusterIP + Headless services
7. **hpa.yaml** - Auto-scaling (3-10 replicas)
8. **pdb.yaml** - Pod Disruption Budget
9. **kustomization.yaml** - Kustomize configuration

### Scripts
- **scripts/build-image.sh** - Build Docker image
- **scripts/deploy.sh** - Deploy to k3s
- **scripts/undeploy.sh** - Remove deployment

## Key Features

### Security
✅ Non-root user (UID 1000)
✅ Read-only root filesystem
✅ No privilege escalation
✅ Dropped capabilities
✅ RBAC configured

### High Availability
✅ 3 replicas minimum
✅ Pod anti-affinity
✅ Pod Disruption Budget (min 2)
✅ Rolling updates
✅ Health checks (liveness + readiness)

### Scalability
✅ Horizontal Pod Autoscaler
✅ Resource requests/limits
✅ Multiple service endpoints

### Observability
✅ Structured JSON logging
✅ Prometheus annotations
✅ Health check endpoints
✅ Pod metadata injection

## Quick Commands

```bash
# Build image
./scripts/build-image.sh latest

# Deploy
./scripts/deploy.sh

# Check status
kubectl get pods -n minitoolstream -l app=minitoolstream-ingress

# View logs
kubectl logs -f -n minitoolstream -l app=minitoolstream-ingress

# Undeploy
./scripts/undeploy.sh
```

## Configuration

### Server
- **Port**: 50051 (gRPC)

### Dependencies
- **Tarantool**: tarantool-service.minitoolstream.svc.cluster.local:3301
- **MinIO**: minio-service.minitoolstream.svc.cluster.local:9000

### Resource Requirements

**Per Pod:**
- Memory: 256Mi (request) / 512Mi (limit)
- CPU: 250m (request) / 500m (limit)

**Scaling:**
- Min: 3 replicas
- Max: 10 replicas
- Targets: CPU 70%, Memory 80%

## Service Endpoints

- **Ingress Service**: minitoolstream-ingress-service.minitoolstream.svc.cluster.local:50051
- **Headless Service**: minitoolstream-ingress-headless.minitoolstream.svc.cluster.local:50051

## Environment Variables

```yaml
# Server
SERVER_PORT: 50051

# Tarantool
TARANTOOL_ADDRESS: tarantool-service.minitoolstream.svc.cluster.local:3301
TARANTOOL_USER: minitoolstream_connector
TARANTOOL_PASSWORD: <from-secret>
TARANTOOL_TIMEOUT: 5s

# MinIO
MINIO_ENDPOINT: minio-service.minitoolstream.svc.cluster.local:9000
MINIO_ACCESS_KEY_ID: <from-secret>
MINIO_SECRET_ACCESS_KEY: <from-secret>
MINIO_USE_SSL: false
MINIO_BUCKET_NAME: minitoolstream

# Logging
LOG_LEVEL: info
LOG_FORMAT: json
LOG_OUTPUT_PATH: stdout
```

## Production Checklist

- [ ] Update secrets in `k8s/secret.yaml` with production values
- [ ] Build Docker image with proper tags
- [ ] Deploy to k3s cluster
- [ ] Verify all pods are running
- [ ] Test gRPC endpoints
- [ ] Configure monitoring (Prometheus)
- [ ] Set up centralized logging
- [ ] Configure alerting
- [ ] Enable TLS for gRPC (production)
- [ ] Implement network policies
- [ ] Set up backup strategy
- [ ] Document runbooks

## Troubleshooting

### Pod Issues
```bash
# Check pod status
kubectl get pods -n minitoolstream -l app=minitoolstream-ingress

# Describe pod
kubectl describe pod <pod-name> -n minitoolstream

# View logs
kubectl logs <pod-name> -n minitoolstream

# Previous logs (if crashed)
kubectl logs <pod-name> -n minitoolstream --previous
```

### Connection Testing
```bash
# Port forward
kubectl port-forward -n minitoolstream svc/minitoolstream-ingress-service 50051:50051

# Test with grpcurl
grpcurl -plaintext localhost:50051 list
```

### Common Issues

1. **ImagePullBackOff** - Verify image exists
2. **CrashLoopBackOff** - Check logs and dependencies
3. **Pending Pods** - Check node resources and PDB

## Next Steps

1. Deploy alongside MiniToolStreamEgress
2. Configure load balancing
3. Set up monitoring dashboards
4. Implement CI/CD pipeline
5. Configure automated testing

## Integration with Egress

Both Ingress and Egress services share:
- Same namespace (minitoolstream)
- Same Tarantool instance
- Same MinIO instance
- Same secrets

Deploy both for complete MiniToolStream functionality.
