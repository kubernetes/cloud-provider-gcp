FROM golang:1.25.6 AS builder
# golang envs
ARG TARGETARCH
ARG GOOS=linux
ENV CGO_ENABLED=0
ENV GOARCH=${TARGETARCH}

WORKDIR /go/src/app
COPY go.mod go.sum ./
COPY providers/go.mod providers/go.sum providers/
COPY vendor/ vendor/

COPY cmd/ cmd/
COPY pkg/ pkg/
COPY providers/ providers/

RUN CGO_ENABLED=0 go build -o /go/bin/cloud-controller-manager ./cmd/cloud-controller-manager

FROM registry.k8s.io/build-image/go-runner:v2.4.0-go1.25.6-bookworm.0
COPY --from=builder --chown=root:root /go/bin/cloud-controller-manager /cloud-controller-manager
CMD ["/cloud-controller-manager"]
ENTRYPOINT ["/cloud-controller-manager"]
