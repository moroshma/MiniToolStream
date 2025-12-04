# k3s Cluster Management Guide

## Cluster Information

**Cluster Name**: minitoolstream
**Type**: k3d (k3s in Docker)
**Platform**: Mac Pro M4 Pro
**Kubernetes Version**: v1.33.4+k3s1

### Architecture

```
┌─────────────────────────────────────────────────────┐
│              k3d-minitoolstream Cluster             │
│                                                     │
│  ┌──────────────┐    ┌──────────────┐              │
│  │   Server 0   │    │   Agent 0    │              │
│  │ (Control)    │    │   (Worker)   │              │
│  └──────────────┘    └──────────────┘              │
│                      ┌──────────────┐              │
│                      │   Agent 1    │              │
│                      │   (Worker)   │              │
│                      └──────────────┘              │
│                                                     │
│  LoadBalancer:                                      │
│  - Port 8080 -> 80                                  │
│  - Port 8443 -> 443                                 │
│  - API: 6550 -> 6443                                │
└─────────────────────────────────────────────────────┘
```

## Quick Commands

### Cluster Management

```bash
# View cluster info
./k3s-cluster-info.sh

# Get cluster details
kubectl cluster-info

# View nodes
kubectl get nodes

# View all resources
kubectl get all --all-namespaces
```

### Start/Stop Cluster

```bash
# Stop cluster
k3d cluster stop minitoolstream

# Start cluster
k3d cluster start minitoolstream

# Delete cluster (WARNING: destroys all data!)
k3d cluster delete minitoolstream

# Create new cluster
k3d cluster create minitoolstream --servers 1 --agents 2 \
  --port "8080:80@loadbalancer" \
  --port "8443:443@loadbalancer" \
  --api-port 6550
```

### Dashboard Access

```bash
# Start Dashboard (easy way)
./start-dashboard.sh

# Start Dashboard (manual)
kubectl -n kubernetes-dashboard port-forward svc/kubernetes-dashboard-kong-proxy 8443:443

# Get admin token (if needed)
kubectl -n kubernetes-dashboard create token admin-user --duration=87600h

# Access URL
open https://localhost:8443
```

**Login Credentials**: See `k3s-dashboard-token.txt`

## Kubernetes Dashboard Features

The web interface provides:

- **Cluster Overview**: Real-time cluster health and resource usage
- **Workloads**: View and manage Deployments, Pods, ReplicaSets, etc.
- **Services & Discovery**: Manage Services, Ingresses, Network Policies
- **Config & Storage**: ConfigMaps, Secrets, PVCs
- **Namespace Management**: Create and manage namespaces
- **RBAC**: View and manage Roles, RoleBindings
- **Custom Resources**: View CRDs and custom resources
- **Logs & Events**: Real-time pod logs and cluster events
- **Shell Access**: Execute commands in pods
- **Resource Editing**: YAML editor for live resources

## Namespace Structure

```bash
# View all namespaces
kubectl get namespaces

# Create minitoolstream namespace (for your apps)
kubectl create namespace minitoolstream

# Set default namespace
kubectl config set-context --current --namespace=minitoolstream
```

## Deploying Applications

### Using kubectl

```bash
# Apply manifests
kubectl apply -f deployment.yaml

# Apply directory
kubectl apply -f ./k8s/

# Apply with Kustomize
kubectl apply -k ./k8s/
```

### Using Helm

```bash
# Add repo
helm repo add myrepo https://charts.example.com

# Install chart
helm install myapp myrepo/mychart -n minitoolstream

# List releases
helm list -n minitoolstream
```

## Monitoring & Debugging

### View Logs

```bash
# Pod logs
kubectl logs <pod-name> -n <namespace>

# Follow logs
kubectl logs -f <pod-name> -n <namespace>

# Previous logs (if crashed)
kubectl logs <pod-name> -n <namespace> --previous

# All pods with label
kubectl logs -l app=myapp -n minitoolstream
```

### Pod Operations

```bash
# Describe pod
kubectl describe pod <pod-name> -n <namespace>

# Execute command in pod
kubectl exec -it <pod-name> -n <namespace> -- /bin/sh

# Port forward
kubectl port-forward <pod-name> 8080:8080 -n <namespace>

# Delete pod
kubectl delete pod <pod-name> -n <namespace>
```

### Resource Usage

```bash
# Node resources
kubectl top nodes

# Pod resources
kubectl top pods -n <namespace>

# All pods
kubectl top pods --all-namespaces
```

## Persistent Storage

k3d uses local-path provisioner by default:

```bash
# View storage classes
kubectl get storageclass

# View persistent volumes
kubectl get pv

# View persistent volume claims
kubectl get pvc -n <namespace>
```

## Networking

### Services

```bash
# View services
kubectl get svc -n <namespace>

# Describe service
kubectl describe svc <service-name> -n <namespace>

# Test service DNS
kubectl run -it --rm debug --image=busybox --restart=Never -n <namespace> -- sh
nslookup <service-name>
```

