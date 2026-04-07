FROM --platform=${BUILDPLATFORM} golang:1.25.8 AS builder

# Install cross-compilers for amd64 and arm64
RUN apt-get update && apt-get install -y \
    gcc-aarch64-linux-gnu \
    gcc-x86-64-linux-gnu \
    sqlite3 \
    libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

# golang envs
ARG TARGETARCH
ARG GOOS=linux
ENV GOARCH=${TARGETARCH}
ENV GOOS=${GOOS}

# Versioning ARGs
ARG GIT_VERSION
ARG GIT_COMMIT
ARG BUILD_DATE

WORKDIR /app

# Copy the go source
COPY . .

# Set the correct C compiler based on the target architecture and build
RUN if [ "${TARGETARCH}" = "amd64" ]; then \
        export CC=x86_64-linux-gnu-gcc; \
    elif [ "${TARGETARCH}" = "arm64" ]; then \
        export CC=aarch64-linux-gnu-gcc; \
    else \
        echo "Unsupported architecture: ${TARGETARCH}"; exit 1; \
    fi && \
    CGO_ENABLED=1 go build -o metis \
    -ldflags="-X 'k8s.io/component-base/version.gitVersion=${GIT_VERSION}' \
              -X 'k8s.io/component-base/version.gitCommit=${GIT_COMMIT}' \
              -X 'k8s.io/component-base/version.buildDate=${BUILD_DATE}'" \
    ./cmd

# CAUTION: The Metis binary leverages CGO and links against the host's C library.
# To prevent runtime panics on baseline GKE fleet nodes, this image must remain
# compatible with the active GLIBC floor constraint defined in the Makefile.
#
# Always verify any base image upgrades by running `make test-glibc-floor`.
#
# See metis/Makefile for the full historical context and version baseline.
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y \
    sqlite3 \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /
COPY --from=builder /app/metis /metis
COPY --from=builder /app/install-plugin.sh /install-plugin.sh
RUN chmod +x /install-plugin.sh
ENTRYPOINT ["/metis"]
