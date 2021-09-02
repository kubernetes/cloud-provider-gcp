# cloud-provider-gcp

## Building Container Images

This command will build and publish
`gcr.io/k8s-image-staging/cloud-controller-manager`:

```sh
make images
```

Environment variables `IMAGE_REGISTRY`, `IMAGE_REPO` and `IMAGE_TAG` can be
used to override destination GCR repository and tag.

This command will build and publish
`example.com/my-repo/cloud-controller-manager:v1`:


```sh
IMAGE_REGISTRY=example.com IMAGE_REPO=my-repo IMAGE_TAG=v1 make images
```

# Cross-compiling

This project uses standard go tools for cross-building.
```sh
GOOS=linux GOARCH=amd64 go build ./cmd/cloud-controller-manager
```
Alternatively, run `make bin` to build both server and node binaries for all supported platforms.

Server
  - linux/amd64

Node:
  - linux/amd64
  - windows/amd64

# Dependency management

This project manages dependencies using standard [Go modules](https://github.com/golang/go/wiki/Modules) (`go mod` subcommands).

## Add a new dependency

```
go get github.com/new/dependency
```

## Update an existing dependency

```
go get -u github.com/existing/dependency
```

## Update all dependencies

```
go get -u
```

Note that this most likely won't work due to cross-dependency issues or repos
not implementing modules correctly.

# Bazel

Bazel is required to build and release cloud-provider-gcp.

To install:

```sh
go get github.com/bazelbuild/bazelisk
alias bazel=bazelisk
```

To re-generate `BUILD` files:

```sh
tools/update_bazel.sh
```
## Clean up unused dependencies

```
go mod tidy
```
