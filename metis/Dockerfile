FROM --platform=${BUILDPLATFORM} golang:1.24.13 AS builder

# Install cross-compilers for amd64 and arm64
RUN apt-get update && apt-get install -y \
    gcc-aarch64-linux-gnu \
    gcc-x86-64-linux-gnu \
    && rm -rf /var/lib/apt/lists/*

# golang envs
ARG TARGETARCH
ARG GOOS=linux
ENV GOARCH=${TARGETARCH}
ENV GOOS=${GOOS}

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
    CGO_ENABLED=1 go build -o metis-daemon-bin ./cmd/metis

# Use ubuntu as base image to package the binary
FROM ubuntu:22.04
WORKDIR /
COPY --from=builder /app/metis-daemon-bin .

ENTRYPOINT ["/metis-daemon-bin"]
