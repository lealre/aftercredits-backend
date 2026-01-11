#!/bin/bash
set -e

# --------------------------
# Default values
# --------------------------
IMAGE_NAME="${DOCKER_IMAGE_NAME:-lealre/aftercredits-backend}"
IMAGE_TAG="${DOCKER_IMAGE_TAG:-latest}"
PLATFORMS="${DOCKER_PLATFORMS:-linux/amd64,linux/arm64}"
DOCKERFILE="${DOCKERFILE_PATH:-Dockerfile.push}"

echo "🔨 Building Docker image for multiple platforms..."
echo "   Image: ${IMAGE_NAME}:${IMAGE_TAG}"
echo "   Platforms: ${PLATFORMS}"
echo "   Dockerfile: ${DOCKERFILE}"

# Ensure Buildx is available
docker buildx create --use 2>/dev/null || true
docker buildx inspect --bootstrap

# Build and push multi-platform image
docker buildx build \
    --file "${DOCKERFILE}" \
    --platform "${PLATFORMS}" \
    --tag "${IMAGE_NAME}:${IMAGE_TAG}" \
    --push \
    .

echo "✅ Successfully built and pushed ${IMAGE_NAME}:${IMAGE_TAG} for platforms: ${PLATFORMS}"