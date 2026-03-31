FROM --platform=${BUILDPLATFORM} golang:1.25.8 AS builder

# Install SQLite
RUN apt-get update && apt-get install -y \
    sqlite3 \
    && rm -rf /var/lib/apt/lists/*

# golang envs
ARG TARGETARCH
ARG GOOS=linux

# Versioning ARGs
ARG GIT_VERSION
ARG GIT_COMMIT
ARG BUILD_DATE

WORKDIR /app

# Copy the go source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${TARGETARCH} go build -o metis \
    -ldflags="-X 'k8s.io/component-base/version.gitVersion=${GIT_VERSION}' \
              -X 'k8s.io/component-base/version.gitCommit=${GIT_COMMIT}' \
              -X 'k8s.io/component-base/version.buildDate=${BUILD_DATE}'" \
    ./cmd

# Use ubuntu as base image to package the binary
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y \
    sqlite3 \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /
COPY --from=builder /app/metis .

ENTRYPOINT ["/metis"]
