# MiniToolStream - Complete K8s Deployment Guide

## Overview

Complete deployment guide for MiniToolStream system to k3s Kubernetes cluster.

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    k3s Cluster (minitoolstream namespace)   │
│                                                             │
│  ┌──────────────┐                                          │
│  │   Clients    │                                          │
│  └──────┬───────┘                                          │
│         │                                                   │
│         ▼                                                   │
│  ┌─────────────────┐      ┌──────────────┐                │
│  │    Ingress      │─────►│  Tarantool   │                │
│  │  Service (3+)   │      │   Service    │                │
│  │   Port: 50051   │      └──────────────┘                │
│  └────────┬────────┘               ▲                       │
│           │                        │                       │
│           │      ┌──────────────┐  │                       │
│           └─────►│    MinIO     │◄─┘                       │
│                  │   Service    │  │                       │
│                  └──────────────┘  │                       │
│                         ▲          │                       │
│                         │          │                       │
│  ┌─────────────────┐    │          │                       │
│  │     Egress      │────┴──────────┘                       │
│  │  Service (3+)   │                                       │
│  │   Port: 50052   │                                       │
│  └────────┬────────┘                                       │
│           │                                                 │
│           ▼                                                 │
│  ┌──────────────┐                                          │
│  │ Subscribers  │                                          │
│  └──────────────┘                                          │
└─────────────────────────────────────────────────────────────┘
```

## Components

### MiniToolStreamIngress
- **Purpose**: Accept and store messages from publishers
- **Port**: 50051
- **Replicas**: 3-10 (auto-scaling)
- **Dependencies**: Tarantool, MinIO

### MiniToolStreamEgress
- **Purpose**: Deliver messages to subscribers
- **Port**: 50052
- **Replicas**: 3-10 (auto-scaling)
- **Dependencies**: Tarantool, MinIO

### Shared Dependencies
- **Tarantool**: Message metadata storage (port 3301)
- **MinIO**: Large message object storage (port 9000)
- **Namespace**: minitoolstream

## Quick Deployment

### Prerequisites

```bash
# Verify k3s is running
kubectl cluster-info

# Verify Docker is running
docker version

# Verify dependencies are deployed
kubectl get pods -n minitoolstream -l app=tarantool
kubectl get pods -n minitoolstream -l app=minio
```

### Deploy Both Services

```bash
# Build images
cd MiniToolStreamIngress
./scripts/build-image.sh latest

cd ../MiniToolStreamEgress
./scripts/build-image.sh latest

# Deploy Ingress
cd ../MiniToolStreamIngress
./scripts/deploy.sh

# Deploy Egress
cd ../MiniToolStreamEgress
./scripts/deploy.sh
```

### Verify Deployment

```bash
# Check all pods
kubectl get pods -n minitoolstream

# Expected output:
# minitoolstream-ingress-xxx    1/1     Running   0          2m
# minitoolstream-ingress-yyy    1/1     Running   0          2m
# minitoolstream-ingress-zzz    1/1     Running   0          2m
# minitoolstream-egress-xxx     1/1     Running   0          2m
# minitoolstream-egress-yyy     1/1     Running   0          2m
# minitoolstream-egress-zzz     1/1     Running   0          2m

# Check services
kubectl get svc -n minitoolstream

# Check HPA
kubectl get hpa -n minitoolstream
```

## Configuration

### Secrets Management

Before deployment, update secrets in both services:

**MiniToolStreamIngress/k8s/secret.yaml:**
```yaml
stringData:
  TARANTOOL_PASSWORD: "your-production-password"
  MINIO_ACCESS_KEY_ID: "your-minio-access-key"
  MINIO_SECRET_ACCESS_KEY: "your-minio-secret-key"
```

**MiniToolStreamEgress/k8s/secret.yaml:**
```yaml
stringData:
  TARANTOOL_PASSWORD: "your-production-password"
  MINIO_ACCESS_KEY_ID: "your-minio-access-key"
  MINIO_SECRET_ACCESS_KEY: "your-minio-secret-key"
```

**Important**: Both must use the same credentials!

### Environment Variables

Both services share similar configuration:

```yaml
# Tarantool
TARANTOOL_ADDRESS: tarantool-service.minitoolstream.svc.cluster.local:3301
TARANTOOL_USER: minitoolstream_connector
TARANTOOL_PASSWORD: <from-secret>

# MinIO
MINIO_ENDPOINT: minio-service.minitoolstream.svc.cluster.local:9000
MINIO_ACCESS_KEY_ID: <from-secret>
MINIO_SECRET_ACCESS_KEY: <from-secret>
MINIO_BUCKET_NAME: minitoolstream

# Logging
LOG_LEVEL: info
LOG_FORMAT: json
```

## Resource Requirements

### Per Service (Ingress or Egress)

**Per Pod:**
- Memory: 256Mi (request) / 512Mi (limit)
- CPU: 250m (request) / 500m (limit)

**Total (3 replicas each):**
- Memory: ~1.5Gi (requests) / ~3Gi (limits)
- CPU: ~1.5 cores (requests) / ~3 cores (limits)

**Combined Total (Both services, 6 pods):**
- Memory: ~3Gi (requests) / ~6Gi (limits)
- CPU: ~3 cores (requests) / ~6 cores (limits)

## High Availability

### Features

✅ **Multiple Replicas**: 3 replicas minimum per service
✅ **Pod Anti-Affinity**: Pods distributed across nodes
✅ **Pod Disruption Budget**: Min 2 pods always available
✅ **Health Checks**: Liveness and readiness probes
✅ **Rolling Updates**: Zero-downtime deployments
✅ **Auto-Scaling**: HPA based on CPU/Memory

### Fault Tolerance

- **Node Failure**: Pods automatically rescheduled
- **Pod Failure**: Liveness probe restarts unhealthy pods
- **Network Issues**: Readiness probe removes unhealthy pods from service
- **Load Spikes**: HPA scales up to 10 replicas per service

## Monitoring & Observability

### Logs

```bash
# View all logs
kubectl logs -f -n minitoolstream -l app.kubernetes.io/part-of=minitoolstream

