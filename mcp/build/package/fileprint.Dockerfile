# SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
#
# SPDX-License-Identifier: Apache-2.0


# Usage:
#   docker run --rm dse-fileprint:test
#   docker run --rm dse-fileprint:test --msg "Hello from Docker"


# Fileprint Builder
# =================
FROM golang:bookworm AS builder
WORKDIR /app

COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /fileprint ./cmd/fileprint


# Fileprint Container Image
# =========================
FROM debian:bookworm-slim
LABEL maintainer="timothy.rule@de.bosch.com"

ENV DEBIAN_FRONTEND=noninteractive

RUN set -eux; \
    groupadd -g 1000 fileprint; \
    useradd -u 1000 -g fileprint -m -s /bin/bash fileprint;

RUN set -eux; \
	apt-get -y update; \
	apt-get -y upgrade; \
    apt-get -y install --no-install-recommends \
        bash \
        ca-certificates \
    && \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/*;

COPY --from=builder /fileprint /app/fileprint
WORKDIR /app
ENTRYPOINT ["/app/fileprint"]
