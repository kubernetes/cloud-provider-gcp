# Building, Testing, and Releasing

The `Makefile` in the root of the repository contains targets for building, testing, and releasing `cloud-provider-gcp`.

## Building Binaries

To build the binaries for your host OS and architecture:

```sh
make build
```

The binaries will be located in the `bin/$(GOOS)_$(GOARCH)` directory.

To build for a specific platform, set the `GOOS` and `GOARCH` environment variables:

```sh
GOOS=linux GOARCH=amd64 make build
```

## Running Tests

To run the unit tests:

```sh
make test-unit
```

To run the integration tests:

```sh
make test-integration
```

Note: The integration tests require a running Kubernetes cluster and a configured GCP environment. See `tools/run-e2e-test.sh` for more details.

## Building and Publishing Container Images

To build all container images:

```sh
make images
```

To build a specific image:

```sh
make image-cloud-controller-manager
make image-auth-provider-gcp
make image-gke-gcloud-auth-plugin
```

To publish all images to a container registry, set the `REGISTRY` and `IMAGE_TAG` environment variables:

```sh
REGISTRY=gcr.io/my-project IMAGE_TAG=v1.0.0 make publish-images
```

To publish a specific image:

```sh
REGISTRY=gcr.io/my-project IMAGE_TAG=v1.0.0 make publish-image-cloud-controller-manager
```

## Creating Release Artifacts

To create the release tarballs:

```sh
make release
```

The release artifacts will be located in the `release` directory.

To publish the release artifacts to a GCS bucket, set the `GCS_BUCKET` environment variable:

```sh
GCS_BUCKET=gs://my-bucket make publish-release
```

# Dependency management

Dependencies are managed using [Go modules](https://github.com/golang/go/wiki/Modules) (`go mod` subcommands).

Note that builds are orchestrated via the Makefile using standard Go commands. Don't follow public
Go module docs, instead use instructions in this readme.

## Working within GOPATH

If you work within `GOPATH`, `go mod` will error out unless you do one of:

- move repo outside of GOPATH (it should "just work")
- set env var `GO111MODULE=on`

## Add a new dependency

```
go get github.com/new/dependency && ./tools/update_vendor.sh
```

## Update an existing dependency

```
go get -u github.com/existing/dependency && ./tools/update_vendor.sh
```

## Update all dependencies

```
go get -u && ./tools/update_vendor.sh
```

Note that this most likely won't work due to cross-dependency issues or repos
not implementing modules correctly.

