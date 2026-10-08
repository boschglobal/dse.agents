#!/usr/bin/env bash

set -euo pipefail
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/../deployments/ministack/docker-compose.yml"
ENDPOINT="${AWS_ENDPOINT_URL:-http://localhost:4566}"
BUCKET_NAME="${SIMULATION_BUCKET_NAME:-my-test-bucket}"

usage() {
    echo "Usage: $0 [start|stop]"
    exit 1
}

if [[ $# -ne 1 ]]; then
    usage
fi

case "$1" in
    start)
        echo "Starting MiniStack sandbox..."
        docker compose -f "$COMPOSE_FILE" up -d

        echo "Waiting for MiniStack and the simulation bucket..."
        READY=false
        for _ in {1..60}; do
            if curl -fsS "$ENDPOINT/_ministack/health" >/dev/null \
                && AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}" \
                    AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}" \
                    AWS_DEFAULT_REGION="${AWS_DEFAULT_REGION:-us-east-1}" \
                    aws --endpoint-url "$ENDPOINT" s3api head-bucket \
                        --bucket "$BUCKET_NAME" >/dev/null 2>&1; then
                READY=true
                break
            fi
            sleep 1
        done

        if [[ "$READY" != true ]]; then
            echo "Error: MiniStack or bucket '$BUCKET_NAME' was not ready within 60 seconds." >&2
            docker logs --tail 80 ministack_sandbox >&2 || true
            exit 1
        fi

        echo "MiniStack and s3://$BUCKET_NAME are ready."
        ;;
    stop)
        echo "Stopping MiniStack sandbox..."
        docker compose -f "$COMPOSE_FILE" down
        ;;
    *)
        usage
        ;;
esac