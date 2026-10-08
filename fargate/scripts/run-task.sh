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
if [[ -n "${AWS_ENDPOINT_URL:-}" ]]; then
    AWS_ARGS+=(--endpoint-url "$AWS_ENDPOINT_URL")
    IS_SANDBOX=true
else
    IS_SANDBOX=false
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

if [[ "$IS_SANDBOX" == true ]]; then
    SANDBOX_VOLUME=$(docker volume create)
    SANDBOX_TASK_DEF=""
    sandbox_cleanup() {
        docker volume rm "$SANDBOX_VOLUME" >/dev/null 2>&1 || true
        if [[ -n "$SANDBOX_TASK_DEF" ]]; then
            aws "${AWS_ARGS[@]}" ecs deregister-task-definition \
                --task-definition "$SANDBOX_TASK_DEF" >/dev/null 2>&1 || true
        fi
    }
    trap sandbox_cleanup EXIT

    SANDBOX_COMMAND='
for attempt in {1..100}; do
    if [[ -f /sim/.simer-ready ]]; then
        cd /sim || exit 1
        exec /usr/local/bin/simer -endtime "${DURATION:-10.0}"
    fi
    sleep 0.1
done
echo "ERROR: simulation extraction did not complete within 10 seconds" >&2
exit 1
'
    TASK_DEFINITION=$(aws "${AWS_ARGS[@]}" ecs describe-task-definition \
        --task-definition "$TASK_DEF_FAMILY" --query taskDefinition --output json)
    SANDBOX_DEFINITION=$(jq --arg volume "$SANDBOX_VOLUME" \
        --arg app "$CONTAINER_NAME" --arg sidecar "$SIDECAR_CONTAINER_NAME" \
        --arg command "$SANDBOX_COMMAND" '
        del(.taskDefinitionArn, .revision, .status, .requiresAttributes,
            .compatibilities, .registeredAt, .registeredBy, .deregisteredAt)
        | .family += "-sandbox"
        | .volumes = [{name: "shared-data", host: {sourcePath: $volume}}]
        | .containerDefinitions |= map(
            if .name == $sidecar then
                .command[0] += "; touch /mnt/shared/.simer-ready"
            elif .name == $app then
                del(.dependsOn)
                | .environment = ((.environment // []) + [{name: "SIMER_EXE", value: "/bin/bash"}])
                | .command = ["-c", $command]
                | (.mountPoints[] | select(.sourceVolume == "shared-data")) .readOnly = false
            else . end)
        ' <<<"$TASK_DEFINITION")
    SANDBOX_TASK_DEF=$(aws "${AWS_ARGS[@]}" ecs register-task-definition \
        --cli-input-json "$SANDBOX_DEFINITION" \
        --query taskDefinition.taskDefinitionArn --output text)
    TASK_DEF_FAMILY="$SANDBOX_TASK_DEF"
fi

echo "Launching Fargate Task: $TASK_NAME..."

# 1. Trigger the run-task command and capture the JSON response payload
RUN_OUTPUT=$(aws "${AWS_ARGS[@]}" ecs run-task \
  --cluster "$CLUSTER_NAME" \
  --task-definition "$TASK_DEF_FAMILY" \
  --launch-type FARGATE \
  --network-configuration "$NETWORK_CONFIG" \
    --overrides "$OVERRIDES" \
    --cli-connect-timeout 3 \
    --cli-read-timeout 20)

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
    if [[ "$IS_SANDBOX" == true ]]; then
        for LOG_PID in "${SIDECAR_LOG_PID:-}" "${SIMER_LOG_PID:-}"; do
            if [[ -n "$LOG_PID" ]]; then kill "$LOG_PID" 2>/dev/null || true; fi
        done
    fi
    echo ""
    echo "Log tailing disconnected."
    exit 0
}
trap cleanup INT TERM

if [[ "$IS_SANDBOX" == true ]]; then
    SIDECAR_DOCKER_CONTAINER="ministack-ecs-${TASK_ID:0:8}-$SIDECAR_CONTAINER_NAME"
    SIMER_DOCKER_CONTAINER="ministack-ecs-${TASK_ID:0:8}-$CONTAINER_NAME"

    wait_for_container() {
        local container="$1"
        for attempt in {1..300}; do
            if docker inspect "$container" >/dev/null 2>&1; then
                return 0
            fi
            sleep 0.1
        done
        echo "Error: Container '$container' was not created within 30 seconds." >&2
        return 1
    }

    wait_for_container "$SIDECAR_DOCKER_CONTAINER"
    wait_for_container "$SIMER_DOCKER_CONTAINER"
    echo "Streaming task stdout/stderr directly from Docker..."
    docker logs --follow "$SIDECAR_DOCKER_CONTAINER" 2>&1 | sed -u 's/^/[sidecar] /' &
    SIDECAR_LOG_PID=$!
    docker logs --follow "$SIMER_DOCKER_CONTAINER" 2>&1 | sed -u 's/^/[Simer] /' &
    SIMER_LOG_PID=$!
    wait "$SIDECAR_LOG_PID"
    wait "$SIMER_LOG_PID"

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
