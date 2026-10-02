#!/usr/bin/env bash
set -e

# Default Target Parameters
CLUSTER_NAME="customer-processing-cluster"
TASK_DEF_FAMILY="customer-s3-processing-task"
CONTAINER_NAME="dse-simer-container"
SIDECAR_CONTAINER_NAME="s3-downloader-sidecar"

# CloudWatch configurations (Only utilized when targeting real AWS)
LOG_GROUP_NAME="/ecs/customer-sidecar-processing"

SUBNET_IDS="${SUBNET_IDS:-}"
SIMULATION_BUCKET_NAME="${SIMULATION_BUCKET_NAME:-my-test-bucket}"

# Endpoint Check
AWS_ARGS=()
IS_LOCAL=false
if [[ -n "${AWS_ENDPOINT_URL:-}" ]]; then
    AWS_ARGS+=(--endpoint-url "$AWS_ENDPOINT_URL")
    IS_LOCAL=true
fi

usage() {
    echo "Usage: $0 <task-name> s3_target_zip=s3://$SIMULATION_BUCKET_NAME/<archive.zip> [key=value ...]"
    echo "Provide S3_TARGET_ZIP for the sidecar; other values become application environment variables."
    exit 1
}

if [[ $# -lt 2 ]]; then
    usage
fi

TASK_NAME="$1"
shift

# Build container environment overrides without interpolating values into jq source.
APP_ENV_JSON=$(jq -cn --arg task_name "$TASK_NAME" '[{name:"TASK_NAME",value:$task_name}]')
S3_TARGET_ZIP=""
for ARG in "$@"; do
    if [[ "$ARG" != *=* ]]; then
        echo "Error: Expected key=value argument: $ARG" >&2
        usage
    fi
    KEY="${ARG%%=*}"
    VALUE="${ARG#*=}"
    if [[ -z "$KEY" ]]; then
        echo "Error: Environment variable name cannot be empty." >&2
        usage
    fi
    if [[ "${KEY^^}" == S3_TARGET_ZIP ]]; then
        S3_TARGET_ZIP="$VALUE"
    else
        APP_ENV_JSON=$(jq -c --arg name "${KEY^^}" --arg value "$VALUE" '. + [{name:$name,value:$value}]' <<<"$APP_ENV_JSON")
    fi
done
if [[ -z "$S3_TARGET_ZIP" ]]; then
    echo "Error: s3_target_zip is required." >&2
    usage
fi
case "$S3_TARGET_ZIP" in
    "s3://$SIMULATION_BUCKET_NAME/"*) ;;
    *)
        echo "Error: ZIP URI must be inside s3://$SIMULATION_BUCKET_NAME/." >&2
        exit 1
        ;;
esac
if [[ -z "$SUBNET_IDS" ]]; then
    echo "Error: SUBNET_IDS must be set (source env.local or configure AWS subnets)." >&2
    exit 1
fi
SIDECAR_ENV_JSON=$(jq -cn --arg zip_uri "$S3_TARGET_ZIP" '[{name:"S3_TARGET_ZIP",value:$zip_uri}]')

OVERRIDES=$(jq -cn \
    --arg app_container "$CONTAINER_NAME" \
    --arg sidecar_container "$SIDECAR_CONTAINER_NAME" \
    --argjson app_env "$APP_ENV_JSON" \
    --argjson sidecar_env "$SIDECAR_ENV_JSON" \
    '{containerOverrides:[{name:$app_container,environment:$app_env},{name:$sidecar_container,environment:$sidecar_env}]}')

IFS=',' read -r -a SUBNET_ARRAY <<< "$SUBNET_IDS"
SUBNET_JSON=$(printf '%s\n' "${SUBNET_ARRAY[@]}" | jq -R . | jq -s .)
NETWORK_CONFIG=$(jq -cn --argjson subnets "$SUBNET_JSON" '{awsvpcConfiguration:{subnets:$subnets,assignPublicIp:"ENABLED"}}')