# Ingress logs only
kubectl logs -f -n minitoolstream -l app=minitoolstream-ingress

# Egress logs only
kubectl logs -f -n minitoolstream -l app=minitoolstream-egress

# Using stern (if installed)
stern -n minitoolstream minitoolstream
```

### Metrics

Both services expose Prometheus metrics:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "50051"  # or 50052 for egress
  prometheus.io/path: "/metrics"
```

### Health Endpoints

- **Ingress**: Port 50051 (gRPC)
- **Egress**: Port 50052 (gRPC)

## Testing

### Test Ingress

```bash
# Port forward
kubectl port-forward -n minitoolstream svc/minitoolstream-ingress-service 50051:50051

# Test with grpcurl
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext localhost:50051 IngressService/Publish
```

### Test Egress

```bash
# Port forward
kubectl port-forward -n minitoolstream svc/minitoolstream-egress-service 50052:50052

# Test with grpcurl
grpcurl -plaintext localhost:50052 list
grpcurl -plaintext localhost:50052 EgressService/Subscribe
```

## Scaling

### Manual Scaling

```bash
# Scale Ingress
kubectl scale deployment minitoolstream-ingress -n minitoolstream --replicas=5

# Scale Egress
kubectl scale deployment minitoolstream-egress -n minitoolstream --replicas=5
```

### Auto-Scaling

HPA automatically scales based on:
- CPU utilization (target: 70%)
- Memory utilization (target: 80%)

```bash
# Check HPA status
kubectl get hpa -n minitoolstream

# Describe HPA
kubectl describe hpa minitoolstream-ingress-hpa -n minitoolstream
kubectl describe hpa minitoolstream-egress-hpa -n minitoolstream
```

## Updates & Rollbacks

### Update Image

```bash
# Build new version
cd MiniToolStreamIngress
./scripts/build-image.sh v1.1.0

# Update deployment
kubectl set image deployment/minitoolstream-ingress \
  ingress=minitoolstream-ingress:v1.1.0 \
  -n minitoolstream

# Watch rollout
kubectl rollout status deployment/minitoolstream-ingress -n minitoolstream
```

### Rollback

```bash
# Rollback to previous version
kubectl rollout undo deployment/minitoolstream-ingress -n minitoolstream

# Rollback to specific revision
kubectl rollout undo deployment/minitoolstream-ingress \
  -n minitoolstream --to-revision=2

# View rollout history
kubectl rollout history deployment/minitoolstream-ingress -n minitoolstream
```

## Troubleshooting

### Common Issues

**1. Pods Not Starting**
```bash
# Check events
kubectl get events -n minitoolstream --sort-by='.lastTimestamp'

# Describe pod
kubectl describe pod <pod-name> -n minitoolstream

# Check init containers
kubectl logs <pod-name> -n minitoolstream -c wait-for-tarantool
kubectl logs <pod-name> -n minitoolstream -c wait-for-minio
```

**2. Service Unreachable**
```bash
# Check service
kubectl get svc -n minitoolstream

# Check endpoints
kubectl get endpoints -n minitoolstream

# Test DNS resolution
kubectl run -it --rm debug --image=busybox --restart=Never -n minitoolstream -- sh
nslookup minitoolstream-ingress-service
nslookup minitoolstream-egress-service
```

**3. High Resource Usage**
```bash
# Check pod resource usage
kubectl top pods -n minitoolstream

# Check HPA metrics
kubectl get hpa -n minitoolstream

# Check node resources
kubectl top nodes
```

## Cleanup

### Remove Both Services

```bash
# Undeploy Egress
cd MiniToolStreamEgress
./scripts/undeploy.sh

# Undeploy Ingress
cd ../MiniToolStreamIngress
./scripts/undeploy.sh

# Delete namespace (if no other services)
kubectl delete namespace minitoolstream
```

## Production Checklist

- [ ] Update all secrets with production credentials
- [ ] Build production images with proper tags
- [ ] Deploy to production k3s cluster
- [ ] Verify all pods running and healthy
- [ ] Test end-to-end message flow
- [ ] Configure Prometheus monitoring
- [ ] Set up Grafana dashboards
- [ ] Configure centralized logging (ELK/Loki)
- [ ] Set up alerting rules
- [ ] Enable TLS for gRPC
- [ ] Implement network policies
- [ ] Configure backup strategies
- [ ] Document runbooks
- [ ] Set up CI/CD pipeline
- [ ] Conduct load testing
- [ ] Plan disaster recovery

## Architecture Decisions

### Why 3 Replicas?
- Provides fault tolerance
- Balances cost vs. availability
- Ensures quorum for consensus operations

### Why Separate Services?
- Independent scaling
- Clear separation of concerns
- Easier debugging and monitoring
- Different resource requirements

### Why HPA?
- Automatic response to load changes
- Cost optimization during low traffic
- Maintains performance during peaks

### Why Pod Disruption Budget?
- Ensures availability during node maintenance
- Prevents all pods from being evicted simultaneously
- Maintains service SLA

## References

- [K8s Ingress Documentation](MiniToolStreamIngress/K8S_DEPLOYMENT_SUMMARY.md)
- [K8s Egress Documentation](MiniToolStreamEgress/K8S_DEPLOYMENT_SUMMARY.md)
- [Kubernetes Best Practices](https://kubernetes.io/docs/concepts/configuration/overview/)
- [k3s Documentation](https://docs.k3s.io/)
