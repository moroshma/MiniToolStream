#!/bin/bash

# Undeploy MiniToolStreamEgress from k3s
# Usage: ./scripts/undeploy.sh

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== MiniToolStreamEgress Undeployment ===${NC}"

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo -e "${RED}Error: kubectl not found${NC}"
    exit 1
fi

# Confirm deletion
read -p "Are you sure you want to delete MiniToolStreamEgress deployment? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted"
    exit 0
fi

# Delete resources in reverse order
echo -e "${YELLOW}Deleting PDB...${NC}"
kubectl delete -f k8s/pdb.yaml --ignore-not-found=true

echo -e "${YELLOW}Deleting HPA...${NC}"
kubectl delete -f k8s/hpa.yaml --ignore-not-found=true

echo -e "${YELLOW}Deleting Deployment...${NC}"
kubectl delete -f k8s/deployment.yaml --ignore-not-found=true

echo -e "${YELLOW}Deleting Service...${NC}"
kubectl delete -f k8s/service.yaml --ignore-not-found=true

echo -e "${YELLOW}Deleting Secret...${NC}"
kubectl delete -f k8s/secret.yaml --ignore-not-found=true

echo -e "${YELLOW}Deleting ConfigMap...${NC}"
kubectl delete -f k8s/configmap.yaml --ignore-not-found=true

echo -e "${YELLOW}Deleting RBAC...${NC}"
kubectl delete -f k8s/rbac.yaml --ignore-not-found=true

# Ask if user wants to delete namespace
read -p "Delete namespace 'minitoolstream'? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Deleting namespace...${NC}"
    kubectl delete -f k8s/namespace.yaml --ignore-not-found=true
fi

echo -e "${GREEN}✓ Undeployment complete${NC}"
