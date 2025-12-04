# MiniToolStreamEgress - Deployment Guide for k3s

## Overview

This guide provides complete instructions for deploying MiniToolStreamEgress to a k3s Kubernetes cluster.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    k3s Cluster                          │
│                                                         │
│  ┌──────────────┐      ┌──────────────┐               │
│  │  Tarantool   │◄─────┤   Egress     │               │
│  │  Service     │      │   Pods (3+)  │               │
│  └──────────────┘      └──────┬───────┘               │
│                               │                         │
│  ┌──────────────┐            │                         │
│  │    MinIO     │◄───────────┘                         │
│  │   Service    │                                      │
│  └──────────────┘                                      │
│                                                         │
│  ┌──────────────────────────────────────────┐          │
│  │         Egress Service                   │          │
│  │  ClusterIP: minitoolstream-egress-service│          │
│  │  Port: 50052 (gRPC)                      │          │
│  └──────────────────────────────────────────┘          │
└─────────────────────────────────────────────────────────┘
```

## Prerequisites

### Required

1. **k3s cluster** - Running and accessible
2. **kubectl** - Configured to access k3s cluster
3. **Docker** - For building container images
4. **Dependencies running in k3s:**
   - Tarantool service
   - MinIO service

### Optional

- **Helm** - For easier deployment management
- **k9s** - For cluster monitoring
- **stern** - For log tailing

## Quick Start

### 1. Prepare Environment

```bash
# Clone repository
cd /Users/moroshma/go/DiplomaThesis/MiniToolStream/MiniToolStreamEgress

# Check k3s is running
kubectl cluster-info
```

### 2. Build Docker Image

```bash
# Build the image
./scripts/build-image.sh latest

# Verify image
docker images | grep minitoolstream-egress
```

### 3. Configure Secrets

**IMPORTANT:** Update secrets before deployment!

Edit `k8s/secret.yaml`:

```yaml
stringData:
  TARANTOOL_PASSWORD: "your-production-password"
  MINIO_ACCESS_KEY_ID: "your-minio-access-key"
  MINIO_SECRET_ACCESS_KEY: "your-minio-secret-key"
```

### 4. Deploy to k3s

```bash
# Quick deployment
./scripts/deploy.sh

# Or step by step
kubectl apply -f k8s/namespace.yaml
kubectl apply -f k8s/rbac.yaml
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/service.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/hpa.yaml
kubectl apply -f k8s/pdb.yaml
```

### 5. Verify Deployment

```bash
# Check pod status
kubectl get pods -n minitoolstream -l app=minitoolstream-egress

# Expected output:
# NAME                                    READY   STATUS    RESTARTS   AGE
# minitoolstream-egress-xxx-yyy          1/1     Running   0          1m
# minitoolstream-egress-xxx-zzz          1/1     Running   0          1m
# minitoolstream-egress-xxx-www          1/1     Running   0          1m

# Check service
kubectl get svc -n minitoolstream minitoolstream-egress-service

# Check logs
kubectl logs -f -n minitoolstream -l app=minitoolstream-egress
```

## Detailed Configuration

### Resource Requirements

**Per Pod:**
- Memory: 256Mi (request) / 512Mi (limit)
- CPU: 250m (request) / 500m (limit)

**For 3 Replicas:**
- Total Memory: ~768Mi (request) / ~1.5Gi (limit)
- Total CPU: ~750m (request) / ~1.5 cores (limit)

### Scaling Configuration

**Horizontal Pod Autoscaler:**
- Min Replicas: 3
- Max Replicas: 10
- Target CPU: 70%
- Target Memory: 80%

**Manual Scaling:**
```bash
kubectl scale deployment minitoolstream-egress -n minitoolstream --replicas=5
```

### High Availability

**Pod Distribution:**
- Anti-affinity ensures pods spread across nodes
- Pod Disruption Budget ensures min 2 pods always available
- Rolling update strategy with maxSurge=1, maxUnavailable=1

**Health Checks:**
- Liveness probe: TCP check on port 50052
- Readiness probe: TCP check on port 50052
- Initial delay: 30s (liveness), 10s (readiness)

## Environment Configuration

### Core Settings

```yaml
# Server
SERVER_PORT: 50052
SERVER_POLL_INTERVAL: 1s

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

### Service Discovery

All services use Kubernetes DNS:
- **Tarantool**: `tarantool-service.minitoolstream.svc.cluster.local`
- **MinIO**: `minio-service.minitoolstream.svc.cluster.local`
- **Egress**: `minitoolstream-egress-service.minitoolstream.svc.cluster.local`

## Monitoring & Observability

### Logs

