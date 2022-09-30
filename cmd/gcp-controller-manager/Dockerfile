FROM ubuntu:latest AS golang

RUN apt-get update
RUN apt-get install -y curl git build-essential

ENV GO_VERSION 1.18b7

RUN mkdir -p /usr/lib/
RUN curl -sS --fail -L https://storage.googleapis.com/go-boringcrypto/go1.18b7.linux-amd64.tar.gz \
  | tar xz -C /usr/lib/

ENV GOROOT /usr/lib/go
ENV PATH "/usr/lib/go/bin:${PATH}"

RUN mkdir /workspace
RUN mkdir /workspace/out

WORKDIR /workspace

FROM golang AS builder

ADD . .

ENV GOFLAGS -mod=vendor

RUN go test ./...
RUN go build -o out/gcp-controller-manager \
  ./cmd/gcp-controller-manager

FROM gcr.io/distroless/base-debian11

COPY --from=builder \
  /workspace/out/gcp-controller-manager /gcp-controller-manager
