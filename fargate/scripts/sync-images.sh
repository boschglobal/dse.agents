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
    # MiniStack exposes ECR's API and Docker Registry V2 protocol on its gateway.
    AWS_ARGS+=(--endpoint-url "$AWS_ENDPOINT_URL")
    REGISTRY_HOST="${MINISTACK_ECR_REGISTRY_HOST:-localhost:4566}"
    echo "Targeting MiniStack ECR: $REGISTRY_HOST"
    echo "Authenticating Docker with MiniStack ECR..."
    aws "${AWS_ARGS[@]}" ecr get-login-password \
        | docker login --username AWS --password-stdin "$REGISTRY_HOST"
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

echo "Synchronizing images to ECR ($REGISTRY_HOST)..."
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

    # ECR's Docker V2 endpoint requires a repository to exist before pushing.
    if [[ -n "${AWS_ENDPOINT_URL:-}" ]] \
        && ! aws "${AWS_ARGS[@]}" ecr describe-repositories \
            --repository-names "$IMAGE_NAME" >/dev/null 2>&1; then
        echo "Creating MiniStack ECR repository: $IMAGE_NAME"
        aws "${AWS_ARGS[@]}" ecr create-repository --repository-name "$IMAGE_NAME" >/dev/null
    fi

    # Use MiniStack's gateway endpoint for Docker V2; its ECR API state is shared.
    TARGET_IMAGE_URI="$REGISTRY_HOST/$IMAGE_NAME:$IMAGE_TAG"
    echo "Retagging image -> '$TARGET_IMAGE_URI'"
    docker tag "$FULL_IMAGE" "$TARGET_IMAGE_URI"

    # 5. Push to the destination registry
    echo "Pushing image to registry..."
    docker push "$TARGET_IMAGE_URI"
    echo "Done with $IMAGE_NAME."
    echo "--------------------------------------------------------"
done

echo "All images synchronized successfully!"