```bash
# All pods
kubectl logs -f -n minitoolstream -l app=minitoolstream-egress

# Specific pod
kubectl logs -f -n minitoolstream minitoolstream-egress-xxx-yyy

# Previous container (if crashed)
kubectl logs -n minitoolstream minitoolstream-egress-xxx-yyy --previous

# Using stern (if installed)
stern -n minitoolstream minitoolstream-egress
```

### Metrics

Pods are annotated for Prometheus scraping:
```yaml
prometheus.io/scrape: "true"
prometheus.io/port: "50052"
prometheus.io/path: "/metrics"
```

### Events

```bash
# Watch events
kubectl get events -n minitoolstream --watch

# Recent events
kubectl get events -n minitoolstream --sort-by='.lastTimestamp' | tail -20
```

## Troubleshooting

### Pod Won't Start

**Check dependencies:**
```bash
# Verify Tarantool is running
kubectl get pods -n minitoolstream -l app=tarantool

# Verify MinIO is running
kubectl get pods -n minitoolstream -l app=minio

# Check init containers
kubectl describe pod <pod-name> -n minitoolstream
```

**Check secrets:**
```bash
kubectl get secret minitoolstream-egress-secret -n minitoolstream -o yaml
```

### Connection Issues

**Test connectivity:**
```bash
# Port forward to test locally
kubectl port-forward -n minitoolstream svc/minitoolstream-egress-service 50052:50052

# Test with grpcurl
grpcurl -plaintext localhost:50052 list
```

**Check DNS resolution:**
```bash
kubectl run -it --rm debug --image=busybox --restart=Never -n minitoolstream -- sh
nslookup tarantool-service.minitoolstream.svc.cluster.local
nslookup minio-service.minitoolstream.svc.cluster.local
```

### Performance Issues

**Check HPA:**
```bash
kubectl get hpa -n minitoolstream
kubectl describe hpa minitoolstream-egress-hpa -n minitoolstream
```

**Check resource usage:**
```bash
kubectl top pods -n minitoolstream -l app=minitoolstream-egress
```

### Common Errors

1. **ImagePullBackOff**
   - Check if image exists: `docker images | grep minitoolstream-egress`
   - Verify imagePullPolicy in deployment.yaml

2. **CrashLoopBackOff**
   - Check logs: `kubectl logs <pod-name> -n minitoolstream`
   - Verify environment variables
   - Check dependency availability

3. **Pending Pods**
   - Check node resources: `kubectl describe node`
   - Verify PodDisruptionBudget: `kubectl get pdb -n minitoolstream`

## Updating Deployment

### Update Configuration

```bash
# Edit ConfigMap
kubectl edit configmap minitoolstream-egress-config -n minitoolstream

# Restart pods to pick up changes
kubectl rollout restart deployment minitoolstream-egress -n minitoolstream
```

### Update Image

```bash
# Build new image
./scripts/build-image.sh v1.1.0

# Update deployment
kubectl set image deployment/minitoolstream-egress \
  egress=minitoolstream-egress:v1.1.0 \
  -n minitoolstream

# Watch rollout
kubectl rollout status deployment/minitoolstream-egress -n minitoolstream
```

### Rollback

```bash
# View rollout history
kubectl rollout history deployment/minitoolstream-egress -n minitoolstream

# Rollback to previous version
kubectl rollout undo deployment/minitoolstream-egress -n minitoolstream

# Rollback to specific revision
kubectl rollout undo deployment/minitoolstream-egress -n minitoolstream --to-revision=2
```

## Security Best Practices

1. **Never commit secrets to version control**
2. **Use Kubernetes Secrets or external secret managers**
3. **Enable RBAC (already configured)**
4. **Run as non-root user (already configured)**
5. **Use read-only root filesystem (already configured)**
6. **Implement network policies for pod-to-pod communication**
7. **Enable TLS for gRPC communication in production**

## Cleanup

```bash
# Remove deployment
./scripts/undeploy.sh

# Or manually
kubectl delete -k k8s/

# Remove namespace (WARNING: removes all resources)
kubectl delete namespace minitoolstream
```

## Production Checklist

- [ ] Update all secrets with production values
- [ ] Configure resource limits based on load testing
- [ ] Set up monitoring (Prometheus + Grafana)
- [ ] Configure centralized logging (ELK/Loki)
- [ ] Enable TLS for gRPC
- [ ] Implement network policies
- [ ] Set up backup strategy
- [ ] Configure alerting rules
- [ ] Document runbooks for common issues
- [ ] Set up CI/CD pipeline
- [ ] Enable pod security policies
- [ ] Review and adjust HPA thresholds

## Support

For issues and questions:
- Check logs: `kubectl logs -f -n minitoolstream -l app=minitoolstream-egress`
- Review events: `kubectl get events -n minitoolstream`
- Check documentation in `k8s/README.md`
