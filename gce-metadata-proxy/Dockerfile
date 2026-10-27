# Copyright 2026 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM --platform=${BUILDPLATFORM} golang:1.26 AS builder
ARG TARGETARCH
ARG GIT_VERSION=unknown
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w -X main.version=${GIT_VERSION}" -o /gce-metadata-proxy .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /gce-metadata-proxy /gce-metadata-proxy
# Binding 169.254.169.252:988 needs CAP_NET_BIND_SERVICE. The Pod spec grants it, so the image need not run as root.
USER 65532:65532
ENTRYPOINT ["/gce-metadata-proxy"]
