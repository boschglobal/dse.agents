# Tell Make to use bash as the execution frame
SHELL := /usr/bin/env bash

.PHONY: help start stop sync deploy docker

# Default target when someone types just 'make'
help:
	@echo "================================================================="
	@echo " Fargate Processor Framework Automation"
	@echo "================================================================="
	@echo " 1. DEPLOYMENT COMMANDS"
	@echo "-----------------------------------------------------------------"
	@echo "  source env.local"
	@echo "  make start         - Initializes environment, syncs images,"
	@echo "                       and deploys CloudFormation infrastructure."
	@echo "  make docker        - Builds the local S3 fetcher sidecar image."
	@echo "  make stop          - Shuts down the environment and clears state."
	@echo "-----------------------------------------------------------------"
	@echo " 2. OPERATION COMMANDS (Run after environment is active)"
	@echo "-----------------------------------------------------------------"
	@echo "  source env.local"
	@echo "  Upload data:       ./fileop.sh upload ./my-data-payload.json"
	@echo "  List files:        ./fileop.sh list"
	@echo "  Execute worker:    ./run-task.sh processing-job s3_target_zip=s3://my-test-bucket/simulation.zip duration=10.0 workers=4"
	@echo "  Download results:  ./fileop.sh download my-data-payload.json"
	@echo "================================================================="

# Master setup chain: Runs localstack boot, then syncs, then deploys infrastructure
start:
	@echo "--> [1/3] Launching LocalStack Container..."
	./localstack.sh start
	@echo "--> [2/3] Synchronizing Container Images to Registry..."
	./sync-images.sh
	@echo "--> [3/3] Executing CloudFormation Deployment Stack..."
	./deploy.sh
	@echo "--> Environment is fully operational!"
	@$(MAKE) help

# Tearing down the sandbox environment
stop:
	@echo "--> Tearing down Sandbox Environment..."
	./localstack.sh stop
	@echo "--> Sandbox has been stopped and memory cleared."

# Hidden sub-targets called by the 'start' recipe if you ever need to run them standalone
sync:
	./sync-images.sh

deploy:
	./deploy.sh

docker:
	./build-fetcher.sh
