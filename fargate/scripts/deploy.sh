#!/usr/bin/env bash

# Exit immediately if a command exits with a non-zero status
set -e

# Defaults
STACK_NAME="dse-fargate-sandbox"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
TEMPLATE_FILE="$SCRIPT_DIR/../deployments/cloudformation/deployment.yaml"
CAPABILITIES="CAPABILITY_IAM"
SIMULATION_BUCKET_NAME="${SIMULATION_BUCKET_NAME:-my-test-bucket}"
SIMULATION_S3_ENDPOINT_URL="${SIMULATION_S3_ENDPOINT_URL:-${AWS_ENDPOINT_URL:-}}"
IMAGE_TAG="latest"
AWS_ARGS=()

# Print usage instructions
usage() {
    echo "Usage: $0 [--stack-name name]"
    echo ""
    echo "Options (Can also be set via sourced environment variables):"
    echo "  -n, --stack-name   CloudFormation stack name (Default: $STACK_NAME)"
    echo "  -h, --help         Show this help message"
    exit 1
}

# Parse command line flags (CLI flags override environment variables)
while [[ $# -gt 0 ]]; do
    case "$1" in
        -n|--stack-name)
            [[ $# -ge 2 ]] || usage
            STACK_NAME="$2"
            shift 2
            ;;
        -h|--help)        usage ;;
        *)                echo "Error: Unknown argument: $1"; usage ;;
    esac
done

# Detect environment routing
if [[ -n "${AWS_ENDPOINT_URL:-}" ]]; then
    AWS_ARGS+=(--endpoint-url "$AWS_ENDPOINT_URL")
    REGISTRY_HOST="${LOCAL_REGISTRY_HOST:-localhost:5001}"
    ENV_TARGET="Local Sandbox ($AWS_ENDPOINT_URL)"
else
    ACCOUNT_ID=$(aws sts get-caller-identity --query "Account" --output text)
    REGION="${AWS_DEFAULT_REGION:-us-east-1}"
    REGISTRY_HOST="${ACCOUNT_ID}.dkr.ecr.${REGION}.amazonaws.com"
    ENV_TARGET="Real AWS Cloud Account"
fi
APP_IMAGE_URI="$REGISTRY_HOST/dse-simer:$IMAGE_TAG"
SIDECAR_IMAGE_URI="$REGISTRY_HOST/dse-simer-fetcher:$IMAGE_TAG"

# Confirmation Prompt
echo "--------------------------------------------------"
echo " Deploying CloudFormation Stack"
echo "--------------------------------------------------"
echo "  Target Environment: $ENV_TARGET"
echo "  Stack Name:         $STACK_NAME"
echo "  S3 bucket:          $SIMULATION_BUCKET_NAME"
echo "--------------------------------------------------"
read -p "Proceed with this deployment? (y/N) " -n 1 -r
echo ""

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Deployment cancelled by user."
    exit 0
fi

aws "${AWS_ARGS[@]}" cloudformation deploy \
  --stack-name "$STACK_NAME" \
  --template-file "$TEMPLATE_FILE" \
    --parameter-overrides \
        SimulationBucketName="$SIMULATION_BUCKET_NAME" \
        SimulationS3EndpointUrl="$SIMULATION_S3_ENDPOINT_URL" \
        AppImageUri="$APP_IMAGE_URI" \
        SidecarImageUri="$SIDECAR_IMAGE_URI" \
  --capabilities "$CAPABILITIES"

echo "Deploy process submitted!"