### LoadBalancer Ports

The cluster exposes these ports on localhost:

- **8080**: HTTP traffic (maps to port 80 in cluster)
- **8443**: HTTPS traffic (maps to port 443 in cluster)
- **6550**: Kubernetes API

## Registry & Images

### Import Images to k3d

```bash
# Import local image
k3d image import <image-name>:<tag> -c minitoolstream

# Example
docker build -t myapp:latest .
k3d image import myapp:latest -c minitoolstream
```

### Use Images in Deployments

```yaml
spec:
  containers:
  - name: myapp
    image: myapp:latest
    imagePullPolicy: Never  # Use local image
```

## Backup & Restore

### Backup Cluster State

```bash
# Export all resources
kubectl get all --all-namespaces -o yaml > cluster-backup.yaml

# Export specific namespace
kubectl get all -n minitoolstream -o yaml > minitoolstream-backup.yaml

# Export secrets
kubectl get secrets -n minitoolstream -o yaml > secrets-backup.yaml
```

### Restore

```bash
kubectl apply -f cluster-backup.yaml
```

## Troubleshooting

### Common Issues

**1. Port already in use**
```bash
# Find process using port
lsof -i :8443
# Kill process
kill -9 <PID>
```

**2. Docker not running**
```bash
# Start Docker Desktop
open -a Docker
# Wait for it to start, then retry
```

**3. Cluster not responding**
```bash
# Restart cluster
k3d cluster stop minitoolstream
k3d cluster start minitoolstream
```

**4. Dashboard not accessible**
```bash
# Check dashboard pods
kubectl -n kubernetes-dashboard get pods

# Restart port-forward
pkill -f "port-forward.*8443"
kubectl -n kubernetes-dashboard port-forward svc/kubernetes-dashboard-kong-proxy 8443:443
```

**5. Image pull errors**
```bash
# Import image to k3d
k3d image import <image-name>:<tag> -c minitoolstream

# Or use imagePullPolicy: Never in deployment
```

### Useful Commands

```bash
# View cluster events
kubectl get events --all-namespaces --sort-by='.lastTimestamp'

# View failing pods
kubectl get pods --all-namespaces --field-selector=status.phase!=Running

# Debug networking
kubectl run -it --rm debug --image=nicolaka/netshoot --restart=Never -- bash

# View cluster certificates
kubectl get certificatesigningrequests
```

## Integration with MiniToolStream

Your MiniToolStream services are ready to deploy:

### Deploy Ingress

```bash
cd MiniToolStreamIngress
./scripts/build-image.sh latest
k3d image import minitoolstream-ingress:latest -c minitoolstream
./scripts/deploy.sh
```

### Deploy Egress

```bash
cd MiniToolStreamEgress
./scripts/build-image.sh latest
k3d image import minitoolstream-egress:latest -c minitoolstream
./scripts/deploy.sh
```

### Verify Deployment

```bash
# Check pods
kubectl get pods -n minitoolstream

# Check services
kubectl get svc -n minitoolstream

# View in Dashboard
# Navigate to: Workloads > Deployments > minitoolstream namespace
```

## Best Practices

1. **Always use namespaces** to organize resources
2. **Set resource limits** for all containers
3. **Use ConfigMaps** for configuration (not hardcoded)
4. **Use Secrets** for sensitive data
5. **Enable health checks** (liveness & readiness probes)
6. **Use labels** for organization and selection
7. **Version your images** (avoid using `:latest` in production)
8. **Test locally** before deploying to production
9. **Monitor resources** with Dashboard and metrics
10. **Backup important data** regularly

## Cleanup

### Remove Everything

```bash
# Stop Dashboard port-forward
pkill -f "port-forward.*8443"

# Delete cluster
k3d cluster delete minitoolstream

# Remove Docker volumes (optional)
docker volume prune
```

## Additional Resources

- [k3d Documentation](https://k3d.io/)
- [k3s Documentation](https://docs.k3s.io/)
- [Kubernetes Dashboard](https://kubernetes.io/docs/tasks/access-application-cluster/web-ui-dashboard/)
- [kubectl Cheat Sheet](https://kubernetes.io/docs/reference/kubectl/cheatsheet/)

## Quick Reference

| Task | Command |
|------|---------|
| Cluster info | `./k3s-cluster-info.sh` |
| Start Dashboard | `./start-dashboard.sh` |
| View nodes | `kubectl get nodes` |
| View pods | `kubectl get pods -A` |
| Apply manifests | `kubectl apply -k ./k8s/` |
| View logs | `kubectl logs -f <pod>` |
| Shell into pod | `kubectl exec -it <pod> -- sh` |
| Port forward | `kubectl port-forward <pod> 8080:8080` |
| Stop cluster | `k3d cluster stop minitoolstream` |
| Start cluster | `k3d cluster start minitoolstream` |

---

**Cluster Created**: 2025-11-26
**Dashboard URL**: https://localhost:8443
**API Server**: https://0.0.0.0:6550
