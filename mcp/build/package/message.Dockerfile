# SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
#
# SPDX-License-Identifier: Apache-2.0


# Usage:
#   docker run --rm dse-message:test
#   docker run --rm dse-message:test --msg "Hello from Docker"


# Message Builder
# ===============
FROM golang:bookworm AS builder
WORKDIR /app

COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /message ./cmd/message


# Message Container Image
# =======================
FROM debian:bookworm-slim
LABEL maintainer="timothy.rule@de.bosch.com"

ENV DEBIAN_FRONTEND=noninteractive

RUN set -eux; \
    groupadd -g 1000 message; \
    useradd -u 1000 -g message -m -s /bin/bash message;

RUN set -eux; \
	apt-get -y update; \
	apt-get -y upgrade; \
    apt-get -y install --no-install-recommends \
        bash \
        ca-certificates \
    && \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/*;

COPY --from=builder /message /app/message
WORKDIR /app
ENTRYPOINT ["/app/message"]
