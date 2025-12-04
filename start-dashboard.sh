#!/bin/bash

echo "========================================="
echo "  Kubernetes Dashboard Access"
echo "========================================="
echo ""
echo "Starting port-forward to Dashboard..."
echo "Dashboard will be available at: https://localhost:8443"
echo ""
echo "To login, use the token from: k3s-dashboard-token.txt"
echo ""
echo "Press Ctrl+C to stop the dashboard"
echo "========================================="
echo ""

kubectl -n kubernetes-dashboard port-forward svc/kubernetes-dashboard-kong-proxy 8443:443
