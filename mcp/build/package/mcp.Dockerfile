# SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
#
# SPDX-License-Identifier: Apache-2.0


# Usage:
#   docker run --rm -i -v /var/run/docker.sock:/var/run/docker.sock dse-mcp:test


# MCP Builder
# ===========
FROM golang:bookworm AS builder
WORKDIR /app

COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /mcp-server ./cmd/mcp-server


# MCP Container Image
# ===================
FROM debian:bookworm-slim
LABEL maintainer="timothy.rule@de.bosch.com"

ENV DEBIAN_FRONTEND=noninteractive
ENV DOCKER_SOCKET_PATH=/var/run/docker.sock


RUN set -eux; \
    groupadd -g 1000 mcp; \
    useradd -u 1000 -g mcp -m -s /bin/bash mcp;

RUN set -eux; \
	apt-get -y update; \
	apt-get -y upgrade; \
    apt-get -y install --no-install-recommends \
        bash \
        ca-certificates \
    && \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/*;

COPY --from=builder /mcp-server /app/mcp-server
WORKDIR /app
ENTRYPOINT ["/app/mcp-server"]
