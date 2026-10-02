#!/usr/bin/env bash

set -e
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/../deployments/localstack/docker-compose.yml"

# Usage helper
usage() {
    echo "Usage: $0 [start|stop]"
    exit 1
}

if [ $# -lt 1 ]; then
    usage
fi

ACTION=$1

case "$ACTION" in
    start)
        echo "Starting LocalStack Sandbox Container..."
        AWS_ENDPOINT_URL="" docker compose -f "$COMPOSE_FILE" up -d

        echo "Waiting for LocalStack to reach READY state and provision S3..."
        READY=false
        for _ in {1..120}; do
            if curl -fsS http://localhost:4566/_localstack/health | grep -Eq '"s3":[[:space:]]*"(available|running)"'; then
                READY=true
                break
            fi
            sleep 1
        done
        if [[ "$READY" != true ]]; then
            echo "Error: LocalStack did not become ready within 120 seconds." >&2
            exit 1
        fi

        echo "--------------------------------------------------------"
        echo " Sandbox is UP!"
        echo " S3 bucket 's3://my-test-bucket' is ready for operations."
        echo " You can now run your Fargate template deployment script."
        echo "--------------------------------------------------------"
        ;;

    stop)
        echo "Stopping LocalStack Sandbox..."
        docker compose -f "$COMPOSE_FILE" down
        echo "Sandbox has been completely stopped and resources cleared."
        ;;

    *)
        echo "Error: Unknown action '$ACTION'"
        usage
        ;;
esac
