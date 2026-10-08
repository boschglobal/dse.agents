#!/usr/bin/env bash
set -e

echo "==== Initializing Simulation S3 Bucket ===="
awslocal s3 mb "s3://${SIMULATION_BUCKET_NAME:-my-test-bucket}"

echo "==== S3 Initialization Complete ===="