#!/usr/bin/env bash
set -euo pipefail

# Add your external GHCR images here.
IMAGES=(
    "ghcr.io/boschglobal/dse-simer:latest"
)
FETCHER_IMAGE="dse-simer-fetcher:latest"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
FETCHER_DIR="$SCRIPT_DIR/../pkg/sidecar/s3-fetcher"
AWS_ARGS=()

# Detect runtime environment structure from sourced variables
if [[ -n "${AWS_ENDPOINT_URL:-}" ]]; then
    # LOCAL SANDBOX ENVIRONMENT
    REGISTRY_HOST="${LOCAL_REGISTRY_HOST:-localhost:5001}"
    echo "Targeting local Docker Registry: $REGISTRY_HOST"
    echo "Waiting for the local registry..."
    REGISTRY_READY=false
    for _ in {1..30}; do
        if curl -fsS "http://$REGISTRY_HOST/v2/" >/dev/null; then
            REGISTRY_READY=true
            break
        fi
        sleep 1
    done
    if [[ "$REGISTRY_READY" != true ]]; then
        echo "Error: Local Docker Registry at '$REGISTRY_HOST' is unavailable." >&2
        exit 1
    fi
else
    # REAL AWS CLOUD ENVIRONMENT
    # Query your actual AWS Account ID and targeted region to map the target host string
    ACCOUNT_ID=$(aws sts get-caller-identity --query "Account" --output text)
    REGION="${AWS_DEFAULT_REGION:-us-east-1}"
    REGISTRY_HOST="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"

    echo "Targeting Real AWS ECR Registry: $REGISTRY_HOST"
    echo "Authenticating Docker with real AWS ECR..."
    aws ecr get-login-password --region "$REGION" | docker login --username AWS --password-stdin "$REGISTRY_HOST"
fi

echo "Building S3 download and extraction image..."
FETCHER_IMAGE="$FETCHER_IMAGE" "$SCRIPT_DIR/build-fetcher.sh"
IMAGES+=("$FETCHER_IMAGE")

echo "Synchronizing images to registry ($REGISTRY_HOST)..."
echo "--------------------------------------------------------"

for FULL_IMAGE in "${IMAGES[@]}"; do
    # 1. Pull the image from the external registry if not already downloaded locally
    if ! docker image inspect "$FULL_IMAGE" >/dev/null 2>&1; then
        echo "Pulling '$FULL_IMAGE' from source registry..."
        docker pull "$FULL_IMAGE"
    fi

    # 2. Extract just the raw image name and tag, discarding external domains/namespaces
    # Example: ghcr.io/boschglobal/dse-simer:latest -> dse-simer:latest
    BASE_IMAGE_WITH_TAG=$(basename "$FULL_IMAGE")
    IMAGE_NAME=$(echo "$BASE_IMAGE_WITH_TAG" | cut -d':' -f1)
    IMAGE_TAG=$(echo "$BASE_IMAGE_WITH_TAG" | cut -d':' -f2)

    # 3. Retag the image for the destination registry
    TARGET_IMAGE_URI="$REGISTRY_HOST/$IMAGE_NAME:$IMAGE_TAG"
    echo "Retagging local image -> '$TARGET_IMAGE_URI'"
    docker tag "$FULL_IMAGE" "$TARGET_IMAGE_URI"

    # 5. Push to the destination registry
    echo "Pushing image to registry..."
    docker push "$TARGET_IMAGE_URI"
    echo "Done with $IMAGE_NAME."
    echo "--------------------------------------------------------"
done

echo "All images synchronized successfully!"
