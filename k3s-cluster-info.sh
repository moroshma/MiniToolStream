#!/bin/bash

echo "========================================="
echo "  k3s Cluster Information"
echo "========================================="
echo ""

echo "Cluster Info:"
kubectl cluster-info
echo ""

echo "========================================="
echo "Cluster Nodes:"
kubectl get nodes -o wide
echo ""

echo "========================================="
echo "All Namespaces:"
kubectl get namespaces
echo ""

echo "========================================="
echo "All Pods (all namespaces):"
kubectl get pods --all-namespaces
echo ""

echo "========================================="
echo "All Services (all namespaces):"
kubectl get svc --all-namespaces
echo ""

echo "========================================="
echo "k3d Cluster Containers:"
docker ps --filter "name=k3d-minitoolstream" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
echo ""

echo "========================================="
echo "Dashboard Access:"
echo "URL: https://localhost:8443"
echo "Token: See k3s-dashboard-token.txt"
echo "Start: ./start-dashboard.sh"
echo "========================================="
