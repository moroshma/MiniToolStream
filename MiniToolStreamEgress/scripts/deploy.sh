#!/bin/bash

# Deploy MiniToolStreamEgress to k3s
# Usage: ./scripts/deploy.sh [environment]

set -e

ENVIRONMENT="${1:-development}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== MiniToolStreamEgress Deployment ===${NC}"
echo "Environment: ${ENVIRONMENT}"

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}Error: kubectl not found${NC}"
    exit 1
fi

# Check if k3s is running
if ! kubectl cluster-info &> /dev/null; then
    echo -e "${RED}Error: Cannot connect to k3s cluster${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Connected to k3s cluster${NC}"

# Create namespace if it doesn't exist
echo -e "${YELLOW}Creating namespace...${NC}"
kubectl apply -f k8s/namespace.yaml

# Apply RBAC
echo -e "${YELLOW}Applying RBAC...${NC}"
kubectl apply -f k8s/rbac.yaml

# Apply ConfigMap
echo -e "${YELLOW}Applying ConfigMap...${NC}"
kubectl apply -f k8s/configmap.yaml

# Apply Secret (with warning)
echo -e "${YELLOW}Applying Secret...${NC}"
echo -e "${RED}WARNING: Make sure to update the secret with production values!${NC}"
kubectl apply -f k8s/secret.yaml

# Apply Service
echo -e "${YELLOW}Applying Service...${NC}"
kubectl apply -f k8s/service.yaml

# Apply Deployment
echo -e "${YELLOW}Applying Deployment...${NC}"
kubectl apply -f k8s/deployment.yaml

# Apply HPA
echo -e "${YELLOW}Applying HPA...${NC}"
kubectl apply -f k8s/hpa.yaml

# Apply PDB
echo -e "${YELLOW}Applying PDB...${NC}"
kubectl apply -f k8s/pdb.yaml

echo -e "${GREEN}✓ Deployment complete${NC}"

# Wait for pods to be ready
echo -e "${YELLOW}Waiting for pods to be ready...${NC}"
kubectl wait --for=condition=ready pod \
  -l app=minitoolstream-egress \
  -n minitoolstream \
  --timeout=300s

echo -e "${GREEN}✓ Pods are ready${NC}"

# Show status
echo -e "${BLUE}=== Deployment Status ===${NC}"
kubectl get pods -n minitoolstream -l app=minitoolstream-egress
echo ""
kubectl get svc -n minitoolstream -l app=minitoolstream-egress

echo -e "${GREEN}Deployment successful!${NC}"
echo ""
echo "To check logs: kubectl logs -f -n minitoolstream -l app=minitoolstream-egress"
echo "To check status: kubectl get pods -n minitoolstream -l app=minitoolstream-egress"
