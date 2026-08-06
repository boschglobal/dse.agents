# SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
#
# SPDX-License-Identifier: Apache-2.0

SRC           	:= $(shell find . -type f -name '*.go' -print)
BIN_DIR       	:= $(CURDIR)/bin
CLI_TOOLS    	?= $(BIN_DIR)/message $(BIN_DIR)/mcp-server $(BIN_DIR)/mcp-run
GOFLAGS       	:=
DOCKER_TOOLS 	?= mcp message fileprint helper
COMPOSE_FILE 	?= deployments/docker-compose.yml
COMPOSE 		?= docker compose -f $(COMPOSE_FILE)

# Testscript parameters.
TESTSCRIPT_IMAGE       ?= ghcr.io/boschglobal/dse-testscript:latest
TESTSCRIPT_E2E_DIRS :=
TESTSCRIPT_E2E_DIRS += tests/e2e/runtime/docker
TESTSCRIPT_E2E_DIRS += tests/e2e/mcp
TESTSCRIPT_E2E_DIRS += tests/e2e/huma
TESTSCRIPT_E2E_FILES := $(foreach dir,$(TESTSCRIPT_E2E_DIRS),$(shell find $(dir) -type f -name "*.txtar" | sort))
ifdef TEST
TESTSCRIPT_E2E_FILES := $(TEST)
endif
ifdef SAVE
TESTSCRIPT_E2E_SAVE := -work
endif
ifdef LOG
TESTSCRIPT_E2E_LOG := -v
endif

.PHONY: all
all: build

## build: Build binary files
.PHONY: build
build: cmd

cmd: $(CLI_TOOLS)

$(CLI_TOOLS): $(SRC)
	go build $(GOFLAGS) -o $@ ./cmd/$(subst $(BIN_DIR)/,,$@)

.PHONY: install
install: build

## docker: Build docker image
.PHONY: docker
docker:
	for d in $(DOCKER_TOOLS) ;\
	do \
		docker build -f build/package/$$d.Dockerfile \
				--tag dse-$$d:test . ;\
	done;

## test: Run Go tests
.PHONY: test
test:
	go test -count=1 -run="$(TEST)" ./...

## test_e2e: Run E2E tests (parameters: TEST, SAVE, LOG)
.PHONY: test_e2e
test_e2e:
	@set -eu; \
	for t in $(TESTSCRIPT_E2E_FILES); do \
		echo "Running E2E Test: $$t"; \
		export HOST_DOCKER_WORKSPACE=$$(pwd -P) ;\
		export ENTRYWORKDIR=$$(mktemp -d) ;\
		docker run -i --rm \
			--network=host \
			-e ENTRYHOSTDIR=$${HOST_DOCKER_WORKSPACE} \
			-e ENTRYWORKDIR=$${ENTRYWORKDIR} \
			-v /var/run/docker.sock:/var/run/docker.sock \
			-v $${HOST_DOCKER_WORKSPACE}:/repo \
			-v $${ENTRYWORKDIR}:/workdir \
			$(TESTSCRIPT_IMAGE) $(TESTSCRIPT_E2E_LOG) $(TESTSCRIPT_E2E_SAVE) \
				-e ENTRYHOSTDIR=$${HOST_DOCKER_WORKSPACE} \
				-e ENTRYWORKDIR=$${ENTRYWORKDIR} \
				-e REPODIR=/repo \
				-e WORKDIR=/workdir \
				-e PACKAGE_VERSION=$(PACKAGE_VERSION) \
				$$t ;\
	done;

## mcp-start: Start the MCP system (docker compose)
.PHONY: mcp-start
mcp-start:
	@$(COMPOSE) up -d

## mcp-stop: Stop the MCP system (docker compose)
.PHONY: mcp-stop
mcp-stop:
	@$(COMPOSE) down

.PHONY: format
format:
	go fmt -x ./...

## lint: Lint (and fix) the code base
.PHONY: lint
lint:
	@docker run -t --rm \
		-v $$(pwd):/app \
		-w /app \
		golangci/golangci-lint:v2.12.2 \
			golangci-lint fmt ./...; \
			golangci-lint run --fix --disable errcheck ./...

## mod: Download dependencies and tidy module files
.PHONY: mod
mod:
	go mod download
	go mod tidy
	go mod verify

## clean: Remove all build artifacts
.PHONY: clean
clean:
	@rm -rf '$(BIN_DIR)'

.PHONY: cleanall
cleanall:

help:
	@echo "Targets:"
	@grep -h "^##" $(MAKEFILE_LIST) \
		| sed -e 's/^## //' \
		| awk '{ \
			c=index($$0, ":"); \
			if (c > 0) { \
				t=substr($$0, 1, c-1); \
				d=substr($$0, c+1); \
				printf "  %-15s %s\n", t, d \
			} else { \
				print "  " $$0 \
			} \
		}'

	@echo "Examples:"
	@echo "  make build"
	@echo "  make docker"
	@echo "  make test TEST=TestFoo"
	@echo "  make test_e2e TEST=TestFoo"
