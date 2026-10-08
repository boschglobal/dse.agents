#!/usr/bin/env bash
set -e

# Configuration
BUCKET_NAME="${SIMULATION_BUCKET_NAME:-my-test-bucket}"

# Check for environmental endpoint overrides
ENDPOINT_FLAG=""
if [[ -n "$AWS_ENDPOINT_URL" ]]; then
    ENDPOINT_FLAG="--endpoint-url=$AWS_ENDPOINT_URL"
    ENV_TARGET="Local Sandbox"
else
    ENV_TARGET="Real AWS Cloud"
fi

# Print usage instructions
usage() {
    echo "Usage: $0 <command> [arguments]"
    echo ""
    echo "Available Commands:"
    echo "  upload <path-to-local-file>   Uploads a file to the S3 bucket"
    echo "  list                          Lists all files currently in the bucket"
    echo "  download <s3-filename>        Downloads a file from S3 to the current directory"
    exit 1
}

# Ensure at least one argument is provided
if [[ $# -lt 1 ]]; then
    usage
fi

COMMAND="$1"

case "$COMMAND" in
    upload)
        FILE_PATH="$2"
        if [[ -z "$FILE_PATH" ]]; then
            echo "Error: Please specify the local file path to upload."
            usage
        fi

        FILENAME=$(basename "$FILE_PATH")
        echo "[$ENV_TARGET] Uploading $FILENAME to s3://$BUCKET_NAME/ ..."
        aws $ENDPOINT_FLAG s3 cp "$FILE_PATH" "s3://$BUCKET_NAME/$FILENAME"
        echo "Upload successful!"
        ;;

    list)
        echo "[$ENV_TARGET] Listing files in s3://$BUCKET_NAME/ ..."
        echo "--------------------------------------------------------"
        # Using standard s3 ls command
        aws $ENDPOINT_FLAG s3 ls "s3://$BUCKET_NAME/" || echo "(Bucket is empty or does not exist)"
        echo "--------------------------------------------------------"
        ;;

    download)
        S3_FILENAME="$2"
        if [[ -z "$S3_FILENAME" ]]; then
            echo "Error: Please specify the filename to download from S3."
            usage
        fi

        echo "[$ENV_TARGET] Downloading s3://$BUCKET_NAME/$S3_FILENAME to current folder..."
        aws $ENDPOINT_FLAG s3 cp "s3://$BUCKET_NAME/$S3_FILENAME" "./$S3_FILENAME"
        echo "Download successful!"
        ;;

    *)
        echo "Error: Unknown command '$COMMAND'"
        usage
        ;;
esac
