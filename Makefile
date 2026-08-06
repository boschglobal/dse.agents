# Copyright 2026 Robert Bosch GmbH
#
# SPDX-License-Identifier: Apache-2.0

default: help


# Check configuration
# ===================
REUSE_IMAGE ?= fsfe/reuse:latest
REUSE ?= docker run --rm \
	--user "$$(id -u):$$(id -g)" \
	-v "$(CURDIR):/data" \
	-w /data \
	$(REUSE_IMAGE)
COPYRIGHT_HOLDER ?= Robert Bosch GmbH
COPYRIGHT_YEAR ?= $(shell date +%Y)
SPDX_LICENSE ?= Apache-2.0


# Modules/Subdirs
# ===============
SHELL := /bin/sh
MODULES ?= $(patsubst %/Makefile,%,$(wildcard */Makefile))
FORWARD_TARGETS := \
	build \
	docker \
	test \
	test_e2e \
	format \
	lint \
	mod \
	clean \
	cleanall

.PHONY: $(FORWARD_TARGETS)
$(FORWARD_TARGETS):
	@set -e; \
	for dir in $(MODULES); do \
		if $(MAKE) -C "$$dir" -n $@ >/dev/null 2>&1; then \
			echo "==> $$dir: make $@"; \
			$(MAKE) -C "$$dir" $@; \
		fi; \
	done


.PHONY: check
check: copyright-fix

.PHONY: copyright-fix
copyright-fix:
	@command -v docker >/dev/null 2>&1 || { \
		echo "error: docker is not installed"; \
		exit 1; \
	}
	@$(REUSE) annotate \
		--copyright "$(COPYRIGHT_HOLDER)" \
		--year "$(COPYRIGHT_YEAR)" \
		--license "$(SPDX_LICENSE)" \
		--merge-copyrights \
		--skip-existing \
		--skip-unrecognised \
		--recursive .
	@git ls-files --others --exclude-standard '*.license' | xargs -r rm -f

.PHONY: help
help:
	@echo "Targets:"
	@echo "  make build        Build binary files"
	@echo "  make docker       Build docker images"
	@echo "  make test         Run Unit/Integration tests"
	@echo "  make test_e2e     Run E2E tests"
	@echo "  make generate     Generate content (normally committed)"
	@echo "  make lint         Lint (and fix) the codebase"
	@echo "  make mod          Download dependencies and tidy module files"
	@echo "  make clean        Remove build artifacts"
	@echo "  make cleanall     Remove all associated build artifacts"
	@echo "Examples:"
	@echo "  make build"
	@echo "  make docker"
	@echo "  make test TEST=TestFoo"
	@echo "  make test_e2e TEST=TestFoo"

