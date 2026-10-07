# Tell Make to use bash as the execution frame
SHELL := /usr/bin/env bash

.PHONY: help start stop sync deploy docker run openloop

OPENLOOP_DIR := testdata/simer/openloop

RUN_ARGUMENT_GOALS := $(filter-out run,$(MAKECMDGOALS))
RUN_ZIPFILE := $(word 1,$(RUN_ARGUMENT_GOALS))
RUN_DURATION := $(word 2,$(RUN_ARGUMENT_GOALS))
RUN_ARGUMENT_COUNT := $(words $(RUN_ARGUMENT_GOALS))

ifneq ($(filter run,$(MAKECMDGOALS)),)
.PHONY: $(RUN_ARGUMENT_GOALS)
$(RUN_ARGUMENT_GOALS):
	@:
endif

# Default target when someone types just 'make'
help:
	@echo "================================================================="
	@echo " Fargate Processor Framework Automation"
	@echo "================================================================="
	@echo " 1. DEPLOYMENT COMMANDS"
	@echo "-----------------------------------------------------------------"
	@echo "  source scripts/env.local"
	@echo "  make docker        - Builds the local S3 fetcher sidecar image."
	@echo "  make start         - Initializes environment, syncs images,"
	@echo "                       and deploys CloudFormation infrastructure."
	@echo "  make openloop      - Builds and packages the open-loop simulation ZIP."
	@echo "  make run <zipfile> <duration> - Uploads the ZIP and runs processing-job."
	@echo "  make stop          - Shuts down the environment and clears state."
	@echo "-----------------------------------------------------------------"
	@echo " 2. OPERATION COMMANDS (Run after environment is active)"
	@echo "-----------------------------------------------------------------"
	@echo "  source scripts/env.local"
	@echo "  make run testdata/simer/openloop/out/openloop.zip 10.0"
	@echo "  Upload data:       ./scripts/fileop.sh upload ./my-data-payload.json"
	@echo "  List files:        ./scripts/fileop.sh list"
	@echo "  Execute worker:    ./scripts/run-task.sh processing-job s3_target_zip=s3://my-test-bucket/simulation.zip duration=10.0 workers=4"
	@echo "  Download results:  ./scripts/fileop.sh download my-data-payload.json"
	@echo "================================================================="

# Master setup chain: Runs localstack boot, then syncs, then deploys infrastructure
start:
	@echo "--> [1/3] Launching LocalStack Container..."
	./scripts/localstack.sh start
	@echo "--> [2/3] Synchronizing Container Images to Registry..."
	./scripts/sync-images.sh
	@echo "--> [3/3] Executing CloudFormation Deployment Stack..."
	./scripts/deploy.sh
	@echo "--> Environment is fully operational!"
	@$(MAKE) help

# Tearing down the sandbox environment
stop:
	@echo "--> Tearing down Sandbox Environment..."
	./scripts/localstack.sh stop
	@echo "--> Sandbox has been stopped and memory cleared."

# Hidden sub-targets called by the 'start' recipe if you ever need to run them standalone
sync:
	./scripts/sync-images.sh

deploy:
	./scripts/deploy.sh

docker:
	./scripts/build-fetcher.sh

openloop:
	cd $(OPENLOOP_DIR) && $(MAKE) build
	rm -f $(OPENLOOP_DIR)/out/openloop.zip
	cd $(OPENLOOP_DIR)/out && zip -r openloop.zip sim

run:
	@set -euo pipefail; \
	if [[ "$(RUN_ARGUMENT_COUNT)" -ne 2 ]]; then \
		echo "Usage: make run <zipfile> <duration>" >&2; \
		exit 2; \
	fi; \
	zipfile="$(RUN_ZIPFILE)"; \
	duration="$(RUN_DURATION)"; \
	if [[ ! -f "$$zipfile" || "$$zipfile" != *.zip ]]; then \
		echo "Error: ZIP file not found or does not end in .zip: $$zipfile" >&2; \
		exit 2; \
	fi; \
	if [[ ! "$$duration" =~ ^[0-9]+([.][0-9]+)?$$ ]]; then \
		echo "Error: duration must be a non-negative integer or decimal number." >&2; \
		exit 2; \
	fi; \
	filename="$$(basename -- "$$zipfile")"; \
	bucket="$${SIMULATION_BUCKET_NAME:-my-test-bucket}"; \
	echo "--> Uploading $$filename to s3://$$bucket/$$filename..."; \
	./scripts/fileop.sh upload "$$zipfile"; \
	echo "--> Launching processing-job for $$duration seconds..."; \
	./scripts/run-task.sh processing-job "s3_target_zip=s3://$$bucket/$$filename" "duration=$$duration" workers=4