echo "Launching Fargate Task: $TASK_NAME..."

# 1. Trigger the run-task command and capture the JSON response payload
RUN_OUTPUT=$(aws "${AWS_ARGS[@]}" ecs run-task \
  --cluster "$CLUSTER_NAME" \
  --task-definition "$TASK_DEF_FAMILY" \
  --launch-type FARGATE \
  --network-configuration "$NETWORK_CONFIG" \
  --overrides "$OVERRIDES")

# 2. Extract Task Identifier Metadata
if ! TASK_ARN=$(jq -er '.tasks[0].taskArn // empty' <<<"$RUN_OUTPUT"); then
    echo "Error: ECS did not start the task:" >&2
    jq -r '.failures[]? | [.reason, (.detail // "")] | @tsv' <<<"$RUN_OUTPUT" >&2
    exit 1
fi
TASK_ID="${TASK_ARN##*/}"

echo "Task started successfully!"
echo "Task ID: $TASK_ID"
echo "--------------------------------------------------------"
echo " Tailing logs... Press Ctrl+C to stop trailing manually."
echo "--------------------------------------------------------"

# Define the log cleanup routine for graceful exit handling
cleanup() {
    echo ""
    echo "Log tailing disconnected."
    exit 0
}
trap cleanup INT TERM

if [ "$IS_LOCAL" = true ]; then
    # LOCALSTACK INTERACTION WORKFLOW:
    # LocalStack translates Fargate tasks directly into local Docker container names.
    # The standard naming format is: localstack_sandbox (or container name) hosting the runtime hash.
    # We inspect the Docker space using our task identifier to attach to stdout.

    echo "Locating sandbox container runtime..."
    sleep 2 # Let the local cluster construct the Docker container instance

    DOCKER_CONTAINER_ID=$(docker ps --filter "label=net.localstack.project=localstack" --filter "name=$TASK_ID" -q | head -n 1)

    if [[ -z "$DOCKER_CONTAINER_ID" ]]; then
        # Fallback search matching standard task format structures
        DOCKER_CONTAINER_ID=$(docker ps -a --format '{{.ID}} {{.Names}}' | grep "$TASK_ID" | cut -d' ' -f1 | head -n 1)
    fi

    if [[ -n "$DOCKER_CONTAINER_ID" ]]; then
        # Follow the live container stream directly down to the terminal frame
        docker logs -f "$DOCKER_CONTAINER_ID"
    else
        echo "Warning: Could not capture explicit Docker container log bindings for $TASK_ID."
        echo "Check main LocalStack engine log prints via: docker logs -f localstack_sandbox"
    fi

else
    # REAL AWS WORKFLOW:
    # 1. Run a detached background loop to monitor when the resource enters STOPPED status.
    (
        aws ecs wait tasks-stopped --cluster "$CLUSTER_NAME" --tasks "$TASK_ARN" >/dev/null 2>&1
        # Once stopped, kill the primary parent process shell script frame to stop tailing logs
        kill -INT $$ 2>/dev/null
    ) &

    # 2. Tail the CloudWatch Log Stream directly inside the main foreground thread loop.
    # Standard prefix scheme layout: prefix/container-name/task-id
    LOG_STREAM_NAME="app/$CONTAINER_NAME/$TASK_ID"

    echo "Waiting for CloudWatch stream provisioning ($LOG_STREAM_NAME)..."
    until aws logs describe-log-streams --log-group-name "$LOG_GROUP_NAME" --log-stream-name-prefix "$LOG_STREAM_NAME" --query "logStreams[0].logStreamName" --output text 2>/dev/null | grep -q "$TASK_ID"; do
        sleep 2
    done

    # Continuous tracking tail utility invocation
    aws logs tail "$LOG_GROUP_NAME" --follow --format short
fi

echo "Task has finished executing."
