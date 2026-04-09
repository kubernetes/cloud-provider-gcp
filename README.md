# cloud-provider-gcp

[![Go Report Card](https://goreportcard.com/badge/k8s.io/cloud-provider-gcp)](https://goreportcard.com/report/k8s.io/cloud-provider-gcp)
[![GitHub stars](https://img.shields.io/github/stars/kubernetes/cloud-provider-gcp.svg)](https://github.com/kubernetes/cloud-provider-gcp/stargazers)
[![Contributions](https://img.shields.io/badge/contributions-welcome-orange.svg)](https://github.com/kubernetes/cloud-provider-gcp/blob/master/CONTRIBUTING.md)
[![License](https://img.shields.io/github/license/kubernetes/cloud-provider-gcp)](https://github.com/kubernetes/cloud-provider-gcp/blob/master/LICENSE)
[![Release tag](https://img.shields.io/github/v/tag/kubernetes/cloud-provider-gcp)](https://github.com/kubernetes/cloud-provider-gcp/tags)

## Introduction

This repository implements the [cloud provider](https://github.com/kubernetes/cloud-provider) interface for [Google Cloud Platform (GCP)](https://cloud.google.com/).
It provides components for Kubernetes clusters running on GCP and is maintained primarily by the Kubernetes team at Google.

To see all available commands in this repository, run `make help`.

## Components

This repository contains the following components, located in `cmd/`:

*   **Cloud Controller Manager (`cloud-controller-manager`)**: The GCP [Cloud Controller Manager (CCM)](https://kubernetes.io/docs/concepts/architecture/cloud-controller/) is responsible for running cloud-provider-dependent controllers (e.g. node health, routing, load balancing, etc.) for Kubernetes clusters running in GCP.
*   **GCP Auth Provider (`auth-provider-gcp`)**: A GCP [Container Runtime Interface (CRI)](https://kubernetes.io/docs/concepts/containers/cri/) plugin for fetching credentials for kubelet to pull images from [Google Container Registry (GCR)](https://cloud.google.com/container-registry) and [Artifact Registry (AR)](https://cloud.google.com/artifact-registry) when needed for pods.
*   **GKE Auth Plugin (`gke-gcloud-auth-plugin`)**: A client-go credential plugin that provides Google Cloud access tokens to `kubectl` and other Kubernetes clients for authenticating to [GKE clusters](https://cloud.google.com/kubernetes-engine), e.g. in `gcloud container clusters get-credentials`.

## Testing

This repository includes several testing commands you can run locally during development:

*   **`make test`**: Runs the standard Go unit tests.
*   **`make verify`**: Runs all verification scripts (format, lint, etc.).
*   **`make run-e2e-test`**: Runs the E2E test suite on a provisional [kOps](https://kops.sigs.k8s.io/) cluster.

## Publishing cloud-controller-manager image

Create an [Artifact Registry repository](https://docs.cloud.google.com/artifact-registry/docs/docker/store-docker-container-images) for the CCM image.

Then use `make publish` to build and push the `cloud-controller-manager` Docker image. For example, the following command will build and push the image to `us-central1-docker.pkg.dev/my-project/my-repo/cloud-controller-manager:v0`.
Change the location, project, and repo names to match yours.

```sh
LOCATION=us-central1 PROJECT=my-project REPO=my-repo
gcloud auth configure-docker ${LOCATION}-docker.pkg.dev
IMAGE_REPO=${LOCATION}-docker.pkg.dev/${PROJECT}/${REPO} IMAGE_TAG=v0 make publish
```

If `IMAGE_REPO` is not set, the script will exit with an error. If `IMAGE_TAG` is not set, it defaults to a unique value combining the current git commit SHA and the build date.

### Docker Commands

**Note:** To push images to Google Artifact Registry, you must first authenticate Docker by running the following command:
`gcloud auth configure-docker ${LOCATION}-docker.pkg.dev`

*   **`make publish`**: Builds the `cloud-controller-manager` Docker image (including multi-architecture support) and pushes it to the container registry specified by the `IMAGE_REPO` environment variable.

*   **`make bundle`**: Builds the `cloud-controller-manager` Docker image and saves it as a `.tar` file locally, along with creating a `.docker_tag` file. This is useful for offline distribution or loading.

*   **`make clean-builder`**: Removes the `docker buildx` builder used for multi-platform Docker builds. This command is useful to reset the builder environment if the builder encounters an error or becomes corrupted. It can also be used to free up resources when the builder is no longer needed.


## Cross-compiling

Platform-specific release tarballs can be built using the following commands.

To build all release artifacts for all platforms, run:
```sh
make release-tars
```

This command builds the release tarball for Windows (`kubernetes-node-windows-amd64.tar.gz`):

```sh
make release-tars-windows-amd64
```

This command builds the release tarballs for Linux (`kubernetes-server-linux-amd64.tar.gz` and `kubernetes-node-linux-amd64.tar.gz`):

```sh
make release-tars-linux-amd64
```


## Dependency management

Dependencies are managed using [Go modules](https://github.com/golang/go/wiki/Modules) (`go mod` subcommands).


### Working within GOPATH

If you work within `GOPATH`, `go mod` will error out unless you do one of:

- move repo outside of GOPATH (it should "just work")
- set env var `GO111MODULE=on`

### Add a new dependency

```sh
go get github.com/new/dependency && make update-vendor
```

### Update an existing dependency

```sh
go get -u github.com/existing/dependency && make update-vendor
```

### Update all dependencies

```sh
go get -u && make update-vendor
```

Note that this most likely won't work due to cross-dependency issues or repos
not implementing modules correctly.