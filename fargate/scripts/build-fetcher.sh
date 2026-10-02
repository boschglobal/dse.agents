#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
FETCHER_DIR="$SCRIPT_DIR/../pkg/sidecar/s3-fetcher"
FETCHER_IMAGE="${FETCHER_IMAGE:-dse-simer-fetcher:latest}"

docker build \
    --tag "$FETCHER_IMAGE" \
    --file "$FETCHER_DIR/Dockerfile" \
    "$FETCHER_DIR"