# Kubernetes Manifests - Recent Changes

## Fixed Issues

### 1. Deprecated `commonLabels` in kustomization.yaml

**Issue**: `commonLabels` is deprecated in newer versions of Kustomize.

**Fixed in**:
- MiniToolStreamIngress/k8s/kustomization.yaml
- MiniToolStreamEgress/k8s/kustomization.yaml

**Change**:
```yaml
# OLD (deprecated)
commonLabels:
  app.kubernetes.io/name: minitoolstream-ingress
  ...

# NEW (current)
labels:
  - pairs:
      app.kubernetes.io/name: minitoolstream-ingress
      ...
```

**Reason**: The `labels` transformer with `pairs` is the current recommended approach in Kustomize v4+.

**References**:
- [Kustomize Labels Documentation](https://kubectl.docs.kubernetes.io/references/kustomize/builtins/#_labels_)
- [Kustomize v4 Migration Guide](https://kubectl.docs.kubernetes.io/guides/config_management/components/)

## Verification

Test the kustomization files:

```bash
# Validate Ingress manifests
kubectl kustomize MiniToolStreamIngress/k8s/

# Validate Egress manifests
kubectl kustomize MiniToolStreamEgress/k8s/

# Apply with dry-run
kubectl apply -k MiniToolStreamIngress/k8s/ --dry-run=client
kubectl apply -k MiniToolStreamEgress/k8s/ --dry-run=client
```

## Other Best Practices Applied

### Security
✅ Non-root user (UID 1000)
✅ Read-only root filesystem
✅ Security context with dropped capabilities
✅ No privilege escalation

### Resource Management
✅ Resource requests and limits defined
✅ HPA for auto-scaling
✅ PDB for high availability

### Monitoring
✅ Prometheus annotations
✅ Structured logging
✅ Health check probes

## Future Considerations

- Consider using `bases` for common configuration
- Implement network policies
- Add ServiceMonitor for Prometheus Operator
- Consider using Helm charts for easier management
