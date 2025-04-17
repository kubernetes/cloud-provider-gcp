ARG GOARCH="amd64"

FROM golang:1.24 AS builder
# golang envs
ARG GOARCH="amd64"
ARG GOOS=linux
ENV CGO_ENABLED=0

WORKDIR /go/src/app
COPY go.mod go.mod
COPY go.sum go.sum
COPY providers/go.mod providers/go.mod
COPY providers/go.sum providers/go.sum
RUN go mod download
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY providers/ providers/
RUN CGO_ENABLED=0 go build -o /go/bin/cloud-controller-manager ./cmd/cloud-controller-manager

FROM registry.k8s.io/build-image/debian-base:bookworm-v1.0.4
COPY --from=builder --chown=root:root /go/bin/cloud-controller-manager /cloud-controller-manager
CMD ["/cloud-controller-manager"]
ENTRYPOINT ["/cloud-controller-manager"]