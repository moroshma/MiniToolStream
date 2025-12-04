# k3s Cluster - Quick Start Guide

## ✅ Cluster Successfully Deployed!

Your k3s cluster is up and running with Kubernetes Dashboard web interface.

---

## 🚀 Quick Access

### Kubernetes Dashboard (Web Interface)

**1. Start the Dashboard:**
```bash
cd /Users/moroshma/go/DiplomaThesis/MiniToolStream
./start-dashboard.sh
```

**2. Open in Browser:**
- URL: **https://localhost:8443**
- Browser will automatically open (or click the URL above)

**3. Login:**
- Select **Token** authentication method
- Copy token from `k3s-dashboard-token.txt` file
- Paste and click **Sign In**

**Admin Token** (valid for 10 years):
```
eyJhbGciOiJSUzI1NiIsImtpZCI6ImhSVzFxcU1DeXVVUVVTSC02dmVQOFhKMzdjTmV2RTNrMkl5Y0owVWVLMm8ifQ.eyJhdWQiOlsiaHR0cHM6Ly9rdWJlcm5ldGVzLmRlZmF1bHQuc3ZjLmNsdXN0ZXIubG9jYWwiLCJrM3MiXSwiZXhwIjoyMDc5NDczODA2LCJpYXQiOjE3NjQxMTM4MDYsImlzcyI6Imh0dHBzOi8va3ViZXJuZXRlcy5kZWZhdWx0LnN2Yy5jbHVzdGVyLmxvY2FsIiwianRpIjoiZDU0NmJkMDktNWY0YS00MTM1LTk3Y2ItNGU5ZWY5MDlmZmRhIiwia3ViZXJuZXRlcy5pbyI6eyJuYW1lc3BhY2UiOiJrdWJlcm5ldGVzLWRhc2hib2FyZCIsInNlcnZpY2VhY2NvdW50Ijp7Im5hbWUiOiJhZG1pbi11c2VyIiwidWlkIjoiYWFiMzVhZTEtZDcyYS00OTVlLWJkZDYtODQwNDY4Y2EzNzBiIn19LCJuYmYiOjE3NjQxMTM4MDYsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDprdWJlcm5ldGVzLWRhc2hib2FyZDphZG1pbi11c2VyIn0.patGPOTGUkI9L638a1t7YTuaSlWYC99KC53yUoItqksg-zz-jrBqK8Q3YlHx3qj50KlOnDx_TJ01jPyFDvPIJnmpgGgO5SYOwj0BsVPy2yQCn8DoTANZSOklQNqrfQMbJ89aEQt-L2lVJ_8_IfRnzyGYkXlUIOK25UCPWBmsAM8apJdLu1hYnZEBpAtYt76wfROKQYgCFHUVKsTmJ3i18PocKJiV7BRYRUeZteXzEG1HCA5i01foy2_jHJmXr80c5pPBsW2pg6gcE1SJVGESm5UpXiHq7fgnqegUKwon3YtQEylVnFoKO9-ikK4h6O2hlStr3NMUzCjDDqCjMEzcHg
```

---

## 📊 Cluster Information

```bash
./k3s-cluster-info.sh
```

**Cluster Details:**
- **Name**: minitoolstream
- **Nodes**: 1 server (control plane) + 2 agents (workers)
- **Kubernetes Version**: v1.33.4+k3s1
- **Platform**: k3d (k3s in Docker)
- **API Server**: https://0.0.0.0:6550

**Exposed Ports:**
- `8080` → HTTP (port 80)
- `8443` → HTTPS (port 443)
- `6550` → Kubernetes API

---

## 🎯 Common Tasks

### View Cluster Resources

```bash
# All resources
kubectl get all --all-namespaces

# Nodes
kubectl get nodes

# Pods
kubectl get pods -A

# Services
kubectl get svc -A
```

### Stop/Start Cluster

```bash
# Stop (preserves all data)
k3d cluster stop minitoolstream

# Start
k3d cluster start minitoolstream

# Delete (WARNING: destroys everything!)
k3d cluster delete minitoolstream
```

### Deploy Your Applications

```bash
# Create namespace
kubectl create namespace minitoolstream

# Deploy using Kustomize
kubectl apply -k ./MiniToolStreamIngress/k8s/
kubectl apply -k ./MiniToolStreamEgress/k8s/

# Check deployment
kubectl get pods -n minitoolstream
```

---

## 📁 Created Files

| File | Description |
|------|-------------|
| `start-dashboard.sh` | Quick script to start Dashboard |
| `k3s-cluster-info.sh` | Display cluster information |
| `k3s-dashboard-token.txt` | Admin token for Dashboard login |
| `k3s-dashboard-admin.yaml` | Admin user configuration |
| `K3S_CLUSTER_GUIDE.md` | Complete cluster management guide |
| `K3S_QUICK_START.md` | This file - quick reference |

---

## 🔍 What's in the Dashboard?

Once logged in, you can:

1. **Overview** - Cluster health, resource usage, warnings
2. **Workloads** - Deployments, Pods, ReplicaSets, StatefulSets, Jobs
3. **Services & Discovery** - Services, Ingresses, Network Policies
4. **Config & Storage** - ConfigMaps, Secrets, PVCs, Storage Classes
5. **Custom Resources** - CRDs and custom resources
6. **Logs & Events** - Real-time pod logs and cluster events
7. **Exec** - Shell access to running pods
8. **Edit Resources** - YAML editor for live resources

---

## 🛠 Troubleshooting

### Dashboard not loading?

```bash
# 1. Check if port-forward is running
lsof -i :8443

# 2. Restart port-forward
pkill -f "port-forward.*8443"
./start-dashboard.sh
```

### Cluster not responding?

```bash
# Check Docker is running
docker ps

# Restart cluster
k3d cluster stop minitoolstream
k3d cluster start minitoolstream
```

### Can't find token?

```bash
# Token is saved in:
cat k3s-dashboard-token.txt

# Or generate new one:
kubectl -n kubernetes-dashboard create token admin-user --duration=87600h
```

---

## 📚 Next Steps

1. ✅ **Cluster is ready** - Dashboard is running
2. 📖 **Read the guide** - See `K3S_CLUSTER_GUIDE.md` for detailed docs
3. 🚀 **Deploy apps** - Your MiniToolStream services are ready to deploy
4. 📊 **Monitor** - Use Dashboard to monitor your deployments

---

## 🎉 Success!

Your k3s cluster is fully operational with web-based monitoring!

**Dashboard**: https://localhost:8443
**Token**: See `k3s-dashboard-token.txt`

For detailed instructions, see: `K3S_CLUSTER_GUIDE.md`

---

**Created**: 2025-11-26
**Cluster**: minitoolstream
**Platform**: Mac Pro M4 Pro
