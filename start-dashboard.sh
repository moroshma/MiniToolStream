#!/bin/bash

echo "========================================="
echo "  Kubernetes Dashboard Access"
echo "========================================="
echo ""
echo "Starting port-forward to Dashboard..."
echo "Dashboard will be available at: https://localhost:8443"
echo ""
echo "To login, use the token below or from: k8s-dashboard-token.txt"
echo ""
echo "Access Token:"
cat k8s-dashboard-token.txt
echo ""
echo ""
echo "Press Ctrl+C to stop the dashboard"
echo "========================================="
echo ""

kubectl -n kubernetes-dashboard port-forward svc/kubernetes-dashboard 8443:443
