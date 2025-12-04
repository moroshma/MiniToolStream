#!/bin/bash

# Build Docker image for MiniToolStreamEgress
# Usage: ./scripts/build-image.sh [TAG]

set -e

# Configuration
IMAGE_NAME="minitoolstream-egress"
TAG="${1:-latest}"
REGISTRY="${REGISTRY:-localhost:5000}"
FULL_IMAGE="${REGISTRY}/${IMAGE_NAME}:${TAG}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Building MiniToolStreamEgress Docker image...${NC}"
echo "Image: ${FULL_IMAGE}"

# Get version info
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION="${TAG}"

echo "Git commit: ${GIT_COMMIT}"
echo "Build date: ${BUILD_DATE}"
echo "Version: ${VERSION}"

# Build the image
docker build \
  --build-arg VERSION="${VERSION}" \
  --build-arg GIT_COMMIT="${GIT_COMMIT}" \
  --build-arg BUILD_DATE="${BUILD_DATE}" \
  --tag "${IMAGE_NAME}:${TAG}" \
  --tag "${IMAGE_NAME}:latest" \
  --tag "${FULL_IMAGE}" \
  -f Dockerfile \
  .

echo -e "${GREEN}✓ Image built successfully${NC}"

# Ask if user wants to push to registry
read -p "Push image to registry? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]
then
    echo -e "${YELLOW}Pushing image to registry...${NC}"
    docker push "${FULL_IMAGE}"
    echo -e "${GREEN}✓ Image pushed successfully${NC}"
fi

echo -e "${GREEN}Done!${NC}"
echo "To deploy: kubectl apply -k k8s/"
